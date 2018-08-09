package terminal

import (
	"github.com/jcorbin/execs/internal/terminfo"
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
		return append(as, b)
	} else if haveBs {
		return append(termContexts{a}, bs...)
	}
	return termContexts{a, b}
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
	if fn := term.Terminfo().Funcs[to.enterFunc]; fn != "" {
		_, _ = term.Output.buf.WriteString(fn)
	}
	return nil
}
func (to termOption) exit(term *Terminal) error {
	if fn := term.Terminfo().Funcs[to.exitFunc]; fn != "" {
		_, _ = term.Output.buf.WriteString(fn)
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

// RawMode is an option that puts the Terminal's Attr into raw mode immediately
// during Open. See Attr.SetRaw.
var RawMode Option = optionFunc(func(term *Terminal) error {
	term.Attr.raw = true
	return nil
})
