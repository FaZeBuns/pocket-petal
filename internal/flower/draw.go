package flower

import "math"

// DrawOptions mirrors the JS buildDrawList options.
type DrawOptions struct {
	Palette string
	// Detail "lite" drops the pencil work (rims, veins, pigment specks) for small
	// or fast-moving renderings, where it would be sub-pixel noise anyway.
	Detail string
	Shadow bool
}

func s3(at float64, c RGB, a float64) Stop { return Stop{At: at, Color: c, Alpha: a} }

func solidStop(c RGB) []Stop { return []Stop{{At: 0, Color: c, Alpha: 1}, {At: 1, Color: c, Alpha: 1}} }

// BuildDrawList turns a pose into flat polygons with fills: the entire drawing,
// in the order it is painted.
func BuildDrawList(rig *Rig, p Pose, opts DrawOptions) []Shape {
	params := rig.Params
	pal := PaletteNamed(opts.Palette)
	if opts.Palette == "" {
		pal = PaletteNamed(rig.Palette)
	}
	// the colourway the flower was grown with, turned as far round the wheel as
	// the settings ask for: one flower's colours, in every colour there is
	pal = HueTurn(pal, params.Hue)
	full := opts.Detail != "lite"
	showShadow := opts.Shadow
	list := []Shape{}
	push := func(part string, poly []Point, fill Fill, alpha float64) {
		if len(poly) < 3 || alpha <= 0.002 {
			return
		}
		list = append(list, Shape{Part: part, Poly: poly, Fill: fill, Alpha: alpha})
	}

	// --- soft contact shadow under the sprout
	if showShadow {
		r := params.Height * 0.16
		push("shadow", Ellipse(Point{0, 0}, r, r*0.26, 22), FillRadial(Point{0, 0}, 0, r, []Stop{
			s3(0, RGB{38, 28, 44}, 0.85),
			s3(0.4, RGB{38, 28, 44}, 0.42),
			s3(1, RGB{38, 28, 44}, 0),
		}), 0.3*clamp01(p.Grow*1.4))
	}

	// --- stem: a tapered ribbon, lit from the upper left, that widens into the
	// neck where it has to carry the flower
	stemHw := params.StemWidth / 2
	widthAt := func(s float64) float64 {
		taper := stemHw * (1.05 - 0.42*s)
		// the last stretch of stalk is the neck, and a neck is not a straw
		f := clamp01((s - 0.78) / 0.22)
		return taper * (1 + (params.CupFlare-1)*math.Pow(f, 1.5)*(1-0.2*math.Pow(f, 6)))
	}
	push("stem", Ribbon(p.Joints, widthAt), FillLinear(
		Point{p.Joints[0].X - stemHw, p.Joints[0].Y},
		Point{p.Joints[0].X + stemHw*1.4, p.Joints[0].Y + params.Height*0.7},
		[]Stop{
			s3(0, pal.StemLight, 1),
			s3(0.55, pal.Stem, 1),
			s3(1, pal.StemDark, 1),
		}), 1)
	highlight := make([]Point, len(p.Joints))
	for i, pt := range p.Joints {
		highlight[i] = Point{pt.X - stemHw*0.45*(1-float64(i)/float64(len(p.Joints))), pt.Y}
	}
	push("stem", Ribbon(highlight, func(s float64) float64 { return stemHw * 0.22 * (1 - s*0.6) }), FillSolid(pal.StemLight), 0.4)

	// --- the neck into the flower.
	//
	// Built from the stem's own last joints, so the two can never come apart:
	// the stalk swells into a calyx cup, and the head sits *in* it. This is the
	// join that used to look like a flower balanced on a stick.
	neckFrom := max(1, int(math.Floor(float64(len(p.Joints))*0.66)))
	neckJoints := append([]Point{}, p.Joints[neckFrom:]...)
	neckJoints = append(neckJoints, Point{p.Head.X, p.Head.Y})
	cupTop := params.HeadRadius * 1.15
	cupBase := stemHw * 0.72
	cupProfile := func(s float64) float64 {
		rise := math.Pow(clamp01(s/0.84), 1.15)
		lip := 1 - 0.16*math.Pow(clamp01((s-0.84)/0.16), 1.6)
		return lerp(cupBase, cupTop, rise*lip)
	}
	push("neck", Ribbon(neckJoints, cupProfile), FillLinear(
		Point{p.Head.X - cupTop, p.Head.Y},
		Point{p.Head.X + cupTop*1.5, p.Head.Y - params.Height*0.16},
		[]Stop{
			s3(0, pal.StemLight, 1),
			s3(0.45, pal.Stem, 1),
			s3(1, pal.StemDark, 1),
		}), 1)
	// the cup's rim catches the light
	tail := neckJoints[len(neckJoints)-4:]
	push("neck", Ribbon(tail, func(s float64) float64 { return cupProfile(0.86+s*0.14) * 0.28 * (1 - s*0.5) }), FillSolid(pal.StemLight), 0.5)
	// and the stalk keeps a lit edge all the way up into it
	lit := make([]Point, len(neckJoints))
	for i, pt := range neckJoints {
		s := float64(i) / float64(len(neckJoints)-1)
		lit[i] = Point{pt.X - cupProfile(s)*0.45, pt.Y}
	}
	push("neck", Ribbon(lit, func(s float64) float64 { return cupProfile(s) * 0.2 * (1 - s*0.4) }), FillSolid(pal.StemLight), 0.35)

	// the knot the bloom hangs from: the calyx takes the weight where the stalk
	// meets the flower, and because it sits on the neck it travels with it — the
	// cue that the flower and the top of the stalk are one piece.
	bossAngle := p.Head.NeckAngle
	bossDist := p.Head.NeckLength * 0.72
	bossX := p.Head.X - math.Sin(bossAngle)*bossDist
	bossY := p.Head.Y - math.Cos(bossAngle)*bossDist
	bossR := params.HeadRadius * 0.7
	bossLight := Point{bossX + math.Sin(bossAngle)*bossR*0.4, bossY + math.Cos(bossAngle)*bossR*0.4}
	push("neck", Ellipse(Point{bossX, bossY}, bossR, bossR*0.92, 22), FillLinear(
		Point{bossX - bossR, bossY - bossR},
		Point{bossX + bossR, bossY + bossR},
		[]Stop{
			s3(0, pal.StemLight, 1),
			s3(0.55, pal.Stem, 1),
			s3(1, pal.StemDark, 1),
		}), 1)
	// a crease of shadow where the head is seated, so the join reads as a joint
	push("neck", Ellipse(Point{bossX, bossY}, bossR*0.62, bossR*0.34, 20), FillRadial(Point{bossX, bossY}, 0, bossR*0.62, solidStop(RGB{22, 34, 20})), 0.26)
	// and the sun catches the top of the knot
	push("neck", Ellipse(bossLight, bossR*0.5, bossR*0.3, 18), FillSolid(pal.StemLight), 0.4)

	// --- leaves (behind the head, alternating sides)
	for _, leaf := range p.Leaves {
		if leaf.Grow <= 0.02 {
			continue
		}
		bladeSteps := 6
		if full {
			bladeSteps = 9
		}
		poly := Blade(BladeOptions{
			Tip:       leaf.Tip,
			HalfWidth: leaf.HalfWide,
			Steps:     bladeSteps,
			Profile:   "leaf",
			Serration: 0.07,
		})
		local := make([]Point, len(poly))
		for i, pt := range poly {
			local[i] = Point{leaf.Attach.X + pt.X, leaf.Attach.Y + pt.Y}
		}
		push("leaf", local, FillLinear(Point{0, 0}, leaf.Tip, []Stop{
			s3(0, Tint(pal.Leaf, 0.82), 1),
			s3(0.6, Tint(pal.Leaf, 1.05), 1),
			s3(1, Tint(pal.Leaf, 0.88), 1),
		}), 0.98)

		bladePt := func(s float64) Point {
			return QuadPoint(Point{0, 0}, Point{leaf.Tip.X * 0.55, leaf.Tip.Y * 0.55}, leaf.Tip, s)
		}
		ribPts := make([]Point, 7)
		for i := range ribPts {
			pt := bladePt(float64(i) / 6)
			ribPts[i] = Point{leaf.Attach.X + pt.X, leaf.Attach.Y + pt.Y}
		}
		push("leaf", Ribbon(ribPts, func(s float64) float64 { return leaf.HalfWide * 0.14 * (1 - s) }), FillSolid(pal.StemDark), 0.45)
		if full {
			for v := 1; v <= 4; v++ {
				s := 0.16 + float64(v)*0.18
				b := bladePt(s)
				for _, dir := range []float64{-1, 1} {
					veinTip := Point{
						b.X + (leaf.Tip.X-b.X)*0.42 + dir*leaf.HalfWide*0.3*(1-s*0.5),
						b.Y + (leaf.Tip.Y-b.Y)*0.34 + dir*leaf.HalfWide*0.16,
					}
					vein := []Point{
						{leaf.Attach.X + b.X, leaf.Attach.Y + b.Y},
						{leaf.Attach.X + veinTip.X, leaf.Attach.Y + veinTip.Y},
					}
					push("leaf", Ribbon(vein, func(t float64) float64 { return leaf.HalfWide * 0.05 * (1 - t) }), FillSolid(pal.StemDark), 0.28)
				}
			}
		}
	}

	// --- the flower face (head-local, then rotated into place)
	local := []Shape{}
	pushLocal := func(part string, poly []Point, fill Fill, alpha float64) {
		local = append(local, Shape{Part: part, Poly: poly, Fill: fill, Alpha: alpha})
	}
	headR := p.Head.Radius
	haloR := params.PetalLength * 2.3
	pushLocal("glow", Ellipse(Point{-haloR * 0.1, haloR * 0.12}, haloR, haloR*0.94, 30), FillRadial(
		Point{-haloR * 0.1, haloR * 0.12}, 0, haloR, []Stop{
			s3(0, RGB{255, 250, 232}, 0.3),
			s3(0.3, RGB{255, 243, 214}, 0.22),
			s3(0.6, RGB{255, 234, 190}, 0.12),
			s3(0.85, RGB{255, 228, 182}, 0.04),
			s3(1, RGB{255, 226, 180}, 0),
		}), clamp01(p.Bloom*1.3))

	// receptacle / calyx: a slim green cup at the back of the flower, never a ball
	pushLocal("head", Ellipse(Point{0, 0}, headR*0.5, headR*0.5, 22), FillSolid(pal.StemDark), 1)
	for _, sepal := range rig.Sepals {
		// closed: the calyx wraps *up* around the bud. open: it folds back under
		// the petals, the way a real flower's sepals do.
		angle := lerp(math.Pi/2+math.Sin(sepal.Angle)*0.45, sepal.Angle, clamp01(p.Bloom*1.5))
		length := headR * (0.95 + sepal.Length*0.5) * lerp(1.1, 1.35, clamp01(p.Bloom))
		tip := Point{math.Cos(angle) * length, math.Sin(angle) * length}
		pushLocal("sepal", Blade(BladeOptions{
			Tip:       tip,
			HalfWidth: headR * 0.26 * sepal.Width,
			Steps:     5,
			Profile:   "leaf",
		}), FillLinear(Point{0, 0}, tip, []Stop{
			s3(0, pal.StemDark, 1),
			s3(1, pal.Stem, 1),
		}), 0.98)
	}

	// Petals: lower ones last so they sit "in front" at the base.
	ordered := make([]Petal, len(p.Petals))
	copy(ordered, p.Petals)
	sortPetalsByRing(ordered)
	petalSteps := 8
	if full {
		petalSteps = 12
	}
	// The sun, rotated into the head's own frame: the head is tipped, so the light
	// has to tip with it or the shading would turn with the flower like a decal.
	cosT := math.Cos(-p.Head.Tilt)
	sinT := math.Sin(-p.Head.Tilt)
	sunX := Sun.X*cosT - Sun.Y*sinT
	sunY := Sun.X*sinT + Sun.Y*cosT
	for _, pet := range ordered {
		// how much this petal faces the sun, and which side of it is lit
		facing := math.Cos(pet.Ring)*sunX + math.Sin(pet.Ring)*sunY
		litSide := math.Signbit(-math.Sin(pet.Ring)*sunX + math.Cos(pet.Ring)*sunY)
		lit := 1.0
		if litSide {
			lit = -1
		}
		shade := pet.Shade * (1 + 0.13*math.Sin(pet.Ring)) * lerp(0.84, 1.24, (facing+1)/2)
		base := InSun(pal.PetalBase, shade, facing)
		tipc := InSun(pal.PetalTip, shade, facing)
		deep := InSun(pal.PetalDeep, shade, facing*0.5)
		// Rim light: a bright copy nudged towards the sun, drawn first so only the
		// sliver that pokes out past the petal shows — the edge of a petal in
		// sunlight, which is what makes the bloom look lit instead of painted.
		if full {
			pushLocal("petal", Blade(BladeOptions{
				Tip:       Point{pet.Tip.X + sunX*pet.HalfWide*0.44, pet.Tip.Y + sunY*pet.HalfWide*0.44},
				HalfWidth: pet.HalfWide,
				Steps:     petalSteps - 3,
				Profile:   "petal",
				Notch:     0.16,
				Bend:      pet.Bend,
			}), FillSolid(InSun(pal.PetalTip, 1.26, 1)), 0.55)
		}
		// Occlusion, not an outline. Every petal in front of this one is already on
		// the canvas, so a dark copy lifted a hair towards the back reads as the
		// shadow this petal casts on them — which is what separates petal from
		// petal. An outline would just make the whole bloom look like a sticker.
		pushLocal("petal", Blade(BladeOptions{
			// down and slightly in: the shadow of this petal falls on the ones
			// behind it (the light is above), never out past its own tip
			Tip:       Point{pet.Tip.X - pet.HalfWide*0.1, pet.Tip.Y - params.PetalLength*0.045},
			HalfWidth: pet.HalfWide * 0.99,
			Steps:     petalSteps - 2,
			Profile:   "petal",
			Notch:     0.2,
			Bend:      pet.Bend,
		}), FillSolid(RGB{26, 15, 28}), pet.layerAlpha())
		pushLocal("petal", Blade(BladeOptions{
			Tip:       pet.Tip,
			HalfWidth: pet.HalfWide,
			Steps:     petalSteps,
			Profile:   "petal",
			Notch:     0.13,
			Bend:      pet.Bend,
		}), FillLinear(Point{0, 0}, pet.Tip, []Stop{
			s3(0, deep, 1),
			s3(0.09, deep, 1),
			s3(0.42, base, 1),
			s3(0.86, InSun(base, 1.03, facing), 1),
			s3(1, tipc, 1),
		}), 1)
		// the inner cup: a narrower blade in a deeper tone gives the petal volume
		pushLocal("petal", Blade(BladeOptions{
			Tip:       Point{pet.Tip.X * 0.86, pet.Tip.Y * 0.86},
			HalfWidth: pet.HalfWide * 0.46,
			Steps:     7,
			Profile:   "petal",
			Bend:      pet.Bend * 0.75,
		}), FillLinear(Point{0, 0}, Point{pet.Tip.X * 0.86, pet.Tip.Y * 0.86}, []Stop{
			s3(0, deep, 1),
			s3(1, Tint(pal.PetalEdge, shade*1.02), 1),
		}), 0.42)
		if full {
			// midrib + two branching veins: the "drawn" part of the drawing
			ribPts := make([]Point, 6)
			for i := range ribPts {
				ribPts[i] = BladeCentre(pet.Tip, float64(i)/5, pet.Bend, 0.55)
			}
			pushLocal("petal", Ribbon(ribPts, func(s float64) float64 { return pet.HalfWide * 0.1 * (1 - s*0.85) }), FillSolid(deep), 0.3)
			for _, dir := range []float64{-1, 1} {
				for _, s := range []float64{0.34, 0.62} {
					b := BladeCentre(pet.Tip, s, pet.Bend, 0.55)
					vTip := Point{
						b.X + pet.Tip.X*0.2 + dir*pet.HalfWide*0.5,
						b.Y + pet.Tip.Y*0.22 + dir*pet.HalfWide*0.2,
					}
					pushLocal("petal", Ribbon([]Point{b, vTip}, func(t float64) float64 { return pet.HalfWide * 0.05 * (1 - t) }), FillSolid(deep), 0.18)
				}
			}
			// and a sheen down the side that catches the light, so the petal has a
			// round surface and not a flat fill
			sheenPts := make([]Point, 3)
			for i, s := range []float64{0.4, 0.68, 0.94} {
				sheenPts[i] = BladeCentre(pet.Tip, s, pet.Bend-pet.HalfWide*0.4*lit, 0.55)
			}
			pushLocal("petal", Ribbon(sheenPts, func(s float64) float64 { return pet.HalfWide * (0.14 + 0.07*facing) * (1 - s*0.7) }),
				FillSolid(InSun(pal.PetalTip, 1.22, facing)),
				// the petals turn to the sun: the ones facing it take the shine, the
				// ones facing away keep their own colour
				0.24+0.22*clamp01((facing+0.15)/1.15))
		}
	}

	// centre disc + phyllotaxis seeds. In the bud the disc is barely there — the
	// petals are what you see — so it grows in as the flower opens.
	discR := params.CenterRadius * lerp(0.14, 1, clamp01(p.Bloom*1.15))
	// a deeper rim behind the disc, so the centre sits *in* the flower
	pushLocal("center", Ellipse(Point{0, 0}, discR*1.16, discR*1.16, 30), FillSolid(pal.CenterHot), lerp(0.3, 0.85, clamp01(p.Bloom)))
	pushLocal("center", Ellipse(Point{0, 0}, discR, discR, 32), FillRadial(
		Point{-discR * 0.35, discR * 0.35}, 0, discR*1.55, []Stop{
			s3(0, pal.Center, 1),
			s3(0.55, pal.Center, 1),
			s3(1, Tint(pal.CenterHot, 1.02), 1),
		}), lerp(0.45, 1, clamp01(p.Bloom*1.2)))
	// the rim of the disc sits *in* the flower: a soft inner shadow
	pushLocal("center", Ellipse(Point{0, 0}, discR*1.06, discR*1.06, 30), FillRadial(Point{0, 0}, discR*0.6, discR*1.06, solidStop(RGB{60, 34, 12})), 0.22*clamp01(p.Bloom*1.4))
	dotScale := lerp(0.4, 1, clamp01(p.Bloom*1.3))
	dotAlpha := clamp01(p.Bloom*1.6 - 0.25)
	dots := rig.Dots
	if !full && len(dots) > 46 {
		dots = dots[:46]
	}
	for _, dot := range dots {
		dx := math.Cos(dot.A) * dot.R * dotScale
		dy := math.Sin(dot.A) * dot.R * dotScale
		r := dot.Size * dotScale
		if r < 0.12 || dotAlpha <= 0.01 {
			continue
		}
		pushLocal("center", Ellipse(Point{dx, dy}, r, r, 7), FillSolid(Tint(pal.CenterHot, dot.Shade)), 0.92*dotAlpha)
	}

	// a warm top-left glow over the whole face: the flower in the sun
	pushLocal("glint", Ellipse(Point{headR * 0.3, headR * 0.42}, headR*0.86, headR*0.62, 22), FillRadial(
		Point{headR * 0.3, headR * 0.42}, 0, headR*0.86, []Stop{
			s3(0, RGB{255, 246, 214}, 1),
			s3(1, RGB{255, 236, 190}, 1),
		}), 0.16)
	// the centre of a sunflower in the sun is a hot little dome
	pushLocal("glint", Ellipse(Point{-discR * 0.3, discR * 0.34}, discR*0.66, discR*0.5, 20), FillRadial(
		Point{-discR * 0.3, discR * 0.34}, 0, discR*0.66, []Stop{
			s3(0, RGB{255, 246, 214}, 1),
			s3(1, RGB{255, 214, 130}, 1),
		}), 0.42*clamp01(p.Bloom*1.2))
	// a dewdrop, and the sparkle it throws
	dropX := -headR * 0.62
	dropY := headR * 0.2
	pushLocal("glint", Ellipse(Point{dropX, dropY}, 2.6, 3.2, 14), FillSolid(RGB{255, 255, 255}), 0.42)
	pushLocal("glint", Ellipse(Point{dropX - 0.7, dropY + 0.8}, 1.1, 1.3, 12), FillSolid(RGB{255, 255, 255}), 0.85)
	if full {
		// the sparkle the sun throws off a wet petal: the one thing that reads as
		// "sunlit" at any size, so it is drawn big and bright
		sp1 := Point{-headR * 1.05, headR * 0.82}
		pushLocal("glint", Star(sp1, headR*0.78, 0.14), FillRadial(sp1, 0, headR*0.78, []Stop{
			s3(0, RGB{255, 255, 255}, 1),
			s3(1, RGB{255, 250, 235}, 1),
		}), 0.62)
		pushLocal("glint", Star(Point{headR * 0.86, -headR * 0.56}, headR*0.42, 0.16), FillSolid(RGB{255, 252, 240}), 0.42)
		pushLocal("glint", Star(Point{headR * 0.1, headR * 0.98}, headR*0.3, 0.2), FillSolid(RGB{255, 255, 246}), 0.3)
	}

	for _, shape := range local {
		push(shape.Part, TransformPoly(shape.Poly, Point{p.Head.X, p.Head.Y}, p.Head.Tilt), shape.Fill, shape.Alpha)
	}

	return list
}

// layerAlpha is the shadow a petal casts on the ones behind it: the inner ring
// sits deeper in the flower's own shade.
func (p Petal) layerAlpha() float64 {
	if p.Layer != 0 {
		return 0.16
	}
	return 0.1
}

// sortPetalsByRing orders petals back to front (by sin(ring): the petals at the bottom of the bloom are painted last so
// they sit in front at the base).
func sortPetalsByRing(list []Petal) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && math.Sin(list[j-1].Ring) < math.Sin(list[j].Ring); j-- {
			list[j-1], list[j] = list[j], list[j-1]
		}
	}
}
