package imtui

// Drawable is a client of a Core to be ran by Run().
type Drawable interface {
	Draw(c *Core) error
}

// Run a drawable client under a core in an event polling loop. Calls core
// Clear and Flush for convenience around the client. Ensures that the core is
// opened before entering the event loop, and that the core is closed before
// returning, if it was closed beforehand.
//
// TODO if we eliminate Flush from the core, then clear also can probably go...
// except for the size update in Core, so we probably still want some sort of a
// Core.Init() that gets called here...
func Run(c *Core, client Drawable) (rerr error) {
	if opened, err := c.Open(); err != nil {
		if opened {
			_ = c.Close()
		}
		return err
	} else if opened {
		defer func() {
			if err := c.Close(); rerr == nil {
				rerr = err
			}
		}()
	}

	c.ClearEvent()
	for {
		err := c.Clear()
		if err == nil {
			err = client.Draw(c)
			if ferr := c.Flush(); err == nil {
				err = ferr
			}
		}
		if err == nil {
			err = c.PollEvent()
		}
		return err
	}
}

// DrawFunc is a convenient way to implement Drawable to call Run().
type DrawFunc func(c *Core) error

// Draw calls the aliased function.
func (f DrawFunc) Draw(c *Core) error { return f(c) }
