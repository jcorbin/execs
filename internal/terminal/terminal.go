package terminal

import (
	"errors"
	"image"
	"os"
	"os/signal"

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
	signals chan os.Signal
	info    *terminfo.Terminfo

	closeOption
	writeOption

	// output
	out    *os.File
	cur    Cursor
	tmp    []byte
	outbuf []byte
	outerr error
	term   copsTerm.Terminal // TODO subsume this

	// input
	in          *os.File
	parseOffset int
	readOffset  int
	inbuf       []byte
	inerr       error
	parser
}

// Open a terminal on the given input/output file pair (defaults to os.Stdin
// and os.Stdout) with the given option(s).
//
// If the user wants to process input, they should call term.Notify() shortly
// after Open() to start event processing.
func Open(in, out *os.File, opt Option) (*Terminal, error) {
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	opt = Options(opt, DefaultTerminfo)
	term := &Terminal{
		in:          in,
		out:         out,
		cur:         StartCursor,
		tmp:         make([]byte, 64),
		inbuf:       make([]byte, minRead*2),
		outbuf:      make([]byte, 0, 1024),
		signals:     make(chan os.Signal, signalCapacity),
		writeOption: FlushWhenFull.(writeOption),
	}
	if err := opt.preOpen(term); err != nil {
		return nil, err
	}
	term.term = copsTerm.New(uintptr(term.out.Fd()))
	if err := opt.postOpen(term); err != nil {
		_ = term.Close()
		return nil, err
	}
	return term, nil
}

// Close resets the terminal, flushing any buffered output.
func (term *Terminal) Close() error {
	if term.closed {
		return errors.New("terminal already closed")
	}
	term.closed = true
	signal.Stop(term.signals)
	var err error
	if term.closeOption != nil {
		err = term.closeOption.preClose(term)
	}

	// TODO do this only if the cursor isn't homed on a new row (requires
	// cursor to have been parsing and following output all along...)?
	term.WriteString("\r\n")

	if ferr := term.Flush(); err == nil {
		err = ferr
	}
	return err
}

func (term *Terminal) closeOnPanic() {
	if e := recover(); e != nil {
		if !term.closed {
			term.Close()
		}
		panic(e)
	}
}

// Size reads and returns the current terminal size.
func (term *Terminal) Size() (image.Point, error) {
	// TODO cache last known good? hide error?
	return term.term.Size()
}
