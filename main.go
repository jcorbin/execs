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

func (it *ui) header(label string) {
	it.WriteByte('|')
	it.WriteString(label)
	it.WriteString(strings.Repeat("-", it.size.X-len(label)))
	it.WriteByte('|')
}

func draw(term *terminal.Terminal, ev terminal.Event) error {
	if ev.Key == terminal.KeyCtrlC {
		panic("bang")
		// return terminal.ErrStop
	}

	if _, err := term.WriteCursor(display.Cursor.Home, display.Cursor.Clear); err != nil {
		return err
	}

	log.Printf("got event: %+v", ev)

	var it ui
	it.init(term)

	it.header(" Logs ")

	sc := bufio.NewScanner(bytes.NewReader(logBuf.Bytes()))
	for sc.Scan() {
		fmt.Fprintf(term, "\r\n| - %q", sc.Bytes())
	}

	// display.Cursor.Hide,

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
