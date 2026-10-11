package win

// The mouse, as far as the flower is concerned: it wants to know where the
// cursor is, it wants to keep receiving messages while the button is down even if
// the cursor runs ahead of the little window it belongs to, and it wants to say
// what the pointer means when it is over something.

// The cursors the app shows, loaded from the system. A window with no cursor of
// its own gets the plain arrow, which says nothing about whether the thing under
// the pointer can be clicked or picked up — so every window that has something to
// do answers WM_SETCURSOR with one of these.
//
// They are the system's own cursors on purpose: whatever the machine is set to use
// for "this can be clicked" is the right one to use here, whether that is the
// default arrow-with-a-hand or something the user chose.
var (
	// CursorArrow is the plain pointer: nothing under it to do.
	CursorArrow = loadCursor(IDC_ARROW)
	// CursorHand is the pointing finger: this can be clicked, or picked up and
	// dragged. It is the cursor every interactive part of the app shows.
	CursorHand = loadCursor(IDC_HAND)
)

// loadCursor fetches one of the system cursors by its resource id. A shared cursor
// returned by LoadCursorW must not be destroyed, which is why nothing here does.
func loadCursor(id uintptr) uintptr {
	h, _, _ := pLoadCursorW.Call(0, id)
	return h
}

// SetCursor sets the pointer for the window being drawn through. Windows resets it
// after every WM_SETCURSOR that is not answered, so this only holds when the
// window answers the message (see windowProc).
func SetCursor(h uintptr) {
	if h != 0 {
		pSetCursor.Call(h)
	}
}

// HitTest is the code a WM_SETCURSOR message carries in the low word of its lparam:
// which part of the window the pointer is over. HTCLIENT is the inside of it — the
// only part these windows ever draw on.
func HitTest(lparam LPARAM) uint16 { return uint16(lparam) }

// SetCapture routes mouse messages to a window until the capture is released.
func SetCapture(hwnd HWND) { pSetCapture.Call(uintptr(hwnd)) }

// ReleaseCapture gives the mouse back to the system.
func ReleaseCapture() { pReleaseCapture.Call() }

// Pressed reports whether the left mouse button is down right now. During a drag
// this is the truth-teller: messages can be missed, the button state cannot.
func Pressed() bool {
	ret, _, _ := pGetAsyncKeyState.Call(0x01 /* VK_LBUTTON */)
	return ret&0x8000 != 0
}
