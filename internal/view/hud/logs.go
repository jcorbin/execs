package hud

import (
	"fmt"
	"unicode/utf8"

	"github.com/jcorbin/execs/internal/moremath"
	"github.com/jcorbin/execs/internal/point"
	"github.com/jcorbin/execs/internal/view"
)

// Logs represents a renderable buffer of log messages.
type Logs struct {
	Buffer   []string
	Align    view.Align
	Min, Max int
}

// Init initializes the log buffer and metadata, allocating the given capacity.
func (logs *Logs) Init(logCap int) {
	logs.Align = view.AlignTop | view.AlignCenter
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
		if n := utf8.RuneCountInString(logs.Buffer[i]); n > needed.X {
			needed.X = n
		}
	}
	if needed.Y > wanted.Y {
		needed.Y = wanted.Y
	}
	wanted.X = needed.X
	return wanted, needed
}

// Render renders the log buffer.
func (logs Logs) Render(g view.Grid) {
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
