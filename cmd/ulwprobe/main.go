// Command ulwprobe is a self-contained diagnostic for putting one solid
// rectangle on screen through UpdateLayeredWindow, with every argument written
// out by hand so that nothing in internal/win can hide a mistake.
//
// Each variant changes one thing and prints what Windows said, because "the
// surface composites nothing" and "UpdateLayeredWindow refused the call" look
// identical on screen and are completely different problems.
//
//	go run ./cmd/ulwprobe -variant 1
package main

import (
	"flag"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	gdi32  = syscall.NewLazyDLL("gdi32.dll")

	pRegisterClassExW     = user32.NewProc("RegisterClassExW")
	pCreateWindowExW      = user32.NewProc("CreateWindowExW")
	pDestroyWindow        = user32.NewProc("DestroyWindow")
	pDefWindowProcW       = user32.NewProc("DefWindowProcW")
	pShowWindow           = user32.NewProc("ShowWindow")
	pSetWindowPos         = user32.NewProc("SetWindowPos")
	pUpdateLayeredWindow  = user32.NewProc("UpdateLayeredWindow")
	pSetLayeredWindowAttr = user32.NewProc("SetLayeredWindowAttributes")
	pGetDC                = user32.NewProc("GetDC")
	pGetWindowRect        = user32.NewProc("GetWindowRect")
	pGetClientRect        = user32.NewProc("GetClientRect")
	pGetWindowLongPtrW    = user32.NewProc("GetWindowLongPtrW")
	pSetWindowLongPtrW    = user32.NewProc("SetWindowLongPtrW")
	pGetDpiForWindow      = user32.NewProc("GetDpiForWindow")
	pSetProcessDpiCtx     = user32.NewProc("SetProcessDpiAwarenessContext")
	pSystemParametersInfo = user32.NewProc("SystemParametersInfoW")
	pPeekMessageW         = user32.NewProc("PeekMessageW")

	pCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	pCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	pSelectObject       = gdi32.NewProc("SelectObject")

	kernel32      = syscall.NewLazyDLL("kernel32.dll")
	pGetModuleH   = kernel32.NewProc("GetModuleHandleW")
	pGetLastError = kernel32.NewProc("GetLastError")
)

type point struct{ x, y int32 }
type size struct{ cx, cy int32 }
type rect struct{ left, top, right, bottom int32 }
type blendFunction struct {
	op, flags, alpha, format byte
}
type bitmapInfoHeader struct {
	size, width, height            uint32
	planes, bitCount               uint16
	compression                    uint32
	sizeImage, xppm, yppm, clrUsed uint32
	clrImportant                   uint32
}
type bitmapInfo struct {
	header bitmapInfoHeader
	colors uint32
}
type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}
type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	private uint32
}

func wndProc(hwnd, m, w, l uintptr) uintptr {
	ret, _, _ := pDefWindowProcW.Call(hwnd, m, w, l)
	return ret
}

func wstr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func main() {
	x := flag.Int("x", 300, "left edge")
	y := flag.Int("y", 200, "top edge")
	w := flag.Int("w", 300, "width")
	h := flag.Int("h", 300, "height")
	seconds := flag.Int("seconds", 10, "how long to stay")
	variant := flag.Int("variant", 1, "1 plain, 2 with SetLayeredWindowAttributes first, 3 screen destination DC, 4 no extended styles beyond layered, 5 window moved first")
	flag.Parse()

	// Per-monitor v2 awareness, so every pixel number below is a real pixel.
	if r, _, err := pSetProcessDpiCtx.Call(^uintptr(3)); r == 0 {
		fmt.Println("probe: SetProcessDpiAwarenessContext failed:", err)
	}
	runtime.LockOSThread()

	hinst, _, _ := pGetModuleH.Call(0)
	proc := syscall.NewCallback(wndProc)
	class := wstr("PocketPetalUlwProbe")
	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   proc,
		hInstance:     hinst,
		lpszClassName: class,
	}
	if atom, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		fmt.Println("probe: RegisterClassExW failed:", err)
		return
	}

	const (
		wsPopup        = 0x80000000
		wsExLayered    = 0x00080000
		wsExTopmost    = 0x00000008
		wsExToolWindow = 0x00000080
		wsExNoActivate = 0x08000000
		swShowNoAct    = 4
		ulwAlpha       = 0x00000002
		dibRGBColors   = 0
		lwaAlpha       = 0x00000002
		hWndTopmost    = ^uintptr(0)
		swpNoMove      = 0x0002
		swpNoSize      = 0x0001
		swpNoActivate  = 0x0010
		gwlExStyle     = ^uintptr(19)
		spiGetWorkArea = 0x0030
	)

	exStyle := uintptr(wsExLayered | wsExTopmost | wsExToolWindow | wsExNoActivate)
	if *variant == 4 {
		exStyle = wsExLayered
	}
	hwnd, _, err := pCreateWindowExW.Call(
		exStyle,
		uintptr(unsafe.Pointer(class)),
		uintptr(unsafe.Pointer(wstr("probe"))),
		wsPopup,
		uintptr(*x), uintptr(*y), uintptr(*w), uintptr(*h),
		0, 0, hinst, 0,
	)
	if hwnd == 0 {
		fmt.Println("probe: CreateWindowExW failed:", err)
		return
	}
	defer pDestroyWindow.Call(hwnd)

	got, _, _ := pGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	var work rect
	pSystemParametersInfo.Call(spiGetWorkArea, 0, uintptr(unsafe.Pointer(&work)), 0)
	dpi, _, _ := pGetDpiForWindow.Call(hwnd)
	var wr, cr rect
	pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))
	fmt.Printf("probe: variant %d hwnd=%d exstyle=0x%x layered=%v dpi=%d\n", *variant, hwnd, got, got&wsExLayered != 0, dpi)
	fmt.Printf("probe: work area %d,%d %dx%d  window rect %d,%d %dx%d (asked %d,%d)  client %dx%d\n",
		work.left, work.top, work.right-work.left, work.bottom-work.top,
		wr.left, wr.top, wr.right-wr.left, wr.bottom-wr.top, *x, *y, cr.right-cr.left, cr.bottom-cr.top)

	if *variant == 2 {
		if r, _, e := pSetLayeredWindowAttr.Call(hwnd, 0, 255, lwaAlpha); r == 0 {
			fmt.Println("probe: SetLayeredWindowAttributes failed:", e)
		} else {
			fmt.Println("probe: SetLayeredWindowAttributes(LWA_ALPHA) done")
		}
	}

	memDC, _, _ := pCreateCompatibleDC.Call(0)
	bmi := bitmapInfo{header: bitmapInfoHeader{
		size:        uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		width:       uint32(*w),
		height:      uint32(int32(-*h)), // top-down
		planes:      1,
		bitCount:    32,
		compression: 0,
	}}
	var bits unsafe.Pointer
	dib, _, derr := pCreateDIBSection.Call(memDC, uintptr(unsafe.Pointer(&bmi)), dibRGBColors, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if dib == 0 {
		fmt.Println("probe: CreateDIBSection failed:", derr)
		return
	}
	pSelectObject.Call(memDC, dib)
	pix := unsafe.Slice((*uint8)(bits), *w**h*4)
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = 255, 0, 255, 255 // BGRA magenta
	}
	fmt.Printf("probe: bitmap %dx%d at %p, first pixel %d,%d,%d,%d\n", *w, *h, bits, pix[0], pix[1], pix[2], pix[3])

	if *variant == 5 {
		pSetWindowPos.Call(hwnd, hWndTopmost, uintptr(*x), uintptr(*y), uintptr(*w), uintptr(*h), 0)
	}

	ptDst := point{int32(*x), int32(*y)}
	sz := size{int32(*w), int32(*h)}
	ptSrc := point{0, 0}
	blend := blendFunction{op: 0 /* AC_SRC_OVER */, flags: 0, alpha: 255, format: 1 /* AC_SRC_ALPHA */}
	hdcDst := uintptr(0)
	if *variant == 3 {
		hdcDst, _, _ = pGetDC.Call(0)
		fmt.Println("probe: blitting with the screen DC as the destination")
	}

	blit := func() error {
		r, _, e := pUpdateLayeredWindow.Call(
			hwnd, hdcDst,
			uintptr(unsafe.Pointer(&ptDst)), uintptr(unsafe.Pointer(&sz)),
			memDC, uintptr(unsafe.Pointer(&ptSrc)),
			0, uintptr(unsafe.Pointer(&blend)), ulwAlpha,
		)
		if r == 0 {
			return fmt.Errorf("UpdateLayeredWindow failed: %v (GetLastError=%d)", e, lastError())
		}
		return nil
	}

	if err := blit(); err != nil {
		fmt.Println("probe: before show:", err)
	} else {
		fmt.Println("probe: UpdateLayeredWindow succeeded before show")
	}
	pShowWindow.Call(hwnd, swShowNoAct)
	if err := blit(); err != nil {
		fmt.Println("probe: after show:", err)
	} else {
		fmt.Println("probe: UpdateLayeredWindow succeeded after show")
	}
	pGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
	fmt.Printf("probe: after blit, window rect %d,%d %dx%d\n", wr.left, wr.top, wr.right-wr.left, wr.bottom-wr.top)

	deadline := time.Now().Add(time.Duration(*seconds) * time.Second)
	for time.Now().Before(deadline) {
		var m msg
		for more, _, _ := pPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, 1); more != 0; more, _, _ = pPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, 1) {
		}
		if err := blit(); err != nil {
			fmt.Println("probe: loop blit failed:", err)
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	fmt.Println("probe: done")
}

func lastError() uint32 {
	r, _, _ := pGetLastError.Call()
	return uint32(r)
}
