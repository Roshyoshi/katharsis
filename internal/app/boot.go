package app

import (
	"context"
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/catalog"
	"katharsis/internal/game"
	"katharsis/internal/screens"
	bootui "katharsis/internal/screens/boot"
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
	if m.state == nil {
		return nil
	}
	before := m.state.Phase
	effs, err := game.Step(m.state, &m.meta, m.pool, a, m.d.Now())
	if errors.Is(err, game.ErrInvalid) {
		return fail("%v", err)
	}
	if err != nil {
		return fail("engine: %v", err)
	}
	if err := m.d.Store.Checkpoint(m.meta, m.state); err != nil {
		return fail("save: %v", err)
	}
	cmd := m.effects(effs)
	if m.state.Phase != before || needsRebuild(a) {
		m.reset()
		return tea.Batch(cmd, m.initTop())
	}
	m.refresh()
	return cmd
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
	// period for submits past the deadline, but not for Tick.
	if m.state.Phase == game.PhaseProblem && !m.judging {
		return m.step(game.Tick{})
	}
	m.refresh()
	return nil
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
func (m *Model) prefetch(ids []catalog.ID) tea.Cmd {
	cat := m.d.Catalog
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		for _, id := range ids {
			cat.Fetch(ctx, id)
		}
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
	if m.open == nil || m.judging || m.d.Judge == nil {
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
