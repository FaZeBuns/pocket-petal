package target

import (
	"regexp"
	"strings"
)

// Window is what the Win32 layer can tell us about someone else's window.
type Window struct {
	Hwnd    uintptr
	Title   string
	Class   string // window class name
	Process string // process image name, e.g. "chrome"
}

// Rect is a window rectangle in physical screen pixels.
type Rect struct{ X, Y, W, H int }

// Contains reports whether a point is inside the rectangle. The drag asks this
// every frame: the ring may only ever be drawn around a window the cursor is
// actually over, or a stale target ends up glowing at you from across the screen.
func (r Rect) Contains(x, y int) bool {
	return r.W > 0 && r.H > 0 && x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Target is a window described in the terms delivery needs.
type Target struct {
	Window
	Rect        Rect
	Kind        string // terminal | ai-chat | browser | app | unknown
	Label       string // what a human would call it
	Browser     bool
	Terminal    bool
	AIChat      bool
	Cancellable bool
}

// chatApp is one known-good AI, in the reference's order.
//
// Two patterns, because a name means different things in the two places a window
// can be. `any` matches anywhere in a window's own title, which is how a terminal
// running a coding agent names itself ("my claude session"). `whole` matches an
// entire segment of a browser page's title, which is the much stricter question
// "is this page the chat, or a page about it?". Delivering into a browser is
// delivering into a web page, so a browser only ever counts when the page *is* the
// chat: a search for claude, a video called "Claude vs ChatGPT", a blog post about
// Claude are pages about something, and typing a note into one is exactly the
// accident this distinction exists to prevent.
//
// `web` says whether the same name is a page you can open in a browser at all.
// ChatGPT is; Cursor, VS Code and the CLI agents are not, so a browser tab called
// "Cursor" is a page about the editor rather than the editor.
//
// Naming is only half of it: a drop is what decides whether the note is typed at
// all (see PlanDelivery), so a chat nobody has heard of still works when you drag
// the flower onto it by hand. This list is what the caption says, and what the
// ring glows around on the way.
type chatApp struct {
	any   *regexp.Regexp
	whole *regexp.Regexp
	label string
	web   bool
}

func chat(pattern, label string, web bool) chatApp {
	return chatApp{any: ci(pattern), whole: ci(`^(?:` + pattern + `)$`), label: label, web: web}
}

var chatApps = []chatApp{
	chat(`antigravity`, "Antigravity", false),
	chat(`chat\s?gpt|openai|\bcodex\b`, "ChatGPT", true),
	chat(`claude|anthropic`, "Claude", true),
	chat(`gemini|\bbard\b`, "Gemini", true),
	chat(`deepseek|deep seek`, "DeepSeek", true),
	chat(`kimi|moonshot`, "Kimi", true),
	chat(`\bqwen|tongyi`, "Qwen", true),
	chat(`\bglm\b|zhipu|chatglm`, "GLM", true),
	chat(`mistral|le chat`, "Le Chat", true),
	chat(`perplexity`, "Perplexity", true),
	chat(`copilot`, "Copilot", true),
	chat(`grok|x\.ai`, "Grok", true),
	// apps that are their own window: an editor, an agent's own desktop client,
	// an AI that lives on your machine
	chat(`cursor`, "Cursor", false),
	chat(`windsurf|codeium`, "Windsurf", false),
	chat(`\bwarp\b`, "Warp", false),
	chat(`\bzed\b`, "Zed", false),
	chat(`freebuff|codebuff`, "Freebuff", false),
	chat(`opencode|\bcrush\b|\bgoose\b|\bdroid\b|\bcline\b|roo code|aider`, "your coding agent", false),
	chat(`lm\s?studio|jan\.ai|open ?webui|librechat|ollama|msty|gpt4all|anythingllm`, "your local model", true),
	chat(`\bchatbox\b|\bchat ?ui\b`, "your AI", true),
}

// segments splits a window title the way the shell writes it: "Flower physics -
// ChatGPT" is two things — a page, and the app it is open in.
func segments(title string) []string {
	out := []string{}
	for _, part := range separator.Split(title, -1) {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

var separator = ci(`\s+[·|–—-]\s+`)

// agents that live inside a terminal, named from the window title.
var terminalAgents = []struct {
	re    *regexp.Regexp
	label string
}{
	{ci(`claude`), "Claude Code"},
	{ci(`codex`), "Codex CLI"},
	{ci(`gemini`), "Gemini CLI"},
	{ci(`opencode`), "opencode"},
	{ci(`aider`), "Aider"},
	{ci(`deepseek`), "DeepSeek"},
	{ci(`qwen`), "Qwen Code"},
	{ci(`\bcrush\b`), "Crush"},
	{ci(`\bgoose\b`), "Goose"},
	{ci(`\bdroid\b`), "Droid"},
	{ci(`kimi`), "Kimi CLI"},
	{ci(`copilot`), "Copilot CLI"},
}

// NB: every Electron app (VS Code, Slack, Cursor...) uses the window class
// Chrome_WidgetWin_1, so a browser is identified by its process, or by the
// browser name at the end of the title — never by the class name.
var browsers = ci(`^(chrome|chromium|msedge|edge|firefox|brave|opera|vivaldi|arc|safari)$`)

var browserInTitle = ci(`\s[-–—]\s+(?:google\s+|mozilla\s+|microsoft\s+)?(chrome|chromium|edge|firefox|brave|opera|vivaldi|arc|safari)\b`)

var browserSuffix = ci(`\s+[-–—]\s+(?:google\s+|mozilla\s+|microsoft\s+)?(chrome|chromium|edge|firefox|brave|opera|vivaldi|arc|safari).*$`)

var unreadCount = ci(`\s*\((\d+)\)\s*$`)

var spaces = ci(`\s+`)

// TidyTitle turns "ChatGPT - Google Chrome - Work Profile" into "ChatGPT": the
// page, not the browser it happens to be open in.
func TidyTitle(title string) string {
	t := browserSuffix.ReplaceAllString(title, "")
	t = unreadCount.ReplaceAllString(t, "")
	t = strings.TrimSpace(spaces.ReplaceAllString(t, " "))
	return t
}

// PrettyProcess names a process the way a human would.
func PrettyProcess(name string) string {
	raw := strings.TrimSpace(strings.NewReplacer(".exe", "", "_", " ", "-", " ").Replace(strings.ToLower(name)))
	if raw == "" {
		return ""
	}
	switch raw {
	case "code", "code insiders":
		return "VS Code"
	case "windows terminal", "windowsterminal", "wt":
		return "Windows Terminal"
	}
	return strings.ToUpper(raw[:1]) + raw[1:]
}

// MatchChat finds the known AI a window looks like, if any: its own window, named
// by itself, wherever in the title the name appears.
func MatchChat(w Window) (string, bool) {
	for _, hay := range []string{w.Title, w.Class, w.Process} {
		if hay == "" {
			continue
		}
		for _, app := range chatApps {
			if app.any.MatchString(hay) {
				return app.label, true
			}
		}
	}
	return "", false
}

// MatchPage finds the AI chat a *browser* window is showing, if any.
//
// It reads the page, never the whole title: the name has to be one of the title's
// own segments, whole. So "ChatGPT" and "Flower physics - ChatGPT" are the chat,
// while "Why ChatGPT lies - The Verge", "Claude vs ChatGPT - YouTube" and
// "bash - Google Search" are pages that happen to mention a word. A browser is the
// one place where a name in a title is evidence of nothing much, and a flower that
// glows over a Google search result is worse than one that stays quiet.
func MatchPage(w Window) (string, bool) {
	for _, part := range segments(TidyTitle(w.Title)) {
		for _, app := range chatApps {
			if app.web && app.whole.MatchString(part) {
				return app.label, true
			}
		}
	}
	return "", false
}

// TerminalAgentLabel names the agent running in a terminal, if the title says.
func TerminalAgentLabel(title string) string {
	for _, agent := range terminalAgents {
		if agent.re.MatchString(title) {
			return agent.label
		}
	}
	return ""
}

const labelLimit = 42

func trimLabel(label string) string {
	if len([]rune(label)) > labelLimit {
		return string([]rune(label)[:labelLimit-3]) + "…"
	}
	return label
}

// Describe is everything we can tell about a window, in the terms the delivery
// flow needs.
func Describe(w Window) Target {
	title := TidyTitle(w.Title)
	processName := strings.TrimSuffix(strings.ToLower(w.Process), ".exe")

	// Order matters. A real terminal *class* wins outright (conhost, Windows
	// Terminal, mintty...). But an unknown class with a title like "Claude" or
	// "ChatGPT" is an AI chat, not a terminal: the title hints are only a
	// fallback for terminals we cannot recognise by class.
	terminalByClass := Classify(w.Class, "").OK
	// a browser is recognised before anything is matched inside it, because what a
	// name in a browser title is worth depends on it: see MatchPage
	browser := browsers.MatchString(processName) || browserInTitle.MatchString(w.Title)
	chat, isChat := "", false
	if !terminalByClass {
		if browser {
			chat, isChat = MatchPage(w)
		} else {
			chat, isChat = MatchChat(w)
		}
	}
	// A title only ever *hints* at a terminal when nothing else could account for
	// the window. In a browser it cannot: a page about bash, wsl or ubuntu is a
	// page, and "bash - Google Search" glowing as a place to deliver a note is the
	// kind of thing that makes the whole app feel like it is guessing.
	terminal := terminalByClass || (!isChat && !browser && Classify("", w.Title).OK)

	pretty := PrettyProcess(processName)
	kind := "unknown"
	label := orDefault(pretty, "that window")
	switch {
	case terminal:
		kind = "terminal"
		switch {
		case TerminalAgentLabel(title) != "":
			// name the agent if the title mentions one, so the caption can say it
			label = TerminalAgentLabel(title)
		case title != "":
			label = trimLabel("terminal — " + title)
		default:
			label = "your terminal"
		}
	case isChat:
		kind = "ai-chat"
		if browser {
			label = chat + " in " + orDefault(pretty, "your browser")
		} else {
			label = chat
		}
	case browser:
		kind = "browser"
		if title != "" {
			label = title + " — " + orDefault(pretty, "browser")
		} else {
			label = orDefault(pretty, "your browser")
		}
	case processName != "":
		kind = "app"
	}

	return Target{
		Window:   w,
		Kind:     kind,
		Label:    label,
		Browser:  browser,
		Terminal: kind == "terminal",
		AIChat:   kind == "ai-chat",
		// Escape interrupts a generation in essentially every AI chat and coding
		// agent; in a terminal it interrupts the running agent without killing
		// the shell.
		Cancellable: kind == "ai-chat" || kind == "terminal",
	}
}
