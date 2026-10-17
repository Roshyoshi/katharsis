package cses

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeCSES serves the testdata pages the way cses.fi does: a PHPSESSID
// session per client, a login form with a per-session csrf_token, a new
// session id on login, and a tests page that is 404 to a logged-out user
// and whose form posts back a zip.
type fakeCSES struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	nick     string
	pass     string
	sessions map[string]*fakeSession
	zips     map[string][]byte // task id -> tests zip
	hits     map[string]int    // "POST /login" -> count
	// zipBody, when set, is served instead of the zip: a page CSES might
	// send in its place.
	zipBody func() []byte
}

type fakeSession struct {
	csrf string
	nick string // "" logged out
}

func newFake(t *testing.T) *fakeCSES {
	f := &fakeCSES{
		t: t, nick: "Ada", pass: "hunter2 \"x\\",
		sessions: map[string]*fakeSession{},
		zips:     map[string][]byte{},
		hits:     map[string]int{},
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeCSES) count(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[key]
}

// expire ends every session server-side, as CSES does after a while.
func (f *fakeCSES) expire() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sessions = map[string]*fakeSession{}
}

func token() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func fixture(t *testing.T, name string) string {
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var (
	taskPathRE  = regexp.MustCompile(`^/problemset/task/(\d+)/?$`)
	testsPathRE = regexp.MustCompile(`^/problemset/tests/(\d+)/$`)
)

func (f *fakeCSES) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hits[r.Method+" "+r.URL.Path]++
	if !strings.HasPrefix(r.UserAgent(), "katharsis/") {
		http.Error(w, "no user agent", http.StatusForbidden)
		return
	}
	sid := ""
	if ck, err := r.Cookie(sessionCookie); err == nil {
		sid = ck.Value
	}
	s := f.sessions[sid]
	if s == nil {
		sid, s = token(), &fakeSession{csrf: token()}
		f.sessions[sid] = s
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: sid, Path: "/"})
	}
	page := func(name string, extra ...string) {
		acct := `<a class="account" href="/login">Login</a>`
		if s.nick != "" {
			acct = `<a class="account" href="/user/42">` + s.nick + `</a>`
		}
		body := strings.NewReplacer(append([]string{
			"<!--ACCOUNT-->", acct, "<!--CSRF-->", s.csrf,
		}, extra...)...).Replace(fixture(f.t, name))
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		_, _ = w.Write([]byte(body))
	}
	p := r.URL.Path
	switch {
	case p == "/problemset/list/" && r.Method == http.MethodGet:
		page("list.html")
	case taskPathRE.MatchString(p):
		name := "task_" + taskPathRE.FindStringSubmatch(p)[1] + ".html"
		if _, err := os.Stat(filepath.Join("testdata", name)); err != nil {
			http.NotFound(w, r)
			return
		}
		page(name)
	case p == "/" && r.Method == http.MethodGet:
		page("home.html")
	case p == "/login" && r.Method == http.MethodGet:
		page("login.html")
	case p == "/login" && r.Method == http.MethodPost:
		if r.FormValue("csrf_token") != s.csrf || r.FormValue("nick") != f.nick || r.FormValue("pass") != f.pass {
			page("login.html", "<!--MESSAGE-->", `<p class="error">Invalid username or password</p>`)
			return
		}
		// A new session id on login, as PHP's session_regenerate_id.
		delete(f.sessions, sid)
		sid = token()
		f.sessions[sid] = &fakeSession{csrf: token(), nick: f.nick}
		http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: sid, Path: "/"})
		http.Redirect(w, r, "/", http.StatusFound)
	case testsPathRE.MatchString(p):
		id := testsPathRE.FindStringSubmatch(p)[1]
		if s.nick == "" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodGet {
			page("tests.html")
			return
		}
		if r.FormValue("csrf_token") != s.csrf || r.FormValue("download") != "true" {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		if f.zipBody != nil {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(f.zipBody())
			return
		}
		z, ok := f.zips[id]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(z)
	default:
		http.NotFound(w, r)
	}
}

// makeZip builds a zip of name -> content.
func makeZip(t *testing.T, files map[string]string) []byte {
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for name, s := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// fakeKeys is an in-memory keyring.
type fakeKeys struct {
	mu   sync.Mutex
	m    map[string]string
	fail error // returned by Set
}

func newKeys() *fakeKeys { return &fakeKeys{m: map[string]string{}} }

func (k *fakeKeys) Set(nick, pass string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.fail != nil {
		return k.fail
	}
	k.m[nick] = pass
	return nil
}

func (k *fakeKeys) Get(nick string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	p, ok := k.m[nick]
	if !ok {
		return "", errNoPassword
	}
	return p, nil
}

func (k *fakeKeys) Delete(nick string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, nick)
	return nil
}

// testPace sends requests without the production gap.
func testPace() *pacer {
	return &pacer{Backoff: time.Millisecond, Retries: 1, Timeout: 10 * time.Second}
}

// openFake opens a catalog on f with a fresh cache root.
func openFake(t *testing.T, f *fakeCSES, keys keyring) (*Catalog, string) {
	t.Helper()
	root := t.TempDir()
	c, err := open(context.Background(), root, f.srv.URL, testPace(), keys)
	if err != nil {
		t.Fatal(err)
	}
	return c, root
}

var errBoom = errors.New("boom")
