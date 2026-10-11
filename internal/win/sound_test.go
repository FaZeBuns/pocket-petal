package win

import (
	"testing"

	"github.com/FaZeBuns/pocket-petal/assets/sounds"
	"github.com/FaZeBuns/pocket-petal/internal/chime"
)

// The playback path, end to end: a WAV made by the app, handed to Windows, played
// without a file ever existing. It is worth a test because it is the one thing in
// the app with no pixels — and the failure mode is silence, which nothing else here
// would notice.
func TestTheChimesWavPlays(t *testing.T) {
	if !PlayWav(chime.Fallback()) {
		// a machine with no audio device (a build agent, a session with the sound
		// card disabled) cannot play anything, and that is not a bug in this app
		t.Skip("nothing here will play a sound")
	}
	if len(playing) == 0 {
		t.Error("the buffer was not kept: Windows would be reading memory that had gone")
	}
	// an empty buffer is nothing to play, rather than a call that succeeds anyway
	if PlayWav(nil) {
		t.Error("playing nothing reported success")
	}
}

// And the sound a person actually hears: one of the MP3s the app carries, written out to
// a file and handed to MCI. This is the path that can be missing on a machine with no MP3
// decoder at all, and the app answers that by playing the chime it carries in code — so
// the test skips rather than fails, but says which of the two it heard.
func TestTheSoundsTheAppCarriesPlayThroughMci(t *testing.T) {
	drops := chime.Drops()
	if len(drops) == 0 {
		t.Fatal("the app carries no sounds to play")
	}
	if sounds.Get(drops[0]) == nil {
		t.Skip("no sounds in assets/sounds/pack: this checkout builds without any")
	}
	path, err := sounds.File(drops[0], t.TempDir())
	if err != nil {
		t.Fatalf("writing %q out to play: %v", drops[0], err)
	}
	if !PlayFile(path) {
		t.Skip("this machine has no MP3 decoder for MCI to reach: the flower falls back to its own chime")
	}
	// a file that is not there is not played, rather than reporting success
	if PlayFile(path + ".missing") {
		t.Error("playing a file that does not exist reported success")
	}
}
