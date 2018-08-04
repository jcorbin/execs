package main

import (
	"errors"
	"image"
	"log"

	termbox "github.com/nsf/termbox-go"

	"github.com/jcorbin/execs/internal/ecs"
)

var (
	aiColors    = []termbox.Attribute{124, 160, 196, 202, 208, 214}
	soulColors  = []termbox.Attribute{19, 20, 21, 27, 33, 39}
	itemColors  = []termbox.Attribute{22, 23, 29, 35, 41, 47}
	wallColors  = []termbox.Attribute{233, 234, 235, 236, 237, 238, 239}
	floorColors = []termbox.Attribute{232, 233, 234}
)

var (
	errQuitGame = errors.New("quit game")
	errGameOver = errors.New("game over")
)

type game struct {
	world world
	name  []byte
}

type world struct {
	ecs.Scope

	char []rune
	attr []termbox.Attribute
	pos  []image.Point
}

/*

// Draws the EditBox in the given location, 'h' is not used at the moment
func (eb *EditBox) Draw(x, y, w, h int) {
	eb.AdjustVOffset(w)

	const coldef = termbox.ColorDefault
	fill(x, y, w, h, termbox.Cell{Ch: ' '})

	t := eb.text
	lx := 0
	tabstop := 0
	for {
		rx := lx - eb.line_voffset
		if len(t) == 0 {
			break
		}

		if lx == tabstop {
			tabstop += tabstop_length
		}

		if rx >= w {
			termbox.SetCell(x+w-1, y, '→',
				coldef, coldef)
			break
		}

		r, size := utf8.DecodeRune(t)
		if r == '\t' {
			for ; lx < tabstop; lx++ {
				rx = lx - eb.line_voffset
				if rx >= w {
					goto next
				}

				if rx >= 0 {
					termbox.SetCell(x+rx, y, ' ', coldef, coldef)
				}
			}
		} else {
			if rx >= 0 {
				termbox.SetCell(x+rx, y, r, coldef, coldef)
			}
			lx += runewidth.RuneWidth(r)
		}
	next:
		t = t[size:]
	}

	if eb.line_voffset != 0 {
		termbox.SetCell(x, y, '←', coldef, coldef)
	}
}

// Adjusts line visual offset to a proper value depending on width
func (eb *EditBox) AdjustVOffset(width int) {
	ht := preferred_horizontal_threshold
	max_h_threshold := (width - 1) / 2
	if ht > max_h_threshold {
		ht = max_h_threshold
	}

	threshold := width - 1
	if eb.line_voffset != 0 {
		threshold = width - ht
	}
	if eb.cursor_voffset-eb.line_voffset >= threshold {
		eb.line_voffset = eb.cursor_voffset + (ht - width + 1)
	}

	if eb.line_voffset != 0 && eb.cursor_voffset-eb.line_voffset < ht {
		eb.line_voffset = eb.cursor_voffset - ht
		if eb.line_voffset < 0 {
			eb.line_voffset = 0
		}
	}
}

func (eb *EditBox) MoveCursorTo(boffset int) {
	eb.cursor_boffset = boffset
	eb.cursor_voffset, eb.cursor_coffset = voffset_coffset(eb.text, boffset)
}

func (eb *EditBox) RuneUnderCursor() (rune, int) {
	return utf8.DecodeRune(eb.text[eb.cursor_boffset:])
}

func (eb *EditBox) RuneBeforeCursor() (rune, int) {
	return utf8.DecodeLastRune(eb.text[:eb.cursor_boffset])
}

func (eb *EditBox) MoveCursorOneRuneBackward() {
	if eb.cursor_boffset == 0 {
		return
	}
	_, size := eb.RuneBeforeCursor()
	eb.MoveCursorTo(eb.cursor_boffset - size)
}

func (eb *EditBox) MoveCursorOneRuneForward() {
	if eb.cursor_boffset == len(eb.text) {
		return
	}
	_, size := eb.RuneUnderCursor()
	eb.MoveCursorTo(eb.cursor_boffset + size)
}

func (eb *EditBox) MoveCursorToBeginningOfTheLine() {
	eb.MoveCursorTo(0)
}

func (eb *EditBox) MoveCursorToEndOfTheLine() {
	eb.MoveCursorTo(len(eb.text))
}

func (eb *EditBox) DeleteRuneBackward() {
	if eb.cursor_boffset == 0 {
		return
	}

	eb.MoveCursorOneRuneBackward()
	_, size := eb.RuneUnderCursor()
	eb.text = byte_slice_remove(eb.text, eb.cursor_boffset, eb.cursor_boffset+size)
}

func (eb *EditBox) DeleteRuneForward() {
	if eb.cursor_boffset == len(eb.text) {
		return
	}
	_, size := eb.RuneUnderCursor()
	eb.text = byte_slice_remove(eb.text, eb.cursor_boffset, eb.cursor_boffset+size)
}

func (eb *EditBox) DeleteTheRestOfTheLine() {
	eb.text = eb.text[:eb.cursor_boffset]
}

func (eb *EditBox) InsertRune(r rune) {
	var buf [utf8.UTFMax]byte
	n := utf8.EncodeRune(buf[:], r)
	eb.text = byte_slice_insert(eb.text, eb.cursor_boffset, buf[:n])
	eb.MoveCursorOneRuneForward()
}

// Please, keep in mind that cursor depends on the value of line_voffset, which
// is being set on Draw() call, so.. call this method after Draw() one.
func (eb *EditBox) CursorX() int {
	return eb.cursor_voffset - eb.line_voffset
}
*/

func (it *imtui) doTextEdit(s []byte) (_ []byte, changed, submitted bool) {
	id := it.nextID()
	if it.active == 0 {
		// TODO better focus system
		if ke, _, ok := it.keyPressed(); ok {
			if ke == termbox.KeyEnter {
				it.active = id
			}
		}
	}
	if id != it.active {
		// TODO draw inactive
		return nil, false, false
	}

	// TODO viva la state
	// type EditBox struct {
	// 	text           []byte
	// 	line_voffset   int
	// 	cursor_boffset int // cursor offset in bytes
	// 	cursor_voffset int // visual cursor offset in termbox cells
	// 	cursor_coffset int // cursor offset in unicode code points
	// }

	// if ke, ch, ok := it.keyPressed(); ok {
	// 	switch ke {
	// 	// case termbox.KeyArrowLeft, termbox.KeyCtrlB:
	// 	// edit_box.MoveCursorOneRuneBackward()
	// 	// case termbox.KeyArrowRight, termbox.KeyCtrlF:
	// 	// edit_box.MoveCursorOneRuneForward()
	// 	// case termbox.KeyBackspace, termbox.KeyBackspace2:
	// 	// edit_box.DeleteRuneBackward()
	// 	// case termbox.KeyDelete, termbox.KeyCtrlD:
	// 	// edit_box.DeleteRuneForward()
	// 	// case termbox.KeyTab:
	// 	// edit_box.InsertRune('\t')
	// 	// case termbox.KeySpace:
	// 	// edit_box.InsertRune(' ')
	// 	// case termbox.KeyCtrlK:
	// 	// edit_box.DeleteTheRestOfTheLine()
	// 	// case termbox.KeyHome, termbox.KeyCtrlA:
	// 	// edit_box.MoveCursorToBeginningOfTheLine()
	// 	// case termbox.KeyEnd, termbox.KeyCtrlE:
	// 	// edit_box.MoveCursorToEndOfTheLine()
	// 	// default:
	// 	// if ch != 0 {
	// 	// edit_box.InsertRune(ch)
	// 	// }
	// 	}
	// }

	// pos := it.size.Div(2)
	// midy := h / 2
	// midx := (w - edit_box_width) / 2

	// termbox.SetCell(midx-1, midy, '│', coldef, coldef)
	// termbox.SetCell(midx+edit_box_width, midy, '│', coldef, coldef)
	// termbox.SetCell(midx-1, midy-1, '┌', coldef, coldef)
	// termbox.SetCell(midx-1, midy+1, '└', coldef, coldef)
	// termbox.SetCell(midx+edit_box_width, midy-1, '┐', coldef, coldef)
	// termbox.SetCell(midx+edit_box_width, midy+1, '┘', coldef, coldef)
	// fill(midx, midy-1, edit_box_width, 1, termbox.Cell{Ch: '─'})
	// fill(midx, midy+1, edit_box_width, 1, termbox.Cell{Ch: '─'})

	// edit_box.Draw(midx, midy, edit_box_width, 1)
	// termbox.SetCursor(midx+edit_box.CursorX(), midy)

	// tbprint(midx+6, midy+3, coldef, coldef, "Press ESC to quit")
	return s, false, false
}

func (g *game) draw(it *imtui) error {
	if ke, _, ok := it.keyPressed(); ok && ke == termbox.KeyEsc {
		return errQuitGame
	}

	if s, changed, submitted := it.doTextEdit(g.name); changed {
		g.name = s
	} else if submitted {
		log.Printf("hello %q", s)
		g.name = s[:0]
	}

	return nil
}

func main() {
	var it imtui
	if err := it.run(drawFunc(func(it *imtui) error {
		for {
			var g game
			if err := it.run(&g); err == errGameOver {
				// TODO prompt for again / present a postmortem
				continue
			} else if err == errQuitGame {
				return nil
			} else if err != nil {
				return err
			}
		}
	})); err != nil {
		panic(err)
	}
}
