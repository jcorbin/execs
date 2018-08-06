package terminal

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"strconv"
	"unicode/utf8"

	"github.com/jcorbin/execs/internal/terminfo"
)

type parser struct {
	ea *escapeAutomaton
}

func (p parser) parse(buf []byte) (n int, ev Event) {
	if len(buf) == 0 {
		return 0, Event{}
	}
	switch c := buf[0]; {
	case c > utf8.RuneSelf: // non-trivial rune
		r, n := utf8.DecodeRune(buf)
		return n, Event{Type: EventKey, Ch: r}
	case c > 0x1f && c < 0x7f: // normal non-control character
		return 1, Event{Ch: rune(c)}
	case c == 0x1b: // escape (maybe sequence)
		return p.ea.parse(buf)
	default: // control character
		return 1, Event{Type: EventKey, Key: Key(c)}
	}
}

type escapeAutomaton struct {
	term [256]terminfo.KeyCode
	next [256]*escapeAutomaton
}

func newEscapeAutomaton(ti *terminfo.Terminfo) *escapeAutomaton {
	var ea escapeAutomaton
	for i, s := range ti.Keys {
		if len(s) > 0 {
			ea.addChain([]byte(s), terminfo.KeyCode(i))
		}
	}
	return &ea
}

func (ea *escapeAutomaton) addChain(bs []byte, kc terminfo.KeyCode) {
	for len(bs) > 1 {
		b := bs[0]
		next := ea.next[b]
		if next == nil {
			next = &escapeAutomaton{}
			ea.next[b] = next
		}
		ea = next
		bs = bs[1:]
	}
	b := bs[0]
	ea.term[b] = kc
}

func (ea *escapeAutomaton) lookup(bs []byte) (n int, _ terminfo.KeyCode) {
	for ea != nil && len(bs) > 1 {
		b := bs[0]
		if kc := ea.term[b]; kc != 0 {
			return n + 1, kc
		}
		ea = ea.next[b]
	}
	return 0, 0
}

func (ea *escapeAutomaton) parse(buf []byte) (n int, ev Event) {
	if n, kc := ea.lookup(buf); kc != 0 {
		return n, Event{
			Type: EventKey,
			Key:  Key(0xFFFF - (uint16(kc) - 1)),
		}
	}
	if bytes.HasPrefix(buf, []byte("\033[")) {
		if n, ev := parseMouseEvent(buf); n > 0 {
			return n, ev
		}
	}
	return 1, Event{Type: EventKey, Key: KeyEsc}
}

// TODO unify mouse escape sequence parsing and the escapeAutomaton

var errNoSemicolon = errors.New("missing ; in xterm mouse code")

func parseMouseEvent(buf []byte) (n int, ev Event) {
	if len(buf) < 4 {
		return 0, ev
	}
	switch buf[2] {
	case 'M':
		if len(buf) < 6 {
			return 0, ev
		}
		n, ev = parseX10MouseEvent(buf[3:6])
		n += 3
	case '<':
		if len(buf) < 8 {
			return 0, ev
		}
		n, ev = parseXtermMouseEvent(buf[3:])
		n += 3
	default:
		if len(buf) < 7 {
			return 0, ev
		}
		n, ev = parseUrxvtMouseEvent(buf[2:])
		n += 2
	}
	ev.Mouse = ev.Mouse.Sub(image.Pt(1, 1)) // the coord is 1,1 for upper left
	return n, ev
}

// X10 mouse encoding, the simplest one: \033 [ M Cb Cx Cy
func parseX10MouseEvent(buf []byte) (n int, ev Event) {
	ev = parseX10MouseEventByte(int64(buf[0]) - 32)
	ev.Mouse = image.Pt(int(buf[1])-32, int(buf[2])-32)
	return 3, ev
}

// xterm 1006 extended mode: \033 [ < Cb ; Cx ; Cy (M or m)
func parseXtermMouseEvent(buf []byte) (n int, ev Event) {
	mi := bytes.IndexAny(buf, "Mm")
	if mi == -1 {
		return 0, ev
	}

	b, x, y, err := parseXtermMouseComponents(buf[:mi])
	if err != nil {
		return 0, ev
	}

	// unlike x10 and urxvt, in xterm Cb is already zero-based
	ev = parseX10MouseEventByte(b)
	if buf[mi] != 'M' {
		// on xterm mouse release is signaled by lowercase m
		ev.Key = MouseRelease
	}
	ev.Mouse = image.Pt(int(x), int(y))
	return mi + 1, ev
}

// urxvt 1015 extended mode: \033 [ Cb ; Cx ; Cy M
func parseUrxvtMouseEvent(buf []byte) (n int, ev Event) {
	mi := bytes.IndexByte(buf, 'M')
	if mi == -1 {
		return 0, ev
	}

	b, x, y, err := parseXtermMouseComponents(buf[:mi])
	if err != nil {
		return 0, ev
	}

	ev = parseX10MouseEventByte(b - 32)
	ev.Mouse = image.Pt(int(x), int(y))

	return mi + 1, ev
}

func parseX10MouseEventByte(b int64) (ev Event) {
	ev.Type = EventMouse

	switch b & 3 {
	case 0:
		if b&64 != 0 {
			ev.Key = MouseWheelUp
		} else {
			ev.Key = MouseLeft
		}
	case 1:
		if b&64 != 0 {
			ev.Key = MouseWheelDown
		} else {
			ev.Key = MouseMiddle
		}
	case 2:
		ev.Key = MouseRight

	case 3:
		ev.Key = MouseRelease
	}

	if b&32 != 0 {
		ev.Mod |= ModMotion
	}
	return ev
}

func parseXtermMouseComponents(buf []byte) (b, x, y int64, err error) {
	// Cb ;
	i := bytes.IndexByte(buf, ';')
	if i == -1 {
		return 0, 0, 0, errNoSemicolon
	}
	s := string(buf[:i])
	b, err = strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid Cb=%q: %v", s, err)
	}
	buf = buf[i+1:]

	// Cx ;
	i = bytes.IndexByte(buf, ';')
	if i == -1 {
		return 0, 0, 0, errNoSemicolon
	}
	s = string(buf[:i])
	x, err = strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid Cx=%q: %v", s, err)
	}
	buf = buf[i+1:]

	// Cy
	s = string(buf)
	y, err = strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid Cy=%q: %v", s, err)
	}

	return b, x, y, nil
}
