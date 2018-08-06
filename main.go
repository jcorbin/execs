package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"syscall"
	"time"

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

func draw(term *terminal.Terminal, ev terminal.Event) error {
	if ev.Key == terminal.KeyCtrlC {
		return terminal.ErrStop
	}

	if _, err := term.WriteCursor(display.Cursor.Home, display.Cursor.Clear); err != nil {
		return err
	}

	log.Printf("got event: %+v", ev)

	// size, err := term.Size()
	// if err != nil {
	// 	return err
	// }

	term.WriteString("| Logs |")
	// term.WriteString(strings.Repeat("-", size.X-8))
	term.WriteByte('\n')
	sc := bufio.NewScanner(bytes.NewReader(logBuf.Bytes()))
	for sc.Scan() {
		fmt.Fprintf(term, "| - %q\n", sc.Bytes())
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

func run() error {
	flog, err := os.Create("log")
	if err != nil {
		return err
	}
	defer flog.Close()

	term, err := terminal.Open(nil, nil, terminal.Options(
		terminal.RawMode,
		terminal.HiddenCursor,
		// terminal.MouseReporting,
		terminal.Signals(syscall.SIGINT, syscall.SIGTERM, syscall.SIGWINCH),
	))
	if err != nil {
		return err
	}

	log.Printf("about to run")

	defer setLogOutput(io.MultiWriter(flog, &logBuf))()

	log.Printf("running")

	err = term.Run(terminal.DrawFunc(draw),
		terminal.ClientFlushEvery(time.Second/60),
	)
	if cerr := term.Close(); err == nil {
		err = cerr
	}
	return err
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
