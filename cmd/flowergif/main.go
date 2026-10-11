// Command flowergif renders the flower's animations as the pictures the README shows:
// the drag and the burst, the bloom unfurling, and the four colourways.
//
//	tools/flowergifs.sh                              # every one of them, into assets/
//	go run ./cmd/flowergif -anim drag    -out assets/gifs/drag.gif
//	go run ./cmd/flowergif -anim burst   -out assets/gifs/burst.gif
//	go run ./cmd/flowergif -anim bloom   -out assets/gifs/bloom.gif
//	go run ./cmd/flowergif -anim colours -out assets/gifs/colours.gif
//	go run ./cmd/flowergif -anim hero    -out assets/pocket-petal-hero.png
//	go run ./cmd/flowergif -anim drag -strip 6 -out assets/story-drag.png
//
// Nothing here is a mock-up and nothing is a screen recording: the frames come out of
// the same rig, the same carry model, the same burst and the same rasteriser the app
// runs on the desk, driven by the same numbers. A burst takes its power from how fast
// the flower was travelling and its direction from where the hand was going (see
// Buddy.startBurst), the drag filmstrip uses the engine's own step so the whip and the
// settle are the app's, and a frame is laid out by the app's own framing rules. The one
// thing this tool owns is the camera: a fixed box for the whole animation, because a
// flower that jumps around a GIF is a flower nobody can look at.
//
// It is deliberately the *pictures* command and not part of the app: the exe has no
// business carrying a GIF encoder, and a tool that renders what the app would draw is
// the only kind of screenshot this project is willing to publish.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"

	"github.com/FaZeBuns/pocket-petal/internal/flower"
)

// backdrop is the colour every animation in the repo is drawn on, and it is a colour
// rather than a transparent background for a reason: a GIF has one bit of transparency
// and no anti-aliasing to go with it, so a flower on "nothing" comes out with a jagged
// edge. The dark grey is what the app looks like on a desktop at night, which is when a
// glowing flower is worth having.
var backdrop = flower.RGB{22, 19, 30}

// engineStep is the step the app runs its physics at, so an animation driven by this
// tool has the same whip, the same settle and the same recoil as the one on the desk.
const engineStep = 1.0 / 120

// picture is one frame: the flower, as the carry has left it, and whatever else is in
// the air beside it.
//
// Two rigs rather than one, because the colourways are shown by fading one into the next
// and a crossfade is only honest if both flowers are drawn: blending the *pixels* of two
// real frames mixes the colours the way the eye would, while drawing one palette over
// the other would mix in the backdrop and darken the flower on the way past.
type picture struct {
	rig    *flower.Rig
	other  *flower.Rig
	blend  float64 // 0 = all of rig, 1 = all of other
	t      float64 // rig time: how far the bloom has opened
	motion flower.Motion
	shine  float64 // where the sun is in its pass, 0..1 (0 draws nothing)
	burst  *flower.Burst
	// offX/offY is where the hand has carried the flower to, in design units. A drag
	// moves the *window* on the desk rather than changing the flower's pose, so it is
	// this and not the maths that makes a carried flower cross the picture.
	offX, offY float64
}

// take is one animation: the frames, and how they are put on the page. The size is fixed
// for the whole animation — a camera that moved would be a camera you cannot watch — and
// with auto set the width is worked out from the box the animation actually draws in, so
// no frame is ever padded with a band of nothing down one side.
type take struct {
	anim   string
	width  int
	height int
	fps    int
	auto   bool
	// clear leaves the picture transparent instead of painting the backdrop, which is
	// only for a still: a GIF has one bit of transparency and no antialiasing to go with
	// it, but a PNG is allowed to be a flower and nothing else — one picture that works
	// on a white page and on a dark one.
	clear  bool
	frames []picture
}

func main() {
	anim := flag.String("anim", "drag", "drag | burst | bloom | colours | hero")
	out := flag.String("out", "", "file to write (default assets/<anim>.gif, or .png for hero)")
	seed := flag.Int("seed", 11, "which flower to grow")
	palette := flag.String("palette", "blush", "the colourway, for the animations that are not about colour")
	speed := flag.Float64("speed", 700, "how fast the hand carries the flower, for the drag: design units a second")
	width := flag.Int("width", 0, "frame width in pixels (0 = the animation's own)")
	height := flag.Int("height", 0, "frame height in pixels (0 = the animation's own)")
	fps := flag.Int("fps", 0, "frames a second (0 = the animation's own)")
	colours := flag.Int("colours", 200, "how many colours a GIF is allowed")
	shot := flag.Bool("still", false, "write one PNG instead of an animation")
	strip := flag.Int("strip", 0, "write this many evenly spaced frames side by side as one PNG (a storyboard), instead of an animation")
	quiet := flag.Bool("q", false, "print nothing but the file")
	flag.Parse()

	t := build(*anim, *seed, *palette, *speed)
	if t == nil {
		fail(fmt.Errorf("no such animation %q: drag, burst, bloom, colours or hero", *anim))
	}
	if *width > 0 {
		t.width, t.auto = *width, false
	}
	if *height > 0 {
		t.height = *height
	}
	if *fps > 0 {
		t.fps = *fps
	}

	path := *out
	if path == "" {
		if *anim == "hero" {
			path = filepath.Join("assets", "pocket-petal-hero.png")
		} else {
			path = filepath.Join("assets", "gifs", *anim+".gif")
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fail(err)
	}

	// one box for the whole animation, measured before anything is drawn: the union of
	// every frame's flower and every frame's flying petals, so a petal can never be cut
	// off by the edge of the picture it is in
	box := t.bounds()
	if t.auto && box.Height() > 0 {
		t.width = int(math.Round(float64(t.height) * box.Width() / box.Height()))
	}
	if !*quiet {
		fmt.Printf("%s: %d frames at %d fps, box %.0fx%.0f units -> %dx%d px\n",
			t.anim, len(t.frames), t.fps, box.Width(), box.Height(), t.width, t.height)
	}

	if *shot || *anim == "hero" {
		pix := t.frame(t.frames[0], box)
		if err := writePNG(path, pix, t.width, t.height); err != nil {
			fail(err)
		}
		report(path, 1)
		return
	}
	if *strip > 0 {
		if err := writeStrip(path, t, box, *strip); err != nil {
			fail(err)
		}
		report(path, *strip)
		return
	}

	frames := make([][]uint8, len(t.frames))
	for i, p := range t.frames {
		frames[i] = t.frame(p, box)
	}
	if err := writeGIF(path, frames, t.width, t.height, t.fps, *colours); err != nil {
		fail(err)
	}
	report(path, len(frames))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "flowergif:", err)
	os.Exit(1)
}

func report(path string, frames int) {
	info, err := os.Stat(path)
	if err != nil {
		fail(err)
	}
	fmt.Printf("%s: %d frames, %.0f KB\n", path, frames, float64(info.Size())/1024)
}

// build is the animation asked for. Every one of them starts from the same rig and the
// same engine: a GIF that needed special drawing code would be a lie about the app.
func build(anim string, seed int, palette string, speed float64) *take {
	switch anim {
	case "drag":
		return buildDrag(seed, palette, speed, false)
	case "burst":
		return buildDrag(seed, palette, speed, true)
	case "bloom":
		return buildBloom(seed, palette)
	case "colours":
		return buildColours(seed)
	case "hero":
		return buildHero(seed, palette)
	}
	return nil
}

// buildDrag is the app in one loop: the flower is picked up, carried off to the right,
// and let go of. It is built twice from the same run — once framed on the flower, where
// the lean, the whip and the settle are the subject, and once framed on the whole flight
// of the petals, where the explosion is.
//
// The burst, when it happens, is the app's own: the same piece count, the same power
// from the same speed, the same recoil on the flower's neck, and the same reach budget
// the surface is clamped to (see Buddy.startBurst and Buddy.throwPetals).
func buildDrag(seed int, palette string, speed float64, bursty bool) *take {
	name := "drag"
	if bursty {
		name = "burst"
	}
	rig := flower.CreateRig(flower.RigOptions{Seed: &seed, Palette: palette})
	// a drag is framed tight on the flower, so the frame is nearly square; an explosion
	// has petals in the air a long way from the bloom, so its picture is taller and the
	// flower is a smaller part of it. Neither is padded: the width comes from the box.
	t := &take{anim: name, height: 470, fps: 20, auto: true}
	if bursty {
		t.height = 620
	}

	const (
		pickUp  = 0.30 // the hand gets a hold of it here
		carried = 0.30 // and carries it this long, so the travel is the speed times this
		dropAt  = pickUp + carried
		settle  = 0.12 // the hand comes to rest this long after the drop
		seconds = 2.30 // long enough for the last petal to have faded
	)
	// the hand: still, then a walking pace to the right, then stopped dead. The stop just
	// after the drop is what swings the head forward and settles it — the flower
	// answering its own explosion (see Buddy.startBurst).
	hand := []struct{ at, vx, ax float64 }{
		{0, 0, 0},
		{pickUp, speed, 6500},
		{pickUp + 0.22, speed, 400},
		{dropAt, speed, 0},
		{dropAt + settle, 0, -9000},
		{seconds + 1, 0, 0},
	}
	carry := flower.CreateCarry(flower.PhysicsOptions{})
	rng := rand.New(rand.NewSource(int64(seed)))
	var burst *flower.Burst
	// where the hand has carried it to: the app moves a window, so this is the flower
	// crossing the desk and not the flower leaning
	atX := 0.0

	next := 0.0
	for at := 0.0; at < seconds; at += engineStep {
		vx, ax := handAt(hand, at)
		dragged := at >= pickUp && at < dropAt
		carry.Step(engineStep, flower.Hand{VX: vx, AX: ax, Dragging: dragged})
		if dragged {
			atX += vx * engineStep
		}

		if bursty && at >= dropAt && burst == nil {
			// exactly what the app does when a flower is let go of, in the same order and
			// from the same numbers: the speed decides how hard it goes, the flower takes
			// the recoil, and the petals keep the hand's own momentum
			state := carry.State()
			move := math.Hypot(state.Fvx, state.Fvy)
			power := 0.8 + 0.9*math.Min(1, move/1900)
			carry.Impact(0.8)
			carry.Flick(-state.Fvx*0.35, -state.Fvy*0.35, 1.0)
			pose := flower.PoseOf(rig, flower.FullBloom, carry.Motion())
			pieces := flower.Confetti(rig, flower.Point{X: pose.Head.X, Y: pose.Head.Y}, burstPetals, rng)
			burst = flower.NewBurst(pieces, rng, power, state.Fvx, state.Fvy)
			burst.Flight(burstReach) // the clamp the app applies, so the throw is the app's
		}
		if burst != nil {
			burst.Step(engineStep)
		}
		if at+1e-9 >= next {
			t.frames = append(t.frames, picture{
				rig: rig, t: flower.FullBloom, motion: carry.Motion(),
				burst: snapshot(burst), offX: atX,
			})
			next += 1.0 / float64(t.fps)
		}
	}
	return t
}

// buildBloom is the flower growing: the bloom unfurling on its golden-angle spiral, with
// the sun passing over it near the end. Real time — six seconds is how long the flower
// takes to open, and a sped-up unfurl would be a different flower.
func buildBloom(seed int, palette string) *take {
	rig := flower.CreateRig(flower.RigOptions{Seed: &seed, Palette: palette})
	t := &take{anim: "bloom", height: 400, fps: 10, auto: true}
	const seconds = flower.FullBloom
	frames := int(seconds * float64(t.fps))
	for i := 0; i < frames; i++ {
		at := float64(i) / float64(frames-1)
		// the light crosses in the last third, once the bloom is out and there is
		// something for it to catch on
		shine := 0.0
		if at > 0.62 {
			shine = (at - 0.62) / 0.38
		}
		t.frames = append(t.frames, picture{rig: rig, t: seconds * at, motion: flower.NoMotion, shine: shine})
	}
	return t
}

// buildColours is the same flower in each of the four colourways, held and then faded
// into the next: the setting the panel offers, shown the way the panel shows it — a
// swatch and a name — rather than described.
func buildColours(seed int) *take {
	t := &take{anim: "colours", height: 430, fps: 20, auto: true}
	const (
		hold = 8 // frames a colourway is held
		fade = 6 // and the frames it takes to become the next one
	)
	rigs := make([]*flower.Rig, 0, len(flower.PaletteNames))
	for _, name := range flower.PaletteNames {
		rigs = append(rigs, flower.CreateRig(flower.RigOptions{Seed: &seed, Palette: name}))
	}
	for i, rig := range rigs {
		next := rigs[(i+1)%len(rigs)]
		for f := 0; f < hold+fade; f++ {
			p := picture{rig: rig, other: next, t: flower.FullBloom, motion: flower.NoMotion}
			if f >= hold {
				p.blend = float64(f-hold+1) / float64(fade)
			}
			t.frames = append(t.frames, p)
		}
	}
	return t
}

// buildHero is one flower at its best, big, with the ground shadow and nothing else:
// the picture for the top of the repository, so that the first thing on the page is the
// thing itself rather than a 150px icon of it.
func buildHero(seed int, palette string) *take {
	rig := flower.CreateRig(flower.RigOptions{Seed: &seed, Palette: palette})
	t := &take{anim: "hero", height: 1040, fps: 1, auto: true, clear: true}
	t.frames = append(t.frames, picture{rig: rig, t: flower.FullBloom, motion: flower.NoMotion})
	return t
}

// handAt is the hand's velocity and acceleration at a moment: the samples are a line,
// and between two of them the hand is doing what it was doing.
func handAt(path []struct{ at, vx, ax float64 }, t float64) (vx, ax float64) {
	for i, h := range path {
		if t <= h.at || i == len(path)-1 {
			return h.vx, h.ax
		}
	}
	return 0, 0
}

// snapshot copies a burst so that a frame keeps the petals where they were: the live one
// is a single object stepped over and over, and every frame held a pointer to it would
// be a picture of the *end* of the explosion.
func snapshot(b *flower.Burst) *flower.Burst {
	if b == nil {
		return nil
	}
	c := *b
	c.Pieces = append([]flower.PetalPiece(nil), b.Pieces...)
	return &c
}

// bounds is the box the whole animation is drawn in: every frame's flower and every
// frame's petals, unioned. It is measured rather than guessed because the alternative is
// a petal sliced off square by the edge of its own picture, which is exactly the bug the
// app's burst surface exists to avoid (see Burst.Bounds).
func (t *take) bounds() flower.Bounds {
	var box flower.Bounds
	first := true
	add := func(b flower.Bounds) {
		if first {
			box, first = b, false
			return
		}
		box = flower.Bounds{
			MinX: math.Min(box.MinX, b.MinX), MinY: math.Min(box.MinY, b.MinY),
			MaxX: math.Max(box.MaxX, b.MaxX), MaxY: math.Max(box.MaxY, b.MaxY),
		}
	}
	for _, p := range t.frames {
		add(flower.ListBounds(t.shapes(p, p.rig)))
		if p.other != nil {
			add(flower.ListBounds(t.shapes(p, p.other)))
		}
	}
	// a little air, so the shadow and the outermost petal are not touching the frame
	return flower.ScaleBox(box, 1.05)
}

// shapes is one flower, plus the petals in the air beside it.
//
// The petals go in *first*, because they are drawn behind the flower on the desk too: a
// petal should look like it came off the bloom and is falling past it, not like a
// sticker laid over the top (see Buddy.drawPetals).
func (t *take) shapes(p picture, rig *flower.Rig) []flower.Shape {
	var list []flower.Shape
	if p.burst != nil {
		list = append(list, p.burst.Shapes()...)
	}
	pose := flower.PoseOf(rig, p.t, p.motion)
	list = append(list, flower.BuildDrawList(rig, pose, flower.DrawOptions{Detail: "full", Shadow: true})...)
	// the petals and the flower are moved together: the burst came out of the bloom, so
	// carrying the flower carries the pieces it is shedding
	return translate(list, p.offX, p.offY)
}

// translate moves a drawn list across the design space.
//
// A drag in the app is a window being moved: the flower's pose never changes, and what
// makes the flower cross the desk is where its window is. So this is the whole of "the
// hand carried it": moving the polygons rather than changing the maths is not a
// shortcut, it is the same thing the app does. Panning the camera instead would show
// nothing at all, because the backdrop is one flat colour and there is no parallax in a
// colour.
func translate(list []flower.Shape, dx, dy float64) []flower.Shape {
	if dx == 0 && dy == 0 {
		return list
	}
	out := make([]flower.Shape, len(list))
	for i, s := range list {
		poly := make([]flower.Point, len(s.Poly))
		for j, p := range s.Poly {
			poly[j] = flower.Point{X: p.X + dx, Y: p.Y + dy}
		}
		s.Poly = poly
		out[i] = s
	}
	return out
}

// raster draws one picture into a straight RGBA buffer with nothing behind it: the
// composite happens afterwards, in one place, so a crossfade is a mix of two flowers and
// not a mix of a flower and the backdrop.
func (t *take) raster(p picture, box flower.Bounds, rig *flower.Rig) []uint8 {
	return flower.Rasterize(t.shapes(p, rig), flower.RasterOptions{
		Width: t.width, Height: t.height, Bounds: box, SubSamples: 2,
	})
}

// frame is one finished picture: the backdrop, the flower (or two of them fading), and
// the sun.
func (t *take) frame(p picture, box flower.Bounds) []uint8 {
	pix := t.raster(p, box, p.rig)
	if p.other != nil && p.blend > 0 {
		pix = mix(pix, t.raster(p, box, p.other), p.blend)
	}
	out := make([]uint8, t.width*t.height*4)
	if !t.clear {
		for i := 0; i+3 < len(out); i += 4 {
			out[i], out[i+1], out[i+2], out[i+3] = uint8(backdrop[0]), uint8(backdrop[1]), uint8(backdrop[2]), 255
		}
	}
	over(out, pix)
	// the light is added into what the flower has already painted, and weighted by the
	// flower's own alpha, so it can only ever brighten flower (see flower.ApplyShine)
	flower.ApplyShine(out, t.width, t.height, p.shine)
	return out
}

// mix blends two straight-RGBA frames, alpha and all. For a colourway being faded into
// the next one the two pictures are the same flower, so this is exactly "the same flower
// in a colour between the two" rather than a composite with a seam in it.
func mix(a, b []uint8, k float64) []uint8 {
	out := make([]uint8, len(a))
	for i := 0; i+3 < len(a) && i+3 < len(b); i += 4 {
		for c := 0; c < 4; c++ {
			out[i+c] = clamp8(float64(a[i+c])*(1-k) + float64(b[i+c])*k)
		}
	}
	return out
}

// over lays a straight-RGBA frame onto an opaque one, source over destination.
func over(dst, src []uint8) {
	for i := 0; i+3 < len(dst) && i+3 < len(src); i += 4 {
		a := float64(src[i+3]) / 255
		if a <= 0 {
			continue
		}
		da := float64(dst[i+3]) / 255
		outA := a + da*(1-a)
		if outA <= 0 {
			continue
		}
		for c := 0; c < 3; c++ {
			dst[i+c] = clamp8((float64(src[i+c])*a + float64(dst[i+c])*da*(1-a)) / outA)
		}
		dst[i+3] = clamp8(outA * 255)
	}
}

func clamp8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// writePNG writes one frame, keeping the alpha: a still is allowed to be transparent
// because a PNG can be, and a hero image that shows the flower on the page's own colour
// is one picture that works in both of GitHub's themes.
func writePNG(path string, pix []uint8, width, height int) error {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	img.Pix = pix[:width*height*4]
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// writeStrip lays evenly spaced frames of an animation side by side in one PNG: the
// storyboard. It is how a film is checked without playing it — and it is the one kind of
// screenshot that is honest here, because it is drawn from the same frames the GIF is.
func writeStrip(path string, t *take, box flower.Bounds, count int) error {
	if count < 2 {
		count = 2
	}
	// half size: a strip is read at a glance, and the whole point of it is that all of the
	// gesture is in one picture
	const step = 2
	sw, sh := t.width/step, t.height/step
	out := make([]uint8, sw*count*sh*4)
	for i := 0; i < count; i++ {
		at := int(float64(i) * float64(len(t.frames)) / float64(count))
		if at >= len(t.frames) {
			at = len(t.frames) - 1
		}
		pix := t.frame(t.frames[at], box)
		for y := 0; y < sh; y++ {
			for x := 0; x < sw; x++ {
				r, g, b, a := 0.0, 0.0, 0.0, 0.0
				for dy := 0; dy < step; dy++ {
					for dx := 0; dx < step; dx++ {
						p := ((y*step+dy)*t.width + x*step + dx) * 4
						r, g, b, a = r+float64(pix[p]), g+float64(pix[p+1]), b+float64(pix[p+2]), a+float64(pix[p+3])
					}
				}
				n := float64(step * step)
				at := (y*(sw*count) + i*sw + x) * 4
				out[at], out[at+1], out[at+2], out[at+3] = clamp8(r/n), clamp8(g/n), clamp8(b/n), clamp8(a/n)
			}
		}
	}
	return writePNG(path, out, sw*count, sh)
}

// writeGIF writes the animation.
//
// The palette is worked out from the frames themselves rather than taken off a shelf: the
// flower is pastel, the backdrop is nearly black, and a fixed 256-colour table spends
// most of its entries on colours that are not in the picture. Then each frame goes
// through the same palette, so the difference between two frames is the flower and not a
// different set of colours (which is what makes a GIF shimmer).
func writeGIF(path string, frames [][]uint8, width, height, fps, colours int) error {
	if len(frames) == 0 {
		return fmt.Errorf("no frames to write")
	}
	if colours < 2 {
		colours = 2
	}
	if colours > 256 {
		colours = 256
	}
	pal := quantize(frames, colours)
	look := newLookup(pal)

	delay := int(math.Round(100 / float64(fps)))
	if delay < 2 { // browsers treat 0 and 1 as "as fast as possible", which looks broken
		delay = 2
	}
	out := &gif.GIF{LoopCount: 0, Config: image.Config{ColorModel: pal, Width: width, Height: height}}
	for _, f := range frames {
		img := image.NewPaletted(image.Rect(0, 0, width, height), pal)
		for i := 0; i < width*height; i++ {
			img.Pix[i] = look.at(f[i*4], f[i*4+1], f[i*4+2])
		}
		out.Image = append(out.Image, img)
		out.Delay = append(out.Delay, delay)
		// nothing is disposed of between frames: every frame is the whole picture, so a
		// player that skips one still shows a flower rather than a mess
		out.Disposal = append(out.Disposal, gif.DisposalNone)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return gif.EncodeAll(f, out)
}

// lookup finds the palette entry nearest a colour, with a small cache in front of it: a
// frame is a few hundred thousand pixels and the palette has two hundred entries, so the
// same handful of colours are asked for again and again. The cache is keyed on the top
// six bits of each channel, which is finer than any difference the eye can find in a
// picture this size.
type lookup struct {
	pal   color.Palette
	cache map[uint32]uint8
}

func newLookup(pal color.Palette) *lookup {
	return &lookup{pal: pal, cache: make(map[uint32]uint8, 4096)}
}

func (l *lookup) at(r, g, b uint8) uint8 {
	key := uint32(r>>2)<<12 | uint32(g>>2)<<6 | uint32(b>>2)
	if i, ok := l.cache[key]; ok {
		return i
	}
	best, bestD := 0, math.MaxFloat64
	for i, c := range l.pal {
		cr, cg, cb, _ := c.RGBA()
		dr := float64(cr>>8) - float64(r)
		dg := float64(cg>>8) - float64(g)
		db := float64(cb>>8) - float64(b)
		// weighted the way the eye weights them, so a miss in green costs more than a
		// miss in blue
		d := 0.30*dr*dr + 0.59*dg*dg + 0.11*db*db
		if d < bestD {
			best, bestD = i, d
		}
	}
	i := uint8(best)
	if len(l.pal) > 256 {
		i = 255
	}
	l.cache[key] = i
	return i
}

// quantize is a median cut over the animation's own pixels: split the cloud of colours
// in half along whichever channel it is widest in, keep splitting the widest box, and
// take the average of each box that is left. It is the plainest way to spend a palette
// on the colours that are actually there, and at this size it is instant.
func quantize(frames [][]uint8, want int) color.Palette {
	// every fourth pixel, and only where something is drawn: the backdrop is one colour
	// and would otherwise win half the palette
	var all []rgb
	for _, f := range frames {
		for i := 0; i+3 < len(f); i += 16 {
			if f[i+3] == 0 {
				continue
			}
			all = append(all, rgb{float64(f[i]), float64(f[i+1]), float64(f[i+2])})
		}
	}
	if len(all) == 0 {
		all = append(all, rgb{})
	}
	boxes := [][]rgb{all}
	for len(boxes) < want {
		best, widest := -1, 0.0
		for i, box := range boxes {
			if len(box) < 2 {
				continue
			}
			if w := width(box); w > widest {
				best, widest = i, w
			}
		}
		if best < 0 {
			break
		}
		lo, hi := cut(boxes[best])
		next := make([][]rgb, 0, len(boxes)+1)
		next = append(next, boxes[:best]...)
		next = append(next, lo, hi)
		next = append(next, boxes[best+1:]...)
		boxes = next
	}
	pal := make(color.Palette, 0, len(boxes))
	for _, box := range boxes {
		var r, g, b float64
		for _, c := range box {
			r, g, b = r+c.r, g+c.g, b+c.b
		}
		n := float64(len(box))
		pal = append(pal, color.RGBA{clamp8(r / n), clamp8(g / n), clamp8(b / n), 255})
	}
	if len(pal) == 0 {
		pal = append(pal, color.RGBA{0, 0, 0, 255})
	}
	return pal
}

// rgb is one colour in the cloud the palette is cut out of.
type rgb struct{ r, g, b float64 }

// width is how far apart the colours in a box are, in the channel they are furthest
// apart in: the number that says "this box has something in it worth splitting". A box
// one colour wide is not worth a palette entry of its own until every other box is
// narrower than it.
func width(box []rgb) float64 {
	lo := [3]float64{math.MaxFloat64, math.MaxFloat64, math.MaxFloat64}
	hi := [3]float64{-math.MaxFloat64, -math.MaxFloat64, -math.MaxFloat64}
	for _, c := range box {
		v := [3]float64{c.r, c.g, c.b}
		for i := 0; i < 3; i++ {
			lo[i], hi[i] = math.Min(lo[i], v[i]), math.Max(hi[i], v[i])
		}
	}
	return math.Max(hi[0]-lo[0], math.Max(hi[1]-lo[1], hi[2]-lo[2]))
}

// cut splits a box in half along its widest channel, at the median: the two halves then
// hold the same number of colours, so a palette entry never ends up representing a
// handful of outliers while a crowded part of the picture shares one.
func cut(box []rgb) ([]rgb, []rgb) {
	spread := [3]float64{}
	for i := 0; i < 3; i++ {
		lo, hi := math.MaxFloat64, -math.MaxFloat64
		for _, c := range box {
			v := [3]float64{c.r, c.g, c.b}[i]
			lo, hi = math.Min(lo, v), math.Max(hi, v)
		}
		spread[i] = hi - lo
	}
	channel := 0
	if spread[1] > spread[channel] {
		channel = 1
	}
	if spread[2] > spread[channel] {
		channel = 2
	}
	sorted := append([]rgb(nil), box...)
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		switch channel {
		case 0:
			return a.r < b.r
		case 1:
			return a.g < b.g
		}
		return a.b < b.b
	})
	mid := len(sorted) / 2
	return sorted[:mid], sorted[mid:]
}

// The app's own numbers for the burst, so the petals in a GIF are the petals on the desk:
// how many a delivery throws, and how far from the middle of the bloom the throw is ever
// allowed to reach (see Buddy.throwPetals).
const (
	burstPetals = 132
	burstReach  = 1400.0
)
