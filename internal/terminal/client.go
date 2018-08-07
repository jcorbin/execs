package terminal

import (
	"errors"
	"runtime"
	"time"
)

// Signaling errors.
var (
	ErrTerm = errors.New("terminate")
	ErrStop = errors.New("stop")
)

// Client of a Terminal that get ran by polling for events one at a time, and
// is expected to handle the given event, calling term.Write* to build output.
type Client interface {
	Draw(term *Terminal, ev Event) error
}

// DrawFunc is a convenient way to implement Drawable to call Run().
type DrawFunc func(term *Terminal, ev Event) error

// Draw calls the aliased function.
func (f DrawFunc) Draw(term *Terminal, ev Event) error { return f(term, ev) }

// BatchClient of a Terminal that handles batches of events, drawing output
// similarly to Client.
type BatchClient interface {
	Client
	DrawBatch(term *Terminal, ev ...Event) error
}

// Run the given Client under the terminal with options.
func (term *Terminal) Run(client Client, copts ...ClientOption) error {
	defer func(wo writeObserver) {
		if term.writeObserver != wo {
			term.setWriteOption(wo)
		}
	}(term.writeObserver)
	// TODO other forms of state restore? maybe this should be a sub-terminal instead?
	cr := clientRunner{
		eventBatchSize: 128,
	}
	cr.apply(term, copts...)
	return cr.run(term, client)
}

// ClientOption is an opaque option customizing Terminal.Run().
type ClientOption interface {
	apply(term *Terminal, cr *clientRunner)
}

type coptFunc func(term *Terminal, cr *clientRunner)

func (f coptFunc) apply(term *Terminal, cr *clientRunner) { f(term, cr) }

type clientRunner struct {
	flushAfter     FlushAfter
	frameTicker    *time.Ticker
	eventBatchSize int
}

// ClientFlushEvery sets a delay to automatically flush output, which
// immediately at the top of the client run loop. See FlushAfter.
func ClientFlushEvery(d time.Duration) ClientOption {
	return coptFunc(func(term *Terminal, cr *clientRunner) {
		cr.flushAfter.Duration = d
		if cr.frameTicker != nil {
			cr.frameTicker.Stop()
			cr.frameTicker = time.NewTicker(d)
		}
		term.setWriteOption(&cr.flushAfter)
	})
}

// ClientDrawTicker sets up a ticker that will deliver nil events every flush
// duration, which defaults to time.Second/60 if none has been given yet.
// ClientFlushEvery may also be specified to customize the interval.
var ClientDrawTicker ClientOption = coptFunc(func(term *Terminal, cr *clientRunner) {
	if cr.flushAfter.Duration == 0 {
		cr.flushAfter.Duration = time.Second / 60
		term.setWriteOption(&cr.flushAfter)
	}
	if cr.frameTicker != nil {
		cr.frameTicker.Stop()
	}
	cr.frameTicker = time.NewTicker(cr.flushAfter.Duration)
})

// ClientEventBatchSize sets the client event batch size, this controls:
// - the size of the event backlog when reading one event at a time
// - the batch size when reading batches of events
// - the size of the event backlog for out-of-band events
//
// Defaults to 128 events.
func ClientEventBatchSize(n int) ClientOption {
	return coptFunc(func(term *Terminal, cr *clientRunner) {
		cr.eventBatchSize = n
	})
}

func (cr *clientRunner) apply(term *Terminal, copts ...ClientOption) {
	for _, copt := range copts {
		copt.apply(term, cr)
	}
}

func (cr *clientRunner) run(term *Terminal, client Client) error {
	if batchClient, ok := client.(BatchClient); ok {
		return cr.runBatchClient(term, batchClient)
	}
	return cr.runClient(term, client)
}

func (cr *clientRunner) runClient(term *Terminal, client Client) error {
	var (
		events = make(chan Event, cr.eventBatchSize)
		stop   = make(chan struct{})
		errs   = make(chan error, 1)
	)

	go term.synthesize(events, errs, stop)
	go term.monitorEvents(events, errs, stop)
	defer func() { close(stop) }()

	err := cr.redraw(term, client)
	for err == nil {
		select {
		case err = <-errs:
		case <-cr.frameTicker.C:
			err = cr.redraw(term, client)
		case ev := <-events:
			err = cr.draw(term, client, ev)
		}
	}
	if err == ErrStop || err == ErrTerm {
		err = nil
	}
	return err
}

func (cr *clientRunner) runBatchClient(term *Terminal, client BatchClient) error {
	var (
		events  = make(chan Event, cr.eventBatchSize)
		batches = make(chan []Event, 1)
		free    = make(chan []Event, 1)
		stop    = make(chan struct{})
		errs    = make(chan error, 1)
	)

	free <- make([]Event, 0, cr.eventBatchSize)
	go term.synthesize(events, errs, stop)
	go term.monitorEventBatches(batches, free, errs, stop)
	defer func() { close(stop) }()

	last := make([]Event, 0, cr.eventBatchSize) // TODO evaluate usefulness
	err := cr.redraw(term, client)
	for err == nil {
		select {
		case err = <-errs:
		case ev := <-events:
			err = cr.draw(term, client, ev)
		case <-cr.frameTicker.C:
			err = cr.redraw(term, client)
		case evs := <-batches:
			if last == nil {
				err = cr.drawBatch(term, client, evs)
				free <- evs
			} else {
				free <- last
				last, err = evs, cr.drawBatch(term, client, evs)
			}
		}
	}
	if err == ErrStop || err == ErrTerm {
		err = nil
	}
	return err
}

func (cr *clientRunner) redraw(term *Terminal, client Client) error {
	cr.flushAfter.Lock()
	defer cr.flushAfter.Unlock()
	if !cr.flushAfter.set {
		return cr.lockedDraw(term, client, Event{})
	}
	return nil
}

func (cr *clientRunner) draw(term *Terminal, client Client, ev Event) error {
	cr.flushAfter.Lock()
	defer cr.flushAfter.Unlock()
	return cr.lockedDraw(term, client, ev)
}

func (cr *clientRunner) drawBatch(term *Terminal, client BatchClient, evs []Event) error {
	cr.flushAfter.Lock()
	defer cr.flushAfter.Unlock()
	return cr.lockedDrawBatch(term, client, evs)
}

func (cr *clientRunner) lockedDraw(term *Terminal, client Client, ev Event) error {
	err := term.Discard()
	if err == nil {
		err = client.Draw(term, ev)
	}
	return err
}

func (cr *clientRunner) lockedDrawBatch(term *Terminal, client BatchClient, evs []Event) error {
	err := term.Discard()
	if err == nil {
		err = client.DrawBatch(term, evs...)
	}
	return err
}

// synthesize signals into special events.
func (term *Terminal) synthesize(events chan<- Event, errs chan<- error, stop <-chan struct{}) {
	runtime.LockOSThread() // dedicate this thread to signal processing
	defer term.closeOnPanic()
	for {
		select {
		case <-stop:
			return
		case sig := <-term.signals:
			if ev, err := term.DecodeSignal(sig); err != nil {
				errs <- err
				if err == ErrTerm {
					return
				}
			} else if ev.Type != EventNone {
				select {
				case events <- ev:
				default:
				}
			}
		}
	}
}

func (term *Terminal) monitorEvents(
	events chan<- Event,
	errs chan<- error,
	stop <-chan struct{},
) {
	runtime.LockOSThread() // dedicate this thread to event reading
	defer term.closeOnPanic()
	for {
		ev, err := term.DecodeEvent()
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

func (term *Terminal) monitorEventBatches(
	batches chan<- []Event, free <-chan []Event,
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
		n, err := term.DecodeEvents(evs)
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
