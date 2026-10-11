// Package chime is the sound of the flower: what it sounds like, and the fallback
// for the one case Windows will not play.
//
// The sounds a person actually hears are the MP3s in assets/sounds, embedded in the
// exe and chosen per moment by library.go. This file is the spare: a small rising
// arpeggio written here, byte by byte, and played out of memory with `SND_MEMORY` —
// which is the one thing a WAV can do that an MP3 cannot, and the reason it is worth
// keeping. If the machine has no MP3 player to hand (a bare Windows install with the
// codecs stripped out, an audio device that is not answering), the flower still makes
// a sound rather than silently doing nothing.
//
// It is a rising arpeggio — three notes of a major triad going up, each struck a
// hair after the one before it — because that is the shape of a small pleasant
// surprise. Underneath it there is a soft second harmonic, which is what stops a
// pure sine from sounding like a test tone.
package chime

import (
	"encoding/binary"
	"math"
)

// The sound: a major triad going up, so it reads as a lift rather than a noise.
const (
	rate    = 22050 // samples per second: plenty for a few sine tones, half the bytes
	bits    = 16
	noteDur = 0.62 // how long one note rings, in seconds
	stagger = 0.07 // how far apart the notes are struck
	attack  = 0.004
	decay   = 5.2 // how fast each note falls away
)

// notes are the frequencies of the fallback chime, in order: C6, E6, G6 — a triad,
// which is the smallest interval that sounds like an answer rather than a beep.
var notes = []float64{1046.50, 1318.51, 1567.98}

// Fallback is the sound of a flower coming apart, as a complete WAV file: the sound
// used when the MP3s cannot be played at all, written here so that there is always
// something to fall back to.
func Fallback() []byte {
	length := int(float64(rate) * (noteDur + stagger*float64(len(notes)-1)))
	samples := make([]float64, length)
	for i, f := range notes {
		start := int(float64(rate) * stagger * float64(i))
		addNote(samples, start, f)
	}
	// a slow fade over the whole thing, so the tail ends in silence rather than at
	// whatever value it happens to be on
	wet := float64(length - 1)
	for i := range samples {
		samples[i] *= 1 - float64(i)/wet
	}
	return wav(samples)
}

// addNote lays one struck note into the buffer: a sine plus a quiet octave, with a
// fast attack (a click would be a discontinuity, and a click is heard as a fault)
// and an exponential fall.
func addNote(out []float64, start int, freq float64) {
	n := int(float64(rate) * noteDur)
	for i := 0; i < n; i++ {
		at := start + i
		if at >= len(out) {
			return
		}
		t := float64(i) / float64(rate)
		env := math.Exp(-decay * t)
		if a := t / attack; a < 1 {
			env *= a
		}
		phase := 2 * math.Pi * freq * t
		out[at] += env * (math.Sin(phase) + 0.17*math.Sin(2*phase))
	}
}

// wav wraps samples in a RIFF header: 16-bit mono PCM, which is the one format
// every Windows mixer has understood for thirty years.
func wav(samples []float64) []byte {
	peak := 0.0
	for _, s := range samples {
		peak = math.Max(peak, math.Abs(s))
	}
	gain := 1.0
	if peak > 0 {
		gain = 0.55 / peak // comfortable, and never clipping
	}
	data := make([]byte, len(samples)*2)
	for i, s := range samples {
		v := math.Max(-1, math.Min(1, s*gain))
		binary.LittleEndian.PutUint16(data[i*2:], uint16(int16(math.Round(v*32767))))
	}

	blockAlign := bits / 8
	byteRate := rate * blockAlign
	out := make([]byte, 0, 44+len(data))
	out = append(out, "RIFF"...)
	out = appendLE(out, uint32(36+len(data)))
	out = append(out, "WAVE"...)
	out = append(out, "fmt "...)
	out = appendLE(out, uint32(16))              // the fmt chunk is 16 bytes
	out = appendLEShort(out, uint16(1))          // PCM
	out = appendLEShort(out, uint16(1))          // mono
	out = appendLE(out, uint32(rate))            // sample rate
	out = appendLE(out, uint32(byteRate))        // bytes per second
	out = appendLEShort(out, uint16(blockAlign)) // bytes per frame
	out = appendLEShort(out, uint16(bits))       // bits per sample
	out = append(out, "data"...)
	out = appendLE(out, uint32(len(data)))
	return append(out, data...)
}

func appendLE(out []byte, v uint32) []byte {
	return binary.LittleEndian.AppendUint32(out, v)
}

func appendLEShort(out []byte, v uint16) []byte {
	return binary.LittleEndian.AppendUint16(out, v)
}
