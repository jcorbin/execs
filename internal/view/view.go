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
	polling bool
	pollErr error
	keys    chan KeyEvent
	redraw  chan struct{}
	done    chan struct{}

	ctxLock sync.Mutex
	size    point.Point
	ctx     Context
}

func (v *View) runWith(f func() error) (rerr error) {
	if v.polling {
		panic("invalid view state")
	}

	v.polling = true

	if err := termbox.Init(); err != nil {
		return err
	}

	priorInputMode := termbox.SetInputMode(termbox.InputCurrent)
	defer termbox.SetInputMode(priorInputMode)
	termbox.SetInputMode(termbox.InputEsc)

	priorOutputMode := termbox.SetOutputMode(termbox.OutputCurrent)
	defer termbox.SetOutputMode(priorOutputMode)
	termbox.SetOutputMode(termbox.Output256)

	v.pollErr = nil
	v.redraw = make(chan struct{}, 1)
	v.keys = make(chan KeyEvent, keyBufferSize)
	v.done = make(chan struct{})
	v.size = termboxSize()

	go v.pollEvents()
	defer func() {
		go termbox.Interrupt()
		v.polling = false
		if v.done != nil {
			<-v.done
		}
		if rerr == nil {
			rerr = v.pollErr
		}
	}()

	return f()
}

func (v *View) runClient(client Client) (rerr error) {
	defer func() {
		if cerr := client.Close(); rerr == nil || rerr == ErrStop {
			rerr = cerr
		}
	}()

	raise(v.redraw)

	// TODO: observability / introspection / other Nice To Haves?

	for {
		select {

		case <-v.done:
			return client.Close()

		case <-v.redraw:

		case k := <-v.keys:
			if err := client.HandleKey(v, k); err != nil {
				return err
			}

		}

		if err := v.render(client); err != nil {
			return err
		}
	}
}

func (v *View) render(client Client) error {
	v.ctxLock.Lock()
	defer v.ctxLock.Unlock()

	if !point.Zero.Less(v.size) {
		v.size = termboxSize()
	}
	if !point.Zero.Less(v.size) {
		return fmt.Errorf("bogus terminal size %v", v.size)
	}

	v.ctx.Avail = v.size.Sub(point.Point{Y: len(v.ctx.Footer) + len(v.ctx.Header)})
	if err := client.Render(&v.ctx); err != nil {
		return err
	}

	buf := make([]termbox.Cell, v.size.X*v.size.Y)
	v.ctx.render(Grid{Size: v.size, Data: buf})
	if err := termbox.Clear(termbox.ColorDefault, termbox.ColorDefault); err != nil {
		return fmt.Errorf("termbox.Clear failed: %v", err)
	}
	copy(termbox.CellBuffer(), buf)
	if err := termbox.Flush(); err != nil {
		return fmt.Errorf("termbox.Flush failed: %v", err)
	}
	return nil
}

func (ctx *Context) render(termGrid Grid) {
	header := ctx.Header
	space := termGrid.Size.Sub(ctx.Grid.Size)
	space.Y -= len(header)
	space.Y -= len(ctx.Footer)

	if len(ctx.Logs) > 0 {
		nLogs := len(ctx.Logs)
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

func (v *View) pollEvents() {
	defer termbox.Close()
	defer close(v.done)

	v.pollErr = func() error {
		for v.polling {
			switch ev := termbox.PollEvent(); ev.Type {
			case termbox.EventKey:
				switch ev.Key {
				case termbox.KeyCtrlC:
					return nil
				case termbox.KeyCtrlL:
					raise(v.redraw)
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
				// TODO: would rather defer this into the client running code
				// to coalesce resize events; that seems to be the intent of
				// termbox.Clear, but we've already built our grid by the time
				// we clear that... basically a simpler/lower layer than
				// termbox would be really nice...
				v.ctxLock.Lock()
				v.size.X = ev.Width
				v.size.Y = ev.Height
				v.ctxLock.Unlock()
				raise(v.redraw)

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

func raise(ch chan<- struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
