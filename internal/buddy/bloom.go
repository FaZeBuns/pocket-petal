package buddy

import (
	"math"

	"github.com/FaZeBuns/pocket-petal/internal/flower"
	"github.com/FaZeBuns/pocket-petal/internal/win"
)

// Where the petals go when the flower comes apart.
//
// A burst is much bigger than a flower, and much less like a box. It is thrown
// from the middle of the bloom, spreads outwards, sinks, and is over in about a
// second — so the picture it makes is a cloud that drifts down and out, and the
// flower's own window (which is exactly as big as the poses the carry can reach)
// would slice it off square at the top and the sides. The petals therefore fly in
// a surface of their own:
//
//   - the surface is as large as the whole flight could ever be, allocated once
//     and kept;
//   - each frame draws only the region the pieces are actually in, into the corner
//     of that surface, and shows it through a window the size of that region.
//
// The two are what make an explosion impossible to cut off without paying for a
// huge window every frame: the surface is sized from the flight (see
// flower.Burst.Flight, which measures it rather than guessing), and the frame only
// ever costs the area the petals are in.

// burstReach is the clamp on a throw, in the flower's own design units: how far
// from the middle of the bloom a burst may ever be *allowed* to draw. A flower flung
// across the desk would otherwise ask for a surface the size of the desk, so the
// flight is fitted to this and a harder throw flies a little less far instead of
// being cut off.
//
// It is a ceiling, not a size. What the surface is actually built from is the
// measured flight (see burstSurfacePixels) — the petals go where the throw sends
// them, and the window follows them, so a big round explosion costs a big round
// window and nothing more.
const burstReach = 1400.0

// burstPetals is how many pieces a delivery throws. Enough to fill the air and
// read as an explosion at a glance, few enough that they are still petals.
const burstPetals = 132

// burstPad is how much air is left around the pieces in the region that is drawn,
// in pixels: a petal's antialiased edge and a rounded pixel must not be the reason
// that a piece looks cut off.
const burstPad = 3

// burstVistaPixels is the surface a whole *budget* would need at this scale: the
// worst case the clamp allows, across the middle of the bloom, plus a little air.
// It is the number the surface must never have to exceed, and the tests hold the
// measured flight to it.
func burstVistaPixels(scale float64) int {
	return int(math.Ceil(2*burstReach*scale)) + 2*burstPad
}

// burstSurfacePixels is the surface a particular flight needs: the box the burst
// measured as its own (see flower.Burst.Flight), in pixels, plus the same thread of
// air a frame pads its region with.
//
// Sizing from the flight rather than from the clamp is what lets the explosion be
// big: the clamp is a square two reaches on a side taking no account of which way
// the petals went, and the measured box is the shape they actually make. The window
// a frame hands Windows is still only the part the pieces are in, so this is what
// the surface is allocated at once and never paid for again.
func burstSurfacePixels(tr flower.Transform, box flower.Bounds) (int, int) {
	air := float64(burstPad) / tr.Scale
	w := int(math.Ceil((box.Width()+2*air)*tr.Scale)) + 2
	h := int(math.Ceil((box.Height()+2*air)*tr.Scale)) + 2
	return max(1, w), max(1, h)
}

// burstRegion is where the pieces are *right now*: the box they are in, widened by
// a thread of air, and the pixel rectangle that shows it on screen.
//
// The pixel rectangle is the design box mapped through the flower's own transform
// — the same one the flower is drawn with — so a petal is the size a petal is, and
// the rectangle the surface is asked to show is exactly the part of the flight
// that has something in it. Since the box is built from the pieces themselves,
// nothing can be left out of it, which is what "never cut off" means.
func burstRegion(tr flower.Transform, box flower.Bounds) (px, py, width, height int, padded flower.Bounds) {
	air := float64(burstPad) / tr.Scale
	padded = flower.Bounds{
		MinX: box.MinX - air, MaxX: box.MaxX + air,
		MinY: box.MinY - air, MaxY: box.MaxY + air,
	}
	left, top := tr.ToPixel(padded.MinX, padded.MaxY)
	px, py = int(math.Floor(left)), int(math.Floor(top))
	width = max(1, int(math.Round(padded.Width()*tr.Scale))+2)
	height = max(1, int(math.Round(padded.Height()*tr.Scale))+2)
	return px, py, width, height, padded
}

// burstSurface is the window the petals fly in: as big as the whole flight could
// ever be for the flower as it is currently sized, made once and then kept.
//
// A layered window's bitmap is fixed when the window is made, so the surface is
// only rebuilt when it needs to be *bigger* — a flower grown, or the window
// reframed — and never to be smaller, because a surface with room to spare costs
// nothing: nothing outside the region each frame hands Windows is ever shown.
func (b *Buddy) burstSurface(w, h int) (*win.Window, error) {
	if b.petals != nil && b.petals.W >= w && b.petals.H >= h {
		return b.petals, nil
	}
	if b.petals != nil {
		b.petals.Destroy()
		b.petals = nil
	}
	window, err := win.NewNamedLayeredWindow(w, h, win.BurstClass, func(win.MSG) bool { return false })
	if err != nil {
		return nil, err
	}
	window.SetClickThrough(true)
	b.petals = window
	return window, nil
}

// throwPetals throws a shower of small petals in the flower's own colour.
//
// The flower itself is left exactly as it is: nothing comes off it, nothing stops
// being drawn. The burst is what a delivery looks like, not what it costs.
func (b *Buddy) throwPetals(pose flower.Pose) {
	centre := flower.Point{X: pose.Head.X, Y: pose.Head.Y}
	pieces := flower.Confetti(b.rig, centre, burstPetals, b.rng)
	if len(pieces) == 0 {
		return
	}
	b.burst = flower.NewBurst(pieces, b.rng, b.burstPower, b.burstVX, b.burstVY)
	// where it will go, and the promise that it fits: the flight is measured here,
	// once, the throw is scaled to the budget if it would not, and the box it will
	// draw in is kept — that box is what the petals' surface is then built from
	b.burstBox = b.burst.Flight(burstReach)
}

// drawPetals puts the pieces in flight on screen, in their own surface, and takes
// that window away once the last of them has fallen.
//
// They are drawn *behind* the flower: a petal should look like it came off the
// bloom and is falling past it, not like a sticker laid over the top.
func (b *Buddy) drawPetals() {
	if b.burst == nil {
		return
	}
	if b.burst.Done() {
		b.burst, b.burstBox = nil, flower.Bounds{}
		if b.petals != nil {
			b.petals.Hide()
		}
		return
	}
	// the same design space the flower is drawn in, at the same scale, so a petal
	// is the size a petal is
	tr := flower.MakeTransform(b.box, b.win.W, b.win.H, 0)
	px, py, width, height, box := burstRegion(tr, b.burst.Bounds())
	x, y := b.posX+px, b.posY+py

	// the surface is the whole flight this burst will make, so the frame that
	// draws it can never want more room than there is
	vistaW, vistaH := burstSurfacePixels(tr, b.burstBox)
	if b.burstBox.Width() <= 0 || b.burstBox.Height() <= 0 {
		vistaW, vistaH = burstVistaPixels(tr.Scale), burstVistaPixels(tr.Scale)
	}
	window, err := b.burstSurface(vistaW, vistaH)
	if err != nil {
		b.trace("could not open a surface for the petals: %v", err)
		b.burst, b.burstBox = nil, flower.Bounds{}
		return
	}
	if width > window.W || height > window.H {
		// unreachable: the region is inside the flight, and the surface is the
		// flight. If it ever happens, drawing the wrong part of a burst is worse
		// than drawing none of it, so nothing is drawn and the reason is said.
		b.trace("the petals want %dx%d in a %dx%d surface", width, height, window.W, window.H)
		b.burst, b.burstBox = nil, flower.Bounds{}
		return
	}

	// One subsample rather than two, on purpose: this is never bigger than the
	// flight, it is only ever on screen while pieces are in the air — moving fast,
	// small and fading — and paying for vertical antialiasing across all that air
	// is what would make an explosion stutter. A stuttering explosion is the one
	// thing it must not be.
	rgba := flower.Rasterize(b.burst.Shapes(), flower.RasterOptions{
		Width:      width,
		Height:     height,
		Bounds:     box,
		SubSamples: 1,
		Workers:    b.cfg.Workers,
	})
	if err := window.BlitRegionAt(x, y, 0, 0, rgba, width, height); err != nil {
		b.trace("%v", err)
		return
	}
	window.Show()
	// and the flower stays in front of them
	b.win.Raise()
}
