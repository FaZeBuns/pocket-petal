package win

import (
	"errors"
	"strings"
	"syscall"
	"unsafe"
)

var pShellExecuteW = shell32.NewProc("ShellExecuteW")

const swShowNormal = 1

// OpenURL hands an http(s) address to the shell, which opens it in the user's default
// browser. Anything that is not a web address is refused, so it can never be made to
// launch a file.
func OpenURL(url string) error {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return errors.New("not a web address: " + url)
	}
	verb, err := syscall.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	target, err := syscall.UTF16PtrFromString(url)
	if err != nil {
		return err
	}
	// ShellExecute returns a value greater than 32 on success
	r, _, _ := pShellExecuteW.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(target)), 0, 0, swShowNormal)
	if r <= 32 {
		return errors.New("the shell would not open " + url)
	}
	return nil
}
