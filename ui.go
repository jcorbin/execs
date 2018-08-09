package main

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"strings"
	"unicode/utf8"

	"github.com/jcorbin/execs/internal/terminal"
)

type uid uint32

func (id uid) next() uid      { return id + 1 }
func (id uid) String() string { return fmt.Sprintf("uid<%d>", uint32(id)) }

type uiState struct {
	crawling bool
	crawl    image.Point

	tick   int
	focus  uid
	active uid
	maxID  uid
	arg    int // XXX
}

type ui struct {
	// TODO only needs the output / encoder side
	// TODO also need more Cursor access
	*terminal.Terminal
	terminal.Event
	*uiState
	size   image.Point
	lastID uid
}

func startUI(term *terminal.Terminal, state *uiState, ev terminal.Event) (*ui, error) {
	size, err := term.Size()
	if err != nil {
		return nil, err
	}
	it := &ui{
		Terminal: term,
		Event:    ev,
		uiState:  state,
		size:     size,
	}

	if it.isKey(terminal.KeyTab) {
		it.focus = (it.focus + 1) % (it.maxID + 1)
	}

	return it, nil
}

func (it *ui) isKey(key terminal.Key) bool {
	return it.Event.Type == terminal.KeyEvent && it.Event.Key == key
}

func (it *ui) nextID() uid {
	it.lastID = it.lastID.next()
	it.maxID = it.lastID
	return it.lastID
}

func (it *ui) activated(id uid) (focused, activated, changed bool) {
	focused, activated = it.focus == id, it.active == id
	if activated {
		if it.isKey(terminal.KeyEsc) {
			it.active = 0 // TODO pop a stack
			changed = true
			activated = false
		}
	} else if focused {
		if it.isKey(terminal.KeyEnter) {
			it.active = id
			changed = true
			activated = true
		}
	}
	return focused, activated, changed
}

func (it *ui) header(label string, args ...interface{}) {
	if len(args) > 0 {
		label = fmt.Sprintf(label, args...)
	}
	max := it.size.X - 2
	it.WriteByte('|')
	if label == "" {
		it.WriteString(strings.Repeat("-", max))
	} else {
		w := utf8.RuneCountInString(label)
		if w < max {
			it.WriteString(label)
			it.WriteString(strings.Repeat("-", max-w))
		} else {
			it.WriteString(label[:max])
		}
	}
	it.WriteByte('|')
}

func (it *ui) textbox(label string, buf *bytes.Buffer) {
	id := it.nextID()

	if focused, activated, changed := it.activated(id); !activated {
		if focused {
			it.WriteString(fmt.Sprintf("[_%s_]", label))
		} else {
			it.WriteString(fmt.Sprintf("[ %s ]", label))
		}
		return
	} else if changed {
		it.arg = 2
	}

	if it.arg < it.size.Y {
		it.arg++
	}

	b := logBuf.Bytes()
	sc := bufio.NewScanner(bytes.NewReader(b))

	totalLines := bytes.Count(b, []byte("\n"))
	numLines := totalLines
	if maxLines := it.arg - 2; numLines > maxLines {
		numLines = maxLines
	}

	if numLines < totalLines {
		it.header(" %s (%v of %v) ]", label, numLines, totalLines)
		for i := numLines; i < totalLines; i++ {
			sc.Scan()
		}
	} else {
		it.header(" %s ]", label)
	}

	for sc.Scan() {
		it.WriteString("\r\n| ")
		b := sc.Bytes()
		if w, max := utf8.RuneCount(b), it.size.X-4; w > max {
			drop := 3 + w - max
			for i := 0; i < drop; i++ {
				_, n := utf8.DecodeLastRune(b)
				b = b[:len(b)-n]
			}
			it.Write(b)
			it.WriteString("... |")
		} else {
			it.Write(b)
			it.WriteString(strings.Repeat(" ", max-w))
			it.WriteString(" |")
		}
	}
	it.header("")
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

/*
func doTextEdit(
	ui *ui,
	box image.Rectangle,
	s []byte,
) (_ []byte, changed, submitted bool) {
	id := ui.nextID()

	// TODO focus system
	// if ui.active == 0 {
	// 	if ke, _, ok := ui.keyPressed(); ok {
	// 		if ke == termbox.KeyEnter {
	// 			ui.active = id
	// 		}
	// 	}
	// }
	// if id != ui.active {
	// 	if id == ui.focus {
	// 	}
	// }

	if id != ui.active {
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

	// if ke, ch, ok := ui.keyPressed(); ok {
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
*/
