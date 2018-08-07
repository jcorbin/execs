package terminal

import (
	"unicode/utf8"
)

// Curse is a single cursor manipulator; NOTE the type asymmetry is due to
// complying with the shape of Cursor methods like Cursor.Show.
type Curse func(Cursor, []byte) ([]byte, Cursor)

// Write into the output buffer, triggering any Flush* options.
func (term *Terminal) Write(p []byte) (n int, err error) {
	if term.outerr == nil {
		term.outerr = term.writeObserver.preWrite(term, len(p))
		// TODO would be nice to give writeOption a choice to pass large
		// buffers through rather than append/growing them
	}
	if term.outerr == nil {
		n, _ = term.outbuf.Write(p)
		term.outerr = term.writeObserver.postWrite(term, n)
	}
	return n, term.outerr
}

// WriteByte into the output buffer, triggering any Flush* options.
func (term *Terminal) WriteByte(c byte) error {
	if term.outerr == nil {
		term.outerr = term.writeObserver.preWrite(term, 1)
	}
	if term.outerr == nil {
		_ = term.outbuf.WriteByte(c)
		term.outerr = term.writeObserver.postWrite(term, 1)
	}
	return term.outerr
}

// WriteRune into the output buffer, triggering any Flush* options.
func (term *Terminal) WriteRune(r rune) (n int, err error) {
	if term.outerr == nil {
		term.outerr = term.writeObserver.preWrite(term, utf8.RuneLen(r))
	}
	if term.outerr == nil {
		n, _ = term.outbuf.WriteRune(r)
		term.outerr = term.writeObserver.postWrite(term, n)
	}
	return n, term.outerr
}

// WriteString into the output buffer, triggering any Flush* options.
func (term *Terminal) WriteString(s string) (n int, err error) {
	if term.outerr == nil {
		term.outerr = term.writeObserver.preWrite(term, len(s))
		// TODO would be nice to give writeOption a choice to pass large
		// strings through rather than append/growing them
	}
	if term.outerr == nil {
		n, _ = term.outbuf.WriteString(s)
		term.outerr = term.writeObserver.postWrite(term, n)
	}
	return n, term.outerr
}

// WriteCursor writes cursor control codes into the output buffer, and updates
// cursor state, triggering any Flush* options.
func (term *Terminal) WriteCursor(curses ...Curse) (n int, err error) {
	switch len(curses) {
	case 0:
		return 0, nil
	case 1:
		_, term.tmp, term.cur = writeCursor(term.cur, term.tmp[:0], curses[0])
	default:
		term.tmp = term.tmp[:0]
		for i := range curses {
			_, term.tmp, term.cur = writeCursor(term.cur, term.tmp, curses[i])
		}
	}
	return term.Write(term.tmp)
}

// Flush any buffered output.
func (term *Terminal) Flush() error {
	if term.outerr == nil && term.outbuf.Len() > 0 {
		_, term.outerr = term.outbuf.WriteTo(term.out)
	}
	return term.outerr
}

// Discard any un-flushed output.
func (term *Terminal) Discard() error {
	if term.outerr == nil {
		term.outbuf.Reset()
		term.outerr = term.writeObserver.preWrite(term, 0)
	}
	return term.outerr
}

func writeCursor(cur Cursor, buf []byte, curse Curse) (n int, _ []byte, _ Cursor) {
	m := len(buf)
	buf, cur = curse(cur, buf)
	n += len(buf) - m
	return n, buf, cur
}
