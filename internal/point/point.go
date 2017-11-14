package point

// Point represents a point in <X,Y> 2-space.
type Point struct{ X, Y int }

// Zero is the origin, the zero value of Point.
var Zero = Point{}

// Less returns true if this point's X or Y component is less than the other's.
func (pt Point) Less(other Point) bool {
	return pt.Y < other.Y || pt.X < other.X
}

// Equal returns true if both this point's X and Y components equal another's.
func (pt Point) Equal(other Point) bool {
	return pt.X == other.X && pt.Y == other.Y
}

// Clamp returns a copy of this point with its X and Y components guaranteed to
// be no smaller than min's nor larger than max's.
func (pt Point) Clamp(min, max Point) Point {
	if pt.X < min.X {
		pt.X = min.X
	}
	if pt.Y < min.Y {
		pt.Y = min.Y
	}
	if pt.X > max.X {
		pt.X = max.X
	}
	if pt.Y > max.Y {
		pt.Y = max.Y
	}
	return pt
}

// Add adds another point's values to a copy of this point, returning the copy.
func (pt Point) Add(other Point) Point {
	pt.X += other.X
	pt.Y += other.Y
	return pt
}

// Sub subtracts another point's values from a copy of this point, returning
// the copy.
func (pt Point) Sub(other Point) Point {
	pt.X -= other.X
	pt.Y -= other.Y
	return pt
}

// Div divides a copy of this point's values by a constant, returning the copy.
func (pt Point) Div(n int) Point {
	pt.X /= n
	pt.Y /= n
	return pt
}

// Mul multiplies a copy of this point's values by a constant, returning the
// copy.
func (pt Point) Mul(n int) Point {
	pt.X *= n
	pt.Y *= n
	return pt
}

// Abs returns a copy of this point with its values non-negative.
func (pt Point) Abs() Point {
	if pt.X < 0 {
		pt.X = -pt.X
	}
	if pt.Y < 0 {
		pt.Y = -pt.Y
	}
	return pt
}

// Sign returns a copy of this point reduced to the values -1, 0, or 1 depending
// on the sign of the original values.
func (pt Point) Sign() Point {
	pt.X = sign(pt.X)
	pt.Y = sign(pt.Y)
	return pt
}

func sign(i int) int {
	if i < 0 {
		return -1
	}
	if i > 0 {
		return 1
	}
	return 0
}
