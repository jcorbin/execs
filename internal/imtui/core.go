package imtui

import (
	"image"
	"os"

	termbox "github.com/nsf/termbox-go" // TODO switch fully off termbox

	"github.com/jcorbin/execs/internal/cops/display"
	"github.com/jcorbin/execs/internal/cops/terminal"
)

// Core handles basic terminal input and output.
type Core struct {
	In, Out *os.File
	Size    image.Point

	open bool

	// output
	term terminal.Terminal
	buf  []byte
	cur  display.Cursor

	// input
	ev termbox.Event
}

// Init ialize the core with the given input/output file pair (which default to
// os.Stdin/os.Stdout respectively). Allocates a 64KiB capacity output buffer
// if none has been yet allocated.
//
// NOTE this is a low level method, most users should use Core.Run() instead.
func (c *Core) Init(in, out *os.File) {
	if in == nil {
		c.In = os.Stdin
	} else {
		c.In = in
	}
	if out == nil {
		c.Out = os.Stdout
	} else {
		c.Out = out
	}
	if c.buf == nil {
		c.buf = make([]byte, 0, 64*1024)
	}
}

// Open the terminal for raw mode output and hide the cursor. Caller SHOULD
// call Close if opened is true to reset the terminal, even in an error path.
//
// NOTE this is a low level method, most users should use Core.Run() instead.
func (c *Core) Open() (opened bool, _ error) {
	if !c.open {
		c.open = true
		c.cur = display.Start
		c.term = terminal.New(uintptr(c.Out.Fd()))
		err := c.term.SetRaw()
		if err == nil {
			c.Swear(display.Cursor.Hide)
			err = c.Flush()
		}
		return true, err
	}
	return false, nil
}

// Close resets the terminal if its been Open()ed, returning any error
// encountered doing so.
//
// NOTE this is a low level method, most users should use Core.Run() instead.
func (c *Core) Close() error {
	if c.open {
		c.open = false
		c.Swear(
			display.Cursor.Home,
			display.Cursor.Clear,
			display.Cursor.Show)
		err := c.Flush()
		if terr := c.term.Restore(); err == nil {
			err = terr
		}
		return err
	}
	return nil
}

// Reset clears any internal polled event, and queries initial terminal size,
// returning any error in so doing.
//
// NOTE called by Core.Run() before entering the event loop, most users should
// not need to call this method directly.
func (c *Core) Reset() (err error) {
	c.ev = termbox.Event{
		Type: termbox.EventNone, // not the zero value, so we have to specify it
	}
	c.Size, err = c.term.Size()
	return err
}

// PollEvent polls an input event from the terminal, storing it for future
// access, and returning any error encountered. If any output bytes have been
// buffered, they are flushed first before potentially blocking.
//
// NOTE called by Core.Run() between each round of the event loop, most users
// should not need to call this method directly.
func (c *Core) PollEvent() error {
	if len(c.buf) > 0 {
		if err := c.flush(); err != nil {
			return err
		}
	}
	c.ev = termbox.PollEvent()
	if c.ev.Type == termbox.EventError {
		return c.ev.Err
	}
	var err error
	c.Size, err = c.term.Size()
	return err
}

func (c *Core) flush() error {
	_, err := c.Out.Write(c.buf)
	c.buf = c.buf[:0]
	return err
}

// Flush any buffered output.
//
// NOTE called if necessary by Core.Run() before polling for the next event,
// most users should not need to call this method directly.
func (c *Core) Flush() error {
	if len(c.buf) > 0 {
		return c.flush()
	}
	return nil
}

// Curse is a single display cursor manipulator; NOTE the type assymetry is due
// to complying with the shape of display.Cursor methods like
// display.Cursor.Show.
type Curse func(display.Cursor, []byte) ([]byte, display.Cursor)

// Swear runs a some curses, accumulating their output into an internal buffer
// for later flushing.
func (c *Core) Swear(curses ...Curse) {
	for i := range curses {
		c.buf, c.cur = curses[i](c.cur, c.buf)
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
