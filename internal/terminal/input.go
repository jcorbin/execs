package terminal

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"
	"os/signal"
	"strconv"
	"syscall"
	"unicode/utf8"

	"github.com/jcorbin/execs/internal/terminfo"
)

// Notify the given event and error channels of terminal input events (both
// read in-band, and signal out-of-band).
//
// Starts multiple goroutines for reading events, and synthesizing os signals,
// but does NOT call signal.Notify. User MUST enable any signals that they
// want to receive events about by calling things like:
//     signal.Notify(term.Signals, syscall.SIGINT, syscall.SIGWINCH)
func (term *Terminal) Notify(events chan<- Event, errs chan<- error) {
	done := make(chan struct{})
	go term.readEvents(events, errs, done)
	go term.synthesize(events, errs, done)
}

func (term *Terminal) synthesize(events chan<- Event, errs chan<- error, readDone <-chan struct{}) {
	defer func() {
		signal.Stop(term.signals)
		_ = term.in.Close()
	}()
	for {
		select {
		case <-readDone:
			return
		case sig := <-term.signals:
			var ev Event
			switch sig {
			case syscall.SIGTERM:
				return
			case syscall.SIGINT:
				ev.Type = EventInterrupt
			case syscall.SIGWINCH:
				ev.Type = EventResize
			default:
				ev.Type = EventSignal
				ev.Signal = sig
			}
			select {
			case events <- ev:
			default:
			}
		}
	}
}

func (term *Terminal) readEvents(events chan<- Event, errs chan<- error, done chan<- struct{}) {
	for {
		if n, ev := term.parse(); ev.Type != EventNone {
			term.parseOffset += n
			events <- ev
			continue
		}
		if err := term.readMore(minRead); err == io.EOF {
			events <- Event{Type: EventEOF}
			break
		} else if err != nil {
			errs <- err
			break
		}
	}
	close(done)
}

// ReadEvent reads one event from the input file; this may happen from
// previously read / in-buffer bytes, and may not necessarily block.
//
// NOTE this is a lower level method, most users should use the above Notify method.
func (term *Terminal) ReadEvent() (Event, error) {
	for {
		if n, ev := term.parse(); ev.Type != EventNone {
			term.parseOffset += n
			return ev, nil
		}
		if err := term.readMore(minRead); err != nil {
			return Event{}, err
		}
	}
}

func (term *Terminal) readMore(n int) error {
	if term.err != nil {
		return term.err
	}
	for len(term.inbuf)-term.readOffset < n {
		if term.parseOffset > 0 {
			// try to free space by shifting down over parsed bytes
			copy(term.inbuf, term.inbuf[term.parseOffset:])
			term.readOffset -= term.parseOffset
			term.parseOffset = 0
		} else {
			// reallocate a bigger buffer
			buf := make([]byte, len(term.inbuf)*2)
			copy(buf, term.inbuf)
			term.inbuf = buf
		}
	}
	n, term.err = term.in.Read(term.inbuf[term.readOffset:])
	term.readOffset += n
	return nil
}

func (term *Terminal) parse() (n int, ev Event) {
	buf := term.inbuf[term.parseOffset:]
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
		return term.ea.parse(buf)
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
		ea.addChain([]byte(s), terminfo.KeyCode(i))
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

var errNoSemicolon = errors.New("missing ; in xterm mouse code")

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
