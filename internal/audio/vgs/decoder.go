package vgs

// voice is one of the six channels (_PSGCH, without the FM fields).
type voice struct {
	wave         []int16 // the tone table for the current key; nil before the first key-on
	vol          uint8
	keyOn        bool
	cur          uint32 // position in wave, in 44.1 kHz samples
	count        uint32 // samples into the attack or release
	env1, env2   uint32 // attack and release lengths
	toneT, toneK uint8  // tone type and key, kept for pitch-down
	pdown, pcnt  uint32 // samples per semitone of fall, and the counter
}

// Decoder renders a Song (vgsdec_execute).
type Decoder struct {
	song  *Song
	ch    [6]voice
	mvol  uint16
	wait  uint32 // samples left before the next note
	nidx  int
	ended bool
}

// NewDecoder starts s from the top.
func NewDecoder(s *Song) *Decoder {
	d := &Decoder{song: s}
	d.Reset()
	return d
}

// Reset rewinds to the start of the song.
func (d *Decoder) Reset() {
	*d = Decoder{song: d.song}
}

// Ended reports whether a song without a loop has played out.
func (d *Decoder) Ended() bool { return d.ended }

// Render fills dst with the next samples and returns how many belong to
// the song; after the end of a song without a loop the rest of dst is
// silence. Successive calls continue where the last one stopped, and
// split the same way C's buffers do: a call boundary changes nothing.
//
// One C behavior that does depend on the call boundary is kept: with the
// master volume at 0, a call renders silence without moving time on.
func (d *Decoder) Render(dst []int16) int {
	clear(dst)
	if d.ended {
		return 0
	}
	if d.wait == 0 {
		if d.wait = d.next(); d.wait == 0 {
			d.ended = true
			return 0
		}
	}
	if d.mvol == 0 {
		return len(dst)
	}
	for i := range dst {
		mix := int32(0)
		for j := range d.ch {
			c := &d.ch[j]
			if c.wave == nil {
				continue
			}
			c.cur %= uint32(c.wave[0])
			w := int32(c.wave[1+c.cur])
			c.cur += 2 // the tables are 44.1 kHz; the output is half that
			w *= int32(c.vol) * int32(d.mvol)
			var pw int32
			if c.keyOn {
				pw = 100
				if c.count < c.env1 {
					c.count++
					pw = int32(c.count * 100 / c.env1)
				}
			} else if c.count < c.env2 {
				c.count++
				pw = 100 - int32(c.count*100/c.env2)
			}
			// C clips after each channel is added, not once at the end.
			mix = max(-32768, min(mix+w*pw/100, 32767))
			if c.pdown != 0 {
				c.pcnt++
				if c.pdown < c.pcnt {
					c.pcnt = 0
					if c.toneK != 0 {
						c.toneK--
					}
					c.setNote(c.toneT, c.toneK)
				}
			}
		}
		dst[i] = int16(mix)
		if d.wait--; d.wait == 0 {
			if d.wait = d.next(); d.wait == 0 {
				d.ended = true // C stops here and leaves the rest of the buffer silent
				return i + 1
			}
		}
	}
	return len(dst)
}

// Skip renders and drops n samples.
func (d *Decoder) Skip(n int64) {
	var buf [2048]int16
	for n > 0 && !d.ended {
		k := int(min(n, int64(len(buf))))
		d.Render(buf[:k])
		n -= int64(k)
	}
}

func (c *voice) setNote(t, n uint8) {
	c.wave = tones[min(int(t), 3)][int(n)%85]
}

// next runs notes up to the next wait and returns its length; 0 is the
// end (get_next_note).
func (d *Decoder) next() uint32 {
	notes := d.song.notes
	at := func(i int) note {
		if i < len(notes) {
			return notes[i]
		}
		return note{} // C's zeroed buffer: type 0 ends the song
	}
	if n := at(d.nidx); n.typ == tWait && n.val == 0 {
		return 0
	}
	jumps := 0
	for ; at(d.nidx).typ != tWait; d.nidx++ {
		n := at(d.nidx)
		if n.op1 >= 6 && n.typ != tMVol && n.typ != tJump && n.typ != tLabel {
			return 0 // the compiler never writes such a channel; C would read past its array
		}
		c := &d.ch[n.op1%6]
		switch n.typ {
		case tEnv1:
			c.env1 = n.val
		case tEnv2:
			c.env2 = n.val
		case tVol:
			c.vol = uint8(n.val)
		case tMVol:
			d.mvol = uint16(n.val)
		case tKeyOn:
			c.keyOn, c.count, c.cur = true, 0, 0
			c.toneT, c.toneK = n.op2, n.op3
			c.setNote(n.op2, n.op3)
		case tKeyOff:
			c.keyOn, c.count = false, 0
		case tPDown:
			c.pdown, c.pcnt = n.val, 0
		case tJump:
			// The loop's label, then the loop's ++ steps past it. Two
			// jumps without a wait between them would spin forever in C.
			if jumps++; jumps > 1 {
				return 0
			}
			d.nidx = int(n.val)
		case tLabel:
		default:
			return 0
		}
	}
	ret := at(d.nidx).val
	if ret != 0 {
		d.nidx++
	}
	return ret
}
