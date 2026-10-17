// Package cses serves the CSES Problem Set (cses.fi/problemset, about 400
// tasks) as a catalog.Catalog.
//
// The task list is public and so are the statements; the tests are not:
// CSES hands a task's test zip only to a logged-in user. So a task is
// Judgeable when it can be judged by comparing output (not interactive,
// one right answer) and either an account is set (Login) or its tests are
// already cached. Tests are downloaded on a task's first Fetch and kept
// for good.
//
// The index is the problem list (titles, sections, solver counts, which
// give the rating) plus each task's limits and judge facts, from
// facts.json until a Fetch reads the task's own page. Open reads the list
// from cses.fi when the cache has none or it is a week old, the latter in
// the background; Cached never touches the network:
//
//	<cache>/katharsis/cses/
//	  index.json                  list rows; facts read by fetches
//	  account.json                0600: nick and session cookie
//	  <id>/statement/             task.html, statement.md, samples/N.in,
//	                              N.out, meta.json written last
//	  <id>/tests/                 N.in, N.out from the zip; done.json last
//
// Each of the two dirs is built in a temp dir and renamed into place, so
// either is whole or absent. The password is kept in the macOS Keychain
// on darwin and nowhere else (see keyring); other systems keep only the
// session, and the player logs in again when it expires.
//
// Requests go one at a time, a second apart, for the whole process (see
// pacer); a Fetch makes three (page, tests form, zip) and two more when it
// must log in again.
package cses

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"katharsis/internal/catalog"
	"katharsis/internal/content/web"
)

// Catalog is the CSES Problem Set.
type Catalog struct {
	root string // <cache>/katharsis/cses
	base string // site; a test server in tests
	pace *pacer
	keys keyring

	mu      sync.RWMutex // guards everything below
	entries map[catalog.ID]*entry
	order   []catalog.ID // list order
	rows    []row
	listed  time.Time
	table   map[string]facts // facts.json
	// tableRows is facts.json whole; its rows are the list when Open has
	// no cache and no network.
	tableRows []factsRow
	verified  map[string]facts // read by fetches, persisted in index.json
	hasTests  map[catalog.ID]bool
	acct      *account
	jar       http.CookieJar // the session's; nil without an account
	authGen   int            // bumped by every login and logout

	authMu  sync.Mutex // orders Login, Logout and relogin
	saveMu  sync.Mutex // orders index.json writers
	flights web.Flights[catalog.ID, catalog.Assets]
	lists   web.Flights[string, struct{}]

	// refreshed is closed when the background refresh Open started has
	// ended; nil when it started none.
	refreshed chan struct{}
}

var _ catalog.Catalog = (*Catalog)(nil)

const site = "https://cses.fi"

// StaleAfter is the age at which Open reads the list again (in the
// background), for new tasks and solver counts.
const StaleAfter = 7 * 24 * time.Hour

// listTimeout bounds the list read Open waits for, made only when the
// cache has no index.
const listTimeout = 20 * time.Second

// Open loads the catalog from the cache. With no cached index it reads the
// problem list from cses.fi (one request, at most 20 s), falling back to
// the list in facts.json when that fails, so it does not fail offline. A
// cached list older than StaleAfter is served at once and read again in
// the background.
func Open(ctx context.Context) (*Catalog, error) {
	root, err := cacheRoot()
	if err != nil {
		return nil, err
	}
	return open(ctx, root, site, pace, systemKeyring())
}

// Cached is the catalog from the cached index alone, with no network, or
// ErrNoIndex.
func Cached() (*Catalog, error) {
	root, err := cacheRoot()
	if err != nil {
		return nil, err
	}
	return cached(root, site, pace, systemKeyring())
}

func cacheRoot() (string, error) {
	c, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(c, "katharsis", "cses"), nil
}

func open(ctx context.Context, root, base string, p *pacer, keys keyring) (*Catalog, error) {
	c, err := newCatalog(root, base, p, keys)
	if err != nil {
		return nil, err
	}
	idx, ok := loadIndex(c.indexPath())
	if ok {
		c.load(idx)
		if time.Since(idx.Listed) > StaleAfter {
			c.refreshed = make(chan struct{})
			go func() {
				defer close(c.refreshed)
				rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
				defer cancel()
				_ = c.refresh(rctx)
			}()
		}
		return c, nil
	}
	lctx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()
	if err := c.refresh(lctx); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Offline: facts.json's list, not saved, so the next Open asks
		// cses.fi again.
		c.load(&index{Rows: tableRows(c.tableRows)})
	}
	return c, nil
}

func cached(root, base string, p *pacer, keys keyring) (*Catalog, error) {
	c, err := newCatalog(root, base, p, keys)
	if err != nil {
		return nil, err
	}
	idx, ok := loadIndex(c.indexPath())
	if !ok {
		return nil, ErrNoIndex
	}
	c.load(idx)
	return c, nil
}

func newCatalog(root, base string, p *pacer, keys keyring) (*Catalog, error) {
	web.CleanDebris(root, web.TempPrefix)
	t, err := loadFactsTable()
	if err != nil {
		return nil, err
	}
	c := &Catalog{
		root: root, base: base, pace: p, keys: keys,
		entries:   map[catalog.ID]*entry{},
		table:     tableFacts(t),
		tableRows: t,
		verified:  map[string]facts{},
		hasTests:  map[catalog.ID]bool{},
	}
	if a, ok := loadAccount(c.accountPath()); ok {
		c.acct, c.jar = a, c.newJar(a.Session)
	}
	c.scanTests()
	return c, nil
}

func (c *Catalog) indexPath() string { return filepath.Join(c.root, "index.json") }

// scanTests notes which tasks have tests cached: a tests dir with its
// done.json. Fetch checks the files themselves before serving them.
func (c *Catalog) scanTests() {
	des, _ := os.ReadDir(c.root)
	for _, d := range des {
		if !d.IsDir() || !validID(d.Name()) {
			continue
		}
		if _, err := os.Stat(filepath.Join(c.root, d.Name(), "tests", "done.json")); err == nil {
			c.hasTests[catalog.NewID(catalog.CSES, d.Name())] = true
		}
	}
}

// load makes idx the catalog's list and derives the entries from it.
func (c *Catalog) load(idx *index) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rows, c.listed = idx.Rows, idx.Listed
	for id, f := range idx.Verified {
		c.verified[id] = f
	}
	c.deriveLocked()
}

// deriveLocked rebuilds entries and order from rows. mu is held.
func (c *Catalog) deriveLocked() {
	es := derive(c.rows, c.table, c.verified)
	c.entries = make(map[catalog.ID]*entry, len(es))
	c.order = c.order[:0]
	for i := range es {
		c.entries[es[i].ID] = &es[i]
		c.order = append(c.order, es[i].ID)
	}
}

// refresh reads the problem list from cses.fi, makes it the catalog's and
// saves the index. Concurrent calls share one read.
func (c *Catalog) refresh(ctx context.Context) error {
	_, err := c.lists.Do(ctx, "list", func() (struct{}, error) {
		resp, err := c.pace.do(ctx, request{url: c.base + "/problemset/list/"})
		if err != nil {
			return struct{}{}, err
		}
		if resp.status != http.StatusOK {
			return struct{}{}, errors.New("CSES problem list: " + http.StatusText(resp.status))
		}
		rows, err := parseList(resp.body)
		if err != nil {
			return struct{}{}, err
		}
		c.mu.Lock()
		c.rows, c.listed = rows, time.Now().UTC()
		c.deriveLocked()
		c.mu.Unlock()
		return struct{}{}, c.persist()
	})
	return err
}

// persist writes index.json from memory. saveMu orders the writers and
// each snapshots under it, so the last rename carries every update.
func (c *Catalog) persist() error {
	c.saveMu.Lock()
	defer c.saveMu.Unlock()
	c.mu.RLock()
	idx := &index{Key: indexKey, Listed: c.listed, Rows: slices.Clone(c.rows), Verified: map[string]facts{}}
	for id, f := range c.verified {
		idx.Verified[id] = f
	}
	c.mu.RUnlock()
	if err := os.MkdirAll(c.root, 0o755); err != nil {
		return err
	}
	return saveIndex(c.indexPath(), idx)
}

// All lists every task in list order, judgeable or not.
func (c *Catalog) All() []catalog.Problem {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]catalog.Problem, 0, len(c.order))
	for _, id := range c.order {
		out = append(out, c.problemLocked(c.entries[id]))
	}
	return out
}

// Get is the task id, with Judgeable as of now: a Login or Logout shows at
// once.
func (c *Catalog) Get(id catalog.ID) (catalog.Problem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[id]
	if !ok {
		return catalog.Problem{}, false
	}
	return c.problemLocked(e), true
}

// problemLocked copies e out, Tags included, and decides Judgeable. mu is
// held.
func (c *Catalog) problemLocked(e *entry) catalog.Problem {
	p := e.Problem
	p.Tags = slices.Clone(p.Tags)
	p.Judgeable = e.Playable && (c.acct != nil || c.hasTests[e.ID])
	return p
}

func (c *Catalog) entry(id catalog.ID) (entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[id]
	if !ok {
		return entry{}, false
	}
	return *e, true
}

// verify records the facts a fetch read from id's page, rederives the
// entries and persists the index when they changed, so a task that turns
// out to need a checker leaves door play at once and stays out.
func (c *Catalog) verify(native string, f facts) error {
	c.mu.Lock()
	if old, ok := c.verified[native]; ok && old == f {
		c.mu.Unlock()
		return nil
	}
	c.verified[native] = f
	c.deriveLocked()
	c.mu.Unlock()
	return c.persist()
}
