package buddy

import (
	"bytes"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FaZeBuns/pocket-petal/internal/flower"
	"github.com/FaZeBuns/pocket-petal/internal/win"
)

// The settings panel is drawn by hand, which means it can be checked by hand —
// as pixels, without a desktop. That matters more here than anywhere else in the
// app: it is the one screen a person reads rather than watches, and a mask laid
// in at the wrong stride, a row that is off the bottom of the plate or a switch
// drawn off its track are all invisible until somebody opens the menu.

// testMenu builds a panel for a page without a window behind it. Only the page's
// rows and the drawing need the Buddy, and neither touches the window.
func testMenu(t *testing.T, b *Buddy, page int) *menuState {
	t.Helper()
	return testMenuAt(t, b, page, 1)
}

// testMenuAt is the same panel drawn at a scale. The plate follows the flower's own scale
// (see panelScale), and a test that only ever looked at 1x would not see a panel that
// failed to grow with the screen it is on.
func testMenuAt(t *testing.T, b *Buddy, page int, scale float64) *menuState {
	t.Helper()
	m := &menuState{
		page:    page,
		accent:  flower.PaletteNamed(b.cfg.Palette).PetalTip,
		dim:     flower.RGB{158, 146, 168},
		ink:     flower.RGB{240, 235, 245},
		panelBG: flower.RGB{24, 19, 27},
		scale:   scale,
		hover:   -1,
		press:   -1,
		drag:    -1,
	}
	var err error
	if m.title, err = win.NewFont(int(m.dp(19)), win.FontBold); err != nil {
		t.Skipf("no fonts on this machine: %v", err)
	}
	if m.body, err = win.NewFont(int(m.dp(15)), win.FontMedium); err != nil {
		t.Fatal(err)
	}
	if m.small, err = win.NewFont(int(m.dp(12)), win.FontRegular); err != nil {
		t.Fatal(err)
	}
	if m.button, err = win.NewFont(int(m.dp(24)), win.FontBold); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		m.closeFonts()
	})
	m.measure(b)
	return m
}

func testBuddy(palette, note, corner string) *Buddy {
	cfg := DefaultConfig()
	cfg.Palette, cfg.Note, cfg.Corner = palette, note, corner
	return &Buddy{cfg: cfg, rig: flower.CreateRig(flower.RigOptions{Seed: &cfg.Seed, Palette: palette})}
}

// everyPage is the panel's whole table of contents.
var everyPage = []int{pageMain, pageFlower, pageDelivery, pageLooks, pagePlace}

// coffeeRowAt is which row holds the coffee button: it is at the foot of every page, but
// on the main page the way out of the app sits under it, so it is not always the last row.
func coffeeRowAt(rows []row) int {
	for i, r := range rows {
		if r.kind == rowCoffee {
			return i
		}
	}
	return -1
}

func TestSettingsPanelDrawsEveryPage(t *testing.T) {
	b := testBuddy("buttercup", "duck", CornerTopRight)
	for _, page := range everyPage {
		m := testMenu(t, b, page)
		pix := m.panelPixels()

		// the plate: one opaque rounded rectangle, drawn as dark as the phone
		// menus it is imitating
		opaque := 0
		for i := 3; i < len(pix); i += 4 {
			if pix[i] > 230 {
				opaque++
			}
		}
		if share := float64(opaque) / float64(len(pix)/4); share < 0.85 {
			t.Errorf("page %d: only %.0f%% of the panel is drawn", page, share*100)
		}
		for _, corner := range [][2]int{{2, 2}, {m.w - 3, 2}, {2, m.h - 3}, {m.w - 3, m.h - 3}} {
			if a := pix[(corner[1]*m.w+corner[0])*4+3]; a > 8 {
				t.Errorf("page %d: the corner at %v is not rounded (%d alpha)", page, corner, a)
			}
		}

		// every row that says something has ink where it says it
		for i, r := range m.rows {
			if r.kind == rowSep {
				continue
			}
			top := int(m.rowTop[i])
			height := int(m.rowH[i])
			if top < 0 || top+height > m.h {
				t.Errorf("page %d: row %q sits at y %d..%d in a %d pixel panel", page, r.label, top, top+height, m.h)
			}
			if r.label == "" {
				continue
			}
			if !hasInk(pix, m.w, m.h, layoutPad-4, top, m.w, top+height, 0.5) {
				t.Errorf("page %d: row %q has no text in it", page, r.label)
			}
			if r.kind == rowToggle && !hasInk(pix, m.w, m.h, m.w-int(menuMarginX)-52, top+4, m.w, top+height-4, 0.5) {
				t.Errorf("page %d: the switch on %q was not drawn", page, r.label)
			}
		}

		// The title is drawn, and nothing is drawn across it. An accent bar used to be
		// painted under the title in the flower's colour and it ran through the first
		// letters of the word — a line across a title reads as a rendering fault, so the
		// plate's own background must hold no accent colour anywhere at all.
		if !hasInk(pix, m.w, m.h, int(menuMarginX), int(menuPadTop)-36, int(menuMarginX)+160, int(menuPadTop)-4, 0.5) {
			t.Errorf("page %d: no title at the top of the panel", page)
		}
		if at, ok := accentPixel(m); ok {
			t.Errorf("page %d: the plate's background carries the accent colour at %d,%d, which is a bar through the title",
				page, at%m.w, at/m.w)
		}
	}
}

// A hover repaints the row it is over and touches nothing else, which is the whole
// of why the panel can be dragged through: a full repaint is eight milliseconds of
// rounded rectangles, and this is two. The promise is exact — every pixel outside
// the row's own band must be byte for byte what it was.
func TestAHoverRepaintsItsOwnRowAndNothingElse(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)
	m := testMenu(t, b, pageShape)
	before := append([]uint8{}, m.panelPixels()...)

	row := -1
	for i, r := range m.rows {
		if r.kind == rowSlider || r.kind == rowToggle || r.kind == rowPick || r.kind == rowPage {
			row = i
			break
		}
	}
	if row < 0 {
		t.Skip("that page has nothing to hover")
	}
	m.hover = row
	m.mark(row)
	m.draw()

	top, height := m.rowTop[row], m.rowH[row]
	bandTop, bandBottom := int(top)-4, int(top+height)+8
	for y := 0; y < m.h; y++ {
		if y >= bandTop && y < bandBottom {
			continue
		}
		for x := 0; x < m.w; x++ {
			at := (y*m.w + x) * 4
			if !bytes.Equal(before[at:at+4], m.pix[at:at+4]) {
				t.Fatalf("repainting one row changed the pixel at %d,%d", x, y)
			}
		}
	}
	// and inside the band the row really was drawn again: a row that is hovered is
	// a row with a highlight on it
	differs := 0
	for y := max(0, bandTop); y < min(m.h, bandBottom); y++ {
		for x := 0; x < m.w; x++ {
			at := (y*m.w + x) * 4
			if !bytes.Equal(before[at:at+4], m.pix[at:at+4]) {
				differs++
			}
		}
	}
	if differs < 200 {
		t.Errorf("the hovered row changed %d pixels: it was not drawn", differs)
	}
}

// lparam packs a client point the way a mouse message carries one, so a test can
// poke the panel through the same door the mouse does.
func lparam(x, y int) win.LPARAM {
	return win.LPARAM(uint16(x)) | win.LPARAM(uint16(y))<<16
}

// The cross in the corner is how the panel is put away without the keyboard, so it
// has to be drawn, and clicking it has to work.
func TestTheCrossInTheCornerPutsThePanelAway(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)
	m := testMenu(t, b, pageMain)
	b.menu = m
	pix := m.panelPixels()

	x0, y0, x1, y1 := m.closeRect()
	if y1 > menuPadTop-4 {
		t.Fatalf("the cross reaches to y %.0f, into the first row's band", y1)
	}
	if x0 < 4 || x1 > float64(m.w)-4 {
		t.Fatalf("the cross sits at x %.0f..%.0f in a %d pixel panel", x0, x1, m.w)
	}
	if !hasInk(pix, m.w, m.h, int(x0), int(y0), int(x1), int(y1), 0.5) {
		t.Fatal("the cross was never drawn")
	}

	cx, cy := int((x0+x1)/2), int((y0+y1)/2)
	if !m.onMessage(b, win.MSG{Message: win.WM_LBUTTONDOWN, LParam: lparam(cx, cy)}) {
		t.Fatal("the panel ignored a click on its own cross")
	}
	if b.menu != nil {
		t.Fatal("clicking the cross did not put the panel away")
	}
}

// A hover on the cross repaints the strip it lives in and nothing else, which is
// the same bargain the rows make: the panel can be moved under the pointer without
// redrawing itself.
func TestHoveringTheCrossRepaintsOnlyTheTitleStrip(t *testing.T) {
	b := testBuddy("buttercup", "flower", CornerTopLeft)
	m := testMenu(t, b, pageShape)
	before := append([]uint8{}, m.panelPixels()...)

	x0, y0, x1, y1 := m.closeRect()
	m.onMessage(b, win.MSG{Message: win.WM_MOUSEMOVE, LParam: lparam(int((x0+x1)/2), int((y0+y1)/2))})
	if !m.closeHover {
		t.Fatal("the panel did not notice the pointer on its cross")
	}
	m.draw()

	// everything from the first row's band down is untouched
	for y := int(menuPadTop) - 4; y < m.h; y++ {
		for x := 0; x < m.w; x++ {
			at := (y*m.w + x) * 4
			if !bytes.Equal(before[at:at+4], m.pix[at:at+4]) {
				t.Fatalf("hovering the cross changed the pixel at %d,%d", x, y)
			}
		}
	}
	// and inside the strip the cross really did light up
	differs := 0
	for y := 0; y < int(menuPadTop)-4; y++ {
		for x := 0; x < m.w; x++ {
			at := (y*m.w + x) * 4
			if !bytes.Equal(before[at:at+4], m.pix[at:at+4]) {
				differs++
			}
		}
	}
	if differs < 50 {
		t.Errorf("the hovered cross changed %d pixels: it was not drawn", differs)
	}
	// and moving off it puts the strip back exactly as it was
	m.onMessage(b, win.MSG{Message: win.WM_MOUSEMOVE, LParam: lparam(10, 200)})
	m.draw()
	for y := 0; y < int(menuPadTop)-4; y++ {
		for x := 0; x < m.w; x++ {
			at := (y*m.w + x) * 4
			if !bytes.Equal(before[at:at+4], m.pix[at:at+4]) {
				t.Fatalf("the cross left a mark behind at %d,%d", x, y)
			}
		}
	}
}

// The plate between the rows, the title and the headings are the panel's handle:
// pressing one of them takes hold of the whole panel so it can be put anywhere.
func TestTheBackgroundIsHowThePanelIsMoved(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)
	m := testMenu(t, b, pageMain)
	b.menu = m

	// a press in the title strip, clear of the cross
	x0, _, _, _ := m.closeRect()
	grabX := int(x0) - 20
	m.onMessage(b, win.MSG{Message: win.WM_LBUTTONDOWN, LParam: lparam(grabX, 20)})
	if !m.panelDrag {
		t.Fatal("pressing the background did not pick the panel up")
	}
	m.onMessage(b, win.MSG{Message: win.WM_LBUTTONUP, LParam: lparam(grabX, 20)})
	if m.panelDrag {
		t.Fatal("the panel is still being dragged after the button came up")
	}

	// but a press on a row is that row, not a drag of the whole panel
	row := -1
	for i, r := range m.rows {
		if r.kind == rowPage || r.kind == rowToggle || r.kind == rowSlider || r.kind == rowAction || r.kind == rowPick {
			row = i
			break
		}
	}
	if row < 0 {
		t.Skip("that page has no rows to press")
	}
	at := int(m.rowTop[row] + m.rowH[row]/2)
	m.onMessage(b, win.MSG{Message: win.WM_LBUTTONDOWN, LParam: lparam(m.w-60, at)})
	if m.panelDrag {
		t.Fatal("pressing a row dragged the whole panel")
	}
}

// testLiveMenu is testMenu with a real window behind it, off the screen: the only
// way to check the parts of the panel that are a window rather than pixels — that
// choosing a row that changes the panel's size leaves a panel you can still see.
func testLiveMenu(t *testing.T, b *Buddy, page int) *menuState {
	t.Helper()
	m := testMenu(t, b, page)
	panel, err := win.NewPopupWindow(m.w, m.h, win.MenuClass, func(msg win.MSG) bool { return m.onMessage(b, msg) })
	if err != nil {
		t.Skipf("no popup windows on this machine: %v", err)
	}
	t.Cleanup(func() {
		if m.panel != nil {
			m.panel.Destroy()
		}
	})
	m.panel = panel
	// far off the screen: the panel is not what is being looked at
	m.x, m.y = -6000, -6000
	return m
}

// A click is a press and a release on the same row, and it has to run the row.
// Choosing a sub-page changes the panel's size, which means the window is replaced
// — and the replacement has to end up on screen and focused, or the menu appears to
// swallow the click and then vanish.
func TestClickingASubPageOpensItAndLeavesAPanelYouCanSee(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)
	t.Cleanup(redirectSettings(filepath.Join(t.TempDir(), "settings.json")))
	m := testLiveMenu(t, b, pageMain)
	b.menu = m
	if !m.paint(b) {
		t.Fatal("the panel would not paint")
	}
	if !m.panel.Visible() {
		t.Fatal("the panel is not on screen after opening")
	}

	row := -1
	for i, r := range m.rows {
		if r.kind == rowPage {
			row = i
			break
		}
	}
	if row < 0 {
		t.Skip("the main page has no sub-pages on it")
	}
	want := m.rows[row].page
	x, y := m.w/2, int(m.rowTop[row]+m.rowH[row]/2)
	m.onMessage(b, win.MSG{Hwnd: m.panel.Hwnd, Message: win.WM_LBUTTONDOWN, LParam: lparam(x, y)})
	m.onMessage(b, win.MSG{Hwnd: m.panel.Hwnd, Message: win.WM_LBUTTONUP, LParam: lparam(x, y)})
	if m.page != want {
		t.Fatalf("clicking %q left the panel on page %d, and it should be on %d", m.rows[row].label, m.page, want)
	}
	if !m.paint(b) {
		t.Fatal("the panel would not paint after the page changed")
	}
	if !m.panel.Visible() {
		t.Fatal("the panel is invisible after a choice changed its size: the click looks like it was swallowed")
	}
}

// And a switch really is thrown by a click: press, release, and the setting is the
// other way round.
func TestClickingASwitchThrowsIt(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)
	t.Cleanup(redirectSettings(filepath.Join(t.TempDir(), "settings.json")))
	m := testLiveMenu(t, b, pageDelivery)
	b.menu = m
	m.paint(b)

	row := -1
	for i, r := range m.rows {
		if r.kind == rowToggle {
			row = i
			break
		}
	}
	if row < 0 {
		t.Skip("the delivery page has no switches on it")
	}
	was := m.rows[row].on
	x, y := m.w-30, int(m.rowTop[row]+m.rowH[row]/2)
	m.onMessage(b, win.MSG{Hwnd: m.panel.Hwnd, Message: win.WM_LBUTTONDOWN, LParam: lparam(x, y)})
	m.onMessage(b, win.MSG{Hwnd: m.panel.Hwnd, Message: win.WM_LBUTTONUP, LParam: lparam(x, y)})

	now := -1
	for _, r := range m.rows {
		if r.kind == rowToggle && r.label == m.rows[row].label {
			if r.on {
				now = 1
			} else {
				now = 0
			}
		}
	}
	if now < 0 {
		t.Fatal("the switch disappeared after being clicked")
	}
	if (now == 1) == was {
		t.Fatalf("clicking the switch left it %v", was)
	}
}

// The pointer says the thing under it is there to be used: the flower and every part
// of the settings panel show the same pointing finger, whether the thing under it is
// clicked (a row) or picked up and moved (the flower, the plate of the panel). A
// window that answers with nothing at all gets the plain arrow, which says there is
// nothing here to do — and there is always something here to do.
func TestThePointerSaysTheThingUnderItIsThereToBeUsed(t *testing.T) {
	// the system's own cursor, and loaded: a zero handle is "no cursor at all",
	// which would leave the pointer showing whatever it happened to be showing
	if win.CursorArrow == 0 || win.CursorHand == 0 {
		t.Fatalf("the system cursors did not load: arrow %#x, hand %#x", win.CursorArrow, win.CursorHand)
	}

	b := testBuddy("blush", "flower", CornerBottomRight)
	// the flower: everything the pointer can be over on its window is the flower
	// (its window is layered, so an empty pixel is not part of it at all), and the
	// whole of it is something to pick up
	if !b.onMessage(win.MSG{Message: win.WM_SETCURSOR, LParam: win.LPARAM(win.HTCLIENT)}) {
		t.Error("the flower did not answer for its own cursor")
	}
	if b.onMessage(win.MSG{Message: win.WM_SETCURSOR, LParam: 2 /* HTCAPTION */}) {
		t.Error("the flower answered for a point outside its own client area")
	}

	// and the panel answers for its own client area, and only its client area: an
	// edge, a corner or a border belongs to Windows and gets Windows' own answer
	m := testMenu(t, b, pageShape)
	b.menu = m
	if !m.onMessage(b, win.MSG{Message: win.WM_SETCURSOR, LParam: win.LPARAM(win.HTCLIENT)}) {
		t.Error("the panel did not answer for its own cursor")
	}
	if m.onMessage(b, win.MSG{Message: win.WM_SETCURSOR, LParam: 2 /* HTCAPTION */}) {
		t.Error("the panel answered for a point outside its own client area")
	}
}

// hasInk reports whether anything was drawn in a rectangle, which is what "the
// text is there" means once the text is a coverage mask.
func hasInk(pix []uint8, w, h int, x0, y0, x1, y1 int, threshold float64) bool {
	for y := max(0, y0); y < min(h, y1); y++ {
		for x := max(0, x0); x < min(w, x1); x++ {
			if pix[(y*w+x)*4+3] > 0 && pixelCoversInk(pix, y*w+x) > threshold {
				return true
			}
		}
	}
	return false
}

// pixelCoversInk is roughly how bright a pixel is, which for this panel means how
// much of it is text.
func pixelCoversInk(pix []uint8, i int) float64 {
	i *= 4
	return (float64(pix[i]) + float64(pix[i+1]) + float64(pix[i+2])) / (3 * 255)
}

// accentPixel looks for the flower's own colour anywhere in the plate's background, which
// is where the accent bar under the title used to be painted. It is asked of the background
// rather than of the finished panel because the rows are drawn over it, and a row is
// allowed to be the flower's colour.
func accentPixel(m *menuState) (int, bool) {
	for i := 0; i+3 < len(m.bg); i += 4 {
		if m.bg[i+3] == 0 {
			continue
		}
		if nearColour(m.bg, i/4, m.accent, 6) {
			return i / 4, true
		}
	}
	return 0, false
}

// nearColour reports whether the pixel at an index is a colour, within a tolerance: a
// coverage mask blended at 0.95 of a colour lands a unit or two off it.
func nearColour(pix []uint8, index int, want flower.RGB, tol float64) bool {
	at := index * 4
	if at+3 >= len(pix) {
		return false
	}
	for c := 0; c < 3; c++ {
		if math.Abs(float64(pix[at+c])-want[c]) > tol {
			return false
		}
	}
	return true
}

// hasColour reports whether anything in a rectangle is drawn in a colour, which is how a
// label that is supposed to be red is checked: the text is a coverage mask, so the colour
// is the row's own and it is the only thing that says a row is a way out.
func hasColour(pix []uint8, w, h int, x0, y0, x1, y1 int, want flower.RGB, tol float64) bool {
	for y := max(0, y0); y < min(h, y1); y++ {
		for x := max(0, x0); x < min(w, x1); x++ {
			if nearColour(pix, y*w+x, want, tol) {
				return true
			}
		}
	}
	return false
}

// inkExtent is the leftmost and rightmost pixel a rectangle has text on, and -1 when it has
// none. It is what "the label is centred" is measured with: the ink's own middle against the
// middle of the button.
func inkExtent(pix []uint8, w, h, y0, y1 int) (int, int) {
	left, right := -1, -1
	for y := max(0, y0); y < min(h, y1); y++ {
		for x := 0; x < w; x++ {
			at := y*w + x
			if pix[at*4+3] == 0 || pixelCoversInk(pix, at) <= 0.5 {
				continue
			}
			if left < 0 || x < left {
				left = x
			}
			if x > right {
				right = x
			}
		}
	}
	return left, right
}

// The rows are the panel's whole logic: which page, which choice is ticked, and
// what the main page says the app is set to. None of that needs a window.
func TestSettingsPanelRowsFollowTheSettings(t *testing.T) {
	b := testBuddy("periwinkle", "cookie", CornerTopLeft)

	main := b.menuPage(pageMain)
	// the foot of the panel is its two doors: the coffee button, and the way out of the
	// app under it (see TestEveryPageHasTheCoffeeButtonAtItsFoot)
	if got := main[len(main)-1]; got.label != "Quit "+Name {
		t.Errorf("the last row of the main page is %q, and it should be the way out", got.label)
	}
	if got := main[len(main)-2]; got.kind != rowCoffee {
		t.Errorf("the row above the way out is %q, and it should be the coffee button", got.label)
	}
	hints := map[string]string{}
	for _, r := range main {
		hints[r.label] = r.hint
	}
	if hints["Colour"] != "Periwinkle" || hints["Delivery"] != "types it in" || hints["Position"] != "Top left" {
		t.Errorf("the main page does not say what the app is set to: %+v", hints)
	}
	if hints["Shape"] != "1.00× stem" {
		t.Errorf("the shape row should say how the flower is shaped, and says %q", hints["Shape"])
	}
	for _, r := range main {
		if r.label == "Back" {
			t.Error("the main page has nothing to go back to")
		}
	}

	for _, page := range []int{pageFlower, pageDelivery, pageLooks, pagePlace} {
		rows := b.menuPage(page)
		if len(rows) < 4 || rows[0].label != "Back" || rows[0].page != pageMain {
			t.Fatalf("page %d does not start with a way back: %+v", page, rows)
		}
	}

	// exactly one colourway is ticked, and it is the one in the settings
	ticked := 0
	for _, r := range b.menuPage(pageFlower) {
		if r.kind == rowPick && r.label == flower.PaletteNamed("periwinkle").Label {
			if !r.on {
				t.Error("the colourway in the settings is not the ticked one")
			}
			ticked++
		} else if r.kind == rowPick && r.on {
			t.Errorf("%q is ticked as well", r.label)
		}
	}
	if ticked != 1 {
		t.Fatalf("the colourway in the settings appears %d times", ticked)
	}

	// and the switches say what the settings say
	switches := map[string]bool{}
	for _, r := range b.menuPage(pageDelivery) {
		if r.kind == rowToggle {
			switches[r.label] = r.on
		}
	}
	if !switches["Press Enter for me"] || !switches["Interrupt a busy chat"] || !switches["Caret at the drop"] {
		t.Errorf("switches do not match the defaults: %+v", switches)
	}
	if !switches["Type the note in"] {
		t.Errorf("the note is typed by default, but that switch is off: %+v", switches)
	}
	b.cfg.AutoSend = false
	found := false
	for _, r := range b.menuPage(pageDelivery) {
		if r.kind == rowToggle && r.label == "Press Enter for me" {
			found = true
			if r.on {
				t.Error("a switch left on after the setting was turned off")
			}
		}
	}
	if !found {
		t.Error("the panel stopped offering the Enter switch")
	}
}

// A page that changes what the flower looks like has to stand on its own: the
// flower's own rows come from the rig, and a broken one would offer a colourway
// that cannot be drawn.
func TestSettingsPanelOffersEveryColourway(t *testing.T) {
	b := testBuddy("blush", "", CornerBottomRight)
	rows := b.menuPage(pageFlower)
	offered := map[string]bool{}
	for _, r := range rows {
		if r.kind == rowPick {
			if r.swatch == nil {
				t.Errorf("%q has no swatch to show", r.label)
			}
			offered[r.label] = true
		}
	}
	for _, name := range flower.PaletteNames {
		if want := flower.PaletteNamed(name).Label; !offered[want] {
			t.Errorf("the panel does not offer %q", want)
		}
	}
}

// The coffee button: the one row in the panel that is not a setting. It has to be at the
// foot of every page — a page without it is a page where the door is missing — and
// pressing it must change nothing at all, because it is not a setting and a button that
// quietly wrote to the settings file would be the worst of both.
func TestEveryPageHasTheCoffeeButtonAtItsFoot(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)
	pages := append(append([]int{}, everyPage...), pageShape)
	for _, page := range pages {
		rows := b.menuPage(page)
		if len(rows) == 0 {
			t.Fatalf("page %d has no rows", page)
		}
		at := -1
		for i, r := range rows {
			if r.kind == rowCoffee {
				at = i
			}
		}
		if at < 0 {
			t.Fatalf("page %d has no coffee button on it: the door is missing", page)
		}
		// the foot of the panel is the coffee button on every page, and on the main page
		// the way out of the app sits under it: the last two rows are the two doors
		want := len(rows) - 1
		if page == pageMain {
			want = len(rows) - 2
			if last := rows[len(rows)-1]; last.label != "Quit "+Name {
				t.Errorf("the row under the coffee button is %q, and it should be the way out", last.label)
			}
		}
		if at != want {
			t.Errorf("page %d puts the coffee button at row %d of %d", page, at, len(rows))
		}
		row := rows[at]
		if row.label != coffeeLabel {
			t.Errorf("page %d calls the button %q", page, row.label)
		}
		if !strings.Contains(row.label, "☕") {
			t.Errorf("the button is %q, and the cup is the part of it a person looks for", row.label)
		}
		if !strings.Contains(strings.ToLower(row.label), "coffee") {
			t.Errorf("the button says %q, and it is supposed to ask for a coffee", row.label)
		}
		if row.act == nil {
			t.Errorf("page %d has a coffee button that cannot be pressed", page)
		}
		if row.plate == nil || *row.plate != coffeeGreen {
			t.Errorf("page %d paints the coffee button %v rather than the green", page, row.plate)
		}
	}
}

// The button as pixels: the one green thing in the panel, at the foot of every page, with a
// cup written on it. The colour is checked against the plate rather than taken on trust —
// the whole point of the button is that it looks like a door and not like a setting.
func TestTheCoffeeButtonIsGreenWithACupOnIt(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)
	for _, page := range append(append([]int{}, everyPage...), pageShape) {
		m := testMenu(t, b, page)
		pix := m.panelPixels()
		row := coffeeRowAt(m.rows)
		top, height := m.rowTop[row], m.rowH[row]
		// inside the plate and clear of the label: a little way in from the left end
		x, y := int(m.mm.marginX+m.dp(16)), int(top+height/2)
		at := (y*m.w + x) * 4
		if pix[at+3] < 200 {
			t.Fatalf("page %d: the button's plate is %d/255 opaque", page, pix[at+3])
		}
		for c := 0; c < 3; c++ {
			got, want := float64(pix[at+c]), coffeeGreen[c]
			if math.Abs(got-want) > 30 {
				t.Errorf("page %d: the button is #%02x%02x%02x and its green is #%02x%02x%02x",
					page, pix[at], pix[at+1], pix[at+2],
					uint8(coffeeGreen[0]), uint8(coffeeGreen[1]), uint8(coffeeGreen[2]))
				break
			}
		}
		// and the cup is on it: white ink over the middle of the button
		if !hasInk(pix, m.w, m.h, m.w/2-80, int(top)+2, m.w/2+80, int(top+height)-2, 0.55) {
			t.Errorf("page %d: the button has nothing written on it", page)
		}
		// the label is centred on the plate and fills it: the ink's own middle is the
		// button's middle, and it is a good share of the button's width rather than a
		// small word lost in the middle of a green bar
		left, right := inkExtent(pix, m.w, m.h, int(top), int(top+height))
		if left < 0 {
			t.Fatalf("page %d: the button has no label on it at all", page)
		}
		if middle := float64(left+right) / 2; math.Abs(middle-float64(m.w)/2) > 6 {
			t.Errorf("page %d: the label is centred at x=%.0f in a %d pixel button", page, middle, m.w)
		}
		if wide := float64(right - left); wide < float64(m.w)*0.4 {
			t.Errorf("page %d: the label is %.0f pixels wide in a %d pixel button, which does not fill it",
				page, wide, m.w)
		}
	}
}

// The panel is drawn at the flower's own scale, so the plate and the flower on the desk are
// the same size to the same person at any resolution: at twice the scale the plate is twice
// as wide, its rows and its type are twice as tall — and a row is still hit exactly where it
// is drawn, which is the part that would be silently wrong.
func TestThePanelIsDrawnAtTheFlowersScale(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)
	small := testMenuAt(t, b, pageMain, 1)
	big := testMenuAt(t, b, pageMain, 2)

	if big.w != 2*small.w {
		t.Errorf("the plate is %d pixels wide at 1x and %d at 2x", small.w, big.w)
	}
	if math.Abs(big.mm.rowH-2*small.mm.rowH) > 0.5 || math.Abs(big.mm.marginX-2*small.mm.marginX) > 0.5 {
		t.Errorf("the rows are %.0f tall and the side margin %.0f at 2x, and %.0f/%.0f at 1x",
			big.mm.rowH, big.mm.marginX, small.mm.rowH, small.mm.marginX)
	}
	_, smallType := small.button.Measure(coffeeLabel)
	_, bigType := big.button.Measure(coffeeLabel)
	if bigType <= smallType {
		t.Errorf("the button's type measures %d pixels tall at 1x and %d at 2x", smallType, bigType)
	}
	// and it is the panel's biggest face: the thing to press is not set in the type of a
	// line of settings
	if _, rowType := small.body.Measure("Colour"); bigType < rowType {
		t.Errorf("the button's type is %d pixels tall and a row's is %d", bigType, rowType)
	}

	for _, m := range []*menuState{small, big} {
		last := len(m.rows) - 1
		middle := m.rowTop[last] + m.rowH[last]/2
		if got := m.rowAt(float64(m.w)/2, middle); got != last {
			t.Errorf("at %.0fx the middle of the last row is hit as row %d", m.scale, got)
		}
		if middle > float64(m.h) {
			t.Errorf("at %.0fx the last row is at y=%.0f in a plate %d tall", m.scale, middle, m.h)
		}
		// the rows are all inside the plate, with the plate's own foot below them
		bottom := m.rowTop[last] + m.rowH[last] + m.mm.padEnd
		if math.Abs(bottom-float64(m.h)) > 1.5 {
			t.Errorf("at %.0fx the rows end at y=%.0f and the plate is %d tall", m.scale, bottom, m.h)
		}
	}
	// and it is drawn at 2x rather than an empty plate at twice the size
	pix := big.panelPixels()
	if !hasInk(pix, big.w, big.h, int(big.mm.marginX)+4, int(big.mm.padTop)+6,
		big.w-int(big.mm.marginX)-4, big.h-int(big.mm.padEnd)-6, 0.3) {
		t.Error("the panel at 2x has no ink on it")
	}
}

// An open panel follows the screen. A resolution change reaches refitDisplay — the one place
// a display change is noticed — and from there the plate is laid out again at the new scale,
// with type to match, and put back inside the work area it is now in.
func TestAnOpenPanelRescalesWithTheScreen(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)
	b.deskScale = 1
	m := testMenuAt(t, b, pageMain, panelScale(b))
	wasW, wasH := m.w, m.h

	// a screen twice the size: the same panel, twice as big, still whole
	b.deskScale = 2
	m.rescale(b)
	if m.w <= wasW || m.h <= wasH {
		t.Errorf("a panel on a screen twice the size is %dx%d and was %dx%d", m.w, m.h, wasW, wasH)
	}
	if m.scale != 2 {
		t.Errorf("the panel is drawn at %.2fx", m.scale)
	}
	if !m.pending() {
		t.Error("the panel was re-scaled without asking to be drawn again")
	}
	last := len(m.rows) - 1
	if got := m.rowAt(float64(m.w)/2, m.rowTop[last]+m.rowH[last]/2); got != last {
		t.Errorf("the re-scaled panel hits row %d where its last row is drawn", got)
	}
	pix := m.panelPixels()
	if !hasInk(pix, m.w, m.h, int(m.mm.marginX)+4, int(m.mm.padTop)+6,
		m.w-int(m.mm.marginX)-4, m.h-int(m.mm.padEnd)-6, 0.3) {
		t.Error("the re-scaled panel has no ink on it")
	}

	// and a screen that has not changed is not a reason to lay the panel out again
	before := m.w
	m.rescale(b)
	if m.w != before {
		t.Error("the panel was laid out again for a screen that had not changed")
	}
}

// The ways out of the panel are red and the row that leaves the app is a *dark* red plate
// with white type on it, and each colourway is a plate in its own colour with white type on
// it. None of that is visible in a code review and all of it is obvious on the screen, so it
// is checked as pixels.
func TestTheWaysOutAreRedAndAColourwayIsItsOwnColour(t *testing.T) {
	b := testBuddy("blush", "flower", CornerBottomRight)
	white := flower.RGB{255, 255, 255}
	for _, page := range append(append([]int{}, everyPage...), pageShape) {
		m := testMenu(t, b, page)
		pix := m.panelPixels()
		// the cross in the corner: red, always
		x0, y0, x1, y1 := m.closeRect()
		if !hasColour(pix, m.w, m.h, int(x0)-1, int(y0)-1, int(x1)+1, int(y1)+1, danger, 26) {
			t.Errorf("page %d: the cross in the corner is not red", page)
		}
		// and the way out of the page or the app
		out := -1
		for i, r := range m.rows {
			if r.label == "Back" || strings.HasPrefix(r.label, "Quit ") {
				out = i
			}
		}
		if out < 0 {
			t.Fatalf("page %d has no way out on it", page)
		}
		row := m.rows[out]
		top, height := m.rowTop[out], m.rowH[out]
		if strings.HasPrefix(row.label, "Quit ") {
			// the way out of the app is a plate: dark red behind white type, centred
			if row.plate == nil {
				t.Fatalf("%q is not painted as a button", row.label)
			}
			if got := luminance(*row.plate); got > plateLum {
				t.Errorf("%q is painted %.2f bright, and a button for white type has to be under %.2f",
					row.label, got, plateLum)
			}
			if !isRed(*row.plate) {
				t.Errorf("%q is painted %v, which is not a red", row.label, *row.plate)
			}
			// the white type, in the middle of the plate
			if !hasColour(pix, m.w, m.h, int(m.mm.marginX), int(top), int(m.w)-int(m.mm.marginX), int(top+height), white, 12) {
				t.Errorf("%q has no white type on it", row.label)
			}
			left, right := inkExtent(pix, m.w, m.h, int(top), int(top+height))
			if middle := float64(left+right) / 2; left < 0 || math.Abs(middle-float64(m.w)/2) > 6 {
				t.Errorf("%q is centred at x=%.0f in a %d pixel plate", row.label, middle, m.w)
			}
		} else {
			// Back is a row rather than a plate, and it is red type with the one chevron
			if got := row.ink; got == nil || *got != danger {
				t.Errorf("page %d: %q is not one of the red rows", page, row.label)
			}
			if !hasColour(pix, m.w, m.h, int(m.mm.labelPad), int(top), int(m.w)-int(m.mm.marginX), int(top+height), danger, 26) {
				t.Errorf("page %d: %q is not drawn in the red", page, row.label)
			}
			if row.hint != "" {
				t.Errorf("page %d: %q carries the hint %q as well as its chevron", page, row.label, row.hint)
			}
		}
	}

	// and the colour page: every colourway is a plate in that colourway's colour, its own
	// colour true on the swatch beside it, with white type on the plate
	m := testMenu(t, b, pageFlower)
	pix := m.panelPixels()
	ways := 0
	for i, r := range m.rows {
		if r.kind != rowPick || r.swatch == nil || r.plate == nil {
			continue
		}
		ways++
		top, height := m.rowTop[i], m.rowH[i]
		name := int(m.mm.labelPad + m.mm.swatchR*2 + m.dp(9))
		if !hasColour(pix, m.w, m.h, name, int(top), int(m.w)-int(m.mm.marginX), int(top+height), white, 12) {
			t.Errorf("the row for %q has no white name on its plate", r.label)
		}
		cx, cy := int(m.mm.labelPad+m.mm.swatchR), int(top+height/2)
		if !nearColour(pix, cy*m.w+cx, *r.swatch, 30) {
			t.Errorf("the swatch beside %q is not that colourway's own colour", r.label)
		}
		// and the plate is that colour, not the panel's accent or another colourway's
		if got := luminance(*r.plate); got > plateLum {
			t.Errorf("the plate for %q is %.2f bright, and white type needs it under %.2f", r.label, got, plateLum)
		}
		x, y := int(m.w)-int(m.mm.marginX)-int(m.dp(6)), int(top+height/2)
		if !nearColour(pix, y*m.w+x, *r.plate, 20) {
			t.Errorf("the plate for %q is not painted in its own colour", r.label)
		}
	}
	if ways < len(flower.PaletteNames) {
		t.Errorf("the colour page shows %d colourways and there are %d", ways, len(flower.PaletteNames))
	}
}

// isRed says whether a colour is a red, which is what "the way out is painted in a dark red"
// means once it is a number.
func isRed(c flower.RGB) bool { return c[0] > c[1]*1.6 && c[0] > c[2]*1.6 }

// and pressing it: it opens the coffee page, writes no settings and moves nothing — see
// Buddy.coffee.
func TestTheCoffeeButtonOpensItsPageAndChangesNothing(t *testing.T) {
	var opened []string
	was := openURL
	openURL = func(u string) error { opened = append(opened, u); return nil }
	defer func() { openURL = was }()

	b := testBuddy("blush", "flower", CornerBottomRight)
	before := b.cfg
	rows := b.menuPage(pageMain)
	// the coffee row rather than the last row: the last row of the main page is the way
	// out, and pressing that in a test would ask the app to quit
	rows[coffeeRowAt(rows)].act(b, nil)
	if b.cfg != before {
		t.Errorf("the coffee button changed the settings:\n %+v\n %+v", before, b.cfg)
	}
	if b.homeX != 0 || b.homeY != 0 || b.handX != 0 || b.handY != 0 {
		t.Error("the coffee button moved the flower")
	}
	if len(opened) != 1 || opened[0] != coffeeURL {
		t.Errorf("the coffee button opened %v, want exactly %s", opened, coffeeURL)
	}
}
