package view_test

import (
	"testing"

	"github.com/jcorbin/execs/internal/point"
	. "github.com/jcorbin/execs/internal/view"
	"github.com/stretchr/testify/assert"
)

func TestLayout(t *testing.T) {
	type sa struct {
		s string
		a Align
	}
	for _, tc := range []struct {
		name     string
		init     func() Grid
		sas      []sa
		expected []string
	}{
		{
			name: "basic",
			init: func() Grid {
				return MakeGrid(point.Point{X: 25, Y: 10})
			},
			sas: []sa{
				{"left1", AlignTop | AlignLeft},
				{"left2", AlignTop | AlignLeft},
				{"left3", AlignTop | AlignLeft | AlignHFlush},
				{"right1", AlignTop | AlignRight},
				{"rrright4", AlignTop | AlignRight},
				{"right2", AlignTop | AlignRight},
				{"right3", AlignTop | AlignRight | AlignHFlush},
				{"center1", AlignTop | AlignCenter},
				{"left4", AlignBottom | AlignLeft},
				{"left5", AlignBottom | AlignLeft},
				{"left6", AlignBottom | AlignLeft | AlignHFlush},
				{"right4", AlignBottom | AlignRight},
				{"right5", AlignBottom | AlignRight},
				{"right6", AlignBottom | AlignRight | AlignHFlush},
				{"center2", AlignBottom | AlignCenter},
				{"left7", AlignMiddle | AlignLeft},
				{"left8", AlignMiddle | AlignLeft},
				{"left9", AlignMiddle | AlignLeft | AlignHFlush},
				{"right7", AlignMiddle | AlignRight},
				{"right8", AlignMiddle | AlignRight},
				{"right9", AlignMiddle | AlignRight | AlignHFlush},
				{"center3", AlignMiddle | AlignCenter},
			},
			expected: []string{
				"left1 left2 right2 right1",
				"left3  center1   rrright4",
				"                   right3",
				"                         ",
				"left9   center3    right9",
				"left7 left8 right8 right7",
				"                         ",
				"                         ",
				"left6   center2    right6",
				"left4 left5 right5 right4",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lay := Layout{}
			lay.Grid = tc.init()
			for _, sa := range tc.sas {
				lay.Render(RenderString(sa.s), sa.a)
			}
			assert.Equal(t, tc.expected, grid2lines(lay.Grid))
		})
	}
}

func grid2lines(g Grid) []string {
	lines := make([]string, g.Size.Y)
	i := 0
	for y := 0; y < g.Size.Y; y++ {
		line := make([]rune, g.Size.X)
		for x := 0; x < g.Size.X; x++ {
			if ch := g.Data[i].Ch; ch != 0 {
				line[x] = ch
			} else {
				line[x] = ' '
			}
			i++
		}
		lines[y] = string(line)
	}
	return lines
}
