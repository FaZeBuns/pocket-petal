package buddy

import (
	"math"
	"time"

	"github.com/FaZeBuns/pocket-petal/internal/flower"
)

// The weather.
//
// Two things make the flower on a desktop feel alive rather than pasted on, and
// neither of them is a rectangle of glow: the wind it hangs in when nobody is
// holding it, and the light that visits it every so often. Both are timed from
// the frame's own clock, so they keep going whether the flower is being carried,
// walked home or standing in its corner.

const (
	// One pass of sunlight: long enough to watch, short enough to stay an event.
	shinePass = 1500 * time.Millisecond
	// How often the sun comes round, plus a random slack so it never becomes a
	// metronome.
	shineEvery  = 13 * time.Second
	shineJitter = 9 * time.Second
	// A pick-up or a landing gets its own quick pass, as an answer to the hand.
	shineTouch = 1100 * time.Millisecond
)

// breeze is the wind the flower hangs in: two slow waves that never line up, so
// the sway never settles into a loop, and neither of them is strong enough to
// look like a bug. It feeds the same Gust the physics already understands, which
// is why the petals flutter a little even when nothing is happening.
func (b *Buddy) breeze() float64 {
	return 0.3 + 0.25*math.Sin(b.clock*0.37) + 0.12*math.Sin(b.clock*0.93+1.7)
}

// weather is the idle frame's bookkeeping: bring the sun round when it is due.
func (b *Buddy) weather() {
	now := time.Now()
	if b.nextShine.IsZero() {
		b.scheduleShine(now)
		return
	}
	if now.After(b.nextShine) {
		b.sunlight(shinePass)
		b.scheduleShine(now)
	}
}

func (b *Buddy) scheduleShine(now time.Time) {
	b.nextShine = now.Add(shineEvery + time.Duration(b.rng.Int63n(int64(shineJitter))))
}

// sunlight starts a pass of light over the flower.
func (b *Buddy) sunlight(d time.Duration) {
	now := time.Now()
	b.shineFrom, b.shineTill = now, now.Add(d)
}

// light is how far through the current pass the sun is, or 0 when it is not out.
// A phase outside (0, 1) means nothing is drawn, so a caller can hand it to
// flower.ApplyShine without checking anything else.
func (b *Buddy) light() float64 {
	if b.shineFrom.IsZero() {
		return 0
	}
	now := time.Now()
	if !now.Before(b.shineTill) {
		return 0
	}
	total := b.shineTill.Sub(b.shineFrom).Seconds()
	if total <= 0 {
		return 0
	}
	return 1 - b.shineTill.Sub(now).Seconds()/total
}

// sunlit paints one pass of sunlight into a finished frame.
func (b *Buddy) sunlit(rgba []uint8, width, height int) {
	if phase := b.light(); phase > 0 {
		flower.ApplyShine(rgba, width, height, phase)
	}
}
