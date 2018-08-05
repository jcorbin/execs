package terminal

import (
	"errors"
	"image"
	"os"

	copsTerm "github.com/jcorbin/execs/internal/cops/terminal"

	"github.com/jcorbin/execs/internal/terminfo"
)

const (
	signalCapacity = 16
	minRead        = 128
)

// Terminal supports interacting with a terminal:
// - in-band event reading
// - out-of-band event signaling
// - tracks cursor state combined with
// - an output buffer to at least coalesce writes (no front/back buffer
//   flipping is required or implied; the buffer serves as more of a command
//   queue)
type Terminal struct {
	closed  bool
	err     error
	in, out *os.File
	info    *terminfo.Terminfo
	signals chan os.Signal

	closeOptions []closeOption

	// output
	term   copsTerm.Terminal // TODO subsume this
	outbuf []byte
	cur    Cursor

	// input
	parseOffset int
	readOffset  int
	inbuf       []byte
	ea          *escapeAutomaton
}

// Open a terminal with the given options.
//
// If the returned *Terminal is non-nil, the user MUST call term.Close() to restore
//
// If the user wants to process input, they should call term.Notify() shortly
// after Open() to start event processing.
func Open(opts ...Option) (*Terminal, error) {
	term := &Terminal{
		in:      os.Stdin,
		out:     os.Stdout,
		cur:     StartCursor,
		inbuf:   make([]byte, minRead*2),
		outbuf:  make([]byte, 0, 1024),
		signals: make(chan os.Signal, signalCapacity),
	}

	for _, opt := range opts {
		if err := opt.preOpen(term); err != nil {
			return nil, err
		}
	}

	if term.info == nil {
		info, err := terminfo.Load(os.Getenv("TERM"))
		if err != nil {
			return nil, err
		}
		term.info = info
	}
	term.ea = newEscapeAutomaton(term.info)
	term.term = copsTerm.New(uintptr(term.out.Fd()))

	for _, opt := range opts {
		if err := opt.postOpen(term); err != nil {
			return term, err
		}
		if co, ok := opt.(closeOption); ok {
			term.closeOptions = append([]closeOption{co}, term.closeOptions...)
		}
	}

	return term, term.Flush()
}

// Close resets the terminal if its been Open()ed, returning any error
// encountered doing so.
func (term *Terminal) Close() error {
	if term.closed {
		return errors.New("terminal already closed")
	}
	term.closed = true
	for _, co := range term.closeOptions {
		// TODO support error handling
		co.preClose(term)
	}
	return term.Flush()
}

// Size reads and returns the current terminal size.
func (term *Terminal) Size() (image.Point, error) {
	// TODO cache last known good? hide error?
	return term.term.Size()
}

// TODO event stolen from termbox; reconcile with tcell

type (
	// EventType is the type of a terminal input event.
	EventType uint8

	// Event is a terminal input event, either read from the input file, or
	// delivered by a relevant signal.
	Event struct {
		Type EventType // one of Event* constants

		Mod Modifier // one of Mod* constants or 0
		Key Key      // one of Key* constants, invalid if 'Ch' is not 0
		Ch  rune     // a unicode character

		Mouse  image.Point // EventMouse
		Signal os.Signal   // EventSignal
	}
)

// Event types.
const (
	EventNone EventType = iota
	EventKey
	EventMouse
	EventEOF
	EventResize
	EventSignal
	EventInterrupt

	FirstUserEvent
)
