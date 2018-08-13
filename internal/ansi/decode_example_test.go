package ansi_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"unicode/utf8"

	"github.com/jcorbin/execs/internal/ansi"
	"github.com/jcorbin/execs/internal/terminal"
)

var errInt = errors.New("interrupt")

var attr = terminal.Attr{File: os.Stdout}

func Example_main() {
	switch err := run(); err {
	case nil:
	case io.EOF, errInt:
		fmt.Println(err)
	default:
		log.Fatal(err)
	}
}

func run() error {
	// put the terminal into raw mode
	attr := terminal.Attr{File: os.Stdout}
	attr.SetRaw(true)
	defer attr.Deactivate()
	if err := attr.Activate(); err != nil {
		return err
	}

	const minRead = 128
	var buf bytes.Buffer

	for {
		// read more input...
		buf.Grow(minRead)
		p := buf.Bytes()
		p = p[len(p):cap(p)]
		n, err := os.Stdin.Read(p)
		if err != nil {
			panic(err)
		}
		if n == 0 {
			continue
		}
		_, _ = buf.Write(p[:n])
		// ...and process it
		if err := process(&buf); err != nil {
			return err
		}
	}
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
