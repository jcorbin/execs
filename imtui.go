package main

import (
	"image"

	termbox "github.com/nsf/termbox-go"
)

type imid int

type drawable interface {
	draw(it *imtui) error
}

type drawFunc func(it *imtui) error

func (f drawFunc) draw(it *imtui) error { return f(it) }

type imtui struct {
	client drawable

	open bool
	ev   termbox.Event
	size image.Point

	next   imid
	active imid
}

func (it *imtui) Open() error {
	if err := termbox.Init(); err != nil {
		return err
	}
	termbox.SetInputMode(termbox.InputEsc)
	return nil
}

func (it *imtui) Close() error {
	termbox.Close()
	return nil
}

func (it *imtui) nextID() imid {
	id := it.next
	it.next++
	return id
}

func (it *imtui) run(client drawable) (rerr error) {
	if !it.open {
		if err := it.Open(); err != nil {
			return err
		}
		defer func() {
			if err := it.Close(); rerr == nil {
				rerr = err
			}
		}()
	}

	priorClient := it.client
	defer func() { it.client = priorClient }()
	it.client = client
	err := it.redraw()
	for err != nil {
		err = it.pollEvent()
	}
	return err
}

func (it *imtui) redraw() error {
	it.ev = termbox.Event{
		Type: termbox.EventNone, // not the zero value, so we have to specify it
	}
	return it.update()
}

func (it *imtui) pollEvent() error {
	ev := termbox.PollEvent()
	if ev.Type == termbox.EventError {
		return ev.Err
	}
	it.ev = ev
	return it.update()
}

func (it *imtui) update() (rerr error) {
	const coldef = termbox.ColorDefault
	it.next = 1

	if err := termbox.Clear(coldef, coldef); err != nil {
		return err
	}
	defer func() {
		if ferr := termbox.Flush(); rerr == nil {
			rerr = ferr
		}
	}()

	it.size.X, it.size.Y = termbox.Size()
	return it.client.draw(it)
}

func (it *imtui) keyPressed() (termbox.Key, rune, bool) {
	if it.ev.Type != termbox.EventKey {
		return 0, 0, false
	}
	return it.ev.Key, it.ev.Ch, true
}
