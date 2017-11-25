package main

import (
	"log"
	"math/rand"

	"github.com/jcorbin/execs/internal/ecs"
	"github.com/jcorbin/execs/internal/point"
	"github.com/jcorbin/execs/internal/view"
)

// const (
// 	XXX ecs.ComponentType = 1 << iota
// )

// const (
// 	XXX = YYY | ZZZ
// )

type world struct {
	ecs.Core
	view.Logs

	// TODO: your state here
	grid view.Grid
}

func (w *world) Render(termGrid view.Grid) error {
	hud := view.HUD{
		Logs:  w.Logs,
		World: w.grid, // TODO: render your world grid and pass it here
	}

	// TODO: call hud methods to build a basic UI, e.g.:
	hud.HeaderF("<left1")
	hud.HeaderF("<left2")
	hud.HeaderF(">right1")
	hud.HeaderF(">right2")
	hud.HeaderF("center by default")

	hud.FooterF("footer has the same stuff")
	hud.FooterF(">one")
	hud.FooterF(">two")
	hud.FooterF(".>three") // the "." forces a new line

	// NOTE: more advanced UI components may use:
	// hud.AddRenderable(ren view.Renderable, align view.Align)

	hud.Render(termGrid)
	return nil
}

func (w *world) Close() error {
	// TODO: shutdown any long-running resources

	return nil
}

func (w *world) HandleKey(k view.KeyEvent) error {
	// TODO: do something with it

	return nil
}

func main() {
	if err := view.JustKeepRunning(func(v *view.View) (view.Client, error) {
		var w world
		w.Logs.Init(1000)

		w.Log("Hello World Of Democraft!")

		// TODO: this is just here for demonstration; replace it with something
		// interesting!
		w.grid = view.MakeGrid(point.Point{X: 64, Y: 32})
		for chs, i := []rune{
			'_', '-',
			'=', '+',
			'/', '?',
			'\\', '|',
			',', '.',
			':', ';',
			'"', '\'',
			'<', '>',
			'[', ']',
			'{', '}',
			'(', ')',
			'!', '@', '#', '$',
			'%', '^', '&', '*',
		}, 0; i < len(w.grid.Data); i++ {
			w.grid.Data[i].Ch = chs[rand.Intn(len(chs))]
		}

		return &w, nil
	}); err != nil {
		log.Fatal(err)
	}
}
