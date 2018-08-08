package terminal

import (
	"errors"
	"os"
	"os/signal"
	"syscall"
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
	Processor
	Output

	closed bool
	termContext
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

	term := &Terminal{}
	term.Decoder.File = in
	term.Output.File = out
	term.termContext = &term.Attr

	term.Processor.Init()
	term.Output.Init()

	if err := opt.init(term); err != nil {
		return nil, err
	}

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
	err := term.Processor.Close()
	if cerr := term.termContext.exit(term); err == nil {
		err = cerr
	}

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

// Suspend the terminal program: restore terminal state, send SIGTSTP, wait for
// SIGCONT, then re-setup terminal state once we're back. It returns any error
// encountered or the received SIGCONT signal for completeness on success.
func (term *Terminal) Suspend() (os.Signal, error) {
	if err := term.termContext.exit(term); err != nil {
		return nil, err
	}
	contCh := make(chan os.Signal, 1)
	signal.Notify(contCh, syscall.SIGCONT)
	if err := syscall.Kill(0, syscall.SIGTSTP); err != nil {
		return nil, err
	}
	sig := <-contCh
	return sig, term.termContext.enter(term)
}
