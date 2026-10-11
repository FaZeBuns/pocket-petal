package buddy

import (
	"encoding/json"
	"os"
	"time"
)

// Where the flower is, written down for whoever is watching.
//
// A drag is a hand and a picture, and the two ways of checking one from outside the
// app are both bad: the pointer can be read but the flower cannot, and the flower's
// pixels are only reachable where nothing else is on top of them. So the app says
// where it is: `-state <file>` writes a small JSON record of the flower's own idea
// of itself — where the window is, where the base of the stem is on screen, and
// where the hand holding it is — a few times a second.
//
// Which is exactly what the awkward question needs: "did the flower move when I let
// go of it?" is a comparison of two numbers, and tools/drag-check.ps1 makes it.
type flowerState struct {
	Clock    float64 `json:"clock"`
	Dragging bool    `json:"dragging"`
	Walking  bool    `json:"walking"`
	WindowX  int     `json:"windowX"`
	WindowY  int     `json:"windowY"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	// Base is the base of the stem — the point the flower stands on — in screen
	// pixels, and Hand is the point the window is being placed by: the same point
	// while the flower is standing, and the piece of it being carried while it is
	// held.
	BaseX int `json:"baseX"`
	BaseY int `json:"baseY"`
	HandX int `json:"handX"`
	HandY int `json:"handY"`
	// what the hand has hold of, while it has hold of anything
	GrabS    float64 `json:"grabS,omitempty"`
	GrabHead bool    `json:"grabHead,omitempty"`

	// The display the flower is standing on, as the flower read it: the work area it
	// was placed in, the scaling its size was worked out at, and how tall it is drawn.
	// These are here because "is the flower the right size on this screen?" is the one
	// question about a desktop flower that cannot be answered by looking at it: the
	// answer is the settings times the display, and the display is what this records.
	// A resolution or scaling change is visible in these three numbers changing.
	Scale        float64 `json:"scale"`
	WorkWidth    int     `json:"workWidth"`
	WorkHeight   int     `json:"workHeight"`
	FlowerHeight float64 `json:"flowerHeight"`
}

// stateEvery is how often the file is rewritten. A drag is sixty frames a second and
// a person's hand is slower than that: ten records a second is enough to see where
// the flower was a moment before it was let go of, and small enough that nobody
// notices it happening.
const stateEvery = 100 * time.Millisecond

// maybeState writes the state file if enough time has passed since the last one.
func (b *Buddy) maybeState(now time.Time) {
	if b.cfg.State == "" {
		return
	}
	if !b.stateAt.IsZero() && now.Sub(b.stateAt) < stateEvery {
		return
	}
	b.stateAt = now

	baseX, baseY := b.standingPoint()
	s := flowerState{
		Clock:        b.clock,
		Dragging:     b.dragging,
		Walking:      b.walking,
		BaseX:        baseX,
		BaseY:        baseY,
		HandX:        int(b.handX + 0.5),
		HandY:        int(b.handY + 0.5),
		Scale:        b.deskScale,
		WorkWidth:    b.deskArea.Width(),
		WorkHeight:   b.deskArea.Height(),
		FlowerHeight: b.frame.Flower,
	}
	if b.win != nil {
		x, y := b.win.Position()
		s.WindowX, s.WindowY = x, y
		s.Width, s.Height = b.win.W, b.win.H
	}
	if b.dragging {
		s.GrabS, s.GrabHead = b.grabS, b.grabHead
	}
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	data = append(data, '\n')
	// written beside itself and moved into place, so a reader never catches half a
	// record: a script that read the state file mid-drag must not parse "…,\"baseY\":"
	tmp := b.cfg.State + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, b.cfg.State)
}
