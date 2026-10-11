package buddy

import (
	"testing"
	"time"

	"github.com/FaZeBuns/pocket-petal/internal/win"
)

// The ring is the only thing that costs anything while a drag is in flight, and
// over a maximised chat its bitmap is eight megabytes: painting that, and
// compositing it, sixty times a second was the stutter that showed up exactly when
// the flower came over something worth dropping it on. So the rule is measured
// rather than assumed — a frame that changes nothing must do nothing.
func TestTheRingCostsNothingWhileTheTargetHoldsStill(t *testing.T) {
	h, err := newHighlighter(64, 64)
	if err != nil {
		t.Skipf("no layered windows on this machine: %v", err)
	}
	defer h.win.Destroy()

	rect := win.RECT{Left: 8, Top: 8, Right: 8 + 200, Bottom: 8 + 120}
	h.target(rect)
	if len(h.pix) == 0 {
		t.Fatal("the ring painted nothing at all")
	}
	first := &h.pix[0]

	started := time.Now()
	for i := 0; i < 60; i++ {
		h.target(rect)
	}
	elapsed := time.Since(started)

	if &h.pix[0] != first {
		t.Fatal("the ring's bitmap was rebuilt under a target that had not moved")
	}
	// one frame of a rebuilt ring is milliseconds of arithmetic and megabytes of
	// compositing; sixty frames of doing nothing is not
	if elapsed > 20*time.Millisecond {
		t.Errorf("sixty frames of an unmoved target cost %v, which is not nothing", elapsed)
	}

	// and when the target does change, the ring follows it: a different window is
	// a different picture, and the ring must not keep the old one
	moved := win.RECT{Left: 100, Top: 100, Right: 100 + 300, Bottom: 100 + 200}
	h.target(moved)
	if h.rect != moved {
		t.Errorf("the ring is still around %+v", h.rect)
	}
	if &h.pix[0] == first {
		t.Error("a differently-sized target kept the old bitmap")
	}
}

// BenchmarkRingRGBA is the cost the drag used to pay every frame. It is kept so
// that "the ring is cached" stays a measurable claim rather than a comment.
func BenchmarkRingRGBA(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ringRGBA(1936, 1056)
	}
}
