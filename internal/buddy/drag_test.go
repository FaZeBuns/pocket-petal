package buddy

import (
	"math"
	"testing"

	"github.com/FaZeBuns/pocket-petal/internal/flower"
	"github.com/FaZeBuns/pocket-petal/internal/win"
)

// The drag, as arithmetic.
//
// "The flower is offset from my mouse" is the one bug this app has had twice, and
// both times it was the same mistake: the window was placed from somewhere other
// than the part of the flower the hand is actually holding. The rule is one line —
// the anchored point of the pose lands on the hand — and it is worth a test that
// states it, because every other way of writing it looks plausible.

// draggedMotions is a spread of the ways a flower is carried, including the ones
// that move the held piece of stem the most.
var draggedMotions = []flower.Motion{
	{},
	{Lean: 1.2, Whip: 1.1, Swing: 0.9, Flutter: 2.2},
	{Lean: -1.2, Whip: -1.2, Swing: -1.15, Droop: 1, Flutter: 2.2},
	{Lean: -0.4, Whip: 0.6, Swing: 1.05, LagX: 1.2, LagY: -1.2},
	{Stretch: -1, Gust: 1.5},
}

// TestTheGrabbedStemStaysUnderTheHand is the whole point of the drag: pick the
// flower up anywhere — the base, a leaf node, the neck, the bloom — and that exact
// piece stays under the cursor while the rest of the flower bends and swings
// around it.
func TestTheGrabbedStemStaysUnderTheHand(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)
	b.reframe()
	b.dragging = true
	b.handX, b.handY = 840, 512

	grabs := []struct {
		s    float64
		head bool
	}{
		{0, false}, {0.25, false}, {0.5, false}, {0.8, false}, {1, false}, {0, true},
	}
	for _, grab := range grabs {
		b.grabS, b.grabHead = grab.s, grab.head
		for _, m := range draggedMotions {
			pose := flower.PoseOf(b.rig, flower.FullBloom, m)
			x, y := b.follow(pose)
			ax, ay := b.anchor(pose)
			tr := flower.MakeTransform(b.box, b.frame.Width, b.frame.Height, 0)
			px, py := tr.ToPixel(ax, ay)
			// where that piece of the flower ends up on screen
			gotX, gotY := float64(x)+px, float64(y)+py
			if math.Abs(gotX-b.handX) > 0.5 || math.Abs(gotY-b.handY) > 0.5 {
				t.Errorf("grabbed at s=%.2f (head=%v) with %+v: the flower is held at %.1f,%.1f "+
					"but the hand is at %.1f,%.1f — off by %.1f,%.1f",
					grab.s, grab.head, m, gotX, gotY, b.handX, b.handY, gotX-b.handX, gotY-b.handY)
			}
		}
	}
}

// Grabbing the bloom holds the top of the stalk, not the middle of the bloom: the
// flower has to be able to swing on its neck while you hold it, which is what
// makes it look like a flower and not a cursor with a picture on it.
func TestHoldingTheBloomLeavesTheBloomFreeToSwing(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)
	b.reframe()
	b.dragging, b.grabHead = true, true
	b.handX, b.handY = 600, 400

	still := flower.PoseOf(b.rig, flower.FullBloom, flower.Motion{})
	swinging := flower.PoseOf(b.rig, flower.FullBloom, flower.Motion{Swing: 1.15, Lean: 0.3})
	ax, ay := b.anchor(still)
	bx, by := b.anchor(swinging)
	if ax == bx && ay == by {
		t.Fatal("the top of the stalk does not move when the neck swings: nothing is being held")
	}
	// and the bloom itself travels further than the point being held, which is
	// exactly the swing
	moved := math.Hypot(still.Head.X-swinging.Head.X, still.Head.Y-swinging.Head.Y)
	if moved < 4 {
		t.Fatalf("the bloom barely moved: %.2f", moved)
	}
}

// Standing still, the flower stands on the base of its stem: the window does not
// move while the flower sways inside it, and the top of the stem goes wherever the
// wind takes it. A window that followed the sway would make the whole flower
// slide: this is the difference between a flower and a puppet.
func TestStandingStillTheWindowDoesNotFollowTheSway(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)
	b.reframe()
	b.handX, b.handY = 300, 900

	x0, y0 := b.follow(flower.PoseOf(b.rig, flower.FullBloom, flower.Motion{}))
	for _, m := range draggedMotions {
		x, y := b.follow(flower.PoseOf(b.rig, flower.FullBloom, m))
		if x != x0 || y != y0 {
			t.Fatalf("the window moved to %d,%d while the flower swayed (was %d,%d)", x, y, x0, y0)
		}
	}
	// ...which means the anchor is the ground, and the ground is the rig's origin
	if ax, ay := b.anchor(flower.PoseOf(b.rig, flower.FullBloom, flower.Motion{})); ax != 0 || ay != 0 {
		t.Fatalf("standing idle the flower is held by %v, and it should be held by its base", flower.Point{X: ax, Y: ay})
	}
}

// Letting go puts the flower down where it is standing.
//
// It is placed by whatever part of it the hand has: the piece of stalk taken hold of
// while it is carried, and the base of the stem once it is standing. Those two have
// to be reconciled at the moment of letting go, and when they are not, the anchor
// changes underneath the flower and the whole thing is dragged down to the cursor —
// a flower let go of halfway up its stem jumps up with its feet under the mouse. So
// this is a property, checked at every place a hand can take hold: let go anywhere,
// and the flower does not move.
func TestLettingGoLeavesTheFlowerWhereItIs(t *testing.T) {
	grabs := []struct {
		what string
		s    float64
		head bool
	}{
		{"the base of the stem", 0, false},
		{"a quarter of the way up", 0.25, false},
		{"the middle of the stem", 0.5, false},
		{"near the top of the stem", 0.9, false},
		{"the bloom", 0, true},
	}
	for _, grab := range grabs {
		b := testBuddy("blush", "flower", CornerBottomRight)
		b.reframe()
		b.dragging, b.grabS, b.grabHead = true, grab.s, grab.head
		b.handX, b.handY = 760, 480
		pose := flower.PoseOf(b.rig, flower.FullBloom, flower.Motion{})
		// the frame that placed the window while it was carried
		b.posX, b.posY = b.follow(pose)
		wasX, wasY := b.standingPoint()

		b.letGo()

		// and the next frame places it as a *standing* flower
		x, y := b.follow(pose)
		nowX := float64(x) + b.basePixX
		nowY := float64(y) + b.basePixY
		if math.Abs(nowX-float64(wasX)) > 1 || math.Abs(nowY-float64(wasY)) > 1 {
			t.Errorf("holding %s and letting go moved the base of the stem from %d,%d to %.0f,%.0f",
				grab.what, wasX, wasY, nowX, nowY)
		}
	}
}

// The flower stands in the corner the settings ask for, with its own outline — not
// its window's envelope — inside the work area, and its feet on the ground.
//
// The ground is the bottom edge of the work area, which is the top of the taskbar:
// that is the line the stem has to meet, whatever the flower is grown into. So the
// bottom corners are checked twice — the flower is inside the work area, *and* the
// lowest point of it is that line rather than a margin above it.
func TestTheFlowerStandsWhereItIsPut(t *testing.T) {
	area := win.PrimaryWorkArea()
	for _, corner := range CornerNames {
		b := testBuddy("blush", "flower", corner)
		b.reframe()
		b.placeHome()
		b.handX, b.handY = float64(b.homeX), float64(b.homeY)
		x, y := b.follow(flower.PoseOf(b.rig, flower.FullBloom, flower.Motion{}))
		tr := flower.MakeTransform(b.box, b.frame.Width, b.frame.Height, 0)
		rest := flower.OutlineBox(b.rig, flower.Motion{})
		for _, p := range []flower.Point{
			{X: rest.MinX, Y: rest.MinY}, {X: rest.MaxX, Y: rest.MinY},
			{X: rest.MinX, Y: rest.MaxY}, {X: rest.MaxX, Y: rest.MaxY},
		} {
			px, py := tr.ToPixel(p.X, p.Y)
			sx, sy := float64(x)+px, float64(y)+py
			// the bottom edge is allowed to be *touched*: rounding a window's
			// position to a whole pixel is worth a fraction of one
			if sx < float64(area.Left) || sx > float64(area.Right) || sy < float64(area.Top) || sy > float64(area.Bottom)+1 {
				t.Errorf("in %s the flower reaches %.0f,%.0f, which is off the work area %v", corner, sx, sy, area)
			}
		}
		// and the base of the stem is the point that sits at the corner
		base := flower.Point{X: float64(x) + b.basePixX, Y: float64(y) + b.basePixY}
		if math.Abs(base.X-float64(b.homeX)) > 1 || math.Abs(base.Y-float64(b.homeY)) > 1 {
			t.Errorf("in %s the stem's base is at %.0f,%.0f and home is %d,%d", corner, base.X, base.Y, b.homeX, b.homeY)
		}
		// a flower standing in the bottom corners stands on the taskbar: the
		// lowest point of the flower is the ground, not a margin above it
		if corner == CornerBottomRight || corner == CornerBottomLeft {
			lowest := base.Y - rest.MinY*b.pixelScale()
			if math.Abs(lowest-float64(area.Bottom)) > 1 {
				t.Errorf("in %s the flower's lowest point is at %.1f, and the top of the taskbar is %d", corner, lowest, area.Bottom)
			}
		}
	}
}
