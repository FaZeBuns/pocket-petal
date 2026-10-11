package buddy

import (
	"math"

	"github.com/FaZeBuns/pocket-petal/internal/flower"
	"github.com/FaZeBuns/pocket-petal/internal/win"
)

// Where the flower stands on a screen of any size, and what happens to that when the
// screen changes.
//
// Both are pure functions of the work area, the flower's own outline and the display's
// scaling, and they are here rather than inside placeHome because they are the part
// worth arguing about with numbers: the same corner has to mean the same *place* at
// 720p, at 4K and at 8K, the ground has to be the top of the taskbar at all of them,
// the flower and its panel have to be the same share of the desk at all of them, and a
// spot somebody picked with their own hands has to still be their spot after the
// resolution changes. See place_test.go.

// standPoint is the base of the stem for a flower standing in a corner.
//
// rest is the flower's resting outline in *design* units and scale is design units to
// pixels at the size it is standing (see Buddy.pixelScale), so everything below is in
// screen pixels: the margin scales with the display, which is what keeps it looking the
// same on a 4K monitor at 150% as on a 1080p one at 100%.
//
// Two rules are worth stating, because they are the whole of how it looks right at
// every size:
//
//   - The ground has no margin. The work area stops where the taskbar begins, so its
//     bottom edge *is* the top of the taskbar and the flower stands on it: measured
//     from the flower's own outline rather than from the base of the stem, so whatever
//     shape it is grown into stands on the same line — a taller flower is taller, not
//     higher off the ground.
//   - The sides and the top *do* have a margin, so a corner flower is a flower beside
//     the edge of the screen rather than one sliced by it.
func standPoint(corner string, area win.WorkArea, rest flower.Bounds, scale float64) (int, int) {
	margin := float64(marginPixels) * scale
	left0, top0 := float64(area.Left), float64(area.Top)
	right0, bottom0 := float64(area.Right), float64(area.Bottom)
	left := left0 + margin - rest.MinX*scale
	right := right0 - margin - rest.MaxX*scale
	// +y is up the stem and pixel y is down the screen, which is why the top of the
	// flower is the base *plus* how far the stem reaches
	top := top0 + margin + rest.MaxY*scale
	bottom := bottom0 + rest.MinY*scale

	x, y := right, bottom
	switch corner {
	case CornerBottomLeft:
		x = left
	case CornerTopRight:
		y = top
	case CornerTopLeft:
		x, y = left, top
	}
	// a screen too small for the flower at all: the work area wins over the margin,
	// and the flower is kept inside it rather than pushed off the edge
	return int(math.Round(math.Min(math.Max(x, left0), right0))),
		int(math.Round(math.Min(math.Max(y, top0), bottom0)))
}

// marginPixels is the air between the flower and the edge of the work area, at a
// display scaling of 1. It is scaled by standPoint, so the gap looks the same to a
// person at every DPI.
const marginPixels = 26

// sameSpot keeps a place somebody picked with their own hands at the same *relative*
// place when the screen changes size: the spot is where they put the flower on their
// desk, so shrinking the display should put it in the same part of the screen rather
// than leave it hanging off the edge in pixels that no longer exist. Anything that is
// still off the work area afterwards is clamped by the caller (clampToDesk).
func sameSpot(x, y int, from, to win.WorkArea) (int, int) {
	if from.Width() <= 0 || from.Height() <= 0 {
		return x, y
	}
	shareX := float64(x-int(from.Left)) / float64(from.Width())
	shareY := float64(y-int(from.Top)) / float64(from.Height())
	return int(to.Left) + int(math.Round(shareX*float64(to.Width()))),
		int(to.Top) + int(math.Round(shareY*float64(to.Height())))
}

// referenceHeight is the screen height this app's measurements were chosen on: a 1080p
// desktop. Every size in here — the flower's box, the settings panel's plate, the air
// around a corner flower — is a number of pixels on a screen this tall, and everything is
// drawn at the ratio between that and the screen it is actually standing on.
const referenceHeight = 1080

// deskScaleFor is the scaling everything is drawn at for a screen: how tall that screen
// is against the reference. A 720p screen gives 0.67, a 1080p one 1.0, a 2K one 1.33, a
// 4K one 2.0 and an 8K one the cap of 4.0.
//
// It is the screen's own height rather than Windows' DPI setting, and that is the point:
// what a person notices is that the flower and its settings panel are the same size on the
// desk — the same share of the screen, in the same corner, the same distance from it —
// and the share is what this keeps. Windows' scaling is a statement about the size of
// somebody's *text*; the flower is not text, and scaling it by DPI alone means that
// changing the resolution under it (the same monitor, the same 100%, a different number of
// pixels) leaves the flower at the same pixel size and therefore twice the flower.
//
// On the setups where the two rules agree — 4K at 200%, 8K at 400%, 1080p at 100%, which is
// how these screens are almost always used — this and the DPI give the same answer.
func deskScaleFor(screen win.WorkArea) float64 {
	if h := screen.Height(); h > 0 {
		return clampScale(float64(h) / referenceHeight)
	}
	return clampScale(win.SystemScale())
}

// displayScale is the scaling the screen the flower is standing on is drawn at, read
// again whenever the screen changes (see refitDisplay). The window's own DPI is the
// last resort, for a machine that will not say how big its screen is.
func (b *Buddy) displayScale() float64 {
	if screen := win.PrimaryScreen(); screen.Height() > 0 {
		return deskScaleFor(screen)
	}
	if b.win != nil {
		if scale := win.ScaleForWindow(b.win.Hwnd); scale > 0 {
			return clampScale(scale)
		}
	}
	return clampScale(win.SystemScale())
}

// panelScale is the scale the settings panel is drawn at, which is the flower's own: the
// plate and the flower are things on the same desk, and a panel that ignored the display
// would be a postage stamp on an 8K screen and a billboard on a 720p one (see menuState).
func panelScale(b *Buddy) float64 {
	if b == nil {
		return 1
	}
	if b.deskScale > 0 {
		return clampScale(b.deskScale)
	}
	return 1
}

// clampScale keeps a scaling factor sane. Half and four are not displays anybody has, and
// a number that is not a number — a screen that will not say how big it is — is the
// reference scale: the flower is the wrong size, rather than undrawable.
func clampScale(scale float64) float64 {
	if math.IsNaN(scale) || math.IsInf(scale, 0) || scale <= 0 {
		return 1
	}
	return math.Min(math.Max(scale, 0.5), 4)
}

// refitDisplay is the screen changing under the flower: a different resolution, a
// different scaling, or the taskbar moving to another edge. Everything the flower's size
// and position were worked out from is stale, so all of it is worked out again — and the
// flower keeps what it was asked for: the same corner, or somebody's own spot at the same
// *relative* place, standing on the new ground at the size that looks the same to a
// person even though the display is now a different number of pixels.
func (b *Buddy) refitDisplay() {
	area := win.PrimaryWorkArea()
	scale := b.displayScale()
	if area == b.deskArea && math.Abs(scale-b.deskScale) < 1e-9 {
		return // nothing a person could see changed: no message moves the flower
	}
	wasDragging := b.dragging
	if b.cfg.Home {
		b.cfg.HomeX, b.cfg.HomeY = sameSpot(b.cfg.HomeX, b.cfg.HomeY, b.deskArea, area)
	}
	b.deskArea, b.deskScale = area, scale

	// the flower's own size at the new scaling, then the window to hold it, then the
	// ground to stand on — in that order, because each is measured from the one before
	b.reframe()
	b.rebuildWindow()
	b.placeHome()
	// and the settings panel, if one is open: it is drawn at the same scale and put at
	// the same place for the same reason, and a plate left at the old size on a screen
	// that just became half of itself is a plate hanging off the edge
	if b.menu != nil {
		b.menu.rescale(b)
	}
	if !wasDragging {
		// a screen that changed is not a journey: the flower is put down where it
		// stands now rather than walking to a corner that moved while it walked
		b.walking = false
		b.handX, b.handY = float64(b.homeX), float64(b.homeY)
		b.posX, b.posY = b.follow(flower.PoseOf(b.rig, b.clock, b.carry.Motion()))
	}
	b.trace("the display changed: work area %v, scaling %.2f, the flower stands at %d,%d (%dx%d)",
		area, scale, b.homeX, b.homeY, b.frame.Width, b.frame.Height)
}
