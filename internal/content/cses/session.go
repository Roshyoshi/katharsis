package cses

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"katharsis/internal/content/web"
)

// ---- the HTTP client -----------------------------------------------------

// pacer spaces requests to cses.fi, a site run by one university group:
// one request at a time, Gap apart, across every Catalog in the process.
type pacer struct {
	Gap     time.Duration
	Backoff time.Duration // first retry wait of a GET; doubles after
	Retries int           // GET attempts after the first
	Timeout time.Duration // one attempt, body included

	sem     chan struct{}
	semOnce sync.Once
	last    time.Time
}

// pace is the process's pacer for cses.fi.
var pace = &pacer{Gap: time.Second, Backoff: 2 * time.Second, Retries: 2, Timeout: 10 * time.Minute}

// sessionCookie is the cookie CSES keeps its session in.
const sessionCookie = "PHPSESSID"

// request is one request to the site. A non-nil form makes it a POST.
type request struct {
	url   string
	form  url.Values
	jar   http.CookieJar
	limit int64
	// into, when set, receives the body instead of a buffer: the tests
	// zip goes straight to disk.
	into io.Writer
}

// response is what came back, after redirects.
type response struct {
	status int
	body   []byte // nil when the request had into
	header http.Header
}

// errRetry marks an attempt worth repeating: 5xx, 429, a transport error.
type errRetry struct{ error }

func (e errRetry) Unwrap() error { return e.error }

// do runs r under the pacer. A GET is retried on 5xx, 429 and transport
// errors; a POST is sent once. Any status comes back as a response, for
// the caller to judge: CSES answers a logged-out tests page with 404.
func (p *pacer) do(ctx context.Context, r request) (*response, error) {
	attempts := 1
	if r.form == nil {
		attempts += p.Retries
	}
	wait := p.Backoff
	for i := 0; ; i++ {
		resp, err := p.once(ctx, r)
		var retry errRetry
		if err == nil || !errors.As(err, &retry) || i+1 >= attempts || ctx.Err() != nil {
			return resp, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait + rand.N(wait/2+1)):
		}
		wait *= 2
	}
}

func (p *pacer) once(ctx context.Context, r request) (*response, error) {
	p.semOnce.Do(func() { p.sem = make(chan struct{}, 1) })
	select {
	case p.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() {
		p.last = time.Now()
		<-p.sem
	}()
	if wait := time.Until(p.last.Add(p.Gap)); wait > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}

	method, body := http.MethodGet, io.Reader(nil)
	if r.form != nil {
		method, body = http.MethodPost, strings.NewReader(r.form.Encode())
	}
	if p.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.Timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, method, r.url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", web.UserAgent+" (+CSES content source)")
	if r.form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if u, err := url.Parse(r.url); err == nil {
			req.Header.Set("Origin", u.Scheme+"://"+u.Host)
			req.Header.Set("Referer", r.url)
		}
	}
	hc := &http.Client{Jar: r.jar}
	resp, err := hc.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, err
		}
		return nil, errRetry{err}
	}
	defer func() { _ = resp.Body.Close() }()
	what := method + " " + r.url
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return nil, errRetry{fmt.Errorf("%s: %s", what, resp.Status)}
	}
	out := &response{status: resp.StatusCode, header: resp.Header}
	lim := r.limit
	if lim <= 0 {
		lim = maxPage
	}
	if r.into != nil && resp.StatusCode == http.StatusOK {
		n, err := io.Copy(r.into, io.LimitReader(resp.Body, lim+1))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", what, err)
		}
		if n > lim {
			return nil, fmt.Errorf("%s: body over %d bytes", what, lim)
		}
		return out, nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, min(lim, maxPage)+1))
	if err != nil {
		return nil, errRetry{fmt.Errorf("%s: %w", what, err)}
	}
	if int64(len(b)) > min(lim, maxPage) {
		return nil, fmt.Errorf("%s: body over %d bytes", what, min(lim, maxPage))
	}
	out.body = b
	return out, nil
}

// maxPage caps an HTML page; the problem list is about 60 KB.
const maxPage = 4 << 20

// ---- the account ---------------------------------------------------------

// account is account.json, mode 0600: who is logged in and the session
// cookie. The password is never here; see keyring.
type account struct {
	Nick    string `json:"nick"`
	Session string `json:"session"` // PHPSESSID
}

// ErrLoginNeeded means CSES needs a login the game cannot make by itself:
// no account is set, or the session expired and the stored password is
// missing or no longer accepted. Its text is for the player.
var ErrLoginNeeded = errors.New("CSES login expired: log in to CSES in settings")

// ErrBadLogin means CSES rejected the username and password.
var ErrBadLogin = errors.New("CSES did not accept that username and password")

func (c *Catalog) accountPath() string { return filepath.Join(c.root, "account.json") }

func loadAccount(p string) (*account, bool) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	var a account
	if json.Unmarshal(b, &a) != nil || a.Nick == "" {
		return nil, false
	}
	return &a, true
}

// saveAccount writes account.json readable by the owner only. web.WriteFile
// creates its temp file 0600 (os.CreateTemp) and renames it, so the
// cookie is never readable by others, not even briefly; the chmod states
// it rather than relying on that.
func saveAccount(p string, a account) error {
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	if err := web.WriteFile(p, b); err != nil {
		return err
	}
	return os.Chmod(p, 0o600)
}

// newJar is a cookie jar holding session for the site, or nothing.
func (c *Catalog) newJar(session string) http.CookieJar {
	jar, _ := cookiejar.New(nil) // never fails without options
	if session != "" {
		if u, err := url.Parse(c.base + "/"); err == nil {
			jar.SetCookies(u, []*http.Cookie{{Name: sessionCookie, Value: session, Path: "/"}})
		}
	}
	return jar
}

func (c *Catalog) jarSession(jar http.CookieJar) string {
	u, err := url.Parse(c.base + "/")
	if err != nil {
		return ""
	}
	for _, ck := range jar.Cookies(u) {
		if ck.Name == sessionCookie {
			return ck.Value
		}
	}
	return ""
}

// Account is the CSES account in use; ok is false when none is.
func (c *Catalog) Account() (nick string, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.acct == nil {
		return "", false
	}
	return c.acct.Nick, true
}

// Login signs in to cses.fi as nick and, when CSES accepts, keeps the
// account: the session cookie in account.json and, on darwin, the
// password in the Keychain, so an expired session renews itself. A
// rejected login returns ErrBadLogin and leaves any account in use as it
// was. From its return, every task that can be judged is Judgeable.
func (c *Catalog) Login(ctx context.Context, nick, pass string) error {
	nick = strings.TrimSpace(nick)
	if nick == "" || pass == "" {
		return errors.New("CSES login needs a username and a password")
	}
	c.authMu.Lock()
	defer c.authMu.Unlock()
	jar := c.newJar("")
	who, err := c.signIn(ctx, jar, nick, pass)
	if err != nil {
		return err
	}
	prev, _ := c.Account()
	if err := c.keys.Set(who, pass); err != nil {
		return fmt.Errorf("CSES accepted the login, but the password could not be saved: %w", err)
	}
	if prev != "" && prev != who {
		_ = c.keys.Delete(prev)
	}
	return c.setAccount(&account{Nick: who, Session: c.jarSession(jar)}, jar)
}

// Logout forgets the account: the stored password, the session and
// account.json. Tasks whose tests are cached stay Judgeable.
func (c *Catalog) Logout() error {
	c.authMu.Lock()
	defer c.authMu.Unlock()
	nick, ok := c.Account()
	if !ok {
		return nil
	}
	if err := c.keys.Delete(nick); err != nil {
		return err
	}
	if err := os.Remove(c.accountPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	c.mu.Lock()
	c.acct, c.jar = nil, nil
	c.authGen++
	c.mu.Unlock()
	return nil
}

// setAccount persists a and makes it the account in use. authMu is held.
func (c *Catalog) setAccount(a *account, jar http.CookieJar) error {
	if err := saveAccount(c.accountPath(), *a); err != nil {
		return err
	}
	c.mu.Lock()
	c.acct, c.jar = a, jar
	c.authGen++
	c.mu.Unlock()
	return nil
}

// session is the jar to send requests with and the login generation it
// belongs to, for relogin; ok is false with no account.
func (c *Catalog) session() (jar http.CookieJar, gen int, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.acct == nil {
		return nil, c.authGen, false
	}
	return c.jar, c.authGen, true
}

// relogin renews an expired session with the stored password. gen is the
// generation the caller found expired: if another caller has renewed it
// since, there is nothing to do, so a burst of fetches logs in once. A
// missing or rejected password is ErrLoginNeeded; a network failure is
// returned as it is, since logging in again in settings would not help.
func (c *Catalog) relogin(ctx context.Context, gen int) error {
	c.authMu.Lock()
	defer c.authMu.Unlock()
	c.mu.RLock()
	a, cur := c.acct, c.authGen
	c.mu.RUnlock()
	if a == nil {
		return ErrLoginNeeded
	}
	if cur != gen {
		return nil
	}
	pass, err := c.keys.Get(a.Nick)
	if err != nil {
		if errors.Is(err, errNoPassword) {
			return ErrLoginNeeded
		}
		return fmt.Errorf("%w (%v)", ErrLoginNeeded, err)
	}
	jar := c.newJar("")
	who, err := c.signIn(ctx, jar, a.Nick, pass)
	if errors.Is(err, ErrBadLogin) {
		return ErrLoginNeeded
	}
	if err != nil {
		return err
	}
	return c.setAccount(&account{Nick: who, Session: c.jarSession(jar)}, jar)
}

// signIn runs CSES's login form on jar: GET /login for the session cookie
// and the form's csrf_token, then POST csrf_token, nick and pass. CSES
// answers either way with a page whose header link a.account names the
// user (href /user/<n>) or says "Login" (href /login); that link is the
// signal, as in ketankr9/cses-cli. It returns the nick as CSES spells it.
func (c *Catalog) signIn(ctx context.Context, jar http.CookieJar, nick, pass string) (string, error) {
	u := c.base + "/login"
	resp, err := c.pace.do(ctx, request{url: u, jar: jar})
	if err != nil {
		return "", fmt.Errorf("CSES login page: %w", err)
	}
	if resp.status != http.StatusOK {
		return "", fmt.Errorf("CSES login page: status %d", resp.status)
	}
	csrf := csrfToken(resp.body)
	if csrf == "" {
		return "", errors.New("CSES login page has no csrf_token")
	}
	resp, err = c.pace.do(ctx, request{url: u, jar: jar, form: url.Values{
		"csrf_token": {csrf}, "nick": {nick}, "pass": {pass},
	}})
	if err != nil {
		return "", fmt.Errorf("CSES login: %w", err)
	}
	if resp.status != http.StatusOK {
		return "", fmt.Errorf("CSES login: status %d", resp.status)
	}
	who, in := loggedIn(resp.body)
	switch {
	case in && strings.EqualFold(who, nick):
		return who, nil
	case in:
		return "", fmt.Errorf("CSES logged in as %q, not %q", who, nick)
	}
	return "", ErrBadLogin
}

// csrfToken is the value of the page's hidden csrf_token input, "" when it
// has none.
func csrfToken(page []byte) string {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return ""
	}
	in := find(doc, func(n *html.Node) bool {
		return n.DataAtom == atom.Input && attr(n, "name") == "csrf_token"
	})
	if in == nil {
		return ""
	}
	return attr(in, "value")
}

// loggedIn reads the header of any CSES page: a.account links to
// /user/<n> with the nick as its text when a user is logged in, and to
// /login otherwise.
func loggedIn(page []byte) (nick string, ok bool) {
	doc, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return "", false
	}
	a := find(doc, elem(atom.A, "account"))
	if a == nil {
		return "", false
	}
	href := attr(a, "href")
	if !strings.HasPrefix(href, "/user/") {
		return "", false
	}
	nick = squash(textOf(a))
	return nick, nick != ""
}
