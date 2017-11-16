package view

import (
	"errors"
	"fmt"

	"github.com/jcorbin/execs/internal/point"
	termbox "github.com/nsf/termbox-go"
)

// Stop may be returned by a client method to mean "we're done, break run loop".
var Stop = errors.New("client stop")

// Client is the interface exposed to the user of View; its various methods are
// called in a loop that provides terminal orchestration.
type Client interface {
	Render(*Context) error
	HandleKey(*View, KeyEvent) error
	Close() error
}

// Context encapsulates all state to be rendered by the view.
type Context struct {
	// TODO: avoiding "layer"s or "window"s for now, but really...
	Header []string
	Footer []string
	Logs   []string
	Avail  point.Point
	Grid   Grid
}

// KeyEvent represents a terminal key event.
type KeyEvent struct {
	Mod termbox.Modifier
	Key termbox.Key
	Ch  rune
}

// JustKeepRunning starts a view, and then running newly minted Runables
// provided by the given factory until an error occurs, or the user quits.
// Useful for implementing main.main.
func JustKeepRunning(factory func(v *View) (Client, error)) error {
	var v View
	return v.runWith(func() error {
		for v.polling {
			if client, err := factory(&v); err != nil {
				return err
			} else if err := v.runClient(client); err != nil && err != Stop {
				return err
			}
		}
		return nil
	})
}

// Run a Client under this view, returning any error from the run (may be
// caused by the client, or view).
func (v *View) Run(client Client) error {
	return v.runWith(func() error {
		err := v.runClient(client)
		if err == Stop {
			return nil
		}
		return err
	})
}

// Log grabs the rendering lock, and adds a log to the context's log buffer;
// see Context.Log.
func (v *View) Log(mess string, args ...interface{}) {
	v.ctxLock.Lock()
	defer v.ctxLock.Unlock()
	v.ctx.Log(mess, args...)
}

// SetHeader copies the given lines into the internal header buffer, replacing
// any prior.
func (ctx *Context) SetHeader(lines ...string) {
	if cap(ctx.Header) < len(lines) {
		ctx.Header = make([]string, len(lines))
	} else {
		ctx.Header = ctx.Header[:len(lines)]
	}
	copy(ctx.Header, lines)
}

// SetFooter copies the given lines into the internal footer buffer, replacing
// any prior.
func (ctx *Context) SetFooter(lines ...string) {
	if cap(ctx.Footer) < len(lines) {
		ctx.Footer = make([]string, len(lines))
	} else {
		ctx.Footer = ctx.Footer[:len(lines)]
	}
	copy(ctx.Footer, lines)
}

// Log adds a line to the internal log buffer. As much tail of the log buffer
// is displayed after the header as possible; at least 5 lines.
func (ctx *Context) Log(mess string, args ...interface{}) {
	ctx.Logs = append(ctx.Logs, fmt.Sprintf(mess, args...))
}
