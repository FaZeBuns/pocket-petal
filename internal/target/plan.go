package target

// What the flower is allowed to do on a window, and whether it may interrupt
// what is running there. Both are pure functions of a Target, so the risky
// decisions are asserted in tests rather than trusted in the drag path.

// Plan is the delivery verdict for one drop.
type Plan struct {
	Action string // "type" | "clipboard"
	Cancel bool   // press Escape first, to interrupt a running answer
	Click  bool   // click where the flower landed, to put the caret there
	// Composer is a *second* click worth trying, at ComposerX/ComposerY, for the one
	// thing that a drop landing on a chat's transcript, sidebar or anywhere else that
	// is not its input box cannot fix by itself: see Target.ComposerGuess.
	Composer             bool
	ComposerX, ComposerY int
	Reason               string
}

// Deliverable reports whether a window is one the flower recognises outright as a
// chat or a terminal, where the coding agent lives. It is used for the two things
// that need a window we are actually sure about: a tray click, which was not aimed
// at anything, and interrupting a running answer, which is only safe on a window we
// know how to read "it is still generating" from. It is *not* what decides whether
// an explicit drop types — see PlanDelivery.
func (t Target) Deliverable() bool { return t.AIChat || t.Terminal }

// composerMargin is how far above the bottom edge of a chat window the compose box
// is guessed to live.
const composerMargin = 56

// ComposerGuess is a second point worth clicking after the drop point itself: the
// middle of the strip near the bottom of a *recognised chat's* window, where its
// compose box almost certainly is. Every popular chat interface — ChatGPT, Claude,
// Gemini, Copilot, a local model's own UI — anchors its box to the bottom of the
// window and runs it most of the width, so this is a far better second guess than
// leaving the caret wherever the cursor happened to be when you let go: a drop
// that lands on the transcript, a sidebar, or any of the rest of a chat's own
// window still finds the box.
//
// It is withheld for a terminal — the caret there follows the shell's own prompt
// wherever you click, so a second click can only ever be a no-op or a distraction
// — and for anything we do not recognise as a chat at all, where we have no reason
// to believe anything about where its own input lives.
func (t Target) ComposerGuess() (x, y int, ok bool) {
	if !t.AIChat || t.Rect.W <= 0 || t.Rect.H <= 0 {
		return 0, 0, false
	}
	y = t.Rect.Y + t.Rect.H - composerMargin
	if y < t.Rect.Y {
		y = t.Rect.Y + t.Rect.H/2
	}
	return t.Rect.X + t.Rect.W/2, y, true
}

// PlanDelivery decides what a drop may do.
//
// explicit means "you dragged the flower there, over whatever window you chose, and
// let go by hand". That is consent to type into it, full stop — the flower cannot
// carry an allowlist of every chat box that exists, and a universal flower that only
// delivers to names it already knows is not universal. So an explicit drop always
// pastes the note into whatever is under it, known chat, terminal, a chat box inside
// a browser tab, an Electron app, or anything else with a window: the one thing a
// drop onto a window we have never heard of does *not* grant is the right to
// interrupt it (Cancel is only ever t.Cancellable, which stays false for anything we
// cannot positively name as a chat or terminal).
//
// Without that explicit drop — a tray click, which was not aimed at any particular
// window — there is no "you chose this" to point to, so typing is still limited to a
// window we recognise outright: a known chat or a terminal. Anything else only gets
// the note on the clipboard, where it costs nobody anything.
func PlanDelivery(t Target, explicit, copyOnly bool) Plan {
	composerX, composerY, hasComposer := t.ComposerGuess()
	if explicit {
		if copyOnly {
			return Plan{Action: "clipboard", Reason: "copy-only mode is on"}
		}
		return Plan{
			Action: "type",
			Cancel: t.Cancellable,
			// put the caret where the flower landed before typing: this is what
			// makes it work with a harness we cannot recognise, and with a chat
			// box inside a browser, an IDE panel or an Electron app. Composer is
			// the fallback for when that point was not the box at all — see
			// Target.ComposerGuess
			Click:     true,
			Composer:  hasComposer,
			ComposerX: composerX, ComposerY: composerY,
			Reason: "you dropped the flower on " + t.Label,
		}
	}
	if !t.Deliverable() {
		return Plan{
			Action: "clipboard",
			Reason: t.Label + " is not a chat or a terminal: the note goes to your clipboard instead",
		}
	}
	if copyOnly {
		return Plan{Action: "clipboard", Reason: "copy-only mode is on"}
	}
	// a tray click is not consent to interrupt someone's work. There is no drop
	// point to click here at all — nobody aimed this at a window — so the composer
	// guess is not a fallback this time, it is the only click there is
	return Plan{
		Action:    "type",
		Composer:  hasComposer,
		ComposerX: composerX, ComposerY: composerY,
		Reason: t.Label + " is the front window",
	}
}

// CancelDecision is whether to press Escape before typing, and what to press when
// the first one does not take.
type CancelDecision struct {
	Cancel bool
	Key    string // "Escape" or "ctrl+c"
	Reason string
}

// DecideCancel answers "should we stop what is running here?". It can only ever
// be yes for a window a note is being delivered to at all.
//
// Escape interrupts a generation in essentially every AI chat and coding agent;
// in a terminal the stronger, unambiguous answer is Ctrl+C, which is only ever
// used for a terminal (in a browser it means "copy").
func DecideCancel(t Target, explicit, enabled, streaming bool, attempt int) CancelDecision {
	if !enabled {
		return CancelDecision{Reason: "cancelling is switched off"}
	}
	if !explicit {
		return CancelDecision{Reason: "only a dropped flower may interrupt a request"}
	}
	if t.Label == "" {
		return CancelDecision{Reason: "there is no window to interrupt"}
	}
	if !t.Deliverable() {
		return CancelDecision{Reason: t.Label + " is not somewhere a note is going"}
	}
	if !t.Cancellable {
		return CancelDecision{Reason: t.Label + " does not look interruptible"}
	}
	if !streaming {
		return CancelDecision{Reason: "nothing was running"}
	}
	if attempt <= 1 {
		return CancelDecision{Cancel: true, Key: "Escape", Reason: "something was being generated"}
	}
	if t.Terminal {
		return CancelDecision{Cancel: true, Key: "ctrl+c", Reason: "the escape did not take, so an interrupt"}
	}
	return CancelDecision{Cancel: true, Key: "Escape", Reason: "it ignored the first escape"}
}
