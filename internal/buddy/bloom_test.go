package buddy

import (
	"math"
	"math/rand"
	"testing"

	"github.com/FaZeBuns/pocket-petal/internal/flower"
	"github.com/FaZeBuns/pocket-petal/internal/win"
)

// The burst's surface is the one place in the app where "never cut off" and "cheap
// enough to fly" pull against each other: the surface has to hold the whole flight,
// and a frame has to pay for the part of it the petals are in. Both halves are
// written down here with numbers, so that a later tuning pass — more petals, a
// slower fade, more gravity — cannot quietly make an explosion that costs a
// frame it cannot afford.

// The scale a flower is drawn at: design units onto screen pixels.
func testScale(t *testing.T, b *Buddy) flower.Transform {
	t.Helper()
	b.reframe()
	return flower.MakeTransform(b.box, b.frame.Width, b.frame.Height, 0)
}

// The surface is built from the flight the burst actually makes, which is what lets
// the explosion be big: the clamp on the throw is a square that knows nothing about
// which way the petals went, and the measured box is the shape they really make.
// The test is the promise that matters — a surface built from the flight can never
// be too small for a frame of it — and the one that keeps it affordable, which is
// that a plain drop does not ask for a giant surface.
func TestThePetalSurfaceIsSizedFromTheFlight(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)
	tr := testScale(t, b)
	pose := flower.PoseOf(b.rig, flower.FullBloom, flower.Motion{})
	centre := flower.Point{X: pose.Head.X, Y: pose.Head.Y}
	pieces := flower.Confetti(b.rig, centre, burstPetals, rand.New(rand.NewSource(3)))
	burst := flower.NewBurst(pieces, rand.New(rand.NewSource(3)), 1.2, 0, 0)
	flight := burst.Flight(burstReach)

	sw, sh := burstSurfacePixels(tr, flight)
	area := float64(sw) * float64(sh)
	t.Logf("a plain drop flies in a %dx%d surface (%.1f megapixels, %.0f MB) at scale %.2f",
		sw, sh, area/1e6, area*4/1e6, tr.Scale)

	biggest := 0
	for i := 0; i < 600 && !burst.Done(); i++ {
		burst.Step(1.0 / 60)
		_, _, w, h, _ := burstRegion(tr, burst.Bounds())
		biggest = max(biggest, w*h)
		if w > sw || h > sh {
			t.Fatalf("frame %d wants a %dx%d region in the %dx%d surface built for it", i, w, h, sw, sh)
		}
	}
	t.Logf("the biggest frame of it draws %.2f megapixels", float64(biggest)/1e6)
	// a drop nobody flung should not allocate a screen: the surface is kept for the
	// life of the process, so it is worth saying out loud what it costs
	if area > 3.0e6 {
		t.Errorf("a plain drop's surface is %.1f megapixels", area/1e6)
	}
}

// And the same promise for the worst the machine can do: a flower thrown hard in
// both directions is fitted to the clamp, the surface is built from that flight, and
// no frame of it ever wants more room than there is.
func TestEvenAHardFlingFitsItsSurface(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)
	tr := testScale(t, b)
	pose := flower.PoseOf(b.rig, flower.FullBloom, flower.Motion{})
	centre := flower.Point{X: pose.Head.X, Y: pose.Head.Y}
	pieces := flower.Confetti(b.rig, centre, burstPetals, rand.New(rand.NewSource(11)))
	burst := flower.NewBurst(pieces, rand.New(rand.NewSource(11)), 1.7, 2600, 2200)
	flight := burst.Flight(burstReach)

	sw, sh := burstSurfacePixels(tr, flight)
	biggest := 0
	for i := 0; i < 600 && !burst.Done(); i++ {
		burst.Step(1.0 / 60)
		_, _, w, h, _ := burstRegion(tr, burst.Bounds())
		biggest = max(biggest, w*h)
		if w > sw || h > sh {
			t.Fatalf("frame %d wants a %dx%d region in the %dx%d surface", i, w, h, sw, sh)
		}
	}
	t.Logf("a hard fling flies in a %dx%d surface (%.1f megapixels); its biggest frame draws %.2f megapixels",
		sw, sh, float64(sw*sh)/1e6, float64(biggest)/1e6)
	// the clamp is what stops a flung flower asking for a surface the size of the
	// desk, so this is the number that says whether the clamp is doing its job
	if worst := float64(burstVistaPixels(tr.Scale)); float64(sw*sh) > worst*worst {
		t.Errorf("a hard fling's surface (%dx%d) is bigger than the clamp allows (%.0fpx)", sw, sh, worst)
	}
}

// The whole of it, for real: a burst thrown from a real flower, drawn through a
// real layered window, frame by frame. The window is put far off the screen — the
// petals are not what is being looked at here, the arithmetic is — so this can run
// on a machine somebody is using.
func TestABurstFliesThroughARealWindow(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)
	tr := testScale(t, b)
	window, err := win.NewLayeredWindow(b.frame.Width, b.frame.Height, func(win.MSG) bool { return false })
	if err != nil {
		t.Skipf("no layered windows on this machine: %v", err)
	}
	defer window.Destroy()
	b.win = window
	b.posX, b.posY = -4000, -4000
	b.rng = rand.New(rand.NewSource(2))

	pose := flower.PoseOf(b.rig, flower.FullBloom, flower.Motion{})
	b.throwPetals(pose)
	if b.burst == nil {
		t.Fatal("the flower was dropped and nothing came off it")
	}
	frames := 0
	var first, last win.RECT
	spanL, spanT, spanR, spanB := -4000, -4000, -4000, -4000
	for i := 0; i < 300 && b.burst != nil; i++ {
		b.drawPetals()
		if b.burst == nil {
			break
		}
		if b.petals == nil {
			t.Fatal("there is nowhere for the petals to fly: no surface was made")
		}
		r := b.petals.Rect()
		if first.Width() == 0 {
			first, spanL, spanT, spanR, spanB = r, int(r.Left), int(r.Top), int(r.Right), int(r.Bottom)
		}
		last = r
		spanL, spanT = min(spanL, int(r.Left)), min(spanT, int(r.Top))
		spanR, spanB = max(spanR, int(r.Right)), max(spanB, int(r.Bottom))
		// every frame is inside the surface it was given, and is a real window
		if r.Width() < 1 || r.Height() < 1 || r.Width() > b.petals.W || r.Height() > b.petals.H {
			t.Fatalf("frame %d is a %dx%d window in a %dx%d surface",
				i, r.Width(), r.Height(), b.petals.W, b.petals.H)
		}
		frames++
		b.burst.Step(1.0 / 60)
	}
	if frames < 30 {
		t.Fatalf("the burst was over in %d frames", frames)
	}
	// the cloud sinks: the window the pieces are shown in moves down as they fall
	if last.Top <= first.Top {
		t.Errorf("the burst did not fall: %d then %d", first.Top, last.Top)
	}
	// and it ends up somewhere the square a burst used to fly in never reached —
	// the fixed 620-unit box centred on the bloom, in pixels
	oldSquare := 620 * tr.Scale
	if h := float64(spanB - spanT); h <= oldSquare {
		t.Errorf("the whole burst spans %.0fpx and the old square was %.0fpx: nothing to fix", h, oldSquare)
	}
	t.Logf("%d frames, from %dx%d at %d,%d to %dx%d at %d,%d; the whole burst spans %dx%d px against the old %.0fpx square",
		frames, first.Width(), first.Height(), first.Left, first.Top,
		last.Width(), last.Height(), last.Left, last.Top, spanR-spanL, spanB-spanT, oldSquare)

	// and when it is over there is nothing left to draw, and the surface is put away
	b.drawPetals()
	if b.burst != nil {
		t.Fatal("the burst is still going")
	}
}

// A frame draws the box the pieces are in, and that box is the pieces: every piece
// still in the air is inside the rectangle, at every moment of the flight.
func TestAFrameOfTheBurstDrawsWhereThePiecesAre(t *testing.T) {
	b := testBuddy("periwinkle", "flower", CornerTopLeft)
	tr := testScale(t, b)
	pose := flower.PoseOf(b.rig, flower.FullBloom, flower.Motion{})
	centre := flower.Point{X: pose.Head.X, Y: pose.Head.Y}

	pieces := flower.Confetti(b.rig, centre, burstPetals, rand.New(rand.NewSource(3)))
	burst := flower.NewBurst(pieces, rand.New(rand.NewSource(3)), 1.4, 400, 0)
	flight := burst.Flight(burstReach)
	sw, sh := burstSurfacePixels(tr, flight)

	biggest, frames := 0, 0
	for i := 0; i < 600 && !burst.Done(); i++ {
		burst.Step(1.0 / 60)
		px, py, w, h, box := burstRegion(tr, burst.Bounds())
		frames++
		biggest = max(biggest, w*h)
		if w > sw || h > sh {
			t.Fatalf("frame %d wants a %dx%d region in the %dx%d surface built for the flight", i, w, h, sw, sh)
		}
		// the design box, mapped to pixels, is the rectangle that is drawn: check it
		// by mapping the box's own corners rather than by trusting the arithmetic
		left, top := tr.ToPixel(box.MinX, box.MaxY)
		right, bottom := tr.ToPixel(box.MaxX, box.MinY)
		if px > int(math.Floor(left)) || py > int(math.Floor(top)) {
			t.Fatalf("frame %d: the region starts at %d,%d and the box at %.0f,%.0f", i, px, py, left, top)
		}
		if float64(px+w)+1 < right || float64(py+h)+1 < bottom {
			t.Fatalf("frame %d: the region is %dx%d at %d,%d and the box ends at %.0f,%.0f", i, w, h, px, py, right, bottom)
		}
		// and every piece is inside it, which is the promise the region makes
		for _, shape := range burst.Shapes() {
			for _, p := range shape.Poly {
				if p.X < box.MinX || p.X > box.MaxX || p.Y < box.MinY || p.Y > box.MaxY {
					t.Fatalf("frame %d draws at %.1f,%.1f, outside %+v", i, p.X, p.Y, box)
				}
			}
		}
	}
	// what a frame of a burst actually costs: the region it draws, against the
	// surface it lives in. The flower's own frame is an order of magnitude more
	// than this, and the surface is only ever written where the petals are.
	t.Logf("%d frames of a burst, the biggest region drawn %.2f megapixels in a %.2f megapixel surface",
		frames, float64(biggest)/1e6, float64(sw*sh)/1e6)
	// the budget for a frame of the burst: bigger petals, more of them and a
	// wider throw all land here, and this is the number that says whether they can
	// be afforded. The flower's own frame is about 0.16 of a megapixel, so this is
	// the same order as the thing it is happening over.
	if float64(biggest) > 2.4e6 {
		t.Errorf("a frame of the burst draws %.2f megapixels", float64(biggest)/1e6)
	}
}
