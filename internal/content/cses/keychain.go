package cses

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// keyring keeps the CSES password, so an expired session can be renewed
// without asking the player. Tests use a fake.
type keyring interface {
	// Set stores pass for nick, replacing any password stored before.
	Set(nick, pass string) error
	// Get returns the password stored for nick, errNoPassword when none.
	Get(nick string) (string, error)
	// Delete forgets nick's password; a missing one is not an error.
	Delete(nick string) error
}

// errNoPassword means the keyring has no password for the account, so a
// session that expires cannot be renewed.
var errNoPassword = errors.New("no CSES password stored")

// keychainService names the generic password item in the macOS Keychain.
const keychainService = "katharsis-cses"

// systemKeyring is the macOS Keychain on darwin. Elsewhere there is no
// password store the game can rely on, so it keeps nothing: the session
// cookie works until CSES expires it, and then the player logs in again.
func systemKeyring() keyring {
	if runtime.GOOS == "darwin" {
		return &keychain{bin: "/usr/bin/security"}
	}
	return noKeyring{}
}

// noKeyring stores nothing.
type noKeyring struct{}

func (noKeyring) Set(string, string) error   { return nil }
func (noKeyring) Get(string) (string, error) { return "", errNoPassword }
func (noKeyring) Delete(string) error        { return nil }

// keychain drives the macOS Keychain through security(1).
//
// The password must not appear in argv, which any local user can read
// with ps. "security add-generic-password ... -w <pass>" puts it there,
// and the prompting form (-w last) reads /dev/tty, which a game in a
// terminal pane must not grab. So Set runs "security -i", the
// interactive mode, and writes the add-generic-password command line to
// its stdin, the password quoted inside it: argv shows only "security
// -i". Get and Delete have no secret to pass and use argv.
type keychain struct {
	bin  string // security(1)
	file string // keychain file; "" is the default (login) keychain
}

func (k *keychain) Set(nick, pass string) error {
	if strings.ContainsAny(nick+pass, "\n\r\x00") {
		return errors.New("the username or password holds a line break, which the Keychain command line cannot carry")
	}
	line := "add-generic-password -U -s " + quote(keychainService) + " -a " + quote(nick) + " -w " + quote(pass)
	if k.file != "" {
		line += " " + quote(k.file)
	}
	cmd := exec.Command(k.bin, "-i")
	cmd.Stdin = strings.NewReader(line + "\n")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("security add-generic-password: %w", err)
	}
	// security -i exits 0 even when a command fails, so read the item
	// back.
	got, err := k.Get(nick)
	if err != nil || got != pass {
		return fmt.Errorf("security add-generic-password did not store the password: %s", strings.TrimSpace(out.String()))
	}
	return nil
}

func (k *keychain) Get(nick string) (string, error) {
	args := []string{"find-generic-password", "-s", keychainService, "-a", nick, "-w"}
	if k.file != "" {
		args = append(args, k.file)
	}
	var out, errb bytes.Buffer
	cmd := exec.Command(k.bin, args...)
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 44 { // errSecItemNotFound
			return "", errNoPassword
		}
		return "", fmt.Errorf("security find-generic-password: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSuffix(out.String(), "\n"), nil
}

func (k *keychain) Delete(nick string) error {
	args := []string{"delete-generic-password", "-s", keychainService, "-a", nick}
	if k.file != "" {
		args = append(args, k.file)
	}
	var errb bytes.Buffer
	cmd := exec.Command(k.bin, args...)
	cmd.Stdout, cmd.Stderr = &errb, &errb
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 44 {
			return nil
		}
		return fmt.Errorf("security delete-generic-password: %w: %s", err, strings.TrimSpace(errb.String()))
	}
	return nil
}

// quote is a word for security -i's command line: double quotes, with
// backslash and double quote escaped (checked 2026-10-17, macOS 26.5).
func quote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
