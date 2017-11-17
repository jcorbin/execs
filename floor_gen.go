package main

import termbox "github.com/nsf/termbox-go"

var (
	floorTable  colorTable
	floorColors = []termbox.Attribute{232, 233, 234}
)

func init() {
	var c1, c2, c3 termbox.Attribute = 232, 233, 234

	floorTable.addTransition(c1, c1, 64)
	floorTable.addTransition(c1, c2, 16)
	floorTable.addTransition(c1, c3, 8)

	floorTable.addTransition(c2, c1, 16)
	floorTable.addTransition(c2, c2, 4)
	floorTable.addTransition(c2, c3, 2)

	floorTable.addTransition(c3, c1, 16)
	floorTable.addTransition(c3, c2, 2)
	floorTable.addTransition(c3, c3, 4)
}
