package terminal

import (
	"io"
	"os"
	"syscall"
)

// ReadEvent reads one event from the input file; this may happen from
// previously read / in-buffer bytes, and may not necessarily block.
//
// NOTE this is a lower level method, most users should use term.Run() instead.
func (term *Terminal) ReadEvent() (Event, error) {
	var tmp [1]Event
	n, err := term.ReadEvents(tmp[:])
	var ev Event
	if n > 0 {
		ev = tmp[0]
	}
	return ev, err
}

// ReadEvents reads events into the given slice, stopping either when there are
// no more buffered inputs bytes to parse, or the given events buffer is full.
// Reads and blocks from the underlying file until at least one event can be
// parsed. Returns the number of events read and any read error.
//
// NOTE this is a lower level method, most users should use term.Run() instead.
func (term *Terminal) ReadEvents(evs []Event) (n int, _ error) {
	n = term.decodeEvents(evs)
	for n == 0 {
		_, err := term.readMore(minRead)
		n = term.decodeEvents(evs)
		if err == io.EOF && n > 0 && n == len(evs) {
			return n, nil
		} else if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (term *Terminal) decodeEvents(evs []Event) int {
	i := 0
	for i < len(evs) {
		buf := term.inbuf.Bytes()
		if len(buf) == 0 {
			break
		}
		kev, n := term.keyDecoder.Decode(buf)
		if n == 0 {
			break
		}
		term.inbuf.Next(n)
		ev := Event{Type: EventKey, Event: kev}
		if kev.Key.IsMouse() {
			ev.Type = EventMouse
		}
		evs[i] = ev
		i++
	}
	return i
}

func (term *Terminal) readMore(n int) (int, error) {
	if term.inerr != nil {
		return 0, term.inerr
	}
	term.inbuf.Grow(n)
	buf := term.inbuf.Bytes()
	buf = buf[len(buf):cap(buf)]
	n, term.inerr = term.in.Read(buf)
	if n > 0 {
		_, _ = term.inbuf.Write(buf[:n])
	}
	return n, term.inerr
}

// DecodeSignal decodes an os Signal into either an Event or an error; maps
// SIGTERM to ErrTerm.
func DecodeSignal(sig os.Signal) (Event, error) {
	var ev Event
	switch sig {
	case syscall.SIGTERM:
		return Event{}, ErrTerm
	case syscall.SIGINT:
		ev.Type = EventInterrupt
	case syscall.SIGWINCH:
		ev.Type = EventResize
	default:
		ev.Type = EventSignal
		ev.Signal = sig
	}
	return ev, nil
}
