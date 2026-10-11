package buddy

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/FaZeBuns/pocket-petal/internal/flower"
	"github.com/FaZeBuns/pocket-petal/internal/win"
)

// Where the milliseconds go while the settings panel is open.
//
// A slider is dragged with the pointer, which means the work it does happens in
// the window's message handler, inside a frame that also has to rasterise the
// flower: everything that is not free shows up as the panel lagging behind the
// mouse, which is the one thing a settings panel may not do. These keep the cost
// of each step visible so that "it feels laggy" can be argued about with numbers.

// menuForBench is testMenu without the *testing.T: the same panel, built by hand.
func menuForBench(b *testing.B, buddy *Buddy, page int) *menuState {
	b.Helper()
	m := &menuState{
		page:    page,
		accent:  flower.PaletteNamed(buddy.cfg.Palette).PetalTip,
		dim:     flower.RGB{158, 146, 168},
		ink:     flower.RGB{240, 235, 245},
		panelBG: flower.RGB{24, 19, 27},
		hover:   -1,
		press:   -1,
		drag:    -1,
	}
	var err error
	if m.title, err = win.NewFont(19, win.FontBold); err != nil {
		b.Skipf("no fonts on this machine: %v", err)
	}
	if m.body, err = win.NewFont(15, win.FontMedium); err != nil {
		b.Fatal(err)
	}
	if m.small, err = win.NewFont(12, win.FontRegular); err != nil {
		b.Fatal(err)
	}
	if m.button, err = win.NewFont(24, win.FontBold); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		m.closeFonts()
	})
	m.measure(buddy)
	return m
}

func benchBuddy(b *testing.B) *Buddy {
	buddy := testBuddy("blush", "flower", CornerBottomRight)
	buddy.reframe()
	buddy.rng = rand.New(rand.NewSource(1))
	// the settings file a benchmark would otherwise write is the one a person is
	// using, sixty times a second
	b.Cleanup(redirectSettings(filepath.Join(b.TempDir(), "settings.json")))
	b.Cleanup(redirectMeasure(func(rig *flower.Rig, w, h int, fit float64) flower.Framing {
		return buddysFrame(buddy)
	}))
	return buddy
}

// buddysFrame is the framing a benchmark uses in place of the measurement: the one
// the flower already has, so that the caller's own cost is what is being measured.
func buddysFrame(buddy *Buddy) flower.Framing { return buddy.frame }

// redirectMeasure swaps the envelope measurement for the length of a benchmark: the
// real one is a third of a second, and measuring a frame that takes a third of a
// second is not a measurement.
func redirectMeasure(stub func(*flower.Rig, int, int, float64) flower.Framing) func() {
	old := measureFrame
	measureFrame = stub
	return func() { measureFrame = old }
}

// The frame the app paints, whether or not a panel is open.
func BenchmarkFlowerFrame(b *testing.B) {
	buddy := benchBuddy(b)
	pose := flower.PoseOf(buddy.rig, flower.FullBloom, flower.Motion{})
	list := frameShapes(flower.BuildDrawList(buddy.rig, pose, flower.DrawOptions{
		Detail: buddy.cfg.Detail, Shadow: true, Palette: buddy.rig.Palette,
	}))
	// everything above is setup. A benchmark that counts growing the plant and
	// measuring its envelope reports a number nobody can act on: reset the clock
	// and the counters, or the first iterations carry the whole frame budget of
	// work that only ever happens once.
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		flower.Rasterize(list, flower.RasterOptions{
			Width: buddy.frame.Width, Height: buddy.frame.Height,
			Bounds: buddy.box, SubSamples: 2, Workers: buddy.cfg.Workers,
		})
	}
}

// The frame costs what the screen asks for. Everything is drawn at the scale of the display
// it is on (see deskScaleFor), so a 4K screen is four times the pixels of a 1080p one and an
// 8K screen sixteen: the frame budget is the same 16 ms at all of them, and what the bigger
// screens actually cost is worth having as a number rather than as an assumption.
func BenchmarkFlowerFramePerScreen(b *testing.B) {
	for _, d := range screens() {
		b.Run(d.name, func(b *testing.B) {
			for _, sub := range []int{2, 1} {
				b.Run(fmt.Sprintf("sub%d", sub), func(b *testing.B) { benchmarkFrameAt(b, d.scale, sub) })
			}
		})
	}
}

func benchmarkFrameAt(b *testing.B, scale float64, sub int) {
	buddy := benchBuddy(b)
	buddy.cfg.Width, buddy.cfg.Height = 190, 250
	buddy.deskScale = scale
	buddy.reframe() // the real envelope: a third of a second, and once
	pose := flower.PoseOf(buddy.rig, flower.FullBloom, flower.Motion{})
	list := frameShapes(flower.BuildDrawList(buddy.rig, pose, flower.DrawOptions{
		Detail: buddy.cfg.Detail, Shadow: true, Palette: buddy.rig.Palette,
	}))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		flower.Rasterize(list, flower.RasterOptions{
			Width: buddy.frame.Width, Height: buddy.frame.Height,
			Bounds: buddy.box, SubSamples: sub, Workers: buddy.cfg.Workers,
		})
	}
}

// A new plant, which is what one dial of the flower costs.
func BenchmarkGrowAFlower(b *testing.B) {
	buddy := benchBuddy(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buddy.newRig()
	}
}

// A dial dragged: a new plant, reframed the cheap way.
func BenchmarkRegrowAFlower(b *testing.B) {
	buddy := benchBuddy(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buddy.regrow()
	}
}

// The exact reframe: the envelope measured by driving the whole carry model, which
// is what a pointer being let go of a slider pays for.
func BenchmarkExactReframe(b *testing.B) {
	buddy := benchBuddy(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buddy.reframe()
	}
}

// A pointer moving over the panel: the row it left, the row it arrived on, and the
// blit. This is what the panel costs on a frame where nothing else changed.
func BenchmarkPanelHover(b *testing.B) {
	buddy := benchBuddy(b)
	m := menuForBench(b, buddy, pageShape)
	m.markAll()
	m.draw()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.mark(i % len(m.rows))
		m.mark((i + 1) % len(m.rows))
		m.draw()
	}
}

// A slider being dragged: the flower regrown, the page laid out with the new number
// on it, and that one row drawn again. Sixty of these a second is the target.
func BenchmarkSliderDrag(b *testing.B) {
	buddy := benchBuddy(b)
	m := menuForBench(b, buddy, pageShape)
	slider := -1
	for i, r := range m.rows {
		if r.kind == rowSlider {
			slider = i
			break
		}
	}
	if slider < 0 {
		b.Skip("that page has no slider on it")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.putSlider(buddy, slider, 1+0.1*float64(i%4))
		m.draw()
	}
}

// Letting go of a slider: everything that happens in the message handler. The exact
// envelope is measured, but off the frame loop, so what is left here is the flower
// regrown, the window, the tray icon and the settings file.
func BenchmarkReleaseASlider(b *testing.B) {
	buddy := benchBuddy(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buddy.cfg.Shape.Stem = 1 + 0.1*float64(i%4)
		buddy.changed()
	}
}

// And the panel itself: laying it out, and painting all of it.
func BenchmarkPanelMeasureAndPaint(b *testing.B) {
	buddy := benchBuddy(b)
	m := menuForBench(b, buddy, pageFlower)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.measure(buddy)
		m.panelPixels()
	}
}
