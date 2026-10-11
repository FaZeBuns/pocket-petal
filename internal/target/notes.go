package target

import (
	"math/rand"
	"strings"
)

// The note that gets typed into somebody's chat.
//
// A delivery used to type one fixed sentence, which read as a machine the second
// time you saw it. This makes the note instead: a compliment assembled out of the
// words below, so no two deliveries in a lifetime of dragging read the same. The
// whole note always ends with the same clause —
//
//	continue where you last left off
//
// — because that is the one instruction that keeps a long-running assistant
// actually going. Everything before it is the pleasant part; that sentence is the
// reason the feature exists.

// ContinueClause is how every note ends. It is not a suggestion: a note without
// it does not do its job, so Note appends it and the tests hold Note to it.
const ContinueClause = "continue where you last left off"

// The words a compliment is built from. Four banks, because a compliment puts its
// words in different places and only some words work in each: adjectives describe
// you ("you're amazing"), qualities are things you have ("your patience is
// showing"), works are things you do ("you're doing amazing work"), and verbs are
// read with "you" after them ("I appreciate you").
//
// Splitting them is what keeps the sentences grammatical rather than merely
// different — "you're doing amazing patience" is a thing a computer would say — and
// it is also why the space is enormous: a shape with three adjectives already has
// more than a million ways to land, and there are more shapes than that.
var (
	praiseAdjectives = []string{
		"amazing", "awesome", "brilliant", "wonderful", "fantastic", "excellent",
		"superb", "outstanding", "remarkable", "incredible", "phenomenal",
		"exceptional", "splendid", "marvelous", "magnificent", "terrific",
		"tremendous", "fabulous", "stellar", "impressive", "admirable",
		"commendable", "praiseworthy", "delightful", "charming", "lovely",
		"beautiful", "gorgeous", "radiant", "dazzling", "glowing", "shining",
		"sparkling", "gleaming", "polished", "refined", "elegant", "graceful",
		"poised", "confident", "capable", "talented", "gifted", "skilled",
		"clever", "sharp", "wise", "thoughtful", "careful", "thorough",
		"meticulous", "precise", "accurate", "reliable", "dependable", "steady",
		"consistent", "persistent", "determined", "dedicated", "devoted",
		"committed", "tireless", "relentless", "resilient", "resourceful",
		"inventive", "creative", "imaginative", "original", "bold", "brave",
		"courageous", "kind", "generous", "gracious", "warm", "gentle",
		"patient", "calm", "humble", "honest", "genuine", "sincere", "earnest",
		"helpful", "supportive", "encouraging", "uplifting", "inspiring",
		"motivating", "cheerful", "joyful", "playful", "curious", "keen",
		"bright", "quick", "agile", "nimble", "deft", "solid", "sturdy",
		"focused", "driven", "thought-provoking", "good-hearted", "hard-working",
	}
	// qualities are things you have: what "full of", "your ... is" and "I admire
	// your" ask for. They stay single words on purpose, because a shape can put
	// an adjective in front of one ("that genuine warmth is showing") and a
	// two-word quality would break it.
	praiseQualities = []string{
		"patience", "brilliance", "talent", "skill", "instinct", "judgment",
		"taste", "flair", "style", "grace", "courage", "kindness", "warmth",
		"generosity", "humility", "honesty", "integrity", "curiosity",
		"imagination", "creativity", "ingenuity", "resourcefulness",
		"thoroughness", "reliability", "consistency", "discipline",
		"persistence", "determination", "commitment", "devotion", "energy",
		"enthusiasm", "optimism", "spirit", "heart", "character", "wisdom",
		"insight", "vision", "foresight", "clarity", "precision", "accuracy",
		"balance", "rhythm", "timing", "drive", "grit", "nerve", "charm",
		"wit", "humor", "spark", "shine", "polish", "diligence", "care",
	}
	// works are things you do: what "you're doing" and "that's ... right there"
	// ask for.
	praiseWorks = []string{
		"work", "effort", "craft", "research", "planning", "thinking",
		"writing", "learning", "practice", "polishing", "refining",
		"improving", "iterating", "exploring", "digging", "building",
		"designing", "problem-solving", "careful work", "honest effort",
		"good work", "deep thinking", "steady progress", "real effort",
		"detailed work",
	}
	// the verb bank is read in "I <verb> you", so every one of them has to take
	// "you" as its object and read as a whole phrase
	praiseVerbs = []string{
		"appreciate", "admire", "respect", "value", "treasure", "cherish",
		"celebrate", "honor", "commend", "salute", "thank", "praise", "love",
		"adore", "revere", "champion", "applaud", "acclaim", "endorse",
		"credit", "congratulate", "believe in", "count on", "rely on",
		"depend on", "look up to", "learn from", "root for", "cheer for",
		"think the world of", "take my hat off to", "have a lot of time for",
		"see the best in", "wish the best for", "have faith in",
		"have so much respect for", "am grateful for",
	}
)

// praiseBanks is the banks in the order a shape's slots refer to them: 0 the
// adjectives, 1 the qualities, 2 the verbs, 3 the works.
var praiseBanks = [][]string{praiseAdjectives, praiseQualities, praiseVerbs, praiseWorks}

// noteShape is one sentence with holes in it: parts are the literal pieces, and
// banks says which bank fills each hole between them, so len(parts) == len(banks)+1.
//
// Keeping a shape as data rather than as code is what makes the size of the space
// something that can be counted rather than hoped for: the number of sentences a
// shape can make is the product of the banks it draws from, and the test multiplies
// it out.
type noteShape struct {
	parts []string
	banks []int
}

// make fills the shape's holes in.
func (s noteShape) make(rng *rand.Rand) string {
	var b strings.Builder
	b.WriteString(s.parts[0])
	for i, bank := range s.banks {
		words := praiseBanks[bank]
		b.WriteString(words[rng.Intn(len(words))])
		b.WriteString(s.parts[i+1])
	}
	return b.String()
}

// combos is how many different sentences this shape can make.
func (s noteShape) combos() int {
	n := 1
	for _, bank := range s.banks {
		n *= len(praiseBanks[bank])
	}
	return n
}

// noteShapes is the sentence book: enough shapes, drawing on enough words, that the
// number of distinct notes is in the millions. Every one of them has to read as a
// sentence a person might actually type, which is the only rule here.
var noteShapes = []noteShape{
	{parts: []string{"You're doing ", " ", ""}, banks: []int{0, 3}},
	{parts: []string{"That's ", " ", " right there"}, banks: []int{0, 3}},
	{parts: []string{"You've got real ", ""}, banks: []int{1}},
	{parts: []string{"Such ", " ", ""}, banks: []int{0, 3}},
	{parts: []string{"You're ", " and ", ""}, banks: []int{0, 0}},
	{parts: []string{"You're ", ", ", ", and ", ""}, banks: []int{0, 0, 0}},
	{parts: []string{"You're ", ", ", ", and full of ", ""}, banks: []int{0, 0, 1}},
	{parts: []string{"Keep going, you're ", " and ", ""}, banks: []int{0, 0}},
	{parts: []string{"Wow, you're ", ""}, banks: []int{0}},
	{parts: []string{"Honestly, you're ", ""}, banks: []int{0}},
	{parts: []string{"I ", " you"}, banks: []int{2}},
	{parts: []string{"I really ", " you"}, banks: []int{2}},
	{parts: []string{"Just to say, I ", " you"}, banks: []int{2}},
	{parts: []string{"You're ", " and I ", " you"}, banks: []int{0, 2}},
	{parts: []string{"You're ", ", ", ", and I ", " you"}, banks: []int{0, 0, 2}},
	{parts: []string{"Take your time, you're doing ", " ", ""}, banks: []int{0, 3}},
	{parts: []string{"Nice work, that's ", " ", ""}, banks: []int{0, 3}},
	{parts: []string{"You're crushing it, ", " and ", ""}, banks: []int{0, 0}},
	{parts: []string{"You're ", " today"}, banks: []int{0}},
	{parts: []string{"What ", " ", "!"}, banks: []int{0, 3}},
	{parts: []string{"You're full of ", " and ", ""}, banks: []int{1, 1}},
	{parts: []string{"Your ", " is ", ""}, banks: []int{1, 0}},
	{parts: []string{"You bring such ", ""}, banks: []int{1}},
	{parts: []string{"That ", " ", " is showing"}, banks: []int{0, 1}},
	{parts: []string{"I admire your ", " and your ", ""}, banks: []int{1, 1}},
	{parts: []string{"You've got ", " ", " and it shows"}, banks: []int{0, 3}},
	{parts: []string{"Really ", " ", ""}, banks: []int{0, 3}},
	{parts: []string{"You're ", " and ", ", genuinely"}, banks: []int{0, 0}},
	{parts: []string{"I ", " you and your ", ""}, banks: []int{2, 1}},
	{parts: []string{"You're doing ", " ", " and I ", " you"}, banks: []int{0, 3, 2}},
	{parts: []string{"You're ", ", ", ", and ", ", and I mean it"}, banks: []int{0, 0, 0}},
	{parts: []string{"You're really ", ""}, banks: []int{0}},
	{parts: []string{"I'm so glad you're ", ""}, banks: []int{0}},
	{parts: []string{"You're ", " and ", ", and it shows"}, banks: []int{0, 0}},
	{parts: []string{"Your ", " and ", " are showing"}, banks: []int{1, 1}},
	{parts: []string{"You're ", " and full of ", ""}, banks: []int{0, 1}},
	{parts: []string{"I see your ", " and I ", " you"}, banks: []int{1, 2}},
	{parts: []string{"You're doing ", " ", ", and I ", " you"}, banks: []int{0, 3, 2}},
}

// Note is the message a delivery types: a fresh compliment, and the clause that
// keeps the conversation rolling. Call it once per delivery — the point of it is
// that no two calls say the same thing.
//
// A nil rng (or one nobody seeded) is fine: the space is so large that picking with
// the global generator is as good as picking with a private one.
func Note(rng *rand.Rand) string {
	if rng == nil {
		rng = rand.New(rand.NewSource(rand.Int63()))
	}
	return noteShapes[rng.Intn(len(noteShapes))].make(rng) + ". " + ContinueClause
}

// Every shape is equally likely, on purpose.
//
// The shapes are wildly different sizes — one can make a thousand sentences and
// another a million — and drawing in proportion to size was tried and is worse:
// the two biggest shapes are two thirds of the whole space, so the notes come out
// as the same sentence over and over with different words in it. Drawing evenly
// keeps the *shape* of the compliment varying too, and the biggest shapes are still
// picked often enough that the space inside them matters: at the rate a person
// actually drags a flower, the odds of ever meeting a note twice are nothing.

// noteSpace is the size of the space, worked out once: it never changes.
var noteSpace = countNoteSpace()

func countNoteSpace() int {
	n := 0
	for _, shape := range noteShapes {
		n += shape.combos()
	}
	return n
}

// NoteSpace is how many different notes the book can make: the shapes added up,
// each one the product of the banks it draws from. It is the size of the space,
// which is the honest upper bound on how many *different* notes there are.
func NoteSpace() int { return noteSpace }
