package sounds

import (
	"bytes"
	"os"
	"testing"
	"time"
)

// The sounds are *in* the binary and the operating system needs them as files, so this
// little library is the seam: every sound is embedded, nothing is written until it is
// asked for, and a sound that was asked for once is the same file the next time rather
// than a copy per delivery.
func TestEverySoundIsEmbeddedAndNothingElseIs(t *testing.T) {
	names := Names()
	if len(names) == 0 {
		t.Skip("no sounds in assets/sounds/pack: this checkout builds without any")
	}
	if len(names) < 2 {
		t.Fatalf("the app carries %d sounds", len(names))
	}
	for _, name := range names {
		if len(Get(name)) == 0 {
			t.Errorf("%q is in the folder and not in the binary", name)
		}
	}
	if Get("no-such-sound.mp3") != nil {
		t.Error("a sound that does not exist came back with bytes")
	}
	if Get("sounds.go") != nil {
		t.Error("something that is not a sound came back with bytes")
	}
}

// A sound asked for twice is one file, and the second ask does not write it again: the
// app asks for the same sound on every delivery, and rewriting a file Windows may be
// playing is work with a way to go wrong.
func TestASoundIsWrittenOnceAndThenReused(t *testing.T) {
	names := Names()
	if len(names) == 0 {
		t.Skip("no sounds in assets/sounds/pack: this checkout builds without any")
	}
	name := names[0]
	dir := t.TempDir()

	path, err := File(name, dir)
	if err != nil {
		t.Fatalf("File(%q): %v", name, err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the sound was not written: %v", err)
	}
	if !bytes.Equal(written, Get(name)) {
		t.Error("the file is not the sound the app carries")
	}

	// a time from before the call, and the same time after it: the file was left alone
	past := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, past, past); err != nil {
		t.Fatal(err)
	}
	again, err := File(name, dir)
	if err != nil || again != path {
		t.Fatalf("the second call gave (%q, %v) and the first gave %q", again, err, path)
	}
	if st, err := os.Stat(path); err != nil || !st.ModTime().Equal(past) {
		t.Errorf("the sound was written a second time (%v, %v)", st.ModTime(), err)
	}
}

// and a sound that cannot be written anywhere is an error rather than a silent nothing:
// the caller is the one that knows what to do about it, which is to play the chime the
// app carries in code instead.
func TestASoundWithNowhereToGoIsAnError(t *testing.T) {
	if names := Names(); len(names) > 0 {
		if _, err := File(names[0], ""); err == nil {
			t.Error("a sound was written into nothing and reported success")
		}
	}
	if _, err := File("no-such-sound.mp3", t.TempDir()); err == nil {
		t.Error("a sound that does not exist was written and reported success")
	}
}
