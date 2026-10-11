package flower

import (
	"math"
	"strconv"
)

// LeafPlacement says where a leaf attaches to the stem and which side it is on.
type LeafPlacement struct {
	At   float64
	Side float64
}

// Params is every dial the flower has. The zero value is not useful: call
// DefaultParams and adjust.
type Params struct {
	Seed    int
	Palette string

	// spine
	Height    float64 // stem length at full growth
	Steps     int     // integration steps (the joints the painters draw through)
	MinShoot  float64 // fraction of the stem present at t = 0 (the sprout)
	Bend      float64 // rest curvature: a gentle S, not a stick
	Hue       float64 // how far round the colour wheel the colourway is turned
	Curl      float64 // how curled the sprout is before it straightens out
	Wind      float64 // sway amplitude scale
	Follow    float64 // head follow-through (0 = rigid, 1 = floppy)
	StemWidth float64

	// The neck. A real stalk is stiff where it leaves the ground and limp under
	// the flower, which is why a flower carried in the hand swings from just
	// below the head instead of pivoting somewhere in the middle.
	NeckFrom   float64 // where the limpness starts, along the stem (0 base, 1 tip)
	NeckSag    float64 // the head's weight bending the top over at rest
	NeckLimp   float64 // how much the movement is concentrated up there
	NeckGain   float64 // the neck's angle per unit of lean/swing
	NeckStub   float64 // how far past the last joint the bloom hangs, in head radii
	NeckHang   float64 // the head's own weight: it hangs this far off the stalk's line
	HeadFollow float64 // how much of the neck's angle the face takes (1 = rigid)
	HeadSwing  float64 // how far the face itself swings on the neck
	CupFlare   float64 // how much the stalk widens going into the flower

	// timeline (seconds)
	GrowDur      float64
	BloomStart   float64
	BloomDur     float64
	PetalStagger float64

	// head
	// One ring of identical petals reads as a pinwheel however well it is
	// shaded, so the bloom is built the way a real one is: an outer ring, and a
	// shorter inner ring turned between the outer petals, opening a beat later.
	PetalCount   int
	PetalLayers  int
	InnerCount   float64
	InnerScale   float64
	InnerTurn    float64
	LayerDelay   float64
	PetalJitter  float64
	PetalSpread  float64
	PetalCurl    float64
	PetalLength  float64
	PetalWidth   float64
	Gravity      float64
	PetalWobble  float64
	HeadRadius   float64
	CenterRadius float64
	CenterDots   int
	Sepals       int

	// foliage
	Leaves     []LeafPlacement
	LeafLength float64
	LeafWidth  float64
}

// DefaultParams is the flower as it ships.
func DefaultParams() Params {
	return Params{
		Seed:    11,
		Palette: "blush",

		Height:    192,
		Steps:     34,
		MinShoot:  0.13,
		Bend:      0.24,
		Curl:      0.8,
		Wind:      0.6,
		Follow:    0.55,
		StemWidth: 8.4,

		NeckFrom:   0.38,
		NeckSag:    0.3,
		NeckLimp:   0.66,
		NeckGain:   0.62,
		NeckStub:   0.72,
		NeckHang:   0.2,
		HeadFollow: 0.84,
		HeadSwing:  0.62,
		CupFlare:   1.9,

		GrowDur:      1.0,
		BloomStart:   0.34,
		BloomDur:     1.25,
		PetalStagger: 0.55,

		PetalCount:   18,
		PetalLayers:  2,
		InnerCount:   0.62,
		InnerScale:   0.64,
		InnerTurn:    0.21,
		LayerDelay:   0.17,
		PetalJitter:  0.17,
		PetalSpread:  0.24,
		PetalCurl:    6.5,
		PetalLength:  62,
		PetalWidth:   26,
		Gravity:      0.5,
		PetalWobble:  0.05,
		HeadRadius:   16,
		CenterRadius: 15,
		CenterDots:   140,
		Sepals:       6,

		Leaves:     []LeafPlacement{{At: 0.34, Side: -1}, {At: 0.63, Side: 1}},
		LeafLength: 68,
		LeafWidth:  30,
	}
}

// Rig is the flower itself: the dials, and every petal, leaf and seed it grew,
// decided once at creation so a seed always draws the same plant.
type Rig struct {
	Params  Params
	Seed    int
	Petals  []PetalSpec
	Leaves  []LeafSpec
	Sepals  []SepalSpec
	Dots    []Dot
	Palette string
}

// NoMotion is the flower at rest.
var NoMotion = Motion{}

// MotionOf sanitises a motion snapshot into the range the rig understands. A
// value that is not a number at all becomes zero, so a bad frame can never draw
// a broken flower.
func MotionOf(m Motion) Motion {
	return Motion{
		Lean:    clamp(num(m.Lean), -1.4, 1.4),
		Swing:   clamp(num(m.Swing), -1.3, 1.3),
		Whip:    clamp(num(m.Whip), -1.4, 1.4),
		Droop:   clamp(num(m.Droop), 0, 1),
		LagX:    clamp(num(m.LagX), -1.6, 1.6),
		LagY:    clamp(num(m.LagY), -1.6, 1.6),
		Flutter: clamp(num(m.Flutter), 0, 2.5),
		Stretch: clamp(num(m.Stretch), -1, 1),
		Gust:    clamp(num(m.Gust), 0, 1.5),
	}
}

// UnfurlOrder ranks petals by golden angle. Petals do not open in ring order —
// they open in the order a spiral visits them, which is what makes a bloom look
// alive.
func UnfurlOrder(n int) []float64 {
	type entry struct {
		i   int
		key float64
	}
	ranked := make([]entry, n)
	for i := 0; i < n; i++ {
		ranked[i] = entry{i, math.Mod(float64(i)*Golden, TAU)}
	}
	// stable sort by key
	for i := 1; i < n; i++ {
		for j := i; j > 0 && ranked[j-1].key > ranked[j].key; j-- {
			ranked[j-1], ranked[j] = ranked[j], ranked[j-1]
		}
	}
	order := make([]float64, n)
	divisor := math.Max(1, float64(n-1))
	for rank, e := range ranked {
		order[e.i] = float64(rank) / divisor
	}
	return order
}

func centerDots(p Params, rng *RNG) []Dot {
	dots := make([]Dot, 0, p.CenterDots)
	for k := 0; k < p.CenterDots; k++ {
		f := math.Sqrt((float64(k) + 0.5) / float64(p.CenterDots))
		a := float64(k)*Golden + (rng.Next()-0.5)*0.05
		dots = append(dots, Dot{
			R: p.CenterRadius * 0.94 * f,
			A: a,
			// smaller and denser than they were: at icon size, big seeds read as dirt
			Size:  p.CenterRadius * (0.1 - 0.04*f) * (0.8 + rng.Next()*0.5),
			Shade: 0.84 + rng.Next()*0.34,
		})
	}
	return dots
}

// RigOptions overrides the defaults when the flower is grown.
type RigOptions struct {
	Seed    *int
	Palette string
	// Tweak gets the params last, for the rare caller that needs a dial the
	// options struct does not name.
	Tweak func(*Params)
}

// CreateRig grows a flower. Same seed, same flower, every time.
func CreateRig(opts RigOptions) *Rig {
	params := DefaultParams()
	if opts.Seed != nil {
		params.Seed = *opts.Seed
	}
	if opts.Palette != "" {
		params.Palette = opts.Palette
	}
	if _, ok := Palettes[params.Palette]; !ok {
		params.Palette = "blush"
	}
	if opts.Tweak != nil {
		opts.Tweak(&params)
	}
	rng := NewRNG(params.Seed)

	petals := []PetalSpec{}
	layers := max(1, params.PetalLayers)
	for layer := 0; layer < layers; layer++ {
		count := params.PetalCount
		if layer != 0 {
			count = max(5, int(math.Round(float64(params.PetalCount)*params.InnerCount)))
		}
		order := UnfurlOrder(count)
		turn := 0.0
		scale := 1.0
		if layer != 0 {
			turn = params.InnerTurn
			scale = params.InnerScale
		}
		jitter := params.PetalJitter
		if layer != 0 {
			jitter *= 1.7
		}
		for i := 0; i < count; i++ {
			petals = append(petals, PetalSpec{
				Index: petalIndex(layer, i),
				Layer: layer,
				Ring:  (float64(i)*TAU)/float64(count) + turn + (rng.Next()-0.5)*jitter,
				Delay: float64(layer)*params.LayerDelay + params.PetalStagger*order[i],
				Len:   scale * (1 + (rng.Next()-0.5)*params.PetalSpread),
				Width: petalWidthScale(layer) * (1 + (rng.Next()-0.5)*0.22),
				// the inner ring sits in the flower's own shadow, so it reads as depth
				Shade: petalShadeBase(layer) * (0.93 + rng.Next()*0.14),
				Sway:  rng.Next() * TAU,
				Twist: (rng.Next() - 0.5) * 0.22,
				// every petal curls a little differently, which is what stops a
				// ring of petals looking like a machine part
				Curl: (rng.Next() - 0.5) * 2,
			})
		}
	}

	leaves := make([]LeafSpec, 0, len(params.Leaves))
	for i, leaf := range params.Leaves {
		sign := -1.0
		if i != 0 {
			sign = 1
		}
		leaves = append(leaves, LeafSpec{
			At:      leaf.At,
			Side:    leaf.Side,
			Phase:   rng.Next() * TAU,
			Scale:   0.86 + rng.Next()*0.3,
			Droop:   0.3 + rng.Next()*0.24,
			Flutter: sign * (0.8 + rng.Next()*0.5),
		})
	}

	sepals := make([]SepalSpec, 0, params.Sepals)
	for k := 0; k < params.Sepals; k++ {
		f := 0.5
		if params.Sepals != 1 {
			f = float64(k) / float64(params.Sepals-1)
		}
		sepals = append(sepals, SepalSpec{
			Angle:  math.Pi * (0.14 + 0.72*f),
			Length: 0.86 + rng.Next()*0.3,
			Width:  0.5 + rng.Next()*0.3,
		})
	}

	return &Rig{
		Params:  params,
		Seed:    params.Seed,
		Petals:  petals,
		Leaves:  leaves,
		Sepals:  sepals,
		Dots:    centerDots(params, rng),
		Palette: params.Palette,
	}
}

func petalIndex(layer, i int) string {
	return strconv.Itoa(layer) + "." + strconv.Itoa(i)
}

func petalWidthScale(layer int) float64 {
	if layer != 0 {
		return 0.84
	}
	return 1
}

func petalShadeBase(layer int) float64 {
	if layer != 0 {
		return 1.05
	}
	return 0.99
}

// StemAngle is the rig's only time-dependent contract: stem angle from vertical
// at arclength s, at time tt — plus how the flower is being carried.
func StemAngle(rig *Rig, s, tt float64, motion Motion) float64 {
	m := MotionOf(motion)
	p := rig.Params
	grow := clamp01(tt / p.GrowDur)

	// 0 at the base, 1 under the flower. This single curve is what makes the
	// stalk behave: planted at the bottom, limp at the top, so the head swings on
	// the neck the way a flower in your hand does.
	neck := math.Pow(clamp01((s-p.NeckFrom)/(1-p.NeckFrom)), 1.7)
	limp := 0.14 + 0.86*neck*(1+p.NeckLimp)

	rest := p.Bend * math.Sin(s*math.Pi*1.15)
	curl := p.Curl * (1 - grow) * (0.15 + 1.15*math.Pow(s, 1.5))
	wind := p.Wind *
		(0.2*math.Sin(tt*1.15-s*1.9) +
			0.1*math.Sin(tt*1.87-s*1.2+1.3) +
			0.05*math.Sin(tt*3.3-s*2.6+0.6)) *
		(0.22 + 0.78*s) *
		clamp01(grow*1.5)
	// rest: the head's weight bends the neck over, only where the neck is
	sag := p.NeckSag * neck * clamp01(grow*1.4)
	// travel and the head's pendulum both go through the neck, which is why the
	// petals answer every movement of the top of the stalk
	travel := (m.Lean + m.Swing) * limp * p.NeckGain
	// momentum: a slower S-shaped flex, so a whip reads as a whip and not a lean
	whip := m.Whip * math.Sin(s*math.Pi*0.92) * (0.3 + 0.85*neck) * 0.5
	// weight: a heavy head curls the upper stem over, never the base
	droop := m.Droop * math.Pow(s, 1.9) * 2.3
	gust := m.Gust * math.Sin(tt*5.4-s*3.1) * (0.1 + 0.5*s) * clamp01(grow)
	return rest + curl + wind + sag + travel + whip + droop + gust
}

// PoseOf is the skeleton of the flower at time t, carried as `motion` says.
func PoseOf(rig *Rig, tIn float64, motion Motion) Pose {
	m := MotionOf(motion)
	p := rig.Params
	t := math.Max(0, tIn)
	if math.IsNaN(t) {
		t = 0
	}

	// --- growth: a curled sprout springs up into a stem
	grow := clamp01(t / p.GrowDur)
	growEase := EaseOutBack(grow, 1.5) // overshoots ~6%: a real spring
	// squash and stretch: moving, the stem is pulled long; on impact it pancakes
	squash := 1 + m.Stretch*0.1
	if m.Stretch < 0 {
		squash = 1 + m.Stretch*0.22
	}
	stemLength := p.Height * lerp(p.MinShoot, 1, growEase) * squash

	// --- bloom
	bloomProgress := clamp01((t - p.BloomStart) / p.BloomDur)

	// --- spine: integrate the curvature function into connected joints
	steps := p.Steps
	joints := make([]Point, 0, steps+1)
	tangents := make([]float64, 0, steps+1)
	ds := stemLength / float64(steps)
	x, y := 0.0, 0.0
	theta := StemAngle(rig, 0, t, m)
	for i := 0; i <= steps; i++ {
		s := float64(i) / float64(steps)
		joints = append(joints, Point{x, y})
		tangents = append(tangents, theta)
		theta = StemAngle(rig, s, t, m)
		if i < steps {
			x += math.Sin(theta) * ds
			y += math.Cos(theta) * ds
		}
	}
	tip := joints[steps]
	tipAngle := tangents[steps]

	// --- head: follows the tip, with follow-through so it nods instead of
	// snapping to the stem's angle
	//
	// The face is attached to the top of the neck: it takes most of the tip's
	// angle, so if the top of the stalk moves the whole flower goes with it.
	lagAngle := StemAngle(rig, 1, math.Max(0, t-0.14), Motion{
		Lean:    m.Lean * 0.6,
		Swing:   m.Swing * 0.6,
		Whip:    m.Whip * 0.6,
		Droop:   m.Droop,
		Flutter: m.Flutter,
		Stretch: m.Stretch,
		Gust:    m.Gust,
	})
	headAngle := lerp(tipAngle, lagAngle, (1-p.HeadFollow)*clamp01(grow*2))
	nod := 0.045 * math.Sin(t*1.15+0.4) * grow
	// A bloom is not welded to the last joint. It hangs a neck's length past it,
	// along the direction that neck is pointing, so the top of the stalk can be
	// seen carrying the flower — and because that direction is the bent one, the
	// whole bloom (petals and all) swings out on the end of the stalk.
	neckLen := p.NeckStub * p.HeadRadius * lerp(0.5, 1, clamp01(grow*1.6)) * lerp(0.4, 1, clamp01(bloomProgress*1.4))
	hang := p.NeckHang * clamp01(grow*1.3) // its own weight, on a limp neck
	neckAngle := headAngle + hang
	// and it swings a little further than the neck that carries it, the way a
	// top-heavy thing does on a limp stem
	head := Head{
		X:          tip.X + math.Sin(neckAngle)*neckLen,
		Y:          tip.Y + math.Cos(neckAngle)*neckLen,
		Angle:      headAngle,
		NeckAngle:  neckAngle,
		NeckLength: neckLen,
		Tip:        tip,
		// the face turns with the neck, but only so far: a flower looks up.
		Tilt:   clamp(neckAngle, -1.1, 1.1)*0.5 + nod + m.Swing*p.HeadSwing*0.5 - m.Lean*0.1 - m.Droop*0.3,
		Radius: p.HeadRadius * lerp(0.62, 1, clamp01(bloomProgress*1.6)),
		Swing:  m.Swing,
	}
	headScale := lerp(0.6, 1, clamp01(bloomProgress*1.5))

	// --- petals: bud (all upright, small) -> golden ring, spiralling open
	// the last petal to open (the inner ring's) has to be fully open exactly when
	// the bloom ends, so the stagger is normalised over the whole opening window
	openSpan := math.Max(0.05, 1-p.PetalStagger-float64(max(0, p.PetalLayers-1))*p.LayerDelay)
	petals := make([]Petal, 0, len(rig.Petals))
	for _, pt := range rig.Petals {
		raw := clamp01((bloomProgress - pt.Delay) / openSpan)
		open := EaseOutBack(raw, 0.85)
		openClamped := clamp01(open)
		angle := lerp(math.Pi/2, pt.Ring, 1-0.85*(1-openClamped))
		reach := p.PetalLength * pt.Len * lerp(0.6, 1, openClamped) * headScale
		halfWidth := (p.PetalWidth / 2) * pt.Width * lerp(0.55, 1, openClamped) * headScale
		wobble := math.Sin(t*2.1+pt.Sway) * p.PetalWobble * reach
		droop := -p.Gravity * reach * 0.2 * math.Max(0, -math.Sin(pt.Ring)) * openClamped
		// a petal is a lever on the head: the further out it sits, the further it
		// trails behind when the flower is carried, and the more wind it catches
		outer := 0.55 + 0.45*openClamped
		flutter := math.Sin(t*1.7+pt.Sway*1.3)*reach*0.02*openClamped +
			math.Sin(t*(5.1+pt.Shade)+pt.Sway*2.3)*reach*0.016*m.Flutter*outer
		// fake depth: petals facing the viewer (down) are a touch shorter
		depth := math.Sin(pt.Ring) * openClamped
		inertiaX := m.LagX * reach * 0.34 * outer
		inertiaY := m.LagY * reach * 0.34 * outer
		curl := pt.Curl * p.PetalCurl * openClamped * lerp(0.4, 1, outer)
		tipX := math.Cos(angle)*reach*(1+0.045*depth) + wobble*0.5 + inertiaX
		tipY := math.Sin(angle)*reach*(1+0.045*depth) + droop + flutter + inertiaY
		petals = append(petals, Petal{
			Index:    pt.Index,
			Layer:    pt.Layer,
			Ring:     pt.Ring,
			Open:     openClamped,
			Flared:   open > 1,
			Length:   reach,
			HalfWide: halfWidth,
			Shade:    pt.Shade,
			Twist:    pt.Twist * openClamped,
			Bend:     curl,
			Tip:      Point{tipX, tipY},
		})
	}

	// --- leaves: attached to the spine, fluttering, drooping, growing late
	leaves := make([]Leaf, 0, len(rig.Leaves))
	for _, leaf := range rig.Leaves {
		idx := clampInt(int(math.Round(leaf.At*float64(steps))), 0, steps)
		ax := joints[idx]
		leafGrow := clamp01((t - p.GrowDur*0.35) / (p.GrowDur * 0.75))
		outward := clamp01(growEase) * clamp01(leafGrow)
		flutter := math.Sin(t*1.7+leaf.Phase)*0.09*leaf.Flutter +
			math.Sin(t*6.3+leaf.Phase*1.7)*0.05*leaf.Flutter*m.Flutter +
			m.LagX*0.16*leaf.Side
		// leaves leave the stem about 72 degrees off vertical, then droop under
		// their own weight as they flare out
		rest := 1.26 - leaf.Droop*0.34*outward - m.Droop*0.34
		angle := rest + flutter
		if leaf.Side <= 0 {
			angle = math.Pi - rest - flutter
		}
		length := p.LeafLength * leaf.Scale * lerp(0.25, 1, outward) * (1 + m.Stretch*0.12)
		halfWidth := (p.LeafWidth / 2) * lerp(0.4, 1, outward)
		leaves = append(leaves, Leaf{
			Index:    idx,
			Attach:   ax,
			Angle:    angle,
			Length:   length,
			HalfWide: halfWidth,
			Tip:      Point{math.Cos(angle) * length, math.Sin(angle) * length},
			Side:     leaf.Side,
			Flutter:  flutter,
			Grow:     outward,
		})
	}

	openCount := 0
	meanOpen := 0.0
	for _, pet := range petals {
		if pet.Open > 0.05 {
			openCount++
		}
		meanOpen += pet.Open
	}
	meanOpen /= float64(max(1, len(petals)))

	return Pose{
		T:              t,
		Motion:         m,
		Grow:           clamp01(growEase),
		Bloom:          bloomProgress,
		StemLength:     stemLength,
		Joints:         joints,
		Tangents:       tangents,
		Head:           head,
		Petals:         petals,
		Leaves:         leaves,
		OpenPetalCount: openCount,
		MeanOpen:       meanOpen,
	}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
