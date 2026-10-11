package target

import "math/rand"

// What a drop delivers: Message is what gets typed into the window, Flavor is the
// caption shown to the human while the item flies in.
type Item struct {
	ID      string
	Emoji   string
	Name    string
	Verb    string
	Flavor  string
	Accent  string
	Message string
}

// Items is what the app can deliver.
//
// There is one, and it is the flower. The app is a flower you pick up and put on
// somebody's chat, and everything about it says so: the bloom it comes off, the
// petals it bursts into, the colour it wears. The type still has every field a
// second item would need.
var Items = []Item{
	{
		ID:      "flower",
		Emoji:   "🌸",
		Name:    "Flower",
		Verb:    "Send a flower",
		Flavor:  "you're doing great, robot.",
		Accent:  "#ff9ec7",
		Message: "🌸 Flower delivery! Take one breath, then tell me one small thing you're sure about, and keep going.",
	},
}

// ByID finds an item, and says no politely for strangers.
func ByID(id string) (Item, bool) {
	for _, item := range Items {
		if item.ID == id {
			return item, true
		}
	}
	return Item{}, false
}

// Random picks one, with the caller's own randomness when it wants reproducible
// runs.
func Random(rng func() float64) Item {
	if rng == nil {
		rng = rand.Float64
	}
	return Items[int(rng()*float64(len(Items)))%len(Items)]
}

// IDs is the list of item ids, in order.
func IDs() []string {
	out := make([]string, 0, len(Items))
	for _, item := range Items {
		out = append(out, item.ID)
	}
	return out
}
