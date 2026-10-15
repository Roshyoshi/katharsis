package app

import (
	"context"
	"errors"
	"fmt"
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

func (m *Model) env() bootui.Env {
	return bootui.Env{State: m.state, Meta: m.meta, Lookup: m.d.Catalog.Get, Now: m.d.Now()}
}

// startBoot begins a new boot. A boot still running is given up first, so
// leaving one from GRUB costs what quitting a door costs.
func (m *Model) startBoot(b screens.StartBoot) tea.Cmd {
	if m.state != nil && m.state.Phase != game.PhaseCrashed && m.state.Phase != game.PhaseCleared {
		if err := m.abandon(); err != nil {
			return fail("abandon boot %d: %v", m.state.Boot, err)
		}
	}
	r, ok := m.d.Presets[b.Preset]
	if !ok {
		r = m.d.Presets["standard"]
	}
	seed := b.Seed
	if seed == 0 {
		seed = m.d.Seed()
	}
	st, effs, err := game.NewBoot(r, &m.meta, m.pool, b.Prestige, seed, m.d.Now())
	if err != nil {
		return fail("new boot: %v", err)
	}
	m.state = st
	// Save before the first step, so quitting right away still resumes.
	if err := m.d.Store.Checkpoint(m.meta, m.state); err != nil {
		return fail("save: %v", err)
	}
	m.reset()
	return tea.Batch(m.initTop(), m.effects(effs))
}

// abandon gives up a running boot. In a door that is :q!; elsewhere there
// is no engine action for it, so the store just marks the boot abandoned.
func (m *Model) abandon() error {
	if m.state.Phase == game.PhaseProblem {
		if _, err := game.Step(m.state, &m.meta, m.pool, game.GiveUp{}, m.d.Now()); err != nil {
			return err
		}
		m.closeDoor()
		return m.d.Store.Checkpoint(m.meta, m.state)
	}
	return m.d.Store.AbandonBoot(m.state.Boot)
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
	before := m.state.Phase
	effs, err := game.Step(m.state, &m.meta, m.pool, a, m.d.Now())
	if errors.Is(err, game.ErrInvalid) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("engine: %w", err)
	}
	// The step happened whether or not it was saved, so its effects run
	// either way: a door the engine opened must open on screen too.
	var cmds []tea.Cmd
	if err := m.d.Store.Checkpoint(m.meta, m.state); err != nil {
		cmds = append(cmds, fail("save: %v", err))
	}
	cmds = append(cmds, m.effects(effs))
	if m.state.Phase != before || needsRebuild(a) {
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

// needsRebuild: actions that change which screen a phase shows without
// changing the phase (an upgrade picked, a door swapped).
func needsRebuild(a game.Action) bool {
	switch a.(type) {
	case game.PickUpgrade, game.Retreat, game.ChooseDoor:
		return true
	}
	return false
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

// effects carries out what the engine asked for.
func (m *Model) effects(effs []game.Effect) tea.Cmd {
	var cmds []tea.Cmd
	for _, e := range effs {
		switch e := e.(type) {
		case game.OpenProblem:
			cmds = append(cmds, m.openDoor(e.Door, false))
		case game.CloseProblem:
			m.closeDoor()
		case game.OpenEditorial:
			cmds = append(cmds, m.openDoc(e.Problem, true))
		case game.Sound:
			if m.d.Sound != nil {
				m.d.Sound(e.Name)
			}
		case game.Prefetch:
			cmds = append(cmds, m.prefetch(e.Problems))
		}
		// RevealTags, ShowCrash and ShowCleared need nothing beyond the
		// screen rebuild: the state already carries them.
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
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		a, err := cat.Fetch(ctx, d.Problem)
		return fetchedMsg{door: d, upsolve: upsolve, assets: a, err: err}
	}
}

func (m *Model) onFetched(f fetchedMsg) tea.Cmd {
	if f.err != nil {
		return fail("fetch %s: %v", f.door.Problem, f.err)
	}
	p, _ := m.d.Catalog.Get(f.door.Problem)
	d := &Door{Door: f.door, Problem: p, Assets: f.assets, Upsolve: f.upsolve, Opened: m.d.Now()}
	if f.upsolve {
		d.ReadEditorial = m.sawEditorial(f.door.Problem)
		d.Sub, d.SeedBoot = m.upsolveFiles(f.door.Problem)
	}
	if m.state != nil && !f.upsolve {
		d.Boot = m.state.Boot
		if m.state.Current != nil {
			d.Deadline = m.state.Current.Deadline
		}
	}
	m.open = d
	if m.d.Runner != nil {
		if err := m.d.Runner.Open(context.Background(), *d); err != nil {
			return fail("open door: %v", err)
		}
	}
	m.refresh()
	if r, ok := m.d.Runner.(hudRunner); ok && r.HUD() != nil {
		return guard(r.HUD().Init())
	}
	return nil
}

func (m *Model) closeDoor() {
	if m.open != nil && m.d.Runner != nil {
		m.d.Runner.Close()
	}
	m.open, m.judging = nil, false
}

// prefetch warms the cache for doors on offer so opening one is instant.
// Failures are ignored here; opening the door reports them.
//
// The doors fetch in parallel: one after another, a cold USACO problem
// took 1-2 s each in play, so the third door was ready about 6 s after the
// room appeared, and a Codeforces one can take 8 s. The sources share one
// download per problem (opening a door mid-prefetch joins it) and limit
// requests per host themselves, so this stays polite.
func (m *Model) prefetch(ids []catalog.ID) tea.Cmd {
	cat := m.d.Catalog
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		var wg sync.WaitGroup
		for _, id := range ids {
			wg.Go(func() { cat.Fetch(ctx, id) })
		}
		wg.Wait()
		return nil
	}
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
	if r, ok := m.d.Runner.(hudRunner); ok && r.HUD() != nil {
		// The HUD judges and reports back with a SubmittedMsg. A submit
		// while one runs replaces it, as :Submit from the editor does.
		m.judging = true
		req, _ := session.NewRequest(session.Command{Cmd: session.CmdSubmit})
		_, cmd := r.HUD().Update(req)
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
	if m.open != nil && m.open.Upsolve {
		return m.onUpsolveJudged(j)
	}
	return m.step(game.SubmitResult{Verdict: j.verdict, Passed: j.passed, Total: j.total})
}
