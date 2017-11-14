package main

import (
	"math/rand"

	"github.com/jcorbin/execs/internal/ecs"
	"github.com/jcorbin/execs/internal/point"
	"github.com/jcorbin/execs/internal/view"
)

const (
	ComponentPosition ecs.ComponentType = 1 << iota
	ComponentGlyph
)

const (
	renderMask = ComponentPosition | ComponentGlyph
)

type Box struct {
	TopLeft     point.Point
	BottomRight point.Point
}

func (b Box) Size() point.Point {
	pt := b.BottomRight.Sub(b.TopLeft).Abs()
	pt.X++
	pt.Y++
	return pt
}

func (b Box) ExpandTo(pt point.Point) Box {
	if pt.X < b.TopLeft.X {
		b.TopLeft.X = pt.X
	}
	if pt.Y < b.TopLeft.Y {
		b.TopLeft.Y = pt.Y
	}
	if pt.X > b.BottomRight.X {
		b.BottomRight.X = pt.X
	}
	if pt.Y > b.BottomRight.Y {
		b.BottomRight.Y = pt.Y
	}
	return b
}

type World struct {
	ecs.Core

	Positions []point.Point
	Glyphs    []rune

	View *view.View
}

func (w *World) AddEntity() ecs.Entity {
	ent := w.Core.AddEntity()
	// TODO: support re-use
	w.Positions = append(w.Positions, point.Point{})
	w.Glyphs = append(w.Glyphs, 0)
	return ent
}

func (w *World) addRenderable(pos point.Point, glyph rune) ecs.Entity {
	ent := w.AddEntity()
	ent.AddComponent(renderMask)
	w.Positions[ent.ID()] = pos
	w.Glyphs[ent.ID()] = glyph
	return ent
}

func (w *World) Bounds() Box {
	var (
		box   Box
		first bool
	)
	for id, t := range w.Entities {
		if t&renderMask == renderMask {
			pos := w.Positions[id]
			if first {
				box.TopLeft = pos
				box.BottomRight = pos
				first = false
				continue
			}
			box = box.ExpandTo(pos)
		}
	}
	return box
}

func (w *World) Step() error {
	return w.View.Update(func(grid view.Grid, avail point.Point) view.Grid {
		box := w.Bounds()
		buf := view.MakeGrid(box.Size())
		for id, t := range w.Entities {
			if t&renderMask == renderMask {
				pos := w.Positions[id].Sub(box.TopLeft)
				i := pos.Y*buf.Size.X + pos.X
				buf.Data[i].Ch = w.Glyphs[id]
			}
		}
		return buf
	})
}

func main() {
	if err := func() error {
		var v view.View
		if err := v.Start(); err != nil {
			return err
		}

	worldloop:
		for {
			v.Log("A Whole New World")
			w := World{
				View: &v,
			}

			moves := []point.Point{
				{X: 0, Y: 1},
				{X: 1, Y: 0},
				{X: 0, Y: -1},
				{X: -1, Y: 0},
			}

			body := []rune{'|', '-', '|', '-'}
			head := []rune{'^', '>', '-', '<'}

			pos := point.Zero
			dir := 1
			for i := 0; i < 40; i++ {
				if rand.Float64() < 0.1 {
					if rand.Intn(2) == 0 {
						dir--
						for dir < 0 {
							dir += len(moves)
						}
					} else {
						dir++
					}
					dir = dir % 4
					w.addRenderable(pos, '+')
				} else {
					w.addRenderable(pos, body[dir])
				}
				pos = pos.Add(moves[dir])
			}
			w.addRenderable(pos, head[dir])

		simloop:
			for {
				if err := w.Step(); err != nil {
					return err
				}

				select {
				case k := <-v.Keys():
					switch k.Ch {
					case '*':
						break simloop
					}
					v.Log("KEY: %+v", k)

				case <-v.Done():
					break worldloop

				}
			}

		}

		return v.Err()
	}(); err != nil {
		panic(err)
	}
}
