package terminal

import (
	"time"
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
	defer func(wo writeOption) {
		if term.writeOption != wo {
			term.setWriteOption(wo)
		}
	}(term.writeOption)
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
	eventBatchSize int
}

// ClientFlushEvery sets a delay to automatically flush output, which
// immediately at the top of the client run loop. See FlushAfter.
func ClientFlushEvery(d time.Duration) ClientOption {
	return coptFunc(func(term *Terminal, cr *clientRunner) {
		cr.flushAfter.Duration = d
		term.setWriteOption(&cr.flushAfter)
	})
}

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

	go term.synthesize(events, stop)
	go term.readEvents(events, errs, stop)
	defer func() { close(stop) }()

	err := cr.draw(term, client, Event{})
	for err == nil {
		select {
		case err = <-errs:
		case ev := <-events:
			err = cr.draw(term, client, ev)
		}
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
	go term.synthesize(events, stop)
	go term.readEventBatches(batches, free, errs, stop)
	defer func() { close(stop) }()

	last := make([]Event, 0, cr.eventBatchSize) // TODO evaluate usefulness
	err := cr.drawBatch(term, client, nil)
	for err == nil {
		select {
		case err = <-errs:
		case ev := <-events:
			err = cr.draw(term, client, ev)
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
	return err
}

func (cr *clientRunner) draw(term *Terminal, client Client, ev Event) error {
	cr.flushAfter.Lock()
	defer cr.flushAfter.Unlock()
	term.Discard()
	return client.Draw(term, ev)
}

func (cr *clientRunner) drawBatch(term *Terminal, client BatchClient, evs []Event) error {
	cr.flushAfter.Lock()
	defer cr.flushAfter.Unlock()
	term.Discard()
	return client.DrawBatch(term, evs...)
}
