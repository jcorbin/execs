package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"unicode/utf8"

	"github.com/jcorbin/execs/internal/ansi"
	"github.com/jcorbin/execs/internal/terminal"
)

func setLogOutput(w io.Writer) func() {
	log.SetOutput(w)
	return func() { log.SetOutput(os.Stderr) }
}

type delimWriter struct {
	delim []byte
	tmp   bytes.Buffer
	io.Writer
}

func (dw *delimWriter) Write(p []byte) (n int, err error) {
	dw.tmp.Reset()
	_, _ = dw.tmp.Write(p)
	_, _ = dw.tmp.Write(dw.delim)
	m, err := dw.tmp.WriteTo(dw.Writer)
	n += int(m)
	return n, err
}

type decMode int
type decModes []decMode
type decSeqStr struct {
	n byte
	a string
}

func (dm decMode) Set() decSeqStr   { return decSeqStr{'h', strconv.Itoa(int(dm))} }
func (dm decMode) Reset() decSeqStr { return decSeqStr{'l', strconv.Itoa(int(dm))} }
func (ds decSeqStr) String() string { return fmt.Sprintf("\x1b[?%s%s", ds.a, string(ds.n)) }

func (dms decModes) SetString() string {
	var buf bytes.Buffer
	buf.Grow(2 * 8 * len(dms))
	for i := range dms {
		_, _ = buf.WriteString(dms[i].Set().String())
	}
	return buf.String()
}

func (dms decModes) ResetString() string {
	var buf bytes.Buffer
	buf.Grow(2 * 8 * len(dms))
	for i := range dms {
		_, _ = buf.WriteString(dms[i].Reset().String())
	}
	return buf.String()
}

var errInt = errors.New("interrupt")

var attr = terminal.Attr{File: os.Stdout}

func run() (rerr error) {
	flog, err := os.Create("log")
	if err != nil {
		return err
	}
	defer flog.Close()

	attr.SetRaw(true)
	defer attr.Deactivate()
	if err := attr.Activate(); err != nil {
		return err
	}

	defer setLogOutput(io.MultiWriter(flog, &delimWriter{
		delim:  []byte("\r"),
		Writer: os.Stderr,
	}))()

	const (
		// See http://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
		modeX10Mouse            decMode = 9
		modeVt200Mouse          decMode = 1000
		modeVt200HighlightMouse decMode = 1001
		modeBtnEventMouse       decMode = 1002
		modeAnyEventMouse       decMode = 1003
		modeFocusEventMouse     decMode = 1004
		modeExtModeMouse        decMode = 1005
		modeSgrExtModeMouse     decMode = 1006
		modeUrxvtExtModeMouse   decMode = 1015
		modeAlternateScroll     decMode = 1007
	)

	modes := decModes{
		modeVt200Mouse,
		modeSgrExtModeMouse,
		modeAnyEventMouse,
	}
	os.Stdout.WriteString(modes.SetString())
	defer os.Stdout.WriteString(modes.ResetString())

	buf := newReadBuffer(os.Stdin, 128)
	log.Printf("reading...")
	for buf.ReadMore() {
		if err := process(&buf.Buffer); err != nil {
			return err
		}
	}
	return buf.Err()
}

func process(buf *bytes.Buffer) error {
	for buf.Len() > 0 {
		e, a, n := ansi.DecodeEscape(buf.Bytes())
		if n > 0 {
			buf.Next(n)
		}

		// have a complete escape sequence
		if e != 0 {
			if err := handle(e, a, 0); err != nil {
				return err
			}
			continue
		}

		// try to decode a rune, maybe read more bytes to complete a
		// partial escape sequence
		switch r, n := utf8.DecodeRune(buf.Bytes()); r {
		case 0x90, 0x9B, 0x9D, 0x9E, 0x9F: // DCS, CSI, OSC, PM, APC
			return nil
		case 0x1B: // ESC
			if p := buf.Bytes(); len(p) == cap(p) {
				return nil
			}
			fallthrough
		default:
			buf.Next(n)
			if err := handle(0, nil, r); err != nil {
				return err
			}
		}
	}
	return nil
}

func handle(e ansi.Escape, a []byte, r rune) error {
	if e != 0 {
		fmt.Print(e)
		if len(a) > 0 {
			fmt.Printf(" %q", a)
		}
		fmt.Printf("\r\n")
		return nil
	}

	switch {
	// advance line on <Enter>
	case r == '':
		fmt.Printf("\r\n")

	// simulate EOF on Ctrl-D
	case r == '':
		fmt.Printf("^D\r\n")
		return io.EOF

	// stop on Ctrl-C
	case r == '':
		fmt.Printf("^C\r\n")
		return errInt

	// suspend on Ctrl-Z
	case r == '':
		fmt.Printf("^Z\r\n")
		cont := make(chan os.Signal)
		signal.Notify(cont, syscall.SIGCONT)
		attr.Deactivate()
		syscall.Kill(0, syscall.SIGTSTP)
		sig := <-cont
		attr.Activate()
		fmt.Printf("signal: %v\r\n", sig)

	// print C0 controls phonetically
	case r < 0x20, r == 0x7f:
		fmt.Printf("^%s", string(0x40^r))

	// print C1 controls mnemonically
	case 0x80 <= r && r <= 0x9f:
		fmt.Print(ansi.C1Names[r&0x1f])

	// print normal rune
	default:
		fmt.Print(string(r))
	}

	return nil
}

func main() {
	switch err := run(); err {
	case nil:
	case io.EOF, errInt:
		fmt.Println(err)
	default:
		log.Fatal(err)
	}
}

func newReadBuffer(r io.Reader, minRead int) *readBuffer {
	return &readBuffer{r: r, minRead: minRead}
}

type readBuffer struct {
	bytes.Buffer
	minRead int
	r       io.Reader
	err     error
}

func (buf *readBuffer) Err() error {
	return buf.err
}

func (buf *readBuffer) ReadMore() bool {
	if buf.err != nil {
		return false
	}
	for {
		buf.Grow(buf.minRead)
		p := buf.Bytes()
		p = p[len(p):cap(p)]
		n, err := buf.r.Read(p)
		if err != nil {
			buf.err = err
		} else if n == 0 {
			continue
		}
		_, _ = buf.Write(p[:n])
		return true
	}
}
