package main

import termbox "github.com/nsf/termbox-go"

var (
	floorTable  colorTable
	floorColors = []termbox.Attribute{232, 233, 234}
)

func init() {
	const (
		zeroOn  = 6
		zeroUp  = 1
		oneDown = 6
		oneOn   = 2
		oneUp   = 1
	)

	n := len(floorColors)
	c0 := floorColors[0]

	for i, c1 := range floorColors {
		if c1 == c0 {
			continue
		}

		floorTable.addTransition(c0, c0, (n-i)*zeroOn)
		floorTable.addTransition(c0, c1, (n-i)*zeroUp)

		floorTable.addTransition(c1, c0, (n-1)*oneDown)
		floorTable.addTransition(c1, c1, (n-1)*oneOn)

		for _, c2 := range floorColors {
			if c2 != c1 && c2 != c0 {
				continue
			}
			floorTable.addTransition(c1, c2, (n-1)*oneUp)
		}
	}
}
