package view

import (
	"unicode/utf8"

	"github.com/jcorbin/execs/internal/point"
)

// HUD provides an opinionated view system with a Header, Footer, and Logs on
// top of a base grid (e.g world map).
type HUD struct {
	World  Grid
	Header []string
	Footer []string
	Logs   []string
}

// Render the context into the given terminal grid.
func (hud HUD) Render(termGrid Grid) {
	const logPlacement = AlignTop

	// NOTE: intentionally not a layout item so that the UI elemenst overlay
	// the world grid.
	termGrid.Copy(hud.World)

	lay := Layout{Grid: termGrid}

	for _, part := range hud.Header {
		align, n := readLayoutOpts(part)
		lay.Place(RenderString(part[n:]), align|AlignTop)
	}

	for _, part := range hud.Footer {
		align, n := readLayoutOpts(part)
		lay.Place(RenderString(part[n:]), align|AlignBottom)
	}

	if len(hud.Logs) > 0 {
		// TODO: scrolling
		lay.Place(renderLogs{
			ss:  hud.Logs,
			min: point.Point{X: 10, Y: 5},
		}, logPlacement)
	}
}

// Log adds a line to the internal log buffer. As much tail of the log buffer
// is displayed after the header as possible; at least 5 lines.
func (hud *HUD) Log(mess string, args ...interface{}) {
	hud.Logs = append(hud.Logs, fmt.Sprintf(mess, args...))
}

// ClearLog clears the internal log buffer.
func (hud *HUD) ClearLog() { hud.Logs = hud.Logs[:0] }

// SetHeader copies the given lines into the internal header buffer, replacing
// any prior.
func (hud *HUD) SetHeader(lines ...string) {
	if cap(hud.Header) < len(lines) {
		hud.Header = make([]string, len(lines))
	} else {
		hud.Header = hud.Header[:len(lines)]
	}
	copy(hud.Header, lines)
}

// SetFooter copies the given lines into the internal footer buffer, replacing
// any prior.
func (hud *HUD) SetFooter(lines ...string) {
	if cap(hud.Footer) < len(lines) {
		hud.Footer = make([]string, len(lines))
	} else {
		hud.Footer = hud.Footer[:len(lines)]
	}
	copy(hud.Footer, lines)
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
