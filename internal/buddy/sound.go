package buddy

import (
	"os"
	"path/filepath"

	"github.com/FaZeBuns/pocket-petal/assets/sounds"
	"github.com/FaZeBuns/pocket-petal/internal/chime"
	"github.com/FaZeBuns/pocket-petal/internal/win"
)

// The noise the flower makes, and the one moment it makes it: being let go of onto an
// AI chat. Nothing else in the app is heard — not picking it up, not a row of the panel,
// not the note arriving, not a drop onto a terminal, a page or the desktop — because the
// chime is how you know a note *landed in a chat*, and a sound that marks nothing is a
// sound that means nothing.
//
// One place decides it: *which* sounds there are is internal/chime, the bytes are the
// embedded files in assets/sounds, and the playing is internal/win. If any link in
// that chain gives way — the switch is off, the file cannot be written to the temp
// folder, the machine has no MP3 player to hand — the flower makes the little chime it
// carries in code instead, because a moment that should have made a sound and makes
// none is the one failure worth avoiding.

// soundDir is where the embedded sounds are written so that Windows has a file to
// play: one folder per app in the user's temp directory, made the first time a sound
// is used. The exe still needs nothing beside it; this is the operating system's
// price for playing an MP3, not an installation.
func soundDir() string {
	return filepath.Join(os.TempDir(), Slug, "sounds")
}

// playSound plays one of the app's sounds by name, if sounds are wanted at all.
func (b *Buddy) playSound(name string) {
	if !b.cfg.Sound || name == "" {
		return
	}
	if path, err := sounds.File(name, soundDir()); err == nil && win.PlayFile(path) {
		return
	}
	win.PlayWav(chime.Fallback())
}

// dropSound is the flower landing on an AI chat: one of the app's chimes, drawn at
// random, the way the note it carries is made fresh every time. It is called from
// exactly one place — a drop onto a chat the app recognised (see lands) — so hearing it
// is itself the news that the note is on its way. There is no other caller, and no other
// sound: this is the whole of what the app says out loud.
func (b *Buddy) dropSound() { b.playSound(b.dropSoundName()) }

// dropSoundName is which chime this drop is heard with: one of the list, drawn at
// random rather than taken in turn, so that two deliveries in a row do not sound like
// a loop. Drawn from the whole list every time, which is the point of there being a
// list — a person should hear all of them, and never be able to predict the next.
func (b *Buddy) dropSoundName() string {
	drops := chime.Drops()
	if len(drops) == 0 {
		return ""
	}
	return drops[b.rng.Intn(len(drops))]
}
