package terminal

import (
	"log"
	"unicode/utf8"
)

// Curse is a single cursor manipulator; NOTE the type asymmetry is due to
// complying with the shape of Cursor methods like Cursor.Show.
type Curse func(Cursor, []byte) ([]byte, Cursor)

// Write into the output buffer, triggering any Flush* options.
func (term *Terminal) Write(p []byte) (n int, err error) {
	if term.outerr == nil {
		term.outerr = term.writeOption.preWrite(term, len(p))
		// TODO would be nice to give writeOption a choice to pass large
		// buffers through rather than append/growing them
	}
	if term.outerr == nil {
		term.outbuf = append(term.outbuf, p...)
		n = len(p)
		term.outerr = term.writeOption.postWrite(term, n)
	}
	return n, term.outerr
}

// WriteByte into the output buffer, triggering any Flush* options.
func (term *Terminal) WriteByte(c byte) error {
	if term.outerr == nil {
		term.outerr = term.writeOption.preWrite(term, 1)
	}
	if term.outerr == nil {
		term.outbuf = append(term.outbuf, c)
		term.outerr = term.writeOption.postWrite(term, 1)
	}
	return term.outerr
}

// WriteRune into the output buffer, triggering any Flush* options.
func (term *Terminal) WriteRune(r rune) (n int, err error) {
	if term.outerr == nil {
		term.outerr = term.writeOption.preWrite(term, utf8.RuneLen(r))
	}
	if term.outerr == nil {
		var tmp [8]byte
		n = utf8.EncodeRune(tmp[:], r)
		term.outbuf = append(term.outbuf, tmp[:n]...)
		term.outerr = term.writeOption.postWrite(term, n)
	}
	return n, term.outerr
}

// WriteString into the output buffer, triggering any Flush* options.
func (term *Terminal) WriteString(s string) (n int, err error) {
	if term.outerr == nil {
		term.outerr = term.writeOption.preWrite(term, len(s))
		// TODO would be nice to give writeOption a choice to pass large
		// strings through rather than append/growing them
	}
	if term.outerr == nil {
		n = len(s)
		term.outbuf = append(term.outbuf, s...)
		term.outerr = term.writeOption.postWrite(term, n)
	}
	return n, term.outerr
}

// WriteCursor writes cursor control codes into the output buffer, and updates
// cursor state, triggering any Flush* options.
func (term *Terminal) WriteCursor(curses ...Curse) (n int, err error) {
	n, term.tmp, term.cur = writeCursor(term.cur, term.tmp[:0])
	log.Printf("write cursor %q", term.tmp)
	n, err = term.Write(term.tmp)
	return n, err
}

// Flush any buffered output.
func (term *Terminal) Flush() error {
	for term.outerr == nil && len(term.outbuf) > 0 {
		n, err := term.out.Write(term.outbuf)
		term.outbuf = term.outbuf[:copy(term.outbuf, term.outbuf[n:])]
		term.outerr = err
	}
	return term.outerr
}

// Discard any un-flushed output.
func (term *Terminal) Discard() error {
	if term.outerr == nil {
		term.outbuf = term.outbuf[:0]
		term.outerr = term.writeOption.preWrite(term, 0)
	}
	return term.outerr
}

func writeCursor(cur Cursor, buf []byte, curses ...Curse) (n int, _ []byte, _ Cursor) {
	for i := range curses {
		m := len(buf)
		buf, cur = curses[i](cur, buf)
		n += len(buf) - m
	}
	return n, buf, cur
}
