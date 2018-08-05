package imtui

import (
	"image"
	"time"

	"github.com/jcorbin/execs/internal/terminal"
)

// TODO reconcile much of this with terminal itself:
// - is this run loop layer universal enough?
// - at least the flush delay logic? that would allow us to further deprecate
//   direct usage of Flush, having a better story around it

const eventQueueDepth = 128

// Drawable is a client of a Terminal to be ran by Run().
type Drawable interface {
	Draw(*Core) error
}

type Core struct {
	*terminal.Terminal
	terminal.Event

	framePending bool
	flushTimer   *time.Timer
	events       chan terminal.Event
	errs         chan error
}

// Custom imtui events.
const (
	EventRedraw terminal.EventType = terminal.FirstUserEvent + iota
	FirstClientEvent
)

// TODO timer/animation support

func Run(client Drawable, opts ...terminal.Option) (rerr error) {
	term, err := terminal.Open(opts...)
	if term != nil {
		defer func() {
			if err := term.Close(); rerr == nil {
				rerr = err
			}
		}()
	}
	if err != nil {
		return err
	}

	c := Core{
		Terminal: term,
		events:   make(chan terminal.Event, eventQueueDepth),
		errs:     make(chan error, 1),
	}
	c.Terminal.Notify(c.events, c.errs)

	return c.Run(client)
}

func (c *Core) Size() image.Point {
	size, err := c.Terminal.Size()
	if err != nil {
		select {
		case c.errs <- err:
		default:
		}
	}
	return size
}

func (c *Core) Redraw() {
	c.events <- terminal.Event{Type: EventRedraw}
}

func (c *Core) Run(client Drawable) (err error) {
	c.Redraw()
	for {
		select {
		case c.Event = <-c.events:

		case err := <-c.errs:
			return err

		case <-c.flushTimer.C:
			c.framePending = false
			if err := c.Terminal.Flush(); err != nil {
				return err
			}
			continue
		}

		if !c.framePending {
			const frameDelay = time.Second / 60
			c.framePending = true
			if c.flushTimer == nil {
				c.flushTimer = time.NewTimer(frameDelay)
			} else {
				c.flushTimer.Reset(frameDelay)
			}
		}

		c.Terminal.Discard()
		if err := client.Draw(c); err != nil {
			return err
		}
	}
}

// DrawFunc is a convenient way to implement Drawable to call Run().
type DrawFunc func(c *Core) error

// Draw calls the aliased function.
func (f DrawFunc) Draw(c *Core) error { return f(c) }
