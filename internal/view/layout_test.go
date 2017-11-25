package view_test

import (
	"testing"

	"github.com/jcorbin/execs/internal/point"
	. "github.com/jcorbin/execs/internal/view"
	"github.com/stretchr/testify/assert"
)

func TestLayout(t *testing.T) {
	lay := Layout{
		Grid: MakeGrid(point.Point{X: 25, Y: 10}),
	}

	lay.Place(RenderString("left1"), AlignTop|AlignLeft)
	lay.Place(RenderString("left2"), AlignTop|AlignLeft)
	lay.Place(RenderString("left3"), AlignTop|AlignLeft|AlignHFlush)

	lay.Place(RenderString("right1"), AlignTop|AlignRight)
	lay.Place(RenderString("rrright4"), AlignTop|AlignRight)
	lay.Place(RenderString("right2"), AlignTop|AlignRight)
	lay.Place(RenderString("right3"), AlignTop|AlignRight|AlignHFlush)

	lay.Place(RenderString("center1"), AlignTop|AlignCenter)

	lay.Place(RenderString("left4"), AlignBottom|AlignLeft)
	lay.Place(RenderString("left5"), AlignBottom|AlignLeft)
	lay.Place(RenderString("left6"), AlignBottom|AlignLeft|AlignHFlush)

	lay.Place(RenderString("right4"), AlignBottom|AlignRight)
	lay.Place(RenderString("right5"), AlignBottom|AlignRight)
	lay.Place(RenderString("right6"), AlignBottom|AlignRight|AlignHFlush)

	lay.Place(RenderString("center2"), AlignBottom|AlignCenter)

	lay.Place(RenderString("left7"), AlignMiddle|AlignLeft)
	lay.Place(RenderString("left8"), AlignMiddle|AlignLeft)
	lay.Place(RenderString("left9"), AlignMiddle|AlignLeft|AlignHFlush)

	lay.Place(RenderString("right7"), AlignMiddle|AlignRight)
	lay.Place(RenderString("right8"), AlignMiddle|AlignRight)
	lay.Place(RenderString("right9"), AlignMiddle|AlignRight|AlignHFlush)

	lay.Place(RenderString("center3"), AlignMiddle|AlignCenter)

	lines := make([]string, lay.Grid.Size.Y)
	i := 0
	for y := 0; y < lay.Grid.Size.Y; y++ {
		line := make([]rune, lay.Grid.Size.X)
		for x := 0; x < lay.Grid.Size.X; x++ {
			if ch := lay.Grid.Data[i].Ch; ch != 0 {
				line[x] = ch
			} else {
				line[x] = ' '
			}
			i++
		}
		lines[y] = string(line)
	}

	assert.Equal(t, []string{
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
	}, lines)
}
