package audio

import (
	"math/rand/v2"
	"os"
	"slices"
	"sync"
	"time"
)

// Engine is the Sink that plays: effects go straight into the mixer, and
// a goroutine of its own turns cues into tracks (file reads, decoding,
// loudness) so the UI never waits on a disk.
type Engine struct {
	set    Settings
	lib    *Library
	mix    *Mixer
	sounds map[string]Sound
	ending chan uint64
	kick   chan struct{}
	cmds   chan func() // run on the engine goroutine (Audition)
	done   chan struct{}
	wg     sync.WaitGroup

	mu  sync.Mutex
	out output // nil until the device is open, and in tests
	// ready is false while the device opens: effects then are dropped
	// rather than played late in a burst. An engine whose device failed
	// stays unready, and silent, for good.
	ready   bool
	dead    bool // no device: cues are ignored too
	want    Cue
	musicOn bool
	now     Track
	serial  uint64

	// Engine goroutine only.
	cue      Cue
	lead     string // the cue key whose lead track plays next, once
	playing  *musicVoice
	track    Track
	tracks   []Track
	wasOn    bool
	bags     map[string]*bag
	bad      map[string]bool // files that would not decode
	nextID   uint64
	rng      *rand.Rand
	cueEpoch int // bumps on every cue reconcile, for tests
}

// New starts an engine on the default output device, opened in the
// background. Without a device it stays silent and the game plays on.
func New(set Settings, lib *Library) *Engine {
	e := newEngine(set, lib, DefaultEffects())
	e.ready = false
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		o, err := openOto(e.mix, set.DeviceBuffer.Duration)
		if err != nil {
			e.mu.Lock()
			e.dead = true
			e.mu.Unlock()
			e.mix.stopAll()
			return
		}
		e.mu.Lock()
		e.out, e.ready = o, true
		e.mu.Unlock()
	}()
	return e
}

// newEngine builds an engine with no output: tests render its mixer by
// hand, so nothing is ever played.
func newEngine(set Settings, lib *Library, fx map[string]Effect) *Engine {
	e := &Engine{
		set:     set,
		lib:     lib,
		sounds:  Render(fx, set.SFXLUFS),
		ending:  make(chan uint64, 8),
		kick:    make(chan struct{}, 1),
		cmds:    make(chan func()),
		done:    make(chan struct{}),
		musicOn: set.Music,
		ready:   true,
		bags:    map[string]*bag{},
		bad:     map[string]bool{},
		rng:     rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
	}
	e.mix = NewMixer(set, e.ending)
	e.wg.Add(1)
	go e.run()
	return e
}

// Close stops the engine after fading the music out.
func (e *Engine) Close() {
	e.mix.stopMusic(0.25)
	if e.mix.busy() && e.live() {
		time.Sleep(300 * time.Millisecond)
	}
	close(e.done)
}

func (e *Engine) live() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.ready && e.out != nil
}

func (e *Engine) Play(name string) {
	if !e.set.SFX {
		return
	}
	s, ok := e.sounds[name]
	if !ok {
		return
	}
	e.mu.Lock()
	out, ready := e.out, e.ready
	e.mu.Unlock()
	if !ready {
		return
	}
	if out != nil {
		out.Resume()
	}
	e.mix.playSFX(s)
}

func (e *Engine) Cue(c Cue) {
	e.mu.Lock()
	changed := c != e.want
	e.want = c
	e.mu.Unlock()
	if changed {
		e.poke()
	}
}

func (e *Engine) Duck(on bool) { e.mix.setDuck(on) }

func (e *Engine) ToggleMusic() bool {
	e.mu.Lock()
	e.musicOn = !e.musicOn
	on := e.musicOn
	e.mu.Unlock()
	e.poke()
	return on
}

func (e *Engine) NowPlaying() (Track, uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.now, e.serial
}

func (e *Engine) poke() {
	select {
	case e.kick <- struct{}{}:
	default:
	}
}

func (e *Engine) run() {
	defer e.wg.Done()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-e.done:
			return
		case f := <-e.cmds:
			f()
		case <-e.kick:
			e.reconcile()
		case id := <-e.ending:
			if e.playing != nil && id == e.playing.id {
				e.advance(e.set.TrackFade.Seconds())
			}
		case <-tick.C:
			e.housekeep()
		}
	}
}

// reconcile moves the music to what the latest cue and toggle ask for.
func (e *Engine) reconcile() {
	e.mu.Lock()
	want, on, dead := e.want, e.musicOn, e.dead
	e.mu.Unlock()
	if dead {
		return
	}
	e.cueEpoch++
	if want.Pool == PoolPanic {
		want.Pool = e.set.Panic
	}
	if !on {
		if e.playing != nil {
			e.mix.stopMusic(0.5)
			e.playing = nil
		}
		e.cue, e.wasOn = want, false
		return
	}
	resumed := !e.wasOn
	e.wasOn = true
	tracks := e.playable(want)
	// Same pool, same scene, and the track still belongs: carry on. Only
	// the key moved (a topic chosen) without bringing folders of its own.
	if !resumed && e.playing != nil && want.Pool == e.cue.Pool && want.Scene == e.cue.Scene &&
		slices.ContainsFunc(tracks, func(t Track) bool { return t.ID == e.track.ID }) {
		e.cue, e.tracks = want, tracks
		return
	}
	e.cue, e.tracks, e.lead = want, tracks, want.Key
	e.advance(e.set.Crossfade.Seconds())
}

// playable is what a cue can play, less files that failed to decode.
func (e *Engine) playable(c Cue) []Track {
	if e.lib == nil {
		return nil
	}
	ts := e.lib.Resolve(c.Pool, c.Key)
	return slices.DeleteFunc(ts, func(t Track) bool { return e.bad[t.Path] })
}

// advance crossfades to the pool's next track over fade seconds, or fades
// to silence when the pool has none.
func (e *Engine) advance(fade float64) {
	for len(e.tracks) > 0 {
		ids := make([]string, len(e.tracks))
		for i, t := range e.tracks {
			ids[i] = t.ID
		}
		k := e.cue.Pool + "/" + e.cue.Key
		pool := e.bag(k)
		var id string
		// A cue with a key (a boss) opens with one of the tracks that lead
		// for it, then moves through its whole pool.
		if lead := e.leads(); len(lead) > 0 {
			id = e.bag(k+"/lead").next(lead, e.rng)
			pool.avoid(id, e.rng)
		} else {
			id = pool.next(ids, e.rng)
		}
		i := slices.IndexFunc(e.tracks, func(t Track) bool { return t.ID == id })
		t := e.tracks[i]
		passes := e.set.Repeats
		if t.Once {
			passes = 1
		}
		if len(e.tracks) == 1 {
			passes = 0 // alone in its pool: loop it
		}
		v, err := e.open(t, passes)
		if err != nil {
			e.bad[t.Path] = true
			e.tracks = slices.Delete(e.tracks, i, i+1)
			continue
		}
		e.mu.Lock()
		out := e.out
		e.now, e.serial = t, e.serial+1
		e.mu.Unlock()
		if out != nil {
			out.Resume()
		}
		e.mix.startMusic(v, fade, fade)
		e.playing, e.track = v, t
		return
	}
	if e.playing != nil {
		e.mix.stopMusic(fade)
		e.playing = nil
	}
}

func (e *Engine) open(t Track, passes int) (*musicVoice, error) {
	data, err := os.ReadFile(t.Path)
	if err != nil {
		return nil, err
	}
	src, err := decode(t.Path, data)
	if err != nil {
		return nil, err
	}
	m, ok := e.lib.Loudness(t)
	if !ok {
		// A file nobody measured yet (the player's own): measure it once,
		// here off the UI goroutine, and keep the number.
		if m, err = e.lib.Measure(t); err != nil {
			return nil, err
		}
	}
	e.nextID++
	return &musicVoice{
		id:     e.nextID,
		st:     newStream(src, t.LoopStart, t.LoopEnd, passes),
		gain:   gainFor(m, e.set.MusicLUFS),
		fadeAt: int64(e.set.TrackFade.Seconds() * Rate),
	}, nil
}

// housekeep suspends an idle output and restarts music that ended
// without the engine hearing of it (a dropped ending signal).
func (e *Engine) housekeep() {
	e.mu.Lock()
	out := e.out
	e.mu.Unlock()
	if out != nil && e.mix.idleFrames() >= int(e.set.IdleSuspend.Seconds()*Rate) {
		out.Suspend()
	}
	if e.playing != nil && !e.mix.hasMusic(e.playing.id) {
		e.advance(e.set.TrackFade.Seconds())
	}
}

// Audition plays t at once, looping, from `from` into it, or from before
// its loop point when negative, so the seam can be heard. It is for
// cmd/demo-audio; the next Cue takes over again.
func (e *Engine) Audition(t Track, from time.Duration) error {
	errc := make(chan error, 1)
	e.cmds <- func() {
		v, err := e.open(t, 0)
		if err != nil {
			errc <- err
			return
		}
		f := int64(from.Seconds() * float64(v.st.src.Rate()))
		if from < 0 {
			f += v.st.loopEnd
		}
		if err := v.st.seek(max(f, 0)); err != nil {
			errc <- err
			return
		}
		e.mu.Lock()
		out := e.out
		e.now, e.serial = t, e.serial+1
		e.mu.Unlock()
		if out != nil {
			out.Resume()
		}
		e.mix.startMusic(v, 0.05, 0.3)
		e.playing, e.track, e.tracks = v, t, []Track{t}
		e.cue = Cue{Pool: "audition", Key: t.ID}
		errc <- nil
	}
	return <-errc
}

// leads are the tracks that lead for the cue's key, the first time
// advance runs after the cue changed; nil after that or without any.
func (e *Engine) leads() []string {
	key := e.lead
	e.lead = ""
	if key == "" {
		return nil
	}
	var ids []string
	for _, t := range e.tracks {
		if slices.Contains(t.Keys, key) {
			ids = append(ids, t.ID)
		}
	}
	return ids
}

func (e *Engine) bag(k string) *bag {
	if e.bags[k] == nil {
		e.bags[k] = &bag{}
	}
	return e.bags[k]
}
