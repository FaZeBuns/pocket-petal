// Package icon is the picture Windows shows for the exe: the flower, drawn by the
// app's own rasteriser at every size an icon is looked for at, packed into an .ico
// and into the resources of the built binary.
//
// Nothing here is a graphic and nothing is loaded: the icon is the same maths as the
// flower on the desktop (see internal/flower), and the file format is written byte by
// byte — which is what keeps the promise that there is nothing to install and nothing
// beside the exe, and means the icon can simply be regenerated when the flower
// changes:
//
//	go run ./cmd/mkicon
package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"

	"github.com/FaZeBuns/pocket-petal/internal/flower"
)

// Sizes are the sizes an exe icon is asked for, from the 16 pixels of a title bar to
// the 256 of the large-icon view in Explorer. They are the ones the shell itself
// uses, so an icon that carries them is never resampled by Windows.
var Sizes = []int{16, 24, 32, 48, 64, 128, 256}

// Picture is one size of the icon, already in the form Windows wants it.
type Picture struct {
	Size int
	Data []byte
}

// Pictures draws the flower at each size. Subject chooses what is in the picture:
// "bloom" (the default) fills the square with the flower's head, which is what reads
// at sixteen pixels, and "flower" is the whole plant, as the tray icon draws it.
func Pictures(rig *flower.Rig, sizes []int, subject string) []Picture {
	out := make([]Picture, 0, len(sizes))
	for _, size := range sizes {
		rgba := RGBA(rig, size)
		if subject != "flower" {
			rgba = Bloom(rig, size)
			// a soft white edge, so the flower reads on a dark taskbar and a light
			// Explorer window alike — the outline is part of the picture, not a crop
			Outline(rgba, size, max(1, size/22))
		}
		out = append(out, Picture{Size: size, Data: pack(rgba, size)})
	}
	return out
}

// Bloom renders the flower's head — petals and middle — filling a square of the
// given size. It is the same maths as the whole flower, framed on the bloom instead
// of on the plant: at sixteen pixels a whole flower is a stem with a dot on it, and
// what a person recognises is the bloom.
func Bloom(rig *flower.Rig, size int) []uint8 {
	pose := flower.PoseOf(rig, 6, flower.Motion{})
	list := flower.BuildDrawList(rig, pose, flower.DrawOptions{
		Detail: "full", Shadow: false, Palette: rig.Palette,
	})
	head := make([]flower.Shape, 0, len(list))
	for _, shape := range list {
		switch shape.Part {
		case "petal", "center", "head":
			head = append(head, shape)
		}
	}
	if len(head) == 0 {
		return RGBA(rig, size)
	}
	return flower.Rasterize(head, flower.RasterOptions{
		Width:  size,
		Height: size,
		Bounds: flower.ListBounds(head),
		// No air around the bloom: it is drawn to the edges of the square, so that
		// sixteen pixels of icon are sixteen pixels of flower. What that costs is the
		// part of the white edge that falls outside the frame — the bloom reaches the
		// frame at its four widest points and is quietly cut there — and it is worth
		// it: measured against the old framing the flower is nearly a tenth larger,
		// and the edge still runs around every side the bloom does not touch.
		Padding:    0,
		SubSamples: 3,
		Shave:      1,
	})
}

// Outline paints a white edge around whatever is already drawn, the way a sticker
// has one: every empty pixel within reach of the picture becomes white, carrying the
// alpha it found, so the edge follows the flower's own soft edge instead of inventing
// a harder one. It is what makes the same icon work on a wallpaper, a taskbar and
// Explorer's white background.
func Outline(rgba []uint8, size, width int) {
	if width < 1 {
		return
	}
	src := make([]uint8, len(rgba))
	copy(src, rgba)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			i := (y*size + x) * 4
			if src[i+3] > 24 {
				continue // the flower itself
			}
			best := uint8(0)
			for dy := -width; dy <= width; dy++ {
				ny := y + dy
				if ny < 0 || ny >= size {
					continue
				}
				for dx := -width; dx <= width; dx++ {
					nx := x + dx
					if nx < 0 || nx >= size {
						continue
					}
					if a := src[(ny*size+nx)*4+3]; a > best {
						best = a
					}
				}
			}
			if best > 24 {
				rgba[i+0], rgba[i+1], rgba[i+2] = 255, 255, 255
				rgba[i+3] = best
			}
		}
	}
}

// RGBA renders the flower as a square picture of the given size, the way the tray
// icon is rendered — the flower at its grown pose, framed with a little air around
// it, antialiased — so the picture in the taskbar and the picture on the exe are the
// same flower.
func RGBA(rig *flower.Rig, size int) []uint8 {
	pose := flower.PoseOf(rig, 6, flower.Motion{})
	list := flower.BuildDrawList(rig, pose, flower.DrawOptions{
		Detail: "full", Shadow: false, Palette: rig.Palette,
	})
	bounds := flower.ListBounds(list)
	return flower.Rasterize(list, flower.RasterOptions{
		Width:      size,
		Height:     size,
		Bounds:     bounds,
		Padding:    float64(size) * 0.06,
		SubSamples: 3,
		Shave:      1,
	})
}

// pack is one rendered flower as an icon image: a DIB for the sizes a DIB is right
// for, which every version of Windows understands, and a PNG for 256, which is what
// PNG is in an icon for — the same picture in a twentieth of the bytes.
func pack(rgba []uint8, size int) []byte {
	if size >= 256 {
		return packPNG(rgba, size)
	}
	return packDIB(rgba, size)
}

// packDIB writes the old icon format: a bitmap header whose height is *twice* the
// icon, the colours bottom-up in BGRA, and then the mask — which says nothing, all
// zeroes, because a 32-bit icon's alpha channel is what Windows believes.
func packDIB(rgba []uint8, size int) []byte {
	colourRow := size * 4
	maskRow := ((size + 31) / 32) * 4
	out := make([]byte, 40+colourRow*size+maskRow*size)

	le := binary.LittleEndian
	le.PutUint32(out[0:], 40)                // the header's own size
	le.PutUint32(out[4:], uint32(size))      // width
	le.PutUint32(out[8:], uint32(size*2))    // height: colours and mask
	le.PutUint16(out[12:], 1)                // planes
	le.PutUint16(out[14:], 32)               // bits a pixel
	le.PutUint32(out[20:], uint32(len(out))) // the bytes that follow, for what it is worth
	at := 40
	for y := size - 1; y >= 0; y-- { // bottom-up
		for x := 0; x < size; x++ {
			i := (y*size + x) * 4
			out[at+0] = rgba[i+2]
			out[at+1] = rgba[i+1]
			out[at+2] = rgba[i+0]
			out[at+3] = rgba[i+3]
			at += 4
		}
	}
	return out
}

// packPNG is the same picture as a PNG, which Explorer has accepted since Vista.
func packPNG(rgba []uint8, size int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	copy(img.Pix, rgba[:size*size*4])
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return packDIB(rgba, size)
	}
	return buf.Bytes()
}

// File packs the pictures into an .ico: a small header, one directory entry for each
// size, and then the pictures themselves.
func File(pictures []Picture) []byte {
	le := binary.LittleEndian
	head := make([]byte, 6+16*len(pictures))
	le.PutUint16(head[2:], 1) // 1 is an icon (2 is a cursor)
	le.PutUint16(head[4:], uint16(len(pictures)))

	at := len(head)
	body := make([]byte, 0, 1<<16)
	for i, pic := range pictures {
		e := 6 + i*16
		if pic.Size >= 256 {
			head[e+0] = 0 // 0 means 256: the width is one byte
			head[e+1] = 0
		} else {
			head[e+0] = uint8(pic.Size)
			head[e+1] = uint8(pic.Size)
		}
		le.PutUint16(head[e+4:], 1)  // planes
		le.PutUint16(head[e+6:], 32) // bits a pixel
		le.PutUint32(head[e+8:], uint32(len(pic.Data)))
		le.PutUint32(head[e+12:], uint32(at))
		body = append(body, pic.Data...)
		at += len(pic.Data)
	}
	return append(head, body...)
}
