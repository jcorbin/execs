package terminal

import (
	"image"
	"os"
	"syscall"
	"unsafe"

	"github.com/pkg/term/termios"
)

// Attr implements a terminal attribute manipulation.
//
// TODO elaborate...
type Attr struct {
	File   *os.File
	orig   syscall.Termios
	cur    syscall.Termios
	raw    bool
	echo   bool
	active bool
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
	if at.active {
		at.cur = at.orig
		at.apply(&at.cur)
		return at.SetAttr(at.cur)
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
	if at.active {
		if echo {
			at.cur.Lflag |= syscall.ECHO
		} else {
			at.cur.Lflag &^= syscall.ECHO
		}
		return at.SetAttr(at.cur)
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

// Activate the terminal attributes if in-active; all future calls to Set* now
// apply immediately.
func (at *Attr) Activate() (err error) {
	if !at.active {
		if at.orig, err = at.GetAttr(); err != nil {
			return err
		}
		at.cur = at.orig
		at.apply(&at.cur)
		if err = at.SetAttr(at.cur); err != nil {
			return err
		}
		at.active = true
	}
	return nil
}

// Deactivate the terminal attributes, restoring the original ones recorded by
// Activate.
func (at *Attr) Deactivate() error {
	if at.active {
		at.active = false
		return at.SetAttr(at.orig)
	}
	return nil
}

func (at *Attr) ioctl(request, arg1, arg2, arg3, arg4 uintptr) error {
	if _, _, e := syscall.Syscall6(syscall.SYS_IOCTL, at.File.Fd(), request, arg1, arg2, arg3, arg4); e != 0 {
		return e
	}
	return nil
}

// GetAttr retrieves terminal attributes.
func (at *Attr) GetAttr() (attr syscall.Termios, err error) {
	err = at.ioctl(syscall.TIOCGETA, uintptr(unsafe.Pointer(&attr)), 0, 0, 0)
	return
}

// SetAttr sets terminal attributes.
func (at *Attr) SetAttr(attr syscall.Termios) error {
	return at.ioctl(syscall.TIOCSETA, uintptr(unsafe.Pointer(&attr)), 0, 0, 0)
}

// Size reads and returns the current terminal size.
func (at *Attr) Size() (size image.Point, err error) {
	// TODO cache last known good? hide error?
	var dim struct {
		rows    uint16
		cols    uint16
		xpixels uint16
		ypixels uint16
	}
	err = at.ioctl(syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&dim)), 0, 0, 0)
	if err == nil {
		size.X = int(dim.cols)
		size.Y = int(dim.rows)
	}
	return size, err
}
