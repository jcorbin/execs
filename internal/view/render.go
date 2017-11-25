package view

import (
	"fmt"
	"unicode/utf8"

	"github.com/jcorbin/execs/internal/point"
)

// RenderString constructs a static string Renderable; either the entire string
// is rendered, or not; no truncation is supported.
func RenderString(mess string, args ...interface{}) Renderable {
	return renderStringT(fmt.Sprintf(mess, args...))
}

type renderStringT string

func (s renderStringT) RenderSize() (wanted, needed point.Point) {
	needed.X = utf8.RuneCountInString(string(s))
	needed.Y = 1
	return needed, needed
}

func (s renderStringT) Render(g Grid, a Align) {
	i := 0
	for _, r := range s {
		g.Data[i].Ch = r
		i++
	}
}
