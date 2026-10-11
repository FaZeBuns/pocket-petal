// Pocket Petal — a flower you can pick up and put on someone's chat.
//
// One executable. No runtime to install, no framework underneath: the flower is
// posed by the maths in internal/flower, rasterised by the same package, and put
// on screen by the Win32 layer in internal/win. Run it and the flower is on your
// desktop; drag it wherever you like.
//
// It is built as a windowed application — -H=windowsgui — because a flower on the
// desktop has no business owning a console window: double-clicking it used to open
// a black box that was there for the whole run and took the flower down with it when
// it was closed. The flags that print something borrow the console of whatever
// started them instead (see win.AttachConsole), which is how -version and -trace
// still have somewhere to say what they have to say.
//
//	go run .
//	go build -ldflags "-s -w -H=windowsgui" -o PocketPetal.exe .
//	tools/build-release.sh        # the same thing, stamped and checked
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/FaZeBuns/pocket-petal/internal/buddy"
	"github.com/FaZeBuns/pocket-petal/internal/flower"
	"github.com/FaZeBuns/pocket-petal/internal/target"
	"github.com/FaZeBuns/pocket-petal/internal/win"
)

// version is what this build calls itself. A plain `go build` gets the source's own
// number; the release build stamps it (-X main.version=…) so that the exe in a
// release is never guessed at from its file date.
var version = "0.1.1"

func main() {
	// the app names itself once (see internal/buddy), and this run is the one that
	// knows which build it is
	buddy.Version = version
	cfg := buddy.DefaultConfig()
	// the menu's choices are remembered between runs; an explicit flag still wins
	if err := cfg.Load(); err != nil {
		fmt.Fprintln(os.Stderr, buddy.Slug+":", err)
	}
	// and on the very first run the file is written, so that the settings are
	// somewhere a person can find them rather than appearing after the first trip
	// through the tray menu
	if path := buddy.SettingsPath(); path != "" {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			_ = cfg.Save()
		}
	}
	flag.IntVar(&cfg.Seed, "seed", cfg.Seed, "which flower to grow")
	flag.StringVar(&cfg.Palette, "palette", cfg.Palette, "flower colour: blush, buttercup, periwinkle or daisy")
	flag.IntVar(&cfg.Width, "width", cfg.Width, "the flower's box on screen, in pixels: the window is sized around it")
	flag.IntVar(&cfg.Height, "height", cfg.Height, "how tall the flower stands on screen, in pixels")
	flag.Float64Var(&cfg.Fit, "fit", cfg.Fit, "how much of the window the flower fills (0.5-1.2)")
	flag.IntVar(&cfg.FPS, "fps", cfg.FPS, "animation frames per second")
	flag.StringVar(&cfg.Detail, "detail", cfg.Detail, "full or lite (lite drops the pencil work)")
	flag.StringVar(&cfg.Note, "note", cfg.Note, "which note a delivery carries: flower")
	flag.BoolVar(&cfg.CopyOnly, "copy-only", cfg.CopyOnly, "never type: copy the note to the clipboard instead")
	flag.BoolVar(&cfg.Interrupt, "interrupt", cfg.Interrupt, "press Escape on a chat that is still answering")
	flag.BoolVar(&cfg.AutoSend, "enter", cfg.AutoSend, "press Enter after pasting the note")
	flag.BoolVar(&cfg.Caret, "caret", cfg.Caret, "click where the flower landed, to put the caret there")
	flag.BoolVar(&cfg.Ring, "ring", cfg.Ring, "draw the warm ring around the window under the flower")
	flag.BoolVar(&cfg.Sunshine, "sunshine", cfg.Sunshine, "let the sun pass over the flower now and then")
	flag.BoolVar(&cfg.Burst, "burst", cfg.Burst, "let the petals fly off on a delivery")
	flag.StringVar(&cfg.Corner, "corner", cfg.Corner, "where it stands: bottom-right, bottom-left, top-right, top-left")
	forget := flag.Bool("forget-settings", false, "ignore the saved settings and start from the defaults")
	flag.BoolVar(&cfg.Dry, "dry-run", cfg.Dry, "work out each delivery and print it, touching nothing")
	flag.BoolVar(&cfg.Trace, "trace", cfg.Trace, "print what the drag found and where the window is")
	flag.StringVar(&cfg.LogFile, "log", cfg.LogFile, "append trace and delivery lines to this file, however the app was started")
	flag.StringVar(&cfg.Dump, "dump", "", "write the first rendered frame to this PNG, alpha and all")
	flag.StringVar(&cfg.State, "state", "", "write where the flower is to this file, for a script to watch")
	flag.BoolVar(&cfg.Opaque, "debug-opaque", cfg.Opaque, "draw a solid frame, to check the window is on screen")
	flag.BoolVar(&cfg.Stats, "stats", false, "print fps and the cost of a frame")
	flag.IntVar(&cfg.Workers, "workers", runtime.NumCPU(), "rasteriser threads (1 = single-threaded)")
	version := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	// the windowed build has no console of its own, so borrow one if anyone is going
	// to read what this run has to say — and only then, so that closing a terminal
	// this was launched from can never take the flower with it
	if *version || cfg.Trace || cfg.Dry || cfg.Stats {
		win.AttachConsole()
	}

	if *version {
		fmt.Println(buddy.Name, buddy.Version)
		fmt.Println("settings:", buddy.SettingsPath())
		// and what those settings actually say, which is how "did my choices come
		// back the way I left them?" is answered without a screen
		if data, err := json.Marshal(cfg); err == nil {
			fmt.Println(string(data))
		}
		return
	}
	if *forget {
		cfg = buddy.DefaultConfig()
	}
	if !buddy.KnownCorner(cfg.Corner) {
		fmt.Fprintf(os.Stderr, buddy.Slug+": no such corner %q, using bottom-right\n", cfg.Corner)
		cfg.Corner = buddy.CornerBottomRight
	}

	runtime.LockOSThread()
	win.MakeDPIAware()

	if _, ok := target.ByID(cfg.Note); !ok && cfg.Note != "" {
		fmt.Fprintf(os.Stderr, buddy.Slug+": no such note %q, delivering a flower\n", cfg.Note)
		cfg.Note = "flower"
	}

	if _, ok := flower.Palettes[cfg.Palette]; !ok {
		fmt.Fprintf(os.Stderr, buddy.Slug+": no such palette %q, using blush\n", cfg.Palette)
		cfg.Palette = "blush"
	}

	b, err := buddy.New(cfg, func() {})
	if err != nil {
		fmt.Fprintln(os.Stderr, buddy.Slug+":", err)
		os.Exit(1)
	}
	defer b.Close()
	b.Run()
}
