package flower

import (
	"math"
	"testing"
)

// The colour wheel is the one thing in the panel that changes every colour at
// once, so it is worth stating what it may and may not do: it may move a colour
// round the spectrum, and it may not change how light or how vivid it is. The
// whole drawing is shaded on the assumption that lightness and saturation mean
// what they mean.

func TestNoTurnChangesNothing(t *testing.T) {
	for _, name := range PaletteNames {
		pal := PaletteNamed(name)
		if got := HueTurn(pal, 0); got != pal {
			t.Errorf("%s: a zero turn changed the colourway", name)
		}
	}
}

func TestTheColourWheelKeepsLightnessAndSaturation(t *testing.T) {
	for _, name := range PaletteNames {
		pal := PaletteNamed(name)
		for _, turn := range []float64{-180, -60, 45, 137, 180} {
			turned := HueTurn(pal, turn)
			for _, pair := range [][2]RGB{
				{pal.PetalTip, turned.PetalTip},
				{pal.PetalBase, turned.PetalBase},
				{pal.PetalDeep, turned.PetalDeep},
				{pal.Center, turned.Center},
				{pal.Stem, turned.Stem},
				{pal.Leaf, turned.Leaf},
			} {
				_, s0, l0 := rgbToHSL(pair[0])
				_, s1, l1 := rgbToHSL(pair[1])
				if math.Abs(l0-l1) > 0.02 || math.Abs(s0-s1) > 0.03 {
					t.Errorf("%s by %.0f°: %v -> %v is not the same colour at a different hue "+
						"(lightness %.3f -> %.3f, saturation %.3f -> %.3f)",
						name, turn, pair[0], pair[1], l0, l1, s0, s1)
				}
			}
		}
	}
}

// The stem and the leaves turn a fifth as far as the bloom, so that a wheel turned
// to its limit gives a flower in a strange colour on a plausible stem rather than
// a violet stalk.
func TestTheStemDoesNotFollowTheWheelAllTheWay(t *testing.T) {
	pal := PaletteNamed("blush")
	turned := HueTurn(pal, 180)
	if turned.PetalTip == pal.PetalTip {
		t.Fatal("a half turn did not change the bloom")
	}
	hue0, _, _ := rgbToHSL(pal.Stem)
	hue1, _, _ := rgbToHSL(turned.Stem)
	moved := math.Abs(hue1 - hue0)
	if moved > 0.5 {
		moved = 1 - moved
	}
	if moved > 0.3 {
		t.Fatalf("the stem turned %.0f° when the bloom turned 180°", moved*360)
	}
	if math.Abs(hue1-hue0) < 1e-6 {
		t.Fatal("the stem did not turn at all: the wheel would leave it behind")
	}
}

// A colour that is already grey has no hue to move, and one that is at the very
// dark end must not come back out of range.
func TestTheWheelSurvivesGreysAndEdges(t *testing.T) {
	for _, c := range []RGB{{0, 0, 0}, {255, 255, 255}, {128, 128, 128}, {255, 0, 0}, {0, 0, 12}} {
		for _, turn := range []float64{-180, 0, 90, 180, 360, -540} {
			got := RotateHue(c, turn)
			for _, v := range got {
				if math.IsNaN(v) || v < 0 || v > 255 {
					t.Fatalf("RotateHue(%v, %.0f) = %v", c, turn, got)
				}
			}
		}
	}
	// and a full turn is the identity
	if got := RotateHue(RGB{200, 40, 90}, 360); math.Abs(got[0]-200) > 1.5 || math.Abs(got[1]-40) > 1.5 || math.Abs(got[2]-90) > 1.5 {
		t.Fatalf("a full turn changed the colour: %v", got)
	}
}
