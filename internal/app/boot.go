package app

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	bootui "katharsis/internal/screens/boot"
	"katharsis/internal/session"
)

// baseScreen is the screen for the boot's phase, or GRUB outside a boot.
func (m *Model) baseScreen() screens.Screen {
	if m.state == nil {
		return m.grub()
	}
	env := m.env()
	switch m.state.Phase {
	case game.PhaseRoute:
		return bootui.NewRoute(env)
	case game.PhaseDoors:
		// The engine refuses doors until a pending upgrade is picked.
		if m.state.PendingUpgrade {
			return bootui.NewUpgradeScreen(env)
		}
		return bootui.NewDoors(env)
	case game.PhaseMerge:
		return bootui.NewMerge(env)
	case game.PhaseProblem:
		return newProblemScreen(m)
	case game.PhaseCrashed:
		return bootui.NewCrash(env)
	case game.PhaseCleared:
		return bootui.NewCleared(env)
	}
	return m.grub()
}

// baseKey is what baseScreen chooses by; when it changes, the stack is
// rebuilt.
type baseKey struct {
	phase   game.Phase
	upgrade bool
}

func (m *Model) baseKey() baseKey {
	return baseKey{m.state.Phase, m.state.Phase == game.PhaseDoors && m.state.PendingUpgrade}
}

func (m *Model) env() bootui.Env {
	return bootui.Env{State: m.state, Meta: m.meta, Lookup: m.d.Catalog.Get, Now: m.d.Now()}
}

// startBoot begins a new boot. A boot still running is given up first.
func (m *Model) startBoot(b screens.StartBoot) tea.Cmd {
	if err := m.abandon(); err != nil {
		return fail("abandon boot %d: %v", m.state.Boot, err)
	}
	r, ok := m.d.Presets[b.Preset]
	if !ok {
		r = m.d.Presets["standard"]
	}
	seed := b.Seed
	if seed == 0 {
		seed = m.d.Seed()
	}
	// A fetch outside the doors (a statement, say) may have found problems
	// unjudgeable since the pool was last read.
	m.pool = m.d.Catalog.All()
	st, effs, err := game.NewBoot(r, &m.meta, m.pool, b.Prestige, seed, m.d.Now())
	if err != nil {
		return fail("new boot: %v", err)
	}
	return m.begin(st, effs)
}

// begin shows a boot or trial that NewBoot or NewTrial just made. It is
// saved before the first step, so quitting right away still resumes; a
// failed save is shown, and the boot goes on, as after a step (try).
func (m *Model) begin(st *game.State, effs []game.Effect) tea.Cmd {
	m.state = st
	m.reset()
	m.saved(m.d.Store.Checkpoint(m.meta, m.state))
	return tea.Batch(m.initTop(), m.effects(effs), m.offerLink())
}

// saved records how a save went and reports whether it worked. A failure
// stays on screen (View) until a later save succeeds.
func (m *Model) saved(err error) bool {
	m.saveErr = err
	return err == nil
}

// abandon gives up a boot still running, so leaving one costs what quitting
// a door costs. In a door that is :q!; elsewhere there is no engine action
// for it, so the store just marks the boot abandoned.
func (m *Model) abandon() error {
	switch {
	case m.state == nil || m.state.Phase == game.PhaseCrashed || m.state.Phase == game.PhaseCleared:
		return nil
	case m.state.Phase == game.PhaseProblem:
		if _, err := game.Step(m.state, &m.meta, m.pool, game.GiveUp{}, m.d.Now()); err != nil {
			return err
		}
		m.closeDoor()
		err := m.d.Store.Checkpoint(m.meta, m.state)
		m.saved(err)
		return err
	}
	err := m.d.Store.AbandonBoot(m.state.Boot)
	m.saved(err)
	return err
}

// step feeds one action to the engine, saves, and applies the effects. A
// refused action leaves the state untouched and is shown, not fatal.
func (m *Model) step(a game.Action) tea.Cmd {
	cmd, err := m.try(a)
	if err != nil {
		return func() tea.Msg { return errMsg{err} }
	}
	return cmd
}

// try is step for a caller that reports a refusal itself: the editor's
// commands answer in vim, where the player is looking, not over the game.
func (m *Model) try(a game.Action) (tea.Cmd, error) {
	if m.state == nil {
		return nil, nil
	}
	before := m.baseKey()
	effs, err := game.Step(m.state, &m.meta, m.pool, a, m.d.Now())
	if errors.Is(err, game.ErrInvalid) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("engine: %w", err)
	}
	// The step happened whether or not it was saved, so its effects run
	// either way: a door the engine opened must open on screen too.
	m.saved(m.d.Store.Checkpoint(m.meta, m.state))
	cmds := []tea.Cmd{m.effects(effs)}
	if m.baseKey() != before {
		m.reset()
		cmds = append(cmds, m.initTop())
	} else {
		m.refresh()
	}
	return tea.Batch(cmds...), nil
}

// refusal is an engine refusal as the player reads it, without the
// "invalid action" prefix.
func refusal(err error) string {
	return strings.TrimPrefix(err.Error(), game.ErrInvalid.Error()+": ")
}

// refresh hands the screens the new state and clock, keeping their cursors.
func (m *Model) refresh() {
	if m.state == nil {
		return
	}
	env := m.env()
	for _, s := range m.stack {
		if r, ok := s.(interface{ Refresh(bootui.Env) }); ok {
			r.Refresh(env)
		}
	}
}

func (m *Model) onTick() tea.Cmd {
	if m.state == nil {
		return nil
	}
	// No Tick while a submit is being judged: the engine grants a grace
	// period for submits past the deadline, but not for Tick. The grace
	// also bounds the wait, so a judge that never reports back (it failed,
	// or the editor closed and took the HUD's pass with it) cannot stop
	// the clock.
	if m.state.Phase == game.PhaseProblem && (!m.judging || m.pastGrace()) {
		return m.step(game.Tick{})
	}
	m.refresh()
	return nil
}

// pastGrace reports whether the open door's deadline plus the submit grace
// has passed, after which no judge result can save it.
func (m *Model) pastGrace() bool {
	c := m.state.Current
	return c == nil || !m.d.Now().Before(c.Deadline.Add(m.state.Rules.SubmitGrace.Duration))
}

// effects carries out what the engine asked for. RevealTags, ShowCrash and
// ShowCleared need nothing beyond the screen rebuild: the state carries
// them. Nor does OpenEditorial, which comes with a :q! crash: the kernel
// panic lands first, and its screen offers the editorial on [e].
func (m *Model) effects(effs []game.Effect) tea.Cmd {
	var cmds []tea.Cmd
	for _, e := range effs {
		switch e := e.(type) {
		case game.OpenProblem:
			cmds = append(cmds, m.openDoor(e.Door, false))
		case game.CloseProblem:
			m.closeDoor()
		case game.Sound:
			if m.d.Sound != nil {
				m.d.Sound(e.Name)
			}
		case game.Prefetch:
			cmds = append(cmds, m.prefetch(e.Problems))
		}
	}
	return tea.Batch(cmds...)
}

type fetchedMsg struct {
	door    game.Door
	upsolve bool
	assets  catalog.Assets
	err     error
}

// openDoor fetches the problem (usually already prefetched) and hands it to
// the runner once it arrives.
func (m *Model) openDoor(d game.Door, upsolve bool) tea.Cmd {
	cat := m.d.Catalog
	return m.background(func(ctx context.Context) tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		a, err := cat.Fetch(ctx, d.Problem)
		return fetchedMsg{door: d, upsolve: upsolve, assets: a, err: err}
	})
}

func (m *Model) onFetched(f fetchedMsg) tea.Cmd {
	m.recheck(f.door.Problem)
	if !m.awaits(f) {
		return nil
	}
	if f.err != nil {
		return fail("fetch %s: %v", f.door.Problem, f.err)
	}
	p, _ := m.d.Catalog.Get(f.door.Problem)
	d := &Door{Door: f.door, Problem: p, Assets: f.assets, Upsolve: f.upsolve, Opened: m.d.Now()}
	if f.upsolve {
		d.ReadEditorial = m.sawEditorial(f.door.Problem)
		d.Sub, d.SeedBoot = m.upsolveFiles(f.door.Problem)
	} else {
		d.Boot = m.state.Boot
		d.Libs = game.LinkedLibs(m.state, &m.meta)
	}
	// A door that failed to open stays unopened, so e can retry it.
	if err := m.d.Runner.Open(context.Background(), *d); err != nil {
		return fail("open door: %v", err)
	}
	m.open = d
	m.refresh()
	if h := m.hud(); h != nil {
		return guard(h.Init())
	}
	return nil
}

// awaits reports whether the door f fetched is still on screen waiting to
// open. While it downloaded, the player may have left the upsolve, the door
// may have crashed or been swapped, or a retry may have opened it.
func (m *Model) awaits(f fetchedMsg) bool {
	return m.open == nil && slices.ContainsFunc(m.stack, func(s screens.Screen) bool {
		p, ok := s.(*problemScreen)
		return ok && p.upsolve == f.upsolve && p.door.Problem == f.door.Problem
	})
}

func (m *Model) closeDoor() {
	if m.open != nil {
		m.d.Runner.Close()
	}
	m.open, m.judging = nil, false
}

// recheck takes the catalog's word on problems a fetch just touched: one
// whose tests turned out missing (a zip that 404s, say) is no longer
// judgeable, and the engine must stop offering it.
func (m *Model) recheck(ids ...catalog.ID) {
	for _, id := range ids {
		p, _ := m.d.Catalog.Get(id)
		stale := !p.Judgeable && slices.ContainsFunc(m.pool, func(q catalog.Problem) bool {
			return q.ID == id && q.Judgeable
		})
		if stale {
			m.pool = m.d.Catalog.All()
			return
		}
	}
}

// prefetchedMsg lists the doors a prefetch fetched or gave up on.
type prefetchedMsg []catalog.ID

// prefetch warms the cache for doors on offer so opening one is instant.
// Failures are ignored here; opening the door reports them.
//
// Each source fetches its doors one at a time, in door order, while the
// sources run side by side. A source sends one request at a time per host,
// so fetching its doors in parallel only interleaves them: in play, three
// cold Codeforces doors fetched together were all ready after 42-56 s,
// where in turn the first would have been ready in about a third of that.
// Door 1, where the cursor starts, goes first. Choosing a door cancels the
// others' fetches (stopPrefetch): those doors are gone, and their requests
// would only hold up the chosen one. The chosen door's own fetch goes on,
// and opening it joins that download rather than starting another.
func (m *Model) prefetch(ids []catalog.ID) tea.Cmd {
	m.stopPrefetch("")
	cat := m.d.Catalog
	ctxs := map[catalog.ID]context.Context{}
	m.prefetching = map[catalog.ID]context.CancelFunc{}
	var bySource [][]catalog.ID
	index := map[catalog.Source]int{}
	for _, id := range ids {
		ctxs[id], m.prefetching[id] = context.WithTimeout(m.ctx, 5*time.Minute)
		p, _ := cat.Get(id)
		i, ok := index[p.Source]
		if !ok {
			i = len(bySource)
			index[p.Source] = i
			bySource = append(bySource, nil)
		}
		bySource[i] = append(bySource[i], id)
	}
	// The maps are only read from here on, so the goroutines share them.
	cancels := m.prefetching
	return m.background(func(context.Context) tea.Msg {
		var wg sync.WaitGroup
		for _, queue := range bySource {
			wg.Go(func() {
				for _, id := range queue {
					if ctxs[id].Err() == nil {
						_, _ = cat.Fetch(ctxs[id], id)
					}
					cancels[id]()
				}
			})
		}
		wg.Wait()
		return prefetchedMsg(ids)
	})
}

// stopPrefetch cancels the prefetch of every door on offer but keep's.
func (m *Model) stopPrefetch(keep catalog.ID) {
	for id, cancel := range m.prefetching {
		if id != keep {
			cancel()
		}
	}
	m.prefetching = nil
}

// doorReadyMsg reports that the problem behind a chosen door is on disk,
// or could not be fetched.
type doorReadyMsg struct {
	index int
	id    catalog.ID
	err   error
}

// chooseDoor opens a door once its problem is on disk. The engine starts a
// door's timer when the door is chosen, so choosing first and fetching
// after spent the timer on the download: a cold Codeforces door fetched
// for 56 s in play with its countdown running and no editor, and a fetch
// that failed left a door that could only time out. A cached problem
// answers at once, so a prefetched door opens as before.
func (m *Model) chooseDoor(a game.ChooseDoor) tea.Cmd {
	s := m.state
	if s == nil || s.Phase != game.PhaseDoors || s.PendingUpgrade || a.Index < 0 || a.Index >= len(s.Doors) {
		return m.step(a) // the engine says why not
	}
	id := s.Doors[a.Index].Problem
	m.stopPrefetch(id)
	m.choosing = id
	m.flash = fmt.Sprintf("fetching door %d… its timer starts once it opens", a.Index+1)
	cat := m.d.Catalog
	return m.background(func(ctx context.Context) tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		_, err := cat.Fetch(ctx, id)
		return doorReadyMsg{index: a.Index, id: id, err: err}
	})
}

func (m *Model) onDoorReady(r doorReadyMsg) tea.Cmd {
	m.recheck(r.id)
	if m.choosing != r.id {
		return nil // the player chose another door meanwhile
	}
	m.choosing, m.flash = "", ""
	if r.err != nil {
		return fail("door %d: %v", r.index+1, r.err)
	}
	s := m.state
	if s == nil || s.Phase != game.PhaseDoors || r.index >= len(s.Doors) || s.Doors[r.index].Problem != r.id {
		return nil
	}
	return m.step(game.ChooseDoor{Index: r.index})
}

type judgedMsg struct {
	verdict       string
	passed, total int
	err           error
}

// submit judges the open door's full tests in the background.
func (m *Model) submit() tea.Cmd {
	if m.open == nil {
		return nil
	}
	if h := m.hud(); h != nil {
		// The HUD judges and reports back with a SubmittedMsg. A submit
		// while one runs replaces it, as :Submit from the editor does.
		m.judging = true
		req, _ := session.NewRequest(session.Command{Cmd: session.CmdSubmit})
		_, cmd := h.Update(req)
		return guard(cmd)
	}
	if m.d.Judge == nil || m.judging {
		return nil
	}
	m.judging = true
	d, judge := *m.open, m.d.Judge
	return func() tea.Msg {
		v, p, t, err := judge(context.Background(), d)
		return judgedMsg{verdict: v, passed: p, total: t, err: err}
	}
}

func (m *Model) onJudged(j judgedMsg) tea.Cmd {
	m.judging = false
	if j.err != nil {
		return fail("judge: %v", j.err)
	}
	// Libs come from accepted code only; harvest before the step so the
	// checkpoint it makes saves them.
	var harvest tea.Cmd
	if j.verdict == "AC" {
		harvest = m.harvestLibs()
	}
	if m.open != nil && m.open.Upsolve {
		return tea.Batch(harvest, m.onUpsolveJudged(j))
	}
	return tea.Batch(harvest, m.step(game.SubmitResult{Verdict: j.verdict, Passed: j.passed, Total: j.total}))
}
