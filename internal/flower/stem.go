package flower

import "math"

// The stem as a material: a point on it that stays the same point.
//
// The rig integrates the spine into 34 joints, so a place on the stalk can be
// named the way it actually is — a fraction of the way up it — instead of by
// where it happens to be drawn this frame. That is what a drag needs: the bit of
// stem under the cursor is the bit that has to stay under the cursor, however the
// lean and the whip carry the rest of the flower around.

// StemPoint is the point on the stalk at s, where 0 is the base and 1 the neck,
// interpolated between the joints the pose was integrated into.
func StemPoint(pose Pose, s float64) Point {
	n := len(pose.Joints)
	if n == 0 {
		return Point{}
	}
	if n == 1 {
		return pose.Joints[0]
	}
	f := clamp(num(s), 0, 1) * float64(n-1)
	i := int(f)
	if i >= n-1 {
		return pose.Joints[n-1]
	}
	t := f - float64(i)
	a, b := pose.Joints[i], pose.Joints[i+1]
	return Point{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t}
}

// NearestStem is the stem parameter closest to a point, and how far away that is.
// It walks the joints rather than solving anything: 34 segments is nothing, and
// the answer is exact to the segment.
func NearestStem(pose Pose, x, y float64) (s float64, dist float64) {
	n := len(pose.Joints)
	if n == 0 {
		return 0, math.Inf(1)
	}
	best := math.Inf(1)
	for i := 0; i+1 < n; i++ {
		a, b := pose.Joints[i], pose.Joints[i+1]
		dx, dy := b.X-a.X, b.Y-a.Y
		len2 := dx*dx + dy*dy
		t := 0.0
		if len2 > 1e-9 {
			t = clamp(((x-a.X)*dx+(y-a.Y)*dy)/len2, 0, 1)
		}
		px, py := a.X+dx*t, a.Y+dy*t
		d := math.Hypot(x-px, y-py)
		if d < best {
			best = d
			s = (float64(i) + t) / float64(n-1)
		}
	}
	if n == 1 {
		best = math.Hypot(x-pose.Joints[0].X, y-pose.Joints[0].Y)
	}
	return s, best
}
