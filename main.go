package main

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/jcorbin/execs/internal/cops/display"
	"github.com/jcorbin/execs/internal/terminal"
)

func setLogOutput(w io.Writer) func() {
	log.SetOutput(w)
	return func() { log.SetOutput(os.Stderr) }
}

var logBuf bytes.Buffer

/*

var (
	errQuitGame = errors.New("quit game")
	errGameOver = errors.New("game over")
)

	ui.next = 1

	if ke, _, ok := ui.KeyPressed(); ok && ke == termbox.KeyEsc {
		return errQuitGame
	}

	mid := ui.Size.Div(2)

	w := 5
	box := image.Rectangle{mid.Sub(image.Pt(w, 1)), mid.Add(image.Pt(w, 0))}
	drawLabel(ui, box, "Who Are You?")

	box = box.Add(image.Pt(0, 1))
	if s, changed, submitted := doTextEdit(
		ui,
		box,
		g.name,
	); changed {
		g.name = s
	} else if submitted {
		log.Printf("hello %q", s)
		g.name = s[:0]
	}

*/

type ui struct {
	*terminal.Terminal
	size image.Point
}

func (it *ui) init(term *terminal.Terminal) {
	it.Terminal = term
	it.size, _ = term.Size()
}

func (it *ui) header(label string, args ...interface{}) {
	if len(args) > 1 {
		label = fmt.Sprintf(label, args...)
	}
	w := utf8.RuneCountInString(label)
	it.WriteByte('|')
	if max := it.size.X - 2; w < max {
		it.WriteString(label)
		it.WriteString(strings.Repeat("-", max-w))
	} else {
		it.WriteString(label[:max])
	}
	it.WriteByte('|')
}

func draw(term *terminal.Terminal, ev terminal.Event) error {
	if ev.Type == terminal.EventKey && ev.Key == terminal.KeyCtrlC {
		return terminal.ErrStop
	}

	if _, err := term.WriteCursor(display.Cursor.Home, display.Cursor.Clear); err != nil {
		return err
	}

	var it ui
	it.init(term)
	if ev.Type == terminal.EventResize {
		log.Printf("size is %v", it.size)
	}

	buf := logBuf.Bytes()
	totalLines := bytes.Count(buf, []byte("\n"))
	numLines := totalLines
	if maxLines := it.size.Y - 1; numLines > maxLines {
		numLines = maxLines
	}

	sc := bufio.NewScanner(bytes.NewReader(buf))
	if numLines < totalLines {
		it.header(" Logs (last %v of %v) ", numLines, totalLines)
		for i := numLines; i < totalLines; i++ {
			sc.Scan()
		}
	} else {
		it.header(" Logs ")
	}
	for sc.Scan() {
		term.WriteString("\r\n| ")
		b := sc.Bytes()
		if w, max := utf8.RuneCount(b), it.size.X-2; w > max {
			drop := 3 + w - max
			for i := 0; i < drop; i++ {
				_, n := utf8.DecodeLastRune(b)
				b = b[:len(b)-n]
			}
			term.Write(b)
			term.WriteString("...")
		} else {
			term.Write(b)
		}
	}

	// TODO game loop needs sub-runs
	// for {
	// 	var g game
	// 	if err := ui.run(&g); err == errGameOver {
	// 		// TODO prompt for again / present a postmortem
	// 		continue
	// 	} else if err == errQuitGame {
	// 		return nil
	// 	} else if err != nil {
	// 		return err
	// 	}
	// }

	return nil
}

func run() (rerr error) {
	flog, err := os.Create("log")
	if err != nil {
		return err
	}
	defer flog.Close()
	defer setLogOutput(io.MultiWriter(flog, &logBuf))()

	term, err := terminal.Open(nil, nil, terminal.Options(
		terminal.RawMode,
		terminal.HiddenCursor,
		// terminal.MouseReporting,
		terminal.Signals(syscall.SIGINT, syscall.SIGTERM, syscall.SIGWINCH),
	))
	if err != nil {
		return err
	}
	defer func() {
		if cerr := term.Close(); rerr == nil {
			rerr = cerr
		}
	}()

	log.Printf("running")
	return term.Run(terminal.DrawFunc(draw), terminal.ClientDrawTicker)
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
