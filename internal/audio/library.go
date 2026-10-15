package audio

import (
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
)

// tracks.toml is the credits data: one entry per track, with its license
// and where it is downloaded from. The files themselves are never in the
// repo or the binary.
//
//go:embed tracks.toml
var tracksTOML string

// Licenses that allow a free, non-commercial game to download a recording
// and play it in a loop, unmodified, with attribution. ND is excluded (it
// forbids sharing adaptations, and a loop edit would be one), as is
// anything without a license.
var accepted = map[string]bool{
	"CC0 1.0":         true,
	"CC BY 3.0":       true,
	"CC BY 4.0":       true,
	"CC BY-SA 3.0":    true,
	"CC BY-SA 4.0":    true,
	"CC BY-NC 3.0":    true,
	"CC BY-NC 4.0":    true,
	"CC BY-NC-SA 3.0": true,
	"CC BY-NC-SA 4.0": true,
	"permission":      true, // a written statement, quoted in license_quote
}

// LoadManifest parses and checks a manifest (tracks.toml's format).
func LoadManifest(data string) ([]Track, error) {
	var doc struct {
		Track []Track `toml:"track"`
	}
	if _, err := toml.Decode(data, &doc); err != nil {
		return nil, fmt.Errorf("tracks: %w", err)
	}
	seen := map[string]bool{}
	for _, t := range doc.Track {
		if err := t.check(); err != nil {
			return nil, fmt.Errorf("track %q: %w", t.ID, err)
		}
		if seen[t.ID] {
			return nil, fmt.Errorf("track %q: listed twice", t.ID)
		}
		seen[t.ID] = true
	}
	return doc.Track, nil
}

// Manifest is the embedded tracks.toml.
func Manifest() []Track {
	ts, err := LoadManifest(tracksTOML)
	if err != nil {
		panic(err) // the embedded manifest is tested
	}
	return ts
}

func (t Track) check() error {
	if t.ID == "" || strings.ContainsAny(t.ID, `/\. `) {
		return fmt.Errorf("id must be a plain name")
	}
	if t.Title == "" || t.Artist == "" {
		return fmt.Errorf("title and artist are required for the credit")
	}
	if t.Touhou && t.Original == "" {
		return fmt.Errorf("a Touhou arrangement must name its original")
	}
	if !accepted[t.License] {
		return fmt.Errorf("license %q is not one the game accepts", t.License)
	}
	if t.LicenseQuote == "" || t.Page == "" {
		return fmt.Errorf("license_quote and page are required: the license must be shown where it is stated")
	}
	if t.License != "permission" && !strings.HasPrefix(t.LicenseURL, "https://creativecommons.org/") {
		return fmt.Errorf("license_url %q is not a creativecommons.org deed", t.LicenseURL)
	}
	for _, s := range []string{t.URL, t.Page} {
		u, err := url.Parse(s)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("%q is not an https URL", s)
		}
	}
	if b, err := hex.DecodeString(t.SHA256); err != nil || len(b) != 32 {
		return fmt.Errorf("sha256 %q is not 64 hex digits", t.SHA256)
	}
	if t.Size <= 0 || t.Size > 64<<20 {
		return fmt.Errorf("size %d is outside 1 B..64 MiB", t.Size)
	}
	switch t.Format {
	case "mp3", "ogg", "wav":
	default:
		return fmt.Errorf("format %q is not mp3, ogg or wav", t.Format)
	}
	if len(t.Pools) == 0 {
		return fmt.Errorf("no pools")
	}
	for _, p := range t.Pools {
		if !isPool(p) {
			return fmt.Errorf("pool %q is not one of %v", p, Pools)
		}
	}
	if t.LoopStart < 0 || t.LoopEnd < 0 || t.LoopEnd > 0 && t.LoopEnd <= t.LoopStart {
		return fmt.Errorf("loop %v..%v", t.LoopStart, t.LoopEnd)
	}
	return nil
}

// Library finds the tracks a cue can play: downloaded manifest tracks and
// the player's own files.
type Library struct {
	Tracks   []Track
	CacheDir string // <CacheDir>/<id>.<format>, and loudness.json
	UserDir  string // <UserDir>/<pool or key>/*.{mp3,ogg,wav}
	Mode     string // "override" or "extend" (Settings.UserMusic)

	mu       sync.Mutex
	loudness map[string]measured // by loudnessKey; nil until loaded
}

// measured is a track's loudness, kept so each file is measured once.
type measured struct {
	LUFS float64 `json:"lufs"`
	Peak float64 `json:"peak"`
}

// DefaultLibrary is the embedded manifest over the standard directories:
// the user cache (~/Library/Caches/katharsis/music on macOS) and
// ~/.config/katharsis/music.
func DefaultLibrary(mode string) (*Library, error) {
	c, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	cfg, err := ConfigDir()
	if err != nil {
		return nil, err
	}
	return &Library{
		Tracks:   Manifest(),
		CacheDir: filepath.Join(c, "katharsis", "music"),
		UserDir:  filepath.Join(cfg, "music"),
		Mode:     mode,
	}, nil
}

// Path is where a manifest track's file is cached.
func (l *Library) Path(t Track) string {
	return filepath.Join(l.CacheDir, t.ID+"."+t.Format)
}

// Have reports whether a manifest track is downloaded. Fetch checks the
// hash before it renames a file into place, so the size is enough here.
func (l *Library) Have(t Track) bool {
	st, err := os.Stat(l.Path(t))
	return err == nil && st.Mode().IsRegular() && st.Size() == t.Size
}

// Resolve is what a cue can play, sorted by ID: the player's music/<key>/
// folder if it has files, else music/<pool>/, then the downloaded
// manifest tracks of the pool, which an "override" library drops when
// the player's folder had any.
func (l *Library) Resolve(pool, key string) []Track {
	if pool == "" {
		return nil
	}
	var user []Track
	if key != "" {
		user = l.userTracks(key)
	}
	if len(user) == 0 {
		user = l.userTracks(pool)
	}
	if len(user) > 0 && l.Mode != "extend" {
		return user
	}
	var out []Track
	for _, t := range l.Tracks {
		if slices.Contains(t.Pools, pool) && l.Have(t) {
			t.Path = l.Path(t)
			out = append(out, t)
		}
	}
	out = append(out, user...)
	slices.SortFunc(out, func(a, b Track) int { return strings.Compare(a.ID, b.ID) })
	return out
}

// userTracks lists a folder's audio files as tracks titled by file name.
func (l *Library) userTracks(name string) []Track {
	if l.UserDir == "" || name == "" || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return nil
	}
	dir := filepath.Join(l.UserDir, name)
	es, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Track
	for _, e := range es {
		if !e.Type().IsRegular() || !IsAudioName(e.Name()) || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		title := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		out = append(out, Track{
			ID:      "user:" + name + "/" + e.Name(),
			Title:   title,
			License: "your file",
			Pools:   []string{name},
			Path:    filepath.Join(dir, e.Name()),
		})
	}
	return out
}

func (l *Library) loudnessPath() string { return filepath.Join(l.CacheDir, "loudness.json") }

// loudnessKey names a file's measurement: the manifest hash, or a user
// file's path, size and modification time.
func (l *Library) loudnessKey(t Track) string {
	if t.SHA256 != "" {
		return t.ID + "@" + t.SHA256[:16]
	}
	st, err := os.Stat(t.Path)
	if err != nil {
		return ""
	}
	return t.Path + "@" + strconv.FormatInt(st.Size(), 10) + "@" + strconv.FormatInt(st.ModTime().UnixNano(), 10)
}

func (l *Library) loadLoudnessLocked() {
	if l.loudness != nil {
		return
	}
	l.loudness = map[string]measured{}
	if b, err := os.ReadFile(l.loudnessPath()); err == nil {
		_ = json.Unmarshal(b, &l.loudness)
	}
}

// Loudness is a track's measurement, if it has one.
func (l *Library) Loudness(t Track) (measured, bool) {
	k := l.loudnessKey(t)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.loadLoudnessLocked()
	m, ok := l.loudness[k]
	return m, ok && k != ""
}

// Measure decodes the whole of t and records its loudness (BS.1770
// integrated) and sample peak in loudness.json.
func (l *Library) Measure(t Track) (measured, error) {
	data, err := os.ReadFile(t.Path)
	if err != nil {
		return measured{}, err
	}
	src, err := decode(t.Path, data)
	if err != nil {
		return measured{}, err
	}
	m := NewMeter(src.Rate(), 2)
	buf := make([]float32, 8192)
	for {
		n, err := src.Read(buf)
		if n == 0 || err != nil {
			break
		}
		m.Add(buf[:n*2])
	}
	r := measured{LUFS: m.Integrated(), Peak: m.Peak()}
	if math.IsInf(r.LUFS, 0) || math.IsNaN(r.LUFS) {
		r.LUFS = -70
	}
	k := l.loudnessKey(t)
	l.mu.Lock()
	defer l.mu.Unlock()
	l.loadLoudnessLocked()
	if k != "" {
		l.loudness[k] = r
		if b, err := json.MarshalIndent(l.loudness, "", "  "); err == nil {
			_ = os.MkdirAll(l.CacheDir, 0o755)
			_ = writeAtomic(l.loudnessPath(), b)
		}
	}
	return r, nil
}

// gainFor brings a measured track to target LUFS, with at most +12 dB of
// boost so a near-silent file is not blown up.
func gainFor(m measured, target float64) float64 {
	return math.Min(dbToGain(target-m.LUFS), dbToGain(12))
}

// writeAtomic writes b to path through a temp file in the same directory
// and a rename, so a reader never sees half a file.
func writeAtomic(path string, b []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
