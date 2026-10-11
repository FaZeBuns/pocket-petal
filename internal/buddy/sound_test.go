package buddy

import (
	"math/rand"
	"testing"

	"github.com/FaZeBuns/pocket-petal/internal/chime"
	"github.com/FaZeBuns/pocket-petal/internal/target"
)

// The chime is drawn rather than fixed, and the promise that makes drawing it worth
// anything is that every sound is reachable: over a lifetime of deliveries a person
// should hear all of them and be able to predict none. A round-robin would pass this and
// is what the app used to do; what it cannot do is what a list is for — two deliveries in
// a row landing on sounds that have nothing to do with each other.
func TestTheDropSoundIsDrawnFromTheWholeList(t *testing.T) {
	b := &Buddy{rng: rand.New(rand.NewSource(1))}
	drops := chime.Drops()
	if len(drops) < 2 {
		t.Fatalf("there are %d sounds to draw from", len(drops))
	}
	heard := map[string]int{}
	for i := 0; i < 500; i++ {
		name := b.dropSoundName()
		if name == "" {
			t.Fatal("the flower went silent: no sound came back")
		}
		heard[name]++
	}
	for _, name := range drops {
		if heard[name] == 0 {
			t.Errorf("%q was never drawn in 500 drops: the list is longer than the choice", name)
		}
	}
	if len(heard) != len(drops) {
		t.Errorf("%d different sounds came back and the list holds %d", len(heard), len(drops))
	}
}

// The chime is for the chat, and the app is silent everywhere else. The rule is
// deliberately narrower than "somewhere a note goes": the flower is carried over a lot
// of windows on the way to the one that matters, a note can be delivered into a terminal
// or a clipboard, and a chime for any of those would be the app reporting something that
// did not happen.
func TestTheChimeIsHeardOnlyOnAChat(t *testing.T) {
	for _, c := range []struct {
		what  string
		spot  target.Target
		lands bool
	}{
		{"a chat it recognised", target.Target{Kind: "ai-chat", Label: "ChatGPT", AIChat: true}, true},
		{"the agent in a terminal", target.Target{Kind: "terminal", Label: "Terminal", Terminal: true}, false},
		{"a browser page", target.Target{Kind: "browser", Label: "The Verge", Browser: true}, false},
		{"an editor", target.Target{Kind: "app", Label: "Notepad"}, false},
		{"the desktop", target.Target{}, false},
	} {
		if got := lands(c.spot); got != c.lands {
			t.Errorf("a drop onto %s: chime = %v, and it should be %v", c.what, got, c.lands)
		}
	}
}
