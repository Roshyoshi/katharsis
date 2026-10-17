package cses

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"katharsis/internal/catalog"
)

var ctx = context.Background()

func judgeable(t *testing.T, c *Catalog, native string) bool {
	t.Helper()
	p, ok := c.Get(catalog.NewID(catalog.CSES, native))
	if !ok {
		t.Fatalf("no task %s", native)
	}
	for _, q := range c.All() {
		if q.ID == p.ID && q.Judgeable != p.Judgeable {
			t.Fatalf("%s: All and Get disagree", native)
		}
	}
	return p.Judgeable
}

func TestOpenBuildsIndex(t *testing.T) {
	f := newFake(t)
	c, root := openFake(t, f, newKeys())
	if n := len(c.All()); n != 6 {
		t.Fatalf("%d tasks, want 6", n)
	}
	if f.count("GET /problemset/list/") != 1 {
		t.Errorf("list read %d times", f.count("GET /problemset/list/"))
	}
	// A second open is offline.
	f.srv.Close()
	c2, err := cached(root, f.srv.URL, testPace(), newKeys())
	if err != nil {
		t.Fatal(err)
	}
	if len(c2.All()) != 6 || c2.refreshed != nil {
		t.Errorf("cached: %d tasks", len(c2.All()))
	}
	c3, err := open(ctx, root, f.srv.URL, testPace(), newKeys())
	if err != nil || c3.refreshed != nil || len(c3.All()) != 6 {
		t.Errorf("fresh cache: %v, refresh %v", err, c3.refreshed)
	}
	if _, err := cached(t.TempDir(), f.srv.URL, testPace(), newKeys()); !errors.Is(err, ErrNoIndex) {
		t.Errorf("empty cache: %v, want ErrNoIndex", err)
	}
}

func TestOpenOfflineFallsBack(t *testing.T) {
	f := newFake(t)
	f.srv.Close()
	root := t.TempDir()
	c, err := open(ctx, root, f.srv.URL, testPace(), newKeys())
	if err != nil {
		t.Fatal(err)
	}
	tab, _ := loadFactsTable()
	if len(c.All()) != len(tab) {
		t.Errorf("%d tasks, want facts.json's %d", len(c.All()), len(tab))
	}
	if _, ok := loadIndex(filepath.Join(root, "index.json")); ok {
		t.Error("the fallback list was saved; the next Open would not ask again")
	}
}

func TestOpenRefreshesStaleList(t *testing.T) {
	f := newFake(t)
	root := t.TempDir()
	old := &index{Key: indexKey, Listed: time.Now().Add(-2 * StaleAfter), Rows: []row{{ID: "1068", Title: "Old", Section: "Introductory Problems", Position: 1, Solvers: 10}}}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := saveIndex(filepath.Join(root, "index.json"), old); err != nil {
		t.Fatal(err)
	}
	c, err := open(ctx, root, f.srv.URL, testPace(), newKeys())
	if err != nil {
		t.Fatal(err)
	}
	if c.refreshed == nil {
		t.Fatal("no background refresh for a stale list")
	}
	<-c.refreshed
	if n := len(c.All()); n != 6 {
		t.Errorf("after refresh: %d tasks", n)
	}
	idx, ok := loadIndex(filepath.Join(root, "index.json"))
	if !ok || time.Since(idx.Listed) > time.Minute || len(idx.Rows) != 6 {
		t.Errorf("refreshed index not saved: %+v", idx)
	}
}

func TestLogin(t *testing.T) {
	f := newFake(t)
	keys := newKeys()
	c, root := openFake(t, f, keys)
	if _, ok := c.Account(); ok {
		t.Fatal("account before login")
	}
	if err := c.Login(ctx, "Ada", "wrong"); !errors.Is(err, ErrBadLogin) {
		t.Fatalf("bad password: %v", err)
	}
	if _, ok := c.Account(); ok || len(keys.m) != 0 {
		t.Fatal("a rejected login was kept")
	}
	if err := c.Login(ctx, " Ada ", f.pass); err != nil {
		t.Fatal(err)
	}
	if nick, ok := c.Account(); !ok || nick != "Ada" {
		t.Errorf("account = %q %v, want Ada", nick, ok)
	}
	if keys.m["Ada"] != f.pass {
		t.Error("password not in the keyring")
	}
	p := filepath.Join(root, "account.json")
	fi, err := os.Stat(p)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("account.json: %v %v", fi, err)
	}
	b, _ := os.ReadFile(p)
	if strings.Contains(string(b), "hunter2") {
		t.Error("account.json holds the password")
	}
	var a account
	if json.Unmarshal(b, &a) != nil || a.Nick != "Ada" || a.Session == "" {
		t.Errorf("account.json = %s", b)
	}
	// The stored session is the logged-in one: a new catalog uses it.
	c2, err := cached(root, f.srv.URL, testPace(), keys)
	if err != nil {
		t.Fatal(err)
	}
	if nick, ok := c2.Account(); !ok || nick != "Ada" {
		t.Error("account not loaded from the cache")
	}
	f.zips["1068"] = makeZip(t, map[string]string{"1.in": "3\n", "1.out": "3 10 5 16 8 4 2 1\n"})
	if _, err := c2.Fetch(ctx, "cses:1068"); err != nil {
		t.Fatal(err)
	}
	if n := f.count("POST /login"); n != 2 {
		t.Errorf("%d login posts; the stored session should have served", n)
	}

	// A keyring failure fails the login and keeps nothing new.
	keys.fail = errBoom
	c3, _ := openFake(t, f, keys)
	if err := c3.Login(ctx, "Ada", f.pass); !errors.Is(err, errBoom) {
		t.Errorf("keyring failure: %v", err)
	}
	if _, ok := c3.Account(); ok {
		t.Error("account kept without its password")
	}
}

func TestLoginNeedsCSRF(t *testing.T) {
	f := newFake(t)
	c, _ := openFake(t, f, newKeys())
	// A jar CSES has not seen fetches a token for another session.
	jar := c.newJar("")
	resp, err := c.pace.do(ctx, request{url: f.srv.URL + "/login", jar: c.newJar("")})
	if err != nil {
		t.Fatal(err)
	}
	stale := csrfToken(resp.body)
	resp, err = c.pace.do(ctx, request{url: f.srv.URL + "/login", jar: jar, form: map[string][]string{
		"csrf_token": {stale}, "nick": {f.nick}, "pass": {f.pass},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, in := loggedIn(resp.body); in {
		t.Fatal("the fake accepted a token from another session")
	}
	if _, err := c.signIn(ctx, c.newJar(""), f.nick, f.pass); err != nil {
		t.Fatalf("signIn: %v", err)
	}
}

func TestJudgeableFollowsAccount(t *testing.T) {
	f := newFake(t)
	f.zips["1068"] = makeZip(t, map[string]string{"1.in": "3\n", "1.out": "3 10 5 16 8 4 2 1\n"})
	c, _ := openFake(t, f, newKeys())
	if judgeable(t, c, "1068") {
		t.Fatal("judgeable with no account and no tests")
	}
	if err := c.Login(ctx, "Ada", f.pass); err != nil {
		t.Fatal(err)
	}
	if !judgeable(t, c, "1068") || !judgeable(t, c, "1084") {
		t.Fatal("not judgeable after login")
	}
	if judgeable(t, c, "3112") {
		t.Error("an interactive task is judgeable")
	}
	if _, err := c.Fetch(ctx, "cses:1068"); err != nil {
		t.Fatal(err)
	}
	// 1070 accepts several answers: facts.json says so, and so does its
	// page once fetched.
	if _, err := c.Fetch(ctx, "cses:1070"); err != nil {
		t.Fatal(err)
	}
	if judgeable(t, c, "1070") {
		t.Error("a several-answers task is judgeable")
	}
	if err := c.Logout(); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Account(); ok {
		t.Error("account after logout")
	}
	if !judgeable(t, c, "1068") {
		t.Error("cached tests stopped being judgeable at logout")
	}
	if judgeable(t, c, "1084") {
		t.Error("judgeable after logout with no tests cached")
	}
	if _, err := os.Stat(c.accountPath()); !os.IsNotExist(err) {
		t.Error("account.json left after logout")
	}
}

func TestFetch(t *testing.T) {
	f := newFake(t)
	f.zips["1068"] = makeZip(t, map[string]string{
		"1.in": "3\n", "1.out": "3 10 5 16 8 4 2 1\n",
		"2.in": "1\n", "2.out": "1\n",
	})
	keys := newKeys()
	c, root := openFake(t, f, keys)
	if err := c.Login(ctx, "Ada", f.pass); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make([]catalog.Assets, 8)
	errs := make([]error, 8)
	for i := range results {
		wg.Go(func() { results[i], errs[i] = c.Fetch(ctx, "cses:1068") })
	}
	wg.Wait()
	for i := range results {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if len(results[i].Tests) != 2 || len(results[i].Samples) != 1 {
			t.Fatalf("fetch %d: %+v", i, results[i])
		}
	}
	if n := f.count("POST /problemset/tests/1068/"); n != 1 {
		t.Errorf("zip downloaded %d times", n)
	}
	a := results[0]
	md, _ := os.ReadFile(a.Statement)
	for _, s := range []string{"# Weird Algorithm", "*CSES Problem Set, Introductory Problems*", "Time limit: 1 s · Memory limit: 512 MB", "Source: <https://cses.fi/problemset/task/1068>"} {
		if !strings.Contains(string(md), s) {
			t.Errorf("statement.md lacks %q", s)
		}
	}
	in, _ := os.ReadFile(a.Samples[0].In)
	out, _ := os.ReadFile(a.Tests[1].Out)
	if string(in) != "3\n" || string(out) != "1\n" {
		t.Errorf("sample in %q, test 2 out %q", in, out)
	}

	// Cached for good: no account and no server needed.
	f.srv.Close()
	if err := c.Logout(); err != nil {
		t.Fatal(err)
	}
	c2, err := cached(root, f.srv.URL, testPace(), keys)
	if err != nil {
		t.Fatal(err)
	}
	if !judgeable(t, c2, "1068") {
		t.Error("cached tests not judgeable in a new catalog")
	}
	a2, err := c2.Fetch(ctx, "cses:1068")
	if err != nil || len(a2.Tests) != 2 {
		t.Fatalf("offline fetch: %+v %v", a2, err)
	}
	// The limits read from the page reached the index.
	idx, _ := loadIndex(filepath.Join(root, "index.json"))
	if _, ok := idx.Verified["1068"]; !ok {
		t.Error("verified facts not saved")
	}
}

func TestFetchUnjudgeable(t *testing.T) {
	f := newFake(t)
	c, _ := openFake(t, f, newKeys())
	if err := c.Login(ctx, "Ada", f.pass); err != nil {
		t.Fatal(err)
	}
	for _, id := range []catalog.ID{"cses:3112", "cses:1092"} {
		a, err := c.Fetch(ctx, id)
		if err != nil || a.Statement == "" || len(a.Tests) != 0 {
			t.Errorf("%s: %+v %v", id, a, err)
		}
	}
	if n := f.count("GET /problemset/tests/1092/") + f.count("GET /problemset/tests/3112/"); n != 0 {
		t.Errorf("tests asked for %d times for unjudgeable tasks", n)
	}
	// Logged out, a judgeable-in-principle task gets its statement only.
	if err := c.Logout(); err != nil {
		t.Fatal(err)
	}
	a, err := c.Fetch(ctx, "cses:1084")
	if err != nil || len(a.Tests) != 0 || len(a.Samples) != 1 {
		t.Errorf("logged out: %+v %v", a, err)
	}
	if p, _ := c.Get("cses:1084"); p.TimeLimit != 2500*time.Millisecond || p.MemLimit != 256<<20 {
		t.Errorf("limits from the page not applied: %v %d", p.TimeLimit, p.MemLimit)
	}
	if _, err := c.Fetch(ctx, "cses:9999"); err == nil {
		t.Error("fetched a task not in the catalog")
	}
}

func TestFetchRelogsIn(t *testing.T) {
	f := newFake(t)
	f.zips["1084"] = makeZip(t, map[string]string{"1.in": "x\n", "1.out": "y\n"})
	keys := newKeys()
	c, root := openFake(t, f, keys)
	if err := c.Login(ctx, "Ada", f.pass); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, "account.json"))
	f.expire()
	// Several fetches find the session expired at once; one login renews it.
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			if _, err := c.Fetch(ctx, "cses:1084"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if n := f.count("POST /login"); n != 2 {
		t.Errorf("%d login posts, want 2 (login, one relogin)", n)
	}
	after, _ := os.ReadFile(filepath.Join(root, "account.json"))
	if string(before) == string(after) {
		t.Error("the renewed session was not saved")
	}
}

func TestFetchReloginFails(t *testing.T) {
	for _, tc := range []struct {
		name  string
		spoil func(f *fakeCSES, k *fakeKeys)
	}{
		{"password changed", func(f *fakeCSES, _ *fakeKeys) { f.pass = "new" }},
		{"no stored password", func(_ *fakeCSES, k *fakeKeys) { delete(k.m, "Ada") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake(t)
			f.zips["1084"] = makeZip(t, map[string]string{"1.in": "x\n", "1.out": "y\n"})
			keys := newKeys()
			c, root := openFake(t, f, keys)
			if err := c.Login(ctx, "Ada", f.pass); err != nil {
				t.Fatal(err)
			}
			f.expire()
			tc.spoil(f, keys)
			_, err := c.Fetch(ctx, "cses:1084")
			if !errors.Is(err, ErrLoginNeeded) || !strings.Contains(err.Error(), "log in to CSES in settings") {
				t.Fatalf("err = %v, want ErrLoginNeeded", err)
			}
			if _, err := os.Stat(filepath.Join(root, "1084", "tests")); !os.IsNotExist(err) {
				t.Error("tests dir published after a failed download")
			}
		})
	}
}

func TestFetchRejectsPageForZip(t *testing.T) {
	f := newFake(t)
	f.zipBody = func() []byte {
		return []byte(strings.ReplaceAll(fixture(t, "home.html"), "<!--ACCOUNT-->", `<a class="account" href="/user/42">Ada</a>`))
	}
	c, root := openFake(t, f, newKeys())
	if err := c.Login(ctx, "Ada", f.pass); err != nil {
		t.Fatal(err)
	}
	_, err := c.Fetch(ctx, "cses:1068")
	if err == nil || !strings.Contains(err.Error(), "instead of the tests zip") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "1068", "tests")); !os.IsNotExist(err) {
		t.Error("a page was cached as tests")
	}
	// A logged-out page in the zip's place is an expired session: one
	// relogin, then the error.
	f.zipBody = func() []byte {
		return []byte(strings.ReplaceAll(fixture(t, "login.html"), "<!--ACCOUNT-->", `<a class="account" href="/login">Login</a>`))
	}
	_, err = c.Fetch(ctx, "cses:1068")
	if err == nil || !strings.Contains(err.Error(), "after logging in again") {
		t.Fatalf("err = %v", err)
	}
	if n := f.count("POST /login"); n != 2 {
		t.Errorf("%d login posts, want 2", n)
	}
	// The statement was kept; only the tests are missing.
	if _, _, ok := loadStatement(filepath.Join(root, "1068", "statement")); !ok {
		t.Error("statement not cached")
	}
}

// TestKeychain runs the real security(1) against a throwaway keychain
// file, never the login keychain. Opt in with KATHARSIS_KEYCHAIN=1.
func TestKeychain(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("KATHARSIS_KEYCHAIN") != "1" {
		t.Skip("set KATHARSIS_KEYCHAIN=1 on darwin to drive security(1)")
	}
	file := filepath.Join(t.TempDir(), "t.keychain-db")
	run := func(args ...string) {
		k := &keychain{bin: "/usr/bin/security"}
		if out, err := execCommand(k.bin, args...); err != nil {
			t.Fatalf("security %v: %v %s", args, err, out)
		}
	}
	run("create-keychain", "-p", "probe", file)
	t.Cleanup(func() { _, _ = execCommand("/usr/bin/security", "delete-keychain", file) })
	run("unlock-keychain", "-p", "probe", file)
	k := &keychain{bin: "/usr/bin/security", file: file}
	if _, err := k.Get("ada"); !errors.Is(err, errNoPassword) {
		t.Fatalf("empty keychain: %v", err)
	}
	pass := `a\b'c $x"y% z`
	if err := k.Set("ada", pass); err != nil {
		t.Fatal(err)
	}
	if got, err := k.Get("ada"); err != nil || got != pass {
		t.Fatalf("Get = %q %v, want %q", got, err, pass)
	}
	if err := k.Set("ada", "second"); err != nil {
		t.Fatal(err)
	}
	if got, _ := k.Get("ada"); got != "second" {
		t.Errorf("Set did not replace: %q", got)
	}
	if err := k.Delete("ada"); err != nil {
		t.Fatal(err)
	}
	if err := k.Delete("ada"); err != nil {
		t.Errorf("second delete: %v", err)
	}
	if _, err := k.Get("ada"); !errors.Is(err, errNoPassword) {
		t.Errorf("after delete: %v", err)
	}
	if err := k.Set("ada", "two\nlines"); err == nil {
		t.Error("a password with a newline was passed to security -i")
	}
}

func TestNoKeyring(t *testing.T) {
	var k noKeyring
	if err := k.Set("a", "b"); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Get("a"); !errors.Is(err, errNoPassword) {
		t.Errorf("noKeyring kept a password: %v", err)
	}
}

func execCommand(bin string, args ...string) ([]byte, error) {
	return exec.Command(bin, args...).CombinedOutput()
}
