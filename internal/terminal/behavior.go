package terminal

import (
	"errors"
	"os"
	"os/signal"
	"syscall"
)

// SuspendOn creates a SuspendOption triggered by the given terminal key(s).
// For example the usual semantic can be had with SuspendOn(KeyCtrlZ).
func SuspendOn(keys ...Key) SuspendOption {
	return SuspendOption{on: keys}
}

// SuspendOption supports the usual terminal suspension behavior.
type SuspendOption struct {
	on   []Key
	term *Terminal
}

func (sus *SuspendOption) init(term *Terminal) error {
	// TODO logic to prevent incorrect usage like if term.out == os.Stdout { }
	term.eventFilter = chainEventFilter(term.eventFilter, sus)
	term.termContext = chainTermContext(term.termContext, sus)
	return nil
}

func (sus *SuspendOption) enter(term *Terminal) error {
	if sus.term == nil {
		sus.term = term
		// TODO Synthesize SIGCONT into an EventRedraw
		signal.Notify(term.signals, syscall.SIGCONT)
	} else if term != sus.term {
		return errors.New("SuspendOption cannot be shared between Terminals")
	}
	return nil
}

func (sus *SuspendOption) exit(term *Terminal) error {
	return nil
}

// Suspend the terminal program.
//
// Will be called when any key given to SuspendOn is seen; user may call this
// to suspend otherwise.
func (sus SuspendOption) Suspend() error {
	if sus.term == nil {
		return errors.New("SuspendOption not attached to any Terminal")
	}
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	if err := sus.term.termContext.exit(sus.term); err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGSTOP); err != nil {
		return err
	}
	return sus.term.termContext.enter(sus.term)
	// TODO re-setup in SIGCONT handler instead?
}

func (sus SuspendOption) filterEvent(term *Terminal, ev Event) (Event, error) {
	if ev.Type == EventKey {
		for i := range sus.on {
			if ev.Key == sus.on[i] {
				if err := sus.Suspend(); err != nil {
					return ev, err
				}
				return Event{}, nil
			}
		}
	}
	return ev, nil
}
