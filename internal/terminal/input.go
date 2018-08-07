package terminal

import (
	"io"
	"os"
	"syscall"
)

// DecodeSignal decodes an os Signal into an Event.
//
// NOTE this is a mid level method, while there may be some use for it,
// term.Run() should suffice for most needs.
func DecodeSignal(sig os.Signal) Event {
	switch sig {
	case syscall.SIGINT:
		return Event{Type: EventInterrupt}
	case syscall.SIGWINCH:
		return Event{Type: EventResize}
	default:
		return Event{Type: EventSignal, Signal: sig}
	}
}

// DecodeSignal decodes an os.Signal into an Event, passing it through any
// event filter(s) first.
//
// NOTE this is a mid level method, while there may be some use for it,
// term.Run() should suffice for most needs.
func (term *Terminal) DecodeSignal(sig os.Signal) (Event, error) {
	ev := DecodeSignal(sig)
	if ev.Type == EventNone {
		return Event{}, nil
	}
	return term.filterEvent(term, ev)
}

// DecodeEvent reads an event with ReadEvent, and then passes it through any
// event filter(s).
//
// If the filter masks the event (returns EventNone with nil error), then
// DecodeEvent loops and reads another event.
//
// Returns the first unfiltered event read and any error.
//
// NOTE this is a mid level method, while there may be some use for it,
// term.Run() should suffice for most needs.
func (term *Terminal) DecodeEvent() (Event, error) {
	var tmp [1]Event
	for {
		var ev Event
		n, err := term.ReadEvents(tmp[:])
		if n > 0 {
			ev = tmp[0]
		}
		if err == nil {
			ev, err = term.filterEvent(term, ev)
		}
		if err != nil || ev.Type != EventNone {
			return ev, err
		}
	}
}

// DecodeEvents reads an event batch with ReadEvents, and then applies any
// event filter(s) to the batch. Any events filtered to EventNone are elided
// from the batch. If a filter returns non-nil error, event filtering stops.
//
// Loops until at least one unfiltered event has been read, and so may do more
// than one round of blocking IO.
//
// Returns the number of unfiltered events and any error.
//
// NOTE this is a mid level method, while there may be some use for it,
// term.Run() should suffice for most needs.
func (term *Terminal) DecodeEvents(evs []Event) (n int, _ error) {
	for {
		n, err := term.ReadEvents(evs)
		if err != nil {
			return n, err
		}

		// filter events
		j := 0
		for i := 0; i < n; i++ {
			ev, err := term.filterEvent(term, evs[i])
			if ev.Type != EventNone {
				evs[j] = ev
				j++
			}
			if err != nil {
				for i++; i < n; i++ {
					evs[j] = evs[i]
					j++
				}
				return j, err
			}
		}

		// return only if any unfiltered
		if j > 0 {
			return j, nil
		}
	}
}

// ReadEvent reads one event from the input file; this may happen from
// previously read / in-buffer bytes, and may not necessarily block.
//
// The event is first passed through any event filter(s), returning their final
// Event/error pair. NOTE this means that it's possible for ReadEvent to return
// EventNone and nil error if the event was masked by a filter.
//
// NOTE this is a lower level method, most users should use term.Run() instead.
func (term *Terminal) ReadEvent() (Event, error) {
	var tmp [1]Event
	n, err := term.ReadEvents(tmp[:])
	var ev Event
	if n > 0 {
		ev = tmp[0]
	}
	if err == nil {
		ev, err = term.filterEvent(term, ev)
	}
	return ev, err
}

// ReadEvents reads events into the given slice, stopping either when there are
// no more buffered inputs bytes to parse, or the given events buffer is full.
// Reads and blocks from the underlying file until at least one event can be
// parsed. Returns the number of events read and any read error.
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
