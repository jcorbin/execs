package view

import (
	"fmt"
	"unicode/utf8"

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

// WriteString writes a string into the grid at the given position, returning
// how many cells were affected.
func (g Grid) WriteString(x, y int, mess string, args ...interface{}) int {
	if len(args) > 0 {
		mess = fmt.Sprintf(mess, args...)
	}
	i := y*g.Size.X + x
	j := i
	for ; x < g.Size.X; x, j = x+1, j+1 {
		r, n := utf8.DecodeRuneInString(mess)
		mess = mess[n:]
		g.Data[i].Ch = r
	}
	return j - i
}

// WriteStringRTL is like WriteString except it gose Right-To-Left (in both the
// string and the grid).
func (g Grid) WriteStringRTL(x, y int, mess string, args ...interface{}) int {
	if len(args) > 0 {
		mess = fmt.Sprintf(mess, args...)
	}
	i := y*g.Size.X + x
	j := i
	for ; x >= 0; x, j = x-1, j-1 {
		r, n := utf8.DecodeLastRuneInString(mess)
		mess = mess[:len(mess)-n]
		g.Data[i].Ch = r
	}
	return j - i
}
