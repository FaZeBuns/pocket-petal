package chime

// The sound the flower makes, and the only sound it makes: the chime for the moment
// it is let go of onto a chat.
//
// The files themselves are the MP3s in assets/sounds (embedded by the sounds package,
// played through internal/win), and this short list is the whole of the app's opinion
// about what it sounds like. There are several of them rather than one because the
// note the flower carries is made fresh every time and the sound of it should not be
// the one thing that never changes — so one of these is drawn at random on each drop
// (see dropSound in internal/buddy).
//
// Swapping one is a matter of naming a different file here: everything in
// assets/sounds is embedded, so any of them will do.
//
// They are the chimes, not the whooshes: this is the flower's answer to landing
// somewhere, not an explosion. A whoosh is the sound of moving, and the flower has
// already stopped by then.

// Drops are the sounds of the flower being let go onto a chat, to be picked from.
var drops = []string{
	"vadim_makes_sound-fantasy-healing-spell-soft-magic-chime-1-546563.mp3",
	"cartoon_music-chime-fast-twinkle-494495.mp3",
	"universfield-magic-twinkle-244951.mp3",
	"humordome-fairy-sparkle-451414.mp3",
	"koiroylers-sparkle-355937.mp3",
}

// Drops are the sounds of the flower landing: one of them is drawn at random and
// played, and there is nothing else in the list of moments the app makes a noise for.
func Drops() []string { return drops }
