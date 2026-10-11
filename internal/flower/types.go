// Package flower is the Pocket Petal flower: posed as pure mathematics, painted
// by whatever is on the other end.
//
//	pose(rig, t, motion)               -> the skeleton: stem joints, head, petals
//	buildDrawList(rig, pose, options)  -> flat polygons with gradient fills
//	rasterize(list, options)           -> pixels (this package draws them itself)
//
// Design space: origin at the base of the stem, +y is UP, units are "design
// units". A fully grown flower is roughly 150 units wide and 210 tall.
//
// The whole rig is deterministic: pose(t) is a closed-form function of t and the
// motion handed in, so the same flower can be scrubbed, unit tested, rasterised
// headlessly, and drawn into a window without a canvas anywhere in sight.
package flower

// Point is a point in design space.
type Point struct{ X, Y float64 }

// Bounds is an axis-aligned box in design space.
type Bounds struct{ MinX, MinY, MaxX, MaxY float64 }

// Width and Height of the box.
func (b Bounds) Width() float64  { return b.MaxX - b.MinX }
func (b Bounds) Height() float64 { return b.MaxY - b.MinY }

// RGB is a colour, 0..255 per channel.
type RGB [3]float64

// A Stop is one stop of a gradient. Alpha defaults to 1 when it is zero-valued
// and unspecified, which is what lets a glow fade out instead of ending in a
// plate edge.
type Stop struct {
	At    float64
	Color RGB
	Alpha float64
}

// Fill is the entire fill vocabulary: solid, linear gradient, radial gradient.
// Keeping it this small is what lets one draw list feed the on-screen renderer
// and the tray icon and come out looking like the same artwork.
type Fill struct {
	Type   FillType
	Color  RGB
	From   Point
	To     Point
	Center Point
	R0, R1 float64
	Stops  []Stop
}

// FillType says which fields of a Fill matter.
type FillType string

const (
	Solid  FillType = "solid"
	Linear FillType = "linear"
	Radial FillType = "radial"
)

// A Shape is one closed polygon with a fill: the only thing painters consume.
type Shape struct {
	Part  string
	Poly  []Point
	Fill  Fill
	Alpha float64
}

// Motion is a snapshot of how the flower is being carried. It is handed in from
// outside — by the physics in physics.go, or by a flight path — so that pose()
// stays a pure function of its arguments.
type Motion struct {
	// Lean -1..1: the stalk bends away from where the flower is going.
	Lean float64
	// Swing -1..1: the head's pendulum. The neck bends and the face tips, which
	// is what a limp stalk does when you carry it.
	Swing float64
	// Whip -1..1: angular momentum, the stem flexing like a whip.
	Whip float64
	// Droop 0..1: the head hangs under its own weight.
	Droop float64
	// LagX/LagY: how far the petals trail behind the head (inertia).
	LagX, LagY float64
	// Flutter 0..2: wind in the petals, strongest at the tips.
	Flutter float64
	// Stretch -1..1: squash and stretch on impact.
	Stretch float64
	// Gust 0..1.5: a travelling wave up the stem.
	Gust float64
}

// Head is the posed bloom head: where it hangs and which way it faces.
type Head struct {
	X, Y       float64
	Angle      float64 // the neck's direction at the head
	NeckAngle  float64 // where the neck points, head included
	NeckLength float64 // how far past the last joint the bloom hangs
	Tip        Point   // the last joint of the stem
	Tilt       float64 // the face's own rotation
	Radius     float64
	Swing      float64
}

// Petal is one petal, posed.
type Petal struct {
	Index    string
	Layer    int
	Ring     float64
	Open     float64
	Flared   bool
	Length   float64
	HalfWide float64
	Shade    float64
	Twist    float64
	Bend     float64
	Tip      Point
}

// Leaf is one leaf, posed and attached to a joint of the stem.
type Leaf struct {
	Index    int
	Attach   Point
	Angle    float64
	Length   float64
	HalfWide float64
	Tip      Point
	Side     float64
	Flutter  float64
	Grow     float64
}

// Pose is the whole skeleton at one instant.
type Pose struct {
	T              float64
	Motion         Motion
	Grow           float64
	Bloom          float64
	StemLength     float64
	Joints         []Point
	Tangents       []float64
	Head           Head
	Petals         []Petal
	Leaves         []Leaf
	OpenPetalCount int
	MeanOpen       float64
}

// PetalSpec is one petal as the rig generated it, before it is posed.
type PetalSpec struct {
	Index string
	Layer int
	Ring  float64
	Delay float64
	Len   float64
	Width float64
	Shade float64
	Sway  float64
	Twist float64
	Curl  float64
}

// LeafSpec is one leaf as the rig generated it.
type LeafSpec struct {
	At      float64
	Side    float64
	Phase   float64
	Scale   float64
	Droop   float64
	Flutter float64
}

// SepalSpec is one sepal of the calyx.
type SepalSpec struct {
	Angle  float64
	Length float64
	Width  float64
}

// Dot is one seed in the centre of the bloom (phyllotaxis).
type Dot struct {
	R     float64
	A     float64
	Size  float64
	Shade float64
}
