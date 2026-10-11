package buddy

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// The panel, drawn so it can be looked at: every page, at three scales — a 720p screen, a
// 1080p one and a 4K one. "The plate and its type are the same size on the desk at every
// resolution" is a claim about pixels, and the only way to settle it is to look at the
// pixels. It writes a gallery page beside the images, for the preview to serve.
//
//	go test ./internal/buddy -run TestThePanelCanBeLookedAt
//
// The gallery is scratch: panelpreview/ is ignored by git, like the icon's own preview.
func TestThePanelCanBeLookedAt(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)

	type shot struct {
		page  int
		scale float64
	}
	shots := []shot{
		{pageMain, 1},
		{pageFlower, 1},
		{pageShape, 1},
		{pageDelivery, 1},
		{pageLooks, 1},
		{pagePlace, 1},
		// and the same page on a screen half the size and one twice the size
		{pageMain, 0.67},
		{pageMain, 2},
	}

	dir, err := filepath.Abs("panelpreview")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	type made struct {
		src   string // the image, inline: the preview serves one file and nothing beside it
		title string
		size  string
	}
	gallery := make([]made, 0, len(shots))
	for i, s := range shots {
		m := testMenuAt(t, b, s.page, s.scale)
		pix := m.panelPixels()
		img := image.NewNRGBA(image.Rect(0, 0, m.w, m.h))
		for at := 0; at+3 < len(pix) && at/4 < m.w*m.h; at += 4 {
			a := float64(pix[at+3]) / 255
			if a <= 0 {
				continue
			}
			// composited over a desktop-ish grey, which is what a person sees
			bg := 0.22
			for c := 0; c < 3; c++ {
				img.Pix[at+c] = uint8(min(255, float64(pix[at+c])+bg*255*(1-a)))
			}
			img.Pix[at+3] = 255
			_ = color.NRGBA{}
		}
		name := fmt.Sprintf("page%d-at%.2fx.png", i, s.scale)
		var png3 bytes.Buffer
		if err := png.Encode(&png3, img); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), png3.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		gallery = append(gallery, made{
			src:   "data:image/png;base64," + base64.StdEncoding.EncodeToString(png3.Bytes()),
			title: fmt.Sprintf("%s (page %d)", m.pageTitle(), s.page),
			size:  fmt.Sprintf("scale %.2fx, plate %dx%d px", s.scale, m.w, m.h),
		})
		t.Logf("%s: %s at %.2fx (%dx%d)", name, m.pageTitle(), s.scale, m.w, m.h)
	}

	page := "<!doctype html><meta charset=utf-8><title>the settings panel at every scale</title>" +
		"<style>body{background:#1b1b1f;color:#eee;font:14px/1.5 Segoe UI,system-ui,sans-serif;margin:24px}" +
		"figure{display:inline-block;margin:0 24px 24px 0;vertical-align:top}" +
		"figcaption{margin:8px 0 0;color:#b9b4c2}img{image-rendering:auto}</style>" +
		"<h1>The settings panel at every scale</h1>" +
		"<p>Green button at the foot of every page, drawn at the scale of the screen the flower is on.</p>"
	for _, g := range gallery {
		page += fmt.Sprintf("<figure><img src=%q><figcaption>%s<br>%s</figcaption></figure>", g.src, g.title, g.size)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("the gallery is %s", filepath.Join(dir, "index.html"))
}
