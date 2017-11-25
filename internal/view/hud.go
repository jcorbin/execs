package view

import (
	"fmt"
	"unicode/utf8"

	"github.com/jcorbin/execs/internal/moremath"
	"github.com/jcorbin/execs/internal/point"
)

// HUD provides an opinionated view system with a Header, Footer, and Logs on
// top of a base grid (e.g world map).
type HUD struct {
	World Grid
	Logs  Logs

	parts []Renderable
	align []Align
}

// Render the context into the given terminal grid.
func (hud HUD) Render(termGrid Grid) {
	// NOTE: intentionally not a layout item so that the UI elemenst overlay
	// the world grid.
	termGrid.Copy(hud.World)

	if len(hud.Logs.Buffer) > 0 {
		// TODO: scrolling
		if hud.Logs.Align == 0 {
			hud.AddRenderable(hud.Logs, AlignTop|AlignCenter)
		} else {
			hud.AddRenderable(hud.Logs, hud.Logs.Align)
		}
	}

	lay := Layout{Grid: termGrid}
	for i := range hud.parts {
		lay.Render(hud.parts[i], hud.align[i])
	}
}

// HeaderF adds a static string part to the header; the mess string may begin
// with layout markers such as "<^>" to cause left, center, right alignment;
// mess may also start with "." to cause an alignment flush (otherwise the
// layout tries to pack as many parts onto one line as possible).
func (hud *HUD) HeaderF(mess string, args ...interface{}) {
	align, n := readLayoutOpts(mess)
	hud.AddRenderable(RenderString(mess[n:], args...), align|AlignTop)
}

// FooterF adds a static string to the header; the same alignment marks are
// available as to AddHeader.
func (hud *HUD) FooterF(mess string, args ...interface{}) {
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

// Logs represents a renderable buffer of log messages.
type Logs struct {
	Buffer   []string
	Align    Align
	Min, Max int
}

// Init initializes the log buffer and metadata, allocating the given capacity.
func (logs *Logs) Init(logCap int) {
	logs.Align = AlignTop | AlignCenter
	logs.Min = 5
	logs.Max = 10
	logs.Buffer = make([]string, 0, logCap)
}

// RenderSize returns the desired and necessary sizes for rendering.
func (logs Logs) RenderSize() (wanted, needed point.Point) {
	needed.X = 1
	needed.Y = moremath.MinInt(len(logs.Buffer), logs.Min)
	wanted.X = 1
	wanted.Y = moremath.MinInt(len(logs.Buffer), logs.Max)
	for i := range logs.Buffer {
		if n := utf8.RuneCountInString(logs.Buffer[i]); n > wanted.X {
			wanted.X = n
		}
	}
	if needed.Y > wanted.Y {
		needed.Y = wanted.Y
	}
	return wanted, needed
}

// Render renders the log buffer.
func (logs Logs) Render(g Grid) {
	off := len(logs.Buffer) - g.Size.Y
	if off < 0 {
		off = 0
	}
	for i, y := off, 0; i < len(logs.Buffer); i, y = i+1, y+1 {
		gi := y * g.Size.X
		for s, x := logs.Buffer[i], 0; len(s) > 0 && x < g.Size.X; x++ {
			r, n := utf8.DecodeRuneInString(s)
			s = s[n:]
			g.Data[gi+x].Ch = r
		}
	}
}

// Log formats and appends a log message to the buffer, discarding the oldest
// message if full.
func (logs *Logs) Log(mess string, args ...interface{}) {
	mess = fmt.Sprintf(mess, args...)
	if len(logs.Buffer) < cap(logs.Buffer) {
		logs.Buffer = append(logs.Buffer, mess)
	} else {
		copy(logs.Buffer, logs.Buffer[1:])
		logs.Buffer[len(logs.Buffer)-1] = mess
	}
}
