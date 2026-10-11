package win

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// BlitFlags is how a frame is composited: ULW_ALPHA for the flower's per-pixel
// alpha, and ULW_OPAQUE to draw the same bitmap without it. It is a variable so
// that which of the two actually reaches the screen can be tested rather than
// assumed.
var BlitFlags = uintptr(ULW_ALPHA) // BlitHDCDst chooses whether the blit is handed the screen DC or a null one.
// UpdateLayeredWindow documents a null destination DC, and a null one is what
// this uses; the knob exists so `cmd/probe` can test the alternative on a
// machine where the documented call composites nothing.
var BlitHDCDst uintptr

// A layered window: a rectangle Windows never draws itself. Every frame the app
// hands it a premultiplied BGRA bitmap and Windows composites it over whatever
// is behind, using the bitmap's own alpha. That one decision gives the flower
// its transparent background, its soft edges and its hit-testing for free —
// the mouse falls through anywhere the picture is empty, so the little window
// is only "in the way" exactly where the flower is.

// Handler answers the messages a window cares about. Returning true means the
// message was handled and the default procedure should not see it.
type Handler func(m MSG) bool

const (
	className = "PocketPetalFlowerWindow"
	// HighlightClass is the glow around the window a dragged flower would land
	// on. It is a class of its own so that it is never mistaken for the flower
	// itself, by us or by anything watching the desktop.
	HighlightClass = "PocketPetalHighlightWindow"
	// BurstClass is the window a delivered flower's petals fly in. It gets its
	// own class too, so nothing that is looking for the flower can mistake an
	// explosion for one.
	BurstClass = "PocketPetalBurstWindow"
)

var (
	classOnce sync.Map // class name -> *sync.Once, one registration per class
	wndProc   = syscall.NewCallback(windowProc)
	handlers  = struct {
		sync.RWMutex
		byHwnd map[HWND]Handler
	}{byHwnd: map[HWND]Handler{}}
)

// Window is a layered window with a bitmap behind it.
type Window struct {
	Hwnd HWND
	W, H int

	memDC HDC
	dib   HBITMAP
	old   HGDIOBJ
	bits  unsafe.Pointer
}

// NewLayeredWindow registers the class (once) and creates a window that is
// always on top, never in the taskbar, never focused, and never a rectangle of
// solid colour.
func NewLayeredWindow(w, h int, handler Handler) (*Window, error) {
	return NewNamedLayeredWindow(w, h, className, handler)
}

// WindowExStyle is the extended style a flower window is created with. It is a
// variable so that a machine where the layered surface composites nothing can be
// tested with other combinations (see cmd/probe) instead of guessed at.
var WindowExStyle = uintptr(WS_EX_LAYERED | WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE)

// NewNamedLayeredWindow is the same window under its own class name, which is
// what lets the flower, its highlighter and its petals be told apart from outside.
func NewNamedLayeredWindow(w, h int, name string, handler Handler) (*Window, error) {
	return newLayered(w, h, name, WindowExStyle, handler)
}

// MenuClass is the class of an open settings menu.
const MenuClass = "PocketPetalMenuWindow"

// NewPopupWindow is a layered window that is allowed to take the focus, which a
// menu needs: the keyboard has to reach it, and losing the focus is how it knows
// to close. Nothing else in this app is that kind of window.
func NewPopupWindow(w, h int, name string, handler Handler) (*Window, error) {
	return newLayered(w, h, name, WS_EX_LAYERED|WS_EX_TOPMOST|WS_EX_TOOLWINDOW, handler)
}

// newLayered is the one window constructor: the same class registration, the same
// bitmap and the same message routing, whatever style the caller needs.
func newLayered(w, h int, name string, exStyle uintptr, handler Handler) (*Window, error) {
	runtime.LockOSThread()

	once, _ := classOnce.LoadOrStore(name, &sync.Once{})
	once.(*sync.Once).Do(func() {
		hinst, _, _ := pGetModuleHandleW.Call(0)
		wc := WNDCLASSEX{
			CbSize:        UINT(unsafe.Sizeof(WNDCLASSEX{})),
			Style:         0,
			LpfnWndProc:   wndProc,
			HInstance:     HINSTANCE(hinst),
			LpszClassName: utf16Ptr(name),
		}
		if atom, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
			panic(fmt.Sprintf("win: RegisterClassExW failed for %s: %v", name, err))
		}
	})

	hinst, _, _ := pGetModuleHandleW.Call(0)

	// A layered window has to be told it is one before the first
	// UpdateLayeredWindow can put anything in it: without a layered attribute
	// Windows keeps the window in "nothing has been painted yet" state, which on
	// screen is a window that exists, is visible, is on top, and shows nothing at
	// all. The attribute is all the way on; the per-pixel alpha comes from the
	// bitmap a moment later.
	_ = hinst
	hwnd, _, err := pCreateWindowExW.Call(
		exStyle,
		uintptr(unsafe.Pointer(utf16Ptr(name))),
		uintptr(unsafe.Pointer(utf16Ptr("Pocket Petal flower"))),
		uintptr(WS_POPUP),
		0, 0, uintptr(w), uintptr(h),
		0, 0, hinst, 0,
	)
	if hwnd == 0 {
		return nil, fmt.Errorf("win: CreateWindowExW failed: %v", err)
	}

	win := &Window{Hwnd: HWND(hwnd)}
	handlers.Lock()
	handlers.byHwnd[win.Hwnd] = handler
	handlers.Unlock()

	// Ask the window how big it actually is rather than trusting the request:
	// on a scaled display Windows may hand back a different number of real pixels
	// than was asked for, and the bitmap has to match the window exactly or the
	// flower ends up resampled and soft.
	win.W, win.H = win.ClientSize()
	if win.W <= 0 || win.H <= 0 {
		win.W, win.H = w, h
	}
	if err := win.allocBitmap(win.W, win.H); err != nil {
		win.Destroy()
		return nil, err
	}
	return win, nil
}

// bitmapHeight chooses the bitmap's row order. A negative height is a top-down
// DIB, which is what the blit wants — row 0 is the top of the picture and the
// pixels can be copied in one pass. It is a variable because a machine whose
// layered surface composites nothing is worth testing with the other order, and
// cmd/probe does exactly that.
var BitmapTopDown = true

func bitmapHeight(height int) int32 {
	if BitmapTopDown {
		return -int32(height)
	}
	return int32(height)
}

// allocBitmap creates the DIB section the flower is rasterised into.
func (w *Window) allocBitmap(width, height int) error {
	w.memDC = HDC(mustCall(pCreateCompatibleDC, 0))
	if w.memDC == 0 {
		return fmt.Errorf("win: CreateCompatibleDC failed")
	}
	bmi := BITMAPINFO{Header: BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		BiWidth:       int32(width),
		BiHeight:      bitmapHeight(height), // see bitmapHeight: the sign decides the row order
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: BI_RGB,
	}}
	var bits unsafe.Pointer
	dib, _, err := pCreateDIBSection.Call(
		uintptr(w.memDC),
		uintptr(unsafe.Pointer(&bmi)),
		DIB_RGB_COLORS,
		uintptr(unsafe.Pointer(&bits)),
		0, 0,
	)
	if dib == 0 {
		return fmt.Errorf("win: CreateDIBSection failed: %v", err)
	}
	w.dib = HBITMAP(dib)
	w.bits = bits
	w.old = HGDIOBJ(mustCall(pSelectObject, uintptr(w.memDC), uintptr(w.dib)))
	return nil
}

// BlitAt takes the flower as straight (non-premultiplied) RGBA, premultiplies
// it and puts it at (x, y) on screen in one call. The alpha is what makes it a
// flower rather than a rectangle.
func (w *Window) BlitAt(x, y int, rgba []uint8, width, height int) error {
	return w.BlitRegionAt(x, y, 0, 0, rgba, width, height)
}

// BlitRegionAt puts one rectangle of a picture on screen, taken from (atX, atY) in
// the window's own bitmap: the rectangle is written there, premultiplied, and the
// layered window is then moved and resized so that exactly that rectangle is what
// shows.
//
// It is what lets a surface be bigger than the picture in it. Windows copies
// psize pixels from pptSrc of the source DC, so a window can be allocated once at
// the size of everything that could ever be drawn in it — the whole flight of an
// explosion — while each frame pays for, and shows, only the part that has
// something in it. Which is the difference between a burst that is cut off by the
// edge of its surface and one that is not.
func (w *Window) BlitRegionAt(x, y, atX, atY int, rgba []uint8, width, height int) error {
	if w.bits == nil {
		return fmt.Errorf("win: no bitmap to blit into")
	}
	if width <= 0 || height <= 0 {
		return fmt.Errorf("win: a %dx%d region is nothing to blit", width, height)
	}
	if len(rgba) < width*height*4 {
		return fmt.Errorf("win: blit of %dx%d needs %d bytes, got %d", width, height, width*height*4, len(rgba))
	}
	if atX < 0 || atY < 0 || atX+width > w.W || atY+height > w.H {
		return fmt.Errorf("win: a %dx%d region at %d,%d does not fit the %dx%d bitmap",
			width, height, atX, atY, w.W, w.H)
	}
	premultiply(unsafe.Slice((*uint8)(w.bits), w.W*w.H*4), w.W, w.H, atX, atY, rgba, width, height)

	ptDst := POINT{int32(x), int32(y)}
	size := SIZE{int32(width), int32(height)}
	ptSrc := POINT{int32(atX), int32(atY)}
	// SourceConstantAlpha has to be 255 even though the per-pixel alpha is what
	// shapes the flower: Windows multiplies the two, and a zero there makes the
	// whole window completely transparent. It reports success and composites
	// nothing, which is exactly how a flower that is drawn, positioned and
	// "visible" ends up on no screen at all.
	blend := BLENDFUNCTION{
		BlendOp:             0,   /* AC_SRC_OVER */
		SourceConstantAlpha: 255, /* the alpha that matters is the bitmap's */
		AlphaFormat:         1,   /* AC_SRC_ALPHA */
	}
	dstArg := uintptr(unsafe.Pointer(&ptDst))
	hdcDst := BlitHDCDst
	if hdcDst != 0 {
		hdc, _, _ := pGetDC.Call(0)
		hdcDst = hdc
		defer pReleaseDC.Call(0, hdc)
	}
	if ret, _, callErr := pUpdateLayeredWindow.Call(
		uintptr(w.Hwnd),
		hdcDst,
		dstArg,
		uintptr(unsafe.Pointer(&size)),
		uintptr(w.memDC),
		uintptr(unsafe.Pointer(&ptSrc)),
		0,
		uintptr(unsafe.Pointer(&blend)),
		BlitFlags,
	); ret == 0 {
		return fmt.Errorf("win: UpdateLayeredWindow(%d,%d %dx%d) failed: %v", x, y, width, height, callErr)
	}
	return nil
}

// premultiply copies a picture into a bitmap of its own size or bigger: Windows
// wants BGRA with the colour multiplied by the alpha, and the destination row the
// picture lands on depends on which way up the DIB is.
func premultiply(dst []uint8, dstW, dstH, atX, atY int, rgba []uint8, width, height int) {
	for row := 0; row < height; row++ {
		y := atY + row
		if !BitmapTopDown {
			y = dstH - 1 - y
		}
		base := y*dstW*4 + atX*4
		for col := 0; col < width; col++ {
			s := (row*width + col) * 4
			d := base + col*4
			a := uint32(rgba[s+3])
			dst[d+0] = uint8(uint32(rgba[s+2]) * a / 255)
			dst[d+1] = uint8(uint32(rgba[s+1]) * a / 255)
			dst[d+2] = uint8(uint32(rgba[s+0]) * a / 255)
			dst[d+3] = uint8(a)
		}
	}
}

// Pixels is the bitmap as Windows holds it, which is what the blit writes into
// and what UpdateLayeredWindow reads back. It exists so a window that is not on
// screen can be told apart from a picture that is transparent.
func (w *Window) Pixels() []uint8 {
	if w.bits == nil {
		return nil
	}
	return unsafe.Slice((*uint8)(w.bits), w.W*w.H*4)
}

// Position is where the window is on screen, in physical pixels.
func (w *Window) Position() (int, int) {
	var r RECT
	pGetWindowRect.Call(uintptr(w.Hwnd), uintptr(unsafe.Pointer(&r)))
	return int(r.Left), int(r.Top)
}

// ClientSize is the window's real client area, in pixels.
func (w *Window) ClientSize() (int, int) {
	var r RECT
	pGetClientRect.Call(uintptr(w.Hwnd), uintptr(unsafe.Pointer(&r)))
	return int(r.Right - r.Left), int(r.Bottom - r.Top)
}

// Rect is the window's box, in physical pixels.
func (w *Window) Rect() RECT {
	var r RECT
	pGetWindowRect.Call(uintptr(w.Hwnd), uintptr(unsafe.Pointer(&r)))
	return r
}

// Raise puts the window back at the very top of the topmost band, without moving
// it, resizing it or taking focus.
//
// Being created topmost is not enough to *stay* on top: another always-on-top
// window (a browser in full screen, a game overlay) that is activated later goes
// above us in the same band, and then the flower is drawn behind somebody else's
// window and simply cannot be found on screen. Re-asserting is what desktop pets
// have always had to do, and it is invisible when nothing is in the way.
func (w *Window) Raise() {
	pSetWindowPos.Call(
		uintptr(w.Hwnd),
		uintptr(HWND_TOPMOST),
		0, 0, 0, 0,
		SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE,
	)
}

// Show makes it visible without taking focus.
func (w *Window) Show() {
	pShowWindow.Call(uintptr(w.Hwnd), SW_SHOWNOACTIVATE)
}

// MoveTo puts the window somewhere else on screen without touching its contents,
// resizing it or taking the focus.
//
// It is the cheap half of what a blit does. UpdateLayeredWindow both moves a
// window and replaces its picture, which is the right trade for a flower that is
// redrawn every frame and the wrong one for a panel being dragged around: the
// picture is not changing, so re-premultiplying a hundred thousand pixels sixty
// times a second to move a rectangle would be the panel's own lag all over again.
func (w *Window) MoveTo(x, y int) {
	pSetWindowPos.Call(
		uintptr(w.Hwnd),
		0,
		uintptr(x), uintptr(y),
		0, 0,
		SWP_NOSIZE|SWP_NOZORDER|SWP_NOACTIVATE,
	)
}

// SetClickThrough makes the window invisible to the mouse while it stays on
// screen: how the flower gets out of its own way when the click that puts the
// caret in someone's chat box has to land *through* it.
func (w *Window) SetClickThrough(on bool) {
	style, _, _ := pGetWindowLongPtrW.Call(uintptr(w.Hwnd), GWL_EXSTYLE)
	next := uintptr(style)
	if on {
		next |= WS_EX_TRANSPARENT
	} else {
		next &^= WS_EX_TRANSPARENT
	}
	pSetWindowLongPtrW.Call(uintptr(w.Hwnd), GWL_EXSTYLE, next)
}

// Hide takes it off the screen without destroying it.
func (w *Window) Hide() {
	pShowWindow.Call(uintptr(w.Hwnd), 0 /* SW_HIDE */)
}

// Visible is whether Windows thinks the window is on screen at all.
//
// It is the difference between "the panel is drawn" and "the panel can be seen",
// which is the thing a recreated window gets wrong: a layered window's bitmap is
// fixed when it is made, so a panel that has changed size is a *new* window, and a
// new window that nothing has shown is one that exists, is on top, has a picture
// in it, and is invisible.
func (w *Window) Visible() bool {
	ret, _, _ := pIsWindowVisible.Call(uintptr(w.Hwnd))
	return ret != 0
}

// Destroy closes the window and frees the bitmap.
func (w *Window) Destroy() {
	if w.memDC != 0 {
		if w.old != 0 {
			pSelectObject.Call(uintptr(w.memDC), uintptr(w.old))
		}
		if w.dib != 0 {
			pDeleteObject.Call(uintptr(w.dib))
		}
		pDeleteDC.Call(uintptr(w.memDC))
		w.memDC, w.dib, w.old, w.bits = 0, 0, 0, nil
	}
	if w.Hwnd != 0 {
		handlers.Lock()
		delete(handlers.byHwnd, w.Hwnd)
		handlers.Unlock()
		pDestroyWindow.Call(uintptr(w.Hwnd))
		w.Hwnd = 0
	}
}

// Pump drains the message queue. It returns false once the app has been asked
// to quit, which is the loop's signal to stop.
func Pump() bool {
	var msg MSG
	for {
		ret, _, _ := pPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 1 /* PM_REMOVE */)
		if ret == 0 {
			return true
		}
		if msg.Message == 0x0012 /* WM_QUIT */ {
			return false
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// Quit ends the message loop.
func Quit() { pPostQuitMessage.Call(0) }

// windowProc is the one window procedure in the program: it looks up the
// window's handler and hands the message over.
func windowProc(hwnd HWND, msg UINT, wparam WPARAM, lparam LPARAM) LRESULT {
	handlers.RLock()
	handler := handlers.byHwnd[hwnd]
	handlers.RUnlock()
	if handler != nil {
		if handler(MSG{Hwnd: hwnd, Message: msg, WParam: wparam, LParam: lparam}) {
			// WM_SETCURSOR is the one message whose answer is inverted from every
			// other: TRUE means "the cursor has been set, leave it alone", and
			// answering zero would let Windows put the class cursor — the plain
			// arrow — back over whatever was just set.
			if msg == WM_SETCURSOR {
				return 1
			}
			return 0
		}
	}
	ret, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), uintptr(wparam), uintptr(lparam))
	return LRESULT(ret)
}

func mustCall(proc *syscall.LazyProc, args ...uintptr) uintptr {
	ret, _, _ := proc.Call(args...)
	return ret
}

// utf16Len is the number of UTF-16 code units in a string, which is what the
// text APIs count in.
func utf16Len(s string) int {
	units, err := syscall.UTF16FromString(s)
	if err != nil {
		return len(s)
	}
	return len(units) - 1 // without the terminator
}

// utf16Ptr is the usual string -> *uint16 conversion for the W API.
func utf16Ptr(s string) *uint16 {
	ptr, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		panic(err)
	}
	return ptr
}

// UTF16Ptr is the exported form, for callers outside this package.
func UTF16Ptr(s string) *uint16 { return utf16Ptr(s) }
