// Command mkicon draws the flower and writes the two things a Windows exe needs to
// look like an app: an .ico file, and the resource section that carries the icon and
// the version stamp inside the binary itself.
//
//	go run ./cmd/mkicon                          # both files, the defaults
//	go run ./cmd/mkicon -version 1.2.0           # stamped as a release build
//	go run ./cmd/mkicon -ico assets/pocket-petal.ico -syso rsrc_windows_amd64.syso
//
// Nothing is downloaded and nothing is compiled but this: the flower comes from the
// app's own renderer (internal/icon), the icon file is written byte by byte, and the
// .syso is a one-section COFF object the Go linker picks up on its own. Which is why
// the release script runs this before it builds — every exe this repo makes has the
// flower on it.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"runtime"

	"github.com/FaZeBuns/pocket-petal/internal/buddy"
	"github.com/FaZeBuns/pocket-petal/internal/flower"
	"github.com/FaZeBuns/pocket-petal/internal/icon"
)

func main() {
	icoPath := flag.String("ico", "assets/pocket-petal.ico", "write the icon to this file (empty for none)")
	sysoPath := flag.String("syso", "", "write the linker object for the resources here (default rsrc_windows_<arch>.syso)")
	version := flag.String("version", "", "the version to stamp (default: the one the source carries)")
	palette := flag.String("palette", "blush", "which colourway the icon is drawn in")
	seed := flag.Int("seed", 0, "which flower to grow for the icon")
	preview := flag.String("preview", "", "write a PNG of each size here, to look at them")
	logo := flag.Int("logo", 0, "with -preview: also write one PNG at this size, for a page rather than a taskbar")
	subject := flag.String("subject", "bloom", "what the icon shows: bloom (the flower's head) or flower")
	arch := flag.String("arch", runtime.GOARCH, "the architecture the object file is for: amd64, arm64, 386")
	flag.Parse()

	if *version == "" {
		*version = buddy.Version
	}
	if *sysoPath == "" {
		*sysoPath = "rsrc_windows_" + *arch + ".syso"
	}

	rig := flower.CreateRig(flower.RigOptions{Seed: seed, Palette: *palette})
	pictures := icon.Pictures(rig, icon.Sizes, *subject)

	if *icoPath != "" {
		if err := os.WriteFile(*icoPath, icon.File(pictures), 0o644); err != nil {
			fail("icon:", err)
		}
		fmt.Printf("icon:   %s — %d sizes, %d bytes\n", *icoPath, len(pictures), size(*icoPath))
	}

	info := icon.Info{
		Name:        buddy.Name,
		Slug:        buddy.Slug,
		Description: "A flower you can pick up and put on someone's chat",
		Version:     *version,
		Copyright:   "MIT licensed",
	}
	table := icon.Resources(pictures, info)
	syso, err := icon.Syso(table, *arch)
	if err != nil {
		fail("icon:", err)
	}
	if err := os.WriteFile(*sysoPath, syso, 0o644); err != nil {
		fail("icon:", err)
	}
	fmt.Printf("res:    %s - %s %s, %d bytes of resources, %d addresses for the linker\n",
		*sysoPath, buddy.Name, *version, len(table.Data), len(table.Fixups))

	if *preview != "" {
		for _, pic := range pictures {
			rgba := icon.RGBA(rig, pic.Size)
			if *subject != "flower" {
				rgba = icon.Bloom(rig, pic.Size)
				icon.Outline(rgba, pic.Size, max(1, pic.Size/22))
			}
			path := fmt.Sprintf("%s-%d.png", *preview, pic.Size)
			if err := writePNG(path, rgba, pic.Size); err != nil {
				fail("icon:", err)
			}
			fmt.Printf("look:   %s\n", path)
		}
		// and one at whatever size a *page* wants — the README's face, a repository's
		// avatar, a social card — drawn from the same flower by the same renderer, so
		// the mark is the app and not a picture of it found somewhere
		if *logo > 0 {
			rgba := icon.RGBA(rig, *logo)
			if *subject != "flower" {
				rgba = icon.Bloom(rig, *logo)
				icon.Outline(rgba, *logo, max(1, *logo/22))
			}
			path := fmt.Sprintf("%s-%d.png", *preview, *logo)
			if err := writePNG(path, rgba, *logo); err != nil {
				fail("icon:", err)
			}
			fmt.Printf("look:   %s\n", path)
		}
	}
}

// writePNG is the preview: the same pixels the icon carries, with the alpha kept, so
// that the sizes can be looked at rather than taken on trust.
func writePNG(path string, rgba []uint8, size int) error {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	copy(img.Pix, rgba[:size*size*4])
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func size(path string) int64 {
	if st, err := os.Stat(path); err == nil {
		return st.Size()
	}
	return 0
}

func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "%s %v\n", what, err)
	os.Exit(1)
}
