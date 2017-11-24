package view

import (
	"unicode/utf8"

	"github.com/jcorbin/execs/internal/point"
)

// HUD provides an opinionated view system with a Header, Footer, and Logs on
// top of a base grid (e.g world map).
type HUD struct {
	World    Grid
	Logs     []string
	LogAlign Align

	parts []Renderable
	align []Align
}

// Render the context into the given terminal grid.
func (hud HUD) Render(termGrid Grid) {
	if hud.LogAlign == 0 {
		hud.LogAlign = AlignTop
	}

	// NOTE: intentionally not a layout item so that the UI elemenst overlay
	// the world grid.
	termGrid.Copy(hud.World)

	if len(hud.Logs) > 0 {
		// TODO: scrolling
		hud.AddRenderable(renderLogs{
			ss:  hud.Logs,
			min: point.Point{X: 10, Y: 5},
		}, hud.LogAlign)
	}

	lay := Layout{Grid: termGrid}
	for i := range hud.parts {
		lay.Place(hud.parts[i], hud.align[i])
	}
}

// AddHeaderF adds a static string part to the header; the mess string may
// begin with layout markers such as "<^>" to cause left, center, right
// alignment; mess may also start with "." to cause an alignment flush
// (otherwise the layout tries to pack as many parts onto one line as
// possible).
func (hud *HUD) AddHeaderF(mess string, args ...interface{}) {
	align, n := readLayoutOpts(mess)
	hud.AddRenderable(RenderString(mess[n:], args...), align|AlignTop)
}

// AddFooterF adds a static string to the header; the same alignment marks are
// available as to AddHeader.
func (hud *HUD) AddFooterF(mess string, args ...interface{}) {
	align, n := readLayoutOpts(mess)
	hud.AddRenderable(RenderString(mess[n:], args...), align|AlignBottom)
}

// AddRenderable adds an aligned Renderable to the hud.
func (hud *HUD) AddRenderable(ren Renderable, align Align) {
	hud.parts = append(hud.parts, ren)
	hud.align = append(hud.align, align)
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

// TODO: probably also export for re-use
type renderLogs struct {
	ss  []string
	min point.Point
}

func (rss renderLogs) RenderSize() (wanted, needed point.Point) {
	wanted.X = 1
	for i := range rss.ss {
		if n := utf8.RuneCountInString(rss.ss[i]); n > wanted.X {
			wanted.X = n
		}
	}
	wanted.Y = len(rss.ss)
	return wanted, rss.min
}

func (rss renderLogs) Render(g Grid) {
	off := len(rss.ss) - g.Size.Y
	for y := off; y < g.Size.Y; y++ {
		s := rss.ss[y]
		i := y * g.Size.X
		for x := 0; len(s) > 0 && x < g.Size.X; x++ {
			r, n := utf8.DecodeRuneInString(s)
			s = s[n:]
			g.Data[i+x].Ch = r
		}
	}
}
