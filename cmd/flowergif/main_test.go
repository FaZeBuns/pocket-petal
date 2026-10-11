package main

import (
	"bytes"
	"image/gif"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/FaZeBuns/pocket-petal/internal/flower"
)

// The tests here are about the two things this tool can get wrong while still writing a
// file: a GIF that is not an animation a page can play, and a picture that cuts the
// flower off at the edge. Both failures look exactly like success from outside.

// blank is a frame of one colour, which is all the encoder's own tests need.
func blank(w, h int, r, g, b uint8) []uint8 {
	out := make([]uint8, w*h*4)
	for i := 0; i+3 < len(out); i += 4 {
		out[i], out[i+1], out[i+2], out[i+3] = r, g, b, 255
	}
	return out
}

// The GIF has to be an animation a page can play: the frames, the delay and the loop are
// the whole interface between this tool and the page the pictures are shown on.
func TestTheAnimationIsAGifAPageCanPlay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.gif")
	const w, h, fps = 24, 16, 20
	frames := [][]uint8{blank(w, h, 210, 90, 140), blank(w, h, 90, 140, 210), blank(w, h, 140, 210, 90)}
	if err := writeGIF(path, frames, w, h, fps, 16); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	anim, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatalf("the file is not a GIF: %v", err)
	}
	if len(anim.Image) != len(frames) {
		t.Errorf("%d frames were written and %d came back", len(frames), len(anim.Image))
	}
	if anim.LoopCount != 0 {
		t.Errorf("the animation does not loop forever: it says %d", anim.LoopCount)
	}
	if want := 100 / fps; anim.Delay[0] != want {
		t.Errorf("a %d fps animation delays by %d hundredths, and it says %d", fps, want, anim.Delay[0])
	}
	if got := anim.Image[0].Bounds().Dx(); got != w {
		t.Errorf("the frame is %d px wide and the picture is %d", got, w)
	}
	if got := anim.Image[0].Bounds().Dy(); got != h {
		t.Errorf("the frame is %d px tall and the picture is %d", got, h)
	}
	// and the frames really are different pictures: an encoder that wrote the same frame
	// forty times would pass everything above
	if bytes.Equal(anim.Image[0].Pix, anim.Image[1].Pix) {
		t.Error("frames 1 and 2 are the same picture, so this is not an animation")
	}
}

// The palette is spent on the colours that are in the picture, and a frame that comes
// back out of it is the same flower: a quantiser that is wrong here shows up as a flower
// with the wrong colour rather than as an error.
func TestTheColoursSurviveThePalette(t *testing.T) {
	const w, h = 64, 64
	frames := make([][]uint8, 4)
	for i := range frames {
		frames[i] = blank(w, h, 22, 19, 30)
		// a pink bloom in the middle, a green stem below it, and a little white
		for y := 8; y < 40; y++ {
			for x := 20; x < 44; x++ {
				p := (y*w + x) * 4
				frames[i][p], frames[i][p+1], frames[i][p+2] = 244, 145, 182
			}
		}
		// a stem wide enough to be a *feature*: the first cut of this fixture drew a two-pixel
		// line, which a 32-colour palette is right to spend nothing on, and the test then
		// failed for being a picture no flower looks like
		for y := 40; y < 60; y++ {
			for x := 28; x < 37; x++ {
				p := (y*w + x) * 4
				frames[i][p], frames[i][p+1], frames[i][p+2] = 104, 165, 96
			}
		}
		for x := 30; x < 34; x++ {
			p := (9*w + x) * 4
			frames[i][p], frames[i][p+1], frames[i][p+2] = 252, 250, 253
		}
	}
	pal := quantize(frames, 32)
	if len(pal) < 4 || len(pal) > 32 {
		t.Fatalf("a palette of at most 32 came back with %d entries", len(pal))
	}
	look := newLookup(pal)
	near := func(r, g, b uint8, tol float64) bool {
		c := pal[look.at(r, g, b)]
		cr, cg, cb, _ := c.RGBA()
		d := math.Abs(float64(cr>>8)-float64(r)) + math.Abs(float64(cg>>8)-float64(g)) + math.Abs(float64(cb>>8)-float64(b))
		return d <= tol
	}
	for _, c := range []struct {
		what    string
		r, g, b uint8
	}{{"the bloom", 244, 145, 182}, {"the stem", 104, 165, 96}, {"the highlight", 252, 250, 253}, {"the backdrop", 22, 19, 30}} {
		if !near(c.r, c.g, c.b, 60) {
			t.Errorf("%s (rgb %d,%d,%d) does not survive the palette", c.what, c.r, c.g, c.b)
		}
	}
}

// A source over an opaque destination is the source, and a half-transparent one is
// halfway: the compositor is the only thing standing between the flower and a black
// rectangle where it should be.
func TestTheFlowerIsCompositedOverTheBackdrop(t *testing.T) {
	dst := blank(2, 1, 22, 19, 30)
	src := blank(2, 1, 255, 255, 255)
	src[3], src[7] = 255, 128
	over(dst, src)
	if dst[0] != 255 || dst[3] != 255 {
		t.Errorf("a solid white pixel came out as %v", dst[0:4])
	}
	half := float64(dst[4])
	if math.Abs(half-((255+22)/2.0)) > 4 {
		t.Errorf("a half-transparent white over the backdrop reads %v, which is not halfway to %v", half, (255+22)/2)
	}
}

// Mixing is how a colourway becomes the next one, so it has to be the two pictures and
// nothing else: 0 is the first flower, 1 is the second, and a half is between them.
func TestOneColourwayBecomesTheNext(t *testing.T) {
	a, b := blank(1, 1, 244, 145, 182), blank(1, 1, 152, 160, 246)
	if got := mix(a, b, 0); got[0] != a[0] || got[1] != a[1] || got[2] != a[2] {
		t.Errorf("mixing nothing in gave %v, not the first flower", got[0:3])
	}
	if got := mix(a, b, 1); got[0] != b[0] || got[1] != b[1] || got[2] != b[2] {
		t.Errorf("mixing all of the second in gave %v, not the second flower", got[0:3])
	}
	mid := mix(a, b, 0.5)
	for c := 0; c < 3; c++ {
		if want := (int(a[c]) + int(b[c])) / 2; math.Abs(float64(int(mid[c])-want)) > 1 {
			t.Errorf("channel %d at halfway is %d and should be %d", c, mid[c], want)
		}
	}
}

// The flower is carried across the picture, which is the one thing a flat backdrop cannot
// show any other way: if this tool panned a camera instead, the drag would be invisible.
func TestTheDragCarriesTheFlowerAcrossThePicture(t *testing.T) {
	tk := buildDrag(3, "blush", 700, false)
	if len(tk.frames) < 10 {
		t.Fatalf("a drag is %d frames long", len(tk.frames))
	}
	first, last := tk.frames[0].offX, tk.frames[len(tk.frames)-1].offX
	if last <= first {
		t.Errorf("the flower started at %.0f and ended at %.0f: it was never carried", first, last)
	}
	// and it is carried while the hand has hold of it and no further: a flower that kept
	// sliding after it was let go would be a flower with no weight
	var moved int
	for _, p := range tk.frames {
		if p.offX > last+0.5 {
			moved++
		}
	}
	if moved > 0 {
		t.Errorf("%d frames carry the flower past where it was let go of", moved)
	}
}

// bounds() is the promise that nothing is cut off, so it is checked the only way it can
// be: every frame's own shapes have to be inside the box the animation is drawn in.
func TestTheFrameHoldsEverythingTheAnimationDraws(t *testing.T) {
	for _, bursty := range []bool{false, true} {
		tk := buildDrag(5, "blush", 700, bursty)
		box := tk.bounds()
		for i, p := range tk.frames {
			inner := flower.ListBounds(tk.shapes(p, p.rig))
			if inner.MinX < box.MinX || inner.MinY < box.MinY || inner.MaxX > box.MaxX || inner.MaxY > box.MaxY {
				t.Fatalf("frame %d of %d draws %v, outside the box %v", i, len(tk.frames), inner, box)
			}
		}
	}
	// and the burst really does put petals outside the flower's own box, which is why the
	// two animations are framed differently
	quiet, loud := buildDrag(5, "blush", 700, false), buildDrag(5, "blush", 700, true)
	if loud.bounds().Width() <= quiet.bounds().Width() {
		t.Errorf("the explosion is %.0f units wide and the drag alone is %.0f: the petals are not in the picture",
			loud.bounds().Width(), quiet.bounds().Width())
	}
}
