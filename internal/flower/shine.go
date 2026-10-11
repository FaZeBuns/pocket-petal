package flower

import "math"

// The sun passing over the flower.
//
// The reference renderer keeps a halo behind the head all the time, and on a
// desktop that does not work: the halo is wider than the flower's little window,
// so the only part that ever reaches the screen is the middle of a big soft
// ellipse, cut off square by the window's edge. What that looks like is a grey
// blob behind the flower, not a glow.
//
// So the light is an event here instead of a layer. Every so often a band of
// warm sunlight sweeps across the flower, and a few glints catch on the petals
// as it passes. Nothing is drawn where the flower is not, because the light is
// added *into* the picture the flower has already painted and weighted by the
// flower's own alpha: the shine can only ever brighten flower, never the gaps
// between the petals.

// The shape of one pass: a narrow core with a wide, faint sheen around it, so it
// reads as sunlight and not as a moving stripe.
const (
	shineSigma    = 0.13  // half-width of the core band, in window diagonals
	shineStrength = 0.62  // how bright the core gets
	shineSheen    = 0.30  // how bright the wide pass is, relative to the core
	shineWiden    = 2.7   // the wide pass is this many times wider
	shineTravel   = 0.80  // how far across the window the band travels
	shineFrom     = 0.10  // where in the window the band starts
	glintReach    = 9.0   // the radius of a glint, in pixels
	glintCount    = 7     // how many glints a pass has
	glintSpread   = 0.55  // how much of the pass the glints are staggered over
	glintStreng   = 0.85  // how bright a glint gets
	shineWarmR    = 255.0 // sunlight, warm rather than white
	shineWarmG    = 246.0
	shineWarmB    = 214.0
)

// ApplyShine adds one pass of light to a finished frame, in place.
//
// Phase runs 0 at the start of the pass to 1 at the end; anything outside that
// draws nothing, so a caller can hand in a raw clock without checking it. The
// frame is straight (non-premultiplied) RGBA, the same buffer Rasterize returns.
func ApplyShine(pix []uint8, width, height int, phase float64) {
	if width <= 0 || height <= 0 || len(pix) < width*height*4 {
		return
	}
	if phase <= 0 || phase >= 1 {
		return
	}
	// in and out, so the light is never switched on or off mid-frame
	envelope := math.Sin(math.Pi * phase)
	if envelope <= 0.002 {
		return
	}

	// The band travels down and to the right, the way sun moves across a table.
	// Projecting a pixel onto that direction and normalising by the window's own
	// extent gives a coordinate where 0 is the top-left corner and 1 the far
	// corner, whatever the window's size or shape.
	const dx, dy = 0.64, 0.77
	span := float64(width)*dx + float64(height)*dy // The window is much bigger than the flower — it holds every pose the carry
	// can reach — so the band is swept across the flower's own band of the
	// window: it arrives from beyond one edge of the bloom and leaves past the
	// other, and the envelope turns that into a fade rather than a switch.
	sweep := shineFrom + phase*shineTravel
	coreSigma := shineSigma
	wideSigma := shineSigma * shineWiden
	reach := 4 * wideSigma

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			off := (y*width + x) * 4
			a := float64(pix[off+3]) / 255
			if a <= 0 {
				continue
			}
			d := (float64(x)*dx + float64(y)*dy) / span
			delta := d - sweep
			if delta > reach || delta < -reach {
				continue
			}
			k := shineStrength * math.Exp(-(delta*delta)/(2*coreSigma*coreSigma))
			k += shineStrength * shineSheen * math.Exp(-(delta*delta)/(2*wideSigma*wideSigma))
			// the flower's alpha is what decides how much light it can take:
			// a soft edge is lit softly, and a gap is not lit at all
			k *= envelope * a
			if k <= 0.004 {
				continue
			}
			addLight(pix, off, k)
		}
	}

	glints(pix, width, height, phase, envelope)
}

// glints are the small bright catches where the sun meets a shiny petal. They
// light up one after another along the sweep, which is what turns a band of
// light into something being lit.
func glints(pix []uint8, width, height int, phase, envelope float64) {
	for i, at := range glintSpots(pix, width, height) {
		// each glint has its own moment inside the pass
		span := 1 - glintSpread
		t := (phase - float64(i)/float64(glintCount)*glintSpread) / span
		if t <= 0 || t >= 1 {
			continue
		}
		brightness := math.Sin(math.Pi*t) * envelope * glintStreng
		if brightness <= 0.01 {
			continue
		}
		r := glintReach * (0.75 + 0.25*math.Sin(math.Pi*t))
		cx, cy := int(at.X), int(at.Y)
		reach := int(r) + 1
		x0, x1 := max(0, cx-reach), min(width-1, cx+reach)
		y0, y1 := max(0, cy-reach), min(height-1, cy+reach)
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				off := (y*width + x) * 4
				a := float64(pix[off+3]) / 255
				if a <= 0 {
					continue
				}
				gx, gy := float64(x-cx), float64(y-cy)
				// a four-pointed star: long and thin one way, long and thin the
				// other, added together
				thin := r * 0.30
				star := math.Exp(-(gx*gx)/(2*thin*thin)) * math.Exp(-(gy*gy)/(2*r*r))
				star += math.Exp(-(gy*gy)/(2*thin*thin)) * math.Exp(-(gx*gx)/(2*r*r))
				k := star * brightness * a
				if k <= 0.004 {
					continue
				}
				addLight(pix, off, k)
			}
		}
	}
}

// glintSpots picks the pixels a glint can sit on: a coarse grid of the frame,
// keeping the first pixel in each cell that the flower actually covers. Sampling
// the frame this way means the glints always land on the flower, whatever the
// seed, the palette or how far round the bloom has turned.
func glintSpots(pix []uint8, width, height int) []Point {
	step := max(4, min(width, height)/7)
	var spots []Point
	for y := step / 2; y < height; y += step {
		for x := step / 2; x < width; x += step {
			if pix[(y*width+x)*4+3] > 180 {
				spots = append(spots, Point{float64(x), float64(y)})
			}
		}
	}
	if len(spots) == 0 {
		return nil
	}
	if len(spots) <= glintCount {
		return spots
	}
	// spread the glints over the whole flower rather than clustering them in the
	// first few cells the scan happens to find
	picked := make([]Point, 0, glintCount)
	for i := 0; i < glintCount; i++ {
		picked = append(picked, spots[i*len(spots)/glintCount])
	}
	return picked
}

// addLight mixes a little warm sunlight into one pixel, leaving its alpha alone:
// the shine brightens the flower, it never changes its shape.
func addLight(pix []uint8, off int, k float64) {
	if k > 0.92 {
		k = 0.92
	}
	pix[off+0] = mixChannel(pix[off+0], shineWarmR, k)
	pix[off+1] = mixChannel(pix[off+1], shineWarmG, k)
	pix[off+2] = mixChannel(pix[off+2], shineWarmB, k)
}

func mixChannel(dst uint8, target, k float64) uint8 {
	v := float64(dst) + (target-float64(dst))*k
	if v > 255 {
		v = 255
	}
	return uint8(v + 0.5)
}
