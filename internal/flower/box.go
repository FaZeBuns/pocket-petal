package flower

import "math"

// How much of design space a window has to show.
//
// A flower is not a picture: the rig leans it, the neck swings the bloom up and
// to the side, and the head whips past where it was. A box cut to the flower
// standing still is what slices the tops off the petals during a fast drag —
// the pose travels, the frame does not.
//
// So the box is the union of the flower's outline over the poses a carry can
// actually reach: hard leans both ways, the head's pendulum at both ends, the
// whip, the flutter of a fast drag, the landing squash, and the weight of being
// held. Nothing outside it can happen, so nothing outside it is ever clipped.

// FullBloom is the rig time at which the flower has finished opening, which is
// the pose everything on screen is judged at.
const FullBloom = 6.0

// envelopeMotions is the set the box has to cover, and it is *measured* rather
// than guessed: the carry model is actually driven through the hardest things a
// hand can do to a flower and the poses it passes through are collected. Listing
// the corners of the state space by hand would mean covering combinations the
// physics can never reach (a full lean and a full swing at once, say), and the
// box would swell until the flower was small enough to look lost.
func envelopeMotions() []Motion {
	const dt = 1.0 / 60
	var out []Motion
	add := func(c *Carry) { out = append(out, c.Motion()) }

	// The scripts show what a hand does, but the rig takes its motion as a
	// vocabulary and clamps each word independently: a whip and a droop can
	// arrive together in a way no single drag produces, and a box built only
	// from whole drags would be a few pixels short of it. So the corners are
	// covered as well, over the fields that move the outline.
	for _, lean := range []float64{-1.2, 0, 1.2} {
		for _, swing := range []float64{-1.15, 0, 1.15} {
			for _, whip := range []float64{-1.2, 1.2} {
				for _, droop := range []float64{0, 1} {
					for _, flutter := range []float64{0, 2.2} {
						out = append(out, Motion{Lean: lean, Swing: swing, Whip: whip, Droop: droop, Flutter: flutter})
					}
				}
			}
		}
	}
	for _, extra := range []Motion{
		{Stretch: -1, Flutter: 2.2, Droop: 1, Lean: -1.2},
		{Stretch: 1, Gust: 1.5, Droop: 1, Lean: 1.2},
		{LagX: 1.2, LagY: 1.2, Flutter: 2.2},
		{LagX: -1.2, LagY: -1.2, Flutter: 2.2, Droop: 1},
	} {
		out = append(out, extra)
	}

	for _, script := range carryScripts {
		carry := CreateCarry(PhysicsOptions{})
		for _, leg := range script {
			steps := int(leg.seconds / dt)
			if steps < 1 {
				steps = 1
			}
			for i := 0; i < steps; i++ {
				carry.Step(dt, leg.hand)
				if i%3 == 0 {
					add(carry)
				}
			}
			if leg.flick != 0 || leg.impact != 0 {
				if leg.impact != 0 {
					carry.Impact(leg.impact)
				}
				if leg.flick != 0 {
					carry.Flick(leg.flick, leg.flick*0.4, 1.6)
				}
				add(carry)
			}
		}
	}
	return out
}

// oneLeg is one thing a hand does to the flower for a while.
type oneLeg struct {
	seconds float64
	hand    Hand
	flick   float64 // a fling, in px/s, applied at the end of the leg
	impact  float64 // a landing, applied at the end of the leg
}

// carryScripts are the ways the flower gets carried: dragged hard in every
// direction "very fast" can mean, stopped dead, reversed, flung, dropped and
// left hanging in the wind. The app's window has to hold the flower through all
// of it.
var carryScripts = [][]oneLeg{
	{
		{seconds: 0.7, hand: Hand{VX: 2600, Dragging: true}},
		{seconds: 0.4, hand: Hand{VX: 0, AX: -22000, Dragging: true}},
		{seconds: 0.6, hand: Hand{}, impact: -1.5},
	},
	{
		{seconds: 0.7, hand: Hand{VX: -2600, Dragging: true}},
		{seconds: 0.4, hand: Hand{VX: 0, AX: 22000, Dragging: true}},
		{seconds: 0.6, hand: Hand{}, impact: 1.5},
	},
	{
		{seconds: 0.6, hand: Hand{VY: -2400, Dragging: true}},
		{seconds: 0.4, hand: Hand{VY: 0, AY: 18000, Dragging: true}},
		{seconds: 0.5, hand: Hand{}, flick: 3000},
	},
	{
		{seconds: 0.6, hand: Hand{VY: 2400, Dragging: true}},
		{seconds: 0.4, hand: Hand{VY: 0, AY: -18000, Dragging: true}},
		{seconds: 0.5, hand: Hand{}, flick: -3000},
	},
	{
		// a hard zig-zag: the direction changes faster than the flower can answer
		{seconds: 0.22, hand: Hand{VX: 2400, AX: 9000, Dragging: true}},
		{seconds: 0.22, hand: Hand{VX: -2400, AX: -18000, Dragging: true}},
		{seconds: 0.22, hand: Hand{VX: 2400, AX: 18000, Dragging: true}},
		{seconds: 0.22, hand: Hand{VX: -2400, AX: -18000, Dragging: true}},
		{seconds: 0.5, hand: Hand{VX: 0, AX: 16000, Dragging: true}},
		{seconds: 0.5, hand: Hand{}, impact: -1.2},
	},
	{
		// a fast diagonal, flicked sideways at the end
		{seconds: 0.6, hand: Hand{VX: 1900, VY: -1700, Dragging: true}},
		{seconds: 0.3, hand: Hand{VX: 0, VY: 0, AX: -16000, AY: 16000, Dragging: true}},
		{seconds: 0.4, hand: Hand{}, flick: -2600, impact: 1.0},
	},
	{
		// hanging still in a gust, and a slow drift
		{seconds: 1.2, hand: Hand{Gust: 1.5}},
		{seconds: 1.0, hand: Hand{VX: 420, Dragging: true, Gust: 1.5}},
		{seconds: 1.0, hand: Hand{Gust: 1.5}},
	},
}

// The flower is given a little room of its own inside the box — enough to absorb
// a pose the scripts never happened to pass through — so a petal tip stops just
// short of the window's edge instead of touching it.
const (
	boxPadX = 0.05
	boxPadY = 0.05
)

// outlineBox is the flower's own outline for one pose, with the halo and the
// contact shadow left out: they are not part of the flower.
func outlineBox(rig *Rig, motion Motion) Bounds {
	pose := PoseOf(rig, FullBloom, motion)
	list := BuildDrawList(rig, pose, DrawOptions{Detail: "full", Shadow: false, Palette: rig.Palette})
	framed := make([]Shape, 0, len(list))
	for _, shape := range list {
		if shape.Part == "glow" {
			continue
		}
		framed = append(framed, shape)
	}
	if len(framed) == 0 {
		return Bounds{MinX: -1, MinY: -1, MaxX: 1, MaxY: 1}
	}
	return ListBounds(framed)
}

// OutlineBox is the flower's outline at rest, which is the flower as a person
// sees it standing still.
func OutlineBox(rig *Rig, motion Motion) Bounds { return outlineBox(rig, motion) }

// MotionBox is every pose the carry can reach, unioned: the whole of design
// space the flower might need a window for. Fit scales the box about its centre,
// so fit 1 is a window whose edges are the envelope's edges.
func MotionBox(rig *Rig, fit float64) Bounds {
	if fit <= 0.1 || fit > 2 {
		fit = 1
	}
	box := Bounds{}
	first := true
	for _, motion := range envelopeMotions() {
		b := outlineBox(rig, motion)
		if first {
			box, first = b, false
			continue
		}
		box = Bounds{
			MinX: min(box.MinX, b.MinX),
			MinY: min(box.MinY, b.MinY),
			MaxX: max(box.MaxX, b.MaxX),
			MaxY: max(box.MaxY, b.MaxY),
		}
	}
	if first {
		return Bounds{MinX: -1, MinY: -1, MaxX: 1, MaxY: 1}
	}
	return ScaleBox(padBox(box), 1/fit)
}

// padBox gives the outline a little air inside the window, so a petal tip stops
// just short of the edge rather than touching it.
func padBox(b Bounds) Bounds {
	w, h := b.Width(), b.Height()
	return Bounds{MinX: b.MinX - w*boxPadX, MaxX: b.MaxX + w*boxPadX, MinY: b.MinY - h*boxPadY, MaxY: b.MaxY + h*boxPadY}
}

// Framing is where the flower sits on screen: the slice of design space the
// window shows, the window's size in physical pixels, and how tall the flower
// stands in it.
type Framing struct {
	Bounds        Bounds
	Width, Height int
	// Flower is the height of the flower's own outline on screen, in pixels.
	Flower float64
	// RestX and RestY are where the *resting* flower's outline sits inside the
	// window, in pixels. The envelope is not centred on the resting flower (a
	// lean goes further one way than the other), so a caller that wants to place
	// the flower itself — rather than the envelope's middle — needs this.
	RestX, RestY float64
}

// framePad is the air left around the envelope, in pixels.
const framePad = 4.0

// Envelope is the box a flower's window has to hold: every pose its carry can
// reach, measured, with the flower's own air added. Measuring it drives the carry
// model through the hardest things a hand can do and draws each pose, so it costs
// about a third of a second — which is why it is a separate function: the exact
// framing calls it once, and a caller previewing a change of shape scales an
// envelope it has already measured instead (see FrameForEnvelope).
func Envelope(rig *Rig) Bounds { return padBox(MotionBox(rig, 1)) }

// FrameFor sizes a window around the flower.
//
// wantW and wantH are the box the flower is meant to fit into on screen — how
// big the flower looks is the one thing worth configuring — and fit is how much
// of that box it fills. The window is then *derived*: big enough to hold every
// pose the carry can reach at exactly that size, so nothing is ever clipped and
// nothing is ever shrunk to compensate. A window that is a rectangle of air
// costs nothing, so there is no reason to make the flower pay for the leash.
func FrameFor(rig *Rig, wantW, wantH int, fit float64) Framing {
	return FrameForEnvelope(rig, Envelope(rig), wantW, wantH, fit)
}

// FrameForEnvelope is FrameFor with the envelope handed in rather than measured.
//
// The framing maths is identical; only where the box comes from is different. It
// exists for the settings panel, which previews a flower that is being changed
// while the pointer moves: measuring a fresh envelope for every mouse move would
// be a third of a second of delay per frame, and scaling the one already measured
// for the flower as it ships is both instant and the same shape.
func FrameForEnvelope(rig *Rig, env Bounds, wantW, wantH int, fit float64) Framing {
	if fit <= 0.1 || fit > 2 {
		fit = 1
	}
	rest := outlineBox(rig, Motion{})
	if rest.Width() <= 0 || rest.Height() <= 0 {
		return Framing{Bounds: env, Width: wantW, Height: wantH, Flower: float64(wantH) * fit, RestX: float64(wantW) / 2, RestY: float64(wantH) / 2}
	}
	scale := math.Min(float64(wantW)/rest.Width(), float64(wantH)/rest.Height()) * fit
	innerW := env.Width() * scale
	innerH := env.Height() * scale
	w := int(math.Ceil(innerW + 2*framePad))
	h := int(math.Ceil(innerH + 2*framePad))
	pad := framePad / scale
	box := Bounds{
		MinX: (env.MinX+env.MaxX)/2 - env.Width()/2 - pad,
		MaxX: (env.MinX+env.MaxX)/2 + env.Width()/2 + pad,
		MinY: (env.MinY+env.MaxY)/2 - env.Height()/2 - pad,
		MaxY: (env.MinY+env.MaxY)/2 + env.Height()/2 + pad,
	}
	restX, restY := MakeTransform(box, w, h, 0).ToPixel((rest.MinX+rest.MaxX)/2, (rest.MinY+rest.MaxY)/2)
	return Framing{Bounds: box, Width: w, Height: h, Flower: rest.Height() * scale, RestX: restX, RestY: restY}
}

// ScaleBox grows a box about its own centre, which is how "fit" is applied.
func ScaleBox(b Bounds, factor float64) Bounds {
	cx := (b.MinX + b.MaxX) / 2
	cy := (b.MinY + b.MaxY) / 2
	halfW := (b.MaxX - b.MinX) / 2 * factor
	halfH := (b.MaxY - b.MinY) / 2 * factor
	return Bounds{MinX: cx - halfW, MinY: cy - halfH, MaxX: cx + halfW, MaxY: cy + halfH}
}
