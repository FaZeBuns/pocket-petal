// Package buddy is the flower on your desktop: one small always-on-top window
// with a per-pixel-alpha picture in it, a real mouse drag, and a tray icon.
//
// Nothing here is a widget toolkit. The window is a bitmap the engine paints
// every frame with its own rasteriser, the drag is the cursor read straight from
// Windows, and the feel is the carry physics — so the flower you drag is the
// same maths the tests cover, not a browser pretending to be a window.
package buddy

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FaZeBuns/pocket-petal/internal/flower"
	appicon "github.com/FaZeBuns/pocket-petal/internal/icon"
	"github.com/FaZeBuns/pocket-petal/internal/target"
	"github.com/FaZeBuns/pocket-petal/internal/win"
)

// What the app calls itself, in one place.
//
// Everywhere a person can see the name — the tray tip, the panel's title, the
// about box, the folder the settings live in, what -version prints — reads it from
// here, so that the app is named once and not in fifteen strings. Slug is the same
// name in the form a file system likes, which is why it is not derived from Name:
// a display name is allowed to be two words with a space in it.
//
// Version is a variable because the release build stamps it (see
// tools/build-release.sh, -X main.version=…), and main hands it down before
// anything is drawn; the value here is what a build from a bare checkout says.
var (
	Name    = "Pocket Petal"
	Slug    = "pocket-petal"
	Version = "0.1.1"
)

// Config is how the flower window is set up, and everything the tray menu can
// change. The json tags are the settings that persist between runs; the fields
// marked `-` are the ones a menu would never offer (traces, dumps, the frame
// rate), so they are left out of the saved file entirely.
type Config struct {
	Width   int     `json:"width"`  // the flower's box on screen, in pixels
	Height  int     `json:"height"` // the window around it is derived from these
	Fit     float64 `json:"fit"`    // how much of that box the flower fills
	Seed    int     `json:"seed"`
	Palette string  `json:"palette"`
	FPS     int     `json:"fps"`
	Detail  string  `json:"detail"` // "full" or "lite"
	Stats   bool    `json:"-"`      // print frames per second and the cost of a frame
	Workers int     `json:"-"`      // rasteriser tiles; 0 or 1 is single-threaded

	// The flower itself. These are the dials that change the *plant* rather than
	// how it is used: the stem, the bloom, the wind it stands in, and which turn
	// of the colour wheel its colourway is. The shape dials are multipliers of the
	// flower as it ships, because "a stem and a third as long" is a decision a
	// person can make and a raw parameter is not.
	Shape FlowerSettings `json:"shape"`

	Note     string `json:"note"`     // which note a delivery carries (empty = surprise me)
	CopyOnly bool   `json:"copyOnly"` // never type: put the note on the clipboard instead

	// what a drop is allowed to do
	Interrupt bool `json:"interrupt"` // press Escape on a chat that is still answering
	AutoSend  bool `json:"autoSend"`  // press Enter after pasting the note
	Caret     bool `json:"caret"`     // click where the flower landed, to put the caret there

	// how it looks while you are carrying it
	Ring     bool   `json:"ring"`     // the warm ring around the window under the flower
	Sunshine bool   `json:"sunshine"` // the sun passing over it now and then
	Burst    bool   `json:"burst"`    // the bloom coming apart on a delivery
	Sound    bool   `json:"sound"`    // a chime when it goes off
	Corner   string `json:"corner"`   // where it stands when nothing is happening

	// Where it goes back to, when it is not the corner. A spot of its own is a
	// place somebody pointed at — the base of the stem, in screen pixels — and it
	// wins over the corner whenever it is set: Home is the switch, and HomeX/HomeY
	// are only ever read through it.
	Home  bool `json:"home"`
	HomeX int  `json:"homeX"`
	HomeY int  `json:"homeY"`

	State  string `json:"-"` // write where the flower is to this file, for scripts to watch
	Dry    bool   `json:"-"` // work out the delivery and print it, but touch nothing
	Trace  bool   `json:"-"` // print what the drag found, for testing the drop path
	Opaque bool   `json:"-"` // draw the frame as a solid block, to check the window really shows
	Dump   string `json:"-"` // write the first rendered frame here as a PNG, alpha and all

	// LogFile appends every trace and delivery line to a file, regardless of how
	// the app was started. -trace only has somewhere to print when a console was
	// there to borrow (see win.AttachConsole in main) — launched the normal way,
	// by double-clicking it or from the taskbar, a windowed build owns no console
	// at all, so nothing it prints is seen anywhere. This is how the very delivery
	// that only goes wrong *that* way gets looked at: a cold start is exactly the
	// one case -trace from a terminal cannot reproduce.
	LogFile string `json:"-"`
}

// FlowerSettings is the flower's own shape, as the settings panel offers it.
//
// Everything here multiplies or replaces a dial of the rig — see rigOptions — so
// a value of 1 is the flower as it ships. The ranges are the ones the panel's
// sliders offer: wide on purpose, because a tall thin wildflower and a squat fat
// one are both flowers and somebody will want both, but never so wide that the
// drawing breaks (petals that cannot be told apart, a stem thinner than its own
// edge).
type FlowerSettings struct {
	Hue       float64 `json:"hue"`       // degrees round the colour wheel, -180..180
	Stem      float64 `json:"stem"`      // stem length, 0.6..1.6 of the ship's
	StemWidth float64 `json:"stemWidth"` // how thick the stalk is, 0.6..1.6
	Wind      float64 `json:"wind"`      // how much it sways, 0.2..2
	Leaves    float64 `json:"leaves"`    // leaf size, 0.5..1.6
	Petals    int     `json:"petals"`    // petals in the outer ring, 8..30
	Bloom     float64 `json:"bloom"`     // how big the bloom is, 0.6..1.5
	Spread    float64 `json:"spread"`    // how wide the petals open, 0.5..1.8
	Flutter   float64 `json:"flutter"`   // how much the petals flutter, 0..2.5
	Limp      float64 `json:"limp"`      // how floppy the neck is, 0.4..1.6
}

// DefaultShape is the flower as it ships: every dial at its natural value.
func DefaultShape() FlowerSettings {
	return FlowerSettings{
		Hue:       0,
		Stem:      1,
		StemWidth: 1,
		Wind:      1,
		Leaves:    1,
		Petals:    18,
		Bloom:     1,
		Spread:    1,
		Flutter:   1,
		Limp:      1,
	}
}

// DefaultConfig is a flower you can actually pick up: 190x250 pixels, sitting in
// the bottom-right corner of the work area.
func DefaultConfig() Config {
	return Config{
		Width: 190, Height: 250, Fit: 0.94,
		Seed: 11, Palette: "blush", FPS: 60, Detail: "full",
		Shape:     DefaultShape(),
		Note:      "flower",
		Interrupt: true, AutoSend: true, Caret: true,
		Ring: true, Sunshine: true, Burst: true, Sound: true,
		Corner: CornerBottomRight,
	}
}

// The corners the flower can stand in.
const (
	CornerBottomRight = "bottom-right"
	CornerBottomLeft  = "bottom-left"
	CornerTopRight    = "top-right"
	CornerTopLeft     = "top-left"
)

// CornerNames is every corner, in the order the menu shows them.
var CornerNames = []string{CornerBottomRight, CornerBottomLeft, CornerTopRight, CornerTopLeft}

// KnownCorner reports whether a name is one of the corners.
func KnownCorner(name string) bool {
	for _, c := range CornerNames {
		if c == name {
			return true
		}
	}
	return false
}

// CornerLabel is a corner as a person reads it.
func CornerLabel(name string) string {
	switch name {
	case CornerBottomLeft:
		return "Bottom left"
	case CornerTopRight:
		return "Top right"
	case CornerTopLeft:
		return "Top left"
	default:
		return "Bottom right"
	}
}

// Buddy is the running flower.
type Buddy struct {
	cfg  Config
	win  *win.Window
	tray *win.Tray
	icon win.HICON
	rig  *flower.Rig

	// logFile is where trace and delivery lines go in addition to stderr, when
	// cfg.LogFile named one. nil whenever it did not, which is the ordinary run.
	logFile *os.File

	carry   *flower.Carry
	tracker *flower.Tracker

	frame flower.Framing // the design box mapped into the window, and the window's size
	box   flower.Bounds  // the same box, kept for the rasteriser's sake
	posX  int
	posY  int

	// the size the window was actually built at, so that a frame that has not
	// changed the answer does not rebuild it (see rebuildWindow)
	winFrameW int
	winFrameH int

	// the display the flower is standing on: its work area and its scaling, which
	// together decide how big the flower is drawn and where it stands. Both are
	// read again whenever the screen changes (see refitDisplay), and the pair is
	// what tells a real change from a message about nothing
	deskArea  win.WorkArea
	deskScale float64

	// The exact envelope, measured off the frame loop: see requestFrame.
	framing framingBox

	// The envelope the flower as it ships needs, measured once when the app starts,
	// and the resting outline that goes with it. A shape being previewed is framed
	// by scaling these (see reframeFast) instead of measuring a new envelope, which
	// costs a third of a second the panel's sliders do not have.
	shipEnv  flower.Bounds
	shipRest flower.Bounds

	scale float64

	// the sun: one pass of light over the flower every so often, and another
	// whenever the flower is picked up or put down
	shineFrom time.Time
	shineTill time.Time
	nextShine time.Time

	dragging bool
	grabS    float64 // which part of the stalk was taken hold of, 0 at the base
	grabHead bool    // or the bloom itself

	// handX, handY is the screen point the flower is *being held by*: the cursor
	// while you drag it, the virtual hand while it walks home, and the place it
	// stands on when nobody is touching it. The window is then placed so that the
	// part of the flower the hand has — a piece of stalk, the top of the stem, or
	// the base it grows from — lands exactly there. That is the whole trick of
	// the drag: what you took hold of stays under your cursor, and everything
	// else bends and swings around it.
	handX float64
	handY float64

	// basePixX, basePixY is where the base of the stem sits inside the window,
	// in pixels. The rig's origin is the base, so this does not move with the
	// pose: it is a constant of the framing, worked out when the flower is sized.
	basePixX float64
	basePixY float64

	// the burst: the petals that came off the flower on the last delivery, and
	// the bloom growing back afterwards
	burst        *flower.Burst
	burstPending bool
	burstPower   float64
	burstVX      float64
	burstVY      float64
	burstBox     flower.Bounds // the whole flight of the burst, measured when it is thrown
	petals       *win.Window   // the surface the pieces fly in, made on demand

	// the settings panel, while one is open: at most one at a time, and nil the
	// rest of the time, which is the whole lifetime of a menu
	menu *menuState

	// delivery: what is under the flower, the glow around it, and the walk home
	pid       uint32
	high      *highlighter
	spot      target.Target
	spotAt    time.Time // when the window under the cursor was last looked up
	spotAtX   int
	spotAtY   int
	rng       *rand.Rand
	walking   bool
	homeX     int
	homeY     int
	stateAt   time.Time    // when the state file was last written (see maybeState)
	delivered atomic.Int64 // unix nanoseconds of the last successful delivery
	// firstDeliveryDone is false until the first drop is attempted. A process
	// right after it starts is the one time Windows can be measurably slower to
	// actually act on a focus change, a click or a paste — most often because it
	// is busy scanning the exe that was just launched — so that one drop alone
	// is given longer waits between steps (see deliver). Nothing else about it
	// is different: the same steps in the same order, just not rushed.
	firstDeliveryDone bool

	blitReported bool
	dumped       bool
	raisedAt     time.Time
	seenAt       time.Time

	clock     float64
	lastFrame float64

	frames    int
	workNanos int64
	statsAt   time.Time

	quit bool
}

// rigOptions is the flower as the settings have it grown: the shape dials are
// multipliers on the flower as it ships, applied where the rig is created so that
// they reach every petal, leaf and seed rather than being fudged at draw time.
func rigOptions(cfg Config) flower.RigOptions {
	s := cfg.Shape
	return flower.RigOptions{
		Seed:    &cfg.Seed,
		Palette: cfg.Palette,
		Tweak: func(p *flower.Params) {
			p.Hue += s.Hue
			p.Height *= s.Stem
			p.StemWidth *= s.StemWidth
			p.Wind *= s.Wind
			p.LeafLength *= s.Leaves
			p.LeafWidth *= s.Leaves
			p.PetalCount = max(5, s.Petals)
			p.PetalLength *= s.Bloom
			p.PetalWidth *= s.Bloom
			p.HeadRadius *= s.Bloom
			p.CenterRadius *= s.Bloom
			p.PetalSpread *= s.Spread
			p.PetalWobble *= s.Flutter
			// how floppy the flower is: the neck's angle per unit of movement, and
			// how much of that the bloom itself takes. Both together are what a
			// person means by "a limp one"
			p.NeckGain *= s.Limp
			p.HeadSwing *= s.Limp
		},
	}
}

// newRig grows the flower the settings describe.
func (b *Buddy) newRig() { b.rig = flower.CreateRig(rigOptions(b.cfg)) }

// New builds the flower and its window.
// framingJob is an envelope measured for one flower: the rig it was measured for,
// so that a flower re-grown while it was being measured does not have somebody
// else's window made for it.
type framingJob struct {
	rig   *flower.Rig
	frame flower.Framing
}

// framingBox is the one place a measured framing is handed over: the measuring
// happens on its own goroutine, and the frame loop takes the newest one when it is
// ready. It holds one job rather than a queue, because the newest measurement is
// the only one worth applying.
type framingBox struct {
	mu  sync.Mutex
	job *framingJob
}

func (f *framingBox) put(j framingJob) {
	f.mu.Lock()
	f.job = &j
	f.mu.Unlock()
}

func (f *framingBox) take() *framingJob {
	f.mu.Lock()
	defer f.mu.Unlock()
	job := f.job
	f.job = nil
	return job
}

func New(cfg Config, onQuit func()) (*Buddy, error) {
	b := &Buddy{
		cfg:     cfg,
		rig:     flower.CreateRig(rigOptions(cfg)),
		carry:   flower.CreateCarry(flower.PhysicsOptions{}),
		tracker: flower.NewTracker(0.32),
		scale:   1,
		pid:     win.OurPid(),
		// the display as it is right now: the flower is framed for it before it
		// has a window to ask about, and refit whenever it changes. The scaling is
		// the screen's own size against the reference (see deskScaleFor), not the
		// DPI setting on its own: the flower and its panel are the same share of
		// the desk at every resolution a person can set
		deskArea:  win.PrimaryWorkArea(),
		deskScale: deskScaleFor(win.PrimaryScreen()),
		// seeded from the clock as well as the flower, so the notes a delivery
		// types are genuinely different run to run rather than the same list every
		// time the app starts
		rng: rand.New(rand.NewSource(int64(cfg.Seed)*7919 + 13 + time.Now().UnixNano())),
	}

	// opened as early as the config names one, so that a cold start — the one
	// case nothing else can see into (see Config.LogFile) — is logged from its
	// very first line, not from whenever something later happens to go wrong
	if cfg.LogFile != "" {
		if f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			b.logFile = f
			fmt.Fprintf(f, "buddy: --- started %s ---\n", time.Now().Format(time.RFC3339))
		} else {
			fmt.Fprintf(os.Stderr, "buddy: could not open %s: %v\n", cfg.LogFile, err)
		}
	}

	// The reference envelope: the flower as it ships, with this seed and this
	// colourway, measured once here. Everything the sliders do is framed against
	// it, and it is the one measurement worth paying for up front.
	shipRig := flower.CreateRig(flower.RigOptions{Seed: &cfg.Seed, Palette: cfg.Palette})
	b.shipEnv = flower.Envelope(shipRig)
	b.shipRest = flower.OutlineBox(shipRig, flower.Motion{})

	// The window is sized around the flower, not the flower around the window:
	// FrameFor measures every pose the carry can reach and hands back a window
	// big enough for all of them at the size the config asks the flower to be.
	// A window is a rectangle of air, so the flower never has to be shrunk to
	// stop a fast drag slicing the top off it.
	b.reframe()

	window, err := win.NewLayeredWindow(b.frame.Width, b.frame.Height, b.onMessage)
	if err != nil {
		return nil, err
	}
	b.win = window
	b.winFrameW, b.winFrameH = b.frame.Width, b.frame.Height

	// stand in the corner the settings ask for, which is also where the flower
	// walks back to after a delivery. The hand is the point the flower stands on:
	// nobody is holding it, so that is where its base goes.
	b.placeHome()
	b.handX, b.handY = float64(b.homeX), float64(b.homeY)
	b.posX, b.posY = b.follow(flower.PoseOf(b.rig, b.clock, b.carry.Motion()))

	// the glow that shows which window the flower will land on
	if high, err := newHighlighter(240, 240); err == nil {
		b.high = high
	}

	// the tray icon is the flower, drawn by the same engine
	icon, err := b.renderIcon(32)
	if err != nil {
		return nil, err
	}
	b.icon = icon
	tray, err := win.AddTray(window.Hwnd, icon, trayTip)
	if err != nil {
		return nil, err
	}
	b.tray = tray
	return b, nil
}

// reframe sizes the window around the flower's outline: the menu can change the
// seed or the palette, and both change how much room the flower needs.
//
// The size the settings ask for is in 96-dpi pixels, so the display's scaling is
// applied *here*, at the one place design units become screen pixels. That is what
// makes the flower the same size to a person on a 4K monitor at 150% as on a 1080p
// one at 100% — and why a change of scaling is a re-frame (see refitDisplay).
func (b *Buddy) reframe() {
	w, h := b.wantedSize()
	b.fitFrame(flower.FrameFor(b.rig, w, h, b.cfg.Fit))
}

// wantedSize is the flower's size in screen pixels: the size the settings ask for,
// which is in 96-dpi pixels, times the display's scaling.
func (b *Buddy) wantedSize() (int, int) {
	scale := b.deskScale
	if scale <= 0 {
		scale = 1
	}
	return int(math.Round(float64(b.cfg.Width) * scale)), int(math.Round(float64(b.cfg.Height) * scale))
}

// reframeFast frames the flower the settings now describe *without measuring its
// envelope*: it scales the envelope the flower as it ships needed, by how much the
// resting flower has grown. That is one pose instead of a hundred and twenty,
// which is the difference between a slider that follows the pointer and one that
// makes you wait a third of a second per pixel. When the pointer stops, changed()
// measures the real thing.
func (b *Buddy) reframeFast() {
	rest := flower.OutlineBox(b.rig, flower.Motion{})
	sx, sy := 1.0, 1.0
	if b.shipRest.Width() > 1e-6 {
		sx = rest.Width() / b.shipRest.Width()
	}
	if b.shipRest.Height() > 1e-6 {
		sy = rest.Height() / b.shipRest.Height()
	}
	// scaled about the base of the stem, which is the one point of the flower
	// that is in the same place whatever shape it grows into
	env := flower.Bounds{
		MinX: b.shipEnv.MinX * sx, MaxX: b.shipEnv.MaxX * sx,
		MinY: b.shipEnv.MinY * sy, MaxY: b.shipEnv.MaxY * sy,
	}
	w, h := b.wantedSize()
	b.fitFrame(flower.FrameForEnvelope(b.rig, env, w, h, b.cfg.Fit))
}

// fitFrame takes a framing as the flower's own.
func (b *Buddy) fitFrame(frame flower.Framing) {
	b.frame = frame
	b.box = b.frame.Bounds
	tr := flower.MakeTransform(b.frame.Bounds, b.frame.Width, b.frame.Height, 0)
	// the base of the stem: the rig's origin, so its pixel is a constant
	b.basePixX, b.basePixY = tr.ToPixel(0, 0)
}

// pixelScale is design units to screen pixels at the size the flower stands.
func (b *Buddy) pixelScale() float64 {
	rest := flower.OutlineBox(b.rig, flower.Motion{})
	if rest.Height() <= 1e-6 {
		return 1
	}
	return b.frame.Flower / rest.Height()
}

// follow is where the window goes for the flower as it is posed right now.
//
// The rule is one line long: the point of the flower the hand is holding lands
// on the hand. Everything else about the drag falls out of that.
//
//   - What is being held is a *place on the flower*, not a rectangle: the piece
//     of stalk the cursor took hold of, or the top of the stem when the bloom is
//     what was grabbed. The pose moves that place — the stem leans, the whip
//     bends it, the neck swings the bloom — so the window moves to keep it under
//     the cursor, and the flower is left to do exactly what a flower does when
//     you pick it up by one spot.
//   - The carry's lag is deliberately *not* added to the window's position. A
//     window that trails the cursor is a picture sliding around; a flower that
//     leans, trails its bloom behind and throws it forward when you stop is the
//     physics, and that lives in the pose.
//   - Standing still, the base of the stem is what is held, which is what a
//     flower stands on: the window is still and the flower sways inside it.
func (b *Buddy) follow(pose flower.Pose) (int, int) {
	tr := flower.MakeTransform(b.box, b.frame.Width, b.frame.Height, 0)
	ax, ay := b.anchor(pose)
	px, py := tr.ToPixel(ax, ay)
	return int(math.Round(b.handX - px)), int(math.Round(b.handY - py))
}

// anchor is the point on the flower the hand is holding, in design space.
func (b *Buddy) anchor(pose flower.Pose) (float64, float64) {
	switch {
	case b.dragging && b.grabHead:
		// held by the bloom: the hand has the top of the stalk, and the bloom
		// swings on its limp neck above it
		tip := pose.Head.Tip
		return tip.X, tip.Y
	case b.dragging:
		p := flower.StemPoint(pose, b.grabS)
		return p.X, p.Y
	default:
		// standing: the base of the stem is on the ground
		return 0, 0
	}
}

// frameShapes is the flower as it is drawn on a desktop.
//
// The reference renderer keeps a halo behind the head all the time. As a layer
// on a desktop it does not work: the halo is wider than the window, so what
// reaches the screen is the middle of a soft ellipse cut off square by the
// window's edge — the grey blob, not a glow. The sun visits the flower instead
// (see flower.ApplyShine), so the halo is simply not drawn here.
func frameShapes(list []flower.Shape) []flower.Shape {
	kept := list[:0]
	for _, shape := range list {
		if shape.Part == "glow" {
			continue
		}
		kept = append(kept, shape)
	}
	return kept
}

// Run is the app: message pump, animation, rendering, until the user quits.
func (b *Buddy) Run() {
	b.win.Show()
	b.lastFrame = b.now()
	b.statsAt = time.Now()
	// where the window ended up, and how much of the picture is opaque: the two
	// facts that answer "why can I not see the flower?"
	if b.cfg.Trace {
		r := b.win.Rect()
		b.trace("window %d,%d %dx%d (client %dx%d), work area %v",
			r.Left, r.Top, r.Width(), r.Height(), b.win.W, b.win.H, win.PrimaryWorkArea())
		// what the display made of the settings: the size the flower is drawn at is
		// the one number that changes when a screen's scaling does, and this is where
		// a person checks that a 4K monitor at 150% still gets the same flower
		b.trace("display scaling %.2fx: the flower stands %.0f pixels tall at %d,%d",
			b.deskScale, b.frame.Flower, b.homeX, b.homeY)
	}

	frame := time.Second / time.Duration(b.cfg.FPS)
	next := time.Now()
	for !b.quit {
		if !win.Pump() {
			break
		}
		now := time.Now()
		if now.Before(next) {
			sleepUntil(next)
			continue
		}
		// schedule the next frame from *now*, so a slow frame cannot make the loop
		// run flat out trying to catch up on frames nobody is waiting for
		next = now.Add(frame)
		// stay at the top of the always-on-top band: a window that is activated
		// later (a browser in full screen) would otherwise be drawn over us
		if now.Sub(b.raisedAt) > raiseEvery {
			b.raisedAt = now
			b.win.Raise()
		}
		b.tick()
		if b.cfg.Stats {
			b.frames++
			if elapsed := now.Sub(b.statsAt); elapsed >= time.Second {
				fmt.Fprintf(os.Stderr, "buddy: %.1f fps, %.2f ms/frame of work\n",
					float64(b.frames)/elapsed.Seconds(),
					float64(b.workNanos)/float64(max(1, b.frames))/1e6)
				b.frames, b.workNanos, b.statsAt = 0, 0, now
			}
		}
	}
}

// grownSeconds is the rig time at which the flower has finished opening, which
// is the pose worth looking at.
const grownSeconds = 6.0

// trayTip is what the shell shows when the pointer rests on the icon.
var trayTip = Name + " — drag me onto a chat"

// raiseEvery is how often the flower re-asserts that it belongs on top. Often
// enough that it never looks buried, rare enough to cost nothing.
const raiseEvery = 1200 * time.Millisecond

func sleepUntil(t time.Time) {
	d := time.Until(t)
	if d > 0 {
		time.Sleep(d)
	}
}

// Close shuts the window, the petals and the tray icon down.
//
// It writes the settings on the way out as well, so that a flower closed from the
// panel and a flower closed by the system both come back the way they were left.
func (b *Buddy) Close() {
	b.save()
	if b.tray != nil {
		b.tray.Remove()
	}
	if b.petals != nil {
		b.petals.Destroy()
		b.petals = nil
	}
	if b.win != nil {
		b.win.Destroy()
	}
	if b.logFile != nil {
		b.logFile.Close()
	}
}

func (b *Buddy) now() float64 { return float64(win.Ticks()) / 1000 }

// tick is one frame: move what needs moving, pose the flower, paint it.
func (b *Buddy) tick() {
	// a framing measured in the background, waiting to be taken: this is where a
	// third of a second of arithmetic that would have frozen the flower is turned
	// into a window that quietly becomes the right size
	b.takeFrame()

	now := b.now()
	dt := math.Min(0.05, math.Max(0, now-b.lastFrame))
	b.lastFrame = now
	b.clock += dt
	b.maybeState(time.Now())

	switch {
	case b.dragging:
		// the hand is the cursor, exactly: whatever part of the flower was taken
		// hold of is pinned under it by follow()
		cursor := win.CursorPosition()
		b.handX, b.handY = float64(cursor.X), float64(cursor.Y)
		moved := b.tracker.Sample(b.handX, b.handY, now)
		b.carry.Step(dt, flower.Hand{VX: moved.VX, VY: moved.VY, AX: moved.AX, AY: moved.AY, Dragging: true})
		// the ring follows the flower, so you can see where it would land
		b.hover()

	case b.walking:
		// a virtual hand carries the flower home at a walking pace, through the
		// same physics as a real drag, so it sways the whole way back
		dx, dy := b.handX-float64(b.homeX), b.handY-float64(b.homeY)
		dist := math.Hypot(dx, dy)
		if dist < 2 {
			b.walking = false
			b.handX, b.handY = float64(b.homeX), float64(b.homeY)
			b.carry.Step(dt, flower.Hand{Gust: b.breeze()})
			b.carry.Impact(0.22)
		} else {
			const walk = 620.0 // pixels per second
			step := math.Min(dist, walk*dt)
			b.handX -= dx / dist * step
			b.handY -= dy / dist * step
			moved := b.tracker.Sample(b.handX, b.handY, now)
			b.carry.Step(dt, flower.Hand{VX: moved.VX, VY: moved.VY, AX: moved.AX, AY: moved.AY, Dragging: true})
		}

	default:
		// hanging still: gravity, and the weather it hangs in
		b.carry.Step(dt, flower.Hand{Gust: b.breeze()})
		b.weather()
		// a delivery that has settled means it is time to go home
		// ...and only once the petals have come off and grown back: the flower
		// stands still for its own delivery rather than walking out on it
		if at := b.delivered.Load(); at != 0 && time.Now().UnixNano()-at > int64(260*time.Millisecond) &&
			b.burst == nil && !b.burstPending {
			b.delivered.Store(0)
			b.homeward()
		}
	}

	motion := b.carry.Motion()
	started := time.Now()
	pose := flower.PoseOf(b.rig, b.clock, motion)
	b.posX, b.posY = b.follow(pose)
	list := frameShapes(flower.BuildDrawList(b.rig, pose, flower.DrawOptions{
		Detail:  b.cfg.Detail,
		Shadow:  true,
		Palette: b.rig.Palette,
	}))

	// the celebration: a shower of the flower's own petals, thrown on the frame
	// after the drop. The flower itself is untouched — nothing comes off it.
	if b.burstPending {
		b.burstPending = false
		if b.cfg.Burst {
			b.throwPetals(pose)
		}
	}
	if b.burst != nil {
		b.burst.Step(dt)
	}
	// the window's own size, not the size we asked for: on a scaled display they
	// are not the same, and the bitmap has to match the window exactly
	width, height := b.win.W, b.win.H
	rgba := flower.Rasterize(list, flower.RasterOptions{
		Width:      width,
		Height:     height,
		Bounds:     b.box,
		SubSamples: 2,
		Workers:    b.cfg.Workers,
	})
	// the sun, if it is passing right now: added into the picture the flower has
	// already painted, so it can only ever brighten flower
	b.sunlit(rgba, width, height)
	if b.cfg.Opaque {
		// a solid frame: the difference between "the window is not on screen"
		// and "the picture in it is transparent"
		for i := 0; i < len(rgba); i += 4 {
			rgba[i], rgba[i+1], rgba[i+2], rgba[i+3] = 255, 0, 255, 255
		}
	}
	// a seedling is not the flower: wait for the bloom before dumping, or the
	// picture is a sprout and the question "why can I not see the flower?" is
	// answered by the wrong frame
	if b.cfg.Dump != "" && !b.dumped && b.clock >= grownSeconds {
		b.dumped = true
		if err := dumpFrame(b.cfg.Dump, rgba, width, height); err != nil {
			b.trace("dump failed: %v", err)
		} else {
			bounds := flower.ListBounds(list)
			b.trace("dumped the first frame to %s: %d pixels with any alpha", b.cfg.Dump, opaquePixels(rgba))
			b.trace("window %dx%d, box %.0f,%.0f..%.0f,%.0f (%.0fx%.0f), shapes %.0f,%.0f..%.0f,%.0f (%.0fx%.0f)",
				width, height,
				b.box.MinX, b.box.MinY, b.box.MaxX, b.box.MaxY, b.box.Width(), b.box.Height(),
				bounds.MinX, bounds.MinY, bounds.MaxX, bounds.MaxY, bounds.Width(), bounds.Height())
		}
	}
	if err := b.win.BlitAt(b.posX, b.posY, rgba, width, height); err != nil {
		b.trace("%v", err)
	} else if b.cfg.Trace && !b.blitReported {
		b.blitReported = true
		r := b.win.Rect()
		b.trace("blit %dx%d at %d,%d: %d opaque pixels; window now at %d,%d %dx%d",
			width, height, b.posX, b.posY, opaquePixels(rgba), r.Left, r.Top, r.Width(), r.Height())
	}
	// and the petals, in a window of their own: they need far more room than the
	// flower does, and the flower's window is only as big as the poses it can
	// reach
	b.drawPetals()

	// an open settings panel redraws itself when something has changed — a hover,
	// a choice, a slider — and asks for nothing on a frame where nothing has. What
	// changed is a *row*, not the panel: see menuState.draw
	if b.menu != nil && b.menu.pending() && !b.menu.paint(b) {
		b.closeMenu()
	}

	// is the flower actually on screen? ask Windows who owns a pixel of the
	// picture we just drew, which is the difference between "not drawn" and
	// "drawn behind somebody else's window"
	if b.cfg.Trace && time.Since(b.seenAt) > 2*time.Second {
		b.seenAt = time.Now()
		if at, ok := firstOpaque(rgba, width); ok {
			px, py := b.posX+at%width, b.posY+at/width
			top, class := win.TopWindowAt(px, py)
			b.trace("opaque pixel at %d,%d: top window is %s (ours=%v)", px, py, class, top == b.win.Hwnd)
			for i, above := range win.AboveWindow(b.win.Hwnd, 4) {
				b.trace("  %d above the flower: %s", i+1, above)
			}
		}
	}
	b.workNanos += time.Since(started).Nanoseconds()
}

// renderIcon draws the flower into a square icon buffer, framed on the bloom the
// way the exe's own icon is (see internal/icon): at sixteen pixels the thing a
// person recognises is the flower's head, and the two pictures being the same one is
// the point — the icon on the exe, the icon in the tray and the flower on the
// desktop are all the same flower.
func (b *Buddy) renderIcon(size int) (win.HICON, error) {
	rgba := appicon.Bloom(b.rig, size)
	appicon.Outline(rgba, size, max(1, size/22))
	hicon, err := win.IconFromRGBA(rgba, size)
	if err != nil {
		return 0, fmt.Errorf("buddy: tray icon: %w", err)
	}
	return hicon, nil
}

// onMessage is the window's behaviour: press to pick the flower up, release to
// put it down, right-click for the menu.
func (b *Buddy) onMessage(m win.MSG) bool {
	switch m.Message {
	case win.WM_SETCURSOR:
		// The flower's window is a layered one, so this message only arrives where
		// the picture is: an empty pixel is not part of the window as far as the
		// mouse is concerned, and the pointer is never over the flower there. Which
		// means there is nothing to test — over the flower, the pointer says so.
		if win.HitTest(m.LParam) != win.HTCLIENT {
			return false
		}
		win.SetCursor(win.CursorHand)
		return true

	case win.WM_LBUTTONDOWN:
		cursor := win.CursorPosition()
		posX, posY := b.win.Position()
		b.dragging = true
		b.grab(flower.PoseOf(b.rig, b.clock, b.carry.Motion()), int(cursor.X), int(cursor.Y), posX, posY)
		// the hand is where the cursor already is, so nothing moves on the frame
		// the flower is picked up: the grab is defined against that point
		b.handX, b.handY = float64(cursor.X), float64(cursor.Y)
		b.trace("picked up at %d,%d (stem s=%.3f, head=%v)", cursor.X, cursor.Y, b.grabS, b.grabHead)
		b.win.Raise()
		if b.cfg.Trace {
			b.trace("window at %v size %dx%d", b.win.Rect(), b.win.W, b.win.H)
		}
		b.tracker.Reset()
		b.carry.Reset()
		b.carry.Impact(0.35)
		// the hand has it now: a flower that was walking itself home stops walking,
		// because being picked up outranks going home
		b.walking = false
		// picking it up catches the light
		b.sunlight(shineTouch)
		win.SetCapture(b.win.Hwnd)
		return true

	case win.WM_LBUTTONUP:
		if b.dragging {
			b.letGo()
			win.ReleaseCapture()
			// put down: the bloom takes the landing
			b.carry.Impact(0.5)
			b.onDropped()
		}
		return true

	case win.WM_RBUTTONUP, win.WM_TRAYICON:
		// WM_TRAYICON carries the mouse message in the low word of lparam. Both
		// buttons open the menu: at a tray icon there is no "select" to do, so a
		// left click that did nothing would just look broken.
		if m.Message == win.WM_TRAYICON {
			switch uint32(m.LParam) & 0xffff {
			case win.WM_RBUTTONUP, win.WM_LBUTTONUP, win.WM_LBUTTONDBLCLK:
			default:
				return true
			}
		}
		b.showMenu()
		return true

	case win.WM_DISPLAYCHANGE, win.WM_SETTINGCHANGE, win.WM_DPICHANGED:
		// The screen changing under the flower: a different resolution, a different
		// scaling factor, or the taskbar moving to another edge. All three arrive
		// here, and refitDisplay is what decides whether anything a person could see
		// changed — a message about nothing does not move the flower. A real change
		// is answered by the work area and the scaling being read again, and by the
		// flower being sized and placed again for the screen it is now on.
		b.refitDisplay()
		return true

	case win.WM_DESTROY:
		win.Quit()
		return true
	}
	return false
}

// trace prints one line when tracing is on, which is how the drag and drop path
// is checked without a pair of eyes on the screen.
func (b *Buddy) trace(format string, args ...any) {
	if b.cfg.Trace {
		b.log(format, args...)
	}
}

// log writes one line to stderr — which is only seen at all when something
// attached a console (see win.AttachConsole in main) — and, when Config.LogFile
// named one, to that file as well, unconditionally. It is how the one run that
// a terminal cannot reproduce still gets looked at: see Config.LogFile.
func (b *Buddy) log(format string, args ...any) {
	line := fmt.Sprintf("buddy: "+format+"\n", args...)
	fmt.Fprint(os.Stderr, line)
	if b.logFile != nil {
		fmt.Fprint(b.logFile, line)
	}
}

// grab works out which part of the flower the cursor just took hold of and where
// that part is on screen, which is what the whole drag is defined against.
func (b *Buddy) grab(pose flower.Pose, cursorX, cursorY, winX, winY int) {
	tr := flower.MakeTransform(b.box, b.win.W, b.win.H, 0)
	dx, dy := tr.ToDesign(float64(cursorX-winX), float64(cursorY-winY))
	// the nearest piece of stalk, and how far the bloom's middle is: whichever is
	// closer is what the hand has
	s, stemDist := flower.NearestStem(pose, dx, dy)
	headDist := math.Hypot(dx-pose.Head.X, dy-pose.Head.Y)
	b.grabS, b.grabHead = s, headDist < stemDist
}

// onDropped is the point of the app: let go over a window and the note is typed
// into it.
func (b *Buddy) onDropped() {
	b.tracker.Reset()
	// letting it go shakes the light off it
	b.sunlight(shineTouch)
	// the bloom comes apart the moment you let go of it, wherever you let go: the
	// explosion belongs to the drop, and the note is a separate question. The
	// *sound* is the narrower question — it is heard only where a note is going,
	// and it is asked in deliver, once the window underneath is known
	b.startBurst()
	// one last look at what is underneath, taken now rather than at the last
	// throttled glance, so the note goes to the window the flower was let go over
	// and not to the one before it
	cursor := win.CursorPosition()
	b.lookAt(int(cursor.X), int(cursor.Y))
	b.deliver()
}

// refreshIcon swaps the picture in the tray without dropping the entry.
func (b *Buddy) refreshIcon(icon win.HICON) {
	if b.tray != nil {
		b.tray.SetIcon(icon)
	}
}
