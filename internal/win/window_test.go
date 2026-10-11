package win

import (
	"testing"
)

// A layered window can be much bigger than the picture it is showing: the caller
// writes a region into one corner of its bitmap and Windows composites exactly
// that region, at that place. The petals' surface depends on it — a burst is flown
// in a window as large as its whole flight and drawn a piece at a time — so the
// mechanism is checked here rather than trusted.
//
// It was checked on the machine this was written on by reading the screen back:
// with a 40x40 region of pure blue written at 100,100 of a 512x512 surface and
// blitted to 300,200, screenshots at 320,220 and 310,210 read 0,0,255 and the
// pixels just outside the window read the desktop. Which is the whole point:
// Windows copies from pptSrc, not from the corner of the bitmap.
func TestOnlyTheRegionOfABitmapThatIsBlittedIsShown(t *testing.T) {
	w, err := NewNamedLayeredWindow(256, 256, "PocketPetalRegionTestWindow", func(MSG) bool { return false })
	if err != nil {
		t.Skipf("no layered windows on this machine: %v", err)
	}
	defer w.Destroy()

	red := solidRed(32, 32)
	// off the bottom-right of the screen, so the suite does not flash a red square
	// at anybody: the geometry is the same either way
	if err := w.BlitRegionAt(-4000, -4000, 64, 64, red, 32, 32); err != nil {
		t.Fatalf("blitting a region: %v", err)
	}
	if r := w.Rect(); r.Width() != 32 || r.Height() != 32 || int(r.Left) != -4000 || int(r.Top) != -4000 {
		t.Fatalf("the window is %d,%d %dx%d and should be the region", r.Left, r.Top, r.Width(), r.Height())
	}
	// and the picture landed in the bitmap where it was asked to
	if got := pixelAt(w, 64, 64); got != [4]uint8{0, 0, 255, 255} {
		t.Errorf("the region's first pixel is %v, and it should be premultiplied red", got)
	}
	if got := pixelAt(w, 0, 0); got != [4]uint8{0, 0, 0, 0} {
		t.Errorf("the corner of the surface was written to: %v", got)
	}
	if got := pixelAt(w, 128, 128); got != [4]uint8{0, 0, 0, 0} {
		t.Errorf("a pixel outside the region was written to: %v", got)
	}

	// a different region of the same surface is a different window: this is what
	// lets one surface hold a whole flight and show a frame at a time
	if err := w.BlitRegionAt(-3100, -2900, 0, 0, solidRed(48, 24), 48, 24); err != nil {
		t.Fatalf("blitting a second region: %v", err)
	}
	if r := w.Rect(); r.Width() != 48 || r.Height() != 24 || int(r.Left) != -3100 || int(r.Top) != -2900 {
		t.Fatalf("the window is %d,%d %dx%d and should have followed the region", r.Left, r.Top, r.Width(), r.Height())
	}

	// and a region that would run off the end of the bitmap is refused rather than
	// written past it
	if err := w.BlitRegionAt(0, 0, 240, 240, red, 32, 32); err == nil {
		t.Error("a region outside the surface should be refused")
	}
	if err := w.BlitRegionAt(0, 0, 0, 0, red, 0, 10); err == nil {
		t.Error("an empty region should be refused")
	}
}

// Moving a window is the cheap half of a blit, and it is what lets the settings
// panel be dragged around: pressing the plate hands Windows a new position instead
// of re-premultiplying a hundred thousand pixels that have not changed. Moving a
// *layered popup* has to work for that — it is the one window in the app that takes
// the focus — and the picture inside it has to survive the move, because the next
// thing the panel does is draw on top of it.
func TestMovingALayeredWindowKeepsItsPicture(t *testing.T) {
	w, err := NewPopupWindow(128, 96, "PocketPetalMoveTestWindow", func(MSG) bool { return false })
	if err != nil {
		t.Skipf("no layered windows on this machine: %v", err)
	}
	defer w.Destroy()

	// off the screen, so the suite does not flash a red rectangle at anybody
	if err := w.BlitAt(-4000, -4000, solidRed(128, 96), 128, 96); err != nil {
		t.Fatalf("blitting: %v", err)
	}
	if got := pixelAt(w, 10, 10); got != [4]uint8{0, 0, 255, 255} {
		t.Fatalf("the picture did not land where it was put: %v", got)
	}

	w.MoveTo(-3200, -2800)
	x, y := w.Position()
	if x != -3200 || y != -2800 {
		t.Fatalf("the window is at %d,%d after being moved to -3200,-2800", x, y)
	}
	// and the move was a move: the size is the same and so is everything in it
	if r := w.Rect(); r.Width() != 128 || r.Height() != 96 {
		t.Fatalf("moving it resized it: %dx%d", r.Width(), r.Height())
	}
	if got := pixelAt(w, 10, 10); got != [4]uint8{0, 0, 255, 255} {
		t.Errorf("moving it changed its picture: %v", got)
	}
}

// solidRed is a picture of a given size, opaque red: enough to tell the pixels
// that were written from the ones that were not.
func solidRed(w, h int) []uint8 {
	out := make([]uint8, w*h*4)
	for i := 0; i < len(out); i += 4 {
		out[i], out[i+3] = 255, 255
	}
	return out
}

func pixelAt(w *Window, x, y int) [4]uint8 {
	px := w.Pixels()
	i := (y*w.W + x) * 4
	if i < 0 || i+4 > len(px) {
		return [4]uint8{}
	}
	return [4]uint8{px[i], px[i+1], px[i+2], px[i+3]}
}
