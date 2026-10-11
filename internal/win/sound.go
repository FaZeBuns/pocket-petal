package win

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// Sound, as far as the app is concerned: two calls and no mixer.
//
// PlaySoundW with SND_MEMORY plays a WAV that is already in memory, which is how the
// app's spare chime is played (see internal/chime). The sounds a person actually
// hears are MP3s, and Windows will only play one of those from a file — so the other
// half of this file is MCI, told to open a path and play it, which is the same
// machinery the old Media Player control used and is present on every Windows.

var (
	winmm          = syscall.NewLazyDLL("winmm.dll")
	pPlaySound     = winmm.NewProc("PlaySoundW")
	pMciSendString = winmm.NewProc("mciSendStringW")
	mciMu          sync.Mutex // MCI has one device open at a time, and the machine is shared
)

const (
	soundAsync     = 0x0001 // come back immediately; the app keeps drawing
	soundNoDefault = 0x0002 // and if it cannot be played, play nothing at all
	soundMemory    = 0x0004 // the pointer is a WAV in memory, not a file name
)

// playing holds the buffer being played. Windows reads it until the sound ends, so
// something has to own it: without this the collector is free to take the sound away
// halfway through, which is heard as the chime stopping early.
var playing []byte

// PlayWav plays a WAV held in memory, without waiting for it.
//
// It replaces whatever is playing rather than queueing behind it: a second flower
// going off should be the sound you hear, not the first one finishing.
func PlayWav(wav []byte) bool {
	if len(wav) == 0 {
		return false
	}
	playing = wav
	ret, _, _ := pPlaySound.Call(
		uintptr(unsafe.Pointer(&wav[0])),
		0, // no module to load it from: it is already here
		soundMemory|soundAsync|soundNoDefault,
	)
	return ret != 0
}

// mciAlias is the name MCI knows the flower's sound by. One at a time: a second
// sound replaces the first rather than playing over it, which is what a small
// twinkle for a small moment wants.
const mciAlias = "pocketpetal"

// PlayFile plays an audio file without waiting for it to finish.
//
// The file is opened as an MPEG file first — which is what an MP3 is, and what tells
// MCI to reach for the DirectShow filter rather than the CD player — and then as plain
// audio, so that a WAV would work here too. If it cannot be opened at all the answer
// is false and the caller falls back to something it can always play.
func PlayFile(path string) bool {
	if path == "" {
		return false
	}
	mciMu.Lock()
	defer mciMu.Unlock()
	// whatever was playing is over: it has been replaced by this
	mci("close " + mciAlias)
	if !mci(fmt.Sprintf("open \"%s\" type mpegvideo alias %s", path, mciAlias)) &&
		!mci(fmt.Sprintf("open \"%s\" alias %s", path, mciAlias)) {
		return false
	}
	if !mci("play " + mciAlias) {
		mci("close " + mciAlias)
		return false
	}
	return true
}

// mci runs one MCI command string and says whether it was understood.
func mci(command string) bool {
	cmd, err := syscall.UTF16PtrFromString(command)
	if err != nil {
		return false
	}
	ret, _, _ := pMciSendString.Call(uintptr(unsafe.Pointer(cmd)), 0, 0, 0)
	return ret == 0
}
