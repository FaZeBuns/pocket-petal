package flower

import (
	"testing"
)

func benchRig() (*Rig, Pose) {
	rig := CreateRig(RigOptions{Seed: ptrInt(11)})
	return rig, PoseOf(rig, 6, Motion{Lean: -0.4, Swing: -0.5})
}

func ptrInt(v int) *int { return &v }

func BenchmarkPose(b *testing.B) {
	rig, _ := benchRig()
	for i := 0; i < b.N; i++ {
		PoseOf(rig, 6, Motion{Lean: -0.4, Swing: -0.5, Flutter: 1})
	}
}

func BenchmarkDrawList(b *testing.B) {
	rig, pose := benchRig()
	for i := 0; i < b.N; i++ {
		BuildDrawList(rig, pose, DrawOptions{Detail: "full", Shadow: true})
	}
}

// BenchmarkAppFrame is the frame the app actually paints: the flower at its own
// size, into the window FrameFor derived for it. The other benchmarks rasterise a
// tightly fitted 190x250 box, which is a different question: most of the app's
// window is air, and air is most of what a frame costs.
func BenchmarkAppFrame(b *testing.B) {
	rig, pose := benchRig()
	frame := FrameFor(rig, 190, 250, 0.94)
	list := BuildDrawList(rig, pose, DrawOptions{Detail: "full", Shadow: true})
	opts := RasterOptions{
		Width: frame.Width, Height: frame.Height, Bounds: frame.Bounds,
		SubSamples: 2, Workers: 8,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Rasterize(list, opts)
	}
}

func BenchmarkRasterizeWindow(b *testing.B) {
	rig, pose := benchRig()
	list := BuildDrawList(rig, pose, DrawOptions{Detail: "full", Shadow: true})
	bounds := ListBounds(list)
	opts := RasterOptions{Width: 190, Height: 250, Bounds: bounds, Padding: 8, SubSamples: 2}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Rasterize(list, opts)
	}
}

func BenchmarkRasterizeNoHalo(b *testing.B) {
	rig, pose := benchRig()
	list := BuildDrawList(rig, pose, DrawOptions{Detail: "full", Shadow: true})
	trimmed := make([]Shape, 0, len(list))
	for _, s := range list {
		if s.Part == "glow" || s.Part == "shadow" {
			continue
		}
		trimmed = append(trimmed, s)
	}
	bounds := ListBounds(list)
	opts := RasterOptions{Width: 190, Height: 250, Bounds: bounds, Padding: 8, SubSamples: 2}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Rasterize(trimmed, opts)
	}
}

func BenchmarkRasterizeWindowParallel(b *testing.B) {
	rig, pose := benchRig()
	list := BuildDrawList(rig, pose, DrawOptions{Detail: "full", Shadow: true})
	bounds := ListBounds(list)
	opts := RasterOptions{Width: 190, Height: 250, Bounds: bounds, Padding: 8, SubSamples: 2, Workers: 8}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Rasterize(list, opts)
	}
}
