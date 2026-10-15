package main

import (
	"testing"
	"testing/synctest"
	"time"

	"katharsis/internal/audio"
)

// closingSink is a sink whose Close (the fade) takes until hold closes.
type closingSink struct {
	audio.Null
	called, hold chan struct{}
}

func (s closingSink) Close() {
	close(s.called)
	<-s.hold
}

// The music fades out on the way out, but a fade that hangs holds the
// exit no longer than the wait. The null sink has nothing to close.
func TestCloseAudio(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		closeAudio(audio.Null{}, time.Second)

		s := closingSink{called: make(chan struct{}), hold: make(chan struct{})}
		start := time.Now()
		closeAudio(s, time.Second)
		if d := time.Since(start); d != time.Second {
			t.Errorf("a hung fade held the exit %v, want the 1s wait", d)
		}
		select {
		case <-s.called:
		default:
			t.Error("Close was not called")
		}
		close(s.hold)

		s = closingSink{called: make(chan struct{}), hold: make(chan struct{})}
		close(s.hold)
		start = time.Now()
		closeAudio(s, time.Second)
		if d := time.Since(start); d != 0 {
			t.Errorf("a fade that finished held the exit %v", d)
		}
	})
}
