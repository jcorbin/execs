package terminal

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

// StandardApp encapsulates the expected default behavior of a standard
// fullscreen terminal application:
// - drawing to a raw mode terminal with the cursor hidden
// - synthesize SIGWINCH into ResizeEvent
// - synthesize KeyCtrlL into RedrawEvent
// - synthesize SIGINT into InterruptEvent
// - synthesize KeyCtrlC into InterruptEvent
// - synthesize SIGTERM into ErrTerm
// - suspend when Ctrl-Z is pressed
// - synthesize SIGCONT into RedrawEvent
var StandardApp = Options(
	HandleCtrlC,
	HandleCtrlL,
	HandleSIGCONT,
	// TODO HandleKey( KeyCtrlBackslash, send SIGQUIT ) ?
	// TODO alt mode buffer?
	HandleSIGINT,
	HandleSIGTERM,
	HandleSIGWINCH,
	SuspendOn(KeyCtrlZ),
	RawMode,
	HiddenCursor,
)

// HandleSIGCONT by turning it into RedrawEvent.
var HandleSIGCONT = HandleSignal(syscall.SIGCONT, func(term *Terminal, ev Event) (Event, error) {
	return Event{Type: RedrawEvent}, nil
})

// HandleSIGWINCH by turning it into ResizeEvent.
var HandleSIGWINCH Option = HandleSignal(syscall.SIGWINCH, func(term *Terminal, ev Event) (Event, error) {
	return Event{Type: ResizeEvent}, nil
})

// HandleSIGINT by turning it into InterruptEvent.
var HandleSIGINT Option = HandleSignal(syscall.SIGINT, func(term *Terminal, ev Event) (Event, error) {
	return Event{Type: InterruptEvent}, nil
})

// HandleSIGTERM by turning it into ErrTerm.
var HandleSIGTERM Option = HandleSignal(syscall.SIGTERM, func(term *Terminal, ev Event) (Event, error) {
	return Event{}, ErrTerm
})

func HandleKey(
	key Key,
	handle func(term *Terminal, ev Event) (Event, error),
) Option {
	return keyHandler{key: key, handle: handle}
}

type keyHandler struct {
	key    Key
	handle func(term *Terminal, ev Event) (Event, error)
}

func (kh keyHandler) init(term *Terminal) error {
	log.Printf("installing %v handler", kh.key)
	term.eventFilter = chainEventFilter(term.eventFilter, kh)
	return nil
}

func (kh keyHandler) filterEvent(term *Terminal, ev Event) (Event, error) {
	if ev.Type == KeyEvent && ev.Key == kh.key {
		return kh.handle(term, ev)
	}
	return Event{}, nil
}

// HandleCtrlC by by turning it into InterruptEvent.
var HandleCtrlC = HandleKey(KeyCtrlC, func(term *Terminal, ev Event) (Event, error) {
	return Event{Type: InterruptEvent}, nil
})

// HandleCtrlL by by turning it into RedrawEvent.
var HandleCtrlL = HandleKey(KeyCtrlL, func(term *Terminal, ev Event) (Event, error) {
	return Event{Type: RedrawEvent}, nil
})

// HandleSignal creates an option that a signal handling event filter during
// terminal lifecycle.
func HandleSignal(
	signal os.Signal,
	handle func(term *Terminal, ev Event) (Event, error),
) Option {
	return signalHandler{signal: signal, handle: handle}
}

type signalHandler struct {
	signal os.Signal
	handle func(term *Terminal, ev Event) (Event, error)
	active bool
}

func (sh signalHandler) init(term *Terminal) error {
	term.eventFilter = chainEventFilter(term.eventFilter, &sh)
	term.termContext = chainTermContext(term.termContext, &sh)
	return nil
}

func (sh *signalHandler) enter(term *Terminal) error {
	if !sh.active {
		sh.active = true
		signal.Notify(term.signals, sh.signal)
	}
	return nil
}

func (sh *signalHandler) exit(term *Terminal) error {
	// TODO support (optional) deregistration (e.g. when suspending)?
	return nil
}

func (sh *signalHandler) filterEvent(term *Terminal, ev Event) (Event, error) {
	if ev.Type == SignalEvent && ev.Signal == sh.signal {
		return sh.handle(term, ev)
	}
	return Event{}, nil
}

// SuspendOn creates an Option that calls Terminal.Suspend() when the specified
// key(s) are pressed. The corresponding KeyEvents are filtered out, never seen
// by the client.
func SuspendOn(keys ...Key) Option {
	return &suspendOn{keys: keys}
}

type suspendOn struct {
	keys   []Key
	active bool
}

func (sus *suspendOn) init(term *Terminal) error {
	term.eventFilter = chainEventFilter(term.eventFilter, sus)
	term.termContext = chainTermContext(term.termContext, sus)
	log.Printf("installed suspendOn")
	return nil
}

func (sus *suspendOn) enter(term *Terminal) error {
	if !sus.active {
		sus.active = true
		signal.Notify(term.signals, syscall.SIGCONT)
	}
	return nil
}

func (sus *suspendOn) exit(term *Terminal) error {
	return nil
}

func (sus suspendOn) filterEvent(term *Terminal, ev Event) (Event, error) {
	if ev.Type == SignalEvent {
		if ev.Signal == syscall.SIGCONT {
			return Event{Type: RedrawEvent}, nil
		}
		return ev, nil
	}
	if ev.Type == KeyEvent {
		for i := range sus.keys {
			if ev.Key == sus.keys[i] {
				log.Printf("suspending on %v", ev)
				if err := term.Suspend(); err != nil {
					return ev, err
				}
				return Event{}, nil
			}
		}
	}
	return ev, nil
}
