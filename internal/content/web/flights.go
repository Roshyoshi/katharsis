package web

import (
	"context"
	"errors"
	"sync"
)

// Flights runs one call per key at a time: a Do for a key already in
// flight waits for that call and shares its result, so the doors on offer
// can all prefetch a problem and download it once. The zero value is
// ready to use.
type Flights[K comparable, V any] struct {
	mu sync.Mutex
	m  map[K]*flight[V]
}

type flight[V any] struct {
	done chan struct{}
	v    V
	err  error
}

// Do returns fn's result, running fn unless a call for key is in flight,
// in which case it waits for that call's. ctx bounds the wait; fn runs
// under whatever context it closes over. A call that failed only because
// its own caller's context ended is not handed on: a waiter whose context
// is still fine runs fn itself.
func (g *Flights[K, V]) Do(ctx context.Context, key K, fn func() (V, error)) (V, error) {
	for {
		g.mu.Lock()
		f, waiting := g.m[key]
		if !waiting {
			if g.m == nil {
				g.m = map[K]*flight[V]{}
			}
			f = &flight[V]{done: make(chan struct{})}
			g.m[key] = f
		}
		g.mu.Unlock()

		if !waiting {
			f.v, f.err = fn()
			g.mu.Lock()
			delete(g.m, key)
			g.mu.Unlock()
			close(f.done)
			return f.v, f.err
		}
		select {
		case <-ctx.Done():
			var zero V
			return zero, ctx.Err()
		case <-f.done:
		}
		if f.err != nil && isCtxErr(f.err) && ctx.Err() == nil {
			continue
		}
		return f.v, f.err
	}
}

func isCtxErr(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
