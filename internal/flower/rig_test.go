package flower

import (
	"math"
	"testing"
)

func dist(a, b Point) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

func TestRigIsDeterministicPerSeed(t *testing.T) {
	seed := 11
	a := CreateRig(RigOptions{Seed: &seed})
	seed2 := 11
	b := CreateRig(RigOptions{Seed: &seed2})
	if len(a.Petals) != len(b.Petals) || len(a.Dots) != len(b.Dots) {
		t.Fatalf("same seed gave different flowers: %d/%d petals", len(a.Petals), len(b.Petals))
	}
	for i := range a.Petals {
		if a.Petals[i].Ring != b.Petals[i].Ring || a.Petals[i].Delay != b.Petals[i].Delay {
			t.Fatalf("petal %d differs between two flowers with the same seed", i)
		}
	}
	other := 12
	c := CreateRig(RigOptions{Seed: &other})
	if c.Petals[0].Ring == a.Petals[0].Ring {
		t.Fatal("a different seed should grow a different flower")
	}
}

func TestUnknownPaletteFallsBack(t *testing.T) {
	rig := CreateRig(RigOptions{Palette: "chartreuse"})
	if rig.Palette != "blush" {
		t.Fatalf("unknown palette should fall back to blush, got %q", rig.Palette)
	}
}

func TestSpineIsAConnectedChain(t *testing.T) {
	rig := CreateRig(RigOptions{})
	for _, motion := range []Motion{{}, {Lean: -0.8, Swing: -0.6}, {Lean: 1.2, Swing: 1.1, Whip: 1}} {
		pose := PoseOf(rig, 3, motion)
		want := pose.StemLength / float64(rig.Params.Steps)
		for i := 1; i < len(pose.Joints); i++ {
			got := dist(pose.Joints[i-1], pose.Joints[i])
			if math.Abs(got-want) > 1e-9 {
				t.Fatalf("joints %d and %d are %.6f apart, expected %.6f: the stem is not a chain", i-1, i, got, want)
			}
		}
	}
}

// The heart of the "limp neck" behaviour: the bloom is not welded to the last
// joint, it hangs a neck's length past it, in the direction the neck points.
func TestHeadHangsOffTheEndOfTheNeck(t *testing.T) {
	rig := CreateRig(RigOptions{})
	for _, motion := range []Motion{{}, {Lean: -0.7, Swing: -0.9}, {Lean: 0.9, Swing: 0.8}, {Droop: 1}} {
		pose := PoseOf(rig, 6, motion)
		tip := pose.Joints[len(pose.Joints)-1]
		offset := dist(tip, Point{pose.Head.X, pose.Head.Y})
		if offset <= 0 {
			t.Fatalf("the head sits exactly on the tip: there is no neck to see")
		}
		if offset > rig.Params.HeadRadius {
			t.Fatalf("the neck is longer than the head is wide: %.3f", offset)
		}
		if math.Abs(offset-pose.Head.NeckLength) > 1e-9 {
			t.Fatalf("the offset (%.6f) is not the neck's length (%.6f)", offset, pose.Head.NeckLength)
		}
		want := Point{
			tip.X + math.Sin(pose.Head.NeckAngle)*offset,
			tip.Y + math.Cos(pose.Head.NeckAngle)*offset,
		}
		if dist(want, Point{pose.Head.X, pose.Head.Y}) > 1e-9 {
			t.Fatalf("the bloom does not hang where the neck points")
		}
	}
}

// worldTip puts a petal's head-local tip where it actually is on screen.
func worldTip(pose Pose, p Petal) Point {
	c := math.Cos(pose.Head.Tilt)
	s := math.Sin(pose.Head.Tilt)
	return Point{
		pose.Head.X + p.Tip.X*c - p.Tip.Y*s,
		pose.Head.Y + p.Tip.X*s + p.Tip.Y*c,
	}
}

// A hanging bloom must move when the top of the stalk moves: if the tip swings,
// the petals go with it. This is the "it is attached" property, asserted in
// world coordinates (petal tips live in the head's own frame).
func TestPetalSMoveWithTheTopOfTheStalk(t *testing.T) {
	rig := CreateRig(RigOptions{})
	still := PoseOf(rig, 6, Motion{})
	carried := PoseOf(rig, 6, Motion{Lean: -0.6, Swing: -0.8})
	moved := 0.0
	for i := range still.Petals {
		moved = math.Max(moved, dist(worldTip(still, still.Petals[i]), worldTip(carried, carried.Petals[i])))
	}
	if moved < 8 {
		t.Fatalf("carrying the flower barely moved the petals: %.2f units", moved)
	}
	// and the bloom travels as one piece: its centre of mass moves too, not just
	// its outermost petal
	centre := func(pose Pose) Point {
		x, y := 0.0, 0.0
		for _, pet := range pose.Petals {
			p := worldTip(pose, pet)
			x += p.X
			y += p.Y
		}
		n := float64(len(pose.Petals))
		return Point{x / n, y / n}
	}
	if travel := dist(centre(still), centre(carried)); travel < 8 {
		t.Fatalf("the whole bloom should travel with the top of the stalk, moved %.2f", travel)
	}
}

// Lean and swing both go through the neck, and the neck is where they bend it,
// but the head's own pendulum also turns the face: that is what reads as a bloom
// swinging on a limp stalk rather than a stalk bending under a fixed flower.
func TestSwingBendsTheNeckAndTurnsTheFace(t *testing.T) {
	rig := CreateRig(RigOptions{})
	base := PoseOf(rig, 6, Motion{})
	leanOnly := PoseOf(rig, 6, Motion{Lean: 0.8})
	swingOnly := PoseOf(rig, 6, Motion{Swing: 0.8})
	leanMove := dist(Point{base.Head.X, base.Head.Y}, Point{leanOnly.Head.X, leanOnly.Head.Y})
	swingMove := dist(Point{base.Head.X, base.Head.Y}, Point{swingOnly.Head.X, swingOnly.Head.Y})
	if swingMove < 8 || leanMove < 8 {
		t.Fatalf("lean %.2f or swing %.2f barely moves the bloom", leanMove, swingMove)
	}
	// same magnitude, same neck: the stalk bends by the same amount either way
	if math.Abs(swingMove-leanMove) > 1e-6 {
		t.Fatalf("lean and swing should bend the neck identically: %.3f vs %.3f", leanMove, swingMove)
	}
	// but only the swing turns the face, and it turns it more the further it goes
	opposite := PoseOf(rig, 6, Motion{Swing: -0.8})
	if math.Abs(swingOnly.Head.Tilt-base.Head.Tilt) <= math.Abs(leanOnly.Head.Tilt-base.Head.Tilt) {
		t.Fatalf("a swing should tip the face more than a lean: %.3f vs %.3f",
			math.Abs(swingOnly.Head.Tilt-base.Head.Tilt), math.Abs(leanOnly.Head.Tilt-base.Head.Tilt))
	}
	if math.Abs(swingOnly.Head.Tilt-opposite.Head.Tilt) < 0.4 {
		t.Fatalf("the face should tip the other way when the swing reverses: %.3f vs %.3f",
			swingOnly.Head.Tilt, opposite.Head.Tilt)
	}
}

func TestPoseIsFiniteUnderWildMotion(t *testing.T) {
	rig := CreateRig(RigOptions{})
	wild := []Motion{
		{Lean: 1e9, Swing: -1e9, Whip: 1e9, Droop: 99, LagX: -99, Flutter: 1e9, Stretch: 1e9, Gust: 99},
		{Lean: math.NaN(), Swing: math.Inf(1), Whip: math.Inf(-1)},
		{},
	}
	for _, motion := range wild {
		pose := PoseOf(rig, 6, motion)
		for i, j := range pose.Joints {
			if math.IsNaN(j.X) || math.IsNaN(j.Y) || math.IsInf(j.X, 0) || math.IsInf(j.Y, 0) {
				t.Fatalf("joint %d is not a number: %+v", i, j)
			}
		}
		if math.IsNaN(pose.Head.X) || math.IsNaN(pose.Head.Tilt) {
			t.Fatalf("the head is not a number: %+v", pose.Head)
		}
		for _, pet := range pose.Petals {
			if math.IsNaN(pet.Tip.X) || math.IsNaN(pet.Length) {
				t.Fatalf("a petal is not a number: %+v", pet)
			}
		}
		motion2 := MotionOf(motion)
		if math.Abs(motion2.Lean) > 1.4 || math.Abs(motion2.Swing) > 1.3 || motion2.Flutter < 0 || motion2.Flutter > 2.5 {
			t.Fatalf("motion was not clamped into the rig's vocabulary: %+v", motion2)
		}
	}
}

func TestDrawListIsPaintable(t *testing.T) {
	rig := CreateRig(RigOptions{})
	pose := PoseOf(rig, 6, Motion{Lean: -0.5, Swing: -0.4})
	list := BuildDrawList(rig, pose, DrawOptions{Detail: "full", Shadow: true})
	if len(list) < 200 {
		t.Fatalf("the drawing is suspiciously empty: %d shapes", len(list))
	}
	parts := map[string]int{}
	for _, shape := range list {
		parts[shape.Part]++
		if len(shape.Poly) < 3 {
			t.Fatalf("part %q has a degenerate polygon (%d points)", shape.Part, len(shape.Poly))
		}
		if shape.Alpha <= 0 || shape.Alpha > 1.0001 {
			t.Fatalf("part %q has an out-of-range alpha %v", shape.Part, shape.Alpha)
		}
		for _, p := range shape.Poly {
			if math.IsNaN(p.X) || math.IsNaN(p.Y) {
				t.Fatalf("part %q has a non-numeric point", shape.Part)
			}
		}
		switch shape.Fill.Type {
		case Solid:
			// fine as it is
		case Linear:
			if len(shape.Fill.Stops) < 2 {
				t.Fatalf("part %q has a gradient with %d stops", shape.Part, len(shape.Fill.Stops))
			}
		case Radial:
			if len(shape.Fill.Stops) < 2 || shape.Fill.R1 <= shape.Fill.R0 {
				t.Fatalf("part %q has a broken radial fill", shape.Part)
			}
		default:
			t.Fatalf("part %q has an unknown fill %q", shape.Part, shape.Fill.Type)
		}
	}
	for _, want := range []string{"shadow", "stem", "neck", "leaf", "glow", "petal", "center", "glint"} {
		if parts[want] == 0 {
			t.Fatalf("the drawing is missing its %q shapes", want)
		}
	}
}

func TestBloomOpensStaggeredAndSettles(t *testing.T) {
	rig := CreateRig(RigOptions{})
	bud := PoseOf(rig, 0, Motion{})
	if bud.MeanOpen > 0.05 {
		t.Fatalf("at t=0 the flower should be a bud, mean open %.3f", bud.MeanOpen)
	}
	bloomed := PoseOf(rig, 6, Motion{})
	if bloomed.MeanOpen < 0.98 {
		t.Fatalf("after the bloom window every petal should be open, mean %.3f", bloomed.MeanOpen)
	}
	// deterministic: the same call twice is the same flower
	again := PoseOf(rig, 6, Motion{})
	if bloomed.Head.X != again.Head.X || bloomed.MeanOpen != again.MeanOpen {
		t.Fatal("pose is not deterministic")
	}
}

func TestUnfurlOrderRanksEveryPetal(t *testing.T) {
	order := UnfurlOrder(18)
	seen := map[float64]bool{}
	for _, v := range order {
		if v < 0 || v > 1 {
			t.Fatalf("rank out of range: %v", v)
		}
		seen[v] = true
	}
	if len(seen) != 18 {
		t.Fatalf("expected 18 distinct ranks, got %d", len(seen))
	}
	// the first petal to open is a different one from the last
	if order[0] == order[len(order)-1] {
		t.Fatal("every petal got the same rank: the bloom would open all at once")
	}
}
