package terminal

import (
	"os"
	"os/signal"

	"github.com/jcorbin/execs/internal/terminfo"
)

// Option is an opaque option to pass to Open().
type Option interface {
	preOpen(*Terminal) error
	postOpen(*Terminal) error
}

type closeOption interface {
	Option
	preClose(*Terminal) error
}

type preCloseFunc func(*Terminal) error
type preOpenFunc func(*Terminal) error
type postOpenFunc func(*Terminal) error
type termOption struct{ enter, exit terminfo.FuncCode }
type cursorOption struct{ enter, exit []Curse }

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
		term.Swear(co.enter...)
	}
	return nil
}
func (co cursorOption) preClose(term *Terminal) error {
	if len(co.exit) > 0 {
		term.Swear(co.exit...)
	}
	return nil
}

// Input specifies an alternate file handle for reading in-band terminal from
// the default os.Stdin.
func Input(in *os.File) Option {
	return preOpenFunc(func(term *Terminal) error {
		term.in = in
		return nil
	})
}

// Output specifies an alternate file handle for writing terminal output from
// the default os.Stdout.
func Output(out *os.File) Option {
	return preOpenFunc(func(term *Terminal) error {
		term.out = out
		return nil
	})
}

// Terminfo overrides the default terminfo.Load(os.Getenv("TERM")).
func Terminfo(info *terminfo.Terminfo) Option {
	return preOpenFunc(func(term *Terminal) error {
		term.info = info
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
func RawMode() Option {
	// TODO should hide/show cursor automatically?
	return rawMode{}
}

// CursorOption specified cursor manipulator(s) to apply during open and close.
func CursorOption(enter, exit []Curse) Option {
	return cursorOption{enter, exit}
}

// HiddenCursor is a CursorOption that hides the cursor, homes it, and clears
// the screen during open; the converse home/clear/show is done during close.
func HiddenCursor() Option {
	return cursorOption{
		[]Curse{Cursor.Hide, Cursor.Home, Cursor.Clear},
		[]Curse{Cursor.Home, Cursor.Clear, Cursor.Show},
	}
}

// MouseReporting enables mouse reporting after opening the terminal, and
// disables it when closing the terminal.
func MouseReporting() Option {
	return termOption{
		terminfo.FuncEnterMouse,
		terminfo.FuncExitMouse,
	}
}

// TODO a "flush policy" could delineate between:
// - coalescing: "flush every N bytes"
// - frame rate throttling: "flush after T time"; NOTE
//   - needs to happen between user sessions of writing to output buffer
//   - therefore probably implies an EventFlush type therefore
//   - the timer probably wants to start at one of:
//     a. the first write into the output buffer
//     b. after delivery of an invalidating event (perhaps just EventFlush?)
