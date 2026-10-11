package flower

import (
	"math"
	"sync"
)

// A tiny software rasteriser for the flower's draw list.
//
// It exists so the flower is not a canvas: the same polygons the rig poses are
// scanline-filled here with antialiasing, gradient fills and source-over alpha
// compositing, and the result is handed to the window as pixels. No drawing
// library, no native module, no binary assets in the repo.
//
// Fill types: solid, linear, radial — exactly the vocabulary the rig emits.

// sortSmall is an insertion sort for the handful of crossings a scanline has.
func sortSmall(xs []float64) {
	for i := 1; i < len(xs); i++ {
		v := xs[i]
		j := i - 1
		for j >= 0 && xs[j] > v {
			xs[j+1] = xs[j]
			j--
		}
		xs[j+1] = v
	}
}

func clamp255(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// SampleStops samples a gradient's stops at t, returning r, g, b and alpha.
// A stop's alpha defaults to 1, which is what lets a glow fade out instead of
// ending in a plate edge.
func SampleStops(stops []Stop, t float64) [4]float64 {
	x := clamp01(t)
	if len(stops) == 0 {
		return [4]float64{0, 0, 0, 1}
	}
	if len(stops) == 1 {
		return [4]float64{stops[0].Color[0], stops[0].Color[1], stops[0].Color[2], alphaOf(stops[0])}
	}
	prev := stops[0]
	for i := 1; i < len(stops); i++ {
		next := stops[i]
		if x <= next.At || i == len(stops)-1 {
			span := next.At - prev.At
			f := 1.0
			if span > 1e-9 {
				f = clamp01((x - prev.At) / span)
			}
			a, b := prev.Color, next.Color
			aa, ab := alphaOf(prev), alphaOf(next)
			return [4]float64{
				a[0] + (b[0]-a[0])*f,
				a[1] + (b[1]-a[1])*f,
				a[2] + (b[2]-a[2])*f,
				aa + (ab-aa)*f,
			}
		}
		prev = next
	}
	return [4]float64{prev.Color[0], prev.Color[1], prev.Color[2], alphaOf(prev)}
}

// alphaOf is the stop's alpha. A stop that names no alpha at all is opaque; a
// stop that names zero is genuinely invisible, which is how a glow fades out.
func alphaOf(s Stop) float64 {
	if math.IsNaN(s.Alpha) {
		return 1
	}
	return s.Alpha
}

// GradientColor is the colour of a fill at a point in design space, with alpha.
func GradientColor(fill Fill, x, y float64) [4]float64 {
	switch fill.Type {
	case Solid:
		return [4]float64{fill.Color[0], fill.Color[1], fill.Color[2], 1}
	case Linear:
		dx := fill.To.X - fill.From.X
		dy := fill.To.Y - fill.From.Y
		len2 := dx*dx + dy*dy
		t := 0.0
		if len2 > 1e-12 {
			t = ((x-fill.From.X)*dx + (y-fill.From.Y)*dy) / len2
		}
		return SampleStops(fill.Stops, t)
	default:
		dx := x - fill.Center.X
		dy := y - fill.Center.Y
		r := math.Hypot(dx, dy)
		span := fill.R1 - fill.R0
		t := 1.0
		if span > 1e-9 {
			t = (r - fill.R0) / span
		} else if r <= fill.R0 {
			t = 0
		}
		return SampleStops(fill.Stops, t)
	}
}

// Transform maps design space to pixels: design space has +y up, a buffer has
// +y down, and the drawing is centred in the buffer.
type Transform struct {
	Scale   float64
	offsetX float64
	offsetY float64
}

// MakeTransform fits a design-space box into a buffer, y flipped.
func MakeTransform(b Bounds, width, height int, padding float64) Transform {
	spanX := math.Max(1e-6, b.MaxX-b.MinX)
	spanY := math.Max(1e-6, b.MaxY-b.MinY)
	scale := math.Min((float64(width)-padding*2)/spanX, (float64(height)-padding*2)/spanY)
	offsetX := (float64(width)-spanX*scale)/2 - b.MinX*scale
	offsetY := float64(height) - (float64(height)-spanY*scale)/2 + b.MinY*scale
	return Transform{Scale: scale, offsetX: offsetX, offsetY: offsetY}
}

// ToPixel maps a design-space point onto the buffer.
func (t Transform) ToPixel(x, y float64) (float64, float64) {
	return t.offsetX + x*t.Scale, t.offsetY - y*t.Scale
}

// ToDesign maps a buffer point back into design space.
func (t Transform) ToDesign(px, py float64) (float64, float64) {
	return (px - t.offsetX) / t.Scale, (t.offsetY - py) / t.Scale
}

// RasterOptions is everything the rasteriser takes.
type RasterOptions struct {
	Width, Height int
	Bounds        Bounds
	Padding       float64
	SubSamples    int
	// Background paints an opaque backdrop first, if set.
	Background *RGB
	// Shave skips polygons thinner than this many pixels (veins on a 32px icon).
	// Zero means no shaving, which is what the on-screen renderer wants.
	Shave float64
	// Workers splits the frame into horizontal bands, one per goroutine. Bands
	// are disjoint and shapes are still painted in order within a band, so the
	// pixels are identical to the single-threaded result — just sooner. 0 or 1
	// keeps it single-threaded (which is what the pixel-for-pixel tests want).
	Workers int
}

// Rasterize draws a list into a straight (non-premultiplied) RGBA buffer.
func Rasterize(shapes []Shape, o RasterOptions) []uint8 {
	if o.SubSamples <= 0 {
		o.SubSamples = 3
	}
	width, height := o.Width, o.Height
	out := make([]uint8, width*height*4)
	if o.Background != nil {
		for i := 0; i < width*height; i++ {
			out[i*4] = uint8(math.Round(clamp255(o.Background[0])))
			out[i*4+1] = uint8(math.Round(clamp255(o.Background[1])))
			out[i*4+2] = uint8(math.Round(clamp255(o.Background[2])))
			out[i*4+3] = 255
		}
	}

	workers := o.Workers
	if workers > 1 && width >= 48 && height >= 48 {
		// A flower is not evenly spread over a window — it is a dense blob in
		// the middle and a thin stalk below — so the window is cut into tiles
		// rather than bands: two columns and half as many rows, which puts the
		// bloom on four workers instead of one.
		cols := 2
		rows := max(1, workers/cols)
		if rows*cols < workers {
			rows++
		}
		// Tiles never overlap, so one coverage buffer is enough and nothing
		// needs locking; a tile only ever reads and clears its own pixels.
		coverage := make([]float32, width*height)
		var wg sync.WaitGroup
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				area := tile{
					x0: width * c / cols, x1: width * (c + 1) / cols,
					y0: height * r / rows, y1: height * (r + 1) / rows,
				}
				wg.Add(1)
				go func(t tile) {
					defer wg.Done()
					rasterizeTile(shapes, o, out, coverage, t.x0, t.x1, t.y0, t.y1)
				}(area)
			}
		}
		wg.Wait()
		return out
	}
	coverage := make([]float32, width*height)
	rasterizeTile(shapes, o, out, coverage, 0, width, 0, height)
	return out
}

// tile is one rectangle of the frame a worker is responsible for.
type tile struct{ x0, x1, y0, y1 int }

// rasterizeTile paints the shapes into the pixels [xStart, xEnd) x [yStart,
// yEnd) of an existing buffer. Tiles never overlap, so there is no locking
// anywhere and the pixels are the same as the single-threaded result.
func rasterizeTile(shapes []Shape, o RasterOptions, out []uint8, coverage []float32, xStart, xEnd, yStart, yEnd int) {
	width, height := o.Width, o.Height
	transform := MakeTransform(o.Bounds, width, height, o.Padding)
	// coverage is float32 to match the reference renderer exactly (its
	// accumulation buffer was a Float32Array, and the port has to agree with it
	// pixel for pixel or the pixel-diff test is worthless)
	xs := make([]float64, 0, 32)
	touched := make([]int, 0, 64)

	for _, shape := range shapes {
		if len(shape.Poly) < 3 || shape.Alpha <= 0.002 {
			continue
		}
		// design space -> pixels
		pts := make([]float64, 0, len(shape.Poly)*2)
		minX, minY := math.Inf(1), math.Inf(1)
		maxX, maxY := math.Inf(-1), math.Inf(-1)
		for _, p := range shape.Poly {
			px, py := transform.ToPixel(p.X, p.Y)
			pts = append(pts, px, py)
			minX, maxX = math.Min(minX, px), math.Max(maxX, px)
			minY, maxY = math.Min(minY, py), math.Max(maxY, py)
		}
		if o.Shave > 0 && maxX-minX < o.Shave && maxY-minY < o.Shave {
			continue
		}

		x0 := max(xStart, max(0, int(math.Floor(minX))))
		x1 := min(xEnd-1, min(width-1, int(math.Ceil(maxX))))
		y0 := max(yStart, max(0, int(math.Floor(minY))))
		y1 := min(yEnd-1, min(height-1, int(math.Ceil(maxY))))
		if x1 < x0 || y1 < y0 {
			continue
		}

		n := len(pts) / 2
		touched = touched[:0]
		for y := y0; y <= y1; y++ {
			covered := false
			for s := 0; s < o.SubSamples; s++ {
				sy := float64(y) + (float64(s)+0.5)/float64(o.SubSamples)
				xs = xs[:0]
				for i := 0; i < n; i++ {
					ax, ay := pts[i*2], pts[i*2+1]
					j := (i + 1) % n
					bx, by := pts[j*2], pts[j*2+1]
					if ay == by {
						continue
					}
					if sy >= math.Min(ay, by) && sy < math.Max(ay, by) {
						xs = append(xs, ax+((sy-ay)/(by-ay))*(bx-ax))
					}
				}
				if len(xs) < 2 {
					continue
				}
				// Almost every scanline has two crossings and the rest very few, so
				// an insertion sort beats sort.Float64s by a wide margin here (it was
				// the single hottest thing in the frame). Every crossing has to be
				// sorted, though: a pair arriving out of order fills the wrong span.
				sortSmall(xs)
				for k := 0; k+1 < len(xs); k += 2 {
					sx0, sx1 := xs[k], xs[k+1]
					if sx1 <= float64(x0) || sx0 >= float64(x1+1) {
						continue
					}
					from := max(x0, int(math.Floor(sx0)))
					to := min(x1, int(math.Ceil(sx1))-1)
					for px := from; px <= to; px++ {
						overlap := math.Min(sx1, float64(px+1)) - math.Max(sx0, float64(px))
						if overlap <= 0 {
							continue
						}
						coverage[y*width+px] += float32(overlap / float64(o.SubSamples))
						covered = true
					}
				}
			}
			if covered {
				touched = append(touched, y)
			}
		}
		if len(touched) == 0 {
			continue
		}

		rowFrom := y0
		rowTo := y1
		for y := rowFrom; y <= rowTo; y++ {
			for px := x0; px <= x1; px++ {
				idx := y*width + px
				cov := float64(coverage[idx])
				if cov <= 0 {
					continue
				}
				var color [4]float64
				if shape.Fill.Type == Solid {
					// the common case, and the cheap one: one colour for the whole
					// shape, so no gradients are sampled at all
					color = [4]float64{shape.Fill.Color[0], shape.Fill.Color[1], shape.Fill.Color[2], 1}
				} else {
					dx, dy := transform.ToDesign(float64(px)+0.5, float64(y)+0.5)
					color = GradientColor(shape.Fill, dx, dy)
				}
				a := math.Min(1, cov) * shape.Alpha * color[3]
				off := idx * 4
				dstA := float64(out[off+3]) / 255
				outA := a + dstA*(1-a)
				if outA <= 0 {
					out[off], out[off+1], out[off+2], out[off+3] = 0, 0, 0, 0
					continue
				}
				for c := 0; c < 3; c++ {
					out[off+c] = uint8(math.Round(clamp255((color[c]*a + float64(out[off+c])*dstA*(1-a)) / outA)))
				}
				out[off+3] = uint8(math.Round(outA * 255))
			}
		}
		for _, y := range touched {
			for px := x0; px <= x1; px++ {
				coverage[y*width+px] = 0
			}
		}
	}
}
