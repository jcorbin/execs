package imtui

import "os"

// Drawable is a client of a Core to be ran by Run().
type Drawable interface {
	Draw(c *Core) error
}

// Run the given client under a core initialized with the given input/output
// file pair (which defaults to os.Stdin/os.Stdout respectively).
func Run(in, out *os.File, client Drawable) error {
	c := Core{
		In:  in,
		Out: out,
	}
	if in != nil || out != nil {
		c.Init(in, out)
	}
	return c.Run(client)
}

// Run a drawable client under a core in an event polling loop. If no input or
// output file have been given, they default to os.Stdin / os.Stdout
// respectively. Ensures that the core is opened before entering the event
// loop, and that the core is closed before returning, if it was closed
// beforehand.
func (c *Core) Run(client Drawable) (rerr error) {
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
	err := c.Reset()
	for err == nil {
		if err = client.Draw(c); err == nil {
			err = c.PollEvent()
		}
	}
	return err
}

// DrawFunc is a convenient way to implement Drawable to call Run().
type DrawFunc func(c *Core) error

// Draw calls the aliased function.
func (f DrawFunc) Draw(c *Core) error { return f(c) }
