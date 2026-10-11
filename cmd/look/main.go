// Command look is a diagnostic: it sweeps a grid of points over the screen and
// reports what the flower's drag would see at each one. It exists because "the
// flower thinks my desktop is a chat" is a claim about *this* machine's windows,
// and the only honest way to answer it is to ask Windows the same question the
// drag asks — with no cursor, no clicking and nothing to watch.
//
//	go run ./cmd/look
//	go run ./cmd/look -step 80
//
// It prints one line per distinct window the sweep landed on, with how many
// points hit it, and marks the ones a dropped flower would type into. Every point
// that comes back as a drop target is worth reading twice: the list is short, and
// anything on it that is not a chat or a terminal is a bug.
package main

import (
	"flag"
	"fmt"
	"sort"

	"github.com/FaZeBuns/pocket-petal/internal/target"
	"github.com/FaZeBuns/pocket-petal/internal/win"
)

type hit struct {
	class   string
	title   string
	process string
	kind    string
	label   string
	rect    win.RECT
	points  int
	first   string
}

func main() {
	step := flag.Int("step", 120, "how far apart the sample points are, in pixels")
	flag.Parse()

	win.MakeDPIAware()
	pid := win.OurPid()
	area := win.PrimaryWorkArea()
	full := win.PrimaryScreen()

	fmt.Printf("look: screen %v, work area %v, our pid %d\n", full, area, pid)

	hits := map[uintptr]*hit{}
	points, missed := 0, 0
	for y := 2; y < int(full.Bottom); y += *step {
		for x := 2; x < int(full.Right); x += *step {
			points++
			info, ok := win.WindowAt(x, y, pid)
			if !ok {
				missed++
				continue
			}
			h := hits[uintptr(info.Hwnd)]
			if h == nil {
				t := target.Describe(target.Window{
					Hwnd:    uintptr(info.Hwnd),
					Title:   info.Title,
					Class:   info.Class,
					Process: info.Process,
				})
				h = &hit{
					class: info.Class, title: info.Title, process: info.Process,
					kind: t.Kind, label: t.Label, rect: info.Rect,
					first: fmt.Sprintf("%d,%d", x, y),
				}
				hits[uintptr(info.Hwnd)] = h
			}
			h.points++
		}
	}

	list := make([]*hit, 0, len(hits))
	deliverable := 0
	for _, h := range hits {
		list = append(list, h)
		if h.kind == "ai-chat" || h.kind == "terminal" {
			deliverable++
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].points > list[j].points })

	fmt.Printf("look: %d points, %d found no window at all, %d distinct windows\n\n", points, missed, len(list))
	for _, h := range list {
		mark := " "
		if h.kind == "ai-chat" || h.kind == "terminal" {
			mark = ">" // a point the flower would type into
		}
		fmt.Printf("%s %5d pts  %-14s %-22s %-16s %dx%d at %d,%d  %q\n",
			mark, h.points, orEmpty(h.process), orEmpty(h.class), h.kind,
			h.rect.Width(), h.rect.Height(), h.rect.Left, h.rect.Top, h.title)
		fmt.Printf("           -> %s (%q)\n", h.label, h.first)
	}
	fmt.Printf("\nlook: %d of %d windows would take a delivered note\n", deliverable, len(list))
}

func orEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
