package main

import (
	"math/rand"

	termbox "github.com/nsf/termbox-go"

	"github.com/jcorbin/execs/internal/ecs"
	"github.com/jcorbin/execs/internal/markov"
	"github.com/jcorbin/execs/internal/point"
)

const (
	componentTableColor ecs.ComponentType = 1 << iota
)

type colorTable struct {
	markov.Table
	color  []termbox.Attribute
	lookup map[termbox.Attribute]int
}

func (ct *colorTable) addLevelTransitions(
	colors []termbox.Attribute,
	zeroOn, zeroUp int,
	oneDown, oneOn, oneUp int,
) {
	n := len(colors)
	c0 := colors[0]

	for i, c1 := range colors {
		if c1 == c0 {
			continue
		}

		ct.addTransition(c0, c0, (n-i)*zeroOn)
		ct.addTransition(c0, c1, (n-i)*zeroUp)

		ct.addTransition(c1, c0, (n-1)*oneDown)
		ct.addTransition(c1, c1, (n-1)*oneOn)

		for _, c2 := range colors {
			if c2 != c1 && c2 != c0 {
				ct.addTransition(c1, c2, (n-1)*oneUp)
			}
		}
	}
}

func (ct *colorTable) AddEntity() ecs.Entity {
	ent := ct.Table.AddEntity()
	ct.color = append(ct.color, 0)
	return ent
}

func (ct *colorTable) toEntity(a termbox.Attribute) ecs.Entity {
	if id, def := ct.lookup[a]; def {
		return ct.Ref(id)
	}
	ent := ct.AddEntity()
	ent.AddComponent(componentTableColor)
	id := ent.ID()
	ct.color[id] = a
	if ct.lookup == nil {
		ct.lookup = make(map[termbox.Attribute]int, 1)
	}
	ct.lookup[a] = id
	return ent
}

func (ct *colorTable) toColor(ent ecs.Entity) (termbox.Attribute, bool) {
	if !ent.Type().All(componentTableColor) {
		return 0, false
	}
	return ct.color[ent.ID()], true
}

func (ct *colorTable) addTransition(a, b termbox.Attribute, w int) (ae, be ecs.Entity) {
	ae, be = ct.toEntity(a), ct.toEntity(b)
	ct.AddTransition(ae, be, w)
	return
}

func (ct *colorTable) genTile(
	rng *rand.Rand,
	box point.Box,
	f func(point.Point, termbox.Attribute),
) {
	// TODO: better 2d generation
	last := floorTable.Ref(0)
	var pos point.Point
	for pos.Y = box.TopLeft.Y + 1; pos.Y < box.BottomRight.Y+1; pos.Y++ {
		first := last
		for pos.X = box.TopLeft.X + 1; pos.X < box.BottomRight.X+1; pos.X++ {
			c, _ := floorTable.toColor(last)
			f(pos, c)
			last = floorTable.ChooseNext(rng, last)
		}
		last = floorTable.ChooseNext(rng, first)
	}
}
