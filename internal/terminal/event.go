package terminal

import (
	"fmt"
	"image"
	"os"
	"strconv"
	"strings"

	"github.com/jcorbin/execs/internal/termkey"
)

// Event is a terminal input event, either read from the input file, or
// delivered by a relevant signal.
type Event struct {
	// TODO event stolen from termbox; reconcile with tcell

	Type EventType // one of Event* constants

	Mod Modifier // one of Mod* constants or 0
	Key Key      // one of Key* constants, invalid if 'Ch' is not 0
	Ch  rune     // a unicode character

	Mouse  image.Point // EventMouse
	Signal os.Signal   // EventSignal
}

// Modifier during a key or mouse event.
type Modifier = termkey.Modifier

// Key code during a key event.
type Key = termkey.Key

// EventType type of an Event.
type EventType uint8

// Event types.
const (
	EventNone EventType = iota
	EventKey
	EventMouse
	EventEOF
	EventResize
	EventSignal
	EventInterrupt

	FirstUserEvent
)

func (ev Event) String() string {
	switch ev.Type {
	case EventNone:
		return "NilEvent"
	case EventKey:
		return fmt.Sprintf("KeyEvent(%s)", ev.keyString())
	case EventMouse:
		return fmt.Sprintf("MouseEvent(%s)", ev.mouseString())
	case EventEOF:
		return "EOFEvent"
	case EventResize:
		return "ResizeEvent"
	case EventSignal:
		return fmt.Sprintf("SignalEvent(%v)", ev.Signal.String())
	case EventInterrupt:
		return "InterruptEvent"
	default:
		return fmt.Sprintf("UserEvent{Type:%d}", ev.Type)
	}
}

func (ev Event) keyString() string {
	var parts [4]string
	i := 0
	if ev.Mod != 0 {
		parts[i] = ev.Mod.String()
		i++
	}
	if ev.Key != 0 {
		parts[i] = ev.Key.String()
		i++
		if ev.Ch != 0 {
			parts[i] = "WITH_INVALID_CHAR"
			i++
		}
	}
	if ev.Ch != 0 {
		if strconv.IsPrint(ev.Ch) {
			parts[i] = string(ev.Ch)
		} else {
			s := strconv.QuoteRune(ev.Ch)
			parts[i] = s[1 : len(s)-1]
		}
		i++
	}
	switch i {
	case 0:
		return ""
	case 1:
		return parts[0]
	default:
		return strings.Join(parts[:i], "+")
	}
}

func (ev Event) mouseString() string {
	parts := [6]string{
		ev.keyString(), "@<",
		strconv.Itoa(ev.Mouse.X), ",",
		strconv.Itoa(ev.Mouse.Y), ">",
	}
	return strings.Join(parts[:], "")
}
