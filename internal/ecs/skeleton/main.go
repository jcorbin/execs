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

	// TODO: your state here
}

func createWorld(v *View) *world {
	var w world

	// TODO: something interesting

	return w
}

func (w *world) Render(ctx *view.Context) {
	// TODO: translate world state into ctx

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
