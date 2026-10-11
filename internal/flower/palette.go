package flower

import "math"

// Palette is one colourway of the same drawing.
type Palette struct {
	Label     string
	PetalTip  RGB
	PetalBase RGB
	PetalDeep RGB
	PetalEdge RGB
	Center    RGB
	CenterHot RGB
	Stem      RGB
	StemLight RGB
	StemDark  RGB
	Leaf      RGB
}

// Palettes are the four colourways the flower ships in.
var Palettes = map[string]Palette{
	"blush": {
		Label:     "Blush",
		PetalTip:  RGB{252, 208, 228},
		PetalBase: RGB{244, 145, 182},
		PetalDeep: RGB{199, 82, 128},
		PetalEdge: RGB{190, 74, 120},
		Center:    RGB{253, 214, 121},
		CenterHot: RGB{232, 152, 58},
		Stem:      RGB{104, 165, 96},
		StemLight: RGB{170, 219, 148},
		StemDark:  RGB{58, 110, 70},
		Leaf:      RGB{126, 190, 116},
	},
	"buttercup": {
		Label:     "Buttercup",
		PetalTip:  RGB{255, 247, 206},
		PetalBase: RGB{255, 214, 92},
		PetalDeep: RGB{223, 148, 32},
		PetalEdge: RGB{232, 172, 58},
		Center:    RGB{246, 190, 88},
		CenterHot: RGB{186, 108, 34},
		Stem:      RGB{110, 166, 96},
		StemLight: RGB{178, 220, 152},
		StemDark:  RGB{62, 112, 68},
		Leaf:      RGB{132, 190, 112},
	},
	"periwinkle": {
		Label:     "Periwinkle",
		PetalTip:  RGB{232, 236, 255},
		PetalBase: RGB{152, 160, 246},
		PetalDeep: RGB{98, 92, 200},
		PetalEdge: RGB{124, 124, 224},
		Center:    RGB{255, 226, 140},
		CenterHot: RGB{226, 152, 62},
		Stem:      RGB{102, 160, 104},
		StemLight: RGB{168, 214, 156},
		StemDark:  RGB{56, 106, 72},
		Leaf:      RGB{122, 186, 124},
	},
	"daisy": {
		Label:     "Snow Daisy",
		PetalTip:  RGB{255, 255, 255},
		PetalBase: RGB{242, 245, 255},
		PetalDeep: RGB{198, 206, 232},
		PetalEdge: RGB{216, 222, 244},
		Center:    RGB{255, 206, 96},
		CenterHot: RGB{218, 138, 42},
		Stem:      RGB{104, 165, 96},
		StemLight: RGB{172, 219, 152},
		StemDark:  RGB{58, 110, 70},
		Leaf:      RGB{126, 190, 116},
	},
}

// PaletteNames is the stable order palettes are offered in.
var PaletteNames = []string{"blush", "buttercup", "periwinkle", "daisy"}

// HueTurn rotates a whole colourway about the colour wheel.
//
// Four colourways are four flowers; a wheel is any flower. Rotating in HSL keeps
// what the drawing is built on — how light each colour is and how vivid — and
// changes only which part of the spectrum it is, so the shading, the rim light
// and the sun all still land the way they were designed to. The stem gets a much
// smaller turn than the bloom: a flower with a violet stem is a bug, not a
// colourway.
func HueTurn(p Palette, degrees float64) Palette {
	if degrees == 0 {
		return p
	}
	turn := func(c RGB, share float64) RGB { return RotateHue(c, degrees*share) }
	return Palette{
		Label:     p.Label,
		PetalTip:  turn(p.PetalTip, 1),
		PetalBase: turn(p.PetalBase, 1),
		PetalDeep: turn(p.PetalDeep, 1),
		PetalEdge: turn(p.PetalEdge, 1),
		Center:    turn(p.Center, 0.35),
		CenterHot: turn(p.CenterHot, 0.35),
		Stem:      turn(p.Stem, 0.35),
		StemLight: turn(p.StemLight, 0.35),
		StemDark:  turn(p.StemDark, 0.35),
		Leaf:      turn(p.Leaf, 0.35),
	}
}

// RotateHue turns one colour about the wheel by degrees, keeping its lightness
// and its saturation.
func RotateHue(c RGB, degrees float64) RGB {
	h, s, l := rgbToHSL(c)
	h = math.Mod(h+degrees/360, 1)
	if h < 0 {
		h += 1
	}
	return hslToRGB(h, s, l)
}

// rgbToHSL is the usual conversion, with h in turns rather than radians.
func rgbToHSL(c RGB) (h, s, l float64) {
	r, g, b := c[0]/255, c[1]/255, c[2]/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (mx + mn) / 2
	if mx == mn {
		return 0, 0, l
	}
	d := mx - mn
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	switch mx {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h / 6, s, l
}

// hslToRGB is its inverse.
func hslToRGB(h, s, l float64) RGB {
	if s == 0 {
		v := l * 255
		return RGB{v, v, v}
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	channel := func(t float64) float64 {
		if t < 0 {
			t++
		}
		if t > 1 {
			t--
		}
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 1.0/2:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	return RGB{channel(h+1.0/3) * 255, channel(h) * 255, channel(h-1.0/3) * 255}
}

// PaletteNamed returns a palette, falling back to blush for an unknown name.
func PaletteNamed(name string) Palette {
	if p, ok := Palettes[name]; ok {
		return p
	}
	return Palettes["blush"]
}

// Tint scales a colour by a factor, clamped to the range colours live in.
func Tint(c RGB, f float64) RGB {
	return RGB{clamp(c[0]*f, 0, 255), clamp(c[1]*f, 0, 255), clamp(c[2]*f, 0, 255)}
}

// Sun is where the light comes from. Every shading decision in the drawing
// already assumed the upper left, so this is where it becomes explicit.
var Sun = Point{X: -0.42, Y: 0.91}

// InSun lights a colour: brighter, and warmer the more the surface faces the
// light. `facing` is -1 (turned away) to 1 (square on to the sun).
func InSun(c RGB, k, facing float64) RGB {
	warm := 1 + 0.05*facing
	return RGB{
		clamp(c[0]*k*warm, 0, 255),
		clamp(c[1]*k*(1+0.014*facing), 0, 255),
		clamp(c[2]*k*(1-0.035*facing), 0, 255),
	}
}

// Star is an eight-point sparkle: what a flower in the sun does in a photograph.
func Star(center Point, r, inner float64) []Point {
	poly := make([]Point, 0, 8)
	for i := 0; i < 8; i++ {
		a := (float64(i) / 8) * TAU
		rad := r
		if i%2 != 0 {
			rad = r * inner
		}
		poly = append(poly, Point{center.X + math.Cos(a)*rad, center.Y + math.Sin(a)*rad})
	}
	return poly
}
