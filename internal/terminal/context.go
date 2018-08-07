package terminal

import (
	"syscall"

	"github.com/jcorbin/execs/internal/terminfo"
	"github.com/pkg/term/termios"
)

type termContext interface {
	enter(term *Terminal) error
	exit(term *Terminal) error
}

func chainTermContext(a, b termContext) termContext {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	as, haveAs := a.(termContexts)
	bs, haveBs := b.(termContexts)
	if haveAs && haveBs {
		return append(as, bs...)
	} else if haveAs {
		return append(termContexts{b}, as)
	} else if haveBs {
		return append(bs, a)
	}
	return a
}

type termContexts []termContext

func (tcs termContexts) enter(term *Terminal) error {
	for i := 0; i < len(tcs); i++ {
		if err := tcs[i].enter(term); err != nil {
			// TODO tcs[:i+1].exit(term)?
			return err
		}
	}
	return nil
}
func (tcs termContexts) exit(term *Terminal) (rerr error) {
	for i := len(tcs) - 1; i >= 0; i-- {
		if err := tcs[i].exit(term); rerr == nil {
			rerr = err
		}
	}
	return rerr
}

type termOption struct{ enterFunc, exitFunc terminfo.FuncCode }

func (to termOption) init(term *Terminal) error {
	term.termContext = chainTermContext(term.termContext, to)
	return nil
}
func (to termOption) enter(term *Terminal) error {
	if fn := term.info.Funcs[to.enterFunc]; fn != "" {
		_, _ = term.outbuf.WriteString(fn)
	}
	return nil
}
func (to termOption) exit(term *Terminal) error {
	if fn := term.info.Funcs[to.exitFunc]; fn != "" {
		_, _ = term.outbuf.WriteString(fn)
	}
	return nil
}

// CursorOption specified cursor manipulator(s) to apply during open and close.
func CursorOption(enter, exit []Curse) Option {
	return cursorOption{enter, exit}
}

type cursorOption struct{ enterCurses, exitCurses []Curse }

func (co cursorOption) init(term *Terminal) error {
	term.termContext = chainTermContext(term.termContext, co)
	return nil
}

func (co cursorOption) enter(term *Terminal) error {
	if len(co.enterCurses) > 0 {
		_, err := term.WriteCursor(co.enterCurses...)
		return err
	}
	return nil
}

func (co cursorOption) exit(term *Terminal) error {
	if len(co.exitCurses) > 0 {
		_, err := term.WriteCursor(co.exitCurses...)
		return err
	}
	return nil
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

func (at *Attr) enter(term *Terminal) (err error) {
	if at.orig, err = term.GetAttr(); err != nil {
		return err
	}
	at.cur = at.orig
	at.apply(&at.cur)
	if err = term.SetAttr(at.cur); err != nil {
		return err
	}
	at.term = term
	return nil
}

func (at *Attr) exit(term *Terminal) error {
	at.term = nil
	return term.SetAttr(at.orig)
}

// RawMode is an option that puts the Terminal's Attr into raw mode immediately
// during Open. See Attr.SetRaw.
var RawMode Option = optionFunc(func(term *Terminal) error {
	term.Attr.raw = true
	return nil
})
