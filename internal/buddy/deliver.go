package buddy

import (
	"math"
	"time"

	"github.com/FaZeBuns/pocket-petal/internal/target"
	"github.com/FaZeBuns/pocket-petal/internal/win"
)

// Delivery: the flower is let go over someone else's window, and the note is
// typed into it.
//
// The whole feature rests on one idea: dropping the flower there is consent. We
// do not have to know which chat it is, or what framework it is built with,
// because we do not type into it through any of its own interfaces — we bring
// the window forward, put the caret where the flower landed, paste the note and
// press Enter. That is why a harness nobody has written a plugin for yet still
// works.
//
// Everything that touches another window happens off the frame loop, in order,
// with small waits between the steps: a chat box that is still coming forward
// will eat a paste, and an Enter that arrives in the same millisecond as the
// paste can send an empty line.

const (
	deliverSettle   = 420 * time.Millisecond // let go, and let the landing be seen
	deliverFocus    = 90 * time.Millisecond
	deliverInterupt = 70 * time.Millisecond
	deliverCaret    = 60 * time.Millisecond
	deliverPaste    = 110 * time.Millisecond
)

// opaquePixels counts how much of a frame would actually be seen, which is the
// quickest way to tell an empty render from a hidden window.
func opaquePixels(rgba []uint8) int {
	n := 0
	for i := 3; i < len(rgba); i += 4 {
		if rgba[i] > 8 {
			n++
		}
	}
	return n
}

// firstOpaque finds a pixel of the frame that a person would actually see, which
// is the only place worth asking Windows what is on top.
func firstOpaque(rgba []uint8, width int) (int, bool) {
	for i := 3; i < len(rgba); i += 4 {
		if rgba[i] > 200 {
			return i / 4, true
		}
	}
	return 0, false
}

// candidate is the window under the flower, worked out while dragging.
func (b *Buddy) candidate(x, y int) (target.Target, bool) {
	info, ok := win.WindowAt(x, y, b.pid)
	if !ok {
		return target.Target{}, false
	}
	t := target.Describe(target.Window{
		Hwnd:    uintptr(info.Hwnd),
		Title:   info.Title,
		Class:   info.Class,
		Process: info.Process,
	})
	t.Rect = target.Rect{X: int(info.Rect.Left), Y: int(info.Rect.Top), W: info.Rect.Width(), H: info.Rect.Height()}
	return t, true
}

// hover updates the glow around whatever the flower is over, and remembers it as
// the drop target so the ring and the delivery can never disagree.
//
// Looking is not free: naming the window under the cursor means walking the
// z-order and opening each process to find out what it is, which is a few hundred
// microseconds and a syscall or two per window. Sixty times a second while a hand
// is moving is real work for an answer that almost never changes, so the look is
// taken again when the cursor has moved somewhere new, or when a quarter of a
// second has gone by — and the ring itself costs nothing at all between looks.
func (b *Buddy) hover() {
	if b.high == nil {
		return
	}
	cursor := win.CursorPosition()
	x, y := int(cursor.X), int(cursor.Y)
	now := time.Now()
	moved := math.Hypot(float64(x-b.spotAtX), float64(y-b.spotAtY))
	if moved > hoverLook && now.Sub(b.spotAt) > hoverEvery {
		b.lookAt(x, y)
	}
	// the ring is only drawn around a window the cursor is *inside*: a drop over the
	// empty desktop has nothing to glow around, and a ring left glowing around the
	// window you have already dragged away from is worse than no ring. Any other
	// window gets the ring — a drop types into whatever is under it (see
	// target.PlanDelivery), so the ring has to promise exactly that and nothing more
	rect := win.RECT{}
	if b.spot.Label != "" && b.spot.Rect.Contains(x, y) {
		rect = win.RECT{
			Left:   int32(b.spot.Rect.X),
			Top:    int32(b.spot.Rect.Y),
			Right:  int32(b.spot.Rect.X + b.spot.Rect.W),
			Bottom: int32(b.spot.Rect.Y + b.spot.Rect.H),
		}
	}
	b.high.target(rect)
}

// lookAt takes the look itself: what is under this point, remembered as the drop
// target. It is the only place b.spot is written, so the ring and the delivery can
// never disagree about where the flower would land.
//
// A drop calls it directly, without the drag's throttle, because the note has to
// go to whatever is under the flower *now* rather than to whatever was there a
// frame ago.
func (b *Buddy) lookAt(x, y int) {
	b.spotAt, b.spotAtX, b.spotAtY = time.Now(), x, y
	b.spot = target.Target{}
	if t, ok := b.candidate(x, y); ok {
		b.spot = t
	}
}

// How the target under the cursor is looked up while a drag is in flight.
const (
	hoverLook  = 6.0                   // pixels of movement that count as "somewhere new"
	hoverEvery = 40 * time.Millisecond // and how often to look again in any case
)

// lands reports whether a drop onto this window is one the chime is heard for: a chat
// it recognised, and nothing else.
//
// This is deliberately narrower than "somewhere a note goes". A note is also delivered
// into a terminal, where the coding agent lives, and into anything a person insists on
// by hand — but the chime belongs to the chat, so letting go anywhere else is silent:
// over a terminal, an editor, a page or the desktop the bloom still comes apart, and
// the sound of it is the one thing the app is allowed to say, so it says nothing.
func lands(t target.Target) bool { return t.AIChat }

// deliver is the drop: work out what is underneath, then say the note into it.
func (b *Buddy) deliver() {
	if b.high != nil {
		b.high.hide()
	}
	if b.spot.Label == "" {
		return
	}
	if lands(b.spot) {
		b.dropSound()
	}
	note := target.Random(b.rng.Float64)
	if item, ok := target.ByID(b.cfg.Note); ok {
		note = item
	}
	// the words on the note are made fresh every time: a compliment nobody has
	// read before, and the clause that keeps the chat going (see target.Note)
	text := target.Note(b.rng)
	plan := target.PlanDelivery(b.spot, true, b.cfg.CopyOnly)
	// the settings have the last word on what the plan may do
	if !b.cfg.Interrupt {
		plan.Cancel = false
	}
	if !b.cfg.Caret {
		plan.Click = false
	}
	if b.cfg.Dry {
		b.log("dry run — %s on %q (%s): %q", plan.Action, b.spot.Label, plan.Reason, text)
		return
	}
	if plan.Action == "clipboard" {
		win.SetClipboardText(text)
		b.log("copied the note for %q", b.spot.Label)
		return
	}

	t := b.spot
	// the very first drop of a run gets every wait stretched: see
	// firstDeliveryDone. Read and set before the goroutine starts, not inside it
	// — two drops let go within the same instant must not both think they are
	// the first
	isFirst := !b.firstDeliveryDone
	b.firstDeliveryDone = true
	patience := time.Duration(1)
	if isFirst {
		patience = 3
	}
	b.log("delivering to %q at rect %+v (plan: click=%v composer=%v cancel=%v, first=%v)",
		t.Label, t.Rect, plan.Click, plan.Composer, plan.Cancel, isFirst)
	// the flower gets out of its own way: the click that hands over the caret has
	// to land *through* it, on the chat behind
	b.win.SetClickThrough(true)
	go func() {
		defer b.win.SetClickThrough(false)

		time.Sleep(deliverSettle * patience)
		method := win.Focus(win.HWND(t.Hwnd))
		time.Sleep(deliverFocus * patience)
		// a window that has never had its focus stolen by us before — most often
		// the very first drop right after the flower starts, onto a chat nobody
		// has clicked into yet — can take Windows longer to hand over than the
		// usual wait: proceeding to click and paste before it actually has means
		// both land wherever focus really ended up, not on the chat at all. One
		// retry with a longer wait is what a person does by hand when they focus
		// the window themselves first and try again; this is the same fix, done
		// without asking for it
		if win.Foreground() != win.HWND(t.Hwnd) {
			b.log("focus (%s) did not take first try, retrying", method)
			method = win.Focus(win.HWND(t.Hwnd))
			time.Sleep(deliverFocus * 3 * patience)
		}
		if win.Foreground() != win.HWND(t.Hwnd) {
			b.log("focus (%s) still has not landed on %q; fore=%v want=%v — continuing anyway",
				method, t.Label, win.Foreground(), win.HWND(t.Hwnd))
		}

		// an explicit drop onto a chat we can interrupt means "stop talking, read
		// this": Escape is the interrupt every AI chat understands
		if plan.Cancel {
			win.KeyPress(win.VK_ESCAPE)
			time.Sleep(deliverInterupt * patience)
		}
		if plan.Click {
			cursor := win.CursorPosition()
			b.log("clicking the drop point at %d,%d", cursor.X, cursor.Y)
			win.ClickHere()
			time.Sleep(deliverCaret * patience)
		}
		// the drop point is a guess at best — it can land on the transcript, a
		// sidebar, anywhere in the window that is not the box the note needs to go
		// into — so a recognised chat gets a second, better-aimed click near the
		// bottom of its own window, where a compose box almost always lives (see
		// target.Target.ComposerGuess). Clicking an already-focused box a second
		// time only moves the caret within it, so this costs nothing when the first
		// click already found it
		if plan.Composer {
			b.log("clicking the composer guess at %d,%d", plan.ComposerX, plan.ComposerY)
			win.MoveCursor(plan.ComposerX, plan.ComposerY)
			win.ClickHere()
			time.Sleep(deliverCaret * patience)
		}
		if !win.SetClipboardText(text) {
			b.log("could not reach the clipboard, note not delivered")
			return
		}
		win.CtrlKey(win.VK_V)
		time.Sleep(deliverPaste * patience)
		if b.cfg.AutoSend {
			win.KeyPress(win.VK_RETURN)
		} else {
			b.log("the note is pasted, press Enter when you are ready")
		}
		b.delivered.Store(time.Now().UnixNano())
		b.log("%s delivered to %q (focus %s)", note.Emoji, t.Label, method)
	}()
}

// letGo is the flower being put down.
//
// Which part of the flower the window is placed by changes the moment the hand lets
// go. While it is carried, the anchor is the piece that was taken hold of; standing
// still, the anchor is the *base of the stem*. Both are the same rule — the anchor
// lands on the hand — so letting go without moving the hand means the anchor changes
// underneath the flower and the whole thing is dragged down to the cursor: a flower
// let go of halfway up its stem leaps up, with its feet under the mouse. That is
// exactly the wrong way round, and exactly what it looked like.
//
// The reconciliation is one line: when the hand lets go, the hand becomes the base
// of the stem, which is the point a standing flower is placed by — so the flower
// stays precisely where it was standing. The walk home starts from the same place
// for the same reason (see homeward).
func (b *Buddy) letGo() {
	b.dragging = false
	x, y := b.standingPoint()
	b.handX, b.handY = float64(x), float64(y)
}

// homeward starts the walk back to the corner after a delivery, so the flower is
// where it belongs by the time you look again.
//
// The walk carries the base of the stem — that is what a flower stands on — and it
// starts from where that base is right now, so the flower does not jump when the
// journey begins.
func (b *Buddy) homeward() {
	b.walking = true
	x, y := b.standingPoint()
	b.handX, b.handY = float64(x), float64(y)
}

// startBurst sets the bloom coming apart on the next frame, where the flower's
// own petals are known: how hard it goes depends on how fast the flower was
// travelling when it was let go, and which way the petals fly is the way you
// were going.
//
// It belongs to *letting go of the flower*, not to the note being typed: the
// explosion is the flower's answer to being dropped, and a flower that stayed
// whole because the window underneath happened not to be a chat would be the
// animation punishing you for an honest drag. Whether a note is delivered is a
// separate question, answered further down (see PlanDelivery).
func (b *Buddy) startBurst() {
	if !b.cfg.Burst || b.burst != nil {
		return
	}
	state := b.carry.State()
	speed := math.Hypot(state.Fvx, state.Fvy)
	b.burstPower = 0.8 + 0.9*math.Min(1, speed/1900)
	b.burstVX = state.Fvx
	b.burstVY = state.Fvy
	// the flower answers its own explosion: it takes the recoil, and the bloom
	// keeps swinging on its neck for a second afterwards. A drop that the flower
	// slept through would be a slideshow; this is the difference
	b.carry.Impact(0.8)
	b.carry.Flick(-b.burstVX*0.35, -b.burstVY*0.35, 1.0)
	b.burstPending = true
}
