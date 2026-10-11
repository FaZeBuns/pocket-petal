package flower

import (
	"math"
	"testing"
)

const dt = 1.0 / 60

func run(c *Carry, frames int, hand Hand) CarryState {
	var last CarryState
	for i := 0; i < frames; i++ {
		last = c.Step(dt, hand)
	}
	return last
}

func TestUntouchedFlowerHangsAlmostStill(t *testing.T) {
	carry := CreateCarry(PhysicsOptions{})
	run(carry, 240, Hand{})
	state := carry.State()
	if math.Abs(state.Ox) > 0.6 {
		t.Fatalf("it should not drift sideways, ox=%v", state.Ox)
	}
	if state.Oy < 0 || state.Oy > 3 {
		t.Fatalf("gravity should sag it a hair, oy=%v", state.Oy)
	}
	motion := carry.Motion()
	if math.Abs(motion.Swing) > 0.05 || motion.Flutter > 0.05 {
		t.Fatalf("a still flower is not swinging or fluttering: %+v", motion)
	}
}

func TestCarriedFlowerTrailsAndHangsBack(t *testing.T) {
	carry := CreateCarry(PhysicsOptions{})
	state := run(carry, 4, Hand{VX: 900, AX: 3000, Dragging: true})
	if state.Ox > -0.5 {
		t.Fatalf("the bloom should lag behind the hand, ox=%v", state.Ox)
	}
	motion := carry.Motion()
	if motion.Lean > -0.1 {
		t.Fatalf("the stalk should bow away from travel, lean=%v", motion.Lean)
	}
	if motion.Swing > -0.02 {
		t.Fatalf("the head should hang back while carried, swing=%v", motion.Swing)
	}
	if motion.Flutter < 0.5 {
		t.Fatalf("moving should catch the air, flutter=%v", motion.Flutter)
	}
}

// The "limp neck": a steady drag leaves the head behind, and letting go leaves it
// swinging, which is the whole difference between carried and welded.
func TestLimpNeckSwingAndFollowThrough(t *testing.T) {
	carry := CreateCarry(PhysicsOptions{})
	run(carry, 30, Hand{VX: 850, AX: 3000, Dragging: true})
	if carry.State().Swing >= -0.02 {
		t.Fatalf("the head should hang back while carried, swing=%v", carry.State().Swing)
	}

	// stopping dead throws the head forward: the neck was left behind, and the
	// bloom carries on for a moment
	before := carry.State().Swing
	run(carry, 6, Hand{AX: -40000, Dragging: true})
	if carry.State().Swing <= before {
		t.Fatalf("stopping should throw the head forward: %v -> %v", before, carry.State().Swing)
	}
	peak := 0.0
	crossings := 0
	previous := carry.State().Swing
	for i := 0; i < 360; i++ {
		state := carry.Step(dt, Hand{Dragging: true})
		if state.Swing > peak {
			peak = state.Swing
		}
		if (previous < 0) != (state.Swing < 0) {
			crossings++
		}
		previous = state.Swing
	}
	if peak < 0.3 {
		t.Fatalf("the follow-through should carry the head past the stem, peak=%v", peak)
	}
	if crossings < 3 {
		t.Fatalf("a limp neck keeps swinging; crossings=%d", crossings)
	}
	if math.Abs(previous) > 0.08 {
		t.Fatalf("it has to come to rest eventually, ended at %v", previous)
	}
}

func TestASuddenStopIsNotAFrameRateExplosion(t *testing.T) {
	carry := CreateCarry(PhysicsOptions{})
	for _, seconds := range []float64{0.5, 2, 10, -1, 0, math.NaN()} {
		carry.Step(seconds, Hand{VX: 4000, AX: 40000, Dragging: true})
	}
	state := carry.State()
	for _, v := range []float64{state.Ox, state.Oy, state.Vx, state.Vy, state.Whip, state.Swing, state.Squash} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("a bad frame made the physics non-finite: %+v", state)
		}
	}
	if math.Hypot(state.Ox, state.Oy) > 31 {
		t.Fatalf("it escaped its tether: %v", math.Hypot(state.Ox, state.Oy))
	}
}

func TestTetherHolds(t *testing.T) {
	carry := CreateCarry(PhysicsOptions{Tweak: func(p *PhysicsParams) { p.MaxOffset = 18 }})
	for i := 0; i < 120; i++ {
		state := carry.Step(dt, Hand{VX: 3000, VY: 3000, AX: 30000, AY: 30000, Dragging: true})
		if math.Hypot(state.Ox, state.Oy) > 18+1e-6 {
			t.Fatalf("escaped the tether: %v,%v", state.Ox, state.Oy)
		}
	}
}

func TestSameMotionTwiceIsTheSameFlower(t *testing.T) {
	a := CreateCarry(PhysicsOptions{})
	b := CreateCarry(PhysicsOptions{})
	script := []Hand{
		{VX: 400, AX: 2000, Dragging: true},
		{VX: 900, AX: 5000, Dragging: true},
		{AX: -8000, Dragging: true},
		{VX: -300, AX: -2000, Dragging: true},
		{Dragging: true},
	}
	for _, hand := range script {
		sa := a.Step(dt, hand)
		sb := b.Step(dt, hand)
		if sa != sb {
			t.Fatalf("two identical runs diverged: %+v vs %+v", sa, sb)
		}
	}
}

func TestLandingSquashesAndBounces(t *testing.T) {
	carry := CreateCarry(PhysicsOptions{})
	hit := carry.Impact(1)
	if hit.Squash >= -0.2 {
		t.Fatalf("it should be pancaked on impact, squash=%v", hit.Squash)
	}
	stretched, bounced := false, false
	for i := 0; i < 120; i++ {
		state := carry.Step(dt, Hand{Dragging: true})
		if state.Squash > 0.02 {
			stretched = true
		}
		if state.Swing > 0.03 {
			bounced = true
		}
	}
	if !stretched {
		t.Fatal("a landing should bounce, not thud")
	}
	if !bounced {
		t.Fatal("a landing should get the head moving too")
	}
	if math.Abs(carry.State().Squash) > 0.02 {
		t.Fatalf("and it has to settle, squash=%v", carry.State().Squash)
	}
}

func TestFlickSendsItOnAndBendsTheStalkBack(t *testing.T) {
	carry := CreateCarry(PhysicsOptions{})
	carry.Flick(600, -200, 1)
	state := carry.State()
	if state.Vx <= 0 || state.Vy >= 0 {
		t.Fatalf("a flick towards 600,-200 gave %v,%v", state.Vx, state.Vy)
	}
	if state.Whip >= 0 {
		t.Fatalf("the stalk should bend the other way straight away, whip=%v", state.Whip)
	}
	run(carry, 3, Hand{Dragging: true})
	if carry.State().Swing >= -0.05 {
		t.Fatalf("the head should trail the fling, swing=%v", carry.State().Swing)
	}
}

func TestResetIsRest(t *testing.T) {
	carry := CreateCarry(PhysicsOptions{})
	run(carry, 10, Hand{VX: 1200, AX: 9000, Dragging: true})
	carry.Impact(1)
	carry.Reset()
	state := carry.State()
	if state.Ox != 0 || state.Oy != 0 || state.Vx != 0 || state.Vy != 0 || state.Whip != 0 || state.Swing != 0 || state.Squash != 0 {
		t.Fatalf("reset left something behind: %+v", state)
	}
}

func TestTrackerMeasuresTheHand(t *testing.T) {
	tracker := NewTracker(1)
	tracker.Sample(0, 0, 0)
	var state TrackerState
	for i := 1; i <= 40; i++ {
		state = tracker.Sample(float64(i*8), float64(-i*2), float64(i)*0.016)
	}
	if state.VX < 400 {
		t.Fatalf("moving right should read as speed, vx=%v", state.VX)
	}
	if state.VY > -100 {
		t.Fatalf("moving up should read negative, vy=%v", state.VY)
	}
	tracker.Reset()
	if got := tracker.State(); got != (TrackerState{}) {
		t.Fatalf("reset left a measurement behind: %+v", got)
	}
	// a repeated position is a stopped hand, not a division by zero
	tracker.Sample(5, 5, 100)
	still := tracker.Sample(5, 5, 116)
	if math.IsNaN(still.VX) || math.Abs(still.VX) > 1e-9 {
		t.Fatalf("a stopped hand should read as still, got %v", still.VX)
	}
}
