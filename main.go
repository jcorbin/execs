package main

import (
	"errors"
	"image"
	"log"

	"github.com/jcorbin/execs/internal/ecs"
	"github.com/jcorbin/execs/internal/terminal"
)

var (
	errQuitGame = errors.New("quit game")
	errGameOver = errors.New("game over")
)

type game struct {
	world world
	name  []byte
}

type world struct {
	ecs.Scope

	char []rune
	fg   []image.RGBA
	pos  []image.Point
}

func (g *game) draw(term *terimnal.Terminal, ev terminal.Event) error {
	/*

		ui.Swear(
			display.Cursor.Home,
			display.Cursor.Clear,
			display.Cursor.Hide)

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

	return nil
}

func run(term *Terminal, ev Event) error {

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

func main() {
	term, err := terminal.Open(nil, nil, terminal.Options(
		terminal.HiddenCursor,
		terminal.MouseReporting,
	))
	if err == nil {
		// terminal.FlushAfter(time.Second / 60)
		err = term.Run(terminal.DrawFunc(run))
	}
	if cerr := term.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		log.Fatal(err)
	}
}
