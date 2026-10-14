// Package guide keeps a pinned, read-only snapshot of the USACO Guide
// repository (github.com/cpinitiative/usaco-guide, CC BY-NC-SA 4.0) in the
// user cache. The USACO catalog reads its problem lists from it and learn
// mode reads its modules, so both see the same commit.
//
// Layout under Snapshot's dir mirrors the repository:
//
//	LICENSE
//	content/ordering.ts, content/**/*.mdx, content/**/*.problems.json, ...
//	solutions/**
//	src/components/markdown/ProblemsList/DivisionList/*.json
package guide

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Commit is the usaco-guide master commit the game is built against
// (looked up 2026-10-14). Bumping it re-downloads the snapshot and
// rebuilds every index derived from it.
const Commit = "092bf547cf4f63140057b220dba2861f884e1905"

// DivisionList is where the guide keeps its USACO contest tables
// (div_to_probs.json, id_to_sol.json, contest_to_points.json), relative to
// the snapshot dir.
const DivisionList = "src/components/markdown/ProblemsList/DivisionList"

const (
	// The codeload tarball is ~150 MB, almost all of it images and PDFs
	// under public/ that are skipped while streaming. The caps only stop a
	// hostile or broken response from filling the disk.
	maxDownload  = 1 << 30
	maxFile      = 16 << 20
	maxExtracted = 256 << 20

	userAgent = "deadcases/0.1 (terminal competitive-programming trainer)"
)

var (
	// tarballURL is a var so tests can point it at an httptest server.
	tarballURL = "https://codeload.github.com/cpinitiative/usaco-guide/tar.gz/" + Commit

	retryDelay = 2 * time.Second

	mu sync.Mutex // serializes snapshots within the process
)

// Snapshot returns the directory holding the pinned snapshot, downloading
// and extracting it on first use. The directory appears atomically (extract
// to a temp dir, then rename), so a partial download is never visible and
// concurrent callers, in this process or another, all get the same dir.
func Snapshot(ctx context.Context) (string, error) {
	root, err := cacheRoot()
	if err != nil {
		return "", err
	}
	return snapshot(ctx, root, tarballURL, Commit)
}

func cacheRoot() (string, error) {
	c, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(c, "deadcases", "guide"), nil
}

func snapshot(ctx context.Context, root, url, commit string) (string, error) {
	dir := filepath.Join(root, commit)
	if isDir(dir) {
		return dir, nil
	}
	mu.Lock()
	defer mu.Unlock()
	if isDir(dir) { // another goroutine finished while we waited
		return dir, nil
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(root, ".tmp-"+commit+"-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp) // no-op after a successful rename

	if err := download(ctx, url, "usaco-guide-"+commit+"/", tmp); err != nil {
		return "", fmt.Errorf("usaco guide snapshot %.12s: %w", commit, err)
	}
	if err := os.Rename(tmp, dir); err != nil {
		// Another process renamed its copy first; theirs is as good.
		if isDir(dir) {
			return dir, nil
		}
		return "", err
	}
	return dir, nil
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// download fetches the tarball with a few retries. Each attempt starts over
// in an emptied dst: the archive is streamed, so there is nothing to resume.
func download(ctx context.Context, url, prefix, dst string) error {
	var err error
	for attempt := range 3 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * retryDelay):
			}
			if err := emptyDir(dst); err != nil {
				return err
			}
		}
		var retry bool
		retry, err = fetchOnce(ctx, url, prefix, dst)
		if err == nil || !retry || ctx.Err() != nil {
			return err
		}
	}
	return err
}

func fetchOnce(ctx context.Context, url, prefix, dst string) (retry bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return true, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests,
			fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	if err := extract(io.LimitReader(resp.Body, maxDownload), prefix, dst); err != nil {
		// A cut connection shows up as a gzip or tar error; worth a retry.
		// A bad archive will be just as bad next time.
		return !errors.Is(err, errBadArchive), err
	}
	return false, nil
}

var errBadArchive = errors.New("bad archive")

// extract writes the wanted regular files of a gzipped tar into dst. Every
// entry must sit under prefix (the "<repo>-<sha>/" dir codeload adds), which
// also proves the tarball is for the commit asked for. Links, devices and
// anything that would land outside dst are refused, not skipped: a tarball
// from codeload never has them, so seeing one means the data is not what
// we think it is.
func extract(r io.Reader, prefix, dst string) error {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	tr := tar.NewReader(zr)
	var total int64
	files := 0
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeXGlobalHeader, tar.TypeDir:
			continue
		case tar.TypeReg:
		default:
			return fmt.Errorf("%w: %s has type %q", errBadArchive, h.Name, h.Typeflag)
		}
		rel, ok := strings.CutPrefix(h.Name, prefix)
		if !ok {
			return fmt.Errorf("%w: %s is outside %s", errBadArchive, h.Name, prefix)
		}
		if !wanted(rel) {
			continue
		}
		if !filepath.IsLocal(rel) || strings.Contains(rel, `\`) {
			return fmt.Errorf("%w: path %q", errBadArchive, h.Name)
		}
		if h.Size > maxFile {
			return fmt.Errorf("%w: %s is %d bytes", errBadArchive, h.Name, h.Size)
		}
		if total += h.Size; total > maxExtracted {
			return fmt.Errorf("%w: over %d bytes extracted", errBadArchive, maxExtracted)
		}
		if err := writeFile(filepath.Join(dst, filepath.FromSlash(rel)), tr, h.Size); err != nil {
			return err
		}
		files++
	}
	if files == 0 {
		return fmt.Errorf("%w: no guide files under %s", errBadArchive, prefix)
	}
	// The catalog cannot work without the contest tables; fail here rather
	// than publish a snapshot that every later reader trips over.
	for _, name := range []string{"div_to_probs.json", "id_to_sol.json"} {
		if _, err := os.Stat(filepath.Join(dst, DivisionList, name)); err != nil {
			return fmt.Errorf("%w: missing %s/%s", errBadArchive, DivisionList, name)
		}
	}
	return nil
}

// wanted keeps the text the game reads and drops the site's code, images
// and PDFs, which are most of the tarball.
func wanted(rel string) bool {
	switch {
	case rel == "LICENSE":
		return true
	case strings.HasPrefix(rel, "content/"), strings.HasPrefix(rel, "solutions/"):
		return true
	case path.Dir(rel) == DivisionList && path.Ext(rel) == ".json":
		return true
	}
	return false
}

func writeFile(p string, r io.Reader, size int64) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(r, size))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n != size {
		err = io.ErrUnexpectedEOF
	}
	return err
}

func emptyDir(dir string) error {
	ents, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	for _, e := range ents {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
