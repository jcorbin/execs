package view

import (
	"fmt"
	"unicode/utf8"

	"github.com/jcorbin/execs/internal/point"
)

// RenderString constructs a static string Renderable; either the entire string
// is rendered, or not; no truncation is supported.
func RenderString(mess string, args ...interface{}) Renderable {
	return renderStringT{
		s:   fmt.Sprintf(mess, args...),
		sep: " ",
	}
}

type renderStringT struct {
	s   string
	sep string
}

func (rs renderStringT) RenderSize() (wanted, needed point.Point) {
	needed.X = utf8.RuneCountInString(rs.s) + utf8.RuneCountInString(rs.sep)
	needed.Y = 1
	return needed, needed
}

func (rs renderStringT) Render(g Grid, a Align) {
	ss := []string{rs.s}
	if a&AlignHFlush == 0 {
		switch a & AlignCenter {
		case AlignLeft:
			ss = []string{rs.sep, rs.s}
		case AlignRight:
			ss = []string{rs.s, rs.sep}
		}
	}
	i := 0
	for _, s := range ss {
		for _, r := range s {
			g.Data[i].Ch = r
			i++
		}
	}
}
