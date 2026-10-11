package buddy

import (
	"math"

	"github.com/FaZeBuns/pocket-petal/internal/win"
)

// The highlighter: a warm ring that glows around whatever window the flower is
// being carried over.
//
// It is a second layered window, drawn by hand rather than by the flower's
// rasteriser, because a ring is not a flower: it is a rectangle of light that has
// to hug somebody else's window, and it has to be cheap enough to move every
// frame while a drag is in flight. It is click-through (WS_EX_TRANSPARENT) so a
// drop still lands on the window underneath, and it never takes focus.
//
// The light is the same colour as the sun in the flower's palette, so the ring
// reads as the same app rather than as a debugging rectangle.

const (
	ringMargin = 6  // how far outside the target the glow starts
	ringEdge   = 2  // the solid edge of the ring, in pixels
	ringFall   = 14 // how far the glow reaches, in pixels
	ringLimit  = 4096
)

// highlighter is the ring window and what it is currently around.
//
// The ring's own picture is kept, not rebuilt: it depends on nothing but the size
// of the window it is around, so a drag across a maximised chat — 1920x1040, eight
// megabytes of bitmap — costs one blit when the target changes and *nothing* per
// frame after that. Rebuilding and re-blitting that bitmap sixty times a second
// was the whole of the stutter people noticed exactly when the flower came over
// something worth dropping it on.
type highlighter struct {
	win     *win.Window
	rect    win.RECT
	have    bool
	visible bool
	pix     []uint8
	pixW    int
	pixH    int

	// A drag visits the same few sizes again and again — a maximised chat, a
	// restored one, a small dialog — and painting one is milliseconds of
	// arithmetic over megabytes. Keeping the last few pictures means the second
	// time you carry the flower over that window it costs a blit and nothing else.
	rings   []ringPic
	evicted int
}

// ringPic is one painted ring, kept by the size of the window it hugs.
type ringPic struct {
	w, h int
	pix  []uint8
}

// ringCacheMax is how many sizes are remembered. Four covers every shape a desk
// usually has; past that the oldest picture is dropped, which costs nothing but
// the paint it will need next time.
const ringCacheMax = 4

// picture is the ring for a size: the remembered one if there is one, otherwise a
// new one that replaces the oldest. It is only ever read (BlitAt copies it into the
// window's own bitmap), so evicting a picture the window is currently showing is
// safe.
func (h *highlighter) picture(w, hh int) []uint8 {
	for i := range h.rings {
		if h.rings[i].w == w && h.rings[i].h == hh {
			return h.rings[i].pix
		}
	}
	pix := ringRGBA(w, hh)
	if len(h.rings) < ringCacheMax {
		h.rings = append(h.rings, ringPic{w: w, h: hh, pix: pix})
		return pix
	}
	h.rings[h.evicted%ringCacheMax] = ringPic{w: w, h: hh, pix: pix}
	h.evicted++
	return pix
}

func newHighlighter(w, h int) (*highlighter, error) {
	// start with a plausible size; the window is resized to the target the first
	// time a drag finds one
	window, err := win.NewNamedLayeredWindow(w, h, win.HighlightClass, func(win.MSG) bool { return false })
	if err != nil {
		return nil, err
	}
	window.SetClickThrough(true)
	h2 := &highlighter{win: window}
	return h2, nil
}

// target is the window the ring should be around, or none for hiding it. It is
// called every frame of a drag, and does nothing at all unless something about the
// ring has actually changed.
func (h *highlighter) target(rect win.RECT) {
	if rect.Width() <= 0 || rect.Height() <= 0 {
		h.hide()
		return
	}
	// bigger than any screen, and a layered window's bitmap is fixed when it is
	// made: both the buffer and the window are clamped together, or the blit would
	// write past the end of the bitmap
	w := min(rect.Width()+ringMargin*2, ringLimit)
	ht := min(rect.Height()+ringMargin*2, ringLimit)
	if h.visible && h.have && h.rect == rect && h.pixW == w && h.pixH == ht {
		return
	}
	if h.pixW != w || h.pixH != ht {
		h.resize(w, ht)
		h.pix = h.picture(w, ht)
		h.pixW, h.pixH = w, ht
	}
	h.rect, h.have = rect, true
	h.win.BlitAt(int(rect.Left)-ringMargin, int(rect.Top)-ringMargin, h.pix, w, ht)
	if !h.visible {
		h.win.Show()
		h.visible = true
	}
}

func (h *highlighter) hide() {
	if h.visible {
		h.win.Hide()
		h.visible = false
	}
	h.have = false
}

// resize rebuilds the ring window at a new size. A layered window's bitmap is
// fixed at creation time, so a bigger target means a new window.
func (h *highlighter) resize(w, hh int) {
	h.win.Hide()
	h.visible = false
	h.win.Destroy()
	h.pix, h.pixW, h.pixH = nil, 0, 0
	w, hh = min(w, ringLimit), min(hh, ringLimit)
	if window, err := win.NewNamedLayeredWindow(w, hh, win.HighlightClass, func(win.MSG) bool { return false }); err == nil {
		window.SetClickThrough(true)
		h.win = window
	}
}

// ringRGBA paints the glow: a bright edge with the light falling off both ways,
// so it reads as a shine rather than as a selection rectangle.
func ringRGBA(w, h int) []uint8 {
	out := make([]uint8, w*h*4)
	// the sun in the flower's palette, plus a little of the bloom's pink
	const (
		goldR, goldG, goldB = 255, 214, 138
		edgeR, edgeG, edgeB = 255, 236, 196
	)
	for y := 0; y < h; y++ {
		// distance to the nearest horizontal edge of the ring's own band
		dy := math.Min(float64(y), float64(h-1-y))
		for x := 0; x < w; x++ {
			dx := math.Min(float64(x), float64(w-1-x))
			// distance from the band's centre line
			band := math.Min(dx, dy)
			d := math.Abs(band - ringEdge)
			var a float64
			switch {
			case d <= 0.5:
				a = 0.62
			case d < ringFall:
				// ease out, so the light has no edge of its own
				k := 1 - d/ringFall
				a = 0.62 * k * k
			}
			if a <= 0.002 {
				continue
			}
			// hotter at the very edge, cooler as it fades
			t := math.Min(1, a/0.62)
			r := goldR + (edgeR-goldR)*t
			g := goldG + (edgeG-goldG)*t
			b := goldB + (edgeB-goldB)*t
			o := (y*w + x) * 4
			out[o] = uint8(r)
			out[o+1] = uint8(g)
			out[o+2] = uint8(b)
			out[o+3] = uint8(a * 255)
		}
	}
	return out
}
