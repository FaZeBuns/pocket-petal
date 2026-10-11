// Package sounds is the little library of noises the flower makes, embedded in the
// binary.
//
// The files are MP3s and they are *in* the exe (embed), not beside it: copy the exe
// anywhere and the sounds come with it. Windows will not play an MP3 out of memory —
// PlaySound only understands a WAV there — so the bytes are handed to it through a
// file: one copy is written under the app's directory in the user's temp folder the
// first time each sound is used, and played from there for the rest of the run. The
// temp copy is an implementation detail of the operating system's, not something the
// app needs installed.
//
// The MP3s are not in the repository: they are git-ignored files in pack/, embedded
// into the exe when it is built on a machine that has them. A checkout with none builds
// a perfectly good exe with no sounds, which plays the chime the app synthesises in
// memory instead.
package sounds

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// pack always matches at least its README.txt, so this compiles with or without sounds.
//
//go:embed pack
var files embed.FS

const packDir = "pack"

func isSound(name string) bool { return strings.HasSuffix(name, ".mp3") }

// Names are every sound there is, in a stable order. There may be none.
func Names() []string {
	entries, err := files.ReadDir(packDir)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && isSound(e.Name()) {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

// Get is one sound's bytes, or nothing when there is no such sound.
func Get(name string) []byte {
	if !isSound(name) {
		return nil
	}
	data, err := files.ReadFile(packDir + "/" + name)
	if err != nil {
		return nil
	}
	return data
}

// File writes one sound into dir — unless it is already there, in which case that copy
// is used — and returns the path Windows can be pointed at.
func File(name, dir string) (string, error) {
	data := Get(name)
	if data == nil {
		return "", fmt.Errorf("sounds: no sound called %q", name)
	}
	if dir == "" {
		return "", fmt.Errorf("sounds: nowhere to put %q", name)
	}
	path := filepath.Join(dir, name)
	if st, err := os.Stat(path); err == nil && st.Size() == int64(len(data)) {
		return path, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
