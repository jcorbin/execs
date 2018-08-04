package imtui

import (
	"image"

	termbox "github.com/nsf/termbox-go" // TODO switch to cops or tcell
)

// Core handles basic terminal input and output.
type Core struct {
	Size image.Point

	open bool
	ev   termbox.Event
}

// Open initializes the terminal if it's not already been opened, returning any
// error, and whether opening was attempted. Caller SHOULD call Close if opened
// is true to reset the terminal, even in an error path.
func (c *Core) Open() (opened bool, _ error) {
	if !c.open {
		c.open = true
		err := termbox.Init()
		if err == nil {
			termbox.SetInputMode(termbox.InputEsc)
		}
		return true, err
	}
	return false, nil
}

// Close resets the terminal if its been Open()ed, returning any error
// encountered doing so.
func (c *Core) Close() error {
	if c.open {
		c.open = false
		termbox.Close()
	}
	return nil
}

// PollEvent polls an input event from the terminal, storing it for future
// access, and returning any error encountered.
func (c *Core) PollEvent() error {
	c.ev = termbox.PollEvent()
	if c.ev.Type == termbox.EventError {
		return c.ev.Err
	}
	return nil
}

// ClearEvent clears any internal polled event, useful for performing a redraw
// independent of input event arrival.
func (c *Core) ClearEvent() {
	c.ev = termbox.Event{
		Type: termbox.EventNone, // not the zero value, so we have to specify it
	}
}

// KeyPressed parses any available input polled event, returning any keycode,
// character, and true if the event was a key press, false otherwise.
func (c *Core) KeyPressed() (termbox.Key, rune, bool) {
	if c.ev.Type != termbox.EventKey {
		return 0, 0, false
	}
	return c.ev.Key, c.ev.Ch, true
}

// Clear the screen, preparing to draw a new frame.
func (c *Core) Clear() error {
	const coldef = termbox.ColorDefault
	c.Size.X, c.Size.Y = termbox.Size()
	return termbox.Clear(coldef, coldef)
}

// Flush the termbox buffer, drawing the frame; TODO this isn't very
// "immediate" mode, the entire need for a Flush isn't really "core" to an
// immediate mode tui, it's more a consequence of using a back/front buffer
// approach.
func (c *Core) Flush() error {
	return termbox.Flush()
}
