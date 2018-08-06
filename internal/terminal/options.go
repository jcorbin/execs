package terminal

import (
	"os"
	"os/signal"
	"runtime"
	"sync"
	"time"

	"github.com/jcorbin/execs/internal/terminfo"
)

// Option is an opaque option to pass to Open().
type Option interface {
	// preOpen gets called while initializing internal terminal state, but
	// before actual external resource interaction.
	preOpen(term *Terminal) error

	// postOpen gets called during initial external resource interaction; it
	// should do things like change terminal mode and initialize cursor state.
	postOpen(term *Terminal) error
}

type closeOption interface {
	// preClose gets called while closing the terminal, it should do things
	// like restore terminal mode and cursor state.
	preClose(term *Terminal) error
}

type writeOption interface {
	// preWrite gets called before a write to the output buffer giving a
	// chance to flush; n is a best-effort size of the bytes about to be
	// written. NOTE preWrite MUST avoid manipulating cursor state, as it may
	// reflect state about to be implemented by the written bytes.
	preWrite(term *Terminal, n int) error

	// postWrite gets called after a write to the output buffer giving a chance to flush.
	postWrite(term *Terminal, n int) error
}

// Options creates a compound option from 0 or more options (returns nil in the
// 0 case).
func Options(opts ...Option) Option {
	if len(opts) == 0 {
		return nil
	}
	a := opts[0]
	opts = opts[1:]
	for len(opts) > 0 {
		b := opts[0]
		opts = opts[1:]
		if a == nil {
			a = b
			continue
		} else if b == nil {
			continue
		}
		as, haveAs := a.(options)
		bs, haveBs := b.(options)
		if haveAs && haveBs {
			a = append(as, bs...)
		} else if haveAs {
			a = append(as, b)
		} else if haveBs {
			a = append(options{a}, bs)
		} else {
			a = options{a, b}
		}
	}
	return a
}

type options []Option

func (os options) preOpen(term *Terminal) error {
	for i := range os {
		if err := os[i].preOpen(term); err != nil {
			return err
		}
	}
	return nil
}
func (os options) postOpen(term *Terminal) error {
	for i := range os {
		if err := os[i].postOpen(term); err != nil {
			return err
		}
		if co, ok := os[i].(closeOption); ok {
			term.closeOption = chainCloseOption(term.closeOption, co)
		}
		if wo, ok := os[i].(writeOption); ok {
			term.setWriteOption(wo)
		}
	}
	return nil
}

func chainCloseOption(a, b closeOption) closeOption {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	as, haveAs := a.(closeOptions)
	bs, haveBs := b.(closeOptions)
	if haveAs && haveBs {
		return append(as, bs...)
	} else if haveAs {
		return append(closeOptions{b}, as)
	} else if haveBs {
		return append(bs, a)
	}
	return a
}

type closeOptions []closeOption
type preCloseFunc func(*Terminal) error
type preOpenFunc func(*Terminal) error
type postOpenFunc func(*Terminal) error
type termOption struct{ enter, exit terminfo.FuncCode }
type cursorOption struct{ enter, exit []Curse }

func (cos closeOptions) preClose(term *Terminal) (err error) {
	for i := range cos {
		if cerr := cos[i].preClose(term); err == nil {
			err = cerr
		}
	}
	return err
}

func (f preOpenFunc) preOpen(term *Terminal) error  { return f(term) }
func (f preOpenFunc) postOpen(term *Terminal) error { return nil }

func (f postOpenFunc) preOpen(term *Terminal) error  { return nil }
func (f postOpenFunc) postOpen(term *Terminal) error { return f(term) }

func (f preCloseFunc) preOpen(term *Terminal) error  { return nil }
func (f preCloseFunc) postOpen(term *Terminal) error { return nil }
func (f preCloseFunc) preClose(term *Terminal) error { return f(term) }

func (to termOption) preOpen(term *Terminal) error { return nil }
func (to termOption) postOpen(term *Terminal) error {
	fn := term.info.Funcs[to.enter]
	term.outbuf = append(term.outbuf, fn...)
	return nil
}
func (to termOption) preClose(term *Terminal) error {
	fn := term.info.Funcs[to.enter]
	term.outbuf = append(term.outbuf, fn...)
	return nil
}

func (co cursorOption) preOpen(term *Terminal) error { return nil }
func (co cursorOption) postOpen(term *Terminal) error {
	if len(co.enter) > 0 {
		_, err := term.WriteCursor(co.enter...)
		return err
	}
	return nil
}
func (co cursorOption) preClose(term *Terminal) error {
	if len(co.exit) > 0 {
		_, err := term.WriteCursor(co.exit...)
		return err
	}
	return nil
}

// DefaultTerminfo loads default terminfo based on the TERM environment
// variable; basically it uses terminfo.Load(os.Getenv("TERM")).
var DefaultTerminfo = preOpenFunc(func(term *Terminal) error {
	if term.info != nil {
		return nil
	}
	info, err := terminfo.Load(os.Getenv("TERM"))
	if err == nil {
		term.info = info
		term.ea = newEscapeAutomaton(term.info)
	}
	return err
})

// Terminfo overrides any DefaultTerminfo with an explicit choice.
func Terminfo(info *terminfo.Terminfo) Option {
	return preOpenFunc(func(term *Terminal) error {
		term.info = info
		term.ea = newEscapeAutomaton(term.info)
		return nil
	})
}

// Signals provides one or more signals that the terminal should be notified
// of. Special handling is provided for:
// - syscall.SIGWINCH will be synthesized into an EventResize
// - syscall.SIGINT will be synthesized into an EventInterrupt
// - syscall.SIGTERM will halt input even processing, and close the input
//   stream resulting in an eventual EventEOF
// - other events are passed through as EventSignal
func Signals(sigs ...os.Signal) Option {
	return postOpenFunc(func(term *Terminal) error {
		signal.Notify(term.signals, sigs...)
		return nil
	})
}

// SignalCapacity sets the buffer capacity for backlogged signals; the default
// is 16.
func SignalCapacity(n int) Option {
	return preOpenFunc(func(term *Terminal) error {
		term.signals = make(chan os.Signal, n)
		return nil
	})
}

// OutbufCapacity sets the initial capacity of the output buffer, which
// defaults to 1KiB.
func OutbufCapacity(n int) Option {
	return preOpenFunc(func(term *Terminal) error {
		term.outbuf = make([]byte, 0, n)
		return nil
	})
}

// TODO more general termios mode option
type rawMode struct {
}

func (rm rawMode) preOpen(*Terminal) error       { return nil }
func (rm rawMode) postOpen(term *Terminal) error { return term.term.SetRaw() }
func (rm rawMode) preClose(term *Terminal) error { return term.term.Restore() }

// RawMode will put the terminal into raw mode when opening, and restore it
// during close.
var RawMode Option = rawMode{}

// CursorOption specified cursor manipulator(s) to apply during open and close.
func CursorOption(enter, exit []Curse) Option {
	return cursorOption{enter, exit}
}

// HiddenCursor is a CursorOption that hides the cursor, homes it, and clears
// the screen during open; the converse home/clear/show is done during close.
var HiddenCursor Option = cursorOption{
	[]Curse{Cursor.Hide, Cursor.Home, Cursor.Clear},
	[]Curse{Cursor.Home, Cursor.Clear, Cursor.Show},
}

// MouseReporting enables mouse reporting after opening the terminal, and
// disables it when closing the terminal.
var MouseReporting Option = termOption{
	terminfo.FuncEnterMouse,
	terminfo.FuncExitMouse,
}

// FlushWhenFull causes a terminal's output buffer to prefer to flush rather
// than grow, similar to a bufio.Writer.
//
// TODO avoid writing large buffers and string indirectly, ability to pass
// through does not exist currently.
//
// NOTE mutually exclusive with any other Flush* options; the last one wins.
var FlushWhenFull Option = flushWhenFull{}

type flushWhenFull struct{}

func (fw flushWhenFull) preOpen(term *Terminal) error {
	return nil
}
func (fw flushWhenFull) postOpen(term *Terminal) error { return nil }
func (fw flushWhenFull) preWrite(term *Terminal, n int) error {
	if m := len(term.outbuf); m > 0 && m+n >= cap(term.outbuf) {
		return term.Flush()
	}
	return nil
}
func (fw flushWhenFull) postWrite(term *Terminal, n int) error {
	if m := len(term.outbuf); m > 0 && m == cap(term.outbuf) {
		return term.Flush()
	}
	return nil
}

// FlushAfter implements an Option that causes a terminal to flush its output
// some specified time after the first write to it. The user should retain and
// lock their FlushAfter instance during their drawing update routine so that
// partial output does not get flushed. Example usage:
//
//	fa := terminal.FlushAfter{Duration: time.Second / 60}
//	term, err := terminal.Open(nil, nil, terminal.Options(&fa))
//	if term != nil {
//		defer term.Close()
//	}
//	var ev terminal.Event
//	for err == nil {
//		fa.Lock()                  // exclude flushing partial output while...
//		term.Discard()             // ... drop any undrawn output from last round
//		draw(term, ev)             // ... call term.Write* to draw new output
//		fa.Unlock()                // ... end exclusion
//		ev, err = term.ReadEvent() // block for next input event
//	}
//	// TODO err handling
//
// NOTE mutually exclusive with any other Flush* options; the last one wins.
type FlushAfter struct {
	sync.Mutex
	time.Duration

	term *Terminal
	set  bool
	stop chan struct{}
	t    *time.Timer
}

func (fa *FlushAfter) preOpen(term *Terminal) error {
	fa.term = term
	return nil
}
func (fa *FlushAfter) postOpen(term *Terminal) error { return nil }
func (fa *FlushAfter) preWrite(term *Terminal, n int) error {
	fa.term = term
	fa.Start()
	return nil
}
func (fa *FlushAfter) postWrite(term *Terminal, n int) error { return nil }

// Start the flush timer, allocating and spawn its monitor goroutine if
// necessary. Should only be called by the user in a locked section.
func (fa *FlushAfter) Start() {
	if fa.t == nil {
		fa.t = time.NewTimer(fa.Duration)
		fa.stop = make(chan struct{})
		go fa.monitor(fa.t.C, fa.stop)
	} else if !fa.set {
		fa.t.Reset(fa.Duration)
	}
	fa.set = true
}

// Stop the flush timer and any monitor goroutine. Should only be called by the
// user in a locked section.
func (fa *FlushAfter) Stop() {
	if fa.stop != nil {
		close(fa.stop)
		fa.t.Stop()
		fa.t = nil
		fa.stop = nil
		fa.set = false
	}
}

// Cancel any flush timer, returning true if one was canceled; users should
// call this method after any manual terminal flush. Should only be called by
// the user in a locked section.
func (fa *FlushAfter) Cancel() bool {
	fa.set = false
	if fa.t == nil {
		return false
	}
	return fa.t.Stop()
}

func (fa *FlushAfter) monitor(ch <-chan time.Time, stop <-chan struct{}) {
	runtime.LockOSThread() // dedicate this thread to terminal writing
	done := false
	for !done {
		select {
		case <-stop:
			done = true
		case t := <-ch:
			if fa.flush(t) != nil {
				break
			}
		}
	}
	fa.Lock()
	defer fa.Unlock()
	if fa.t != nil && fa.t.C == ch {
		fa.t = nil
		fa.set = false
	}
	if fa.stop == stop {
		fa.stop = nil
	}
}

func (fa *FlushAfter) flush(_ time.Time) error {
	fa.Lock()
	defer fa.Unlock()
	fa.set = false
	return fa.term.Flush()
}

func (term *Terminal) setWriteOption(wo writeOption) {
	if fa, ok := term.writeOption.(*FlushAfter); ok {
		fa.Stop()
	}
	if wo == nil {
		term.writeOption = FlushWhenFull.(writeOption)
	} else {
		term.writeOption = wo
	}
}
