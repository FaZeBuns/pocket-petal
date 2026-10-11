package flower

import (
	"math"
	"math/rand"
)

// The bloom burst: a delivered flower comes apart, and the petals go everywhere.
//
// Nothing is invented here. The petals are the shapes the flower was already
// wearing, lifted out of its own frame — same polygon, same size, same fill —
// so the drop does not switch from one animation to another: the bloom *becomes*
// the flight. Each piece keeps its own weight, gets a push away from the middle
// of the flower, a spin, and a pull towards the ground, and fades at the end of
// its own life so the burst thins out instead of disappearing all at once.

// PetalPiece is one piece in flight, in the flower's own design space (y up).
type PetalPiece struct {
	Poly  []Point // the petal's polygon, centred on its own middle
	Fill  Fill
	Alpha float64
	At    Point // where that middle is now
	VX    float64
	VY    float64
	Angle float64
	Spin  float64
	Scale float64
	Life  float64
	Hold  float64 // seconds before the piece starts to fade
	Fade  float64 // seconds the fade takes
	Dead  bool
}

// Burst is a live explosion.
type Burst struct {
	Pieces  []PetalPiece
	Gravity float64 // design units per second squared, downwards
	Air     float64 // per-second damping on the pieces
	Elapsed float64
	Centre  Point // the middle of the bloom it came out of
	Tint    RGB   // the colour of the flash: the flower's own
}

// The flash is the first quarter of a second of a burst: a soft disc in the
// flower's own colour opening out of the middle of the bloom and thinning as it
// goes. It is what turns a shower of petals into an explosion — the petals say
// "the flower came apart", and the flash says "it went off".
const (
	flashLife = 0.30
	flashFrom = 40.0
	flashTo   = 340.0
)

// burst tuning: a piece leaves the flower hard, slows in the air, and is gone
// inside about a second.
//
// The throw is the whole character of a drop. A petal is pushed away from the
// middle of the bloom, and the hand's own velocity is added *to all of them*
// (see NewBurst), so an explosion carries the momentum of the drag that caused it:
// flick the flower to the right and the shower goes right, because that is what it
// looked like the flower did. The reach budget in the app is what keeps that from
// meaning "a surface the size of the desk" (see Burst.Flight).
//
// And it is tuned for *area*, not for the size of the pieces. What a person sees
// when a flower goes off is how far the petals travel, so the speeds are high and
// the fall is gentle: a heavy fall drags the whole cloud straight down and eats the
// reach the petals needed to go outwards, which is what made an explosion read as a
// small puff with big petals in it rather than as a circle.
const (
	burstGravity  = 760.0
	burstAir      = 0.90
	burstSpeedMin = 430.0
	burstSpeedMax = 1020.0
	burstSpinMax  = 10.5
	burstLift     = 230.0 // a little pop upwards before gravity takes over
	burstHoldMin  = 0.18
	burstHoldMax  = 0.46
	burstFade     = 0.60
	// how much of the hand's own velocity the shower takes with it
	burstMomentum = 0.72
)

// PetalPieces lifts the petals out of a drawn frame: each one centred on its own
// middle, which is the only thing a piece in flight needs to know about where it
// came from.
func PetalPieces(list []Shape) []PetalPiece {
	var out []PetalPiece
	for _, shape := range list {
		if shape.Part != "petal" || len(shape.Poly) < 3 {
			continue
		}
		var cx, cy float64
		for _, p := range shape.Poly {
			cx += p.X
			cy += p.Y
		}
		n := float64(len(shape.Poly))
		cx, cy = cx/n, cy/n
		local := make([]Point, len(shape.Poly))
		for i, p := range shape.Poly {
			local[i] = Point{p.X - cx, p.Y - cy}
		}
		out = append(out, PetalPiece{
			Poly:  local,
			Fill:  shape.Fill,
			Alpha: shape.Alpha,
			At:    Point{cx, cy},
		})
	}
	return out
}

// Confetti is a shower of small petals in the flower's own colours.
//
// The pieces are cut from the palette rather than lifted off the flower, because
// the flower is not what is being thrown: it stays whole, and the burst comes off
// it like a cheer. One flower's colour, worn by every piece, is the whole point —
// an explosion that reads as one gesture rather than as party confetti.
func Confetti(rig *Rig, centre Point, count int, rng *rand.Rand) []PetalPiece {
	if count <= 0 {
		return nil
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(int64(count)))
	}
	pal := HueTurn(PaletteNamed(rig.Palette), rig.Params.Hue)
	// three tones of the one colour, so the pieces catch the light differently
	// without ever becoming a different flower
	tones := []RGB{pal.PetalDeep, pal.PetalBase, pal.PetalTip}
	pieces := make([]PetalPiece, 0, count)
	for i := 0; i < count; i++ {
		// A petal, not confetti: the size is deliberately steady — an explosion of
		// a hundred different shapes reads as party confetti, and the same petal
		// over and over reads as the flower. It is also deliberately *small*: what
		// makes an explosion feel big is the area it covers, and petals that fill
		// that area themselves take the area away (see the throw, below).
		length := 14 + rng.Float64()*7
		half := length * (0.38 + 0.08*rng.Float64())
		tone := tones[i%len(tones)]
		ang := rng.Float64() * 2 * math.Pi
		tip := Point{X: math.Cos(ang) * length, Y: math.Sin(ang) * length}
		poly := Blade(BladeOptions{
			Tip:       tip,
			HalfWidth: half,
			Steps:     7,
			Profile:   "petal",
			Notch:     0.12,
			Bend:      (rng.Float64()*2 - 1) * length * 0.10,
		})
		// centred on its own middle, like every piece the burst throws
		var cx, cy float64
		for _, p := range poly {
			cx += p.X
			cy += p.Y
		}
		n := float64(len(poly))
		cx, cy = cx/n, cy/n
		local := make([]Point, len(poly))
		for j, p := range poly {
			local[j] = Point{X: p.X - cx, Y: p.Y - cy}
		}
		// scattered through the middle of the bloom, so they come out of the
		// flower rather than off a flat plane in front of it
		r := rng.Float64() * length * 0.7
		a := rng.Float64() * 2 * math.Pi
		pieces = append(pieces, PetalPiece{
			Poly:  local,
			Fill:  FillLinear(Point{-math.Cos(a) * length, -math.Sin(a) * length}, tip, []Stop{s3(0, Tint(tone, 0.82), 1), s3(1, Tint(tone, 1.08), 1)}),
			Alpha: 1,
			At:    Point{X: centre.X + math.Cos(a)*r, Y: centre.Y + math.Sin(a)*r},
		})
	}
	return pieces
}

// NewBurst throws a set of petals: power is how hard the flower was dropped
// (1 is a plain drop), and throwX/throwY carry the direction the hand was moving,
// so a flower flung onto a chat explodes the way it was travelling.
func NewBurst(pieces []PetalPiece, rng *rand.Rand, power, throwX, throwY float64) *Burst {
	if power <= 0 {
		power = 1
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(1))
	}
	// the middle of the bloom: the petals all fly away from it
	var cx, cy float64
	for _, p := range pieces {
		cx += p.At.X
		cy += p.At.Y
	}
	if len(pieces) > 0 {
		cx, cy = cx/float64(len(pieces)), cy/float64(len(pieces))
	}
	burst := &Burst{Gravity: burstGravity * (0.9 + 0.25*rng.Float64()), Air: burstAir, Centre: Point{cx, cy}}
	// the flash wears the flower's colour: every piece is cut from the palette, so
	// the first one knows what the flower looks like
	for _, piece := range pieces {
		if len(piece.Fill.Stops) > 0 {
			burst.Tint = Tint(piece.Fill.Stops[len(piece.Fill.Stops)-1].Color, 1.18)
			break
		}
	}
	for _, piece := range pieces {
		dx, dy := piece.At.X-cx, piece.At.Y-cy
		dist := math.Hypot(dx, dy)
		if dist < 1e-3 {
			dx, dy, dist = rng.Float64()-0.5, 1, 1
		}
		// away from the middle, with enough spread that no two petals take the
		// same line, and the hand's own motion thrown in
		jitter := 0.75 + 0.55*rng.Float64()
		speed := (burstSpeedMin + (burstSpeedMax-burstSpeedMin)*rng.Float64()) * power * jitter
		piece.VX = dx/dist*speed + throwX*burstMomentum + (rng.Float64()-0.5)*100
		piece.VY = dy/dist*speed + throwY*burstMomentum + burstLift*(0.5+rng.Float64())
		piece.Spin = (rng.Float64()*2 - 1) * burstSpinMax * (0.6 + 0.7*rng.Float64())
		piece.Angle = (rng.Float64()*2 - 1) * 0.5
		piece.Scale = 0.88 + 0.3*rng.Float64()
		piece.Hold = burstHoldMin + (burstHoldMax-burstHoldMin)*rng.Float64()
		piece.Fade = burstFade * (0.8 + 0.5*rng.Float64())
		burst.Pieces = append(burst.Pieces, piece)
	}
	return burst
}

// Step moves the burst on. dt is clamped like every other step in the engine, so
// a hitch in the frame loop cannot fling a petal across the county.
func (b *Burst) Step(dt float64) {
	dt = clamp(dt, 0, 0.05)
	if dt <= 0 {
		return
	}
	b.Elapsed += dt
	air := math.Max(0, 1-b.Air*dt)
	for i := range b.Pieces {
		p := &b.Pieces[i]
		if p.Dead {
			continue
		}
		p.VY -= b.Gravity * dt
		p.VX *= air
		p.VY *= air
		p.At.X += p.VX * dt
		p.At.Y += p.VY * dt
		p.Angle += p.Spin * dt
		p.Life += dt
		if p.Life > p.Hold+p.Fade {
			p.Dead = true
		}
	}
}

// reach is how far a piece's own polygon reaches from the middle it is drawn
// around, at the scale it is being drawn at.
func (p PetalPiece) reach() float64 {
	d := 0.0
	for _, q := range p.Poly {
		d = math.Max(d, math.Hypot(q.X, q.Y))
	}
	return d * p.Scale
}

// Bounds is the box the burst draws in *right now*: every piece still in the air,
// its own polygon included, and the flash while the flash lasts.
//
// This is the box a burst has to be given, and it is worth being precise about
// why. The petals are thrown from the middle of the bloom and then fall, so the
// picture they make is nothing like a box around the flower — it is a cloud that
// spreads out and sinks, and a fixed square centred on the bloom is either much
// bigger than it needs to be or (worse, and this is what happened) smaller: a
// petal that outran the square was sliced off square by the edge of its own
// window, at the bottom and at the sides, exactly where the eye is following it.
func (b *Burst) Bounds() Bounds {
	box := Bounds{
		MinX: b.Centre.X, MaxX: b.Centre.X,
		MinY: b.Centre.Y, MaxY: b.Centre.Y,
	}
	for i := range b.Pieces {
		p := &b.Pieces[i]
		if p.Dead {
			continue
		}
		r := p.reach()
		box.MinX = math.Min(box.MinX, p.At.X-r)
		box.MaxX = math.Max(box.MaxX, p.At.X+r)
		box.MinY = math.Min(box.MinY, p.At.Y-r)
		box.MaxY = math.Max(box.MaxY, p.At.Y+r)
	}
	if _, ok := b.flash(); ok {
		box.MinX = math.Min(box.MinX, b.Centre.X-flashTo)
		box.MaxX = math.Max(box.MaxX, b.Centre.X+flashTo)
		box.MinY = math.Min(box.MinY, b.Centre.Y-flashTo)
		box.MaxY = math.Max(box.MaxY, b.Centre.Y+flashTo)
	}
	return box
}

// Flight is the box the whole burst will ever draw in, worked out by flying it
// once, and the guarantee that it fits the budget: if the burst would reach
// further than `budget` from the middle of the bloom, every piece's speed is
// scaled down — all of them by the same factor, so an explosion still reads as
// one gesture — until it fits.
//
// Both halves matter. The box is what the surface is sized to, so nothing can be
// cut off by the edge of a window nobody measured; and the budget is what keeps a
// flower flung across the desk from asking for a surface the size of the desk.
// The flight is deterministic and cheap to fly (eighty-four pieces, about a
// second each), so this is a real answer rather than a closed form that has to be
// re-derived every time the physics is touched.
func (b *Burst) Flight(budget float64) Bounds {
	if budget <= 0 {
		budget = flashTo * 2
	}
	box := b.sample(1)
	if reach := b.reachOf(box); reach > budget {
		// the largest scale that still fits: bisection rather than algebra, because
		// gravity does not scale with the throw and a formula would be a second
		// copy of the physics to keep in step
		lo, hi := 0.0, 1.0
		for i := 0; i < 24; i++ {
			mid := (lo + hi) / 2
			if b.reachOf(b.sample(mid)) <= budget {
				lo = mid
			} else {
				hi = mid
			}
		}
		box = b.sample(lo)
		for i := range b.Pieces {
			b.Pieces[i].VX *= lo
			b.Pieces[i].VY *= lo
		}
	}
	return box
}

// reachOf is how far a box gets from the middle of the bloom, which is the
// distance the budget is expressed in: a piece at the corner of the box.
func (b *Burst) reachOf(box Bounds) float64 {
	dx := math.Max(math.Abs(box.MinX-b.Centre.X), math.Abs(box.MaxX-b.Centre.X))
	dy := math.Max(math.Abs(box.MinY-b.Centre.Y), math.Abs(box.MaxY-b.Centre.Y))
	return math.Hypot(dx, dy)
}

// sample flies a copy of the burst to its end with every speed multiplied by k,
// and returns the box that covers all of it. It uses the same Step the app does,
// so the box cannot drift away from the physics it is measuring.
func (b *Burst) sample(k float64) Bounds {
	pieces := make([]PetalPiece, len(b.Pieces))
	copy(pieces, b.Pieces)
	for i := range pieces {
		pieces[i].VX *= k
		pieces[i].VY *= k
		pieces[i].Life, pieces[i].Dead = 0, false
	}
	probe := &Burst{Pieces: pieces, Gravity: b.Gravity, Air: b.Air, Centre: b.Centre}
	box := probe.Bounds()
	// a burst is over in about a second and a half; the cap is only there so that a
	// future tuning mistake shows up as a test failure rather than a hung one
	for i := 0; i < 600 && !probe.Done(); i++ {
		probe.Step(1.0 / 60)
		box = unionBounds(box, probe.Bounds())
	}
	return box
}

func unionBounds(a, b Bounds) Bounds {
	return Bounds{
		MinX: math.Min(a.MinX, b.MinX),
		MaxX: math.Max(a.MaxX, b.MaxX),
		MinY: math.Min(a.MinY, b.MinY),
		MaxY: math.Max(a.MaxY, b.MaxY),
	}
}

// Done reports whether every piece has gone, and there is nothing left to draw.
func (b *Burst) Done() bool {
	for i := range b.Pieces {
		if !b.Pieces[i].Dead {
			return false
		}
	}
	return true
}

// Left is how many pieces are still in the air.
func (b *Burst) Left() int {
	n := 0
	for i := range b.Pieces {
		if !b.Pieces[i].Dead {
			n++
		}
	}
	return n
}

// Shapes is the burst as drawing: the flash while it lasts, then each piece
// rotated about its own middle, moved to where it is now, and faded by how long
// it has been flying.
func (b *Burst) Shapes() []Shape {
	out := make([]Shape, 0, len(b.Pieces)+1)
	if flash, ok := b.flash(); ok {
		out = append(out, flash)
	}
	for i := range b.Pieces {
		p := &b.Pieces[i]
		if p.Dead {
			continue
		}
		alpha := p.Alpha
		if p.Life > p.Hold {
			k := clamp01((p.Life - p.Hold) / p.Fade)
			// ease out, so a petal thins away rather than switching off
			alpha *= 1 - k*k
		}
		if alpha <= 0.01 {
			continue
		}
		sin, cos := math.Sincos(p.Angle)
		poly := make([]Point, len(p.Poly))
		for j, q := range p.Poly {
			x, y := q.X*p.Scale, q.Y*p.Scale
			poly[j] = Point{p.At.X + x*cos - y*sin, p.At.Y + x*sin + y*cos}
		}
		out = append(out, Shape{Part: "burst", Poly: poly, Fill: p.Fill, Alpha: alpha})
	}
	return out
}

// flash is the shockwave, while it is still opening.
func (b *Burst) flash() (Shape, bool) {
	if b.Elapsed >= flashLife {
		return Shape{}, false
	}
	k := clamp01(b.Elapsed / flashLife)
	// out fast, settling: the light leaves the bloom before the petals do
	ease := 1 - math.Pow(1-k, 2.7)
	radius := flashFrom + (flashTo-flashFrom)*ease
	alpha := (1 - k) * (1 - k) * 0.72
	if alpha <= 0.01 {
		return Shape{}, false
	}
	// a disc whose middle is clear: read as a ring of light, not a blob of paint
	return Shape{
		Part:  "flash",
		Poly:  Ellipse(b.Centre, radius, radius, 48),
		Alpha: alpha,
		Fill: FillRadial(b.Centre, 0, radius, []Stop{
			s3(0, b.Tint, 0),
			s3(0.62, b.Tint, 1),
			s3(1, b.Tint, 0),
		}),
	}, true
}
