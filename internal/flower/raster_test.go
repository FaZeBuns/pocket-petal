package flower

import (
	"math"
	"testing"
)

func squareAt(size float64, color RGB, alpha float64) Shape {
	return Shape{
		Part:  "test",
		Poly:  []Point{{-size, -size}, {size, -size}, {size, size}, {-size, size}},
		Fill:  FillSolid(color),
		Alpha: alpha,
	}
}

func pixelAt(rgba []uint8, width, x, y int) [4]uint8 {
	o := (y*width + x) * 4
	return [4]uint8{rgba[o], rgba[o+1], rgba[o+2], rgba[o+3]}
}

func TestSolidSquareFillsAndLeavesTheOutsideEmpty(t *testing.T) {
	out := Rasterize([]Shape{squareAt(1, RGB{255, 0, 0}, 1)}, RasterOptions{
		Width: 16, Height: 16, Bounds: Bounds{-1.6, -1.6, 1.6, 1.6},
	})
	centre := pixelAt(out, 16, 8, 8)
	if centre[0] != 255 || centre[1] != 0 || centre[2] != 0 || centre[3] != 255 {
		t.Fatalf("centre should be solid red, got %v", centre)
	}
	if corner := pixelAt(out, 16, 0, 0); corner[3] != 0 {
		t.Fatalf("the corner should be empty, got %v", corner)
	}
}

func TestEdgesAreAntialiased(t *testing.T) {
	diamond := Shape{
		Part:  "test",
		Poly:  []Point{{0, 1}, {1, 0}, {0, -1}, {-1, 0}},
		Fill:  FillSolid(RGB{0, 0, 255}),
		Alpha: 1,
	}
	out := Rasterize([]Shape{diamond}, RasterOptions{
		Width: 32, Height: 32, Bounds: Bounds{-1.2, -1.2, 1.2, 1.2},
	})
	partial := 0
	for i := 3; i < len(out); i += 4 {
		if out[i] > 4 && out[i] < 251 {
			partial++
		}
	}
	if partial < 8 {
		t.Fatalf("expected antialiased edge pixels, got %d", partial)
	}
}

func TestAlphaCompositesSourceOver(t *testing.T) {
	shapes := []Shape{
		squareAt(1, RGB{0, 0, 0}, 1),
		squareAt(1, RGB{255, 255, 255}, 0.5),
	}
	out := Rasterize(shapes, RasterOptions{Width: 16, Height: 16, Bounds: Bounds{-1.6, -1.6, 1.6, 1.6}})
	px := pixelAt(out, 16, 8, 8)
	if px[0] < 120 || px[0] > 136 {
		t.Fatalf("white over black at 50%% should be about 128, got %d", px[0])
	}
	if px[3] != 255 {
		t.Fatalf("opaque underneath stays opaque, got %d", px[3])
	}
}

// A gradient stop can carry its own alpha, which is the only way to draw a glow
// that fades out instead of ending in a plate edge.
func TestGradientsSampleInDesignSpace(t *testing.T) {
	linear := GradientColor(FillLinear(Point{0, 0}, Point{10, 0}, []Stop{
		StopA(0, RGB{0, 0, 0}, 1),
		StopA(1, RGB{100, 0, 0}, 1),
	}), 5, 0)
	if math.Abs(linear[0]-50) > 0.001 {
		t.Fatalf("halfway along a 0..100 ramp should be 50, got %v", linear[0])
	}
	radial := GradientColor(FillRadial(Point{0, 0}, 0, 10, []Stop{
		StopA(0, RGB{0, 0, 0}, 1),
		StopA(1, RGB{200, 0, 0}, 1),
	}), 5, 0)
	if math.Abs(radial[0]-100) > 0.001 {
		t.Fatalf("halfway out a radial ramp should be 100, got %v", radial[0])
	}
	beyond := GradientColor(FillRadial(Point{0, 0}, 0, 10, []Stop{
		StopA(0, RGB{0, 0, 0}, 1),
		StopA(1, RGB{200, 0, 0}, 1),
	}), 30, 0)
	if beyond[0] != 200 {
		t.Fatalf("a radial gradient clamps past its outer radius, got %v", beyond[0])
	}
	faded := GradientColor(FillRadial(Point{0, 0}, 0, 10, []Stop{
		StopA(0, RGB{255, 255, 255}, 1),
		StopA(1, RGB{255, 255, 255}, 0),
	}), 10, 0)
	if faded[3] != 0 {
		t.Fatalf("the last stop's alpha should be honoured, got %v", faded[3])
	}
}

func TestMakeTransformFitsAndFlips(t *testing.T) {
	transform := MakeTransform(Bounds{-10, 0, 10, 20}, 100, 200, 0)
	if transform.Scale != 5 {
		t.Fatalf("a 20x20 box in a 100x200 buffer scales by 5, got %v", transform.Scale)
	}
	px, py := transform.ToPixel(0, 20)
	if math.Abs(px-50) > 1e-9 || math.Abs(py-50) > 1e-9 {
		t.Fatalf("the top of design space should be the top of the drawing, got %v,%v", px, py)
	}
	_, bottom := transform.ToPixel(0, 0)
	if math.Abs(bottom-150) > 1e-9 {
		t.Fatalf("the base belongs at the bottom of the box, got %v", bottom)
	}
	bx, by := transform.ToDesign(px, py)
	if math.Abs(bx) > 1e-9 || math.Abs(by-20) > 1e-9 {
		t.Fatalf("design<->pixel should round trip, got %v,%v", bx, by)
	}
	padded := MakeTransform(Bounds{-10, 0, 10, 20}, 100, 100, 2)
	if padded.Scale >= transform.Scale {
		t.Fatal("padding should shrink the drawing")
	}
}

// The two threading paths have to agree exactly: bands are an optimisation, not
// a different renderer.
func TestParallelRenderingIsIdentical(t *testing.T) {
	seed := 11
	rig := CreateRig(RigOptions{Seed: &seed})
	pose := PoseOf(rig, 6, Motion{Lean: -0.5, Swing: -0.6, Flutter: 1})
	list := BuildDrawList(rig, pose, DrawOptions{Detail: "full", Shadow: true})
	bounds := ListBounds(list)
	opts := RasterOptions{Width: 190, Height: 250, Bounds: bounds, Padding: 8, SubSamples: 2}

	single := Rasterize(list, opts)
	opts.Workers = 8
	parallel := Rasterize(list, opts)
	if len(single) != len(parallel) {
		t.Fatalf("different buffer sizes: %d vs %d", len(single), len(parallel))
	}
	for i := range single {
		if single[i] != parallel[i] {
			t.Fatalf("byte %d differs between one worker and eight: %d vs %d", i, single[i], parallel[i])
		}
	}
}

func TestGradientAndSolidFillPathsAgree(t *testing.T) {
	// a "solid" fill written as a gradient has to land on the same pixels, or the
	// fast path would be a different renderer
	solid := Shape{Part: "test", Poly: Ellipse(Point{0, 0}, 1, 1, 24), Fill: FillSolid(RGB{200, 60, 90}), Alpha: 0.8}
	asGradient := solid
	asGradient.Fill = FillLinear(Point{-1, -1}, Point{1, 1}, []Stop{
		StopA(0, RGB{200, 60, 90}, 1),
		StopA(1, RGB{200, 60, 90}, 1),
	})
	opts := RasterOptions{Width: 40, Height: 40, Bounds: Bounds{-1.3, -1.3, 1.3, 1.3}}
	a := Rasterize([]Shape{solid}, opts)
	b := Rasterize([]Shape{asGradient}, opts)
	for i := range a {
		if diff := int(a[i]) - int(b[i]); diff > 1 || diff < -1 {
			t.Fatalf("byte %d: solid %d vs gradient %d", i, a[i], b[i])
		}
	}
}

func TestFlowerRendersAGreenStemAndAPinkBloom(t *testing.T) {
	seed := 11
	rig := CreateRig(RigOptions{Seed: &seed})
	pose := PoseOf(rig, 6, Motion{})
	list := BuildDrawList(rig, pose, DrawOptions{Detail: "full", Shadow: true})
	bounds := ListBounds(list)
	const size = 128
	rgba := Rasterize(list, RasterOptions{Width: size, Height: size, Bounds: bounds, Padding: 4, SubSamples: 3})

	green, pink, opaque := 0, 0, 0
	for i := 0; i < len(rgba); i += 4 {
		r, g, b, a := int(rgba[i]), int(rgba[i+1]), int(rgba[i+2]), int(rgba[i+3])
		if a < 32 {
			continue
		}
		opaque++
		if g > r+12 && g > b+12 {
			green++
		}
		if r > g+20 && b > g-10 && r > 120 {
			pink++
		}
	}
	// A flower is mostly air inside its own box — a bare stem is a long thin
	// thing in the middle of a big rectangle — and a longer stem means more air,
	// so this is a floor against a speck rather than a measure of the drawing.
	if opaque < size*size*10/100 {
		t.Fatalf("the flower covers too few pixels: %d", opaque)
	}
	if green < 200 {
		t.Fatalf("the stem and leaves should be green: %d px", green)
	}
	if pink < 600 {
		t.Fatalf("the petals should be pink: %d px", pink)
	}
	if rgba[3] != 0 {
		t.Fatal("the top-left corner should be transparent")
	}
}
