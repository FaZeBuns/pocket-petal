package buddy

import (
	"fmt"
	"image"
	"image/png"
	"os"
)

// dumpFrame writes one rendered frame to a PNG. It exists because the two
// questions that look identical from the outside — "the window is not on
// screen" and "the picture in it is transparent" — are answered by looking at
// the picture: `pocket-petal -dump frame.png` shows exactly the pixels the
// window is being handed, with the alpha kept.
func dumpFrame(path string, rgba []uint8, width, height int) error {
	if width <= 0 || height <= 0 || len(rgba) < width*height*4 {
		return fmt.Errorf("buddy: frame of %dx%d does not fit %d bytes", width, height, len(rgba))
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, rgba[:width*height*4])
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
