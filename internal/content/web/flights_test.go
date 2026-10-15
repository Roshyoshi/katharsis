package web

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// A waiter's own context bounds its wait; the call it waited on runs on.
func TestFlightsWaiterCancel(t *testing.T) {
	t.Parallel()
	var g Flights[string, int]
	started, release := make(chan struct{}), make(chan struct{})
	leader := make(chan int)
	go func() {
		v, _ := g.Do(context.Background(), "k", func() (int, error) {
			close(started)
			<-release
			return 7, nil
		})
		leader <- v
	}()
	<-started // the flight is registered before fn runs

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := g.Do(ctx, "k", func() (int, error) {
		t.Error("a waiter ran fn while the key was in flight")
		return 0, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("waiter err = %v, want context.Canceled", err)
	}
	close(release)
	if v := <-leader; v != 7 {
		t.Errorf("leader = %d, want 7", v)
	}

	// The key is free once the call returns.
	if v, err := g.Do(context.Background(), "k", func() (int, error) { return 8, nil }); v != 8 || err != nil {
		t.Errorf("after the flight: %d, %v; want 8, nil", v, err)
	}
}

// A call that failed only because its caller gave up is not handed to a
// waiter that has not: the waiter runs fn itself.
func TestFlightsTakeOver(t *testing.T) {
	t.Parallel()
	var g Flights[string, int]
	var calls atomic.Int32
	lctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	leader := make(chan error)
	go func() {
		_, err := g.Do(lctx, "k", func() (int, error) {
			calls.Add(1)
			close(started)
			<-lctx.Done()
			return 0, lctx.Err()
		})
		leader <- err
	}()
	<-started
	waiter := make(chan int)
	go func() {
		v, err := g.Do(context.Background(), "k", func() (int, error) {
			calls.Add(1)
			return 7, nil
		})
		if err != nil {
			t.Errorf("waiter err = %v, want the waiter's own result", err)
		}
		waiter <- v
	}()
	time.Sleep(20 * time.Millisecond) // let the waiter start waiting
	cancel()
	if err := <-leader; !errors.Is(err, context.Canceled) {
		t.Errorf("leader err = %v", err)
	}
	if v := <-waiter; v != 7 {
		t.Errorf("waiter = %d, want 7", v)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("fn ran %d times, want 2 (the cancelled call, the waiter's)", n)
	}
}
