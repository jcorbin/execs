package main

import (
	"log"

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
	View *view.View
	ecs.Core
	// XXX
}

func createWorld(v *View) *world {
	w := world{
		View: v,
	}

	// TODO: something interesting

	return w
}

func (w *world) Render(grid view.Grid, avail point.Point) view.Grid {
	// TODO: translate world state into a grid, return it

}

func (w *world) Step(v *view.View) bool {
	select {
	case k := <-v.Keys():
		// TODO: do something in response to the key

	case <-v.Done():
		return false
	}

	return true
}

func main() {
	if err := view.JustKeepRunning(func(v *view.View) (view.Stepable, error) {
		return createWorld(v), nil
	}); err != nil {
		log.Fatal(err)
	}
}
