package flower

import "math"

// TAU is a full turn.
const TAU = math.Pi * 2

// Golden is the golden angle: the spacing a sunflower's seeds and petals use.
var Golden = math.Pi * (3 - math.Sqrt(5)) // 2.399963...

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clamp01(v float64) float64 { return clamp(v, 0, 1) }

// num turns anything that is not a real number into zero. The maths here must
// never be able to draw a broken flower, whatever a frame hands in.
func num(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func easeOutCubic(t float64) float64 {
	u := 1 - clamp01(t)
	return 1 - u*u*u
}

// EaseOutBack overshoots past 1 then settles back: the spring in a growing stem.
func EaseOutBack(t, s float64) float64 {
	p := clamp01(t) - 1
	return 1 + (s+1)*p*p*p + s*p*p
}

// RNG is mulberry32: the same tiny deterministic generator the reference
// implementation used, so seeds pick the same flower in both.
type RNG struct{ a uint32 }

// NewRNG seeds the generator.
func NewRNG(seed int) *RNG { return &RNG{a: uint32(seed)} }

// Next returns the next value in [0, 1).
func (r *RNG) Next() float64 {
	r.a += 0x6d2b79f5
	t := r.a
	t = (t ^ (t >> 15)) * (1 | t)
	t = (t + (t^(t>>7))*(61|t)) ^ t
	return float64(t^(t>>14)) / 4294967296
}

// Ribbon is a tapered band around a polyline: the stem, a midrib, a vein.
// halfWidth(i / (n-1)) is the half width in design units. No caps: callers taper
// to zero at the tip.
func Ribbon(points []Point, halfWidth func(float64) float64) []Point {
	n := len(points)
	left := make([]Point, 0, n)
	right := make([]Point, 0, n)
	for i := 0; i < n; i++ {
		p := points[i]
		prev := points[max(0, i-1)]
		next := points[min(n-1, i+1)]
		tx := next.X - prev.X
		ty := next.Y - prev.Y
		length := math.Hypot(tx, ty)
		if length == 0 {
			length = 1
		}
		tx /= length
		ty /= length
		s := 0.0
		if n > 1 {
			s = float64(i) / float64(n-1)
		}
		hw := math.Max(0, halfWidth(s))
		left = append(left, Point{p.X - ty*hw, p.Y + tx*hw})
		right = append(right, Point{p.X + ty*hw, p.Y - tx*hw})
	}
	poly := make([]Point, 0, 2*n)
	poly = append(poly, left...)
	for i := len(right) - 1; i >= 0; i-- {
		poly = append(poly, right[i])
	}
	return poly
}

// QuadPoint is a point on a quadratic Bézier.
func QuadPoint(p0, p1, p2 Point, s float64) Point {
	u := 1 - s
	return Point{
		X: u*u*p0.X + 2*u*s*p1.X + s*s*p2.X,
		Y: u*u*p0.Y + 2*u*s*p1.Y + s*s*p2.Y,
	}
}

// BladeOptions describes a petal or leaf blade.
type BladeOptions struct {
	Tip       Point
	HalfWidth float64
	Steps     int
	Profile   string // "petal" or "leaf"
	Notch     float64
	Serration float64
	BendAt    float64
	// TipEnd is how far along the centreline the blade ends (0.96 default for a
	// petal, 1 for a leaf).
	TipEnd float64
	Bend   float64
}

// Blade is a petal or a leaf: a curved centreline with a shaped width profile,
// closed with either a rounded cap (a daisy petal) or a point (a leaf). Both are
// the same maths with different profiles, which is why they look like parts of
// one drawing.
func Blade(o BladeOptions) []Point {
	steps := o.Steps
	if steps <= 0 {
		steps = 12
	}
	bendAt := o.BendAt
	if bendAt == 0 {
		bendAt = 0.55
	}
	p0 := Point{0, 0}
	length := math.Hypot(o.Tip.X, o.Tip.Y)
	if length == 0 {
		length = 1
	}
	perpX := -o.Tip.Y / length
	perpY := o.Tip.X / length
	p1 := Point{o.Tip.X*bendAt + perpX*o.Bend, o.Tip.Y*bendAt + perpY*o.Bend}
	p2 := o.Tip
	end := o.TipEnd
	if end == 0 {
		if o.Profile == "leaf" {
			end = 1
		} else {
			end = 0.96
		}
	}
	widthAt := func(s float64) float64 {
		if o.Profile == "leaf" {
			return o.HalfWidth * math.Pow(math.Sin(math.Pi*math.Pow(s, 0.88)), 0.6)
		}
		// a fuller petal: the width holds almost to the tip, and the cap closes
		// it in a soft round — the difference between a petal and a paper spike
		return o.HalfWidth * math.Pow(math.Sin(math.Pi*math.Pow(s, 0.66)), 0.46)
	}
	at := func(s float64) Point { return QuadPoint(p0, p1, p2, s) }
	tangentAt := func(s float64) Point {
		a := at(math.Max(0, s-0.02))
		b := at(math.Min(1, s+0.02))
		dx := b.X - a.X
		dy := b.Y - a.Y
		l := math.Hypot(dx, dy)
		if l == 0 {
			l = 1
		}
		return Point{dx / l, dy / l}
	}

	left := make([]Point, 0, steps+1)
	right := make([]Point, 0, steps+1)
	for i := 0; i <= steps; i++ {
		s := (float64(i) / float64(steps)) * end
		p := at(s)
		d := tangentAt(s)
		w := widthAt(s)
		if o.Serration != 0 {
			w *= 1 + o.Serration*math.Sin(s*TAU*4.2)
		}
		left = append(left, Point{p.X - d.Y*w, p.Y + d.X*w})
		right = append(right, Point{p.X + d.Y*w, p.Y - d.X*w})
	}

	// The cap: sweep the width around the end of the centreline, dipping the
	// middle slightly for a petal's notch.
	ex := at(end)
	tt := tangentAt(end)
	capR := widthAt(end)
	cap := []Point{}
	if capR > 0.01 {
		capSteps := 7
		if steps >= 10 {
			capSteps = 10
		}
		for i := 1; i < capSteps; i++ {
			th := (float64(i) / float64(capSteps)) * math.Pi
			c := math.Cos(th)
			sn := math.Sin(th)
			px := ex.X + -tt.Y*capR*c + tt.X*capR*sn
			py := ex.Y + tt.X*capR*c + tt.Y*capR*sn
			if o.Notch > 0 && th > math.Pi*0.3 && th < math.Pi*0.7 {
				k := math.Min(capR*0.55, capR*o.Notch)
				px -= tt.X * k
				py -= tt.Y * k
			}
			cap = append(cap, Point{px, py})
		}
	}

	poly := make([]Point, 0, len(left)+len(cap)+len(right))
	poly = append(poly, left...)
	poly = append(poly, cap...)
	for i := len(right) - 1; i >= 0; i-- {
		poly = append(poly, right[i])
	}
	return poly
}

// BladeCentre is the centreline a blade was built around, so ribs and veins
// follow the same curl as the petal they are drawn on.
func BladeCentre(tip Point, s, bend, bendAt float64) Point {
	if bendAt == 0 {
		bendAt = 0.55
	}
	length := math.Hypot(tip.X, tip.Y)
	if length == 0 {
		length = 1
	}
	p1 := Point{tip.X*bendAt + (-tip.Y/length)*bend, tip.Y*bendAt + (tip.X/length)*bend}
	return QuadPoint(Point{0, 0}, p1, tip, s)
}

// Ellipse is a closed polygon approximating an ellipse.
func Ellipse(center Point, rx, ry float64, steps int) []Point {
	if steps <= 0 {
		steps = 20
	}
	poly := make([]Point, 0, steps)
	for i := 0; i < steps; i++ {
		a := (float64(i) / float64(steps)) * TAU
		poly = append(poly, Point{center.X + math.Cos(a)*rx, center.Y + math.Sin(a)*ry})
	}
	return poly
}

// TransformPoly moves and rotates a polygon into place.
func TransformPoly(poly []Point, origin Point, angle float64) []Point {
	c := math.Cos(angle)
	s := math.Sin(angle)
	out := make([]Point, len(poly))
	for i, p := range poly {
		out[i] = Point{origin.X + p.X*c - p.Y*s, origin.Y + p.X*s + p.Y*c}
	}
	return out
}

// BoundsOf is the box around one polygon.
func BoundsOf(poly []Point) Bounds {
	b := Bounds{MinX: math.Inf(1), MinY: math.Inf(1), MaxX: math.Inf(-1), MaxY: math.Inf(-1)}
	for _, p := range poly {
		if p.X < b.MinX {
			b.MinX = p.X
		}
		if p.Y < b.MinY {
			b.MinY = p.Y
		}
		if p.X > b.MaxX {
			b.MaxX = p.X
		}
		if p.Y > b.MaxY {
			b.MaxY = p.Y
		}
	}
	return b
}

// ListBounds is the box around a whole draw list.
func ListBounds(list []Shape) Bounds {
	box := Bounds{MinX: math.Inf(1), MinY: math.Inf(1), MaxX: math.Inf(-1), MaxY: math.Inf(-1)}
	for _, shape := range list {
		b := BoundsOf(shape.Poly)
		if b.MinX < box.MinX {
			box.MinX = b.MinX
		}
		if b.MinY < box.MinY {
			box.MinY = b.MinY
		}
		if b.MaxX > box.MaxX {
			box.MaxX = b.MaxX
		}
		if b.MaxY > box.MaxY {
			box.MaxY = b.MaxY
		}
	}
	return box
}

// StopA is one gradient stop with an explicit alpha. Alpha 0 is a real, fully
// transparent stop: that is how a glow fades out instead of ending in a plate
// edge, so it is never second-guessed.
func StopA(at float64, c RGB, alpha float64) Stop {
	return Stop{At: at, Color: c, Alpha: alpha}
}

// FillSolid, FillLinear and FillRadial build the three fills.
func FillSolid(color RGB) Fill { return Fill{Type: Solid, Color: color} }

// FillLinear builds a linear gradient from one point to another.
func FillLinear(from, to Point, stops []Stop) Fill {
	return Fill{Type: Linear, From: from, To: to, Stops: stops}
}

// FillRadial builds a radial gradient about a centre.
func FillRadial(center Point, r0, r1 float64, stops []Stop) Fill {
	return Fill{Type: Radial, Center: center, R0: r0, R1: r1, Stops: stops}
}
