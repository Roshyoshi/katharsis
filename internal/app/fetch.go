package app

import (
	"context"
	"errors"
	"sync"

	tea "charm.land/bubbletea/v2"

	"katharsis/internal/catalog"
	bootui "katharsis/internal/screens/boot"
)

// Door readiness (UX J9): every door's download is on a board the cards
// read, with the bytes it has received (content/web WithProgress); a door
// chosen before its problem is on disk shows how far it has got on the
// doors screen's last row, with its timer not running, and esc gives the
// choice back. Choosing another door, a ready one say, switches to it.

// doorFetch is the readiness state on the Model.
type doorFetch struct {
	board fetchBoard
	seen  uint64 // the board version the screens last got
	// wait is the door chosen while it downloads, nil for none: chosen is
	// its problem, which opens when the fetch lands (chooseDoor), and stop
	// stops waiting on it.
	wait   *bootui.Wait
	chosen catalog.ID
	stop   context.CancelFunc
}

// fetchBoard is each door's download, by problem. The fetch goroutines
// write it and the screens' Env reads a copy, so it has its own lock.
type fetchBoard struct {
	mu    sync.Mutex
	doors map[catalog.ID]bootui.Fetch
	ver   uint64 // bumps on every change, for syncFetch
}

func (b *fetchBoard) set(id catalog.ID, s bootui.FetchState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.doors == nil {
		b.doors = map[catalog.ID]bootui.Fetch{}
	}
	f := b.doors[id]
	if s == bootui.FetchGoing && f.State != bootui.FetchGoing {
		f.Got = 0
	}
	f.State = s
	b.doors[id] = f
	b.ver++
}

// queue marks doors that are not on disk yet as waiting their turn.
func (b *fetchBoard) queue(ids []catalog.ID) {
	for _, id := range ids {
		if b.state(id) != bootui.FetchReady {
			b.set(id, bootui.FetchQueued)
		}
	}
}

func (b *fetchBoard) state(id catalog.ID) bootui.FetchState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.doors[id].State
}

// adder is a WithProgress sink counting id's bytes.
func (b *fetchBoard) adder(id catalog.ID) func(int64) {
	return func(n int64) {
		b.mu.Lock()
		defer b.mu.Unlock()
		if f, ok := b.doors[id]; ok {
			f.Got += n
			b.doors[id] = f
			b.ver++
		}
	}
}

// finish records how id's fetch ended: on disk, failed (a timeout
// included), or stopped (its door went, or another was chosen), which says
// nothing about the door.
func (b *fetchBoard) finish(id catalog.ID, err error) {
	switch {
	case err == nil:
		b.set(id, bootui.FetchReady)
	case errors.Is(err, context.Canceled):
		if b.state(id) != bootui.FetchReady {
			b.set(id, bootui.FetchNone)
		}
	default:
		b.set(id, bootui.FetchFailed)
	}
}

// snapshot is a copy of the board for an Env.
func (b *fetchBoard) snapshot() map[catalog.ID]bootui.Fetch {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make(map[catalog.ID]bootui.Fetch, len(b.doors))
	for id, f := range b.doors {
		out[id] = f
	}
	return out
}

// fetchEnv puts the board and the wait into a boot screen's Env.
func (m *Model) fetchEnv(e *bootui.Env) {
	e.Fetch = m.fetch.board.snapshot()
	if w := m.fetch.wait; w != nil {
		c := *w
		e.Wait = &c
	}
}

// waitFor notes the door chosen at index, problem id, while it downloads,
// and returns the context its fetch runs under: cancelWait cancels it.
func (m *Model) waitFor(index int, id catalog.ID) context.Context {
	m.endWait()
	ctx, stop := context.WithCancel(m.ctx)
	m.fetch.wait = &bootui.Wait{Index: index, Since: m.d.Now()}
	m.fetch.chosen, m.fetch.stop = id, stop
	return ctx
}

// endWait forgets the chosen door and its wait, if any.
func (m *Model) endWait() {
	if m.fetch.stop != nil {
		m.fetch.stop()
	}
	m.fetch.wait, m.fetch.chosen, m.fetch.stop = nil, "", nil
}

// cancelWait is esc on the doors screen while a chosen door downloads: the
// choice is given back and the door stays on offer. Its own prefetch, if
// it has one, goes on, so choosing it again picks the download up; the
// other doors' prefetches, which the choice stopped, start again.
func (m *Model) cancelWait() tea.Cmd {
	w := m.fetch.wait
	if w == nil {
		return nil
	}
	m.endWait()
	m.refresh()
	s := m.state
	if s == nil || w.Index >= len(s.Doors) {
		return nil
	}
	var rest []catalog.ID
	for i, d := range s.Doors {
		if i != w.Index && m.fetch.board.state(d.Problem) != bootui.FetchReady {
			rest = append(rest, d.Problem)
		}
	}
	return tea.Batch(m.prefetchQueue(rest), note("door %d stays on offer; its download goes on", w.Index+1))
}

// version is the board's change count.
func (b *fetchBoard) version() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ver
}

// syncFetch runs after every update: when a download moved since the
// screens last got the board, they get it again, so a card turns ready
// when its download lands, not on the next tick.
func (m *Model) syncFetch() {
	if v := m.fetch.board.version(); v != m.fetch.seen {
		m.fetch.seen = v
		m.refresh()
	}
}
