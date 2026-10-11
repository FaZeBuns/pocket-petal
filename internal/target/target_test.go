package target

import (
	"regexp"
	"strings"
	"testing"
)

// "Works with any AI" is only true if we can tell what we landed on, and "safe"
// is only true if an accidental click cannot interrupt a stranger's window.

func win(over Window) Window {
	over.Hwnd = 42
	return over
}

func TestAChatInABrowserIsNamedByItsPage(t *testing.T) {
	target := Describe(win(Window{Title: "ChatGPT - Google Chrome", Class: "Chrome_WidgetWin_1", Process: "chrome"}))
	if target.Kind != "ai-chat" || target.Label != "ChatGPT in Chrome" {
		t.Fatalf("got %q / %q", target.Kind, target.Label)
	}
	if !target.AIChat || !target.Cancellable || !target.Browser {
		t.Fatalf("flags wrong: %+v", target)
	}
}

func TestKnownDesktopAIsAreRecognised(t *testing.T) {
	cases := []struct {
		facts Window
		label string
	}{
		{Window{Title: "Claude", Class: "Claude", Process: "claude"}, "Claude"},
		{Window{Title: "Freebuff", Class: "Chrome_WidgetWin_1", Process: "Freebuff"}, "Freebuff"},
		{Window{Title: "Cursor", Class: "Chrome_WidgetWin_1", Process: "Cursor"}, "Cursor"},
		{Window{Title: "Gemini - Brave", Process: "brave"}, "Gemini in Brave"},
		{Window{Title: "Perplexity", Process: "msedge"}, "Perplexity in Msedge"},
		{Window{Title: "DeepSeek", Process: "chrome"}, "DeepSeek in Chrome"},
		{Window{Title: "Grok", Process: "firefox"}, "Grok in Firefox"},
		{Window{Title: "Ollama", Process: "ollama"}, "your local model"},
		{Window{Title: "Antigravity", Process: "antigravity"}, "Antigravity"},
	}
	for _, c := range cases {
		target := Describe(win(c.facts))
		if target.Kind != "ai-chat" || target.Label != c.label || !target.Cancellable {
			t.Errorf("%+v -> %q / %q / cancellable=%v", c.facts, target.Kind, target.Label, target.Cancellable)
		}
	}
}

func TestTerminalsAreRecognisedAndNamedAfterTheAgent(t *testing.T) {
	plain := Describe(win(Window{Class: "ConsoleWindowClass", Title: "Command Prompt", Process: "cmd"}))
	if plain.Kind != "terminal" || !plain.Terminal {
		t.Fatalf("got %q", plain.Kind)
	}
	if !strings.Contains(plain.Label, "Command Prompt") && plain.Label != "your terminal" {
		t.Fatalf("label %q", plain.Label)
	}
	claude := Describe(win(Window{Class: "CASCADIA_HOSTING_WINDOW_CLASS", Title: "claude — ~/project", Process: "WindowsTerminal"}))
	if claude.Kind != "terminal" || claude.Label != "Claude Code" || !claude.Cancellable {
		t.Fatalf("got %q / %q", claude.Kind, claude.Label)
	}
	aider := Describe(win(Window{Class: "mintty", Title: "aider (main)", Process: "mintty"}))
	if aider.Label != "Aider" {
		t.Fatalf("got %q", aider.Label)
	}
}

func TestAnEditorOrAnUnknownAppIsNotAChat(t *testing.T) {
	code := Describe(win(Window{Title: "main.js - Visual Studio Code", Class: "Chrome_WidgetWin_1", Process: "Code"}))
	if code.Kind != "app" || code.Cancellable || code.AIChat {
		t.Fatalf("got %+v", code)
	}
	if code.Label != "VS Code" {
		t.Fatalf("label %q", code.Label)
	}
	notepad := Describe(win(Window{Title: "Untitled - Notepad", Class: "Notepad", Process: "notepad"}))
	if notepad.Kind != "app" || notepad.Label != "Notepad" {
		t.Fatalf("got %q / %q", notepad.Kind, notepad.Label)
	}
	nothing := Describe(Window{})
	if nothing.Kind != "unknown" || nothing.Label != "that window" || nothing.Cancellable {
		t.Fatalf("got %+v", nothing)
	}
}

func TestABrowserShowingSomethingElseIsLabelledByItsPage(t *testing.T) {
	target := Describe(win(Window{Title: "Pull requests · GitHub", Class: "Chrome_WidgetWin_1", Process: "chrome"}))
	if target.Kind != "browser" || target.Cancellable {
		t.Fatalf("got %+v", target)
	}
	if !strings.Contains(target.Label, "GitHub") {
		t.Fatalf("label %q", target.Label)
	}
}

// A browser is the one window where a name in the title is evidence of almost
// nothing: a search for claude, a video about Cursor, a page about bash are all
// pages *about* something, so none of them is named or glowed-at as a *chat* —
// Kind stays "browser", not "ai-chat", and the window is not one we would ever
// interrupt (Cancellable stays false). But it is still a window, and a universal
// flower delivers to whatever window you explicitly aimed it at: see
// TestADroppedFlowerTalksToChatsAndOnlyCopiesEverywhereElse for what an actual
// drop does there.
func TestABrowserPageAboutAChatIsNotAChat(t *testing.T) {
	for _, title := range []string{
		"Why ChatGPT lies - The Verge",
		"Claude vs ChatGPT - YouTube",
		"Cursor - Google Search",
		"bash - Google Search",
		"wsl tutorial - Microsoft Edge",
		"I let Claude run my life for a week",
		"AustinPC - Google Chrome",
		"New Tab",
	} {
		tgt := Describe(win(Window{Title: title, Class: "Chrome_WidgetWin_1", Process: "chrome"}))
		if tgt.AIChat || tgt.Terminal || tgt.Cancellable {
			t.Errorf("a browser page %q should not be named a chat or be interruptible: %+v", title, tgt)
		}
		if tgt.Kind != "browser" {
			t.Errorf("a browser page %q is a browser, got %q", title, tgt.Kind)
		}
		// an *unaimed* tray click still only ever types into a window we recognise
		if got := PlanDelivery(tgt, false, false); got.Action != "clipboard" {
			t.Errorf("a tray click landing on %q should only copy: %+v", title, got)
		}
	}
}

// ...but the chat you actually have open in a browser is still the chat: that is
// where a lot of people keep it, and the drop has to work there.
func TestAChatPageIsStillAChatInABrowser(t *testing.T) {
	cases := []struct {
		title string
		proc  string
		label string
	}{
		{"Flower physics - ChatGPT", "chrome", "ChatGPT in Chrome"},
		{"ChatGPT - Google Chrome", "chrome", "ChatGPT in Chrome"},
		{"Claude", "firefox", "Claude in Firefox"},
		{"Gemini", "msedge", "Gemini in Msedge"},
	}
	for _, c := range cases {
		tgt := Describe(win(Window{Title: c.title, Class: "Chrome_WidgetWin_1", Process: c.proc}))
		if tgt.Kind != "ai-chat" || tgt.Label != c.label {
			t.Errorf("%q in %s -> %q / %q, want %q", c.title, c.proc, tgt.Kind, tgt.Label, c.label)
		}
		if got := PlanDelivery(tgt, true, false); got.Action != "type" {
			t.Errorf("%q should be typed into: %+v", c.title, got)
		}
	}
}

// The ring is only ever drawn around a window the cursor is inside, which is what
// stops a stale target glowing at you after you have dragged away from it.
func TestARectangleKnowsWhatIsInsideIt(t *testing.T) {
	r := Rect{X: 100, Y: 200, W: 300, H: 400}
	for _, p := range []struct{ x, y int }{{100, 200}, {399, 599}, {250, 350}} {
		if !r.Contains(p.x, p.y) {
			t.Errorf("%d,%d should be inside %+v", p.x, p.y, r)
		}
	}
	for _, p := range []struct{ x, y int }{{99, 200}, {100, 199}, {400, 300}, {250, 600}} {
		if r.Contains(p.x, p.y) {
			t.Errorf("%d,%d should be outside %+v", p.x, p.y, r)
		}
	}
	// and a window nobody has found yet is nowhere
	if (Rect{}).Contains(0, 0) {
		t.Error("an empty rectangle contains nothing")
	}
}

// The windows that were actually open on the machine this was tuned on, exactly
// as Windows reported them. A rule that is only checked against titles somebody
// made up is a rule about made-up titles.
func TestTheWindowsOfTheMachineThisWasTunedOn(t *testing.T) {
	cases := []struct {
		facts Window
		kind  string
	}{
		// an AI chat that names itself only in its process
		{Window{Title: "Test Conversation - windo...", Class: "", Process: "antigravity"}, "ai-chat"},
		{Window{Title: "Freebuff Desktop", Class: "Chrome_WidgetWin_1", Process: "Freebuff"}, "ai-chat"},
		// a browser showing a video, which is not a chat however it is titled
		{Window{Title: "I Challenged GTAs Best Player - YouTube - Google Chrome", Class: "Chrome_WidgetWin_1", Process: "chrome"}, "browser"},
		// the text input host is not somewhere a note goes
		{Window{Title: "Microsoft Text Input Application", Class: "Windows.UI.Core.CoreWindow", Process: "TextInputHost"}, "app"},
	}
	for _, c := range cases {
		tgt := Describe(c.facts)
		if tgt.Kind != c.kind {
			t.Errorf("%q (%s) -> %q, want %q", c.facts.Title, c.facts.Process, tgt.Kind, c.kind)
		}
		if want := c.kind == "ai-chat" || c.kind == "terminal"; tgt.Deliverable() != want {
			t.Errorf("%q deliverable=%v, want %v", c.facts.Title, tgt.Deliverable(), want)
		}
	}
}

func TestTitlesAreTidied(t *testing.T) {
	cases := map[string]string{
		"ChatGPT - Google Chrome":                     "ChatGPT",
		"Claude — Mozilla Firefox (Private Browsing)": "Claude",
		"ChatGPT - Microsoft Edge":                    "ChatGPT",
		"Gemini - Brave":                              "Gemini",
		"(3) ChatGPT - Google Chrome":                 "(3) ChatGPT",
		"  spaced   out  ":                            "spaced out",
		"":                                            "",
	}
	for in, want := range cases {
		if got := TidyTitle(in); got != want {
			t.Errorf("TidyTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestProcessesArePrettified(t *testing.T) {
	cases := map[string]string{
		"Code.exe":         "VS Code",
		"windows_terminal": "Windows Terminal",
		"chrome":           "Chrome",
		"":                 "",
	}
	for in, want := range cases {
		if got := PrettyProcess(in); got != want {
			t.Errorf("PrettyProcess(%q) = %q, want %q", in, got, want)
		}
	}
}

// A drop is consent to type, wherever it landed. The flower cannot carry a list of
// every chat box in the world, so a "universal" flower that only delivers to names
// it already knows is not universal — it only looks that way until you try it on
// the one chat nobody taught it about. What a drop is never consent to, on a window
// we do not positively recognise, is interrupting whatever is running there
// (Cancel stays false — see t.Cancellable in TestWeOnlyInterruptWhenSomethingIsRunning).
func TestADroppedFlowerTalksToChatsAndOnlyCopiesEverywhereElse(t *testing.T) {
	chat := Describe(win(Window{Title: "Claude", Process: "claude"}))
	plan := PlanDelivery(chat, true, false)
	if plan.Action != "type" || !plan.Cancel || !plan.Click {
		t.Fatalf("got %+v", plan)
	}
	if !strings.Contains(plan.Reason, "you dropped the flower") {
		t.Fatalf("reason %q", plan.Reason)
	}

	// a terminal is somewhere an agent lives, so it is somewhere we type. Whether
	// it may also be interrupted is the window's own answer (Cancellable), which is
	// what the settings switch is over the top of
	terminal := Describe(win(Window{Class: "ConsoleWindowClass", Title: "Command Prompt", Process: "cmd"}))
	if got := PlanDelivery(terminal, true, false); got.Action != "type" {
		t.Fatalf("a terminal should be typed into: %+v", got)
	}

	// and a window we have *never heard of* is still typed into, because you aimed
	// the flower at it yourself — it just never gets Cancel or a chime, because
	// nothing told us there was anything there worth interrupting
	for _, c := range []struct {
		what  string
		facts Window
	}{
		{"an editor", Window{Title: "notes.txt - Notepad", Process: "notepad"}},
		{"a browser page", Window{Title: "Pull requests · GitHub", Process: "chrome"}},
		{"a chat box nobody taught the flower about", Window{Title: "Mystery AI", Process: "mysteryai"}},
	} {
		tgt := Describe(win(c.facts))
		got := PlanDelivery(tgt, true, false)
		if got.Action != "type" || got.Cancel || !got.Click {
			t.Errorf("dropping the flower on %s should type without interrupting: %+v", c.what, got)
		}
		if !strings.Contains(got.Reason, "you dropped the flower") {
			t.Errorf("dropping the flower on %s should say so: %q", c.what, got.Reason)
		}
	}

	// a tray click is a different thing: nobody aimed it at this particular window,
	// so it still only ever types into one we recognise outright, and copies
	// everywhere else
	for _, c := range []struct {
		what  string
		facts Window
	}{
		{"an editor", Window{Title: "notes.txt - Notepad", Process: "notepad"}},
		{"a browser page", Window{Title: "Pull requests · GitHub", Process: "chrome"}},
	} {
		tgt := Describe(win(c.facts))
		got := PlanDelivery(tgt, false, false)
		if got.Action != "clipboard" || got.Cancel || got.Click {
			t.Errorf("a tray click landing on %s should only copy: %+v", c.what, got)
		}
		if !strings.Contains(got.Reason, "clipboard") {
			t.Errorf("a tray click landing on %s should say where it went: %q", c.what, got.Reason)
		}
		if tgt.Deliverable() {
			t.Errorf("%s is not deliverable", c.what)
		}
	}
}

// The desktop is the biggest thing on the screen and the one window that is never
// a drop target. The protection is not here, though: these classes never reach
// Describe at all — win.WindowAt filters them out while walking the z-order (see
// notAWindow in internal/win), so a drag over the desktop or the taskbar finds
// whatever real window is behind it, or finds nothing and never calls PlanDelivery
// (deliver() returns early on an empty Target — see lands() and b.spot.Label).
// What this package still owns is recognition: none of these should ever be
// mistaken for a chat or a terminal, in case they ever did reach here.
func TestTheShellAndTheDesktopAreNeverTargets(t *testing.T) {
	for _, facts := range []Window{
		{Title: "Program Manager", Class: "Progman", Process: "explorer"},
		{Class: "WorkerW", Process: "explorer"},
		{Class: "Shell_TrayWnd", Process: "explorer"},
		{Class: "SysListView32", Process: "explorer"},
	} {
		tgt := Describe(facts)
		if tgt.Deliverable() {
			t.Errorf("%s %s is not a chat", facts.Class, facts.Title)
		}
		// an unaimed tray click still respects that: it only types into a window
		// recognised outright, so it copies here too
		if got := PlanDelivery(tgt, false, false); got.Action != "clipboard" {
			t.Errorf("%s: %+v", facts.Class, got)
		}
	}
}

// The drop point is a guess — it can land on the transcript, a sidebar, anywhere in
// a chat's window that is not the box a note needs to go into. A recognised chat
// gets a second, better-aimed point near the bottom of its own window to fall back
// on; a terminal and a window we do not recognise as a chat get none, because we
// have no reason to believe anything about either one's layout.
func TestComposerGuessOnlyOffersAPointForARecognisedChat(t *testing.T) {
	chat := Describe(win(Window{Title: "Claude", Process: "claude"}))
	chat.Rect = Rect{X: 100, Y: 50, W: 800, H: 600}
	x, y, ok := chat.ComposerGuess()
	if !ok {
		t.Fatal("a recognised chat should offer a composer guess")
	}
	if x != 500 { // the horizontal middle of the window
		t.Errorf("x = %d, want the window's middle (500)", x)
	}
	bottom := chat.Rect.Y + chat.Rect.H
	if y <= chat.Rect.Y || y >= bottom {
		t.Errorf("y = %d is not inside the window (%d..%d)", y, chat.Rect.Y, bottom)
	}
	if y < bottom-200 {
		t.Errorf("y = %d is not close to the bottom edge (%d)", y, bottom)
	}

	terminal := Describe(win(Window{Class: "ConsoleWindowClass", Title: "Command Prompt", Process: "cmd"}))
	terminal.Rect = Rect{X: 0, Y: 0, W: 800, H: 600}
	if _, _, ok := terminal.ComposerGuess(); ok {
		t.Error("a terminal's caret follows its own prompt; it should not get a guessed click")
	}

	editor := Describe(win(Window{Title: "x - Notepad", Process: "notepad"}))
	editor.Rect = Rect{X: 0, Y: 0, W: 800, H: 600}
	if _, _, ok := editor.ComposerGuess(); ok {
		t.Error("a window we do not recognise as a chat should not get a guessed click")
	}

	// a chat with no known size (never actually found on screen) has nowhere to
	// guess a point inside
	if _, _, ok := (Target{AIChat: true}).ComposerGuess(); ok {
		t.Error("a chat with an empty rect has no composer to guess at")
	}
}

// PlanDelivery hands the composer guess along on every path that types, aimed or
// not — a tray click has no drop point to fall back from at all, so there the guess
// is the only click there is.
func TestPlanDeliveryCarriesTheComposerGuess(t *testing.T) {
	chat := Describe(win(Window{Title: "Claude", Process: "claude"}))
	chat.Rect = Rect{X: 0, Y: 0, W: 800, H: 600}

	dropped := PlanDelivery(chat, true, false)
	if !dropped.Composer {
		t.Errorf("an explicit drop onto a chat should carry a composer guess: %+v", dropped)
	}

	trayClick := PlanDelivery(chat, false, false)
	if !trayClick.Composer {
		t.Errorf("a tray click landing on a chat should carry a composer guess: %+v", trayClick)
	}

	terminal := Describe(win(Window{Class: "ConsoleWindowClass", Title: "Command Prompt", Process: "cmd"}))
	terminal.Rect = Rect{X: 0, Y: 0, W: 800, H: 600}
	if got := PlanDelivery(terminal, true, false); got.Composer {
		t.Errorf("a terminal should not carry a composer guess: %+v", got)
	}
}

func TestCopyOnlyAlwaysWins(t *testing.T) {
	chat := Describe(win(Window{Title: "ChatGPT", Process: "chrome"}))
	copy := PlanDelivery(chat, true, true)
	if copy.Action != "clipboard" || copy.Cancel {
		t.Fatalf("got %+v", copy)
	}
}

func TestATrayClickOnlyTypesIntoTerminalsAndChats(t *testing.T) {
	chat := Describe(win(Window{Title: "ChatGPT - Google Chrome", Process: "chrome"}))
	chatPlan := PlanDelivery(chat, false, false)
	if chatPlan.Action != "type" || chatPlan.Cancel {
		t.Fatalf("a tray click is not consent to interrupt: %+v", chatPlan)
	}
	terminal := Describe(win(Window{Class: "ConsoleWindowClass", Title: "claude", Process: "cmd"}))
	if PlanDelivery(terminal, false, false).Action != "type" {
		t.Fatalf("terminal should be typed into")
	}
	editor := Describe(win(Window{Title: "thing.ts - Visual Studio Code", Process: "Code"}))
	guarded := PlanDelivery(editor, false, false)
	if guarded.Action != "clipboard" || !strings.Contains(guarded.Reason, "clipboard") {
		t.Fatalf("got %+v", guarded)
	}
}

func TestWeOnlyInterruptWhenSomethingIsRunning(t *testing.T) {
	chat := Describe(win(Window{Title: "Claude", Process: "claude"}))

	yes := DecideCancel(chat, true, true, true, 1)
	if !yes.Cancel || yes.Key != "Escape" || !strings.Contains(yes.Reason, "generated") {
		t.Fatalf("got %+v", yes)
	}
	tray := DecideCancel(chat, false, true, true, 1)
	if tray.Cancel || !strings.Contains(tray.Reason, "dropped flower") {
		t.Fatalf("got %+v", tray)
	}
	quiet := DecideCancel(chat, true, true, false, 1)
	if quiet.Cancel || !strings.Contains(quiet.Reason, "nothing was running") {
		t.Fatalf("got %+v", quiet)
	}
	if disabled := DecideCancel(chat, true, false, true, 1); disabled.Cancel {
		t.Fatal("cancelling was switched off")
	}
	editor := Describe(win(Window{Title: "x - Notepad", Process: "notepad"}))
	if notCancellable := DecideCancel(editor, true, true, true, 1); notCancellable.Cancel {
		t.Fatal("an editor does not look interruptible")
	}
	if none := DecideCancel(Target{}, true, true, true, 1); none.Cancel {
		t.Fatal("no information is not a reason to interrupt")
	}
	// a terminal gets the stronger second answer, because Escape is not a
	// reliable interrupt in a shell
	term := Describe(win(Window{Class: "ConsoleWindowClass", Title: "claude", Process: "cmd"}))
	if second := DecideCancel(term, true, true, true, 2); second.Key != "ctrl+c" {
		t.Fatalf("got %+v", second)
	}
}

func TestTheChatMatcherIsCaseInsensitiveAndSurvivesOddTitles(t *testing.T) {
	cases := []struct {
		facts Window
		label string
	}{
		{Window{Title: "CHATGPT"}, "ChatGPT"},
		{Window{Title: "my claude session"}, "Claude"},
		{Window{Process: "Freebuff"}, "Freebuff"},
		{Window{Title: "notepad"}, ""},
		{Window{}, ""},
	}
	for _, c := range cases {
		got, ok := MatchChat(c.facts)
		if ok != (c.label != "") || got != c.label {
			t.Errorf("MatchChat(%+v) = %q/%v", c.facts, got, ok)
		}
	}
}

func TestWindowsTerminalsAreRecognisedByClass(t *testing.T) {
	for _, class := range []string{"ConsoleWindowClass", "CASCADIA_HOSTING_WINDOW_CLASS", "mintty"} {
		if !Classify(class, "").OK {
			t.Errorf("%s should be a terminal", class)
		}
	}
}

func TestTerminalTitlesAreRecognisedWhenTheClassIsUnknown(t *testing.T) {
	if !Classify("", "claude — whipping-not-required").OK {
		t.Error("a claude title is a terminal")
	}
	if !Classify("SomeEditor", "Windows PowerShell").OK {
		t.Error("a PowerShell title is a terminal")
	}
}

func TestAnEditorOrBrowserIsRefused(t *testing.T) {
	vscode := Classify("Chrome_WidgetWin_1", "main.js - GoodAi - Visual Studio Code")
	if vscode.OK || !strings.Contains(vscode.Reason, "not a terminal") {
		t.Fatalf("got %+v", vscode)
	}
	if Classify("Chrome_WidgetWin_1", "YouTube").OK {
		t.Error("a browser page is not a terminal")
	}
	if Classify("", "").OK {
		t.Error("nothing is not a terminal")
	}
}

// The app is the flower. The duck, the cookie, the coffee and the star were the
// flower wearing hats: they are gone, and this is what stops them creeping back
// in one at a time.
func TestTheTrayIsTheFlowerAndNothingElse(t *testing.T) {
	if len(Items) != 1 || Items[0].ID != "flower" {
		ids := make([]string, 0, len(Items))
		for _, item := range Items {
			ids = append(ids, item.ID)
		}
		t.Fatalf("the tray should hold the flower alone, and holds %v", ids)
	}
	if _, ok := ByID("duck"); ok {
		t.Error("the duck is retired")
	}

	seen := map[string]bool{}
	hex := regexp.MustCompile(`^#[0-9a-f]{6}$`)
	for _, item := range Items {
		if item.ID == "" || seen[item.ID] {
			t.Errorf("id %q is missing or repeated", item.ID)
		}
		seen[item.ID] = true
		for what, value := range map[string]string{
			"emoji": item.Emoji, "name": item.Name, "verb": item.Verb,
			"flavor": item.Flavor, "accent": item.Accent, "message": item.Message,
		} {
			if value == "" {
				t.Errorf("%s.%s is empty", item.ID, what)
			}
		}
		if strings.ContainsAny(item.Message, "\r\n") {
			t.Errorf("%s.message is not single line", item.ID)
		}
		if len([]rune(item.Message)) > 160 {
			t.Errorf("%s.message is too long to type", item.ID)
		}
		if !hex.MatchString(item.Accent) {
			t.Errorf("%s.accent %q is not a hex colour", item.ID, item.Accent)
		}
	}
}

func TestByIDFindsItemsAndRefusesStrangers(t *testing.T) {
	if item, ok := ByID("flower"); !ok || item.Name != "Flower" {
		t.Fatalf("got %+v", item)
	}
	if _, ok := ByID("whip"); ok {
		t.Error("the whip is retired")
	}
	if _, ok := ByID(""); ok {
		t.Error("empty id is nobody")
	}
}

func TestRandomAlwaysReturnsAnItemFromTheTray(t *testing.T) {
	for i := 0; i < 200; i++ {
		if item := Random(nil); item.ID == "" {
			t.Fatal("empty item")
		}
	}
	for _, r := range []float64{0, 0.5, 0.999999} {
		if got := Random(func() float64 { return r }); got.ID != "flower" {
			t.Fatalf("random %v gave %q", r, got.ID)
		}
	}
}
