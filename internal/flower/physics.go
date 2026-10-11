package flower

import "math"

// What makes the flower feel heavy.
//
// A drawing follows the cursor exactly. A *thing* does not: it lags behind,
// swings when you stop, keeps moving when you do, and squashes when it lands.
// This is that model, and nothing else — one mass on a damped spring, plus a
// whipping stalk, a limp-neck pendulum for the head, and an impact squash.
//
// It is deliberately frame-driven instead of reading the world itself:
// Step(dt, Hand{...}) takes the *carrying frame's* motion (the cursor, or the
// little window you are dragging) and answers with how the flower is hanging
// right now. No clocks, no globals, no windowing — so the whole feel of the
// thing is unit tested.
//
//	carried := flower.CreateCarry(flower.PhysicsOptions{})
//	carried.Step(dt, flower.Hand{VX: vx, VY: vy, AX: ax, Dragging: true})
//	motion := carried.Motion()   // -> straight into flower.PoseOf
//	offset := carried.Offset()   // -> where the bloom sits, in px

// PhysicsParams is every dial of the carry model.
type PhysicsParams struct {
	// the mass on its tether
	Stiffness    float64 // pull back to rest, per second squared
	Damping      float64 // how quickly a swing dies out (underdamped on purpose)
	Inertia      float64 // how much of the frame's acceleration the bloom feels
	Gravity      float64 // constant downward pull, px/s^2
	MaxOffset    float64 // the flower is tethered, not loose: clamp the lag, px
	GravityScale float64 // 0 = weightless, 1 = a real flower

	// the stalk, which lags the bloom
	WhipStiffness float64
	WhipDamping   float64
	WhipScale     float64 // apparent speed -> how far the stalk bends
	WhipAccel     float64 // and how much of it is acceleration, not speed

	// The head on its limp neck: a slow, barely damped pendulum. This is the
	// difference between a flower that is being carried and a flower you are
	// holding — it keeps swinging after you stop, and since the rig's bend is
	// concentrated in the neck, the top of the stalk goes with it and the petals
	// answer every movement.
	SwingStiffness    float64 // the neck's spring: a low frequency is a limp neck
	SwingDamping      float64 // almost none, so it swings a few times before it settles
	SwingFromVelocity float64 // a moving hand drags the head back
	SwingFromAccel    float64 // a change of direction whips it
	SwingMax          float64

	// landing
	SquashStiffness float64
	SquashDamping   float64

	// mapping the state onto the rig's motion vocabulary
	LeanScale     float64
	LeanFromLag   float64
	LagScale      float64
	LagFromOffset float64
	LagMix        float64 // how much of the petal lag is apparent speed vs position
	FlutterSpeed  float64 // px/s that counts as "moving fast"
	StretchScale  float64
}

// DefaultPhysicsParams is tuned against real drag speeds: a hand moves the
// flower at roughly 300-1500 px/s with 2000-8000 px/s^2 of acceleration, and
// these numbers are what make that read as weight rather than as jitter.
func DefaultPhysicsParams() PhysicsParams {
	return PhysicsParams{
		Stiffness:    120,
		Damping:      8.2,
		Inertia:      0.55,
		Gravity:      96,
		MaxOffset:    26,
		GravityScale: 1,

		WhipStiffness: 78,
		WhipDamping:   10,
		WhipScale:     0.0008,
		WhipAccel:     0.00012,

		SwingStiffness:    24,
		SwingDamping:      1.85,
		SwingFromVelocity: 0.013,
		SwingFromAccel:    0.0013,
		SwingMax:          1.1,

		SquashStiffness: 190,
		SquashDamping:   12.5,

		LeanScale:     0.00075,
		LeanFromLag:   0.016,
		LagScale:      0.0008,
		LagFromOffset: 0.024,
		LagMix:        0.6,
		FlutterSpeed:  900,
		StretchScale:  0.00016,
	}
}

// SwingLimit is how far the head can hang off the stalk before the neck refuses.
const SwingLimit = 1.1

// Hand is what is carrying the flower this frame.
type Hand struct {
	VX, VY   float64
	AX, AY   float64
	Dragging bool
	Gust     float64
}

// CarryState is the whole state of the carried flower.
type CarryState struct {
	Ox, Oy   float64
	Vx, Vy   float64
	Fvx, Fvy float64
	Whip     float64
	Swing    float64
	Squash   float64
	Gust     float64
	Speed    float64
	Travel   float64
}

// Carry is the model itself.
type Carry struct {
	P PhysicsParams

	ox, oy          float64
	vx, vy          float64
	whip, whipV     float64
	fvx, fvy        float64
	fax, fay        float64
	swing, swingV   float64
	squash, squashV float64
	gusts           float64
}

// PhysicsOptions overrides the defaults when the model is built.
type PhysicsOptions struct {
	// Tweak gets the params, for the rare caller that needs a dial this struct
	// does not name.
	Tweak func(*PhysicsParams)
}

// CreateCarry builds the model with the default feel.
func CreateCarry(opts PhysicsOptions) *Carry {
	p := DefaultPhysicsParams()
	if opts.Tweak != nil {
		opts.Tweak(&p)
	}
	return &Carry{P: p}
}

// Reset puts the flower back at rest under the hand.
func (c *Carry) Reset() {
	c.ox, c.oy = 0, 0
	c.vx, c.vy = 0, 0
	c.whip, c.whipV = 0, 0
	c.fvx, c.fvy = 0, 0
	c.fax, c.fay = 0, 0
	c.swing, c.swingV = 0, 0
	c.squash, c.squashV = 0, 0
	c.gusts = 0
}

// Integrate the mass. Semi-implicit Euler, in small fixed substeps, so a laggy
// frame (a 40 ms hitch) cannot make the flower explode.
func (c *Carry) integrate(dt, ax, ay float64, dragging bool) {
	steps := 1
	if dt > 0.02 {
		steps = 2
	}
	h := dt / float64(steps)
	for i := 0; i < steps; i++ {
		// the frame yanks the flower the other way, and gravity always pulls:
		// everything below is an acceleration (force per unit mass)
		gravity := 0.28
		if dragging {
			gravity = 1
		}
		fx := -num(ax)*c.P.Inertia - c.ox*c.P.Stiffness - c.vx*c.P.Damping
		fy := -num(ay)*c.P.Inertia - c.oy*c.P.Stiffness - c.vy*c.P.Damping + c.P.Gravity*c.P.GravityScale*gravity
		c.vx += fx * h
		c.vy += fy * h
		c.ox += c.vx * h
		c.oy += c.vy * h
		d := math.Hypot(c.ox, c.oy)
		if d > c.P.MaxOffset {
			k := c.P.MaxOffset / d
			c.ox *= k
			c.oy *= k
			// bleeding the velocity here is what stops the tether from buzzing
			c.vx *= 0.7
			c.vy *= 0.7
		}
	}
}

// Step hands the frame's motion in and gets the hanging-flower state back.
func (c *Carry) Step(dtIn float64, input Hand) CarryState {
	dt := clamp(num(dtIn), 0, 0.05)
	if dt <= 0 {
		return c.State()
	}
	c.fvx = num(input.VX)
	c.fvy = num(input.VY)
	c.fax = num(input.AX)
	c.fay = num(input.AY)

	c.integrate(dt, input.AX, input.AY, input.Dragging)

	// The stalk bends towards where the flower *was*, not where it is: a spring
	// on the frame's velocity, deliberately slower than the mass.
	whipTarget := clamp(-(c.fvx+c.vx*0.5)*c.P.WhipScale-c.fax*c.P.WhipAccel, -1, 1)
	whipForce := (whipTarget-c.whip)*c.P.WhipStiffness - c.whipV*c.P.WhipDamping
	c.whipV += whipForce * dt
	c.whip += c.whipV * dt

	// The head's pendulum. The hand's speed and direction changes drive it, and
	// the bloom's own motion feeds back into that drive — which is what turns a
	// steady drag into a swing that carries on after you have stopped.
	swingTorque := -c.swing*c.P.SwingStiffness -
		c.swingV*c.P.SwingDamping -
		(c.fvx+c.vx*0.6)*c.P.SwingFromVelocity -
		c.fax*c.P.SwingFromAccel
	c.swingV += swingTorque * dt
	c.swing += c.swingV * dt
	if c.swing > c.P.SwingMax {
		c.swing = c.P.SwingMax
		c.swingV *= 0.4
	} else if c.swing < -c.P.SwingMax {
		c.swing = -c.P.SwingMax
		c.swingV *= 0.4
	}

	// and the squash unwinds on its own
	squashForce := -c.squash*c.P.SquashStiffness - c.squashV*c.P.SquashDamping
	c.squashV += squashForce * dt
	c.squash += c.squashV * dt
	if math.Abs(c.squash) < 1e-4 && math.Abs(c.squashV) < 1e-3 {
		c.squash = 0
		c.squashV = 0
	}

	c.gusts += (clamp(num(input.Gust), 0, 1.5) - c.gusts) * math.Min(1, dt*3.2)
	return c.State()
}

// Impact is a landing (or a poke): squash it, and let the spring throw it back
// out.
func (c *Carry) Impact(power float64) CarryState {
	p0 := clamp(num(power), -1.5, 1.5)
	c.squashV -= p0 * 2.6
	c.squash = math.Min(c.squash, -p0*0.28)
	// and the head keeps going after the flower has stopped, like a head does
	c.swingV += p0 * 1.9
	return c.State()
}

// Flick flings it: the flower keeps going, and the stalk bends back the other
// way straight away (the whip has to be visible on the very first frame, not
// three frames later).
func (c *Carry) Flick(fx, fy, scale float64) CarryState {
	c.vx += num(fx) * 0.09 * scale
	c.vy += num(fy) * 0.09 * scale
	c.whip -= clamp(num(fx)*0.0012*scale, -0.55, 0.55)
	c.whipV -= clamp(num(fx)*0.0016*scale, -6, 6)
	// the head is flicked the other way on the neck
	c.swingV -= clamp(num(fx)*0.004*scale, -2.4, 2.4)
	return c.State()
}

// State is everything the model knows, at once.
func (c *Carry) State() CarryState {
	return CarryState{
		Ox:     c.ox,
		Oy:     c.oy,
		Vx:     c.vx,
		Vy:     c.vy,
		Fvx:    c.fvx,
		Fvy:    c.fvy,
		Whip:   c.whip,
		Swing:  c.swing,
		Squash: c.squash,
		Gust:   c.gusts,
		Speed:  math.Hypot(c.vx, c.vy),
		Travel: math.Hypot(c.fvx+c.vx*0.55, c.fvy+c.vy*0.55),
	}
}

// Offset is where the bloom actually is, in pixels, relative to what carries it.
func (c *Carry) Offset() Point { return Point{c.ox, c.oy} }

// Motion is the rig's vocabulary: one place where the maths becomes a look, so
// the feel can be tuned without touching the drawing.
//
// "Where is the flower going" is the hand's motion plus the flower's own
// momentum: carrying it fast and having flung it both bend the stalk back.
func (c *Carry) Motion() Motion {
	tx := c.fvx + c.vx*0.55
	ty := c.fvy + c.vy*0.55
	speed := math.Hypot(tx, ty)
	lag := -tx*c.P.LagScale*c.P.LagMix - c.ox*c.P.LagFromOffset*(1-c.P.LagMix)
	lagY := -ty*c.P.LagScale*c.P.LagMix - c.oy*c.P.LagFromOffset*(1-c.P.LagMix)
	weight := 0.05
	if speed > 40 {
		weight = 0.2
	}
	return Motion{
		Lean: clamp(-tx*c.P.LeanScale-c.ox*c.P.LeanFromLag, -1.2, 1.2),
		// the head's pendulum, straight through to the rig's neck
		Swing: clamp(c.swing, -1.15, 1.15),
		Whip:  clamp(c.whip, -1.2, 1.2),
		// carried by the head, the bloom's weight curls the stem over a little;
		// hanging still, it only nods
		Droop:   clamp(c.oy*0.0032+weight+math.Abs(c.fay)*0.00002, 0, 1),
		LagX:    clamp(lag, -1.2, 1.2),
		LagY:    clamp(lagY, -1.2, 1.2),
		Flutter: clamp(speed/c.P.FlutterSpeed+c.gusts*0.5, 0, 2.2),
		Stretch: clamp(-c.fay*c.P.StretchScale+c.squash, -1, 1),
		Gust:    c.gusts,
	}
}

// TrackerState is the carrying frame's motion, as measured from positions.
type TrackerState struct {
	VX, VY float64
	AX, AY float64
}

// Tracker is a tiny read of the carrying frame's motion, for callers that only
// have positions (every caller has only positions).
type Tracker struct {
	smoothing float64
	lastX     float64
	lastY     float64
	lastT     float64
	started   bool
	state     TrackerState
}

// NewTracker builds a tracker; smoothing 0.35 is the default feel.
func NewTracker(smoothing float64) *Tracker {
	if smoothing <= 0 {
		smoothing = 0.35
	}
	return &Tracker{smoothing: smoothing}
}

// Sample takes the carrier's position at time t (in seconds) and answers with
// its velocity and acceleration, smoothed.
func (t *Tracker) Sample(x, y, seconds float64) TrackerState {
	if !t.started {
		t.started = true
		t.lastX, t.lastY, t.lastT = x, y, seconds
		return TrackerState{}
	}
	dt := math.Max(0.001, math.Min(0.1, seconds-t.lastT))
	nvx := (x - t.lastX) / dt
	nvy := (y - t.lastY) / dt
	// smooth the derivative: raw per-frame deltas are mostly noise
	nax := (nvx - t.state.VX) / dt
	nay := (nvy - t.state.VY) / dt
	t.state.VX += (nvx - t.state.VX) * t.smoothing
	t.state.VY += (nvy - t.state.VY) * t.smoothing
	t.state.AX += (clamp(nax, -20000, 20000) - t.state.AX) * t.smoothing
	t.state.AY += (clamp(nay, -20000, 20000) - t.state.AY) * t.smoothing
	t.lastX, t.lastY, t.lastT = x, y, seconds
	return t.state
}

// Reset forgets the previous position.
func (t *Tracker) Reset() {
	t.started = false
	t.state = TrackerState{}
}

// State is the last measurement.
func (t *Tracker) State() TrackerState { return t.state }
