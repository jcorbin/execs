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

func (w *world) Render(ctx *view.Context) error {
	// TODO: translate world state into ctx

	return nil
}

func (w *world) Close() error {
	// TODO: shutdown any long-running resources

	return nil
}

func (w *world) HandleKey(v *view.View, k view.KeyEvent) error {
	// TODO: do something with it

	return nil
}

func main() {
	if err := view.JustKeepRunning(func(v *view.View) (view.Client, error) {
		var w world

		// TODO: something interesting

		return &w, nil
	}); err != nil {
		log.Fatal(err)
	}
}
