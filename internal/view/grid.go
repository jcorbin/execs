package view

import (
	termbox "github.com/nsf/termbox-go"

	"github.com/jcorbin/execs/internal/point"
)

// Grid represents a sized buffer of terminal cells.
type Grid struct {
	Size point.Point
	Data []termbox.Cell
}

// MakeGrid makes a new Grid with the given size.
func MakeGrid(sz point.Point) Grid {
	g := Grid{Size: sz}
	g.Data = make([]termbox.Cell, sz.X*sz.Y)
	return g
}

// Align serves to align text when laying it out in a grid row.
type Align uint8

const (
	// AlignLeft aligns text to the left in a row.
	AlignLeft Align = iota

	// AlignCenter aligns text to the center in a row.
	AlignCenter

	// AlignRight aligns text to the right in a row.
	AlignRight
)

// Get sets a cell in the grid.
func (g Grid) Get(x, y int) termbox.Cell {
	return g.Data[y*g.Size.X+x]
}

// Set sets a cell in the grid.
func (g Grid) Set(x, y int, ch rune, fg, bg termbox.Attribute) {
	g.Data[y*g.Size.X+x] = termbox.Cell{Ch: ch, Fg: fg, Bg: bg}
}

// Merge merges data into a cell in the grid.
func (g Grid) Merge(x, y int, ch rune, fg, bg termbox.Attribute) {
	i := y*g.Size.X + x
	if ch != 0 {
		g.Data[i].Ch = ch
	}
	if fg != 0 {
		g.Data[i].Fg = fg
	}
	if bg != 0 {
		g.Data[i].Bg = bg
	}
}

// Copy copies another grid into this one, centered and clipped as necessary.
func (g Grid) Copy(og Grid) {
	diff := g.Size.Sub(og.Size)
	offset := diff.Div(2)

	ix, nx := 0, og.Size.X
	if diff.X < 0 {
		ix = -offset.X
		nx = ix + g.Size.X
	}

	y := 0
	if diff.Y < 0 {
		y = -offset.Y
		offset.Y = -y
	}

	offset = offset.Max(point.Zero).Min(g.Size)

	for yi := 0; yi < g.Size.Y && y < og.Size.Y; y, yi = y+1, yi+1 {
		x := ix
		i := (yi+offset.Y)*g.Size.X + offset.X
		j := y*og.Size.X + x
		for ; x < nx; x++ {
			c := og.Data[j]
			g.Data[i] = c
			i++
			j++
		}
	}

}

// WriteString writes and aligns a string into a grid row.
func (g Grid) WriteString(y int, align Align, s string) {
	var x int
	switch align {
	case AlignLeft:
		x = 0
	case AlignCenter:
		x = g.Size.X - len(s)/2
		if x < 0 {
			x = 0
		}
	case AlignRight:
		x = g.Size.X - len(s)
		if x < 0 {
			x = 0
		}
	}
	off := y*g.Size.X + x
	for _, r := range s {
		if off >= len(g.Data) {
			break
		}
		g.Data[off].Ch = r
		off++
	}
}
