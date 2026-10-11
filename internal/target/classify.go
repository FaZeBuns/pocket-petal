// Package target is who else is on the desktop, and what the flower is allowed
// to do there.
//
// The rule the whole app hangs on: you dragged the flower onto that window, so
// that is consent — we may type there, whatever window it turns out to be. We may
// also interrupt a running answer there, but only on a window we actually
// recognise as a chat or a terminal — a drop is consent to talk to a stranger, not
// consent to press Escape in their spreadsheet. A tray click is not consent at
// all, so it only types when the window in front is a terminal or a known AI
// chat, and copies to the clipboard otherwise.
//
// Everything in here is a pure function over the facts the Win32 layer can see
// (class name, title, process), which is what makes the risky half of the app
// testable on a machine with no windows open at all.
package target

import (
	"regexp"
	"strings"
)

// ci compiles a case-insensitive pattern, which is how the reference writes all
// of them.
func ci(pattern string) *regexp.Regexp { return regexp.MustCompile("(?i)" + pattern) }

// Verdict is whether a window looks like a terminal we may type into.
type Verdict struct {
	OK     bool
	Reason string
}

// Terminal window classes. A terminal *class* is the strongest signal there is:
// conhost, Windows Terminal, Git Bash and the rest of them are unmistakable.
var winClasses = []*regexp.Regexp{
	ci(`^ConsoleWindowClass$`), // cmd.exe / conhost
	ci(`cascadia`),             // Windows Terminal
	ci(`mintty`),               // Git Bash
	ci(`conemu`),
	ci(`virtualconsole`), // ConEmu
	ci(`alacritty`),
	ci(`wezterm`),
	ci(`^hyper$`),
	ci(`kitty`),
}

// A title that reads like a terminal, for the ones we cannot recognise by class.
var titleHints = ci(`claude|terminal|powershell|pwsh|cmd\.exe|mintty|wsl|ubuntu|bash|conhost|wezterm|alacritty|kitty|git bash`)

// Classify decides whether a window is a terminal worth typing into. The class
// name is trusted first; a title that reads like a terminal is the fallback for
// terminals we cannot recognise, which is also what keeps an unknown AI chat
// from being mistaken for one.
func Classify(windowClass, title string) Verdict {
	for _, re := range winClasses {
		if re.MatchString(windowClass) {
			return Verdict{OK: true, Reason: "terminal window class: " + windowClass}
		}
	}
	if titleHints.MatchString(title) {
		return Verdict{OK: true, Reason: "window title looks like a terminal"}
	}
	reason := "no foreground window detected"
	if windowClass != "" {
		reason = "front window is " + windowClass + ", not a terminal"
	}
	return Verdict{OK: false, Reason: reason}
}

// orDefault is the reference's `||` for captions: an empty fact falls back to a
// friendlier word rather than an empty label.
func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
