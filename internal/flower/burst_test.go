package flower

import (
	"math"
	"math/rand"
	"testing"
)

func bloomFrame(t *testing.T) []Shape {
	t.Helper()
	rig := CreateRig(RigOptions{})
	pose := PoseOf(rig, FullBloom, Motion{Lean: -0.3})
	return BuildDrawList(rig, pose, DrawOptions{Detail: "full", Shadow: true})
}

func TestPetalPiecesAreTheFlowersOwnPetals(t *testing.T) {
	list := bloomFrame(t)
	pieces := PetalPieces(list)
	want := 0
	for _, shape := range list {
		if shape.Part == "petal" {
			want++
		}
	}
	if len(pieces) != want || want < 10 {
		t.Fatalf("lifted %d pieces out of %d petal shapes", len(pieces), want)
	}
	// every piece is centred on its own middle, which is what lets it spin about
	// itself instead of orbiting the flower
	for _, piece := range pieces {
		var cx, cy float64
		for _, p := range piece.Poly {
			cx += p.X
			cy += p.Y
		}
		if n := float64(len(piece.Poly)); math.Abs(cx/n) > 1e-9 || math.Abs(cy/n) > 1e-9 {
			t.Fatalf("piece is not centred on its middle: %.6f, %.6f", cx/n, cy/n)
		}
		if piece.Alpha <= 0 {
			t.Fatal("a petal came off invisible")
		}
	}
}

// burstBudget is the reach a burst is fitted to, in design units, and oldVistaHalf
// is the half-width of the fixed square a burst used to fly in.
const (
	burstBudget  = 1400.0 // the reach the app fits a burst to (buddy.burstReach)
	oldVistaHalf = 310.0
)

// A burst is a cloud that spreads out and sinks, not a box around the bloom, and
// the square that used to be handed to the rasteriser was smaller than the flight:
// a petal that outran it was sliced off square at the bottom and at the sides,
// which is exactly what a person watching the explosion sees. This states the size
// of the problem, so that the fix cannot quietly be undone.
func TestABurstOutrunsTheSquareItUsedToFlyIn(t *testing.T) {
	rig := CreateRig(RigOptions{})
	pose := PoseOf(rig, FullBloom, Motion{})
	for _, c := range []struct {
		what  string
		power float64
	}{
		{"a plain drop", 1},
		{"a firm drop", 1.4},
		{"a flower flung across the desk", 1.7},
	} {
		pieces := Confetti(rig, Point{X: pose.Head.X, Y: pose.Head.Y}, 84, rand.New(rand.NewSource(4)))
		burst := NewBurst(pieces, rand.New(rand.NewSource(4)), c.power, 0, 0)
		box := burst.Flight(burstBudget)
		if reach := reachFrom(burst, box); reach <= oldVistaHalf {
			t.Errorf("%s reaches %.0f, and the old square only had %.0f: the fix is untested", c.what, reach, oldVistaHalf)
		}
	}
}

// The promise the surface rests on: whatever box a burst says it will draw in,
// every petal of it stays inside that box for the whole flight. If this holds, a
// surface the size of the box cannot cut a burst off, and no amount of tuning — a
// harder throw, a slower fade, more gravity — can change that.
func TestTheBurstNeverDrawsOutsideTheBoxItIsGiven(t *testing.T) {
	rig := CreateRig(RigOptions{})
	pose := PoseOf(rig, FullBloom, Motion{Lean: -0.4})
	centre := Point{X: pose.Head.X, Y: pose.Head.Y}
	for _, c := range []struct {
		what   string
		power  float64
		throwX float64
		throwY float64
	}{
		{"a plain drop", 1, 0, 0},
		{"a gentle drop", 0.4, 0, 0},
		{"thrown to the right", 1.4, 900, 0},
		{"flung downwards", 1.6, 0, -1400},
		{"flung hard in both", 1.7, 2600, 2200},
	} {
		pieces := Confetti(rig, centre, 84, rand.New(rand.NewSource(21)))
		burst := NewBurst(pieces, rand.New(rand.NewSource(21)), c.power, c.throwX, c.throwY)
		box := burst.Flight(burstBudget)
		if reach := reachFrom(burst, box); reach > burstBudget+1e-9 {
			t.Errorf("%s: the flight reaches %.1f and the budget is %.0f", c.what, reach, burstBudget)
		}
		// fly it for real, and check every frame against the box it was given
		for i := 0; i < 600 && !burst.Done(); i++ {
			burst.Step(1.0 / 60)
			for _, shape := range burst.Shapes() {
				for _, p := range shape.Poly {
					if p.X < box.MinX-1e-6 || p.X > box.MaxX+1e-6 || p.Y < box.MinY-1e-6 || p.Y > box.MaxY+1e-6 {
						t.Fatalf("%s: frame %d draws at %.1f,%.1f, outside the box %+v", c.what, i, p.X, p.Y, box)
					}
				}
			}
		}
	}
}

// The hard fling is fitted to the budget rather than cut off by it: its throw is
// scaled down — all of it together, so the explosion still reads as one gesture —
// and it is still the way you were going.
func TestAHardFlingIsFittedToTheBudgetAndStillGoesYourWay(t *testing.T) {
	rig := CreateRig(RigOptions{})
	pose := PoseOf(rig, FullBloom, Motion{})
	centre := Point{X: pose.Head.X, Y: pose.Head.Y}
	pieces := Confetti(rig, centre, 84, rand.New(rand.NewSource(31)))
	burst := NewBurst(pieces, rand.New(rand.NewSource(31)), 1.7, 2600, 2200)
	before := make([]Point, len(burst.Pieces))
	for i := range burst.Pieces {
		before[i] = Point{X: burst.Pieces[i].VX, Y: burst.Pieces[i].VY}
	}
	burst.Flight(burstBudget)
	var wasX, wasY, nowX, nowY float64
	for i := range burst.Pieces {
		wasX += before[i].X
		wasY += before[i].Y
		nowX += burst.Pieces[i].VX
		nowY += burst.Pieces[i].VY
	}
	if wasX <= 0 || wasY <= 0 {
		t.Fatal("the throw did not start going the way the hand was going")
	}
	if nowX <= 0 || nowY <= 0 {
		t.Fatalf("after fitting, the burst goes %.0f,%.0f", nowX, nowY)
	}
	if math.Hypot(nowX, nowY) >= math.Hypot(wasX, wasY) {
		t.Errorf("a throw that had to be fitted was not slowed down: %.0f then %.0f", math.Hypot(wasX, wasY), math.Hypot(nowX, nowY))
	}
}

func reachFrom(b *Burst, box Bounds) float64 {
	dx := math.Max(math.Abs(box.MinX-b.Centre.X), math.Abs(box.MaxX-b.Centre.X))
	dy := math.Max(math.Abs(box.MinY-b.Centre.Y), math.Abs(box.MaxY-b.Centre.Y))
	return math.Hypot(dx, dy)
}

// The burst has to be over: an explosion that leaves one petal hanging in the air
// forever is worse than no explosion, and it is the kind of thing only a test
// ever notices.
func TestBurstFinishes(t *testing.T) {
	pieces := PetalPieces(bloomFrame(t))
	burst := NewBurst(pieces, rand.New(rand.NewSource(7)), 1.4, 0, 0)
	if burst.Done() {
		t.Fatal("a burst that is done before it starts")
	}
	for i := 0; i < 600 && !burst.Done(); i++ {
		burst.Step(1.0 / 60)
	}
	if !burst.Done() {
		t.Fatalf("%d petals still in the air after ten seconds", burst.Left())
	}
	if burst.Elapsed > 3 {
		t.Fatalf("the burst took %.1fs, which is not an explosion", burst.Elapsed)
	}
	if len(burst.Shapes()) != 0 {
		t.Fatal("a finished burst still draws")
	}
}

// Every petal leaves the bloom: they all travel outwards, and gravity has the
// last word, so nothing ends up above where it started.
func TestBurstThrowsPetalsOutAndDown(t *testing.T) {
	pieces := PetalPieces(bloomFrame(t))
	start := make([]Point, len(pieces))
	for i, p := range pieces {
		start[i] = p.At
	}
	burst := NewBurst(pieces, rand.New(rand.NewSource(3)), 1, 0, 0)
	var cx, cy float64
	for _, p := range start {
		cx += p.X
		cy += p.Y
	}
	cx, cy = cx/float64(len(start)), cy/float64(len(start))

	movedOut := 0
	for i := 0; i < 40; i++ {
		burst.Step(1.0 / 60)
	}
	for i := range burst.Pieces {
		p := burst.Pieces[i]
		before := math.Hypot(start[i].X-cx, start[i].Y-cy)
		after := math.Hypot(p.At.X-cx, p.At.Y-cy)
		if after > before+8 {
			movedOut++
		}
	}
	if movedOut < len(burst.Pieces)/2 {
		t.Fatalf("only %d of %d petals left the bloom", movedOut, len(burst.Pieces))
	}
	// The pop lifts them, and then the ground wins: once the burst is over the
	// whole cloud has fallen well below where the bloom was. (A single petal can
	// still be above its own starting point — that is what the lift is for.)
	for i := 0; i < 400 && !burst.Done(); i++ {
		burst.Step(1.0 / 60)
	}
	if !burst.Done() {
		t.Fatal("the burst outlived the gravity check")
	}
	var startY, endY float64
	for i := range burst.Pieces {
		startY += start[i].Y
		endY += burst.Pieces[i].At.Y
	}
	n := float64(len(burst.Pieces))
	if startY-endY < n*60 {
		t.Fatalf("the petals only fell %.0f between them", startY-endY)
	}
}

// A petal in flight is still a petal: same shape, same fill, and its alpha only
// ever fades. The burst draws with the same rasteriser as the flower, so a piece
// that lost its polygon or its gradient would show up as a hole on screen.
func TestBurstPiecesKeepTheirInk(t *testing.T) {
	list := bloomFrame(t)
	pieces := PetalPieces(list)
	burst := NewBurst(pieces, rand.New(rand.NewSource(11)), 1, 0, 0)
	want := len(pieces)
	flashed := 0
	for i := 0; i < 40; i++ {
		burst.Step(1.0 / 60)
		shapes := burst.Shapes()
		// the flash is the first quarter of a second and nothing after it: it is
		// the shockwave, and a shockwave that lingers is a stain
		flying := 0
		for _, shape := range shapes {
			switch shape.Part {
			case "burst":
				flying++
			case "flash":
				flashed++
				if i > 20 {
					t.Errorf("frame %d still draws the flash", i)
				}
			default:
				t.Errorf("the burst drew a %q shape", shape.Part)
			}
		}
		if flying == 0 || flying > want {
			t.Fatalf("frame %d drew %d pieces, expected 1..%d", i, flying, want)
		}
		for _, shape := range shapes {
			if len(shape.Poly) < 3 {
				t.Fatal("a piece lost its polygon")
			}
			if shape.Alpha <= 0 || shape.Alpha > 1 {
				t.Fatalf("alpha is %.3f", shape.Alpha)
			}
			for _, p := range shape.Poly {
				if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
					t.Fatal("a piece is somewhere impossible")
				}
			}
		}
	}
	if flashed < 5 {
		t.Errorf("a burst of 40 frames drew its flash %d times: an explosion needs one", flashed)
	}
}

// The burst is thrown the way the hand was moving, so a flower flung onto a chat
// explodes in the direction it was travelling.
func TestBurstGoesTheWayYouWereGoing(t *testing.T) {
	pieces := PetalPieces(bloomFrame(t))
	burst := NewBurst(pieces, rand.New(rand.NewSource(5)), 1.5, 900, 0)
	var sum float64
	for i := range burst.Pieces {
		sum += burst.Pieces[i].VX
	}
	if sum <= 0 {
		t.Fatalf("a burst thrown to the right went left (sum %.0f)", sum)
	}
	// and harder throws go further than gentle ones
	gentle := NewBurst(PetalPieces(bloomFrame(t)), rand.New(rand.NewSource(5)), 0.6, 0, 0)
	hard := NewBurst(PetalPieces(bloomFrame(t)), rand.New(rand.NewSource(5)), 1.6, 0, 0)
	gentle.Step(0.2)
	hard.Step(0.2)
	if hard.Left() == 0 || gentle.Left() == 0 {
		t.Fatal("a burst lost everything in one step")
	}
	gentleSpread := spread(gentle)
	hardSpread := spread(hard)
	if hardSpread <= gentleSpread {
		t.Fatalf("the harder throw spread %.0f, the gentle one %.0f", hardSpread, gentleSpread)
	}
}

// The explosion carries the drag, which is half of why it reads as the flower
// going off: flick it to the right and the shower goes right, flick it left and it
// goes left, and both go further than a flower let go of standing still. This is
// the moment a person is watching, so it is stated as a property.
func TestTheExplosionCarriesTheDrag(t *testing.T) {
	rig := CreateRig(RigOptions{})
	pose := PoseOf(rig, FullBloom, Motion{})
	centre := Point{X: pose.Head.X, Y: pose.Head.Y}
	drift := func(throwX float64) float64 {
		pieces := Confetti(rig, centre, 132, rand.New(rand.NewSource(9)))
		burst := NewBurst(pieces, rand.New(rand.NewSource(9)), 1.2, throwX, 0)
		burst.Flight(burstBudget)
		start := meanX(burst)
		for i := 0; i < 18; i++ {
			burst.Step(1.0 / 60)
		}
		return meanX(burst) - start
	}
	right, left, still := drift(900), drift(-900), drift(0)
	if right <= still+1 || left >= still-1 {
		t.Errorf("the shower does not go the way the flower was thrown: right %.0f, still %.0f, left %.0f", right, still, left)
	}
	if math.Abs(right-left) < 40 {
		t.Errorf("both throws made the same explosion: right %.0f, left %.0f", right, left)
	}
}

func meanX(b *Burst) float64 {
	if len(b.Pieces) == 0 {
		return 0
	}
	var sum float64
	for i := range b.Pieces {
		sum += b.Pieces[i].At.X
	}
	return sum / float64(len(b.Pieces))
}

func spread(b *Burst) float64 {
	var cx, cy float64
	for i := range b.Pieces {
		cx += b.Pieces[i].At.X
		cy += b.Pieces[i].At.Y
	}
	n := float64(len(b.Pieces))
	cx, cy = cx/n, cy/n
	var sum float64
	for i := range b.Pieces {
		sum += math.Hypot(b.Pieces[i].At.X-cx, b.Pieces[i].At.Y-cy)
	}
	return sum / n
}
