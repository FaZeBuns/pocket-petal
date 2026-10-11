package buddy

import (
	"math"
	"testing"

	"github.com/FaZeBuns/pocket-petal/internal/flower"
	"github.com/FaZeBuns/pocket-petal/internal/win"
)

// The flower has to look right on every screen a person might have: a 720p laptop, a
// 1080p monitor, a 2K, a 4K and an 8K one. Three promises are checked here, and they are
// the three a person would notice:
//
//   - the flower and its settings panel are the same *share* of the desk at every
//     resolution, which is what makes changing the resolution leave them where they were
//     rather than leaving them the same number of pixels on a different amount of desk
//   - the same corner is the same place, with the flower standing on the taskbar and the
//     same share of the screen between it and the edges
//   - a spot somebody picked with their own hands keeps its share of the desk when the
//     screen changes size, which is what makes a resolution change survivable

// screen is one display: the monitor, and the work area on it (the desktop minus the
// taskbar). The pairings are the real ones — a 4K panel is four times the pixels of a
// 1080p one and its work area is twice as tall.
type screen struct {
	name    string
	monitor win.WorkArea
	area    win.WorkArea
	scale   float64 // what deskScaleFor makes of the monitor
}

func screens() []screen {
	monitors := []struct {
		name string
		box  win.WorkArea
		task int32 // what the taskbar takes off the bottom
	}{
		{"720p", win.WorkArea{Left: 0, Top: 0, Right: 1280, Bottom: 720}, 40},
		{"1080p", win.WorkArea{Left: 0, Top: 0, Right: 1920, Bottom: 1080}, 40},
		{"2K", win.WorkArea{Left: 0, Top: 0, Right: 2560, Bottom: 1440}, 40},
		{"4K", win.WorkArea{Left: 0, Top: 0, Right: 3840, Bottom: 2160}, 40},
		{"8K", win.WorkArea{Left: 0, Top: 0, Right: 7680, Bottom: 4320}, 80},
	}
	out := make([]screen, 0, len(monitors))
	for _, m := range monitors {
		area := m.box
		area.Bottom -= m.task
		out = append(out, screen{name: m.name, monitor: m.box, area: area, scale: deskScaleFor(m.box)})
	}
	return out
}

// restingFlower is the flower's own outline, in design units, at the pose it stands in.
func restingFlower(t *testing.T) flower.Bounds {
	t.Helper()
	seed := 0
	rig := flower.CreateRig(flower.RigOptions{Seed: &seed, Palette: "blush"})
	rest := flower.OutlineBox(rig, flower.Motion{})
	if rest.Height() <= 0 || rest.Width() <= 0 {
		t.Fatalf("the flower's outline is %v", rest)
	}
	return rest
}

// pixelsPerUnit is the design-units-to-pixels scale the app works at: the flower is asked
// for so many pixels of height (the setting, which is in 96-dpi pixels, times the
// display's scaling) and the rig is measured in design units, so one division is all of
// it (see Buddy.pixelScale and flower.FrameFor).
func pixelsPerUnit(rest flower.Bounds, settingHeight int, scale float64) float64 {
	return float64(settingHeight) * scale / rest.Height()
}

// Where the flower stands, on every screen and in every corner: inside the work area,
// with its own outline kept off the sides and the top by the margin, and its lowest point
// on the line the taskbar starts at. That last one is the rule that makes the flower
// stand on the bar at every resolution instead of floating above it or sinking into it.
func TestTheFlowerStandsOnTheTaskbarAtEveryResolution(t *testing.T) {
	rest := restingFlower(t)
	const settingHeight = 300
	for _, d := range screens() {
		scale := pixelsPerUnit(rest, settingHeight, d.scale)
		for _, corner := range CornerNames {
			x, y := standPoint(corner, d.area, rest, scale)
			if x < int(d.area.Left) || x > int(d.area.Right) || y < int(d.area.Top) || y > int(d.area.Bottom) {
				t.Errorf("%s, %s: the base of the stem lands at %d,%d, outside the work area %v",
					d.name, corner, x, y, d.area)
				continue
			}
			// the flower's outline in the work area's pixels: the corners of the box
			// it occupies, measured from the base of the stem. Design space is y-up
			// and the screen is y-down (see flower.MakeTransform), so the vertical
			// terms come out reversed — the base *minus* the reach.
			left := float64(x) + rest.MinX*scale
			right := float64(x) + rest.MaxX*scale
			lowest := float64(y) - rest.MinY*scale
			highest := float64(y) - rest.MaxY*scale
			const slack = 1.5 // a pixel of rounding either way, and no more
			// a flower in a bottom corner stands on the taskbar; one in a top corner is
			// hanging from the top of the screen and its lowest petal is wherever it is
			if corner == CornerBottomLeft || corner == CornerBottomRight {
				if lowest < float64(d.area.Bottom)-slack || lowest > float64(d.area.Bottom)+slack {
					t.Errorf("%s, %s: the flower's lowest petal is at y=%.1f and the taskbar starts at %d",
						d.name, corner, lowest, d.area.Bottom)
				}
			}
			if highest < float64(d.area.Top)-slack {
				t.Errorf("%s, %s: the flower is off the top of the screen (%.1f < %d)", d.name, corner, highest, d.area.Top)
			}
			if left < float64(d.area.Left)-slack || right > float64(d.area.Right)+slack {
				t.Errorf("%s, %s: the flower spans x %.1f..%.1f and the screen is %d..%d",
					d.name, corner, left, right, d.area.Left, d.area.Right)
			}
			// the flower is at the edge it was asked for: a margin from the side or the
			// top, and the ground with no margin at all. The margin is the flower's own
			// — it grows with the flower, which is what keeps it looking the same to a
			// person on a display with twice the pixels at twice the scaling.
			margin := float64(marginPixels) * scale
			wantLeft := float64(d.area.Left) + margin
			wantRight := float64(d.area.Right) - margin
			wantTop := float64(d.area.Top) + margin
			switch corner {
			case CornerBottomLeft, CornerTopLeft:
				if math.Abs(left-wantLeft) > slack {
					t.Errorf("%s, %s: the flower's left edge is %.1f from the work area and %.1f was asked for",
						d.name, corner, left-float64(d.area.Left), margin)
				}
			case CornerBottomRight, CornerTopRight:
				if math.Abs(right-wantRight) > slack {
					t.Errorf("%s, %s: the flower's right edge is %.1f from the work area and %.1f was asked for",
						d.name, corner, float64(d.area.Right)-right, margin)
				}
			}
			if corner == CornerTopLeft || corner == CornerTopRight {
				if math.Abs(highest-wantTop) > slack {
					t.Errorf("%s, %s: the flower hangs %.1f from the top and %.1f was asked for",
						d.name, corner, highest-float64(d.area.Top), margin)
				}
			}
		}
	}
}

// The flower is the same *share* of the desk at every resolution, which is the promise a
// person actually checks: change the resolution under the flower and it is the same flower
// on the same part of the same desk, not the same number of pixels on a different amount of
// screen. The setting is in the reference screen's pixels and the screen's own height is the
// scale, so 300 is 300/1080 of a 720p screen, of a 4K one and of an 8K one.
func TestTheFlowerIsTheSameShareOfTheDeskAtEveryResolution(t *testing.T) {
	b := &Buddy{cfg: Config{Width: 320, Height: 300}}
	const (
		wantTall = 300.0 / referenceHeight
		wantWide = 320.0 / referenceHeight
	)
	for _, d := range screens() {
		b.deskScale = d.scale
		w, h := b.wantedSize()
		tall := float64(h) / float64(d.monitor.Height())
		wide := float64(w) / float64(d.monitor.Height())
		if math.Abs(tall-wantTall) > 0.002 {
			t.Errorf("%s: the flower is %d pixels tall on a %d pixel screen — %.1f%% of it, and %.1f%% was asked for",
				d.name, h, d.monitor.Height(), tall*100, wantTall*100)
		}
		if math.Abs(wide-wantWide) > 0.002 {
			t.Errorf("%s: the flower is %.1f%% of the screen's height across and %.1f%% was asked for",
				d.name, wide*100, wantWide*100)
		}
		if w <= 0 || h <= 0 {
			t.Errorf("%s: the flower is %dx%d pixels", d.name, w, h)
		}
	}
	// and a display that reports nonsense is a flower of the wrong size, not one that
	// cannot be drawn
	b.deskScale = 0
	if w, h := b.wantedSize(); w <= 0 || h <= 0 {
		t.Errorf("a display with no scaling at all gave a flower of %dx%d", w, h)
	}
}

// The scale a screen is drawn at is its own height against the reference: a 720p screen
// gives a smaller flower, an 8K one the largest the app will draw. A screen that will not
// say how big it is falls back rather than dividing by nothing.
func TestTheScaleAsksTheScreenHowBigItIs(t *testing.T) {
	for _, d := range screens() {
		if got := deskScaleFor(d.monitor); math.Abs(got-d.scale) > 1e-9 {
			t.Errorf("%s: the scale is %.3fx and %.3fx was worked out", d.name, got, d.scale)
		}
	}
	if got := deskScaleFor(win.WorkArea{}); got <= 0 {
		t.Errorf("a screen with no size at all gave a scale of %.2f", got)
	}
	if got := deskScaleFor(win.WorkArea{Right: 7680, Bottom: 43200}); got != 4 {
		t.Errorf("an absurdly tall screen gave a scale of %.2f, and the app's cap is 4", got)
	}
}

// The same corner is the same place in *logical* terms, which is the comparison a person
// actually makes: the flower sits the same distance from the corner of the screen on a
// 1080p display at 100% as on a 4K one at 200%, because those are the same desktop with
// four times the pixels.
func TestTheSameCornerIsTheSamePlaceAtEveryResolution(t *testing.T) {
	rest := restingFlower(t)
	const settingHeight = 300
	// the same desktop, once at 100% and once at 200%
	small := win.WorkArea{Left: 0, Top: 0, Right: 1920, Bottom: 1040}
	big := win.WorkArea{Left: 0, Top: 0, Right: 3840, Bottom: 2080}
	for _, corner := range CornerNames {
		x1, y1 := standPoint(corner, small, rest, pixelsPerUnit(rest, settingHeight, 1))
		x2, y2 := standPoint(corner, big, rest, pixelsPerUnit(rest, settingHeight, 2))
		if math.Abs(float64(x1)-float64(x2)/2) > 1.5 || math.Abs(float64(y1)-float64(y2)/2) > 1.5 {
			t.Errorf("%s: the base of the stem is %d,%d on a 1080p screen at 100%% and %d,%d on a 4K at 200%%",
				corner, x1, y1, x2/2, y2/2)
		}
	}
}

// Somebody's own spot keeps its share of the desk when the screen changes size: the spot
// is where they put the flower when they looked at their desk, so shrinking the display
// puts it in the same part of the screen rather than leaving it in pixels that no longer
// exist.
func TestASpotKeepsItsShareOfTheDeskWhenTheScreenChanges(t *testing.T) {
	big := win.WorkArea{Left: 0, Top: 0, Right: 3840, Bottom: 2160}
	small := win.WorkArea{Left: 0, Top: 0, Right: 1280, Bottom: 720}
	// three quarters across, and a quarter of the way down
	x, y := sameSpot(2880, 540, big, small)
	if wantX, wantY := 960, 180; x != wantX || y != wantY {
		t.Errorf("a spot at 2880,540 on a 4K screen became %d,%d on a 720p one, and %d,%d was expected", x, y, wantX, wantY)
	}
	// the same screen twice is the same spot, to the pixel
	if x, y := sameSpot(2880, 540, big, big); x != 2880 || y != 540 {
		t.Errorf("a spot moved to %d,%d on the screen it was already on", x, y)
	}
	// and a work area with no size at all (a display that is asleep, a query that
	// failed) leaves the spot exactly where it was rather than dividing by nothing
	if x, y := sameSpot(300, 400, win.WorkArea{}, small); x != 300 || y != 400 {
		t.Errorf("a spot at 300,400 became %d,%d when the old work area was empty", x, y)
	}
}

// The screen changing under the flower, on the real machine this is running on: the work
// area and the scaling are read again, the flower is framed for the new display, and it
// is put back where it belongs — the same corner, or somebody's own spot at the same
// share of the desk. The display is not actually changed (that is not a thing a test may
// do to somebody's desk), so the flower is told it was somewhere else and left to notice
// that it is not.
func TestTheScreenChangingUnderTheFlowerPutsItBackInPlace(t *testing.T) {
	seed := 0
	b := &Buddy{
		rig:   flower.CreateRig(flower.RigOptions{Seed: &seed, Palette: "blush"}),
		carry: flower.CreateCarry(flower.PhysicsOptions{}),
		cfg:   Config{Width: 320, Height: 300},
		// a 4K screen at 175%: not this machine, which is the point
		deskArea:  win.WorkArea{Left: 0, Top: 0, Right: 3840, Bottom: 2120},
		deskScale: 1.75,
	}
	real := win.PrimaryWorkArea()
	if real.Width() <= 0 || real.Height() <= 0 {
		t.Skip("this machine will not say how big its screen is")
	}
	b.reframe()
	b.refitDisplay()

	if b.deskArea != real {
		t.Errorf("the flower is standing on %v and the screen is %v", b.deskArea, real)
	}
	// the flower's own size is the settings at the screen's scaling, and the frame it was
	// given is a window around that
	_, h := b.wantedSize()
	if got, want := b.deskScale, deskScaleFor(win.PrimaryScreen()); got != want {
		t.Errorf("the flower is framed at %.2fx and this screen works out at %.2fx", got, want)
	}
	if b.frame.Width <= 0 || b.frame.Height <= 0 {
		t.Fatalf("the flower was framed into a %dx%d window", b.frame.Width, b.frame.Height)
	}
	if b.frame.Flower < float64(h)*0.5 {
		t.Errorf("the flower is %.0f pixels tall in a frame asked for %d", b.frame.Flower, h)
	}

	// and it is standing on the new ground, in the corner it was asked for
	rest := flower.OutlineBox(b.rig, flower.Motion{})
	wantX, wantY := standPoint(b.cfg.Corner, real, rest, b.pixelScale())
	if b.homeX != wantX || b.homeY != wantY {
		t.Errorf("the flower stands at %d,%d and the %s corner of %v is %d,%d",
			b.homeX, b.homeY, b.cfg.Corner, real, wantX, wantY)
	}
	if b.handX != float64(b.homeX) || b.handY != float64(b.homeY) {
		t.Errorf("the flower stands at %d,%d with its hand at %.0f,%.0f", b.homeX, b.homeY, b.handX, b.handY)
	}

	// a message about nothing does not move the flower: a second refit, with the screen
	// exactly as it was, leaves everything alone
	b.homeX, b.homeY = -1234, -5678
	b.refitDisplay()
	if b.homeX != -1234 || b.homeY != -5678 {
		t.Errorf("a message about nothing moved the flower to %d,%d", b.homeX, b.homeY)
	}
}

// Somebody's own spot survives the screen changing size: it keeps its share of the desk
// through refitDisplay, which is the path a resolution change actually takes.
func TestASpotSurvivesTheScreenChangingSize(t *testing.T) {
	seed := 0
	was := win.WorkArea{Left: 0, Top: 0, Right: 3840, Bottom: 2160}
	b := &Buddy{
		rig:       flower.CreateRig(flower.RigOptions{Seed: &seed, Palette: "blush"}),
		carry:     flower.CreateCarry(flower.PhysicsOptions{}),
		cfg:       Config{Width: 320, Height: 300, Home: true, HomeX: 2880, HomeY: 1620},
		deskArea:  was,
		deskScale: 1.75,
	}
	real := win.PrimaryWorkArea()
	if real.Width() <= 0 || real.Height() <= 0 {
		t.Skip("this machine will not say how big its screen is")
	}
	b.reframe()
	b.refitDisplay()

	wantX, wantY := sameSpot(2880, 1620, was, real)
	if b.cfg.HomeX != wantX || b.cfg.HomeY != wantY {
		t.Errorf("the spot at 2880,1620 on a 4K screen became %d,%d and %d,%d was expected",
			b.cfg.HomeX, b.cfg.HomeY, wantX, wantY)
	}
	// three quarters across and three quarters down, wherever the screen ends
	if got := float64(b.cfg.HomeX) / float64(real.Width()); math.Abs(got-0.75) > 0.01 {
		t.Errorf("the spot is %.3f of the way across the screen", got)
	}
	if got := float64(b.cfg.HomeY) / float64(real.Height()); math.Abs(got-0.75) > 0.01 {
		t.Errorf("the spot is %.3f of the way down the screen", got)
	}
}

// A screen too small for the flower is still a screen the flower stands on: the margin
// gives way and the flower is kept inside the work area rather than pushed off the edge.
func TestATinyScreenStillHoldsTheFlower(t *testing.T) {
	rest := restingFlower(t)
	tiny := win.WorkArea{Left: 0, Top: 0, Right: 200, Bottom: 120}
	scale := pixelsPerUnit(rest, 300, 1) // a flower taller than the screen it is on
	for _, corner := range CornerNames {
		x, y := standPoint(corner, tiny, rest, scale)
		if x < 0 || x > 200 || y < 0 || y > 120 {
			t.Errorf("%s: the base of the stem is at %d,%d on a 200x120 screen", corner, x, y)
		}
	}
}
