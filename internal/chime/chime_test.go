package chime

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
)

// The chime is written here rather than shipped, so the bytes are worth checking
// the way any other generated artefact is: a WAV whose header disagrees with its
// own data is a file that plays as noise, or as nothing.
func TestTheChimeIsAWellFormedWav(t *testing.T) {
	wav := Fallback()
	if len(wav) < 44 {
		t.Fatalf("the chime is %d bytes, which is not even a header", len(wav))
	}
	if string(wav[0:4]) != "RIFF" || string(wav[8:12]) != "WAVE" {
		t.Fatalf("the chime does not start as a WAV: %q %q", wav[0:4], wav[8:12])
	}
	if got := binary.LittleEndian.Uint32(wav[4:8]); int(got) != len(wav)-8 {
		t.Errorf("the RIFF size says %d and the file is %d", got, len(wav)-8)
	}
	// 16-bit mono PCM at 22050, which every Windows mixer has understood for decades
	if got := binary.LittleEndian.Uint16(wav[20:22]); got != 1 {
		t.Errorf("the format is %d, and it should be PCM", got)
	}
	if got := binary.LittleEndian.Uint16(wav[22:24]); got != 1 {
		t.Errorf("the chime has %d channels", got)
	}
	if got := binary.LittleEndian.Uint32(wav[24:28]); got != rate {
		t.Errorf("the sample rate is %d and the synthesiser used %d", got, rate)
	}
	if got := binary.LittleEndian.Uint16(wav[34:36]); got != 16 {
		t.Errorf("the chime is %d bits a sample", got)
	}
	if got := binary.LittleEndian.Uint32(wav[40:44]); int(got) != len(wav)-44 {
		t.Errorf("the data chunk says %d bytes and carries %d", got, len(wav)-44)
	}

	// and it is the length the tuning asks for: a note plus the gaps between them
	want := int(float64(rate) * (noteDur + stagger*float64(len(notes)-1)))
	if got := (len(wav) - 44) / 2; got != want {
		t.Errorf("the chime is %d samples and should be %d", got, want)
	}
}

// Loud enough to hear, quiet enough not to clip, and not silence: a level mistake is
// the difference between a chime and a click or a hiss.
func TestTheChimeIsAudibleAndDoesNotClip(t *testing.T) {
	wav := Fallback()
	peak, energy := 0, 0.0
	for i := 44; i+1 < len(wav); i += 2 {
		v := int16(binary.LittleEndian.Uint16(wav[i:]))
		if p := int(v); p < 0 {
			if -p > peak {
				peak = -p
			}
		} else if p > peak {
			peak = p
		}
		energy += math.Abs(float64(v))
	}
	share := float64(peak) / 32767
	t.Logf("the chime peaks at %.0f%% of full scale", share*100)
	if share < 0.3 {
		t.Errorf("the chime peaks at %.0f%% of full scale, which is barely audible", share*100)
	}
	if peak >= 32767 {
		t.Error("the chime clips")
	}
	if mean := energy / float64((len(wav)-44)/2) / 32767; mean < 0.01 {
		t.Errorf("the chime is %.3f of full scale on average, which is next to silence", mean)
	}

	// it starts and ends quietly: a chime that begins at full volume is a click, and
	// one that stops mid-note is a cut
	first := math.Abs(float64(int16(binary.LittleEndian.Uint16(wav[44:]))))
	last := math.Abs(float64(int16(binary.LittleEndian.Uint16(wav[len(wav)-2:]))))
	if first > float64(peak)*0.2 {
		t.Errorf("the chime starts at %v against a peak of %d", first, peak)
	}
	if last > float64(peak)*0.05 {
		t.Errorf("the chime ends at %v against a peak of %d", last, peak)
	}
}

// The same sound every time: it is a fixed piece of music, not a generator, and two
// runs that differed would mean a stray clock or a map iteration in the synthesis.
func TestTheChimeIsTheSameSoundEveryTime(t *testing.T) {
	if !bytes.Equal(Fallback(), Fallback()) {
		t.Fatal("the chime is different every time it is made")
	}
}
