package session

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ctlEnv makes the test binary act as `deadcases ctl`, so the overlay tests
// drive the real client: vim runs <test binary> ctl <cmd>.
const ctlEnv = "DEADCASES_TEST_CTL"

func TestMain(m *testing.M) {
	if os.Getenv(ctlEnv) == "1" && len(os.Args) > 1 && os.Args[1] == "ctl" {
		os.Exit(CtlMain(os.Args[2:], os.Stdout, os.Stderr))
	}
	os.Exit(m.Run())
}

// sockDir is a short scratch dir: macOS caps socket paths at 104 bytes,
// which t.TempDir under /var/folders can exceed.
func sockDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "dcctl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

// serve runs a control server until the test ends and returns its path and
// the requests it delivers.
func serve(t *testing.T) (string, <-chan Request) {
	t.Helper()
	path := filepath.Join(sockDir(t), "g.sock")
	s, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	reqs := make(chan Request, 16)
	done := make(chan error, 1)
	go func() { done <- s.Serve(func(r Request) { reqs <- r }) }()
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
		if err := <-done; err != nil {
			t.Errorf("Serve: %v", err)
		}
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("socket %s left behind (lstat: %v)", path, err)
		}
	})
	return path, reqs
}

// answer replies to the next request with rep once it arrives, and returns
// what was asked.
func answer(t *testing.T, reqs <-chan Request, rep Reply) <-chan Command {
	t.Helper()
	got := make(chan Command, 1)
	go func() {
		select {
		case r := <-reqs:
			got <- r.Command
			r.Answer(rep)
		case <-time.After(patience):
		}
	}()
	return got
}

func TestControlRoundTrip(t *testing.T) {
	path, reqs := serve(t)
	got := answer(t, reqs, Reply{OK: true, Text: "WA 37/52"})
	rep, err := Call(path, Command{Cmd: CmdDebug, Args: []string{"3"}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if rep != (Reply{OK: true, Text: "WA 37/52"}) {
		t.Errorf("reply = %+v", rep)
	}
	if c := <-got; c.Cmd != CmdDebug || len(c.Args) != 1 || c.Args[0] != "3" {
		t.Errorf("server saw %+v, want debug [3]", c)
	}
}

// Commands outside the protocol, and lines that are not JSON, are refused
// by the server; the game never sees them.
func TestControlRefusesUnknown(t *testing.T) {
	path, reqs := serve(t)
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(patience))
	c.Write([]byte("{\"cmd\":\"rm -rf\"}\nnot json\n"))
	dec := json.NewDecoder(c)
	for _, want := range []string{"unknown command", "bad request"} {
		var rep Reply
		if err := dec.Decode(&rep); err != nil {
			t.Fatal(err)
		}
		if rep.OK || !strings.Contains(rep.Text, want) {
			t.Errorf("reply %+v, want a refusal containing %q", rep, want)
		}
	}
	select {
	case r := <-reqs:
		t.Errorf("game was handed %+v", r.Command)
	default:
	}
}

// Several commands on one connection are answered in order.
func TestControlPipelined(t *testing.T) {
	path, reqs := serve(t)
	go func() {
		for i := 0; i < 3; i++ {
			r := <-reqs
			r.Answer(Reply{OK: true, Text: r.Cmd + strconv.Itoa(i)})
		}
	}()
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(patience))
	c.Write([]byte("{\"cmd\":\"status\"}\n{\"cmd\":\"quack\"}\n{\"cmd\":\"submit\"}\n"))
	dec := json.NewDecoder(c)
	for _, want := range []string{"status0", "quack1", "submit2"} {
		var rep Reply
		if err := dec.Decode(&rep); err != nil {
			t.Fatal(err)
		}
		if rep.Text != want {
			t.Errorf("reply %q, want %q", rep.Text, want)
		}
	}
}

// A request the game never answered is answered at Close, so `ctl` does
// not hang on a game that quit.
func TestControlCloseAnswersPending(t *testing.T) {
	path := filepath.Join(sockDir(t), "g.sock")
	s, err := Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	delivered := make(chan struct{})
	go s.Serve(func(Request) { close(delivered) })
	type result struct {
		rep Reply
		err error
	}
	res := make(chan result, 1)
	go func() {
		rep, err := Call(path, Command{Cmd: CmdSubmit}, patience)
		res <- result{rep, err}
	}()
	<-delivered
	s.Close()
	r := <-res
	if r.err == nil && (r.rep.OK || !strings.Contains(r.rep.Text, "shutting down")) {
		t.Errorf("pending call got %+v, want a shutting-down refusal or an error", r.rep)
	}
}

func TestListenStaleAndLive(t *testing.T) {
	dir := sockDir(t)
	path := filepath.Join(dir, "g.sock")

	// A socket file nobody listens on: what a killed game leaves.
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	ln.SetUnlinkOnClose(false)
	ln.Close()
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("stale socket not left behind: %v", err)
	}
	s, err := Listen(path)
	if err != nil {
		t.Fatalf("Listen over a stale socket: %v", err)
	}
	defer s.Close()

	if s2, err := Listen(path); err == nil {
		s2.Close()
		t.Fatal("Listen over a live socket succeeded")
	}

	long := filepath.Join(dir, strings.Repeat("x", 120)+".sock")
	if s3, err := Listen(long); err == nil {
		s3.Close()
		t.Fatal("Listen on a 130-byte path succeeded")
	} else if !strings.Contains(err.Error(), "bytes") {
		t.Errorf("long path error %q should say why", err)
	}
}

func TestCtlMain(t *testing.T) {
	path, reqs := serve(t)
	for _, tc := range []struct {
		name   string
		sock   string
		args   []string
		rep    *Reply // answer given, nil for none expected
		code   int
		stdout string
		stderr string
	}{
		{"ok", path, []string{"submit"}, &Reply{OK: true, Text: "AC 52/52"}, 0, "AC 52/52\n", ""},
		{"refused", path, []string{"quack"}, &Reply{Text: "no ducks left"}, 1, "no ducks left\n", ""},
		{"no args", path, nil, nil, 2, "", "usage"},
		{"no socket variable", "", []string{"status"}, nil, 2, "", SockEnv},
		{"no game", path + ".gone", []string{"status"}, nil, 2, "", "no game"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(SockEnv, tc.sock)
			if tc.rep != nil {
				answer(t, reqs, *tc.rep)
			}
			var out, errb bytes.Buffer
			code := CtlMain(tc.args, &out, &errb)
			if code != tc.code || out.String() != tc.stdout || !strings.Contains(errb.String(), tc.stderr) {
				t.Errorf("CtlMain = %d, stdout %q, stderr %q; want %d, %q, stderr with %q",
					code, out.String(), errb.String(), tc.code, tc.stdout, tc.stderr)
			}
		})
	}
}

func TestRunDirAndSweep(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	dir, err := RunDir()
	if err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(dir); err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("RunDir %s: %v, mode %v; want 0700", dir, err, fi.Mode())
	}
	sock, _ := SocketPath()
	ov, _ := OverlayPath()
	me := strconv.Itoa(os.Getpid())
	if filepath.Base(sock) != me+".sock" || filepath.Base(ov) != me+".vim" || filepath.Dir(sock) != dir {
		t.Errorf("SocketPath %s, OverlayPath %s; want %s/%s.{sock,vim}", sock, ov, dir, me)
	}

	// pid 1 (launchd/init) is alive; a pid past pid_max is not.
	files := []string{me + ".vim", "1.vim", "99999999.sock", "99999999-debug-3", "notes.txt"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := SweepRunDir(); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		_, err := os.Stat(filepath.Join(dir, f))
		gone := os.IsNotExist(err)
		if want := strings.HasPrefix(f, "99999999"); gone != want {
			t.Errorf("%s: removed=%v, want %v", f, gone, want)
		}
	}
}
