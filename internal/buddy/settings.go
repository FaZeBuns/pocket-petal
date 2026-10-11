package buddy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// The settings the menu changes, kept between runs.
//
// A tray app is opened, configured once and then left alone for weeks, so a
// setting that forgets itself on the next run is worse than no setting at all.
// The file is plain JSON in the user's config directory, and every field is
// optional: a file written by an older build loads, and anything missing keeps
// its default rather than becoming zero.

// SettingsPath is where the choices live: %APPDATA%\<slug>\settings.json on
// Windows, and the equivalent elsewhere. The folder is the app's slug rather than
// its display name, because a folder with a space in it is a folder somebody has
// to quote.
//
// POCKETPETAL_SETTINGS names the file instead, which is how a script — or a copy of
// the exe kept on a memory stick, next to its own settings — gets a settings file of
// its own rather than the one a person is using.
func SettingsPath() string {
	if path := os.Getenv("POCKETPETAL_SETTINGS"); path != "" {
		return path
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, Slug, "settings.json")
}

// The name this app had before it had this one, kept for one reason: the settings
// folder moved with the name, and a rename that quietly put somebody's flower back
// to its defaults would be the worst thing a rename can do. So the old file is read
// when the new one does not exist yet, and the next save writes it out under the new
// name (see Load).
const (
	oldSlug    = "care-package"
	oldPathEnv = "CAREPACKAGE_SETTINGS"
)

// settingsFile is the file Load and Save use. It is a variable so that a test can
// point the settings code at a file of its own: a test that writes the file a
// person is actually using would be testing with their settings.
var settingsFile = SettingsPath()

// previousSettingsPath is where the settings lived under the app's old name, or ""
// when they could not have been there at all.
func previousSettingsPath() string {
	if path := os.Getenv(oldPathEnv); path != "" {
		return path
	}
	dir, err := os.UserConfigDir()
	if err != nil || dir == "" {
		return ""
	}
	return filepath.Join(dir, oldSlug, "settings.json")
}

// Load merges the saved settings over this config. A missing or unreadable file
// is not an error: it means the defaults.
//
// It also carries settings across the rename: if this name has no file yet and the
// old one does, the old file is the settings this run starts with, and the first
// thing the app saves puts them under the current name.
func (c *Config) Load() error {
	if err := c.loadFrom(settingsFile); err != nil {
		return err
	}
	// a settings file named for this run (a script, a portable copy) is that run's
	// business and nobody else's: it is never filled in from the usual place
	if settingsFile == "" || os.Getenv("POCKETPETAL_SETTINGS") != "" {
		return nil
	}
	if _, err := os.Stat(settingsFile); err == nil {
		return nil
	}
	was := previousSettingsPath()
	if was == "" || was == settingsFile {
		return nil
	}
	if err := c.loadFrom(was); err != nil {
		return err
	}
	if path := settingsFile; path != "" {
		// written out under the new name there and then, so that this is a move
		// rather than a read of the old file on every run from now on
		if err := c.saveTo(path); err == nil {
			fmt.Fprintf(os.Stderr, "%s: your settings moved with the app to %s\n", Slug, path)
		}
	}
	return nil
}

// Save writes the settings out to the usual place.
func (c Config) Save() error { return c.saveTo(settingsFile) }

// loadFrom and saveTo are Load and Save against a named file, which is what makes
// "the settings come back the way I left them" a thing that can be tested rather
// than hoped for: the tests write a real file into a temporary directory and read
// it back with the same code the app runs.
func (c *Config) loadFrom(path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var saved Config
	if err := json.Unmarshal(data, &saved); err != nil {
		return fmt.Errorf("buddy: %s: %w", path, err)
	}
	// only what the file actually carries is applied, so a field added later
	// keeps the value the app was started with
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err == nil {
		applySaved(c, saved, raw)
	}
	return nil
}

// saveTo writes the settings out. It is best-effort on purpose: a read-only
// config directory must not stop the flower from being dragged around.
func (c Config) saveTo(path string) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// applySaved copies the fields the file named, field by field. Doing it this way
// rather than decoding straight into the config is what makes an old settings
// file safe: a field it never heard of keeps its default.
func applySaved(c *Config, saved Config, raw map[string]json.RawMessage) {
	has := func(name string) bool { _, ok := raw[name]; return ok }
	if has("seed") {
		c.Seed = saved.Seed
	}
	if has("palette") {
		c.Palette = saved.Palette
	}
	// the flower's own shape is a struct, so it is merged the same way one level
	// down: a file written before a dial existed cannot zero the dial it has
	// never heard of
	if has("shape") {
		var rawShape map[string]json.RawMessage
		if json.Unmarshal(raw["shape"], &rawShape) == nil {
			if _, ok := rawShape["hue"]; ok {
				c.Shape.Hue = saved.Shape.Hue
			}
			if _, ok := rawShape["stem"]; ok {
				c.Shape.Stem = saved.Shape.Stem
			}
			if _, ok := rawShape["stemWidth"]; ok {
				c.Shape.StemWidth = saved.Shape.StemWidth
			}
			if _, ok := rawShape["wind"]; ok {
				c.Shape.Wind = saved.Shape.Wind
			}
			if _, ok := rawShape["leaves"]; ok {
				c.Shape.Leaves = saved.Shape.Leaves
			}
			if _, ok := rawShape["bloom"]; ok {
				c.Shape.Bloom = saved.Shape.Bloom
			}
			if _, ok := rawShape["spread"]; ok {
				c.Shape.Spread = saved.Shape.Spread
			}
			if _, ok := rawShape["flutter"]; ok {
				c.Shape.Flutter = saved.Shape.Flutter
			}
			if _, ok := rawShape["limp"]; ok {
				c.Shape.Limp = saved.Shape.Limp
			}
			if _, ok := rawShape["petals"]; ok {
				c.Shape.Petals = saved.Shape.Petals
			}
		}
	}
	if has("note") {
		c.Note = saved.Note
	}
	if has("width") {
		c.Width = saved.Width
	}
	if has("height") {
		c.Height = saved.Height
	}
	if has("fit") {
		c.Fit = saved.Fit
	}
	if has("fps") {
		c.FPS = saved.FPS
	}
	if has("detail") {
		c.Detail = saved.Detail
	}
	if has("copyOnly") {
		c.CopyOnly = saved.CopyOnly
	}
	if has("interrupt") {
		c.Interrupt = saved.Interrupt
	}
	if has("autoSend") {
		c.AutoSend = saved.AutoSend
	}
	if has("caret") {
		c.Caret = saved.Caret
	}
	if has("ring") {
		c.Ring = saved.Ring
	}
	if has("sunshine") {
		c.Sunshine = saved.Sunshine
	}
	if has("burst") {
		c.Burst = saved.Burst
	}
	if has("sound") {
		c.Sound = saved.Sound
	}
	if has("corner") {
		c.Corner = saved.Corner
	}
	if has("home") {
		c.Home = saved.Home
	}
	if has("homeX") {
		c.HomeX = saved.HomeX
	}
	if has("homeY") {
		c.HomeY = saved.HomeY
	}
}
