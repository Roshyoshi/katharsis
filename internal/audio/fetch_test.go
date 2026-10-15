package audio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestFetch(t *testing.T) {
	good := wav(Rate, sine(Rate, 440, 0.3, 1))
	sum := sha256.Sum256(good)
	var flaky atomic.Int32
	var agent atomic.Value
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent.Store(r.Header.Get("User-Agent"))
		switch r.URL.Path {
		case "/good.wav":
			_, _ = w.Write(good)
		case "/flaky.wav":
			if flaky.Add(1) == 1 {
				http.Error(w, "busy", http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write(good)
		case "/changed.wav":
			_, _ = w.Write(append([]byte{}, good[:len(good)-1]...)) // a byte short
		case "/tampered.wav":
			b := append([]byte{}, good...)
			b[len(b)-1] ^= 1
			_, _ = w.Write(b)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	track := func(id, path string) Track {
		return Track{ID: id, Title: id, Artist: "a", URL: srv.URL + path, SHA256: hex.EncodeToString(sum[:]),
			Size: int64(len(good)), Format: "wav", Pools: []string{PoolGrub}}
	}
	dir := t.TempDir()
	l := &Library{CacheDir: filepath.Join(dir, "music"), Tracks: []Track{
		track("good", "/good.wav"), track("flaky", "/flaky.wav"), track("changed", "/changed.wav"),
		track("tampered", "/tampered.wav"), track("gone", "/gone.wav"), {ID: "other", Pools: []string{PoolBoss}},
	}}
	var log strings.Builder
	r := Fetch(context.Background(), l, FetchOptions{Out: &log, Client: srv.Client(), Retries: 1, Only: []string{PoolGrub}})
	if r.Fetched != 2 || r.Failed != 3 || r.Had != 0 {
		t.Errorf("Fetch = %+v; want 2 fetched, 3 failed\n%s", r, log.String())
	}
	for _, id := range []string{"good", "flaky"} {
		tr := l.Tracks[0]
		tr.ID = id
		if !l.Have(tr) {
			t.Errorf("%s is not in the cache", id)
		}
		tr.Path = l.Path(tr)
		if m, ok := l.Loudness(tr); !ok || m.LUFS > -10 || m.LUFS < -20 {
			t.Errorf("%s loudness = %+v, %v; want about -13 LUFS (a 0.3 sine)", id, m, ok)
		}
	}
	es, _ := os.ReadDir(l.CacheDir)
	for _, e := range es {
		if strings.HasPrefix(e.Name(), partPrefix) || strings.HasPrefix(e.Name(), "changed") || strings.HasPrefix(e.Name(), "tampered") {
			t.Errorf("left behind %s", e.Name())
		}
	}
	if ua, _ := agent.Load().(string); !strings.HasPrefix(ua, "katharsis/") || strings.Contains(ua, "@") {
		t.Errorf("User-Agent %q", ua)
	}
	if n := flaky.Load(); n != 2 {
		t.Errorf("flaky was asked %d times, want 2 (one retry)", n)
	}
	// "tampered" failed on its hash, which no retry fixes.
	if !strings.Contains(log.String(), "tampered: failed: sha256") {
		t.Errorf("no hash failure in the log:\n%s", log.String())
	}

	// A second run downloads nothing it has.
	r = Fetch(context.Background(), l, FetchOptions{Client: srv.Client(), Only: []string{"good"}})
	if r.Had != 1 || r.Fetched != 0 {
		t.Errorf("second Fetch = %+v, want 1 already here", r)
	}

	// A cancelled fetch fails what is left, cleanly.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = Fetch(ctx, l, FetchOptions{Client: srv.Client(), Only: []string{"changed"}})
	if r.Failed != 1 {
		t.Errorf("cancelled Fetch = %+v", r)
	}
}

func TestCredits(t *testing.T) {
	l := library(t, map[string][]string{"a": {PoolGrub}}, nil)
	l.Tracks[0].Touhou, l.Tracks[0].Original = true, "ZUN, Touhou 6"
	l.Tracks[0].License, l.Tracks[0].LicenseURL = "CC BY 4.0", "https://creativecommons.org/licenses/by/4.0/"
	l.Tracks[0].Page, l.Tracks[0].LicenseQuote = "https://example.org/a", "Licensed CC BY 4.0"
	md := Credits(l, DefaultSettings())
	for _, want := range []string{"東方Project二次創作", "ZUN", "arranged by a (original: ZUN, Touhou 6)",
		"[CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)", "https://example.org/a",
		"Silence", "sfx.toml", "ctrl+t", "All 1 tracks are downloaded"} {
		if !strings.Contains(md, want) {
			t.Errorf("credits lack %q:\n%s", want, md)
		}
	}
}
