// Command probe is a diagnostic: it puts one solid-coloured layered window on
// screen and holds it there, which is how "the flower is not on screen" is told
// apart from "the flower is drawn behind something" or "the picture is
// transparent".
//
//	go run ./cmd/probe -x 400 -y 300 -w 200 -h 200 -seconds 8
package main

import (
	"flag"
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"github.com/FaZeBuns/pocket-petal/internal/win"
)

// paint draws a solid rectangle into the window with plain GDI: the other half
// of the diagnostic, used when a layered window shows nothing. If this appears
// and the layered blit does not, the window is fine and the compositing is not.
func paint(hwnd uintptr, w, h int) {
	user32 := syscall.NewLazyDLL("user32.dll")
	gdi32 := syscall.NewLazyDLL("gdi32.dll")
	getDC := user32.NewProc("GetDC")
	releaseDC := user32.NewProc("ReleaseDC")
	fillRect := user32.NewProc("FillRect")
	createSolidBrush := gdi32.NewProc("CreateSolidBrush")
	deleteObject := gdi32.NewProc("DeleteObject")

	brush, _, _ := createSolidBrush.Call(0x00ff00ff) // COLORREF is 0x00BBGGRR
	hdc, _, _ := getDC.Call(hwnd)
	rect := [4]int32{0, 0, int32(w), int32(h)}
	fillRect.Call(hdc, uintptr(unsafe.Pointer(&rect)), brush)
	releaseDC.Call(hwnd, hdc)
	deleteObject.Call(brush)
}

func main() {
	x := flag.Int("x", 400, "left edge")
	y := flag.Int("y", 300, "top edge")
	w := flag.Int("w", 200, "width")
	h := flag.Int("h", 200, "height")
	seconds := flag.Int("seconds", 8, "how long to stay on screen")
	mode := flag.String("mode", "alpha", "alpha (per-pixel alpha) or opaque (the alpha ignored)")
	flag.Parse()
	switch *mode {
	case "bottom-up":
		// the conventional DIB row order, in case the compositor wants it
		win.BitmapTopDown = false
	case "styles-min":
		// layered and on top, nothing else
		win.WindowExStyle = uintptr(0x00080000 | 0x00000008)
	case "styles-layered":
		win.WindowExStyle = uintptr(0x00080000)
	case "opaque":
		win.BlitFlags = uintptr(win.ULW_OPAQUE)
	case "screendc":
		win.BlitHDCDst = 1
	}

	win.MakeDPIAware()
	rgba := make([]uint8, *w**h*4)
	for i := 0; i < len(rgba); i += 4 {
		rgba[i], rgba[i+1], rgba[i+2], rgba[i+3] = 255, 0, 255, 255
	}

	window, err := win.NewLayeredWindow(*w, *h, func(win.MSG) bool { return false })
	if err != nil {
		fmt.Println("probe:", err)
		return
	}
	defer window.Destroy()
	if *mode == "hide-first" {
		// the order every sample uses: composite the surface, *then* show the
		// window, rather than showing an empty one and filling it afterwards
		if err := window.BlitAt(*x, *y, rgba, *w, *h); err != nil {
			fmt.Println("probe:", err)
		}
		window.Show()
		window.BlitAt(*x, *y, rgba, *w, *h)
	} else {
		window.Show()
	}
	if *mode == "paint" || *mode == "paint+blit" {
		window.Raise()
		paint(uintptr(window.Hwnd), *w, *h)
		fmt.Println("probe: painted the window with GDI instead of compositing a bitmap")
		if *mode == "paint+blit" {
			// if the paint disappears here, the composited bitmap replaced it: the
			// blit is being applied, and it is arriving fully transparent
			if err := window.BlitAt(*x, *y, rgba, *w, *h); err != nil {
				fmt.Println("probe:", err)
			}
		}
	} else if err := window.BlitAt(*x, *y, rgba, *w, *h); err != nil {
		fmt.Println("probe:", err)
		return
	}
	if px := window.Pixels(); len(px) >= 8 {
		fmt.Printf("probe: bitmap holds %d,%d,%d,%d at its first pixel\n", px[0], px[1], px[2], px[3])
	}
	r := window.Rect()
	top, class := win.TopWindowAt(*x+*w/2, *y+*h/2)
	fmt.Printf("probe: window %d,%d %dx%d (asked %d,%d), on top there: %s (ours=%v)\n",
		r.Left, r.Top, r.Width(), r.Height(), *x, *y, class, top == window.Hwnd)

	deadline := time.Now().Add(time.Duration(*seconds) * time.Second)
	for time.Now().Before(deadline) {
		if !win.Pump() {
			break
		}
		switch *mode {
		case "paint":
			paint(uintptr(window.Hwnd), *w, *h)
		case "paint+blit":
			window.BlitAt(*x, *y, rgba, *w, *h)
		default:
			window.Raise()
			window.BlitAt(*x, *y, rgba, *w, *h)
		}
		time.Sleep(16 * time.Millisecond)
	}
	window.Hide()
}
