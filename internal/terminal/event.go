package terminal

import (
	"fmt"
	"image"
	"os"
)

// TODO event stolen from termbox; reconcile with tcell

type (
	// EventType is the type of a terminal input event.
	EventType uint8

	// Event is a terminal input event, either read from the input file, or
	// delivered by a relevant signal.
	Event struct {
		Type EventType // one of Event* constants

		Mod Modifier // one of Mod* constants or 0
		Key Key      // one of Key* constants, invalid if 'Ch' is not 0
		Ch  rune     // a unicode character

		Mouse  image.Point // EventMouse
		Signal os.Signal   // EventSignal
	}
)

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
		if ev.Key != 0 {
			return fmt.Sprintf("KeyEvent{Mod:%v, Key:%v}", ev.Mod, ev.Key)
		}
		return fmt.Sprintf("KeyEvent{Mod:%v, Ch:%q}", ev.Mod, ev.Ch)
	case EventMouse:
		return fmt.Sprintf("MouseEvent{Mod:%v, Key:%v, Point:%v}", ev.Mod, ev.Key, ev.Mouse)
	case EventEOF:
		return "EOFEvent"
	case EventResize:
		return "ResizeEvent"
	case EventSignal:
		return fmt.Sprintf("SignalEvent(%v)", ev.Signal.String())
	case EventInterrupt:
		return "InterruptEvent"
	default:
		return fmt.Sprintf("UserEvent<%d>", ev.Type)
	}
}
