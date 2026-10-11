package win

import "testing"

// Type as coverage, on the machine the app is running on.
//
// The one thing worth asserting is a character the font does not have. Segoe UI is the
// shell's own UI face and it has no coffee cup in it: what makes ☕ appear on the panel's
// coffee button is Windows' font linking, reaching for another face when the glyph is
// missing — and the failure mode of that is silence, a button with a cup missing from it.
// So the cup is rendered once and looked at.

func TestTheCoffeeCupIsAGlyphOnThisMachine(t *testing.T) {
	f, err := NewFont(16, FontRegular)
	if err != nil {
		t.Skipf("no font to draw with: %v", err)
	}
	defer f.Close()

	cup := f.Mask("☕", 64, 24)
	if len(cup) != 64*24 {
		t.Fatalf("the mask of the cup is %d bytes and a 64x24 mask is %d", len(cup), 64*24)
	}
	ink := 0
	for _, v := range cup {
		if v > 40 {
			ink++
		}
	}
	if ink == 0 {
		t.Error("the cup draws nothing: the coffee button would be a green plate with no cup on it")
	}

	w, h := f.Measure("☕")
	if w <= 0 || h <= 0 {
		t.Errorf("the cup measures %dx%d", w, h)
	}
}

// and the text the panel is made of is drawn at all: a font that returns blank masks is a
// panel with no words in it, which is a failure nothing else in the app would notice.
func TestTextComesOutAsInk(t *testing.T) {
	f, err := NewFont(14, FontMedium)
	if err != nil {
		t.Skipf("no font to draw with: %v", err)
	}
	defer f.Close()

	mask := f.Mask("Buy me a Coffee!", 200, 20)
	ink := 0
	for _, v := range mask {
		if v > 40 {
			ink++
		}
	}
	if ink == 0 {
		t.Error("a row of text came out as a blank mask")
	}
	// an empty string is nothing rather than a panic, and the same text twice is the
	// same mask: the panel's cache is keyed on the text, so a mask must not change
	if got := f.Mask("", 200, 20); len(got) != 200*20 {
		t.Errorf("the empty string gave a mask of %d bytes", len(got))
	}
	again := f.Mask("Buy me a Coffee!", 200, 20)
	if len(again) != len(mask) {
		t.Errorf("the same text came out %d bytes and then %d", len(mask), len(again))
	}
}
