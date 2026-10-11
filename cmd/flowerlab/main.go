// Command flowerlab renders the flower off-screen: the same maths and the same
// rasteriser the app uses, so a change to the drawing can be looked at without a window.
//
//	flowerlab -mode whole -out assets/go-whole.png
//	flowerlab -mode neck  -raw -out assets/go-neck.rgba
//	flowerlab -mode motion -out assets/go-swing.png
//
// It is also the quickest way to look at a change to the drawing without
// running the app at all.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	"github.com/FaZeBuns/pocket-petal/internal/flower"
)

type state struct {
	name   string
	motion flower.Motion
}

// The states the reference lab renders, in the same order: at rest, carried at
// speed, carried hard, just stopped, caught on the way back.
var states = []state{
	{"rest", flower.Motion{}},
	{"carried", flower.Motion{Lean: -0.7, Swing: -0.45, Flutter: 1.4}},
	{"carried hard", flower.Motion{Lean: -1.05, Swing: -0.95, Flutter: 2.1}},
	{"just stopped", flower.Motion{Lean: 0.2, Swing: 0.7, Flutter: 0.6}},
	{"caught", flower.Motion{Lean: 0.05, Swing: 1.05}},
}

// A hand's movement, sampled: nothing for a moment, then carried right at a
// walking speed, then stopped dead.
var hand = []struct {
	at, vx, ax float64
}{
	{0.0, 0, 0},
	{0.12, 780, 6500},
	{0.34, 900, 400},
	{0.4, 900, -9000},
	{0.46, 0, 0},
	{2.0, 0, 0},
}

var motionFrames = []float64{0.16, 0.28, 0.4, 0.47, 0.56, 0.72, 1.05, 1.6}

const fullAt = 6.0 // seconds of rig time: fully bloomed

type cell struct {
	name   string
	pose   flower.Pose
	motion flower.Motion
}

func main() {
	mode := flag.String("mode", "whole", "whole | neck | desk | motion | window")
	out := flag.String("out", "", "file to write (default assets/go-<mode>.png)")
	raw := flag.Bool("raw", false, "write a raw RGBA dump (header + pixels) instead of a PNG")
	seed := flag.Int("seed", 11, "flower seed")
	flag.Parse()

	rig := flower.CreateRig(flower.RigOptions{Seed: seed})
	cells := buildCells(rig, *mode)

	cellW, cellH, padding, shave := 320, 430, 10.0, 0.5
	if *mode == "desk" {
		cellW, cellH, padding, shave = 178, 250, 6, 0.7
	}
	// the app's own framing: the window it actually puts on the desktop, sized
	// around the flower, so this is the picture to judge the flower by
	app := flower.FrameFor(rig, windowWantW, windowWantH, windowFit)
	if *mode == "window" {
		cellW, cellH, padding, shave = app.Width, app.Height, 0, 0
	}

	box := frameFor(rig, cells, *mode)
	var bg *flower.RGB
	if *mode != "window" {
		// the window mode is how the flower really looks on a desktop: no
		// backdrop behind it, so the alpha can be judged too
		onDark := flower.RGB{22, 19, 30}
		bg = &onDark
	}
	width := cellW * len(cells)
	buf := make([]uint8, width*cellH*4)

	for i, c := range cells {
		list := flower.BuildDrawList(rig, c.pose, flower.DrawOptions{Detail: "full", Shadow: *mode != "neck"})
		px := flower.Rasterize(list, flower.RasterOptions{
			Width:      cellW,
			Height:     cellH,
			Bounds:     box,
			Padding:    padding,
			SubSamples: 2,
			Background: bg,
			Shave:      shave,
		})
		for y := 0; y < cellH; y++ {
			from := y * cellW * 4
			to := (y*width + i*cellW) * 4
			copy(buf[to:to+cellW*4], px[from:from+cellW*4])
		}
	}

	path := *out
	if path == "" {
		name := filepath.Join("assets", "go-"+*mode)
		if *raw {
			path = name + ".rgba"
		} else {
			path = name + ".png"
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fail(err)
	}
	if *raw {
		header := make([]byte, 8)
		binary.LittleEndian.PutUint32(header[0:4], uint32(width))
		binary.LittleEndian.PutUint32(header[4:8], uint32(cellH))
		if err := os.WriteFile(path, append(header, buf...), 0o644); err != nil {
			fail(err)
		}
	} else {
		img := image.NewNRGBA(image.Rect(0, 0, width, cellH))
		for i := 0; i < width*cellH; i++ {
			img.SetNRGBA(i%width, i/width, color.NRGBA{buf[i*4], buf[i*4+1], buf[i*4+2], buf[i*4+3]})
		}
		f, err := os.Create(path)
		if err != nil {
			fail(err)
		}
		if err := png.Encode(f, img); err != nil {
			fail(err)
		}
		f.Close()
	}

	names := make([]string, len(cells))
	for i, c := range cells {
		names[i] = c.name
	}
	fmt.Printf("%s: %v -> %s (%dx%d)\n", *mode, names, path, width, cellH)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "flowerlab:", err)
	os.Exit(1)
}

func buildCells(rig *flower.Rig, mode string) []cell {
	if mode == "motion" {
		carry := flower.CreateCarry(flower.PhysicsOptions{})
		frames := map[float64]flower.Motion{}
		dt := 1.0 / 120
		for t := 0.0; t <= motionFrames[len(motionFrames)-1]+1e-9; t += dt {
			vx, ax := 0.0, 0.0
			for _, h := range hand {
				if t <= h.at {
					vx, ax = h.vx, h.ax
					break
				}
			}
			carry.Step(dt, flower.Hand{VX: vx, AX: ax, Dragging: true})
			for _, want := range motionFrames {
				if _, ok := frames[want]; !ok && t >= want {
					frames[want] = carry.Motion()
				}
			}
		}
		cells := make([]cell, 0, len(motionFrames))
		for _, t := range motionFrames {
			m := frames[t]
			cells = append(cells, cell{fmt.Sprintf("%.2fs", t), flower.PoseOf(rig, fullAt, m), m})
		}
		return cells
	}
	cells := make([]cell, 0, len(states))
	for _, s := range states {
		cells = append(cells, cell{s.name, flower.PoseOf(rig, fullAt, s.motion), s.motion})
	}
	return cells
}

// frameFor picks one design-space box for every cell, so the cells can be
// compared with each other (and with the reference renderer's).
func frameFor(rig *flower.Rig, cells []cell, mode string) flower.Bounds {
	union := func(a, b flower.Bounds) flower.Bounds {
		return flower.Bounds{
			MinX: min(a.MinX, b.MinX),
			MinY: min(a.MinY, b.MinY),
			MaxX: max(a.MaxX, b.MaxX),
			MaxY: max(a.MaxY, b.MaxY),
		}
	}
	first := true
	var box flower.Bounds
	add := func(b flower.Bounds) {
		if first {
			box, first = b, false
			return
		}
		box = union(box, b)
	}

	switch mode {
	case "whole", "desk":
		for _, c := range cells {
			list := flower.BuildDrawList(rig, c.pose, flower.DrawOptions{Detail: "full", Shadow: true})
			add(flower.ListBounds(list))
		}
		if mode == "desk" {
			box = scaleBox(box, 1.26)
		}
	case "neck":
		add(neckBox(cells[0].pose))
	case "window":
		box = appFrameFor(rig).Bounds
	case "motion":
		for _, c := range cells {
			add(neckBox(c.pose))
		}
		box = scaleBox(box, 1.1)
	}
	return box
}

func scaleBox(b flower.Bounds, factor float64) flower.Bounds {
	cx := (b.MinX + b.MaxX) / 2
	cy := (b.MinY + b.MaxY) / 2
	halfW := (b.MaxX - b.MinX) / 2 * factor
	halfH := (b.MaxY - b.MinY) / 2 * factor
	return flower.Bounds{MinX: cx - halfW, MinY: cy - halfH, MaxX: cx + halfW, MaxY: cy + halfH}
}

// The app's window, as `pocket-petal` sizes it.
const (
	windowWantW = 190
	windowWantH = 250
	windowFit   = 0.94
)

func appFrameFor(rig *flower.Rig) flower.Framing {
	return flower.FrameFor(rig, windowWantW, windowWantH, windowFit)
}

// neckBox is a window onto the top of the stalk and the bloom: the join the eye
// goes to first, wide enough for the bloom and deep enough for the neck.
func neckBox(p flower.Pose) flower.Bounds {
	return flower.Bounds{
		MinX: p.Head.X - 86,
		MaxX: p.Head.X + 86,
		MinY: p.Head.Y - 96,
		MaxY: p.Head.Y + 62,
	}
}
