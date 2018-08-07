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
// - synthesize SIGINT into EventInterrupt
// - synthesize SIGTERM into ErrTerm
// - synthesize SIGWINCH into EventResize
// - TODO stop when Ctrl-C is pressed
// - TODO redraw when Ctrl-L is pressed
// - suspend when Ctrl-Z is pressed
var StandardApp = Options(
	HandleSIGINT,
	HandleSIGTERM,
	HandleSIGWINCH,
	// TODO eventFilter for KeyCtrlC
	// TODO eventFilter for KeyCtrlL
	SuspendOn(KeyCtrlZ),
	RawMode,
	HiddenCursor,
)

// TODO HandleSIGCONT ?

// HandleSIGWINCH by turning it EventResize.
var HandleSIGWINCH Option = signalHandler{
	signal: syscall.SIGWINCH,
	handle: func(term *Terminal, ev Event) (Event, error) {
		return Event{Type: EventResize}, nil
	},
}

// HandleSIGINT by turning it EventInterrupt.
var HandleSIGINT Option = signalHandler{
	signal: syscall.SIGINT,
	handle: func(term *Terminal, ev Event) (Event, error) {
		return Event{Type: EventInterrupt}, nil
	},
}

// HandleSIGTERM by turning it ErrTerm.
var HandleSIGTERM Option = signalHandler{
	signal: syscall.SIGTERM,
	handle: func(term *Terminal, ev Event) (Event, error) {
		return Event{}, ErrTerm
	},
}

type signalHandler struct {
	active bool
	signal os.Signal
	handle func(term *Terminal, ev Event) (Event, error)
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
	if ev.Type == EventSignal && ev.Signal == sh.signal {
		return sh.handle(term, ev)
	}
	return ev, nil
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
	// TODO Synthesize SIGCONT into an EventRedraw
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
	log.Printf("viva la %v", ev)
	if ev.Type == EventKey {
		for i := range sus.keys {
			if ev.Key == sus.keys[i] {
				if err := term.Suspend(); err != nil {
					return ev, err
				}
				return Event{}, nil
			}
		}
	}
	return ev, nil
}
