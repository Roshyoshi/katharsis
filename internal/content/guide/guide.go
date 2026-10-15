// Package guide keeps a pinned, read-only snapshot of the USACO Guide
// repository (github.com/cpinitiative/usaco-guide, CC BY-NC-SA 4.0) in the
// user cache. The USACO catalog reads its problem lists from it and learn
// mode reads its modules, so both see the same commit. Both also read
// ModuleTopic, the one table of which topic each module teaches.
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
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"katharsis/internal/content/web"
)

// Commit is the usaco-guide master commit the game is built against
// (looked up 2026-10-14). Bumping it re-downloads the snapshot, removes
// the old one and rebuilds every index derived from it.
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
)

const tarballURL = "https://codeload.github.com/cpinitiative/usaco-guide/tar.gz/" + Commit

var (
	// client has no overall timeout: the tarball takes minutes on a slow
	// line, and web's idle cut ends an attempt that stalls.
	client = &web.Client{Retries: 2, Backoff: 2 * time.Second}

	mu sync.Mutex // serializes snapshots within the process
)

// Progress, when set, is told the bytes received so far as a download
// streams in, for a caller that shows it (the game, before its TUI).
var Progress func(received int64)

// Snapshot returns the directory holding the pinned snapshot, downloading
// and extracting it on first use. The directory appears atomically (extract
// to a temp dir, then rename), so a partial download is never visible and
// concurrent callers, in this process or another, all get the same dir.
func Snapshot(ctx context.Context) (string, error) {
	root, err := cacheRoot()
	if err != nil {
		return "", err
	}
	return snapshot(ctx, client, root, tarballURL, Commit)
}

func cacheRoot() (string, error) {
	c, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(c, "katharsis", "guide"), nil
}

func snapshot(ctx context.Context, c *web.Client, root, url, commit string) (string, error) {
	// A download killed part way leaves a temp dir of up to maxExtracted.
	// Cleaned on every call, not just before a download: after a kill the
	// next start usually downloads again at once, while the debris is too
	// young to remove, and from then on the snapshot is there.
	web.CleanDebris(root, web.TempPrefix)
	removeOtherCommits(root, commit)
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
	tmp, err := os.MkdirTemp(root, web.TempPrefix+commit+"-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp) // no-op after a successful rename

	prefix := "usaco-guide-" + commit + "/"
	err = c.Do(ctx, web.Request{URL: url, Progress: Progress, Read: func(resp *http.Response) error {
		// The archive is streamed, so there is nothing to resume: each
		// attempt starts over in an emptied dir.
		if err := emptyDir(tmp); err != nil {
			return web.Permanent(err)
		}
		// A cut connection shows up as a gzip or tar error, worth a
		// retry; a bad archive will be just as bad next time.
		err := extract(io.LimitReader(resp.Body, maxDownload), prefix, tmp)
		if errors.Is(err, errBadArchive) {
			return web.Permanent(err)
		}
		return err
	}})
	if err != nil {
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

// removeOtherCommits deletes the snapshots of commits other than commit: a
// Commit bump leaves the old one behind, and nothing else reads it. Only
// dirs named like a commit go. A failed removal is tried again next time.
func removeOtherCommits(root, commit string) {
	des, _ := os.ReadDir(root)
	for _, d := range des {
		if name := d.Name(); d.IsDir() && name != commit && isCommit(name) {
			_ = os.RemoveAll(filepath.Join(root, name))
		}
	}
}

// isCommit is true for a full lowercase hex SHA-1, as Commit is written.
func isCommit(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range []byte(s) {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
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
	if err != nil {
		return err
	}
	for _, e := range ents {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}
