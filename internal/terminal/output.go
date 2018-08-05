package terminal

// Curse is a single cursor manipulator; NOTE the type asymmetry is due to
// complying with the shape of Cursor methods like Cursor.Show.
type Curse func(Cursor, []byte) ([]byte, Cursor)

// Swear runs a some curses, accumulating their output into an internal buffer
// for later flushing.
func (term *Terminal) Swear(curses ...Curse) {
	for i := range curses {
		term.outbuf, term.cur = curses[i](term.cur, term.outbuf)
	}
}

// Flush any buffered output.
//
// NOTE called if necessary by Core.Run() before polling for the next event,
// most users should not need to call this method directly.
func (term *Terminal) Flush() error {
	if len(term.outbuf) > 0 {
		return term.flush()
	}
	return nil
}

// Discard any un-flushed output.
func (term *Terminal) Discard() {
	term.outbuf = term.outbuf[:0]
}

func (term *Terminal) flush() error {
	_, err := term.out.Write(term.outbuf)
	term.outbuf = term.outbuf[:0]
	return err
}
