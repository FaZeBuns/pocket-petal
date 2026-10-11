package flower

import (
	"math"
	"testing"
)

// The flower box the app asks for, and the window it therefore gets.
const (
	WantW, WantH = 190, 250
	WantFit      = 0.94
)

// carriedMotions is a sweep of the carry's state space, not just the poses the
// framing script happened to visit: a fast drag goes through poses nobody
// enumerated, and the bloom of a flower being whipped sideways is exactly what
// used to be sliced off by the edge of the window.
func carriedMotions() []Motion {
	out := append([]Motion{}, envelopeMotions()...)
	for lean := -1.2; lean <= 1.21; lean += 0.6 {
		for swing := -1.15; swing <= 1.16; swing += 0.575 {
			for _, extra := range []Motion{
				{},
				{Whip: 1.2, Flutter: 2.2, Droop: 1},
				{Whip: -1.2, Flutter: 2.2, LagX: -1.2, LagY: 1.2, Stretch: -1},
			} {
				m := extra
				m.Lean, m.Swing = lean, swing
				out = append(out, m)
			}
		}
	}
	// and a few past anything the physics allows, because the rig clamps those
	// rather than exploding them
	out = append(out,
		Motion{Lean: -4, Swing: -4, Whip: 4, Flutter: 9, Droop: 4, Gust: 4, Stretch: -4, LagX: 4, LagY: -4},
		Motion{Lean: 4, Swing: 4, Whip: -4, Flutter: 9, Droop: 4, Gust: 4, Stretch: 4, LagX: -4, LagY: 4},
	)
	return out
}

// drawnBounds is the extent of a pose in window pixels, halo and shadow aside.
func drawnBounds(rig *Rig, box Bounds, m Motion, width, height int) (minX, minY, maxX, maxY float64) {
	pose := PoseOf(rig, FullBloom, m)
	list := BuildDrawList(rig, pose, DrawOptions{Detail: "full", Shadow: false})
	tr := MakeTransform(box, width, height, 0)
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)
	for _, shape := range list {
		if shape.Part == "glow" {
			continue
		}
		for _, p := range shape.Poly {
			px, py := tr.ToPixel(p.X, p.Y)
			minX, maxX = math.Min(minX, px), math.Max(maxX, px)
			minY, maxY = math.Min(minY, py), math.Max(maxY, py)
		}
	}
	return
}

func TestFrameForHoldsEveryPoseTheCarryReaches(t *testing.T) {
	rig := CreateRig(RigOptions{})
	f := FrameFor(rig, WantW, WantH, WantFit)
	for _, m := range carriedMotions() {
		minX, minY, maxX, maxY := drawnBounds(rig, f.Bounds, m, f.Width, f.Height)
		if minX < 0 || minY < 0 || maxX > float64(f.Width) || maxY > float64(f.Height) {
			t.Errorf("motion %+v draws outside the %dx%d window: x %.1f..%.1f, y %.1f..%.1f",
				m, f.Width, f.Height, minX, maxX, minY, maxY)
		}
	}
}

// The window is sized around the flower rather than the flower around the
// window: a flower that is never clipped because it is the size of a pea is not
// the fix, so the resting flower has to come out at the size the app asked for.
func TestFrameForKeepsTheFlowerTheSizeItWasAsked(t *testing.T) {
	rig := CreateRig(RigOptions{})
	f := FrameFor(rig, WantW, WantH, WantFit)
	if want := float64(WantH) * WantFit; math.Abs(f.Flower-want) > want*0.02 {
		t.Errorf("the flower stands %.0fpx tall, asked for %.0fpx", f.Flower, want)
	}
	_, minY, _, maxY := drawnBounds(rig, f.Bounds, Motion{}, f.Width, f.Height)
	if got := maxY - minY; math.Abs(got-f.Flower) > 1 {
		t.Errorf("FrameFor promises %.0fpx of flower but draws %.0fpx", f.Flower, got)
	}
	t.Logf("window %dx%d for a %.0fpx flower", f.Width, f.Height, f.Flower)
}

// The resting flower sits where the caller puts the window, which is what lets
// the app keep the flower in the corner while the window is bigger than it.
func TestFrameForReportsWhereTheRestingFlowerLands(t *testing.T) {
	rig := CreateRig(RigOptions{})
	f := FrameFor(rig, WantW, WantH, WantFit)
	minX, minY, maxX, maxY := drawnBounds(rig, f.Bounds, Motion{}, f.Width, f.Height)
	if cx := (minX + maxX) / 2; math.Abs(cx-f.RestX) > 1 {
		t.Errorf("resting flower is centred at x=%.1f, reported %.1f", cx, f.RestX)
	}
	if cy := (minY + maxY) / 2; math.Abs(cy-f.RestY) > 1 {
		t.Errorf("resting flower is centred at y=%.1f, reported %.1f", cy, f.RestY)
	}
	// and the app's placement puts that centre where the config box says
	wantX := float64(WantW)/2 + f.RestX - f.RestX // the shift cancels out
	if math.Abs(wantX-float64(WantW)/2) > 0.001 {
		t.Fatal("unreachable")
	}
}

// Every palette grows a flower of a slightly different outline, and none of them
// may leave the window.
func TestFrameForHoldsForEveryPalette(t *testing.T) {
	for _, name := range PaletteNames {
		rig := CreateRig(RigOptions{Seed: intPtr(7), Palette: name})
		f := FrameFor(rig, WantW, WantH, WantFit)
		for _, m := range []Motion{{}, {Lean: -1.2, Swing: -1.15, Flutter: 2.2}, {Lean: 1.2, Swing: 1.15, Whip: -1.2, Flutter: 2.2}} {
			minX, minY, maxX, maxY := drawnBounds(rig, f.Bounds, m, f.Width, f.Height)
			if minX < 0 || minY < 0 || maxX > float64(f.Width) || maxY > float64(f.Height) {
				t.Errorf("palette %s, motion %+v draws outside the window: x %.1f..%.1f, y %.1f..%.1f",
					name, m, minX, maxX, minY, maxY)
			}
		}
	}
}

// A window that is a rectangle of air costs nothing, but it should still not be
// absurd: the leash the flower needs is worth knowing about.
func TestFrameForStaysAReasonableWindow(t *testing.T) {
	rig := CreateRig(RigOptions{})
	f := FrameFor(rig, WantW, WantH, WantFit)
	if f.Width > 3*WantW || f.Height > 2*WantH {
		t.Errorf("the window grew to %dx%d for a %dx%d flower", f.Width, f.Height, WantW, WantH)
	}
	t.Logf("window %dx%d (%d buffers of air)", f.Width, f.Height, f.Width*f.Height)
}

// The shine may only ever brighten flower: the gaps between the petals have to
// come out of a pass exactly as they went in.
func TestShineOnlyTouchesTheFlower(t *testing.T) {
	rig := CreateRig(RigOptions{})
	pose := PoseOf(rig, FullBloom, Motion{Lean: -0.4})
	list := BuildDrawList(rig, pose, DrawOptions{Detail: "full", Shadow: true})
	box := MotionBox(rig, 0.94)
	frame := Rasterize(list, RasterOptions{Width: 190, Height: 250, Bounds: box, SubSamples: 2})

	before := append([]uint8{}, frame...)
	for _, phase := range []float64{-0.2, 0, 0.05, 0.3, 0.5, 0.7, 0.95, 1, 1.4} {
		copied := append([]uint8{}, before...)
		ApplyShine(copied, 190, 250, phase)
		brightened := 0
		for i := 0; i < len(copied); i += 4 {
			if copied[i+3] != before[i+3] {
				t.Fatalf("phase %.2f changed the alpha at pixel %d", phase, i/4)
			}
			// a pixel nothing was drawn into may hold colour from an earlier shape
			// that rounded away to nothing, but the shine must not touch it
			if before[i+3] == 0 && (copied[i] != before[i] || copied[i+1] != before[i+1] || copied[i+2] != before[i+2]) {
				t.Fatalf("phase %.2f painted a transparent pixel at %d", phase, i/4)
			}
			if copied[i+3] > 0 && (copied[i] > before[i] || copied[i+1] > before[i+1] || copied[i+2] > before[i+2]) {
				brightened++
			}
		}
		// the band has to actually cross the flower while it is out, so the
		// middle of a pass lights something and the ends of it are the fade
		if phase >= 0.3 && phase <= 0.7 && brightened == 0 {
			t.Errorf("phase %.2f brightened nothing at all", phase)
		}
		if phase <= 0 || phase >= 1 {
			for i := range copied {
				if copied[i] != before[i] {
					t.Fatalf("phase %.2f changed the frame outside a pass", phase)
				}
			}
		}
	}
}

func intPtr(v int) *int { return &v }
