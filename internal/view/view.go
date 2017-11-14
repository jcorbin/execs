package view

import (
	"fmt"
	"sync"
	"time"

	"github.com/jcorbin/execs/internal/point"
	termbox "github.com/nsf/termbox-go"
)

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

	// TODO: avoiding "layer"s or "window"s for now, but really...
	header []string
	footer []string
	logs   []string
	grid   Grid
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

// SetHeader copies the given lines into the internal header buffer, replacing
// any prior.
func (v *View) SetHeader(lines ...string) {
	v.renderLock.Lock()
	if cap(v.header) < len(lines) {
		v.header = make([]string, len(lines))
	} else {
		v.header = v.header[:len(lines)]
	}
	copy(v.header, lines)
	v.renderLock.Unlock()
}

// SetFooter copies the given lines into the internal footer buffer, replacing
// any prior.
func (v *View) SetFooter(lines ...string) {
	v.renderLock.Lock()
	if cap(v.footer) < len(lines) {
		v.footer = make([]string, len(lines))
	} else {
		v.footer = v.footer[:len(lines)]
	}
	copy(v.footer, lines)
	v.renderLock.Unlock()
}

// Log adds a line to the internal log buffer. As much tail of the log buffer
// is displayed after the header as possible; at least 5 lines.
func (v *View) Log(mess string, args ...interface{}) {
	v.logs = append(v.logs, fmt.Sprintf(mess, args...))
}

// Update updates the grid, by passing it to a function, and then using
// whatever grid is returned; triggers a render, returning any error. The
// function also gets how much space is available (after header and footer); it
// may use this to choose to limit the returned grid, although this is not
// required.
func (v *View) Update(f func(Grid, point.Point) Grid) error {
	v.renderLock.Lock()
	defer v.renderLock.Unlock()
	if v.size.X <= 0 || v.size.Y <= 0 {
		v.size = termboxSize()
	}
	avail := v.size.Sub(point.Point{Y: len(v.footer) + len(v.header)})
	g := f(v.grid, avail)
	v.grid = g
	return v.render()
}

// Render renders the current view, returning any terminal drawing error.
func (v *View) Render() (rerr error) {
	v.renderLock.Lock()
	defer v.renderLock.Unlock()
	if v.size.X <= 0 || v.size.Y <= 0 {
		v.size = termboxSize()
	}
	return v.render()
}

func (v *View) render() (rerr error) {
	const minLogLines = 5

	if v.size.X <= 0 || v.size.Y <= 0 {
		return fmt.Errorf("bogus terminal size %v", v.size)
	}

	buf := Grid{
		Size: v.size,
		Data: make([]termbox.Cell, v.size.X*v.size.Y),
	}
	defer func() {
		if rerr == nil {
			rerr = termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
		}
		if rerr == nil {
			copy(termbox.CellBuffer(), buf.Data)
			rerr = termbox.Flush()
		}
	}()

	header := v.header
	space := v.size.Sub(v.grid.Size)
	space.Y -= len(header)
	space.Y -= len(v.footer)

	if len(v.logs) > 0 {
		nLogs := len(v.logs)
		if nLogs > minLogLines {
			nLogs = minLogLines
		}
		if n := nLogs + len(header); n > cap(header) {
			nh := make([]string, len(header), n)
			copy(nh, header)
			v.header = nh
			header = nh[:n]
		} else {
			header = header[:n]
		}
		copy(header[len(v.header):], v.logs[len(v.logs)-nLogs:])
		space.Y -= len(v.logs)
	}

	buf.Copy(v.grid)
	for i := 0; i < len(header); i++ {
		buf.WriteString(i, AlignLeft, header[i])
	}
	for i, j := len(v.footer)-1, 1; i >= 0; i, j = i-1, j+1 {
		buf.WriteString(v.size.Y-j, AlignRight, v.footer[i])
	}

	return nil
}

// Start takes control of the terminal and starts interaction loop (running on
// a new goroutine).
func (v *View) Start() error {
	if err := termbox.Init(); err != nil {
		return err
	}

	priorInputMode := termbox.SetInputMode(termbox.InputCurrent)
	defer termbox.SetInputMode(priorInputMode)

	termbox.SetInputMode(termbox.InputEsc)

	go v.run()
	return nil
}

// Stop stops any interaction started by Start.
func (v *View) Stop() {
	v.renderLock.Lock()
	v.running = false
	v.renderLock.Unlock()
}

func (v *View) run() {
	defer termbox.Close()

	const keyBufferSize = 1100

	if v.running {
		return
	}
	v.running = true

	v.err = nil
	v.keys = make(chan KeyEvent, keyBufferSize)
	v.done = make(chan struct{})
	v.size = termboxSize()

	v.err = func() error {
		for v.running {
			switch ev := termbox.PollEvent(); ev.Type {
			case termbox.EventKey:
				switch ev.Key {
				case termbox.KeyCtrlC:
					return nil
				case termbox.KeyCtrlL:
					if err := v.Render(); err != nil {
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
				if n := v.size.X * v.size.Y; n > cap(v.grid.Data) {
					m := len(v.grid.Data)
					data := make([]termbox.Cell, m, n)
					copy(data, v.grid.Data)
					v.grid.Data = data
				}
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

	close(v.done)
	v.keys = nil
	v.done = nil
}

func termboxSize() point.Point {
	w, h := termbox.Size()
	return point.Point{X: w, Y: h}
}
