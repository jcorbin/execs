package view

import (
	"fmt"
	"unicode/utf8"

	"github.com/jcorbin/execs/internal/point"
)

// Context contains a convenience Header, Footer, and list of Log-ed messages
// that can be rendered by a Client.
type Context struct {
	Header []string
	Footer []string
	Logs   []string
	Grid   Grid
}

// Render the context into the given terminal grid.
func (ctx Context) Render(termGrid Grid) {
	const logPlacement = AlignTop

	// NOTE: intentionally not a layout item so that the UI elemenst overlay
	// the world grid.
	termGrid.Copy(ctx.Grid)

	lay := Layout{Grid: termGrid}

	for _, part := range ctx.Header {
		align, n := readLayoutOpts(part)
		lay.Place(renderString(part[n:]), align|AlignTop)
	}

	for _, part := range ctx.Footer {
		align, n := readLayoutOpts(part)
		lay.Place(renderString(part[n:]), align|AlignBottom)
	}

	if len(ctx.Logs) > 0 {
		lay.Place(renderStrings{
			off: 0, // TODO: scrolling
			ss:  ctx.Logs,
			min: point.Point{X: 10, Y: 5},
		}, logPlacement)
	}
}

// Log adds a line to the internal log buffer. As much tail of the log buffer
// is displayed after the header as possible; at least 5 lines.
func (ctx *Context) Log(mess string, args ...interface{}) {
	ctx.Logs = append(ctx.Logs, fmt.Sprintf(mess, args...))
}

// ClearLog clears the internal log buffer.
func (ctx *Context) ClearLog() { ctx.Logs = ctx.Logs[:0] }

// SetHeader copies the given lines into the internal header buffer, replacing
// any prior.
func (ctx *Context) SetHeader(lines ...string) {
	if cap(ctx.Header) < len(lines) {
		ctx.Header = make([]string, len(lines))
	} else {
		ctx.Header = ctx.Header[:len(lines)]
	}
	copy(ctx.Header, lines)
}

// SetFooter copies the given lines into the internal footer buffer, replacing
// any prior.
func (ctx *Context) SetFooter(lines ...string) {
	if cap(ctx.Footer) < len(lines) {
		ctx.Footer = make([]string, len(lines))
	} else {
		ctx.Footer = ctx.Footer[:len(lines)]
	}
	copy(ctx.Footer, lines)
}

func readLayoutOpts(s string) (opts Align, n int) {
	for len(s) > 0 {
		switch r, m := utf8.DecodeRuneInString(s[n:]); r {
		case '.':
			opts |= AlignHFlush
			n += m
			continue
		case '<':
			opts |= AlignLeft
			n += m
		case '>':
			opts |= AlignRight
			n += m
		case '^':
			opts |= AlignCenter
			n += m
		}
		break
	}
	return opts, n
}

// TODO: export for re-use
type renderString string

func (s renderString) RenderSize() (wanted, needed point.Point) {
	needed.X = utf8.RuneCountInString(string(s))
	needed.Y = 1
	return needed, needed
}

func (s renderString) Render(g Grid) {
	i := 0
	for _, r := range s {
		g.Data[i].Ch = r
		i++
	}
}

// TODO: probably also export for re-use
type renderStrings struct {
	ss  []string
	off int
	min point.Point
}

func (rss renderStrings) RenderSize() (wanted, needed point.Point) {
	wanted.X = 1
	for i := range rss.ss {
		if n := utf8.RuneCountInString(rss.ss[i]); n > wanted.X {
			wanted.X = n
		}
	}
	wanted.Y = len(rss.ss)
	return wanted, rss.min
}

func (rss renderStrings) Render(g Grid) {
	for y := rss.off; y < g.Size.Y; y++ {
		s := rss.ss[y]
		i := y * g.Size.X
		for x := 0; len(s) > 0 && x < g.Size.X; x++ {
			r, n := utf8.DecodeRuneInString(s)
			s = s[n:]
			g.Data[i+x].Ch = r
		}
	}
}
