package win

// Other people's windows: which one is under the flower, how to bring it
// forward, and how to say something into it.
//
// This is the half of the app that is not the flower. Windows only lets the
// foreground process take the foreground, so delivering a note is a small
// diplomacy problem: ask nicely (attach the threads), then less nicely (tap a
// modifier, which grants the right), then rudely (minimise and restore). The
// note itself is pasted rather than typed, so apostrophes, accents and emoji
// arrive intact in a chat box that may be a browser, an IDE panel or an
// Electron app — none of which we have to understand.

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

// WindowInfo is what we can tell about someone else's window.
type WindowInfo struct {
	Hwnd    HWND
	Pid     uint32
	Title   string
	Class   string
	Process string // image name without the extension, e.g. "chrome"
	Rect    RECT
}

// Width and Height of a rectangle, which Windows gives us as two corners.
func (r RECT) Width() int  { return int(r.Right - r.Left) }
func (r RECT) Height() int { return int(r.Bottom - r.Top) }

// Contains reports whether a point is inside the rectangle.
func (r RECT) Contains(x, y int) bool {
	return x >= int(r.Left) && x < int(r.Right) && y >= int(r.Top) && y < int(r.Bottom)
}

// OurPid is this process, so we can ignore our own windows when looking at what
// is under the cursor — the flower and its highlighter are always in the way.
func OurPid() uint32 {
	ret, _, _ := pGetCurrentProcessId.Call()
	return uint32(ret)
}

// Foreground is the window with the focus, if any.
func Foreground() HWND {
	ret, _, _ := pGetForegroundWindow.Call()
	return HWND(ret)
}

// Describe reads everything we need about a window, following it up to its
// top-level ancestor: the handle under the cursor is usually a child.
func Describe(hwnd HWND) (WindowInfo, bool) {
	if hwnd == 0 {
		return WindowInfo{}, false
	}
	root, _, _ := pGetAncestor.Call(uintptr(hwnd), GA_ROOT)
	if root != 0 {
		hwnd = HWND(root)
	}
	var pid uint32
	pGetWindowThreadProcessId.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&pid)))
	info := WindowInfo{
		Hwnd:    hwnd,
		Pid:     pid,
		Title:   windowText(hwnd),
		Class:   windowClass(hwnd),
		Process: processName(pid),
	}
	var r RECT
	if ret, _, _ := pGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&r))); ret == 0 {
		return info, false
	}
	info.Rect = r
	return info, true
}

func windowText(hwnd HWND) string {
	buf := make([]uint16, 512)
	n, _, _ := pGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

func windowClass(hwnd HWND) string {
	buf := make([]uint16, 256)
	n, _, _ := pGetClassNameW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

func processName(pid uint32) string {
	if pid == 0 {
		return ""
	}
	h, _, _ := pOpenProcess.Call(PROCESS_QUERY_LIMITED_INFORMATION, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer pCloseHandle.Call(h)
	buf := make([]uint16, syscall.MAX_PATH)
	size := uint32(len(buf))
	ret, _, _ := pQueryFullProcessImageNam.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if ret == 0 {
		return ""
	}
	full := syscall.UTF16ToString(buf[:size])
	if i := strings.LastIndexAny(full, `\/`); i >= 0 {
		full = full[i+1:]
	}
	return strings.TrimSuffix(strings.ToLower(full), ".exe")
}

// WindowAt is the topmost window under a point that is not ours.
//
// WindowFromPoint is not enough while the flower is being carried: the flower is
// always on top and follows the cursor, so it is what you hit — the chat behind
// it would never be named, never glow, and never receive the note. So walk the
// z-order (EnumWindows runs top to bottom) and skip our own windows.
func WindowAt(x, y int, skipPid uint32) (WindowInfo, bool) {
	point := POINT{int32(x), int32(y)}
	if hwnd, _, _ := pWindowFromPoint.Call(packPoint(point)); hwnd != 0 {
		if info, ok := Describe(HWND(hwnd)); ok && info.Pid != skipPid && interesting(info) {
			return info, true
		}
	}
	return walkZOrder(x, y, skipPid)
}

// AboveWindow lists the windows sitting above a window in the z-order, nearest
// first. It is how the app answers "why can I not see the flower?" — the answer is
// always written here.
func AboveWindow(hwnd HWND, limit int) []string {
	out := []string{}
	h := hwnd
	for i := 0; i < limit; i++ {
		next, _, _ := pGetWindow.Call(uintptr(h), GW_HWNDPREV)
		if next == 0 {
			break
		}
		h = HWND(next)
		info, _ := Describe(h)
		shown, _, _ := pIsWindowVisible.Call(uintptr(h))
		out = append(out, fmt.Sprintf("%s (%q, visible=%v)", info.Class, info.Title, shown != 0))
	}
	return out
}

// TopWindowAt is exactly what WindowFromPoint says: the window the mouse would
// land on at a point, with no filtering of ours or anybody else's. The app asks
// this when it needs to know that the flower is really on screen.
func TopWindowAt(x, y int) (HWND, string) {
	hwnd, _, _ := pWindowFromPoint.Call(packPoint(POINT{int32(x), int32(y)}))
	if hwnd == 0 {
		return 0, ""
	}
	info, _ := Describe(HWND(hwnd))
	return HWND(hwnd), info.Class
}

// packPoint hands a POINT to a function that takes it by value: Windows passes
// an eight-byte struct in one register, x in the low half, y in the high.
func packPoint(p POINT) uintptr {
	return uintptr(uint32(p.X)) | uintptr(uint32(p.Y))<<32
}

// notAWindow is the shell itself: the desktop, the taskbar, notification area,
// tooltips, the SysShadow rectangles Windows keeps behind menus. None of them is
// somewhere a person would drop a flower, and a couple of them cover the whole
// screen — which is how an empty desktop ends up looking like the biggest drop
// target on the machine.
var notAWindow = map[string]bool{
	"Progman":                         true,
	"WorkerW":                         true,
	"Shell_TrayWnd":                   true,
	"Shell_SecondaryTrayWnd":          true,
	"Shell_InputSwitchTopLevelWindow": true,
	"SysListView32":                   true, // the desktop's icons
	"SysHeader32":                     true,
	"SysShadow":                       true,
	"tooltips_class32":                true,
	"#32768":                          true, // any context menu
	"TaskListThumbnailWnd":            true,
	"Windows.UI.Core.CoreWindow":      true,
	"Windows.UI.Composition.DesktopWindowContentBridge": true,
	"ForegroundStaging":                     true,
	"EdgeUiInputTopWndClass":                true,
	"Windows.Internal.Shell.TabProxyWindow": true,
	"XamlExplorerHostIslandWindow":          true,
}

func interesting(info WindowInfo) bool {
	if notAWindow[info.Class] {
		return false
	}
	if info.Title == "" && info.Process == "explorer" {
		// explorer's own chrome (the taskbar, the desktop host, the alt-tab
		// switcher) is never a target, whatever name its class happens to have
		return false
	}
	return info.Rect.Width() > 0 && info.Rect.Height() > 0
}

var enumState struct {
	sync.Mutex
	pid     uint32
	x, y    int
	found   WindowInfo
	hasFind bool
}

var enumProc = syscall.NewCallback(func(hwnd HWND, _ uintptr) uintptr {
	st := &enumState
	if st.hasFind {
		return 0
	}
	if ret, _, _ := pIsWindowVisible.Call(uintptr(hwnd)); ret == 0 {
		return 1
	}
	if ret, _, _ := pIsIconic.Call(uintptr(hwnd)); ret != 0 {
		return 1
	}
	info, ok := Describe(hwnd)
	if !ok || info.Pid == st.pid || !interesting(info) {
		return 1
	}
	if !info.Rect.Contains(st.x, st.y) {
		return 1
	}
	st.found, st.hasFind = info, true
	return 1
})

func walkZOrder(x, y int, skipPid uint32) (WindowInfo, bool) {
	enumState.Lock()
	defer enumState.Unlock()
	enumState.pid, enumState.x, enumState.y = skipPid, x, y
	enumState.found, enumState.hasFind = WindowInfo{}, false
	pEnumWindows.Call(enumProc, 0)
	return enumState.found, enumState.hasFind
}

// Focus brings a window forward. Returns how it was won: "already", "attach",
// "keytap", "restore", or "failed".
func Focus(hwnd HWND) string {
	if hwnd == 0 {
		return "failed"
	}
	restore, _, _ := pIsIconic.Call(uintptr(hwnd))
	if restore != 0 {
		pShowWindow.Call(uintptr(hwnd), SW_RESTORE)
	}
	if Foreground() == hwnd {
		return "already"
	}

	fore := Foreground()
	foreThread, _, _ := pGetWindowThreadProcessId.Call(uintptr(fore), 0)
	targetThread, _, _ := pGetWindowThreadProcessId.Call(uintptr(hwnd), 0)

	if foreThread != 0 && targetThread != 0 && foreThread != targetThread {
		pAttachThreadInput.Call(foreThread, targetThread, 1)
		pShowWindow.Call(uintptr(hwnd), SW_SHOW)
		pBringWindowToTop.Call(uintptr(hwnd))
		pSetForegroundWindow.Call(uintptr(hwnd))
		pAttachThreadInput.Call(foreThread, targetThread, 0)
		if Foreground() == hwnd {
			return "attach"
		}
	} else {
		pShowWindow.Call(uintptr(hwnd), SW_SHOW)
		pBringWindowToTop.Call(uintptr(hwnd))
		pSetForegroundWindow.Call(uintptr(hwnd))
		if Foreground() == hwnd {
			return "direct"
		}
	}

	// a bare modifier tap is enough to be granted the foreground right
	KeyPress(VK_CONTROL)
	pBringWindowToTop.Call(uintptr(hwnd))
	pSetForegroundWindow.Call(uintptr(hwnd))
	if Foreground() == hwnd {
		return "keytap"
	}

	pShowWindow.Call(uintptr(hwnd), SW_MINIMIZE)
	pShowWindow.Call(uintptr(hwnd), SW_RESTORE)
	pSetForegroundWindow.Call(uintptr(hwnd))
	if Foreground() == hwnd {
		return "restore"
	}
	return "failed"
}

// TakeFocus brings one of *our own* windows to the foreground.
//
// It is the same diplomacy Focus() uses on somebody else's chat, minus the
// escalation: minimising and restoring our own settings panel to win the
// foreground would make it vanish instead of opening, so if the gentle way does
// not work the panel simply stays where it is and the mouse still works.
func TakeFocus(hwnd HWND) bool {
	if hwnd == 0 {
		return false
	}
	if Foreground() == hwnd {
		return true
	}
	fore := Foreground()
	foreThread, _, _ := pGetWindowThreadProcessId.Call(uintptr(fore), 0)
	targetThread, _, _ := pGetWindowThreadProcessId.Call(uintptr(hwnd), 0)
	attached := foreThread != 0 && targetThread != 0 && foreThread != targetThread
	if attached {
		pAttachThreadInput.Call(foreThread, targetThread, 1)
	}
	pShowWindow.Call(uintptr(hwnd), SW_SHOW)
	pBringWindowToTop.Call(uintptr(hwnd))
	pSetForegroundWindow.Call(uintptr(hwnd))
	if attached {
		pAttachThreadInput.Call(foreThread, targetThread, 0)
	}
	return Foreground() == hwnd
}

// Virtual key codes the app presses. Escape interrupts a generation in
// essentially every chat; Ctrl+V pastes the note; Enter sends it.
const (
	VK_BACK    = 0x08
	VK_TAB     = 0x09
	VK_RETURN  = 0x0d
	VK_SHIFT   = 0x10
	VK_CONTROL = 0x11
	VK_ESCAPE  = 0x1b
	VK_SPACE   = 0x20
	VK_LEFT    = 0x25
	VK_UP      = 0x26
	VK_RIGHT   = 0x27
	VK_DOWN    = 0x28
	VK_C       = 0x43
	VK_V       = 0x56
)

const keyEventFKeyUp = 0x0002

// KeyPress taps one key.
func KeyPress(vk byte) {
	pKeybdEvent.Call(uintptr(vk), 0, 0, 0)
	pKeybdEvent.Call(uintptr(vk), 0, keyEventFKeyUp, 0)
}

// CtrlKey taps a key with Ctrl held — the paste chord.
func CtrlKey(vk byte) {
	pKeybdEvent.Call(VK_CONTROL, 0, 0, 0)
	KeyPress(vk)
	pKeybdEvent.Call(VK_CONTROL, 0, keyEventFKeyUp, 0)
}

// ClickHere clicks the left button where the pointer already is. The human let
// go there a moment ago, so this moves nothing: it hands the caret to whatever
// is under the flower, which is what lets the app type into a chat it has never
// heard of.
func ClickHere() {
	pMouseEvent.Call(MOUSEEVENTF_LEFTDOWN, 0, 0, 0, 0)
	runtime.Gosched()
	pMouseEvent.Call(MOUSEEVENTF_LEFTUP, 0, 0, 0, 0)
}

const (
	MOUSEEVENTF_LEFTDOWN  = 0x0002
	MOUSEEVENTF_LEFTUP    = 0x0004
	MOUSEEVENTF_RIGHTDOWN = 0x0008
	MOUSEEVENTF_RIGHTUP   = 0x0010
)

// Button holds or releases the left button where the pointer is. Used to drive
// a drag from the outside, so the drag path can be tested by a script instead of
// a hand.
func Button(down bool) {
	if down {
		pMouseEvent.Call(MOUSEEVENTF_LEFTDOWN, 0, 0, 0, 0)
		return
	}
	pMouseEvent.Call(MOUSEEVENTF_LEFTUP, 0, 0, 0, 0)
}

// MoveCursor puts the pointer somewhere, which is how a scripted drag steers.
func MoveCursor(x, y int) {
	pSetCursorPos.Call(uintptr(x), uintptr(y))
}

const (
	CF_UNICODETEXT = 13
	GMEM_MOVEABLE  = 0x0042
)

// SetClipboardText puts text on the clipboard so it can be pasted rather than
// typed: the paste route is the one that survives everything a chat box can be.
func SetClipboardText(text string) bool {
	utf16, err := syscall.UTF16FromString(text)
	if err != nil {
		return false
	}
	if ret, _, _ := pOpenClipboard.Call(0); ret == 0 {
		return false
	}
	defer pCloseClipboard.Call()
	if ret, _, _ := pEmptyClipboard.Call(); ret == 0 {
		return false
	}
	size := uintptr(len(utf16) * 2)
	h, _, _ := pGlobalAlloc.Call(GMEM_MOVEABLE, size)
	if h == 0 {
		return false
	}
	ptr, _, _ := pGlobalLock.Call(h)
	if ptr == 0 {
		pGlobalFree.Call(h)
		return false
	}
	// RtlMoveMemory rather than a Go slice over the handle: the block belongs to
	// Windows, so it is copied into rather than pointed at.
	pRtlMoveMemory.Call(ptr, uintptr(unsafe.Pointer(&utf16[0])), size)
	pGlobalUnlock.Call(h)
	if ret, _, _ := pSetClipboardData.Call(CF_UNICODETEXT, h); ret == 0 {
		pGlobalFree.Call(h)
		return false
	}
	return true
}
