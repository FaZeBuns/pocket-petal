package chime

import (
	"testing"

	"github.com/FaZeBuns/pocket-petal/assets/sounds"
)

// The list holds *names*, and every link in that chain is a way for the flower to land
// in silence: a name no file answers to, a file that is not an MP3 (Windows would refuse
// it and the app would fall back to the chime it carries), or a list of one — where
// "drawn at random" would mean the same sound every time, which is the thing the list
// exists to avoid.
func TestEverySoundInTheListIsAnMp3TheAppCarries(t *testing.T) {
	if len(sounds.Names()) == 0 {
		t.Skip("no sounds in assets/sounds/pack: this checkout builds without any")
	}
	if len(Drops()) < 2 {
		t.Fatalf("there are %d sounds to draw from, and a random choice needs more than one", len(Drops()))
	}
	seen := map[string]bool{}
	for _, name := range Drops() {
		if seen[name] {
			t.Errorf("%q is in the list twice, so the odds are not what the list looks like", name)
		}
		seen[name] = true

		data := sounds.Get(name)
		if data == nil {
			t.Errorf("%q is in the list and not in assets/sounds: the exe would have no sound to play", name)
			continue
		}
		if len(data) < 4096 {
			t.Errorf("%q is %d bytes, which is not a sound", name, len(data))
		}
		// an MPEG frame sync, layer III: what an MP3 is, and what MCI is being asked for
		// by name when the file is opened as mpegvideo
		if data[0] != 0xFF || data[1]&0xE0 != 0xE0 {
			t.Errorf("%q does not start with an MPEG frame sync (% x)", name, data[:2])
		}
		if layer := (data[1] >> 1) & 3; layer != 1 {
			t.Errorf("%q is MPEG layer %d, and the app asks Windows for an MPEG file", name, layer)
		}
	}
}

// and the list is a choice, not the whole cupboard: whatever is named here is embedded,
// so naming another file is all it takes to use it.
func TestTheListIsDrawnFromTheFilesTheAppCarries(t *testing.T) {
	names := sounds.Names()
	if len(names) == 0 {
		t.Skip("no sounds in assets/sounds/pack: this checkout builds without any")
	}
	if len(names) < len(Drops()) {
		t.Fatalf("%d files are embedded and %d are named in the list", len(names), len(Drops()))
	}
	embedded := map[string]bool{}
	for _, name := range names {
		if sounds.Get(name) == nil {
			t.Errorf("%q is in the folder and not in the binary", name)
		}
		embedded[name] = true
	}
	for _, name := range Drops() {
		if !embedded[name] {
			t.Errorf("%q is in the list and not among the %d files the app carries", name, len(names))
		}
	}
}
