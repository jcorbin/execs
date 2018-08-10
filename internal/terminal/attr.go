package terminal

import (
	"errors"
	"syscall"

	"github.com/pkg/term/termios"
)

// Attr implements a terminal attribute manipulation.
//
// TODO elaborate...
type Attr struct {
	orig syscall.Termios
	cur  syscall.Termios
	raw  bool
	echo bool
	term *Terminal // non-nil after enter and before exit
}

// SetRaw controls whether the terminal should be in raw raw mode.
//
// Raw mode is suitable for full-screen terminal user interfaces, eliminating
// keyboard shortcuts for job control, echo, line buffering, and escape key
// debouncing.
//
// If Attr is attached to an active Terminal, than this applies immediately;
// otherwise it's a forward setting terminal setup.
func (at *Attr) SetRaw(raw bool) error {
	if raw == at.raw {
		return nil
	}
	at.raw = raw
	if at.term != nil {
		at.cur = at.orig
		at.apply(&at.cur)
		return at.term.SetAttr(at.cur)
	}
	return nil
}

// SetEcho toggles input echoing mode, which is off by default in raw mode, and
// on in normal mode.
//
// If Attr is attached to an active Terminal, than this applies immediately;
// otherwise it's a forward setting terminal setup.
func (at *Attr) SetEcho(echo bool) error {
	if echo == at.echo {
		return nil
	}
	at.echo = echo
	if at.term != nil {
		if echo {
			at.cur.Lflag |= syscall.ECHO
		} else {
			at.cur.Lflag &^= syscall.ECHO
		}
		return at.term.SetAttr(at.cur)
	}
	return nil
}

func (at *Attr) apply(attr *syscall.Termios) {
	if at.raw {
		// TODO naturalize / decompose
		// TODO read things like antirez's kilo notes again
		termios.Cfmakeraw(attr)
	}
	if at.echo {
		at.cur.Lflag |= syscall.ECHO
	} else {
		at.cur.Lflag &^= syscall.ECHO
	}
}

// Enter ensures desired termios state, and retains a reference to the passed
// terminal so that any future calls to Set* calls are immediate.
func (at *Attr) Enter(term *Terminal) (err error) {
	if at.term != nil {
		if term != at.term {
			return errors.New("terminal.Attr got a foreign Enter")
		}
		if at.term.active {
			return nil
		}
	}
	if term.closed {
		return errors.New("cannot enter closed terminal")
	}
	if at.term == nil {
		if at.orig, err = term.GetAttr(); err != nil {
			return err
		}
		at.cur = at.orig
		at.apply(&at.cur)
		at.term = term
	}
	if err = term.SetAttr(at.cur); err != nil {
		return err
	}
	at.term.active = true
	return nil
}

// Exit restores termios state to before the last Enter(), clearing any
// retained terminal pointer.
func (at *Attr) Exit(term *Terminal) error {
	if term != at.term {
		return errors.New("terminal.Attr got a foreign Exit")
	}
	if term.closed {
		return errors.New("cannot exit closed terminal")
	}
	at.term.active = false
	at.term = nil
	return term.SetAttr(at.orig)
}
