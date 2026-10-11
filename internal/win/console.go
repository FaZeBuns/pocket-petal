package win

import (
	"os"
	"syscall"
)

// A console, for the times one is wanted.
//
// The app is built as a *windowed* application (golang.org/x/sys/windows would say
// GUI subsystem; the linker flag is -H=windowsgui), which is what stops Windows from
// giving it a console window of its own: a flower on the desktop has no more business
// owning a black box full of nothing than a screensaver has. The cost of that is that
// the few flags that print something — -version, -trace, -dry-run, -stats — would
// print into nothing at all.
//
// So the console is borrowed when, and only when, something is going to be read.
// AttachConsole takes the console of whoever started the process, if there is one:
// launched from a terminal the output appears there, and launched from the Explorer
// there is nothing to attach to and the call simply fails. It is deliberately not
// done unconditionally — an attached process dies with the console that owns it, and
// a flower that quit when somebody closed their terminal would be a worse bug than
// the one this is fixing.
func AttachConsole() bool {
	if ret, _, _ := pAttachConsole.Call(attachParentProcess); ret == 0 {
		return false
	}
	h, err := syscall.Open("CONOUT$", syscall.O_WRONLY, 0)
	if err != nil {
		return false
	}
	f := os.NewFile(uintptr(h), "CONOUT$")
	os.Stdout, os.Stderr = f, f
	return true
}
