package buddy

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/FaZeBuns/pocket-petal/internal/flower"
	"github.com/FaZeBuns/pocket-petal/internal/win"
)

// The settings panel.
//
// This is the app's whole user interface, and it is drawn the way everything else
// in here is drawn: polygons and fills into an RGBA buffer, per-pixel alpha, and
// real type blended in through a coverage mask. The shell's own menus cannot be
// styled at all — they arrive grey and square, at whatever size the system font
// happens to be — and a settings panel is the one place a person looks at the app
// and not at the flower, so it is worth the drawing code.
//
// The panel is a real window that takes the focus, which is what gives it the
// keyboard and, more usefully, tells it when to close: any click anywhere else
// takes the focus away.
//
// Pages keep it short. One list of twenty-eight rows is a telephone book; four
// pages of seven rows is a settings screen.

const (
	menuWidth   = 316.0
	menuRadius  = 15.0
	rowHeight   = 34.0
	sliderH     = 47.0 // a slider needs room for its label and its track
	headingH    = 27.0
	sepHeight   = 10.0
	menuPadTop  = 48.0 // room for the title
	menuPadEnd  = 10.0
	menuMarginX = 12.0
	swatchR     = 5.0
	layoutPad   = 14.0 // left inset for labels that have no swatch
	trackH      = 6.0  // the height of a slider's track
	knobR       = 6.5  // the radius of the knob on it
	titleTop    = 12.0 // where the title's mask starts
	titleH      = 40.0 // and how tall it is
)

// metrics is the panel's geometry in screen pixels: the measurements above, multiplied by
// the scale the panel is being drawn at. The layout below is written entirely in these and
// in m.dp, so nothing in here has to know that the plate on a 4K screen is twice the plate
// on a 1080p one — while a panel that ignored the display would be a postage stamp beside a
// flower that had grown with the screen (see panelScale).
type metrics struct {
	radius   float64
	rowH     float64
	sliderH  float64
	headingH float64
	sepH     float64
	padTop   float64
	padEnd   float64
	marginX  float64
	swatchR  float64
	labelPad float64
	trackH   float64
	knobR    float64
	titleTop float64
	titleH   float64
}

// metricsFor is the panel's geometry at a scale, every measurement rounded to a whole
// pixel: rows then land on the pixel grid and the text in them stays crisp, at any scale.
func metricsFor(scale float64) metrics {
	if scale <= 0 {
		scale = 1
	}
	dp := func(v float64) float64 { return math.Round(v * scale) }
	return metrics{
		radius: dp(menuRadius), rowH: dp(rowHeight), sliderH: dp(sliderH),
		headingH: dp(headingH), sepH: dp(sepHeight), padTop: dp(menuPadTop),
		padEnd: dp(menuPadEnd), marginX: dp(menuMarginX), swatchR: dp(swatchR),
		labelPad: dp(layoutPad), trackH: dp(trackH), knobR: dp(knobR),
		titleTop: dp(titleTop), titleH: dp(titleH),
	}
}

// rowKind is what a row is.
type rowKind int

const (
	rowHeading rowKind = iota
	rowSep
	rowToggle // a switch on the right
	rowPick   // one of a set: a swatch or a tick, and it stays open when chosen
	rowPage   // opens a page
	rowAction // does something and closes the panel
	rowSlider // a value on a track, dragged with the mouse or nudged with an arrow
	rowCoffee // the green button at the foot of every page: a door out of the app
)

// The coffee button. It is the one row in the panel that is not a setting and does not
// belong to the flower's palette: a green plate with a cup on it, at the foot of every
// page, which will one day open the page where somebody can buy the flower a coffee. The
// address is not decided yet, so pressing it says so and does nothing else — a button that
// pretends to work is worse than one that is obviously waiting.
const coffeeLabel = "Buy me a Coffee! ☕"

// coffeeGreen is the button's own colour, and the only green in the app: nothing else
// here is allowed to look like a doorway out.
var coffeeGreen = flower.RGB{47, 168, 102}

// danger is the colour of the ways out — the cross that puts the panel away, Back, and
// Quit — and the only red in the app. A row that leaves should not look like a row that
// changes a setting, and a colour is the fastest way of saying so.
var danger = flower.RGB{235, 92, 92}

// dangerHot is the same red with the light of a hover on it.
var dangerHot = flower.RGB{255, 128, 128}

// quitPlate is the dark red the way out of the app is painted in, and coffeePlate the
// green it is pressed in: a button is a colour, white type on it, and the whole row.
var (
	quitPlate   = plateInk(danger)
	coffeePlate = coffeeGreen
)

// plateLum is how dark a button's colour has to be to carry white type: about 6 to 1
// against white, which is the ratio a label has to reach to be read rather than guessed at.
// It is also what makes the way out a *dark* red rather than a red.
const plateLum = 0.12

// plateInk is a button's depth of a colour: the same hue, the same character, darkened
// until white type can be read on it. A pastel petal colour is a colour rather than a
// button, and the way out of the app is a *dark* red rather than a red — the same rule
// gives both, which is what makes a row painted in a colour look like a thing to press.
func plateInk(base flower.RGB) flower.RGB {
	out := base
	for f := 1.0; f >= 0.04; f -= 0.02 {
		out = flower.RGB{base[0] * f, base[1] * f, base[2] * f}
		if luminance(out) <= plateLum {
			return out
		}
	}
	return out
}

// luminance is a colour's relative brightness, added up the way a person's eye does.
func luminance(c flower.RGB) float64 {
	linear := func(v float64) float64 {
		s := v / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(c[0]) + 0.7152*linear(c[1]) + 0.0722*linear(c[2])
}

// row is one line of the panel. The state is captured when the page is built, so
// a page is rebuilt after every change: the tick that moves is the truth.
//
// A slider carries three functions rather than a value: `value` reads the setting
// as it is now, `set` changes it and shows the change, and `commit` does the part
// that is too heavy to do while the pointer is moving (a window is a rectangle of
// air, but resizing one mid-drag is still a resize).
type row struct {
	kind   rowKind
	label  string
	hint   string
	on     bool
	swatch *flower.RGB
	page   int
	act    func(b *Buddy, m *menuState)

	// ink is the row's own colour, for the rows that are not simply settings: the red of
	// a way out (the cross, Back), or a colourway's name in the colourway. Nil means the
	// panel's ordinary ink.
	ink *flower.RGB

	// plate is a row that is a button rather than a line of settings: the whole row is
	// painted in the colour, and the label is white type on it — centred, unless the row
	// also has a swatch, in which case the swatch is the colour shown true and the name
	// sits beside it. The coffee button, the way out of the app, and each colourway are
	// drawn this way.
	plate *flower.RGB

	// slider
	lo, hi float64
	step   float64
	value  func() float64
	set    func(b *Buddy, v float64)
	write  func(v float64) string
}

// menuState is an open panel.
type menuState struct {
	panel  *win.Window
	title  *win.Font
	body   *win.Font
	small  *win.Font
	button *win.Font // the coffee button's label: the panel's largest face

	// the scale the panel is drawn at, and the geometry that comes out of it: the
	// layout is written in design measurements (the constants at the top of this file,
	// which are pixels on a 1080p screen) and every one of them reaches the pixels
	// through m.dp or m.mm, which multiply by this. It is the flower's own scale, so
	// the plate and the flower on the desk are the same size to the same person
	scale float64
	mm    metrics

	page int
	x, y int
	w, h int
	rows []row

	// layout, in pixels
	rowTop  []float64
	rowH    []float64
	masks   [][]uint8 // one text mask per row label
	hints   [][]uint8
	hintW   []float64
	maskH   []int // how tall each of those masks is (a slider's is shorter)
	titlePx []uint8

	hover   int
	press   int
	drag    int // the slider being dragged, or -1
	accent  flower.RGB
	dim     flower.RGB
	ink     flower.RGB
	panelBG flower.RGB

	// The panel is kept as pixels rather than redrawn: the shadow, the plate and
	// the title go into bg once, and the picture on screen is built from bg plus
	// one pass over the rows. A panel that is merely being hovered or dragged over
	// then costs one band copy and one row, instead of a hundred and twelve
	// milliseconds of rounded rectangles — which is the difference between a
	// settings panel and a slideshow.
	pix    []uint8
	bg     []uint8
	marked []int // rows that need drawing again
	all    bool  // ...or all of them

	// the cross in the corner, and the drag that moves the whole panel. The
	// chrome is above the rows and belongs to no row, so it is redrawn on its own
	// flag rather than by marking a row that does not exist.
	closeHover bool
	chrome     bool // the chrome needs drawing again
	panelDrag  bool // the background is being dragged: the panel follows the pointer
	dragDX     int  // where inside the panel the pointer took hold of it
	dragDY     int

	shown  bool
	closed bool
}

// pages
const (
	pageMain = iota
	pageFlower
	pageShape
	pageDelivery
	pageLooks
	pagePlace
)

// showMenu opens the panel, or closes it if it is already open.
func (b *Buddy) showMenu() {
	if b.menu != nil {
		b.closeMenu()
		return
	}
	m := &menuState{
		hover: -1, press: -1, drag: -1, page: pageMain,
		accent:  flower.PaletteNamed(b.rig.Palette).PetalTip,
		dim:     flower.RGB{158, 146, 168},
		ink:     flower.RGB{240, 235, 245},
		panelBG: flower.RGB{24, 19, 27},
		// the plate is drawn at the flower's own scale: a panel and a flower on one
		// desk are the same size to the same person (see panelScale)
		scale: panelScale(b),
	}
	if !m.fonts() {
		return
	}
	b.menu = m
	if !m.open(b) {
		b.closeMenu()
	}
}

// fonts builds the panel's three faces for the scale it is drawn at: a plate twice the
// size wants type twice the size, and it is real type at both — which is why the faces
// are made here rather than picked from a fixed set.
func (m *menuState) fonts() bool {
	m.closeFonts()
	var err error
	if m.title, err = win.NewFont(int(m.dp(19)), win.FontBold); err != nil {
		fmt.Fprintln(os.Stderr, "buddy: menu font:", err)
		return false
	}
	if m.body, err = win.NewFont(int(m.dp(15)), win.FontMedium); err != nil {
		fmt.Fprintln(os.Stderr, "buddy: menu font:", err)
		return false
	}
	if m.small, err = win.NewFont(int(m.dp(12)), win.FontRegular); err != nil {
		fmt.Fprintln(os.Stderr, "buddy: menu font:", err)
		return false
	}
	// the button's own face, and the biggest type in the panel: the green plate is the
	// one thing on it to press, and it is meant to look like it
	if m.button, err = win.NewFont(int(m.dp(24)), win.FontBold); err != nil {
		fmt.Fprintln(os.Stderr, "buddy: menu font:", err)
		return false
	}
	return true
}

// closeFonts puts the three faces down, so that a panel being drawn at a new scale does
// not leave the old ones behind.
func (m *menuState) closeFonts() {
	m.title.Close()
	m.body.Close()
	m.small.Close()
	m.button.Close()
	m.title, m.body, m.small, m.button = nil, nil, nil, nil
}

// rescale is the screen changing under an open panel: the plate is drawn at the scale of
// the screen it is on, so it is laid out again at the new one — type to match, rows to
// match, a wider plate — and put back inside the work area, which is itself a different
// size now. refitDisplay calls it, so a resolution change reaches the panel by the same
// path the flower takes.
func (m *menuState) rescale(b *Buddy) {
	scale := panelScale(b)
	if math.Abs(scale-m.scale) < 1e-9 {
		return
	}
	m.scale = scale
	if !m.fonts() {
		b.closeMenu()
		return
	}
	m.rebuild(b)
	m.keepOnScreen()
}

// keepOnScreen puts the panel back inside the work area: a plate that has grown, or a
// screen that has shrunk, must not leave the panel hanging off an edge where it cannot be
// read or reached. It moves the plate's own window with it.
func (m *menuState) keepOnScreen() {
	area := win.PrimaryWorkArea()
	if m.x+m.w > int(area.Right)-8 {
		m.x = int(area.Right) - m.w - 8
	}
	if m.y+m.h > int(area.Bottom)-8 {
		m.y = int(area.Bottom) - m.h - 8
	}
	if m.x < int(area.Left)+8 {
		m.x = int(area.Left) + 8
	}
	if m.y < int(area.Top)+8 {
		m.y = int(area.Top) + 8
	}
	if m.panel != nil {
		m.panel.MoveTo(m.x, m.y)
	}
}

// closeMenu puts it away.
func (b *Buddy) closeMenu() {
	m := b.menu
	if m == nil {
		return
	}
	b.menu = nil
	if m.panel != nil {
		m.panel.Destroy()
	}
	m.closeFonts()
}

// open lays the page out and puts the window on screen.
func (m *menuState) open(b *Buddy) bool {
	m.measure(b)
	// Where it goes: at the cursor, opening up and to the left the way a menu off
	// the tray does, folded back inside the work area when there is no room there.
	// A panel that opens off the edge of the screen is a panel nobody can read.
	area := win.PrimaryWorkArea()
	pt := win.CursorPosition()
	// the gaps round the cursor and the edge are the panel's own measurements, so a
	// panel twice the size stands twice as far off the corner of the tray icon
	gap := int(m.dp(8))
	x := int(pt.X) - m.w + int(m.dp(24))
	y := int(pt.Y) - m.h - int(m.dp(12))
	if y < int(area.Top)+gap {
		y = int(pt.Y) + int(m.dp(22)) // under the cursor instead, on a taskbar at the top
	}
	if x < int(area.Left)+gap {
		x = int(area.Left) + gap
	}
	if x+m.w > int(area.Right)-gap {
		x = int(area.Right) - m.w - gap
	}
	if y+m.h > int(area.Bottom)-gap {
		y = int(area.Bottom) - m.h - gap
	}
	m.x, m.y = x, y
	panel, err := win.NewPopupWindow(m.w, m.h, win.MenuClass, func(msg win.MSG) bool { return m.onMessage(b, msg) })
	if err != nil {
		return false
	}
	m.panel = panel
	m.markAll()
	return m.paint(b)
}

// measure builds the page's rows and then the text masks and geometry they
// need. The rows are built here rather than at open time because every choice
// rebuilds them: the tick that moves is the truth, and it is built from the
// settings as they are now.
//
// Measuring does not decide what has to be drawn again: a slider dragged through
// a page lays the same rows out again with a new number on one of them, and
// repainting the whole panel for that is exactly the lag this is here to avoid.
// The caller says what changed (see mark, markAll).
func (m *menuState) measure(b *Buddy) {
	m.layout()
	m.rows = b.menuPage(m.page)
	m.rowTop = m.rowTop[:0]
	m.rowH = m.rowH[:0]
	m.masks = m.masks[:0]
	m.hints = m.hints[:0]
	m.hintW = m.hintW[:0]
	m.maskH = m.maskH[:0]

	m.titlePx = m.title.Mask(m.pageTitle(), m.w, int(m.mm.titleH))

	y := m.mm.padTop
	for _, r := range m.rows {
		switch r.kind {
		case rowHeading, rowSep:
			height := m.mm.headingH
			if r.kind == rowSep {
				height = m.mm.sepH
			}
			var px []uint8
			if r.kind == rowHeading {
				px = m.small.Mask(strings.ToUpper(r.label), m.w, int(height))
			}
			m.masks = append(m.masks, px)
			m.hints = append(m.hints, nil)
			m.hintW = append(m.hintW, 0)
			m.maskH = append(m.maskH, int(height))
			m.rowTop = append(m.rowTop, y)
			m.rowH = append(m.rowH, height)
			y += height
			continue
		}
		// every mask is the panel's own width, so one stride serves them all: the
		// mask is laid in at an x, and its own left edge is where the text starts
		height := m.mm.rowH
		labelH := int(m.mm.rowH)
		font := m.body
		if r.kind == rowSlider {
			height = m.mm.sliderH
			labelH = int(m.dp(22))
		}
		if r.plate != nil {
			// a button is a little taller than a row of settings, and its label is the one
			// piece of type in the panel that is not a line of a list: it is the thing to
			// press, so it is set in the biggest face there is and given the whole of the
			// button's height to sit in the middle of. A label too long for a narrow plate
			// steps back down to the title's size rather than running out of either end of
			// a painted row.
			height = m.mm.rowH + m.dp(14)
			labelH = int(height)
			font = m.button
			if wide, _ := m.button.Measure(r.label); wide > int(float64(m.w)-2*m.mm.marginX-m.dp(16)) {
				font = m.title
			}
		}
		px := font.Mask(r.label, m.w, labelH)
		m.masks = append(m.masks, px)
		// a slider writes its value where a row writes its hint
		hint := r.hint
		if r.kind == rowSlider && r.write != nil {
			hint = r.write(r.value())
		}
		// Every row gets one entry, whether or not there is anything in it: the three
		// slices are read by row number (see drawRow), so a row that appended nothing
		// would shift every hint below it onto the wrong line.
		switch {
		case r.plate != nil:
			// the button's label is centred, so its width is what the drawing needs from
			// here — measured once per layout rather than once per paint, with the same
			// face its mask was made in
			hw, _ := font.Measure(r.label)
			m.hints = append(m.hints, nil)
			m.hintW = append(m.hintW, float64(hw))
		case hint != "":
			hw, _ := m.small.Measure(hint)
			m.hints = append(m.hints, m.small.Mask(hint, m.w, labelH))
			m.hintW = append(m.hintW, float64(hw))
		default:
			m.hints = append(m.hints, nil)
			m.hintW = append(m.hintW, 0)
		}
		m.maskH = append(m.maskH, labelH)
		m.rowTop = append(m.rowTop, y)
		m.rowH = append(m.rowH, height)
		y += height
	}
	m.h = int(math.Round(y + m.mm.padEnd))
	m.alloc()
}

// layout works out the panel's size for the scale it is being drawn at: the plate's width
// in screen pixels, and every measurement the rows are laid out with. It is called before
// the rows are measured, because a mask is as wide as the plate it will be laid into.
func (m *menuState) layout() {
	if m.scale <= 0 {
		m.scale = 1
	}
	m.mm = metricsFor(m.scale)
	m.w = int(math.Round(menuWidth * m.scale))
}

// dp scales one design measurement to screen pixels. The layout constants are already in
// m.mm; this is for the small measurements a row works out for itself, which are written
// where they are used (a chevron's arm, the gap beside a hint).
func (m *menuState) dp(v float64) float64 {
	if m.scale <= 0 {
		return math.Round(v)
	}
	return math.Round(v * m.scale)
}

// alloc makes sure the panel's own buffers exist at the size it is laid out at,
// and repaints its background: a different size is a different panel.
func (m *menuState) alloc() {
	if m.w <= 0 || m.h <= 0 {
		return
	}
	if len(m.pix) == m.w*m.h*4 {
		return
	}
	m.pix = make([]uint8, m.w*m.h*4)
	m.bg = make([]uint8, m.w*m.h*4)
	m.background()
	m.all = true
}

// pending reports whether anything on the panel needs drawing again, which is what
// the frame loop asks before it does any work for it at all.
func (m *menuState) pending() bool { return m.all || m.chrome || len(m.marked) > 0 }

// mark is one row needing to be drawn again.
func (m *menuState) mark(row int) {
	if m.all || row < 0 {
		return
	}
	for _, got := range m.marked {
		if got == row {
			return
		}
	}
	m.marked = append(m.marked, row)
}

// markAll is the whole panel: a new page, a new layout, or a choice that changes
// what every row says.
func (m *menuState) markAll() {
	m.all = true
	m.chrome = false
	m.marked = m.marked[:0]
}

// pageTitle is the panel's own header.
func (m *menuState) pageTitle() string {
	switch m.page {
	case pageFlower:
		return "Your flower"
	case pageShape:
		return "Its shape"
	case pageDelivery:
		return "A drop delivers"
	case pageLooks:
		return "While carrying"
	case pagePlace:
		return "Where it stands"
	}
	return Name
}

// paint brings the panel's picture up to date and puts it on screen.
//
// It takes the focus only when the window has just appeared: taking it is a few
// syscalls and a "who is in front now?" each time, and a panel that is being
// dragged through is not a panel that needs to be re-focused sixty times a
// second.
func (m *menuState) paint(b *Buddy) bool {
	if m.panel == nil {
		return false
	}
	m.draw()
	if err := m.panel.BlitAt(m.x, m.y, m.pix, m.w, m.h); err != nil {
		fmt.Fprintln(os.Stderr, "buddy: menu:", err)
		return false
	}
	if !m.shown {
		m.shown = true
		m.panel.Show()
		win.TakeFocus(m.panel.Hwnd)
	}
	return true
}

// moveTo puts the panel at a new place on screen, keeping it on the desktop: it
// can be dragged anywhere, but never so far that there is nothing left to grab to
// bring it back.
func (m *menuState) moveTo(x, y int) {
	if m.panel == nil {
		return
	}
	area := win.PrimaryWorkArea()
	edge := int(m.dp(12))
	x = min(max(x, int(area.Left)-m.w+edge), int(area.Right)-edge)
	y = min(max(y, int(area.Top)-m.h+edge), int(area.Bottom)-edge)
	m.x, m.y = x, y
	m.panel.MoveTo(x, y)
}

// draw is the panel's picture, built from the part of it that never changes and
// the rows that have.
func (m *menuState) draw() {
	if m.pix == nil || m.bg == nil {
		return
	}
	if m.all {
		copy(m.pix, m.bg)
		m.drawChrome()
		for i := range m.rows {
			m.drawRow(i)
		}
		m.all, m.chrome = false, false
		m.marked = m.marked[:0]
		return
	}
	if m.chrome {
		m.drawChrome()
		m.chrome = false
	}
	for _, i := range m.marked {
		m.drawRow(i)
	}
	m.marked = m.marked[:0]
}

// panelPixels is the panel as pixels, drawn in full. Nothing here touches a
// window, which is what makes how the panel looks a thing that can be rendered
// and looked at on its own.
func (m *menuState) panelPixels() []uint8 {
	m.markAll()
	m.draw()
	return m.pix
}

// background paints the part of the panel that does not depend on the rows: the
// shadow it is lifted on, the plate, the hairline of light along its top edge, the
// accent under the title and the title itself.
func (m *menuState) background() {
	pix := m.bg
	for i := range pix {
		pix[i] = 0
	}
	// the shadow: the panel lifted off the desktop by a few millimetres
	pad := m.dp(4)
	for i := 8; i >= 1; i-- {
		step := m.dp(float64(i))
		roundRect(pix, m.w, m.h, pad, pad+step, float64(m.w)-pad, float64(m.h)+m.dp(2)-step,
			m.mm.radius+step*0.5, flower.RGB{0, 0, 0}, 0.045)
	}
	// the panel, and a hairline of light along its top edge
	roundRect(pix, m.w, m.h, pad, m.dp(3), float64(m.w)-pad, float64(m.h), m.mm.radius, m.panelBG, 0.985)
	roundRect(pix, m.w, m.h, pad, m.dp(3), float64(m.w)-pad, float64(m.h), m.mm.radius, flower.RGB{255, 255, 255}, 0.06)
	roundRect(pix, m.w, m.h, m.dp(5), pad, float64(m.w)-m.dp(5), float64(m.h)-m.dp(1), m.mm.radius-m.dp(1), m.panelBG, 0.99)
	// and the title, on its own: an accent bar used to be drawn under it, and it ran
	// through the first few letters of the title — a line across the word, which read
	// as a rendering fault rather than as decoration
	blendMask(pix, m.w, m.h, m.mm.marginX, m.mm.titleTop, m.titlePx, m.w, int(m.mm.titleH), m.ink, 0.97)
}

// closeRect is the cross's hit box, in pixels: a circle in the top right corner of
// the plate, level with the title and clear of every row.
func (m *menuState) closeRect() (x0, y0, x1, y1 float64) {
	r := m.dp(11)
	cx := float64(m.w) - m.mm.marginX - r - m.dp(2)
	cy := m.dp(24)
	return cx - r, cy - r, cx + r, cy + r
}

// overClose reports whether a point is on the cross, with a little slack so a
// corner of the plate is not a dead zone.
func (m *menuState) overClose(x, y float64) bool {
	x0, y0, x1, y1 := m.closeRect()
	slack := m.dp(3)
	return x >= x0-slack && x <= x1+slack && y >= y0-slack && y <= y1+slack
}

// drawChrome paints the top strip of the panel: the title it was given, and the
// cross that puts the panel away.
//
// It restores the strip from the background before drawing, so the cross's hover
// highlight cannot smear — the same trick a row uses, on the one band that is not
// a row.
func (m *menuState) drawChrome() {
	if m.pix == nil || m.bg == nil {
		return
	}
	// the strip ends where the first row's own band begins, so a hover on the
	// cross cannot reach down and rub out the top of a row's highlight
	y1 := min(m.h, int(m.mm.padTop)-int(m.dp(4)))
	if y1 > 0 {
		copy(m.pix[:y1*m.w*4], m.bg[:y1*m.w*4])
	}
	x0, y0, x1, y1b := m.closeRect()
	cx, cy := (x0+x1)/2, (y0+y1b)/2
	if m.closeHover {
		circle(m.pix, m.w, m.h, cx, cy, m.dp(13), danger, 0.18)
	}
	// the cross is one of the ways out, so it is drawn in the red and never in the
	// panel's ordinary ink: a hover only makes it brighter
	ink := danger
	if m.closeHover {
		ink = dangerHot
	}
	arm, wide, stroke := m.dp(4.5), m.dp(4.5), m.dp(1.9)
	line(m.pix, m.w, m.h, cx-arm, cy-wide, cx+arm, cy+wide, stroke, ink)
	line(m.pix, m.w, m.h, cx-arm, cy+wide, cx+arm, cy-wide, stroke, ink)
}

// drawRow puts one row back on the panel: the band it lives in goes back to the
// background it was drawn over, and then the row is drawn into it. Doing it this
// way — rather than remembering what was there before — is what keeps a hover from
// leaving a smear behind it.
func (m *menuState) drawRow(i int) {
	if i < 0 || i >= len(m.rows) {
		return
	}
	top, height := m.rowTop[i], m.rowH[i]
	y0 := max(0, int(top)-int(m.dp(4)))
	y1 := min(m.h, int(top+height)+int(m.dp(8)))
	copy(m.pix[y0*m.w*4:y1*m.w*4], m.bg[y0*m.w*4:y1*m.w*4])

	pix, r := m.pix, m.rows[i]
	{
		if r.plate != nil {
			// a button: the whole row is the colour, the label is laid on it in white,
			// and the only thing that moves is the light — a hover brightens the plate,
			// a press darkens it
			x0, x1 := m.mm.marginX, float64(m.w)-m.mm.marginX
			inset := m.dp(3)
			roundRect(pix, m.w, m.h, x0, top+inset, x1, top+height-inset, m.dp(10), *r.plate, 1)
			if i == m.hover {
				roundRect(pix, m.w, m.h, x0, top+inset, x1, top+height-inset, m.dp(10), flower.RGB{255, 255, 255}, 0.10)
			}
			if i == m.press {
				roundRect(pix, m.w, m.h, x0, top+inset, x1, top+height-inset, m.dp(10), flower.RGB{0, 0, 0}, 0.16)
			}
			labelX := (float64(m.w) - m.hintW[i]) / 2
			if r.swatch != nil {
				// a colourway shows its colour twice: the swatch is the colour itself, at
				// the tip a petal ends in, and the plate is that colour at button depth —
				// so the name sits beside the swatch rather than over it
				circle(pix, m.w, m.h, m.mm.labelPad+m.mm.swatchR, top+height/2, m.mm.swatchR+m.dp(1.5), flower.RGB{0, 0, 0}, 0.35)
				circle(pix, m.w, m.h, m.mm.labelPad+m.mm.swatchR, top+height/2, m.mm.swatchR, *r.swatch, 1)
				labelX = m.mm.labelPad + m.mm.swatchR*2 + m.dp(9)
			}
			blendMask(pix, m.w, m.h, labelX, top, m.masks[i], m.w, m.maskH[i], flower.RGB{255, 255, 255}, 1)
			if r.on && r.swatch != nil {
				// the one that is on: a white tick, because the accent colour is very
				// likely the colour the row itself is painted in
				x := float64(m.w) - m.mm.marginX - m.dp(20)
				cy := top + height/2
				white := flower.RGB{255, 255, 255}
				line(pix, m.w, m.h, x-m.dp(7), cy, x-m.dp(2.5), cy+m.dp(5), m.dp(2.1), white)
				line(pix, m.w, m.h, x-m.dp(2.5), cy+m.dp(5), x+m.dp(6), cy-m.dp(5), m.dp(2.1), white)
			}
			return
		}
		switch r.kind {
		case rowHeading:
			blendMask(pix, m.w, m.h, m.mm.marginX, top, m.masks[i], m.w, m.maskH[i], m.dim, 0.85)
			return
		case rowSep:
			roundRect(pix, m.w, m.h, m.mm.marginX+m.dp(2), top+height/2,
				float64(m.w)-m.mm.marginX-m.dp(2), top+height/2+m.dp(1), m.dp(0.5),
				flower.RGB{255, 255, 255}, 0.07)
			return
		}
		hovered := i == m.hover
		pressed := i == m.press
		bg := 0.055
		if hovered {
			bg = 0.10
		}
		if pressed {
			bg = 0.16
		}
		if bg > 0 {
			roundRect(pix, m.w, m.h, m.mm.marginX, top+m.dp(1), float64(m.w)-m.mm.marginX, top+height-m.dp(1),
				m.dp(9), flower.RGB{255, 255, 255}, bg)
		}
		labelX := m.mm.labelPad
		if r.swatch != nil {
			circle(pix, m.w, m.h, m.mm.labelPad+m.mm.swatchR, top+height/2, m.mm.swatchR+m.dp(1.5), flower.RGB{0, 0, 0}, 0.35)
			circle(pix, m.w, m.h, m.mm.labelPad+m.mm.swatchR, top+height/2, m.mm.swatchR, *r.swatch, 1)
			labelX = m.mm.labelPad + m.mm.swatchR*2 + m.dp(9)
		}
		// a row with a colour of its own is drawn in it: a colourway's name in the
		// colourway, and the rows that leave the panel in the red
		ink := m.ink
		if r.ink != nil {
			ink = *r.ink
		}
		blendMask(pix, m.w, m.h, labelX, top, m.masks[i], m.w, m.maskH[i], ink, 0.95)

		switch r.kind {
		case rowSlider:
			m.drawSlider(pix, r, top, height)
		case rowToggle:
			// a switch, drawn rather than ticked: it says "on" without a legend
			x1 := float64(m.w) - m.mm.marginX - m.dp(46)
			x2 := float64(m.w) - m.mm.marginX - m.dp(8)
			cy := top + height/2
			colour := flower.RGB{84, 74, 92}
			if r.on {
				colour = m.accent
			}
			roundRect(pix, m.w, m.h, x1, cy-m.dp(9), x2, cy+m.dp(9), m.dp(9), colour, 0.95)
			knob := x1 + m.dp(10)
			if r.on {
				knob = x2 - m.dp(10)
			}
			circle(pix, m.w, m.h, knob, cy, m.dp(7), flower.RGB{252, 250, 253}, 1)
			circle(pix, m.w, m.h, knob, cy, m.dp(7), flower.RGB{0, 0, 0}, 0.10)
		case rowPick:
			if r.on {
				// a tick, as two strokes of a round-capped line
				x := float64(m.w) - m.mm.marginX - m.dp(20)
				cy := top + height/2
				line(pix, m.w, m.h, x-m.dp(7), cy, x-m.dp(2.5), cy+m.dp(5), m.dp(2.1), m.accent)
				line(pix, m.w, m.h, x-m.dp(2.5), cy+m.dp(5), x+m.dp(6), cy-m.dp(5), m.dp(2.1), m.accent)
			}
		case rowPage:
			// the chevron is drawn in the row's own ink, so a Back in red has a red
			// arrow beside it rather than a grey one pointing at nothing
			x := float64(m.w) - m.mm.marginX - m.dp(18)
			cy := top + height/2
			line(pix, m.w, m.h, x, cy-m.dp(5), x+m.dp(5), cy, m.dp(2.1), ink)
			line(pix, m.w, m.h, x+m.dp(5), cy, x, cy+m.dp(5), m.dp(2.1), ink)
		}
		if m.hints[i] != nil {
			hx := float64(m.w) - m.mm.marginX - m.dp(30) - m.hintW[i]
			if r.kind == rowSlider && r.swatch == nil {
				// a slider's value is its own right-hand column, not a hint squeezed
				// in beside a chevron
				hx = float64(m.w) - m.mm.marginX - m.dp(4) - m.hintW[i]
			}
			blendMask(pix, m.w, m.h, hx, top, m.hints[i], m.w, m.maskH[i], m.dim, 0.9)
		}
	}
}

// drawSlider paints a slider's track, the part of it that is turned up, and the
// knob: a filled bar is a value at a glance, which is the whole reason a slider
// beats a number.
func (m *menuState) drawSlider(pix []uint8, r row, top, height float64) {
	x0, x1 := m.mm.labelPad, float64(m.w)-m.mm.marginX
	cy := top + height - m.dp(12)
	t := 0.0
	if r.hi > r.lo {
		t = clampFloat((r.value()-r.lo)/(r.hi-r.lo), 0, 1)
	}
	half := m.mm.trackH / 2
	roundRect(pix, m.w, m.h, x0, cy-half, x1, cy+half, half, flower.RGB{58, 50, 64}, 0.95)
	knob := x0 + (x1-x0)*t
	if knob > x0+half {
		roundRect(pix, m.w, m.h, x0, cy-half, knob, cy+half, half, m.accent, 0.95)
	}
	circle(pix, m.w, m.h, knob, cy, m.mm.knobR+m.dp(1.5), flower.RGB{0, 0, 0}, 0.30)
	circle(pix, m.w, m.h, knob, cy, m.mm.knobR, flower.RGB{250, 247, 252}, 1)
	circle(pix, m.w, m.h, knob, cy, m.mm.knobR, m.accent, 0.25)
}

// rowAt is the row under a point in the panel, or -1.
func (m *menuState) rowAt(x, y float64) int {
	edge := m.dp(4)
	if x < edge || x > float64(m.w)-edge || y < 0 || y > float64(m.h) {
		return -1
	}
	for i := range m.rows {
		if m.rows[i].kind == rowHeading || m.rows[i].kind == rowSep {
			continue
		}
		if y >= m.rowTop[i] && y < m.rowTop[i]+m.rowH[i] {
			return i
		}
	}
	return -1
}

// onMessage is everything the panel does with the mouse and the keyboard.
func (m *menuState) onMessage(b *Buddy, msg win.MSG) bool {
	switch msg.Message {
	case win.WM_SETCURSOR:
		// Only the inside of the panel: an edge, a corner or a border belongs to
		// Windows, and gets Windows' own answer.
		//
		// The whole plate shows the same cursor, and it is the pointing finger: every
		// part of it does something — a row is clicked, the cross is clicked, and the
		// rest of it is the handle the panel is dragged by — and the finger is the
		// signal the whole machine uses for "there is something here", whether that
		// something is a click or a drag.
		if win.HitTest(msg.LParam) != win.HTCLIENT {
			return false
		}
		win.SetCursor(win.CursorHand)
		return true
	case win.WM_MOUSEMOVE:
		x, y := clientPoint(msg.LParam)
		if m.panelDrag {
			// the panel follows the pointer. The cursor is read from Windows rather
			// than from the message: the panel is moving, so its own client
			// coordinates shift under the pointer and would chase themselves
			cur := win.CursorPosition()
			m.moveTo(int(cur.X)-m.dragDX, int(cur.Y)-m.dragDY)
			return true
		}
		if m.drag >= 0 {
			// a slider follows the pointer wherever it goes inside the row, and the
			// capture keeps the messages coming when it wanders out of the panel
			m.setSlider(b, m.drag, x)
			return true
		}
		if m.overClose(x, y) {
			if !m.closeHover {
				m.closeHover, m.chrome = true, true
			}
			if m.hover != -1 {
				m.mark(m.hover)
				m.hover = -1
			}
			return true
		}
		if m.closeHover {
			m.closeHover, m.chrome = false, true
		}
		if h := m.rowAt(x, y); h != m.hover {
			// only the two rows involved: the one the pointer left, and the one it
			// arrived on. A hover is not a reason to draw the whole panel
			m.mark(m.hover)
			m.mark(h)
			m.hover = h
		}
		return true
	case win.WM_LBUTTONDOWN:
		x, y := clientPoint(msg.LParam)
		if m.overClose(x, y) {
			b.closeMenu()
			return true
		}
		m.press = m.rowAt(x, y)
		m.mark(m.press)
		switch {
		case m.onSlider(m.press):
			// one click puts the knob where the pointer is: a slider you have to
			// drag from exactly the right pixel is a puzzle, not a setting
			m.drag = m.press
			m.capture()
			m.setSlider(b, m.drag, x)
		case m.press < 0:
			// the background — the title, a heading, the plate between the rows —
			// is the panel's handle: take hold of it and the panel goes with the
			// pointer, which is how it gets put wherever you want it
			cur := win.CursorPosition()
			m.dragDX, m.dragDY = int(cur.X)-m.x, int(cur.Y)-m.y
			m.panelDrag = true
			m.capture()
		}
		return true
	case win.WM_LBUTTONUP:
		x, y := clientPoint(msg.LParam)
		if m.panelDrag {
			m.panelDrag = false
			win.ReleaseCapture()
			return true
		}
		h := m.rowAt(x, y)
		pressed := m.press
		m.press = -1
		m.mark(pressed)
		m.mark(h)
		if m.drag >= 0 {
			dragged := m.drag
			m.drag = -1
			win.ReleaseCapture()
			// now the pointer has stopped: the window and the tray icon catch up,
			// and the choice is written down
			b.changed()
			m.measure(b)
			_ = dragged
			return true
		}
		if h >= 0 && h == pressed {
			m.choose(b, h)
		}
		return true
	case win.WM_KEYDOWN:
		switch uint32(msg.WParam) {
		case win.VK_ESCAPE:
			b.closeMenu()
		case win.VK_UP:
			m.step(-1)
		case win.VK_DOWN:
			m.step(1)
		case win.VK_TAB:
			if msg.WParam != 0 {
				m.step(1)
			} else {
				m.step(-1)
			}
		case win.VK_RETURN, win.VK_SPACE:
			if m.hover >= 0 {
				m.choose(b, m.hover)
			}
		case win.VK_LEFT, win.VK_RIGHT:
			if m.onSlider(m.hover) {
				sign := 1.0
				if uint32(msg.WParam) == win.VK_LEFT {
					sign = -1
				}
				r := m.rows[m.hover]
				m.putSlider(b, m.hover, r.value()+sign*r.step*4)
				b.changed()
			}
		case win.VK_BACK:
			if m.page != pageMain {
				m.page = pageMain
				m.hover = -1
				m.rebuild(b)
			}
		}
		return true
	case win.WM_ACTIVATE:
		// the low word is WA_INACTIVE: something else has the focus now, so the
		// panel is done — this is also what a click anywhere else does
		if uint16(msg.WParam) == 0 {
			b.closeMenu()
		}
		return true
	case win.WM_KILLFOCUS, win.WM_DESTROY:
		b.closeMenu()
		return true
	}
	return false
}

// capture routes the mouse to the panel until the button comes up, so a drag that
// wanders off the plate still arrives. A panel with no window behind it has nothing
// to capture with, and is fine: this is a menu somebody has opened, or a test.
func (m *menuState) capture() {
	if m.panel != nil {
		win.SetCapture(m.panel.Hwnd)
	}
}

// onSlider reports whether a row is a slider, which is what the mouse and the
// arrow keys both ask before they do anything.
func (m *menuState) onSlider(i int) bool {
	return i >= 0 && i < len(m.rows) && m.rows[i].kind == rowSlider
}

// setSlider is a drag: the pointer's x turns into the row's value, which is
// written into the settings and shown straight away. Anything that cannot be done
// at 60 drags a second (the window's own size, the tray icon) waits for the
// pointer to be let go — see the mouse-up handler.
func (m *menuState) setSlider(b *Buddy, i int, x float64) {
	if !m.onSlider(i) {
		return
	}
	r := m.rows[i]
	x0, x1 := m.mm.labelPad, float64(m.w)-m.mm.marginX
	t := clampFloat((x-x0)/(x1-x0), 0, 1)
	m.putSlider(b, i, r.lo+t*(r.hi-r.lo))
}

// putSlider quantises a value onto the row's steps, writes it into the settings
// and lays the panel out again with the new value on it.
func (m *menuState) putSlider(b *Buddy, i int, v float64) {
	if !m.onSlider(i) {
		return
	}
	r := m.rows[i]
	if r.step > 0 {
		v = math.Round(v/r.step) * r.step
	}
	v = clampFloat(v, r.lo, r.hi)
	if r.set != nil {
		r.set(b, v)
	}
	m.measure(b)
	// the number the slider shows is the only thing on the panel that changed
	m.mark(i)
}

// step moves the keyboard's idea of the cursor.
func (m *menuState) step(delta int) {
	n := len(m.rows)
	if n == 0 {
		return
	}
	i := m.hover
	for k := 0; k < n; k++ {
		i += delta
		if i < 0 {
			i = n - 1
		}
		if i >= n {
			i = 0
		}
		if m.rows[i].kind != rowHeading && m.rows[i].kind != rowSep {
			m.mark(m.hover)
			m.hover = i
			m.mark(i)
			return
		}
	}
}

// choose runs a row.
func (m *menuState) choose(b *Buddy, i int) {
	if i < 0 || i >= len(m.rows) {
		return
	}
	r := m.rows[i]
	switch r.kind {
	case rowPage:
		if r.page == pageMain && m.page == pageMain {
			return
		}
		m.page = r.page
		m.hover = -1
		m.rebuild(b)
	case rowToggle, rowPick:
		if r.act != nil {
			r.act(b, m)
		}
		b.save()
		m.rebuild(b)
	case rowAction, rowCoffee:
		if r.act != nil {
			r.act(b, m)
		}
		// a row that changed something without closing (resetting the settings,
		// say) gets the panel laid out again; one that closed the panel is done
		if b.menu == m && m.panel != nil {
			m.rebuild(b)
		}
	}
}

// coffeeURL is the page the coffee button opens.
const coffeeURL = "https://buymeacoffee.com/fazebunssa"

// openURL is how the coffee button reaches the browser; a test swaps it out so pressing
// the button there does not launch one.
var openURL = win.OpenURL

// coffee is the Buy me a Coffee button at the foot of the panel: it opens the coffee page
// in the default browser. What it must never do is change a setting — it is not a
// setting, it is a door.
func (b *Buddy) coffee() {
	if err := openURL(coffeeURL); err != nil {
		b.trace("the coffee button could not open its page: " + err.Error())
		return
	}
	b.trace("the coffee button opened " + coffeeURL)
}

// rebuild lays the panel out again after a choice, keeping it where it is.
func (m *menuState) rebuild(b *Buddy) {
	x, y := m.x, m.y
	m.measure(b)
	if m.panel == nil {
		// a panel with no window behind it — a test, or a menu that never managed
		// to open — is still a panel: lay it out and let the caller paint it
		m.markAll()
		return
	}
	if m.w != m.panel.W || m.h != m.panel.H {
		// a layered window's bitmap is fixed when it is made, so a panel that has
		// changed size is a *new* window
		m.panel.Destroy()
		panel, err := win.NewPopupWindow(m.w, m.h, win.MenuClass, func(msg win.MSG) bool { return m.onMessage(b, msg) })
		if err != nil {
			b.closeMenu()
			return
		}
		m.panel = panel
		// ...and a new window has to be *shown*, and told to take the focus again,
		// exactly as the first one was. A window that is created, blitted into and
		// never shown has a picture in it, sits on top of everything and is
		// invisible — which is what made opening a page look like the click had
		// been swallowed and the panel had closed itself.
		m.shown = false
		area := win.PrimaryWorkArea()
		gap := int(m.dp(8))
		if y+m.h > int(area.Bottom)-gap {
			y = int(area.Bottom) - m.h - gap
		}
		if x+m.w > int(area.Right)-gap {
			x = int(area.Right) - m.w - gap
		}
	}
	m.x, m.y = x, y
	m.markAll()
}

// menuPage builds the rows of a page. State is read here, so the ticks are right.
func (b *Buddy) menuPage(page int) []row {
	rows := []row{}
	if page != pageMain {
		// one arrow and no hint: the key that goes back is a shortcut, not something to
		// read, and a row with the Backspace glyph in it *and* a chevron beside it looked
		// like two arrows pointing at nothing
		rows = append(rows, row{kind: rowPage, label: "Back", page: pageMain, ink: &danger}, row{kind: rowSep})
	}
	// the coffee button goes at the foot of the panel, whatever the page, and it is
	// put there in one place rather than in each page's list so that no page can be
	// built without it (see the end of this function)
	switch page {
	case pageFlower:
		rows = append(rows, row{kind: rowHeading, label: "Colour"})
		for _, name := range flower.PaletteNames {
			// both of these turn with the colour wheel, so a row is always the colour the
			// flower would actually be: the swatch is the colour its petals are tipped
			// with, and the row itself is painted in that colour at button depth, which is
			// what makes it a button showing its colour rather than a word about one
			pal := flower.HueTurn(flower.PaletteNamed(name), b.cfg.Shape.Hue)
			colour, plate := pal.PetalTip, plateInk(pal.PetalBase)
			rows = append(rows, row{
				kind: rowPick, label: pal.Label, on: name == b.cfg.Palette, swatch: &colour, plate: &plate,
				act: func(b *Buddy, _ *menuState) {
					b.cfg.Palette = name
					b.grow()
				},
			})
		}
		rows = append(rows,
			// the wheel: four colourways shown, every colour there is underneath
			b.dial("Turn the colour wheel", -180, 180, 5,
				func() float64 { return b.cfg.Shape.Hue },
				func(bud *Buddy, v float64) { bud.cfg.Shape.Hue = v },
				fmtDegrees),
			row{kind: rowSep},
			row{kind: rowHeading, label: "This flower"},
			row{kind: rowAction, label: "Grow a different one", hint: fmt.Sprintf("#%d", b.cfg.Seed), act: func(b *Buddy, m *menuState) {
				b.cfg.Seed++
				b.grow()
				m.rebuild(b)
			}},
			row{
				// how tall the flower stands on screen. It is the one dial that is
				// about the window rather than the plant: the box it fills is what
				// the window is derived from, so this is where "make it bigger" went
				kind: rowSlider, label: "How tall it stands", lo: 130, hi: 420, step: 2,
				value: func() float64 { return float64(b.cfg.Height) },
				set: func(bud *Buddy, v float64) {
					if bud.cfg.Height <= 0 {
						bud.cfg.Height = 250
					}
					ratio := v / float64(bud.cfg.Height)
					bud.cfg.Height = int(v + 0.5)
					bud.cfg.Width = int(float64(bud.cfg.Width)*ratio + 0.5)
					bud.reframe()
				},
				write: fmtPixels,
			},
		)

	case pageShape:
		s := &b.cfg.Shape
		rows = append(rows,
			row{kind: rowHeading, label: "The stalk"},
			b.dial("How long the stem is", 0.6, 1.6, 0.02,
				func() float64 { return s.Stem }, func(bud *Buddy, v float64) { bud.cfg.Shape.Stem = v }, fmtTimes),
			b.dial("How thick the stem is", 0.6, 1.6, 0.02,
				func() float64 { return s.StemWidth }, func(bud *Buddy, v float64) { bud.cfg.Shape.StemWidth = v }, fmtTimes),
			b.dial("How much it sways", 0.2, 2, 0.02,
				func() float64 { return s.Wind }, func(bud *Buddy, v float64) { bud.cfg.Shape.Wind = v }, fmtTimes),
			b.dial("How big the leaves are", 0.5, 1.6, 0.02,
				func() float64 { return s.Leaves }, func(bud *Buddy, v float64) { bud.cfg.Shape.Leaves = v }, fmtTimes),
			row{kind: rowSep},
			row{kind: rowHeading, label: "The bloom"},
			b.dial("How many petals", 8, 30, 1,
				func() float64 { return float64(s.Petals) }, func(bud *Buddy, v float64) { bud.cfg.Shape.Petals = int(v + 0.5) }, fmtPetals),
			b.dial("How big the bloom is", 0.6, 1.5, 0.02,
				func() float64 { return s.Bloom }, func(bud *Buddy, v float64) { bud.cfg.Shape.Bloom = v }, fmtTimes),
			b.dial("How wide it opens", 0.5, 1.8, 0.02,
				func() float64 { return s.Spread }, func(bud *Buddy, v float64) { bud.cfg.Shape.Spread = v }, fmtTimes),
			b.dial("How much it flutters", 0, 2.5, 0.05,
				func() float64 { return s.Flutter }, func(bud *Buddy, v float64) { bud.cfg.Shape.Flutter = v }, fmtTimes),
			row{kind: rowSep},
			row{kind: rowHeading, label: "The head"},
			b.dial("How floppy it is", 0.4, 1.6, 0.02,
				func() float64 { return s.Limp }, func(bud *Buddy, v float64) { bud.cfg.Shape.Limp = v }, fmtTimes),
		)

	case pageDelivery:
		rows = append(rows, row{kind: rowHeading, label: "When I let go"},
			toggleRow("Type the note in", !b.cfg.CopyOnly, func(b *Buddy) { b.cfg.CopyOnly = !b.cfg.CopyOnly }),
			toggleRow("Interrupt a busy chat", b.cfg.Interrupt, func(b *Buddy) { b.cfg.Interrupt = !b.cfg.Interrupt }),
			toggleRow("Press Enter for me", b.cfg.AutoSend, func(b *Buddy) { b.cfg.AutoSend = !b.cfg.AutoSend }),
			toggleRow("Caret at the drop", b.cfg.Caret, func(b *Buddy) { b.cfg.Caret = !b.cfg.Caret }),
		)

	case pageLooks:
		rows = append(rows,
			row{kind: rowHeading, label: "While you carry it"},
			toggleRow("A chime when it lands on a chat", b.cfg.Sound, func(b *Buddy) { b.cfg.Sound = !b.cfg.Sound }),
			toggleRow("Ring around the window", b.cfg.Ring, func(b *Buddy) {
				b.cfg.Ring = !b.cfg.Ring
				if !b.cfg.Ring && b.high != nil {
					b.high.hide()
				}
			}),
			toggleRow("Sunshine now and then", b.cfg.Sunshine, func(b *Buddy) {
				b.cfg.Sunshine = !b.cfg.Sunshine
				if b.cfg.Sunshine {
					b.sunlight(shinePass)
				}
			}),
			toggleRow("Petals on a delivery", b.cfg.Burst, func(b *Buddy) { b.cfg.Burst = !b.cfg.Burst }),
			row{kind: rowSep},
			row{kind: rowAction, label: "Sunshine now", act: func(b *Buddy, _ *menuState) { b.sunlight(shinePass) }},
			row{kind: rowAction, label: "Petals now", act: func(b *Buddy, _ *menuState) {
				b.burstPower, b.burstVX, b.burstVY = 1.2, 0, 0
				b.burstPending = true
			}},
		)

	case pagePlace:
		rows = append(rows, row{kind: rowHeading, label: "Its own spot"})
		spot := "not set yet"
		if b.cfg.Home {
			spot = fmt.Sprintf("%d, %d", b.cfg.HomeX, b.cfg.HomeY)
		}
		rows = append(rows,
			row{kind: rowAction, label: "Set this spot as home", hint: spot, act: func(b *Buddy, m *menuState) {
				b.setHomeHere()
				b.save()
			},
			},
			row{kind: rowAction, label: "Forget my spot", act: func(b *Buddy, m *menuState) {
				b.cfg.Home = false
				b.placeHome()
				b.homeward()
				b.save()
			}},
			row{kind: rowSep},
			row{kind: rowHeading, label: "Its corner"},
		)
		for _, name := range CornerNames {
			name := name
			rows = append(rows, row{
				kind: rowPick, label: CornerLabel(name), on: b.cfg.Corner == name && !b.cfg.Home,
				act: func(b *Buddy, _ *menuState) {
					// asking for a corner asks for the corner: a spot of its own would
					// otherwise quietly win and the choice would look broken
					b.cfg.Home = false
					b.cfg.Corner = name
					b.placeHome()
					// standing still, the hand is the ground the flower stands on:
					// put it in the new corner and let the frame loop place the window
					b.handX, b.handY = float64(b.homeX), float64(b.homeY)
					b.walking = false
				},
			})
		}
		rows = append(rows,
			row{kind: rowSep},
			row{kind: rowAction, label: "Come home now", act: func(b *Buddy, _ *menuState) { b.homeward() }},
		)

	default:
		// the pages, each saying its current answer on the right, so the main page
		// is also a summary of the app
		deliveryHint := "types it in"
		if b.cfg.CopyOnly {
			deliveryHint = "to clipboard"
		}
		rows = append(rows,
			row{kind: rowPage, label: "Colour", hint: flower.PaletteNamed(b.cfg.Palette).Label, page: pageFlower},
			row{kind: rowPage, label: "Shape", hint: fmtTimes(b.cfg.Shape.Stem) + " stem", page: pageShape},
			row{kind: rowPage, label: "Delivery", hint: deliveryHint, page: pageDelivery},
			row{kind: rowPage, label: "Looks", page: pageLooks},
			row{kind: rowPage, label: "Position", hint: b.homeLabel(), page: pagePlace},
			row{kind: rowSep},
			row{kind: rowAction, label: "About " + Name, act: func(b *Buddy, _ *menuState) {
				b.closeMenu()
				win.MessageBox(Name, aboutText())
			}},
			row{kind: rowAction, label: "Reset all settings", act: func(b *Buddy, _ *menuState) {
				b.resetSettings()
			}},
		)
	}
	// the foot of the panel is a line and its two doors: the coffee button on every page —
	// a green plate under everything the app itself has to say — and, on the page that has
	// one, the way out of the app *under* it, so the last thing on the panel is the last
	// thing it does
	rows = append(rows, row{kind: rowSep}, row{kind: rowCoffee, label: coffeeLabel, plate: &coffeePlate,
		act: func(b *Buddy, _ *menuState) { b.coffee() }})
	if page == pageMain {
		rows = append(rows, row{kind: rowAction, label: "Quit " + Name, plate: &quitPlate,
			act: func(b *Buddy, _ *menuState) {
				b.quit = true
				win.Quit()
			}})
	}
	return rows
}

// homeLabel is where the flower goes back to, in three words: a spot somebody
// picked, or the corner it lives in.
func (b *Buddy) homeLabel() string {
	if b.cfg.Home {
		return "my spot"
	}
	return CornerLabel(b.cfg.Corner)
}

// toggleRow is a switch row. The label names the thing and the switch carries the
// state: a label that changes with the setting ("press Enter" / "I will press
// it") makes a person read the whole row to work out which way it is, and moves
// the row they were about to click.
func toggleRow(label string, state bool, flip func(*Buddy)) row {
	return row{
		kind: rowToggle, label: label, on: state,
		act: func(b *Buddy, _ *menuState) { flip(b) },
	}
}

// dial is one of the flower's own settings, as a slider.
//
// The value is read and written straight through the settings, and every change
// regrows the flower without re-planting it: that is what makes the panel feel
// like a hand on the plant rather than a form. The heavy half — the window the
// flower lives in, the tray icon, the settings file — happens when the pointer is
// let go (see the mouse-up handler).
func (b *Buddy) dial(label string, lo, hi, step float64, get func() float64, set func(*Buddy, float64), write func(float64) string) row {
	return row{
		kind: rowSlider, label: label, lo: lo, hi: hi, step: step,
		value: get,
		set: func(bud *Buddy, v float64) {
			set(bud, v)
			bud.regrow()
		},
		write: write,
	}
}

// How a slider's value is written: the same in the panel as in the settings file,
// so "1.00×" always means the flower as it ships.
func fmtTimes(v float64) string  { return strconv.FormatFloat(v, 'f', 2, 64) + "×" }
func fmtPetals(v float64) string { return strconv.Itoa(int(v + 0.5)) }
func fmtPixels(v float64) string { return strconv.Itoa(int(v+0.5)) + " px" }

func fmtDegrees(v float64) string {
	sign := ""
	if v > 0 {
		sign = "+"
	}
	return sign + strconv.Itoa(int(v+0.5)) + "°"
}

// grow plants a new flower with the settings as they are now.
func (b *Buddy) grow() {
	b.newRig()
	b.clock = 0 // and it grows again from a seedling, which is the point
	b.changed()
}

// regrow is the same flower with the shape dials where they are now, without
// planting it again: this is what a slider drags, and a flower that re-sprouted on
// every mouse move would be unusable. The window, the tray icon and the settings
// file wait for the pointer to be let go (see the mouse-up handler), which is what
// keeps a drag smooth.
func (b *Buddy) regrow() {
	b.newRig()
	b.reframeFast()
}

// resetSettings puts every choice back to its default, keeping the flags this run
// was started with.
func (b *Buddy) resetSettings() {
	flight := b.cfg
	b.cfg = DefaultConfig()
	b.cfg.FPS, b.cfg.Detail, b.cfg.Workers = flight.FPS, flight.Detail, flight.Workers
	b.cfg.Stats, b.cfg.Trace, b.cfg.Dry, b.cfg.Opaque, b.cfg.Dump = flight.Stats, flight.Trace, flight.Dry, flight.Opaque, flight.Dump
	b.newRig()
	b.clock = 0
	b.reframeFast()
	b.placeHome()
	b.handX, b.handY = float64(b.homeX), float64(b.homeY)
	b.walking = false
	b.rebuildWindow()
	if icon, err := b.renderIcon(32); err == nil {
		b.icon = icon
		b.refreshIcon(icon)
	}
	b.sunlight(shinePass)
	b.save()
	b.requestFrame()
}

// aboutText is what the about box says. It is written here rather than stored, so
// that the version in it is the version this build was stamped with.
func aboutText() string {
	return Name + " " + Version + "\n\n" +
		"A flower you can pick up and put on someone's chat.\n\n" +
		"Drag it onto any window and let go: it works out what is underneath, " +
		"puts the caret where you dropped it, and types a note. Click the tray " +
		"icon for these settings.\n\n" +
		"Everything it draws is its own: no toolkit, no runtime, one small exe."
}

// save writes the choices out, and says so if it cannot. A settings file that
// cannot be written is worth a line on the console, but never worth stopping for:
// the flower keeps working either way.
func (b *Buddy) save() {
	if err := b.cfg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "buddy: could not save settings: %v\n", err)
	}
}

// changed is save() plus everything that has to be redone when the flower itself
// changes shape: the window it lives in, its tray icon, and a little light.

func (b *Buddy) changed() {
	// Where the flower is standing, said the one way that survives a change of
	// shape: the base of the stem, which is the point it grows from and the point
	// it stands on. A new seed, a new colourway or a new size leaves it there.
	if !b.dragging && b.win != nil {
		baseX, baseY := b.standingPoint()
		b.reframeFast()
		b.handX, b.handY = float64(baseX), float64(baseY)
	} else {
		b.reframeFast()
	}
	b.rebuildWindow()
	if icon, err := b.renderIcon(32); err == nil {
		b.icon = icon
		b.refreshIcon(icon)
	}
	b.sunlight(shinePass)
	b.save()
	b.requestFrame()
}

// requestFrame asks for the flower's *exact* envelope — the one measured by
// driving the whole carry model, which is about a third of a second of arithmetic
// — without doing it here.
//
// It is done on another goroutine because of where it is called from: the moment a
// slider is let go of, inside the window's own message handling and with the frame
// clock running. A third of a second there is a freeze you can see on every
// release, and the flower does not need the answer that badly: the fast framing it
// already has is the right shape, a fraction of a percent out on the window's
// size. The measured one arrives a frame or two later and quietly takes over.
func (b *Buddy) requestFrame() {
	// everything the measurement needs is read here, on the main thread, and the
	// rig it is measured from is immutable — so the goroutine holds no part of the
	// running flower but a pointer to a plant that will not change
	rig, w, h, fit := b.rig, b.cfg.Width, b.cfg.Height, b.cfg.Fit
	go func() {
		b.framing.put(framingJob{rig: rig, frame: measureFrame(rig, w, h, fit)})
	}()
}

// measureFrame is the exact-envelope measurement, behind a variable so that a test
// can watch the hand-off without paying a third of a second for it.
var measureFrame = flower.FrameFor

// takeFrame applies a measured framing if one is ready and it is still the flower
// it was measured for: a flower re-grown while the measurement was in flight must
// not be given the shape of the flower before it.
func (b *Buddy) takeFrame() bool {
	job := b.framing.take()
	if job == nil || job.rig != b.rig {
		return false
	}
	b.applyFrame(*job)
	return true
}

// applyFrame takes a measured framing as the flower's own, on the frame loop.
func (b *Buddy) applyFrame(job framingJob) {
	// where it is standing now, so that a window that changed size does not move
	// the flower off its corner
	atHome := false
	if b.win != nil && !b.dragging && !b.walking {
		bx, by := b.standingPoint()
		atHome = abs(bx-b.homeX) <= 2 && abs(by-b.homeY) <= 2
	}
	b.fitFrame(job.frame)
	b.rebuildWindow()
	if icon, err := b.renderIcon(32); err == nil {
		b.icon = icon
		b.refreshIcon(icon)
	}
	if atHome {
		b.placeHome()
		b.handX, b.handY = float64(b.homeX), float64(b.homeY)
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// resizeFlower changes how big the flower stands on screen. The window it needs is
// a consequence, not a setting: see FrameFor.
func (b *Buddy) resizeFlower(factor float64) {
	height := float64(b.cfg.Height) * factor
	if height < 110 || height > 460 {
		return
	}
	ratio := height / float64(b.cfg.Height)
	b.cfg.Height = int(height + 0.5)
	b.cfg.Width = int(float64(b.cfg.Width)*ratio + 0.5)
	b.changed()
}

// standingPoint is where the base of the stem is on screen right now, which is
// the point the flower stands on.
func (b *Buddy) standingPoint() (int, int) {
	return b.posX + int(math.Round(b.basePixX)), b.posY + int(math.Round(b.basePixY))
}

// rebuildWindow replaces the window when the flower's box changes size: a layered
// window's bitmap is fixed the moment it is created, so a different size means a
// different window, and the tray icon has to be handed the new one.
//
// The flower itself does not move: what the user is looking at is the flower, not
// the window around it. It is put back by the same rule every frame uses — the
// hand stays where it was, so whatever the hand is holding stays where it was.
func (b *Buddy) rebuildWindow() {
	if b.win == nil {
		return
	}
	// a window is already the size the flower asks for: rebuilding it would be a
	// new bitmap, a new tray entry and a visible blink for nothing, and the exact
	// framing that lands a moment after the fast one usually lands here
	if b.winFrameW == b.frame.Width && b.winFrameH == b.frame.Height {
		return
	}
	oldW, oldH := b.win.W, b.win.H
	wasDragging := b.dragging

	b.win.Hide()
	b.win.Destroy()
	window, err := win.NewLayeredWindow(b.frame.Width, b.frame.Height, b.onMessage)
	if err != nil {
		b.trace("could not rebuild the window: %v", err)
		return
	}
	b.win = window
	b.winFrameW, b.winFrameH = b.frame.Width, b.frame.Height
	b.posX, b.posY = b.follow(flower.PoseOf(b.rig, b.clock, b.carry.Motion()))
	if b.tray != nil {
		b.tray.Remove()
	}
	if tray, err := win.AddTray(window.Hwnd, b.icon, trayTip); err == nil {
		b.tray = tray
	}
	window.Show()
	b.win.Raise()
	b.dragging = wasDragging
	b.blitReported = false
	b.dumped = false
	b.trace("rebuilt the window: %dx%d -> %dx%d", oldW, oldH, b.frame.Width, b.frame.Height)
}

// setHomeHere makes where the flower is standing right now the place it goes back
// to: the base of its stem, recorded in screen pixels. It is the whole of the
// "put it where I want it and have it stay there" feature.
func (b *Buddy) setHomeHere() {
	x, y := b.standingPoint()
	b.cfg.HomeX, b.cfg.HomeY = clampToDesk(x, y)
	b.cfg.Home = true
}

// clampToDesk keeps a spot somebody picked on the desk. The base of the stem is the
// point that gets placed, so that is the point that is kept inside the work area:
// a spot the flower can never be fetched back from is not a spot, it is a bug.
func clampToDesk(x, y int) (int, int) {
	area := win.PrimaryWorkArea()
	x = min(max(x, int(area.Left)), int(area.Right))
	y = min(max(y, int(area.Top)), int(area.Bottom))
	return x, y
}

// placeHome is where the flower stands: a spot of its own if the settings have one,
// and otherwise the base of its stem in the corner they ask for, with the flower's
// own outline kept inside the work area by a margin. The window around it is mostly
// air and is allowed to hang off the edge of the screen, because nobody can see air.
func (b *Buddy) placeHome() {
	// the work area the flower is standing on right now, recorded so that a change
	// of screen can be told apart from a message about nothing (see refitDisplay),
	// and so that a spot of somebody's own keeps its share of the desk
	b.deskArea = win.PrimaryWorkArea()
	if b.cfg.Home {
		b.homeX, b.homeY = clampToDesk(b.cfg.HomeX, b.cfg.HomeY)
		return
	}
	// the corner itself is pure arithmetic over the work area — the same corner at
	// 720p, at 4K and at 8K — and lives in place.go, where it is tested
	b.homeX, b.homeY = standPoint(b.cfg.Corner, b.deskArea,
		flower.OutlineBox(b.rig, flower.Motion{}), b.pixelScale())
}

// The drawing the panel is made of: rounded rectangles, circles and lines, all
// antialiased by coverage rather than by supersampling, since a menu is mostly
// straight edges and does not move.

func roundRect(pix []uint8, w, h int, x0, y0, x1, y1, r float64, colour flower.RGB, alpha float64) {
	for y := int(math.Floor(y0)) - 1; y <= int(math.Ceil(y1))+1; y++ {
		if y < 0 || y >= h {
			continue
		}
		for x := int(math.Floor(x0)) - 1; x <= int(math.Ceil(x1))+1; x++ {
			if x < 0 || x >= w {
				continue
			}
			d := roundRectDistance(float64(x)+0.5, float64(y)+0.5, x0, y0, x1, y1, r)
			cov := math.Min(1, math.Max(0, 0.5-d))
			if cov <= 0.002 {
				continue
			}
			blendPixel(pix, w, x, y, colour, alpha*cov)
		}
	}
}

// roundRectDistance is the distance from a point to a rounded rectangle: negative
// inside, positive outside, which is what makes the edge one line of code.
func roundRectDistance(px, py, x0, y0, x1, y1, r float64) float64 {
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	cx := math.Min(math.Max(px, x0+r), x1-r)
	cy := math.Min(math.Max(py, y0+r), y1-r)
	return math.Hypot(px-cx, py-cy) - r
}

func circle(pix []uint8, w, h int, cx, cy, r float64, colour flower.RGB, alpha float64) {
	for y := int(cy - r - 1); y <= int(cy+r+1); y++ {
		if y < 0 || y >= h {
			continue
		}
		for x := int(cx - r - 1); x <= int(cx+r+1); x++ {
			if x < 0 || x >= w {
				continue
			}
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy) - r
			cov := math.Min(1, math.Max(0, 0.5-d))
			if cov <= 0.002 {
				continue
			}
			blendPixel(pix, w, x, y, colour, alpha*cov)
		}
	}
}

// line is a round-capped stroke, which is what a tick and a chevron are made of.
func line(pix []uint8, w, h int, x0, y0, x1, y1, width float64, colour flower.RGB) {
	minX := int(math.Min(x0, x1) - width)
	maxX := int(math.Max(x0, x1) + width)
	minY := int(math.Min(y0, y1) - width)
	maxY := int(math.Max(y0, y1) + width)
	dx, dy := x1-x0, y1-y0
	len2 := dx*dx + dy*dy
	for y := minY; y <= maxY; y++ {
		if y < 0 || y >= h {
			continue
		}
		for x := minX; x <= maxX; x++ {
			if x < 0 || x >= w {
				continue
			}
			px, py := float64(x)+0.5, float64(y)+0.5
			t := 0.0
			if len2 > 1e-9 {
				t = math.Min(1, math.Max(0, ((px-x0)*dx+(py-y0)*dy)/len2))
			}
			d := math.Hypot(px-(x0+dx*t), py-(y0+dy*t)) - width/2
			cov := math.Min(1, math.Max(0, 0.5-d))
			if cov <= 0.002 {
				continue
			}
			blendPixel(pix, w, x, y, colour, cov)
		}
	}
}

// blendMask lays a text coverage mask into the panel at (x, y).
func blendMask(pix []uint8, w, h int, x, y float64, mask []uint8, mw, mh int, colour flower.RGB, alpha float64) {
	if mask == nil {
		return
	}
	for row := 0; row < mh; row++ {
		py := int(y) + row
		if py < 0 || py >= h {
			continue
		}
		base := row * mw
		for col := 0; col < mw; col++ {
			cov := float64(mask[base+col]) / 255
			if cov <= 0.004 {
				continue
			}
			px := int(x) + col
			if px < 0 || px >= w {
				continue
			}
			blendPixel(pix, w, px, py, colour, alpha*cov)
		}
	}
}

// blendPixel mixes a colour into one pixel of a straight (non-premultiplied) RGBA
// buffer. The buffer starts empty, so a pixel being written for the first time
// takes the colour whole.
func blendPixel(pix []uint8, w, x, y int, colour flower.RGB, a float64) {
	if a <= 0 {
		return
	}
	if a > 1 {
		a = 1
	}
	i := (y*w + x) * 4
	dstA := float64(pix[i+3]) / 255
	outA := a + dstA*(1-a)
	if outA <= 0 {
		return
	}
	for c := 0; c < 3; c++ {
		out := (colour[c]*a + float64(pix[i+c])*dstA*(1-a)) / outA
		pix[i+c] = uint8(math.Min(255, math.Max(0, out+0.5)))
	}
	pix[i+3] = uint8(outA*255 + 0.5)
}

// clientPoint unpacks a mouse message's coordinates.
func clientPoint(lparam win.LPARAM) (float64, float64) {
	x := int16(uint16(lparam))
	y := int16(uint16(uint32(lparam) >> 16))
	return float64(x), float64(y)
}

// clampFloat keeps a value inside a range, which sliders do by definition.
func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// hexColour reads the "#rrggbb" an item carries as its accent.
func hexColour(s string) flower.RGB {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return flower.RGB{200, 180, 255}
	}
	parse := func(at int) float64 {
		v, err := strconv.ParseUint(s[at:at+2], 16, 8)
		if err != nil {
			return 0
		}
		return float64(v)
	}
	return flower.RGB{parse(0), parse(2), parse(4)}
}
