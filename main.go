package main

import (
	"bytes"
	"io"
	"log"
	"os"

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
	if ev.Type != terminal.EventNone {
		log.Printf("got %v", ev)
	}

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

	it.textbox("Logs", logBuf.Bytes())

	return nil
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

func run() (rerr error) {
	flog, err := os.Create("log")
	if err != nil {
		return err
	}
	defer flog.Close()
	defer setLogOutput(io.MultiWriter(flog, &logBuf))()

	term, err := terminal.Open(nil, nil, terminal.Options(
		terminal.StandardApp,
		terminal.MouseReporting,
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
