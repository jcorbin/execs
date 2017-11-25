package main

import (
	"log"

	"github.com/jcorbin/execs/internal/ecs"
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
}

func (w *world) Render(termGrid view.Grid) error {
	hud := view.HUD{
		Logs: w.Logs,
		// World: , // TODO: render your world grid and pass it here
	}

	// TODO: call hud methods to build a basic UI, e.g.:
	hud.AddHeaderF("<left1")
	hud.AddHeaderF("<left2")
	hud.AddHeaderF(">right1")
	hud.AddHeaderF(">right2")
	hud.AddHeaderF("center by default")

	hud.AddFooterF("footer has the same stuff")
	hud.AddFooterF(">one")
	hud.AddFooterF(">two")
	hud.AddFooterF(".>three") // the "." forces a new line

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

		// TODO: something interesting

		return &w, nil
	}); err != nil {
		log.Fatal(err)
	}
}
