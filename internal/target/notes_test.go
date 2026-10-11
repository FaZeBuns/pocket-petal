package target

import (
	"math/rand"
	"strings"
	"testing"
	"unicode"
)

// The note is the whole point of a delivery, so it is worth being exact about the
// two things that make it work: it never repeats itself in any way a person would
// notice, and it always ends with the clause that keeps the chat going.

// Whatever else it says, every note ends with the same instruction. This is the
// requirement, stated once: a note that does not end with it does not do its job.
func TestEveryNoteEndsWithTheClauseThatKeepsTheChatGoing(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 5000; i++ {
		note := Note(rng)
		if !strings.HasSuffix(note, ContinueClause) {
			t.Fatalf("note does not end with %q: %q", ContinueClause, note)
		}
		if strings.ContainsAny(note, "\r\n") {
			t.Fatalf("note %q is not one line", note)
		}
		trimmed := strings.TrimSpace(note)
		if trimmed == "" {
			t.Fatal("a delivery with nothing to say")
		}
		if r := []rune(trimmed)[0]; !unicode.IsUpper(r) {
			t.Fatalf("note starts with %q: %q", string(r), note)
		}
		if n := len([]rune(note)); n > 160 {
			t.Fatalf("note is %d characters, too long to type: %q", n, note)
		}
	}
}

// How big the space is. The user asked for something on the order of a hundred
// thousand or a million different notes; this states that the sentence book can
// make more than a million before a single one is drawn.
func TestTheNoteSpaceIsInTheMillions(t *testing.T) {
	space := NoteSpace()
	t.Logf("the sentence book can make %d different notes from %d shapes", space, len(noteShapes))
	if space < 1_000_000 {
		t.Fatalf("only %d notes are possible: a lifetime of dragging would repeat", space)
	}
}

// And that the space is really used: at the rate a person actually drags a flower
// the notes are all different, which a small fixed list dressed up with one word
// could not manage. The run is longer than anybody's patience with the app, so
// anything a person meets is inside it.
func TestTheNotesDoNotRepeatThemselves(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	const draws = 5_000
	seen := make(map[string]struct{}, draws)
	for i := 0; i < draws; i++ {
		seen[Note(rng)] = struct{}{}
	}
	t.Logf("%d draws made %d different notes", draws, len(seen))
	if len(seen) < 4_000 {
		t.Fatalf("%d draws made only %d different notes", draws, len(seen))
	}
}

// The longest sentence the book can make has to fit what a chat box will take,
// worked out from the longest word in each bank rather than by sampling and
// hoping the long one came up.
func TestTheLongestNoteFitsWhatCanBeTyped(t *testing.T) {
	longest := func(bank int) int {
		n := 0
		for _, w := range praiseBanks[bank] {
			n = max(n, len([]rune(w)))
		}
		return n
	}
	size := func(bank int) int {
		n := 0
		for _, w := range praiseBanks[bank] {
			if w == "" {
				t.Fatalf("bank %d has an empty word in it", bank)
			}
			n++
		}
		return n
	}
	seen := map[string]bool{}
	for bank := range praiseBanks {
		if size(bank) < 20 {
			t.Fatalf("bank %d holds only %d words", bank, size(bank))
		}
		for _, w := range praiseBanks[bank] {
			if seen[w] {
				t.Errorf("%q appears in the banks twice", w)
			}
			seen[w] = true
		}
	}

	worst := 0
	for _, shape := range noteShapes {
		if len(shape.parts) != len(shape.banks)+1 {
			t.Fatalf("shape %v has %d parts and %d banks", shape.parts, len(shape.parts), len(shape.banks))
		}
		n := 0
		for _, p := range shape.parts {
			n += len([]rune(p))
		}
		for _, bank := range shape.banks {
			n += longest(bank)
		}
		n += len([]rune(". ")) + len([]rune(ContinueClause))
		worst = max(worst, n)
	}
	t.Logf("the longest note the book can make is %d characters", worst)
	if worst > 160 {
		t.Fatalf("the longest note is %d characters, which is more than a chat box takes", worst)
	}
}

// A delivery picks a sentence from the book and fills it, so every possible note
// has to actually come out of that path.
func TestNoteShapesAreFilledIn(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for i := 0; i < 200; i++ {
		note := Note(rng)
		body := strings.TrimSuffix(note, ". "+ContinueClause)
		if body == note {
			t.Fatalf("note %q is not a compliment followed by the clause", note)
		}
		if strings.Contains(body, "%") || strings.Contains(body, "  ") {
			t.Fatalf("note %q has an unfilled hole in it", note)
		}
		if len(strings.Fields(body)) < 2 {
			t.Fatalf("note %q is not a sentence", note)
		}
	}
	// a nil generator is a fine thing to hand it: it is how a caller with no
	// randomness of its own asks for one
	if note := Note(nil); !strings.HasSuffix(note, ContinueClause) {
		t.Fatalf("Note(nil) gave %q", note)
	}
}
