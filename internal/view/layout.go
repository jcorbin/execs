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
	Render(Grid, Align)
}

func (lay *Layout) init() {
	n := lay.Grid.Size.Y
	if cap(lay.avail) < n {
		lay.lused = make([]int, n)
		lay.rused = make([]int, n)
		lay.cused = make([]int, n)
		lay.avail = make([]int, n)
	} else {
		lay.lused = lay.lused[:n]
		lay.rused = lay.rused[:n]
		lay.cused = lay.cused[:n]
		lay.avail = lay.avail[:n]
	}
	n = lay.Grid.Size.X
	for i := range lay.avail {
		lay.avail[i] = n
	}
}

// LayoutPlacement represents a placement made by a Layout for a Renderable.
type LayoutPlacement struct {
	lay *Layout

	ren    Renderable
	align  Align
	wanted point.Point
	needed point.Point

	ok    bool
	start int
	have  point.Point
}

// Place a Renderable into layout, returning false if the placement can't be
// done.
func (lay *Layout) Place(ren Renderable, align Align) LayoutPlacement {
	if len(lay.avail) != lay.Grid.Size.Y {
		lay.init()
	}
	plc := LayoutPlacement{
		lay: lay,
		ren: ren,
	}
	plc.wanted, plc.needed = ren.RenderSize()
	plc.Try(align)
	return plc
}

// Render places and renders a Renderable if the placement succeeded.
func (lay *Layout) Render(ren Renderable, align Align) LayoutPlacement {
	plc := lay.Place(ren, align)
	plc.Render()
	return plc
}

// Try attempts to (re)resolve the placement with an other alignment.
func (plc *LayoutPlacement) Try(align Align) bool {
	// h-flush should default to left-align, not center
	if align&AlignCenter == 0 && align&AlignHFlush != 0 {
		align |= AlignLeft
	}
	plc.align = align

	switch align & AlignMiddle {
	case AlignTop:
		plc.find(0, 1)

	case AlignBottom:
		plc.find(len(plc.lay.avail)-1, -1)

	default: // NOTE: defaults to AlignMiddle:
		mid := len(plc.lay.avail) / 2
		plc.find(mid, -1)
		if !plc.ok {
			plc.find(mid, 1)
		} else {
			alt := *plc
			alt.find(mid, 1)
			if alt.ok {
				if ud, ld := plc.start-mid, mid-alt.start; ud > ld {
					*plc = alt
				}
			}
		}
	}

	return plc.ok
}

func (plc *LayoutPlacement) find(init, dir int) {
	cused := plc.align&AlignCenter != 0
	lflush := plc.align&AlignHFlush != 0 && plc.align&AlignCenter == AlignLeft
	rflush := plc.align&AlignHFlush != 0 && plc.align&AlignCenter == AlignRight

	plc.ok = false
	plc.start = init
seekStart:
	plc.have = point.Zero
	for plc.start >= 0 && plc.start < len(plc.lay.avail) {
		if plc.lay.avail[plc.start] >= plc.needed.X &&
			!(cused && plc.lay.cused[plc.start] > 0) &&
			!(lflush && plc.lay.lused[plc.start] > 0) &&
			!(rflush && plc.lay.rused[plc.start] > 0) {
			plc.have.X = minInt(plc.wanted.X, plc.lay.avail[plc.start])
			goto seekEnd
		}
		plc.start += dir
	}
	return

seekEnd:
	end := plc.start + dir
	plc.have.Y++
	for end >= 0 && end < len(plc.lay.avail) {
		if plc.have.Y >= plc.wanted.Y {
			break
		}
		if plc.lay.avail[end] < plc.needed.X ||
			(cused && plc.lay.cused[end] > 0) ||
			(lflush && plc.lay.lused[end] > 0) ||
			(rflush && plc.lay.rused[end] > 0) {
			if plc.have.Y >= plc.needed.Y {
				break
			}
			plc.start += dir
			goto seekStart
		}
		if plc.lay.avail[end] < plc.have.X {
			plc.have.X = plc.lay.avail[end]
		}
		plc.have.Y++
		end += dir
	}

	plc.ok = plc.have.Y >= plc.needed.Y
}

// Render renders the placement, if it has been resolved successfully.
func (plc *LayoutPlacement) Render() {
	if !plc.ok {
		return
	}

	plc.align &= ^AlignHFlush
	off, used := 0, []int(nil)

	switch plc.align & AlignCenter {
	case AlignLeft:
		off = maxInt(plc.lay.lused[plc.start : plc.start+plc.have.Y]...)
		if off == 0 {
			plc.align |= AlignHFlush
		}
		used = plc.lay.lused

	case AlignRight:
		off = maxInt(plc.lay.rused[plc.start : plc.start+plc.have.Y]...)
		if off == 0 {
			plc.align |= AlignHFlush
		}
		off = plc.lay.Grid.Size.X - plc.have.X - off
		used = plc.lay.rused

	default: // NOTE: defaults to AlignCenter:
		lused := maxInt(plc.lay.lused[plc.start : plc.start+plc.have.Y]...)
		rused := maxInt(plc.lay.rused[plc.start : plc.start+plc.have.Y]...)
		off = lused + (plc.lay.Grid.Size.X-plc.have.X-lused-rused)/2
		used = plc.lay.cused
	}

	grid := MakeGrid(plc.have)
	plc.ren.Render(grid, plc.align)
	plc.have = plc.lay.copy(grid, plc.start, off, plc.align)

	for y, i := 0, plc.start; y < grid.Size.Y; y, i = y+1, i+1 {
		used[i] += plc.have.X
		plc.lay.avail[i] -= plc.have.X
	}
}

func (lay Layout) copy(g Grid, row, off int, align Align) point.Point {
	off, ix, have := trim(g, off, align)
	for y := 0; y < g.Size.Y; y, row = y+1, row+1 {
		li := row*lay.Grid.Size.X + off
		gi := y*g.Size.X + ix
		for x := ix; x < have.X; x++ {
			lay.Grid.Data[li] = g.Data[gi]
			li++
			gi++
		}
	}
	return have
}

func trim(g Grid, off int, align Align) (_, ix int, have point.Point) {
	have = g.Size
	any := usedColumns(g)

	// trim left
	actual := have.X
	for ; ix < g.Size.X; ix++ {
		if any[ix] {
			break
		}
		actual--
	}

	// trim right
	for x := g.Size.X - 1; x >= ix; x-- {
		if any[x] {
			break
		}
		actual--
	}

	if diff := have.X - actual; diff > 0 {
		switch align & AlignCenter {
		case AlignRight:
			off += diff
		case AlignCenter:
			off += diff / 2
		}
	}

	have.X = actual

	return off, ix, have
}

func usedColumns(g Grid) []bool {
	any := make([]bool, g.Size.X)
	for gi := 0; gi < len(g.Data); {
		for x := 0; x < g.Size.X; x++ {
			if ch := g.Data[gi].Ch; ch != 0 {
				any[x] = true
			}
			if fg := g.Data[gi].Fg; fg != 0 {
				any[x] = true
			}
			if bg := g.Data[gi].Bg; bg != 0 {
				any[x] = true
			}
			gi++
		}
	}
	return any
}

func minInt(ints ...int) int {
	min := ints[0]
	for i := 1; i < len(ints); i++ {
		if n := ints[i]; n < min {
			min = n
		}
	}
	return min
}

func maxInt(ints ...int) int {
	max := ints[0]
	for i := 1; i < len(ints); i++ {
		if n := ints[i]; n > max {
			max = n
		}
	}
	return max
}
