package view

import (
	"fmt"

	"github.com/jcorbin/execs/internal/point"
)

// Layout places Renderables in a Grid, keeping track of used left/right/center
// space to inform future placements.
type Layout struct {
	Grid

	// invariant: avail[i] == Grid.Size.X - lused[i] - rused[i]
	lused []int
	rused []int
	cused []int
	avail []int
}

// Align specifies alignment to Layout placements.
type Align uint8

const (
	// AlignLeft causes left horizontal alignment in a Layout.
	AlignLeft Align = 1 << iota
	// AlignRight causes right horizontal alignment in a Layout.
	AlignRight

	// AlignTop causes top vertical alignment in a Layout.
	AlignTop
	// AlignBottom causes bottom vertical alignment in a Layout.
	AlignBottom

	// AlignHFlush causes horizontal alignment to accept no offset; so it will
	// always get the "next empty row" in the relevant vertical direction.
	AlignHFlush

	// AlignCenter causes center horizontal alignment in a layout.
	AlignCenter = AlignLeft | AlignRight

	// AlignMiddle causes middle vertical alignment in a layout.
	AlignMiddle = AlignTop | AlignBottom
)

func (a Align) String() string {
	parts := make([]string, 0, 3)

	if a&AlignHFlush != 0 {
		parts = append(parts, "flush")
	}
	switch a & AlignCenter {
	case AlignLeft:
		parts = append(parts, "left")
	case AlignRight:
		parts = append(parts, "right")
	case AlignCenter:
		parts = append(parts, "center")
	default:
		parts = append(parts, "default")
	}

	switch a & AlignMiddle {
	case AlignTop:
		parts = append(parts, "top")
	case AlignBottom:
		parts = append(parts, "bottom")
	case AlignMiddle:
		parts = append(parts, "middle")
	default:
		parts = append(parts, "default")
	}

	return fmt.Sprintf("Align%s", parts)
}

// Renderable is an element for Layout to place and maybe render; if its Render
// method is called, it will get a grid of at least the needed RenderSize.
type Renderable interface {
	RenderSize() (wanted, needed point.Point)
	Render(Grid)
}

// Place a Renderable into layout, returning false if the placement can't be
// done. If the placement is done, then the Renderable is Render()ed into the
// Grid.
func (lay Layout) Place(ren Renderable, align Align) bool {
	if lay.avail == nil {
		lay.lused = make([]int, lay.Grid.Size.Y)
		lay.rused = make([]int, lay.Grid.Size.Y)
		lay.cused = make([]int, lay.Grid.Size.Y)
		lay.avail = make([]int, lay.Grid.Size.Y)
	}

	// h-flush should default to left-align, not center
	if align&AlignCenter == 0 && align&AlignHFlush != 0 {
		align |= AlignLeft
	}

	wanted, needed := ren.RenderSize()

	switch align & AlignMiddle {
	case AlignTop:
		row, have, found := lay.findAvailRow(align, 0, 1, wanted, needed)
		if !found {
			return false
		}
		lay.render(row, have, ren, align)
		return true

	case AlignBottom:
		row, have, found := lay.findAvailRow(align, len(lay.avail)-1, -1, wanted, needed)
		if !found {
			return false
		}
		lay.render(row, have, ren, align)
		return true

	default: // NOTE: defaults to AlignMiddle:
		mid := len(lay.avail) / 2
		ur, uh, uf := lay.findAvailRow(align, mid, -1, wanted, needed)
		lr, lh, lf := lay.findAvailRow(align, mid, 1, wanted, needed)
		if !uf && !lf {
			return false
		}
		if !lf {
			lay.render(ur, uh, ren, align)
		} else if !uf {
			lay.render(lr, lh, ren, align)
		} else if ud, ld := ur-mid, mid-lr; ud <= ld {
			lay.render(ur, uh, ren, align)
		} else {
			lay.render(lr, lh, ren, align)
		}
		return true
	}
}

func (lay Layout) render(row int, have point.Point, ren Renderable, align Align) {
	grid := MakeGrid(have)
	ren.Render(grid)

	switch align & AlignCenter {
	case AlignLeft:
		off := maxInt(lay.lused[row : row+have.Y])
		lay.copy(grid, row, off)
		for i := row; i < have.Y; i++ {
			lay.lused[i] += have.X
			lay.avail[i] -= have.X
		}

	case AlignRight:
		off := lay.Grid.Size.X - have.X - maxInt(lay.rused[row:row+have.Y])
		lay.copy(grid, row, off)
		for i := row; i < have.Y; i++ {
			lay.rused[i] += have.X
			lay.avail[i] -= have.X
		}

	default: // NOTE: defaults to AlignCenter:
		gap := lay.Grid.Size.X - have.X
		gap -= maxInt(lay.lused[row : row+have.Y])
		gap -= maxInt(lay.rused[row : row+have.Y])
		off := gap / 2
		lay.copy(grid, row, off)
		for i := row; i < have.Y; i++ {
			lay.cused[i] += have.X
			lay.avail[i] -= have.X
		}
	}
}

func (lay Layout) copy(grid Grid, row, off int) {
	rem := lay.Grid.Size.X - grid.Size.X
	li, gi := row*lay.Grid.Size.X+off, 0
	for li < len(lay.Grid.Data) && gi < len(grid.Data) {
		for x := 0; x < grid.Size.X; x++ {
			lay.Grid.Data[li] = grid.Data[gi]
			li++
			gi++
		}
		li += rem
	}
}

func (lay Layout) findAvailRow(
	align Align,
	init, dir int,
	wanted, needed point.Point,
) (row int, have point.Point, found bool) {
	cused := align&AlignCenter != 0
	lflush := align&AlignHFlush != 0 && align&AlignCenter == AlignLeft
	rflush := align&AlignHFlush != 0 && align&AlignCenter == AlignRight
	for row, end := init, 0; row >= 0 && end >= 0 && row < len(lay.avail) && end < len(lay.avail); {
		if lay.avail[end] < needed.X || (cused && lay.cused[end] != 0) {
			end += dir
			row, have = end, point.Zero
			continue
		}
		if have.X == 0 {
			if (!lflush || lay.lused[end] == 0) && (!rflush || lay.rused[end] == 0) {
				have.X = lay.avail[end]
			}
		} else if lay.avail[end] < have.X {
			have.X = lay.avail[end]
		}
		if have.Y = end - row + 1; have.Y >= needed.Y {
			return row, have, true
		}
		end += dir
	}
	return 0, point.Zero, false
}

func maxInt(ints []int) int {
	max := 0
	for _, n := range ints {
		if n > max {
			max = n
		}
	}
	return max
}
