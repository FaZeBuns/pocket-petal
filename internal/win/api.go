package win

import "syscall"

// The Windows API the flower actually uses, declared here rather than pulled in
// as a dependency: user32 for the window, gdi32 for the bitmap it paints into,
// shell32 for the tray icon, kernel32 for the cursor and the clock. Everything
// else the app needs is written by hand in this repo.

// AttachConsole(-1) means "the console that started me, whoever that is": it is the
// parent process's console, not a new window (see AttachConsole in console.go).
const attachParentProcess = ^uintptr(0)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	pRegisterClassExW           = user32.NewProc("RegisterClassExW")
	pUnregisterClassW           = user32.NewProc("UnregisterClassW")
	pCreateWindowExW            = user32.NewProc("CreateWindowExW")
	pDestroyWindow              = user32.NewProc("DestroyWindow")
	pDefWindowProcW             = user32.NewProc("DefWindowProcW")
	pPeekMessageW               = user32.NewProc("PeekMessageW")
	pTranslateMessage           = user32.NewProc("TranslateMessage")
	pDispatchMessageW           = user32.NewProc("DispatchMessageW")
	pPostQuitMessage            = user32.NewProc("PostQuitMessage")
	pUpdateLayeredWindow        = user32.NewProc("UpdateLayeredWindow")
	pSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	pGetCursorPos               = user32.NewProc("GetCursorPos")
	pGetAsyncKeyState           = user32.NewProc("GetAsyncKeyState")
	pSetCapture                 = user32.NewProc("SetCapture")
	pReleaseCapture             = user32.NewProc("ReleaseCapture")
	pGetDC                      = user32.NewProc("GetDC")
	pReleaseDC                  = user32.NewProc("ReleaseDC")
	pGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	pSystemParametersInfoW      = user32.NewProc("SystemParametersInfoW")
	pSetProcessDPIAware         = user32.NewProc("SetProcessDPIAware")
	pSetProcessDpiAwareness     = user32.NewProc("SetProcessDpiAwarenessContext")
	pGetDpiForWindow            = user32.NewProc("GetDpiForWindow")
	pGetDpiForSystem            = user32.NewProc("GetDpiForSystem")
	pTrackPopupMenu             = user32.NewProc("TrackPopupMenu")
	pCreatePopupMenu            = user32.NewProc("CreatePopupMenu")
	pDestroyMenu                = user32.NewProc("DestroyMenu")
	pAppendMenuW                = user32.NewProc("AppendMenuW")
	pGetWindowRect              = user32.NewProc("GetWindowRect")
	pGetClientRect              = user32.NewProc("GetClientRect")
	pSetWindowPos               = user32.NewProc("SetWindowPos")
	pShowWindow                 = user32.NewProc("ShowWindow")
	pCreateIconIndirect         = user32.NewProc("CreateIconIndirect")
	pDestroyIcon                = user32.NewProc("DestroyIcon")
	pMessageBoxW                = user32.NewProc("MessageBoxW")
	pPostMessageW               = user32.NewProc("PostMessageW")
	pSendInput                  = user32.NewProc("SendInput")
	pOpenClipboard              = user32.NewProc("OpenClipboard")
	pCloseClipboard             = user32.NewProc("CloseClipboard")
	pEmptyClipboard             = user32.NewProc("EmptyClipboard")
	pSetClipboardData           = user32.NewProc("SetClipboardData")
	pWindowFromPoint            = user32.NewProc("WindowFromPoint")
	pGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	pGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	pGetClassNameW              = user32.NewProc("GetClassNameW")
	pGetWindowTextW             = user32.NewProc("GetWindowTextW")
	pIsWindowVisible            = user32.NewProc("IsWindowVisible")
	pGetWindowLongPtrW          = user32.NewProc("GetWindowLongPtrW")
	pEnumWindows                = user32.NewProc("EnumWindows")
	pGetWindow                  = user32.NewProc("GetWindow")
	pIsIconic                   = user32.NewProc("IsIconic")
	pGetAncestor                = user32.NewProc("GetAncestor")
	pBringWindowToTop           = user32.NewProc("BringWindowToTop")
	pSetForegroundWindow        = user32.NewProc("SetForegroundWindow")
	pAttachThreadInput          = user32.NewProc("AttachThreadInput")
	pMouseEvent                 = user32.NewProc("mouse_event")
	pSetCursorPos               = user32.NewProc("SetCursorPos")
	pLoadCursorW                = user32.NewProc("LoadCursorW")
	pSetCursor                  = user32.NewProc("SetCursor")
	pAttachConsole              = kernel32.NewProc("AttachConsole")
	pSetWindowLongPtrW          = user32.NewProc("SetWindowLongPtrW")
	pKeybdEvent                 = user32.NewProc("keybd_event")
	pVkKeyScanW                 = user32.NewProc("VkKeyScanW")
	pMapVirtualKeyW             = user32.NewProc("MapVirtualKeyW")

	pCreateFontW           = gdi32.NewProc("CreateFontW")
	pSetTextColor          = gdi32.NewProc("SetTextColor")
	pSetBkColor            = gdi32.NewProc("SetBkColor")
	pSetBkMode             = gdi32.NewProc("SetBkMode")
	pGetTextExtentPoint32W = gdi32.NewProc("GetTextExtentPoint32W")
	pDrawTextW             = user32.NewProc("DrawTextW")

	pCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	pCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	pSelectObject       = gdi32.NewProc("SelectObject")
	pDeleteObject       = gdi32.NewProc("DeleteObject")
	pDeleteDC           = gdi32.NewProc("DeleteDC")
	pCreateBitmap       = gdi32.NewProc("CreateBitmap")

	pShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	pGetTickCount64   = kernel32.NewProc("GetTickCount64")
	pGlobalAlloc      = kernel32.NewProc("GlobalAlloc")
	pGlobalLock       = kernel32.NewProc("GlobalLock")
	pGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
	pGlobalFree       = kernel32.NewProc("GlobalFree")

	pGetCurrentThreadId       = kernel32.NewProc("GetCurrentThreadId")
	pRtlMoveMemory            = kernel32.NewProc("RtlMoveMemory")
	pGetCurrentProcessId      = kernel32.NewProc("GetCurrentProcessId")
	pOpenProcess              = kernel32.NewProc("OpenProcess")
	pQueryFullProcessImageNam = kernel32.NewProc("QueryFullProcessImageNameW")
	pCloseHandle              = kernel32.NewProc("CloseHandle")
)

// Handles and structs, declared to match Windows exactly.

type (
	HWND      uintptr
	HINSTANCE uintptr
	HICON     uintptr
	HCURSOR   uintptr
	HBRUSH    uintptr
	HMENU     uintptr
	HDC       uintptr
	HBITMAP   uintptr
	HGDIOBJ   uintptr
	HANDLE    uintptr
	ATOM      uint16
	UINT      uint32
	DWORD     uint32
	LPCWSTR   *uint16
	LPARAM    uintptr
	WPARAM    uintptr
	LRESULT   uintptr
)

type POINT struct{ X, Y int32 }
type SIZE struct{ CX, CY int32 }
type RECT struct{ Left, Top, Right, Bottom int32 }

type MSG struct {
	Hwnd    HWND
	Message UINT
	WParam  WPARAM
	LParam  LPARAM
	Time    DWORD
	Pt      POINT
}

type WNDCLASSEX struct {
	CbSize        UINT
	Style         UINT
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     HINSTANCE
	HIcon         HICON
	HCursor       HCURSOR
	HbrBackground HBRUSH
	LpszMenuName  LPCWSTR
	LpszClassName LPCWSTR
	HIconSm       HICON
}

type BLENDFUNCTION struct {
	BlendOp             byte
	BlendFlags          byte
	SourceConstantAlpha byte
	AlphaFormat         byte
}

type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

type BITMAPINFO struct {
	Header BITMAPINFOHEADER
	Colors [1]uint32
}

type ICONINFO struct {
	FIcon    int32
	XHotspot DWORD
	YHotspot DWORD
	HbmMask  HBITMAP
	HbmColor HBITMAP
}

type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// NOTIFYICONDATAW is the shell's tray icon record, laid out as Windows 7+ sees
// it (the whole struct, with a ballon icon at the end).
type NOTIFYICONDATAW struct {
	CbSize           DWORD
	Hwnd             HWND
	UID              UINT
	UFlags           UINT
	UCallbackMessage UINT
	HIcon            HICON
	SzTip            [128]uint16
	DwState          DWORD
	DwStateMask      DWORD
	SzInfo           [256]uint16
	UVersion         UINT
	SzInfoTitle      [64]uint16
	DwInfoFlags      DWORD
	GuidItem         GUID
	HBalloonIcon     HICON
}

// Message constants, the few the flower answers.
const (
	WM_DESTROY = 0x0002
	WM_CLOSE   = 0x0010
	WM_PAINT   = 0x000F
	// the ones an open settings menu answers: it takes the focus, so it is told
	// when the focus goes away, which is what closes it
	WM_ACTIVATE      = 0x0006
	WM_KILLFOCUS     = 0x0008
	WM_KEYDOWN       = 0x0100
	WM_MOUSEMOVE     = 0x0200
	WM_LBUTTONDOWN   = 0x0201
	WM_LBUTTONUP     = 0x0202
	WM_RBUTTONUP     = 0x0205
	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONDOWN   = 0x0204
	WM_NCHITTEST     = 0x0084
	// sent before the pointer is drawn, and the one message whose answer is
	// inverted: TRUE means "the cursor has been set, leave it alone"
	WM_SETCURSOR = 0x0020
	HTCLIENT     = 1

	// the system cursors this app has a use for, by resource id
	IDC_ARROW = 32512
	IDC_HAND  = 32649 // the pointing finger: this can be clicked, or picked up

	WM_COMMAND  = 0x0111
	WM_APP      = 0x8000
	WM_TRAYICON = WM_APP + 1

	// the screen changing under the flower: a resolution, a scaling factor, or the
	// taskbar moving. Windows sends all three to every top-level window, which is
	// how the flower hears about a change it can see but cannot ask about.
	WM_SETTINGCHANGE = 0x001A
	WM_DISPLAYCHANGE = 0x007E
	WM_DPICHANGED    = 0x02E0
)

// Window styles and extended styles.
const (
	WS_POPUP          = 0x80000000
	WS_EX_TOPMOST     = 0x00000008
	WS_EX_TOOLWINDOW  = 0x00000080
	WS_EX_LAYERED     = 0x00080000
	WS_EX_NOACTIVATE  = 0x08000000
	WS_EX_TRANSPARENT = 0x00000020

	SW_SHOWNOACTIVATE = 4
	SW_SHOW           = 5
	SW_MINIMIZE       = 6
	SW_RESTORE        = 9
	SWP_NOACTIVATE    = 0x0010
	SWP_NOMOVE        = 0x0002
	SWP_NOSIZE        = 0x0001
	SWP_NOZORDER      = 0x0004

	ULW_ALPHA      = 0x00000002
	ULW_OPAQUE     = 0x00000004
	LWA_ALPHA      = 0x00000002
	DIB_RGB_COLORS = 0
	BI_RGB         = 0
)

// Shell_NotifyIconW messages and flags.
const (
	NIM_ADD        = 0
	NIM_MODIFY     = 1
	NIM_DELETE     = 2
	NIM_SETVERSION = 4
	NIF_MESSAGE    = 0x00000001
	NIF_ICON       = 0x00000002
	NIF_TIP        = 0x00000004
	NIF_SHOWTIP    = 0x00000080
)

// System metrics and shell queries.
const (
	SM_CXSCREEN     = 0
	SM_CYSCREEN     = 1
	SPI_GETWORKAREA = 0x0030

	TPM_RIGHTBUTTON = 0x0002
	TPM_RETURNCMD   = 0x0100

	DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 = ^uintptr(3) // (HANDLE)-4

	GA_ROOT                           = 2
	GW_HWNDPREV                       = 3
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
)

// HWND_TOPMOST is (HWND)-1, and GWL_EXSTYLE is -20 as an unsigned index.
var (
	HWND_TOPMOST = ^HWND(0)
	GWL_EXSTYLE  = ^uintptr(19)
)

// copyUTF16 writes a string into a fixed W buffer, NUL terminated.
func copyUTF16(dst []uint16, s string) {
	n := len(dst) - 1
	for i, r := range s {
		if i >= n {
			break
		}
		dst[i] = uint16(r)
	}
	dst[len(dst)-1] = 0
}
