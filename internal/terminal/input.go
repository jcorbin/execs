package terminal

import (
	"fmt"
	"io"
	"runtime"
	"syscall"
)

// synthesize signals into special events.
func (term *Terminal) synthesize(events chan<- Event, errs chan<- error, stop <-chan struct{}) {
	runtime.LockOSThread() // dedicate this thread to signal processing
	defer term.closeOnPanic()
	for {
		select {
		case <-stop:
			return
		case sig := <-term.signals:
			var ev Event
			switch sig {
			case syscall.SIGTERM:
				errs <- ErrTerm
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

func (term *Terminal) readEvents(events chan<- Event, errs chan<- error, stop <-chan struct{}) {
	runtime.LockOSThread() // dedicate this thread to event reading
	defer term.closeOnPanic()
	for {
		ev, err := term.ReadEvent()
		if err != nil {
			select {
			case errs <- err:
			case <-stop:
				return
			}
			return
		}
		select {
		case events <- ev:
		case <-stop:
			return
		}
	}
}

func (term *Terminal) readEventBatches(
	batches chan<- []Event,
	free <-chan []Event,
	errs chan<- error,
	stop <-chan struct{},
) {
	runtime.LockOSThread() // dedicate this thread to event reading
	defer term.closeOnPanic()
	for {
		var evs []Event
		select {
		case evs = <-free:
			evs = evs[:cap(evs)]
		case <-stop:
			return
		}
		n, err := term.ReadEvents(evs)
		if err != nil {
			select {
			case errs <- err:
			case <-stop:
			}
			return
		}
		select {
		case batches <- evs[:n]:
		case <-stop:
			return
		}
	}
}

// ReadEvent reads one event from the input file; this may happen from
// previously read / in-buffer bytes, and may not necessarily block.
//
// NOTE this is a lower level method, most users should use term.Run() instead.
func (term *Terminal) ReadEvent() (Event, error) {
	for {
		if n, ev := term.parse(); ev.Type != EventNone {
			term.parseOffset += n
			return ev, nil
		}
		if _, err := term.readMore(minRead); err != nil {
			return Event{}, err
		}
	}
}

// ReadEvents reads events into the given slice, stopping either when there are
// no more buffered inputs bytes to parse, or the given events buffer is full.
// Reads and blocks from the underlying file until at least one event can be
// parsed. Returns the number of events read and any read error.
//
// NOTE this is a lower level method, most users should use term.Run() instead.
func (term *Terminal) ReadEvents(evs []Event) (n int, _ error) {
	n = term.parseEvents(evs)
	for n == 0 {
		_, err := term.readMore(minRead)
		n = term.parseEvents(evs)
		if err == io.EOF && n > 0 && n == len(evs) {
			return n, nil
		} else if err != nil {
			return n, err
		}
	}
	return n, nil
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

func (term *Terminal) parseEvents(evs []Event) int {
	i := 0
	for i < len(evs) {
		n, ev := term.parse()
		if n == 0 {
			break
		}
		term.parseOffset += n
		evs[i] = ev
		i++
	}
	return i
}

func (term *Terminal) parse() (n int, ev Event) {
	if term.parseOffset >= term.readOffset {
		return 0, Event{}
	}
	buf := term.inbuf[term.parseOffset:term.readOffset]
	defer func() {
		if len(buf) > 16 && n == 0 {
			panic(fmt.Sprintf("FIXME broken terminal parsing; making no progress on %q", buf))
		}
	}()
	return term.parser.parse(buf)
}

/*

	// XXX historical readEventBatches
	for done := false; ; {
		for {
			var evs []Event
			select {
			case evs = <-free:
				evs = evs[:cap(evs)]
			case <-stop:
				return
			}
			n := term.parseEvents(evs)
			if n == 0 {
				break
			}
			select {
			case batches <- evs:
			case <-stop:
				return
			}
		}
		if done {
			break
		} else if _, err := term.readMore(minRead); err == io.EOF {
			done = true
		} else if err != nil {
			errs <- err
			return
		}
	}

	// XXX historical readEvents
	for done := false; ; {
		if n, ev := term.parse(); ev.Type != EventNone {
			term.parseOffset += n
			select {
			case events <- ev:
			case <-stop:
				return
			}
			continue
		}
		if done {
			break
		} else if _, err := term.readMore(minRead); err == io.EOF {
			done = true
		} else if err != nil {
			errs <- err
			return
		}
	}
	events <- Event{Type: EventEOF}


*/
