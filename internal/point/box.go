package point

// Box represents a bounding box defined by a top-left and bottom-right point.
type Box struct {
	TopLeft     Point
	BottomRight Point
}

// Size returns the width and height of the box as a point.
func (b Box) Size() Point {
	pt := b.BottomRight.Sub(b.TopLeft).Abs()
	pt.X++
	pt.Y++
	return pt
}

// ExpandTo expands a copy of the box to include the given point, returning the
// copy.
func (b Box) ExpandTo(pt Point) Box {
	if pt.X < b.TopLeft.X {
		b.TopLeft.X = pt.X
	}
	if pt.Y < b.TopLeft.Y {
		b.TopLeft.Y = pt.Y
	}
	if pt.X > b.BottomRight.X {
		b.BottomRight.X = pt.X
	}
	if pt.Y > b.BottomRight.Y {
		b.BottomRight.Y = pt.Y
	}
	return b
}

// ExpandBy symmetrically expands a copy of the box by a given x/y
// displacement, returning the copy.
func (b Box) ExpandBy(d Point) Box {
	b.TopLeft = b.TopLeft.Sub(d)
	b.BottomRight = b.BottomRight.Add(d)
	return b
}

// Contains returns true if a given point is inside the box.
func (b Box) Contains(pt Point) bool {
	return !(pt.Less(b.TopLeft) || b.BottomRight.Less(pt))
}
