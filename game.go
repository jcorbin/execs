package main

import (
	"fmt"
	"image"
	"math/rand"

	"github.com/jcorbin/anansi/ansi"
	"github.com/jcorbin/anansi/x/platform"

	"github.com/jcorbin/execs/internal/ecs"
)

type game struct {
	ecs.Scope
	render render

	ctl control
}

const (
	gameRender ecs.Type = 1 << iota
	gameInput

	// gamePosition // TODO separate from gameRender

	gameWall      = gameRender
	gameFloor     = gameRender
	gameCharacter = gameRender
	gamePlayer    = gameCharacter | gameInput
)

func newGame() *game {
	g := &game{}

	g.Scope.Watch(gameRender, 0, &g.render)
	g.Scope.Watch(gamePlayer, 0, &g.ctl)
	g.ctl.getPos = g.getEntPos
	g.ctl.setPos = g.setEntPos

	walls := builder{
		g: g,
		style: style(gameWall, 5, '#', ansi.SGRAttrBold|
			ansi.RGB(0x10, 0x10, 0x10).BG()|
			ansi.RGB(0x20, 0x20, 0x20).FG()),
	}

	floors := builder{
		g: g,
		style: style(gameFloor, 4, '·',
			ansi.RGB(0x08, 0x08, 0x08).BG()|
				ansi.RGB(0x10, 0x10, 0x10).FG()),
	}

	// create room walls
	bounds := image.Rect(0, 0, 20, 10)

	walls.reset()
	floors.reset()
	walls.rectangle(bounds)
	floors.fill(bounds.Inset(1))

	var door ecs.Entity
	for i, wall := range walls.ents {
		pos := g.render.Get(wall).Point()
		if !isCorner(pos, bounds) {
			if door.Scope == nil || rand.Intn(i+1) <= 1 {
				door = wall
			}
		}
	}
	if door.Scope != nil {
		floors.style.applyTo(g, door)
	}
	// TODO actual door entity
	// TODO hallway
	// TODO room
	// TODO loop

	// place characters
	style(gamePlayer, 10, '@', ansi.SGRAttrBold|
		ansi.RGB(0x40, 0x60, 0x80).FG(),
	).createAt(g, image.Pt(10, 5))

	return g
}

func (g *game) getEntPos(ent ecs.Entity) image.Point {
	return g.render.Get(ent).Point()
}

func (g *game) setEntPos(ent ecs.Entity, pos image.Point) {
	g.render.Get(ent).SetPoint(pos)
}

func (g *game) Update(ctx *platform.Context) (err error) {
	// Ctrl-C interrupts
	if ctx.Input.HasTerminal('\x03') {
		// ... AFTER any other available input has been processed
		err = errInt
		// ... NOTE err != nil will prevent wasting any time flushing the final
		//          lame-duck frame
	}

	// Ctrl-Z suspends
	if ctx.Input.CountRune('\x1a') > 0 {
		defer func() {
			if err == nil {
				err = ctx.Suspend()
			} // else NOTE don't bother suspending, e.g. if Ctrl-C was also present
		}()
	}

	g.ctl.Update(ctx)

	ctx.Output.Clear()
	g.render.drawRegionInto(g.ctl.view, &ctx.Output.Grid)

	return err
}

type builder struct {
	g    *game
	pos  image.Point
	ents []ecs.Entity

	style buildStyle
}

func (bld *builder) reset() {
	bld.ents = bld.ents[:0]
}

func (bld *builder) moveTo(pos image.Point) {
	bld.pos = pos
}

func (bld *builder) rectangle(box image.Rectangle) {
	bld.moveTo(box.Min)
	bld.lineTo(image.Pt(0, 1), box.Dy()-1)
	bld.lineTo(image.Pt(1, 0), box.Dx()-1)
	bld.lineTo(image.Pt(0, -1), box.Dy()-1)
	bld.lineTo(image.Pt(-1, 0), box.Dx()-1)
}

func (bld *builder) fill(box image.Rectangle) {
	for bld.moveTo(box.Min); bld.pos.Y < box.Max.Y; bld.pos.Y++ {
		for bld.pos.X = box.Min.X; bld.pos.X < box.Max.X; bld.pos.X++ {
			bld.ents = append(bld.ents,
				bld.style.createAt(bld.g, bld.pos))
		}
	}
}

func (bld *builder) lineTo(d image.Point, n int) {
	for i := 0; i < n; i++ {
		bld.ents = append(bld.ents,
			bld.style.createAt(bld.g, bld.pos))
		bld.pos = bld.pos.Add(d)
	}
}

type buildStyle struct {
	t ecs.Type
	z int
	r rune
	a ansi.SGRAttr
}

func style(t ecs.Type, z int, r rune, a ansi.SGRAttr) buildStyle {
	return buildStyle{t, z, r, a}
}

func (st buildStyle) String() string {
	return fmt.Sprintf("t:%v z:%v rune:%q attr:%v", st.t, st.z, st.r, st.a)
}

func (st buildStyle) createAt(g *game, pos image.Point) ecs.Entity {
	ent := g.Create(st.t)
	rend := g.render.Get(ent)
	rend.SetPoint(pos)
	rend.SetZ(st.z)
	rend.SetCell(st.r, st.a)
	return ent
}

func (st buildStyle) applyTo(g *game, ent ecs.Entity) {
	ent.SetType(st.t)
	rend := g.render.Get(ent)
	rend.SetZ(st.z)
	rend.SetCell(st.r, st.a)
}

func isCorner(p image.Point, r image.Rectangle) bool {
	return (p.X == r.Min.X && p.Y == r.Min.Y) ||
		(p.X == r.Min.X && p.Y == r.Max.Y-1) ||
		(p.X == r.Max.X-1 && p.Y == r.Min.Y) ||
		(p.X == r.Max.X-1 && p.Y == r.Max.Y-1)
}
