package main

import (
	"bytes"
	"io"
	"log"
	"os"

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

type app struct {
	uiState
}

var spinner = []rune{
	'◜',
	'◠',
	'◝',
	'◞',
	'◡',
	'◟',
}

func (ap *app) Draw(term *terminal.Terminal, ev terminal.Event) error {
	if ap.uiState.maxID == 0 && ev.Type != terminal.NoEvent {
		log.Printf("doing a first dry-run draw %v", ev)
		err := ap.Draw(term, terminal.Event{})
		if err == nil {
			err = term.Discard()
		}
		if err != nil {
			return err
		}
		log.Printf("initial maxID %v", ap.uiState.maxID)
	}

	it, err := startUI(term, &ap.uiState, ev)
	if err != nil {
		return err
	}

	switch ev.Type {
	case terminal.NoEvent:

	case terminal.KeyEvent:
		if ev.Ch == '*' {
			ap.crawling = !ap.crawling
			log.Printf("toggled crawling to %v", ap.crawling)
		}

	case terminal.TickEvent:
		ap.tick++

		// TODO there's still something not right here, something to learn
		crawl := ap.crawling
		if !crawl {
			if ap.crawl != it.size {
				crawl = true
			}
		}
		if crawl {
			if ap.crawl.Y == 0 {
				ap.crawl.X++
			} else if ap.crawl.Y == it.size.Y {
				ap.crawl.X--
			}
			if ap.crawl.X == it.size.X {
				ap.crawl.Y++
			} else if ap.crawl.X == 0 {
				ap.crawl.Y--
			}
		}

	case terminal.ResizeEvent:
		log.Printf("resized to %v", it.size)
	default:
		log.Printf("got %v", ev)
	}

	if ev.Type == terminal.InterruptEvent {
		return terminal.ErrStop
	}

	term.WriteCursor(
		terminal.Cursor.Hide,
		terminal.Cursor.Home,
		terminal.Cursor.Clear,
	)

	it.textbox("Logs", &logBuf)

	// TODO jank
	term.WriteCursor(func(cur terminal.Cursor, buf []byte) ([]byte, terminal.Cursor) {
		return cur.Go(buf, terminal.ImageToTermPoint(ap.crawl))
	})

	term.WriteRune(spinner[ap.tick%len(spinner)])

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
		terminal.FullscreenApp,
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

	var ap app
	log.Printf("running")
	return term.Run(&ap, terminal.ClientDrawTicker)
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
