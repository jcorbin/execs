package view

import (
	"fmt"
	"sync"
	"time"

	termbox "github.com/nsf/termbox-go"

	"github.com/jcorbin/execs/internal/point"
)

const keyBufferSize = 1100

// View implements a terminal user interaction, based around a grid, header,
// and footer. Additionally a log is provided, whose tail is displayed beneath
// the header.
type View struct {
	renderLock sync.Mutex

	running bool // TODO: atomic
	keys    chan KeyEvent
	done    chan struct{}
	err     error
	size    point.Point

	ctx Context
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

// Keys returns a channel to read for key events.
func (v *View) Keys() <-chan KeyEvent { return v.keys }

// Done returns a signal channel that will send once the view loop is done.
func (v *View) Done() <-chan struct{} { return v.done }

// Err returns any error after done has signaled.
func (v *View) Err() error { return v.err }

// Log grabs the rendering lock, and adds a log to the context's log buffer;
// see Context.Log.
func (v *View) Log(mess string, args ...interface{}) {
	v.renderLock.Lock()
	defer v.renderLock.Unlock()
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

func (ctx *Context) render(termGrid Grid) {
	const minLogLines = 5

	header := ctx.Header
	space := termGrid.Size.Sub(ctx.Grid.Size)
	space.Y -= len(header)
	space.Y -= len(ctx.Footer)

	if len(ctx.Logs) > 0 {
		nLogs := len(ctx.Logs)
		if nLogs > minLogLines {
			nLogs = minLogLines
		}
		if n := nLogs + len(header); n > cap(header) {
			nh := make([]string, len(header), n)
			copy(nh, header)
			ctx.Header = nh
			header = nh[:n]
		} else {
			header = header[:n]
		}
		copy(header[len(ctx.Header):], ctx.Logs[len(ctx.Logs)-nLogs:])
		space.Y -= len(ctx.Logs)
	}

	termGrid.Copy(ctx.Grid)
	for i := 0; i < len(header); i++ {
		termGrid.WriteString(i, AlignLeft, header[i])
	}
	for i, j := len(ctx.Footer)-1, 1; i >= 0; i, j = i-1, j+1 {
		termGrid.WriteString(termGrid.Size.Y-j, AlignRight, ctx.Footer[i])
	}
}

func (v *View) render() error {
	if v.size.X <= 0 || v.size.Y <= 0 {
		v.size = termboxSize()
	}
	if v.size.X <= 0 || v.size.Y <= 0 {
		return fmt.Errorf("bogus terminal size %v", v.size)
	}
	buf := make([]termbox.Cell, v.size.X*v.size.Y)

	v.ctx.render(Grid{
		Size: v.size,
		Data: buf,
	})

	err := termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	if err == nil {
		copy(termbox.CellBuffer(), buf)
		err = termbox.Flush()
	}
	return err
}

// Start takes control of the terminal and starts interaction loop (running on
// a new goroutine).
func (v *View) Start() error {
	v.renderLock.Lock()
	defer v.renderLock.Unlock()

	if v.running {
		return nil
	}
	v.running = true

	if err := termbox.Init(); err != nil {
		return err
	}

	v.err = nil
	v.keys = make(chan KeyEvent, keyBufferSize)
	v.done = make(chan struct{})
	v.size = termboxSize()

	priorInputMode := termbox.SetInputMode(termbox.InputCurrent)
	defer termbox.SetInputMode(priorInputMode)

	termbox.SetInputMode(termbox.InputEsc)

	go v.pollEvents()
	return nil
}

// Stop stops any interaction loop started by Start, and waits for it to
// finish.
func (v *View) Stop() {
	v.renderLock.Lock()
	defer v.renderLock.Unlock()
	v.running = false
	if v.done != nil {
		<-v.done
	}
}

func (v *View) pollEvents() {
	defer termbox.Close()
	defer close(v.done)

	v.err = func() error {
		for v.running {
			switch ev := termbox.PollEvent(); ev.Type {
			case termbox.EventKey:
				switch ev.Key {
				case termbox.KeyCtrlC:
					return nil
				case termbox.KeyCtrlL:
					v.renderLock.Lock()
					err := v.render()
					v.renderLock.Unlock()
					if err != nil {
						return err
					}
					continue
				}
				switch ev.Ch {
				case 'q', 'Q':
					return nil
				}
				select {
				case v.keys <- KeyEvent{ev.Mod, ev.Key, ev.Ch}:
				case <-time.After(10 * time.Millisecond):
				}

			case termbox.EventResize:
				v.renderLock.Lock()
				v.size.X, v.size.Y = ev.Width, ev.Height
				if err := v.render(); err != nil {
					return err
				}
				v.renderLock.Unlock()

			case termbox.EventError:
				return ev.Err
			}
		}
		return nil
	}()
}

func termboxSize() point.Point {
	w, h := termbox.Size()
	return point.Point{X: w, Y: h}
}
