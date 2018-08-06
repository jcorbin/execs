package terminal

import "io"

// ReadEvent reads one event from the input file; this may happen from
// previously read / in-buffer bytes, and may not necessarily block.
//
// NOTE this is a lower level method, most users should use term.Run() instead.
func (term *Terminal) ReadEvent() (Event, error) {
	for {
		if n, ev := term.decodeEvent(); ev.Type != EventNone {
			term.parseOffset += n
			return ev, nil
		}
		if _, err := term.readMore(minRead); err != nil {
			return Event{}, err
		}
	}
}

func (term *Terminal) decodeEvent() (n int, ev Event) {
	if term.parseOffset >= term.readOffset {
		return 0, Event{}
	}
	buf := term.inbuf[term.parseOffset:term.readOffset]
	ev.Event, n = term.keyDecoder.Decode(buf)
	if n == 0 {
		return 0, Event{}
	}
	if ev.Key.IsMouse() {
		ev.Type = EventMouse
	} else {
		ev.Type = EventKey
	}
	return n, ev
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
		n, ev := term.decodeEvent()
		if n == 0 {
			break
		}
		term.parseOffset += n
		evs[i] = ev
		i++
	}
	return i
}

func (term *Terminal) readMore(n int) (int, error) {
	if term.inerr != nil {
		return 0, term.inerr
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
	n, term.inerr = term.in.Read(term.inbuf[term.readOffset:])
	term.readOffset += n
	return n, nil
}
