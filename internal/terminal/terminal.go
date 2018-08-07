package terminal

import (
	"bytes"
	"errors"
	"image"
	"os"
	"os/signal"
	"syscall"
	"unsafe"

	"github.com/jcorbin/execs/internal/terminfo"
	"github.com/jcorbin/execs/internal/termkey"
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
	Attr

	closed  bool
	signals chan os.Signal
	info    *terminfo.Terminfo

	termContext
	writeObserver
	eventFilter

	// output
	out    *os.File
	cur    Cursor
	tmp    []byte
	outbuf bytes.Buffer
	outerr error

	// input
	in         *os.File
	inbuf      bytes.Buffer
	inerr      error
	keyDecoder *termkey.Decoder
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
		in:      in,
		out:     out,
		cur:     StartCursor,
		tmp:     make([]byte, 64),
		signals: make(chan os.Signal, signalCapacity),

		writeObserver: flushWhenFull{},
	}
	term.termContext = &term.Attr
	if err := opt.init(term); err != nil {
		return nil, err
	}

	term.keyDecoder = termkey.NewDecoder(term.info)

	if err := term.termContext.enter(term); err != nil {
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

	err := term.termContext.exit(term)

	// TODO do this only if the cursor isn't homed on a new row (requires
	// cursor to have been parsing and following output all along...)?
	_, _ = term.WriteString("\r\n")

	if ferr := term.Flush(); err == nil {
		err = ferr
	}
	return err
}

func (term *Terminal) closeOnPanic() {
	if e := recover(); e != nil {
		if !term.closed {
			_ = term.Close()
		}
		panic(e)
	}
}

func (term *Terminal) ioctl(request, argp uintptr) error {
	if _, _, e := syscall.Syscall6(syscall.SYS_IOCTL, term.out.Fd(), request, argp, 0, 0, 0); e != 0 {
		return e
	}
	return nil
}

// GetAttr retrieves terminal attributes.
//
// NOTE this is a low level method, most users should use the Attr Option.
func (term *Terminal) GetAttr() (attr syscall.Termios, err error) {
	err = term.ioctl(syscall.TIOCGETA, uintptr(unsafe.Pointer(&attr)))
	return
}

// SetAttr sets terminal attributes.
//
// NOTE this is a low level method, most users should use the Attr Option.
func (term *Terminal) SetAttr(attr syscall.Termios) error {
	return term.ioctl(syscall.TIOCSETA, uintptr(unsafe.Pointer(&attr)))
}

// Size reads and returns the current terminal size.
func (term *Terminal) Size() (size image.Point, err error) {
	// TODO cache last known good? hide error?
	var dim struct {
		rows    uint16
		cols    uint16
		xpixels uint16
		ypixels uint16
	}
	err = term.ioctl(syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&dim)))
	if err == nil {
		size.X = int(dim.cols)
		size.Y = int(dim.rows)
	}
	return size, err
}
