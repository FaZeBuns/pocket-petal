package flower

import (
	"math/rand"
	"testing"
)

// The desktop window is small and redrawn sixty times a second, so what matters
// is the cost of one frame at the size the app actually shows. These benchmarks
// are how the render budget is kept honest: run `go test ./internal/flower -bench
// Frame -benchtime 40x`.
func frameBench(b *testing.B, width, height int, detail string, shadow bool, sub int) {
	b.Helper()
	rig := CreateRig(RigOptions{})
	pose := PoseOf(rig, 6, Motion{Lean: 0.2, Swing: 0.3})
	list := BuildDrawList(rig, pose, DrawOptions{Detail: detail, Shadow: shadow, Palette: rig.Palette})

	// the app's box: the flower's own outline, a little air, then the fit
	trimmed := make([]Shape, 0, len(list))
	for _, s := range list {
		if s.Part != "glow" {
			trimmed = append(trimmed, s)
		}
	}
	box := ListBounds(trimmed)
	padX := box.Width() * 0.04
	padY := box.Height() * 0.03
	box.MinX -= padX
	box.MaxX += padX
	box.MinY -= padY
	box.MaxY += padY
	const fit = 0.94
	cx, cy := (box.MinX+box.MaxX)/2, (box.MinY+box.MaxY)/2
	halfW, halfH := box.Width()/2/fit, box.Height()/2/fit
	box = Bounds{MinX: cx - halfW, MinY: cy - halfH, MaxX: cx + halfW, MaxY: cy + halfH}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Rasterize(list, RasterOptions{Width: width, Height: height, Bounds: box, SubSamples: sub})
	}
}

func BenchmarkFrame190x250(b *testing.B) { frameBench(b, 190, 250, "full", true, 2) }
func BenchmarkFrame152x200(b *testing.B) { frameBench(b, 152, 200, "full", true, 2) }
func BenchmarkFrameSub1(b *testing.B)    { frameBench(b, 190, 250, "full", true, 1) }
func BenchmarkFrameNoShadow(b *testing.B) {
	frameBench(b, 190, 250, "full", false, 2)
}
func BenchmarkFrameLite(b *testing.B) { frameBench(b, 190, 250, "lite", true, 2) }

func BenchmarkFrameHaloOnly(b *testing.B) {
	rig := CreateRig(RigOptions{})
	pose := PoseOf(rig, 6, Motion{Lean: 0.2, Swing: 0.3})
	list := BuildDrawList(rig, pose, DrawOptions{Detail: "full", Shadow: true, Palette: rig.Palette})
	glow := make([]Shape, 0, 4)
	for _, s := range list {
		if s.Part == "glow" || s.Part == "shadow" {
			glow = append(glow, s)
		}
	}
	box := ListBounds(list)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Rasterize(glow, RasterOptions{Width: 190, Height: 250, Bounds: box, SubSamples: 2})
	}
}

// One frame of the explosion, at its widest: this is the number that decides
// whether a bigger, rounder burst is still affordable while the flower is being
// dragged. The region drawn is measured from the flight itself, and the scale is
// the one the desktop window draws the flower at.
func BenchmarkBurstFrame(b *testing.B) {
	const scale = 0.86
	rig := CreateRig(RigOptions{})
	pose := PoseOf(rig, 6, Motion{})
	centre := Point{X: pose.Head.X, Y: pose.Head.Y}
	pieces := Confetti(rig, centre, 132, rand.New(rand.NewSource(3)))
	burst := NewBurst(pieces, rand.New(rand.NewSource(3)), 1.2, 0, 0)
	burst.Flight(1400)

	// the widest instant of the flight, which is the frame worth budgeting for
	var worst []Shape
	box, area := Bounds{}, 0.0
	for i := 0; i < 600 && !burst.Done(); i++ {
		burst.Step(1.0 / 60)
		shapes, at := burst.Shapes(), burst.Bounds()
		if a := at.Width() * at.Height(); a > area {
			area, worst, box = a, shapes, at
		}
	}
	w, h := int(box.Width()*scale)+8, int(box.Height()*scale)+8
	b.Logf("the widest frame draws %d pieces in %dx%d pixels (%.2f megapixels)", len(worst), w, h, float64(w*h)/1e6)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Rasterize(worst, RasterOptions{Width: w, Height: h, Bounds: box, SubSamples: 1})
	}
}

func TestShapeCounts(t *testing.T) {
	rig := CreateRig(RigOptions{})
	pose := PoseOf(rig, 6, Motion{})
	full := BuildDrawList(rig, pose, DrawOptions{Detail: "full", Shadow: true, Palette: rig.Palette})
	lite := BuildDrawList(rig, pose, DrawOptions{Detail: "lite", Shadow: true, Palette: rig.Palette})
	t.Logf("full=%d lite=%d", len(full), len(lite))
}
