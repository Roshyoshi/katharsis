package audio

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

// output is where the mixer plays. The engine suspends it when nothing has
// played for a while: a running output costs about 2% of a core even
// rendering silence (measured on an M-series Mac; see
// docs/research/audio.md), a suspended one nothing.
type output interface {
	Suspend()
	Resume()
}

// otoOutput plays through ebitengine/oto, which reaches CoreAudio (and
// ALSA, WASAPI) through purego, without cgo.
type otoOutput struct {
	mu        sync.Mutex
	ctx       *oto.Context
	player    *oto.Player
	suspended bool
}

// playerFrames is the player's own read-ahead. Its default, half a second,
// would delay every effect by that much; 1024 frames is 23 ms.
const playerFrames = 1024

// openOto opens the device and starts pulling r. It blocks until the
// device is ready, which took 0.4 to 1.2 s when measured, so the engine
// calls it off the UI's goroutine. oto allows one context per process.
func openOto(r io.Reader, buffer time.Duration) (*otoOutput, error) {
	ctx, ready, err := oto.NewContext(&oto.NewContextOptions{
		SampleRate:   Rate,
		ChannelCount: 2,
		Format:       oto.FormatFloat32LE,
		BufferSize:   buffer,
	})
	if err != nil {
		return nil, fmt.Errorf("audio device: %w", err)
	}
	<-ready
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("audio device: %w", err)
	}
	p := ctx.NewPlayer(r)
	p.SetBufferSize(playerFrames * 8)
	p.Play()
	return &otoOutput{ctx: ctx, player: p}, nil
}

func (o *otoOutput) Suspend() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.suspended {
		o.suspended = o.ctx.Suspend() == nil
	}
}

func (o *otoOutput) Resume() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.suspended {
		o.suspended = o.ctx.Resume() != nil
	}
}
