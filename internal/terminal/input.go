package terminal

import (
	"bytes"
	"io"
	"os"
	"syscall"

	"github.com/jcorbin/execs/internal/termkey"
)

// DecodeSignal decodes an os Signal into an Event.
func DecodeSignal(sig os.Signal) Event {
	switch sig {
	case syscall.SIGINT:
		return Event{Type: InterruptEvent}
	case syscall.SIGWINCH:
		return Event{Type: ResizeEvent}
	default:
		return Event{Type: SignalEvent, Signal: sig}
	}
}

// Decoder decodes terminal input, whether read in-band, or signaled
// out-of-band.
//
// NOTE this is a mid level object, while there may be some use for it,
// most users would be better served by Terminal.Run().
type Decoder struct {
	eventFilter
	in         *os.File
	buf        bytes.Buffer
	err        error
	keyDecoder *termkey.Decoder
}

// Err returns any read error encountered so far; if this is non-nill, all
// future reads will fail, and the decoder is dead.
func (dec *Decoder) Err() error { return dec.err }

// DecodeSignal decodes an os.Signal into an Event, passing it through any
// event filter(s) first.
func (dec *Decoder) DecodeSignal(sig os.Signal) (Event, error) {
	ev := DecodeSignal(sig)
	if ev.Type == NoEvent {
		return Event{}, nil
	}
	return dec.filterEvent(ev)
}

// DecodeEvent reads an event with ReadEvent, and then passes it through any
// event filter(s).
//
// If the filter masks the event (returns EventNone with nil error), then
// DecodeEvent loops and reads another event.
//
// Returns the first unfiltered event read and any error.
func (dec *Decoder) DecodeEvent() (Event, error) {
	var tmp [1]Event
	for {
		var ev Event
		n, err := dec.ReadEvents(tmp[:])
		if n > 0 {
			ev = tmp[0]
		}
		if err == nil {
			ev, err = dec.filterEvent(ev)
		}
		if err != nil || ev.Type != NoEvent {
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
func (dec *Decoder) DecodeEvents(evs []Event) (n int, _ error) {
	for {
		n, err := dec.ReadEvents(evs)
		if err != nil {
			return n, err
		}

		// filter events
		j := 0
		for i := 0; i < n; i++ {
			ev, err := dec.filterEvent(evs[i])
			if ev.Type != NoEvent {
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
// NOTE this is a low level method, most users should use dec.DecodeEvent() instead.
func (dec *Decoder) ReadEvent() (Event, error) {
	var tmp [1]Event
	n, err := dec.ReadEvents(tmp[:])
	var ev Event
	if n > 0 {
		ev = tmp[0]
	}
	if err == nil {
		ev, err = dec.filterEvent(ev)
	}
	return ev, err
}

// ReadEvents reads events into the given slice, stopping either when there are
// no more buffered inputs bytes to parse, or the given events buffer is full.
// Reads and blocks from the underlying file until at least one event can be
// parsed. Returns the number of events read and any read error.
func (dec *Decoder) ReadEvents(evs []Event) (n int, _ error) {
	n = dec.decodeEvents(evs)
	for n == 0 {
		_, err := dec.readMore(minRead)
		n = dec.decodeEvents(evs)
		if err == io.EOF && n > 0 && n == len(evs) {
			return n, nil
		} else if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (dec *Decoder) decodeEvents(evs []Event) int {
	i := 0
	for i < len(evs) {
		buf := dec.buf.Bytes()
		if len(buf) == 0 {
			break
		}
		kev, n := dec.keyDecoder.Decode(buf)
		if n == 0 {
			break
		}
		dec.buf.Next(n)
		ev := Event{Type: KeyEvent, Event: kev}
		if kev.Key.IsMouse() {
			ev.Type = MouseEvent
		}
		evs[i] = ev
		i++
	}
	return i
}

func (dec *Decoder) readMore(n int) (int, error) {
	if dec.err != nil {
		return 0, dec.err
	}
	dec.buf.Grow(n)
	buf := dec.buf.Bytes()
	buf = buf[len(buf):cap(buf)]
	n, dec.err = dec.in.Read(buf)
	if n > 0 {
		_, _ = dec.buf.Write(buf[:n])
	}
	return n, dec.err
}
