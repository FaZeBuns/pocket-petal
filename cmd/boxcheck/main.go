// Command boxcheck prints how much design space each carry script needs, which
// is how the window's size is chosen without guessing.
//
//	go run ./cmd/boxcheck
package main

import (
	"fmt"

	"github.com/FaZeBuns/pocket-petal/internal/flower"
)

func main() {
	rig := flower.CreateRig(flower.RigOptions{})
	rest := flower.OutlineBox(rig, flower.Motion{})
	fmt.Printf("rest outline:     %.1f x %.1f\n", rest.Width(), rest.Height())
	full := flower.MotionBox(rig, 1)
	fmt.Printf("motion envelope:  %.1f x %.1f  (%.2fx wide, %.2fx tall of rest)\n",
		full.Width(), full.Height(), full.Width()/rest.Width(), full.Height()/rest.Height())
	q := flower.FrameFor(rig, 190, 250, 0.94)
	fmt.Printf("framing: window %dx%d, flower %.0fpx tall, box %.1f x %.1f\n",
		q.Width, q.Height, q.Flower, q.Bounds.Width(), q.Bounds.Height())
}
