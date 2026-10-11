package win

import (
	"fmt"
	"unsafe"
)

// Text, as a coverage mask.
//
// The flower's windows are per-pixel-alpha surfaces, and GDI cannot draw into one:
// it writes colour and no alpha, so anything it paints comes out invisible once the
// surface is composited. What GDI *can* do is draw into a plain 32bpp bitmap, so
// that is what this does — the text is rendered white on black into a scratch
// bitmap, and the brightness of each pixel is read back as "how much of this pixel
// is ink". The caller then blends its own colour into its own RGBA buffer with that
// coverage, which keeps the alpha honest and lets the app's own drawing and real
// type share one surface.
//
// Greyscale antialiasing is requested on purpose: ClearType would fringe the mask
// with colour, and a mask has no colour of its own.

// Font is a real GDI font, sized for the screen.
//
// It remembers the masks it has made. A settings panel is drawn dozens of times a
// second while somebody drags a slider through it, and every one of those frames
// wants the same twenty labels: rendering them again through a device context and
// a fresh bitmap, at a few hundred microseconds each, was most of what made the
// panel feel slow. The cache is bounded, and the masks in it are shared — a caller
// may read one, and must not draw into it.
type Font struct {
	handle uintptr
	Height int

	masks map[maskKey][]uint8
	order []maskKey // insertion order, so the oldest can be dropped
}

// maskKey is what makes two masks the same mask.
type maskKey struct {
	text          string
	width, height int
}

// maskCache is how many lines of text one font remembers. The panel's pages are a
// few dozen rows of short strings, and a slider's value changes as it is dragged:
// this holds a page, the page next to it, and every value the last few frames
// showed.
const maskCache = 256

// FontWeight is the weight a font is created with.
const (
	FontRegular = 400
	FontMedium  = 500
	FontBold    = 600
)

// NewFont creates a font of the given pixel height. It uses the shell's own UI
// face, so the menu reads as part of the machine rather than as part of the app.
func NewFont(height, weight int) (*Font, error) {
	if height < 8 {
		height = 8
	}
	const (
		charset      = 1 // DEFAULT_CHARSET
		outTTPrecis  = 4 // OUT_TT_PRECIS
		clipPrecis   = 0 // CLIP_DEFAULT_PRECIS
		quality      = 4 // ANTIALIASED_QUALITY: greyscale, which a mask needs
		pitchFamily  = 0 // DEFAULT_PITCH | FF_DONTCARE
		defaultWidth = 0
	)
	face := utf16Ptr("Segoe UI")
	r, _, err := pCreateFontW.Call(
		uintptr(-int32(height)), // negative: character height, not cell height
		defaultWidth,
		0, 0, // escapement, orientation
		uintptr(weight),
		0, 0, 0, // italic, underline, strikeout
		charset, outTTPrecis, clipPrecis, quality, pitchFamily,
		uintptr(unsafe.Pointer(face)),
	)
	if r == 0 {
		return nil, fmt.Errorf("win: CreateFontW: %v", err)
	}
	return &Font{handle: r, Height: height}, nil
}

// Close frees the font.
func (f *Font) Close() {
	if f != nil && f.handle != 0 {
		pDeleteObject.Call(f.handle)
		f.handle = 0
	}
}

// Measure is the width of a line of text in this font, and its height.
func (f *Font) Measure(text string) (int, int) {
	if f == nil || f.handle == 0 || text == "" {
		return 0, 0
	}
	dc := HDC(mustCall(pCreateCompatibleDC, 0))
	if dc == 0 {
		return 0, 0
	}
	defer pDeleteDC.Call(uintptr(dc))
	old := mustCall(pSelectObject, uintptr(dc), f.handle)
	sz := SIZE{}
	pGetTextExtentPoint32W.Call(
		uintptr(dc),
		uintptr(unsafe.Pointer(utf16Ptr(text))),
		uintptr(utf16Len(text)),
		uintptr(unsafe.Pointer(&sz)),
	)
	pSelectObject.Call(uintptr(dc), old)
	return int(sz.CX), int(sz.CY)
}

// Mask renders one line of text as coverage: one byte per pixel, 0 for nothing and
// 255 for full ink, in a buffer width x height. The text is drawn from the left,
// vertically centred.
func (f *Font) Mask(text string, width, height int) []uint8 {
	if f == nil || f.handle == 0 || width <= 0 || height <= 0 || text == "" {
		return make([]uint8, max(0, width)*max(0, height))
	}
	key := maskKey{text: text, width: width, height: height}
	if f.masks != nil {
		if got, ok := f.masks[key]; ok {
			return got
		}
	}
	out := f.renderMask(text, width, height)
	f.keep(key, out)
	return out
}

// keep files a rendered mask, dropping the oldest when the cache is full.
func (f *Font) keep(key maskKey, mask []uint8) {
	if f.masks == nil {
		f.masks = map[maskKey][]uint8{}
	}
	if _, ok := f.masks[key]; ok {
		return
	}
	if len(f.order) >= maskCache {
		delete(f.masks, f.order[0])
		f.order = append(f.order[:0], f.order[1:]...)
	}
	f.masks[key] = mask
	f.order = append(f.order, key)
}

// renderMask does the work: one device context, one bitmap, one line of text.
func (f *Font) renderMask(text string, width, height int) []uint8 {
	out := make([]uint8, width*height)
	dc := HDC(mustCall(pCreateCompatibleDC, 0))
	if dc == 0 {
		return out
	}
	bmi := BITMAPINFO{Header: BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(BITMAPINFOHEADER{})),
		BiWidth:       int32(width),
		BiHeight:      -int32(height), // top-down, like every other buffer here
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: BI_RGB,
	}}
	var bits unsafe.Pointer
	dib, _, _ := pCreateDIBSection.Call(
		uintptr(dc),
		uintptr(unsafe.Pointer(&bmi)),
		DIB_RGB_COLORS,
		uintptr(unsafe.Pointer(&bits)),
		0, 0,
	)
	if dib == 0 || bits == nil {
		pDeleteDC.Call(uintptr(dc))
		return out
	}
	oldBitmap := mustCall(pSelectObject, uintptr(dc), dib)
	oldFont := mustCall(pSelectObject, uintptr(dc), f.handle)

	const (
		bkOpaque   = 2
		dtLeft     = 0x00000000
		dtVCenter  = 0x00000004
		dtSingle   = 0x00000020
		dtNoPrefix = 0x00000800
	)
	pSetBkMode.Call(uintptr(dc), bkOpaque)
	pSetBkColor.Call(uintptr(dc), 0x00000000)
	pSetTextColor.Call(uintptr(dc), 0x00ffffff)
	rect := RECT{Left: 0, Top: 0, Right: int32(width), Bottom: int32(height)}
	pDrawTextW.Call(
		uintptr(dc),
		uintptr(unsafe.Pointer(utf16Ptr(text))),
		^uintptr(0), // -1: the string is NUL terminated
		uintptr(unsafe.Pointer(&rect)),
		dtLeft|dtVCenter|dtSingle|dtNoPrefix,
	)

	pixels := unsafe.Slice((*uint8)(bits), width*height*4) // the DIB: BGRA, top-down
	for i := range out {
		out[i] = pixels[i*4+1] // the green channel: unchanged by any colour fringe
	}

	pSelectObject.Call(uintptr(dc), oldFont)
	pSelectObject.Call(uintptr(dc), oldBitmap)
	pDeleteObject.Call(dib)
	pDeleteDC.Call(uintptr(dc))
	return out
}
