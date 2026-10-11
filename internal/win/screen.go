package win

import "unsafe"

// WorkArea is the usable part of a display, in physical pixels.
type WorkArea struct{ Left, Top, Right, Bottom int32 }

// Width and Height are the work area's size, which is what everything that has to
// divide the screen up (a corner, a place somebody picked) asks it for.
func (a WorkArea) Width() int  { return int(a.Right - a.Left) }
func (a WorkArea) Height() int { return int(a.Bottom - a.Top) }

// PrimaryWorkArea asks the shell for the primary monitor's work area: the screen
// minus the taskbar, which is where a corner flower wants to sit.
func PrimaryWorkArea() WorkArea {
	var r RECT
	ret, _, _ := pSystemParametersInfoW.Call(SPI_GETWORKAREA, 0, uintptr(unsafe.Pointer(&r)), 0)
	if ret == 0 {
		return WorkArea{0, 0, int32(GetSystemMetrics(SM_CXSCREEN)), int32(GetSystemMetrics(SM_CYSCREEN))}
	}
	return WorkArea{r.Left, r.Top, r.Right, r.Bottom}
}

// PrimaryScreen is the primary monitor's box, taskbar included. It is what a
// sweep of the whole screen starts from, where PrimaryWorkArea is where the
// flower may stand.
func PrimaryScreen() WorkArea {
	return WorkArea{0, 0, int32(GetSystemMetrics(SM_CXSCREEN)), int32(GetSystemMetrics(SM_CYSCREEN))}
}

// GetSystemMetrics reads one of the SM_* numbers.
func GetSystemMetrics(index int) int {
	ret, _, _ := pGetSystemMetrics.Call(uintptr(index))
	return int(int32(ret))
}

// MakeDPIAware asks Windows for real pixels rather than a scaled, blurry copy.
// Everything the flower draws is 1:1 with the screen after this call.
func MakeDPIAware() {
	if ret, _, _ := pSetProcessDpiAwareness.Call(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2); ret != 0 {
		return
	}
	if ret, _, _ := pSetProcessDPIAware.Call(); ret != 0 {
		return
	}
}

// ScaleForWindow reports a window's DPI scaling: 1.0 at 96 dpi.
func ScaleForWindow(hwnd HWND) float64 {
	if ret, _, _ := pGetDpiForWindow.Call(uintptr(hwnd)); ret > 0 {
		return float64(ret) / 96
	}
	return 1
}

// SystemScale reports the display's own scaling before any window exists to ask about:
// 1.0 at 96 dpi, 1.5 on a laptop panel at 150%. The app is per-monitor DPI aware, so
// this is what the flower is sized by at startup and what a window's own DPI agrees
// with once there is one.
func SystemScale() float64 {
	if ret, _, _ := pGetDpiForSystem.Call(); ret > 0 {
		return float64(ret) / 96
	}
	return 1
}

// CursorPosition is where the mouse is, in physical pixels.
func CursorPosition() POINT {
	var p POINT
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p
}

// Ticks returns the system tick count in milliseconds, the app's clock.
func Ticks() uint64 {
	ret, _, _ := pGetTickCount64.Call()
	return uint64(ret)
}
