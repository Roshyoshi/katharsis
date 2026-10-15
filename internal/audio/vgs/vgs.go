// Package vgs plays SUZUKI PLAN's VGS music: MML text compiled to a note
// list and rendered by a six-channel wavetable synthesizer at 22050 Hz,
// 16-bit mono. It is a Go port of the VGS MML compiler (vgsmml.c) and BGM
// decoder (vgsdec.c, vgs2tone.c) as shipped in tohovgs-cli at
// cbb13ada5dd1d9244148a265946310bdd192d4b6, and renders the same samples:
// the conformance test compares it against the C build.
//
// The port keeps what the "東方BGM on VGS" songs use and drops the rest:
// the zlib step between compiler and decoder (the notes pass directly),
// the "kobushi" FM modulation (off unless a player turns it on), the
// fade-out, per-channel mute and volume registers, and the threaded
// decode queue.
//
// A song's `|` marks its loop point. Rendering past the end of the song
// jumps back there, so a looping song plays forever; IntroFrames and
// Frames say where the loop starts and where the first pass ends.
//
// The C sources carry this license, which covers this port:
//
//	SUZUKI PLAN - Video Game System MML Compiler
//	SUZUKI PLAN - Video Game System BGM Decoder
//	Copyright (c) 2016, Yoji Suzuki
//	All rights reserved.
//
//	Redistribution and use in source and binary forms, with or without
//	modification, are permitted provided that the following conditions
//	are met:
//
//	1. Redistributions of source code must retain the above copyright
//	   notice, this list of conditions and the following disclaimer.
//	2. Redistributions in binary form must reproduce the above copyright
//	   notice, this list of conditions and the following disclaimer in
//	   the documentation and/or other materials provided with the
//	   distribution.
//
//	THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
//	"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
//	LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
//	A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
//	OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
//	SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
//	LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
//	DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
//	THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
//	(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
//	OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
//
//	The views and conclusions contained in the software and documentation
//	are those of the authors and should not be interpreted as
//	representing official policies, either expressed or implied, of the
//	FreeBSD Project.
package vgs

// Notice is the copyright line the BSD license asks binaries to show.
const Notice = "VGS MML compiler and BGM decoder: Copyright (c) 2016, Yoji Suzuki (SUZUKI PLAN), BSD 2-Clause License"

// Rate is the sample rate songs are written for and rendered at.
const Rate = 22050

// Note types, as vgsdec_internal.h numbers them.
const (
	tEnv1   = 1  // op1 = channel, val = attack in samples
	tEnv2   = 2  // op1 = channel, val = release in samples
	tVol    = 3  // op1 = channel, val = volume
	tMVol   = 4  // val = master volume
	tKeyOn  = 5  // op1 = channel, op2 = tone type, op3 = key
	tKeyOff = 6  // op1 = channel
	tWait   = 7  // val = samples; 0 ends the song
	tPDown  = 8  // op1 = channel, val = samples per semitone of fall
	tJump   = 9  // val = index of the loop label
	tLabel  = 10 // the loop point
)

// maxNotes is the decoder's note buffer; the C decoder rejects a song
// that fills it.
const maxNotes = 65536

type note struct {
	typ, op1, op2, op3 uint8
	val                uint32
}

// Song is compiled MML.
type Song struct {
	notes []note
	intro int64 // frames before the loop label
	total int64 // frames of the first pass, to the jump or the end
	loops bool
}

// Frames is the length of the first pass through the song: the intro and
// one time through the loop, or the whole song when it does not loop.
func (s *Song) Frames() int64 { return s.total }

// IntroFrames is where the loop starts; rendering past Frames continues
// from there.
func (s *Song) IntroFrames() int64 { return s.intro }

// Loops reports whether the song has a loop point.
func (s *Song) Loops() bool { return s.loops }

// newSong times the first pass the way the decoder plays it: up to the
// first jump, or the first zero wait, which ends the song.
func newSong(notes []note) *Song {
	s := &Song{notes: notes}
	for _, n := range notes {
		if n.typ == tWait && n.val == 0 {
			break
		}
		if n.typ == tJump {
			s.loops = true
			for _, m := range notes[:min(int(n.val), len(notes))] {
				if m.typ == tWait {
					s.intro += int64(m.val)
				}
			}
			break
		}
		if n.typ == tWait {
			s.total += int64(n.val)
		}
	}
	if s.total == s.intro {
		s.loops = false // an empty loop body: the decoder ends there
	}
	return s
}
