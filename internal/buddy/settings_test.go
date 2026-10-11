package buddy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Settings that forget themselves are worse than no settings at all: the panel is
// opened once, set up, and then left alone for weeks. So the round trip is tested
// with a real file in a temporary directory, using the same code the app runs —
// including the awkward half: a file written by an *older* build, and a file that
// carries only some of the fields, must not zero the dials it has never heard of.

// ResetSettingsWhenAsked puts the settings file back to the defaults. It only runs
// when it is asked to, because the point of it is to write the file a *person* is
// using — which no test should ever do by accident.
//
//	POCKETPETAL_RESET_SETTINGS=1 go test ./internal/buddy -run TestWriteDefaultSettingsWhenAsked -v
func TestWriteDefaultSettingsWhenAsked(t *testing.T) {
	if os.Getenv("POCKETPETAL_RESET_SETTINGS") != "1" {
		t.Skip("set POCKETPETAL_RESET_SETTINGS=1 to rewrite the settings file with the defaults")
	}
	if err := DefaultConfig().Save(); err != nil {
		t.Fatalf("could not write %s: %v", SettingsPath(), err)
	}
	t.Logf("the settings at %s are the defaults again", SettingsPath())
}

// useSettingsFile points Load and Save at a file of the test's own for the rest
// of the test, so that everything from here on exercises the real save path
// without touching the settings a person is using.
func useSettingsFile(t *testing.T, path string) {
	t.Helper()
	t.Cleanup(redirectSettings(path))
}

// redirectSettings points Load and Save at a file of the caller's own until the
// returned function is called. It is shared by the tests and the benchmarks, which
// is the point: neither may ever write the settings a person is using.
func redirectSettings(path string) func() {
	old := settingsFile
	settingsFile = path
	return func() { settingsFile = old }
}

// Changing something writes the file there and then, rather than hoping for a
// clean exit: a flower that is closed with the panel still open, or killed and
// started again, comes back the way it was left. This is the path every slider
// and switch in the panel takes (see Buddy.changed, Buddy.save).
func TestChangingSomethingWritesTheSettingsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	useSettingsFile(t, path)

	b := testBuddy("periwinkle", "flower", CornerTopRight)
	b.reframe()

	// what the panel does when the pointer is let go of a slider, and when a row
	// that changes the whole flower is chosen
	b.cfg.Shape.Petals = 21
	b.cfg.Shape.Stem = 1.3
	b.cfg.Corner = CornerTopLeft
	b.cfg.Sunshine = false
	b.changed()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("nothing was written to %s: %v", path, err)
	}
	// read it back the way the next run of the app does
	reloaded := DefaultConfig()
	reloaded.Palette = "periwinkle"
	if err := reloaded.Load(); err != nil {
		t.Fatalf("reading back what the panel wrote: %v", err)
	}
	if reloaded.Shape.Petals != 21 || reloaded.Shape.Stem != 1.3 {
		t.Errorf("the shape dials were not written down: %+v", reloaded.Shape)
	}
	if reloaded.Corner != CornerTopLeft || reloaded.Sunshine {
		t.Errorf("the other choices were not written down: %+v", reloaded)
	}
	// and the file is a real settings file, not a fragment: it parses as a whole
	// configuration, so the next run keeps every default it did not name
	if raw, err := os.ReadFile(path); err != nil {
		t.Fatal(err)
	} else if !json.Valid(raw) {
		t.Fatalf("the settings file is not valid json: %s", raw)
	}
}

func writeSettings(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSettingsComeBackTheWayTheyWereLeft(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	saved := DefaultConfig()
	saved.Seed = 4242
	saved.Palette = "periwinkle"
	saved.Height = 310
	saved.Width = 236
	saved.Corner = CornerTopLeft
	saved.CopyOnly = true
	saved.Sunshine = false
	saved.Shape = FlowerSettings{
		Hue: -75, Stem: 1.4, StemWidth: 0.8, Wind: 1.7, Leaves: 1.25,
		Petals: 26, Bloom: 1.2, Spread: 1.4, Flutter: 1.5, Limp: 1.3,
	}
	// saved through the same code the app uses, but to a file of the test's own:
	// a test must never write the settings a person is actually using
	if err := saved.saveTo(path); err != nil {
		t.Fatalf("could not save to %s: %v", path, err)
	}

	loaded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// a fresh config, the way the app starts
	got := DefaultConfig()
	if err := got.loadFrom(path); err != nil {
		t.Fatalf("reading back %d bytes: %v", len(loaded), err)
	}
	if got != saved {
		t.Fatalf("the settings did not survive the trip:\n got %+v\nwant %+v", got, saved)
	}
}

// The flower's shape is a nested struct, so it is the one part of the file that
// could quietly come back empty. Every dial has to make the trip on its own.
func TestEveryShapeDialIsRemembered(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	saved := DefaultConfig()
	saved.Shape.Hue = 120
	saved.Shape.Stem = 0.7
	saved.Shape.Petals = 11
	if err := saved.saveTo(path); err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["shape"]; !ok {
		t.Fatalf("the shape is not in the file at all: %s", data)
	}
	got := DefaultConfig()
	if err := got.loadFrom(path); err != nil {
		t.Fatal(err)
	}
	if got.Shape != saved.Shape {
		t.Fatalf("shape did not survive: got %+v, want %+v", got.Shape, saved.Shape)
	}
	// and a dial the file does not mention keeps the value this run was started
	// with, rather than being zeroed by a file that has never heard of it
	partial := writeSettings(t, `{"shape": {"stem": 1.5}}`)
	before := got.Shape
	if err := got.loadFrom(partial); err != nil {
		t.Fatal(err)
	}
	if got.Shape.Stem != 1.5 {
		t.Errorf("the file's stem was ignored: %+v", got.Shape)
	}
	if got.Shape.Hue != before.Hue || got.Shape.Petals != before.Petals || got.Shape.Bloom != before.Bloom {
		t.Errorf("a half-written shape zeroed the dials it did not mention: %+v", got.Shape)
	}
}

// A settings file from an older build, and a file full of nonsense, both have to
// leave a usable flower behind.
func TestOldAndBrokenSettingsFilesAreSurvivable(t *testing.T) {
	old := writeSettings(t, `{"seed": 9, "palette": "daisy", "width": 180}`)
	c := DefaultConfig()
	if err := c.loadFrom(old); err != nil {
		t.Fatalf("an old file was not readable: %v", err)
	}
	if c.Seed != 9 || c.Palette != "daisy" || c.Width != 180 {
		t.Errorf("an old file's fields were dropped: %+v", c)
	}
	if c.Shape != DefaultShape() {
		t.Errorf("an old file changed the shape it never mentioned: %+v", c.Shape)
	}
	if c.Height != DefaultConfig().Height || c.Corner != CornerBottomRight {
		t.Errorf("an old file zeroed the fields it does not carry: %+v", c)
	}

	broken := writeSettings(t, `{"seed": 9,`)
	if err := c.loadFrom(broken); err == nil {
		t.Error("a truncated file should be reported, not silently ignored")
	}

	// and a missing file is simply the defaults
	missing := filepath.Join(t.TempDir(), "nope.json")
	fresh := DefaultConfig()
	if err := fresh.loadFrom(missing); err != nil {
		t.Errorf("a missing file is not an error: %v", err)
	}
	if fresh != DefaultConfig() {
		t.Error("a missing file changed the defaults")
	}
}
