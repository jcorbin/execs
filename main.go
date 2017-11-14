package main

import (
	"log"
	"math/rand"

	"github.com/jcorbin/execs/internal/ecs"
	"github.com/jcorbin/execs/internal/point"
	"github.com/jcorbin/execs/internal/view"
)

const (
	componentPosition ecs.ComponentType = 1 << iota
	componentGlyph
	componentTile
)

const (
	renderMask = componentPosition | componentGlyph
	tileMask   = componentPosition | componentTile
)

type tile struct {
	size point.Point
	data []uint8
}

func tileGlyph(d uint8) rune {
	switch d {
	case 0:
		return 0
	default:
		return '#'
	}
}

func makeTile(sz point.Point) tile {
	return tile{
		size: sz,
		data: make([]uint8, sz.X*sz.Y),
	}
}

type world struct {
	ecs.Core

	view *view.View

	positions []point.Point
	glyphs    []rune

	tileIndex  []int
	tileRIndex []int
	tiles      []tile
}

func (w *world) AddEntity() ecs.Entity {
	ent := w.Core.AddEntity()
	// TODO: support re-use
	w.positions = append(w.positions, point.Point{})
	w.glyphs = append(w.glyphs, 0)
	w.tileIndex = append(w.tileIndex, 0)
	w.tileRIndex = append(w.tileRIndex, 0)
	return ent
}

func (w *world) addRenderable(pos point.Point, glyph rune) ecs.Entity {
	ent := w.AddEntity()
	ent.AddComponent(renderMask)
	w.positions[ent.ID()] = pos
	w.glyphs[ent.ID()] = glyph
	return ent
}

func (w *world) addTile(pos point.Point, tile tile) ecs.Entity {
	ent := w.AddEntity()
	ent.AddComponent(tileMask)
	tileID := len(w.tiles)
	w.positions[ent.ID()] = pos
	w.tiles = append(w.tiles, tile)
	w.tileIndex[ent.ID()] = tileID
	w.tileRIndex[tileID] = ent.ID()
	return ent
}

func (w *world) Bounds() point.Box {
	var (
		box   point.Box
		first bool
	)
	for id, t := range w.Entities {
		if t&renderMask == renderMask {
			pos := w.positions[id]
			if first {
				box.TopLeft = pos
				box.BottomRight = pos
				first = false
				continue
			}
			box = box.ExpandTo(pos)
		}
	}
	for tid := range w.tiles {
		eid := w.tileRIndex[tid]
		pos := w.positions[eid]
		box = box.ExpandTo(pos)
		box = box.ExpandTo(pos.Add(w.tiles[tid].size))
	}
	return box
}

func (w *world) Step() error {
	return w.view.Update(func(grid view.Grid, avail point.Point) view.Grid {
		bbox := w.Bounds()

		buf := view.MakeGrid(bbox.Size())

		for tid := range w.tiles {
			eid := w.tileRIndex[tid]
			pos := w.positions[eid]
			stride := w.tiles[tid].size.X
			data := w.tiles[tid].data
			for i, j := pos.Y*buf.Size.X+pos.X, 0; j < len(data); {
				for k := 0; k < stride; {
					if r := tileGlyph(data[j]); r != 0 {
						buf.Data[i].Ch = r
					}
					i++
					j++
					k++
				}
				i += buf.Size.X
			}
		}

		for id, t := range w.Entities {
			if t&renderMask == renderMask {
				pos := w.positions[id].Sub(bbox.TopLeft)
				i := pos.Y*buf.Size.X + pos.X
				buf.Data[i].Ch = w.glyphs[id]
			}
		}

		return buf
	})
}

func (w *world) genWire(rng *rand.Rand, turnProb float64) point.Box {
	moves := []point.Point{
		{X: 0, Y: 1},
		{X: 1, Y: 0},
		{X: 0, Y: -1},
		{X: -1, Y: 0},
	}

	body := []rune{'|', '-', '|', '-'}
	head := []rune{'^', '>', '-', '<'}

	var box point.Box
	pos := point.Zero
	dir := 1
	for i := 0; i < 40; i++ {
		if rng.Float64() < turnProb {
			if rng.Intn(2) == 0 {
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
		box = box.ExpandTo(pos)
		pos = pos.Add(moves[dir])
	}
	box = box.ExpandTo(pos)
	w.addRenderable(pos, head[dir])
	return box
}

type markovInts map[int]map[int]int

func (mi markovInts) next(s int, rng *rand.Rand) int {
	var r int
	wsum := float64(0)
	for t, w := range mi[s] {
		wf := float64(w)
		wsum += wf
		if rand.Float64() <= wf/wsum {
			r = t
		}
	}
	return r
}

var (
	roomDigStateTrantsitions = markovInts{
		0: {
			0: 4,
			1: 2, -1: 2,
			-2: 1, 2: 1,
		},

		1:  {0: 3, 1: 2, -1: 1},
		-1: {0: 3, 1: 1, -1: 2},

		2:  {0: 2, -2: 1},
		-2: {0: 2, 2: 1},
	}
)

func genRoomAround(rng *rand.Rand, around, within point.Box) tile {
	const stepTryLimit = 10
	tile := makeTile(within.Size())
	log.Printf("gen room tile %v around %v within %v", tile.size, around, within)
	pos := around.TopLeft
	isize := around.Size()
	off := 0
	for _, r := range []struct {
		head, norm point.Point
		k          int
	}{
		{head: point.Point{X: 1, Y: 0}, norm: point.Point{X: 0, Y: 1}, k: isize.X},   // top edge
		{head: point.Point{X: 0, Y: -1}, norm: point.Point{X: 1, Y: 0}, k: isize.Y},  // right edge
		{head: point.Point{X: -1, Y: 0}, norm: point.Point{X: 0, Y: -1}, k: isize.X}, // bottom edge
		{head: point.Point{X: 0, Y: 1}, norm: point.Point{X: -1, Y: 0}, k: isize.Y},  // left edge
		{head: point.Point{X: 1, Y: 0}, norm: point.Point{X: 0, Y: 1}, k: 0},         // top-left corner
	} {
		state := 0
		log.Printf("run @%v %+v", pos, r)
		for i := 0; i < r.k+off; i++ {
			pos = pos.Add(r.head)
			for i := 0; i < stepTryLimit; i++ {
				nextState := roomDigStateTrantsitions.next(state, rng)
				if pt := pos.Sub(r.norm.Mul(nextState)); within.Contains(pt) {
					state, pos = nextState, pt
					break
				}
			}
			tpos := pos.Sub(within.TopLeft)
			log.Printf("room state=% 2v @% 3v ix@% 3v", state, pos, tpos)
			tile.data[tpos.Y*tile.size.X+tpos.X] = 1
		}
		off = within.DistanceTo(pos).Dot(r.norm)
	}
	return tile
}

func main() {
	rng := rand.New(rand.NewSource(rand.Int63()))

	const (
		roomInnerSpace  = 2
		roomMaxDisplace = 5
	)

	if err := func() error {
		var v view.View
		if err := v.Start(); err != nil {
			return err
		}

	worldloop:
		for {
			v.Log("A Whole New World")
			w := world{
				view: &v,
			}

			bbox := w.genWire(rng, 0.1)

			inner := bbox.ExpandBy(point.Point{X: roomInnerSpace, Y: roomInnerSpace})
			bbox = inner.ExpandBy(point.Point{X: roomMaxDisplace, Y: roomMaxDisplace})
			w.addTile(bbox.TopLeft, genRoomAround(rng, inner, bbox))

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
