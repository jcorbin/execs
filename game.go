package main

import (
	"fmt"
	"image"
	"log"

	"github.com/jcorbin/anansi"
	"github.com/jcorbin/anansi/ansi"
	"github.com/jcorbin/anansi/x/platform"

	"github.com/jcorbin/execs/internal/ecs"
)

type game struct {
	ecs.Scope
	ren   render
	pos   position
	spawn ecs.ArrayIndex

	ctl  control
	drag dragState

	inspecting ecs.Entity
	pop        popup

	gen worldGen
}

const (
	gamePosition ecs.Type = 1 << iota
	gameRender
	gameCollides
	gameInput
	gameSpawn
	gameInteract

	gameWall       = gamePosition | gameRender | gameCollides
	gameFloor      = gamePosition | gameRender
	gameSpawnPoint = gamePosition | gameSpawn
	gameCharacter  = gamePosition | gameRender | gameCollides
	gamePlayer     = gameCharacter | gameInput
	gameDoor       = gamePosition | gameRender // FIXME | gameCollides | gameInteract
)

var worldConfig = worldGenConfig{
	Wall: style(gameWall, 5, '#', ansi.SGRAttrBold|
		ansi.RGB(0x18, 0x18, 0x18).BG()|
		ansi.RGB(0x30, 0x30, 0x30).FG()),
	Floor: style(gameFloor, 4, '·',
		ansi.RGB(0x10, 0x10, 0x10).BG()|
			ansi.RGB(0x18, 0x18, 0x18).FG()),
	Door: style(gameDoor, 6, '+',
		ansi.RGB(0x18, 0x18, 0x18).BG()|
			ansi.RGB(0x60, 0x40, 0x30).FG()),
	Player: style(gamePlayer, 10, '@', ansi.SGRAttrBold|
		ansi.RGB(0x60, 0x80, 0xa0).FG(),
	),
	RoomSize:    image.Rect(5, 3, 21, 13),
	ExitDensity: 25,
	GenDepth:    10,
}

func newGame() *game {
	g := &game{}

	g.gen.worldGenConfig = worldConfig

	g.Scope.Watch(gamePosition, 0, &g.pos)
	g.Scope.Watch(gameRender, 0, &g.ren)
	g.Scope.Watch(gamePlayer, 0, &g.ctl)
	g.Scope.Watch(gameSpawnPoint, 0, &g.spawn)

	// TODO better dep coupling
	g.ren.pos = &g.pos
	g.ctl.pos = &g.pos
	g.gen.g = g

	// generate level
	g.gen.genLevel()

	// place characters
	spawnPos := g.pos.Get(g.spawn.Scope.Entity(g.spawn.ID(0)))
	log.Printf("spawn player @%v", spawnPos)
	g.gen.Player.createAt(g, spawnPos.Point())

	return g
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

	// TODO debug why empty
	if r := g.drag.process(ctx); r != image.ZR {
		r = r.Canon().Add(g.ctl.view.Min)
		n := 0
		for q := g.pos.Within(r); q.next(); n++ {
			posd := q.handle()
			rend := g.ren.Get(posd.Entity())
			log.Printf("%v %v", posd, rend)
		}
		log.Printf("queried %v entities in %v", n, r)
		g.pop.active = false
	}

	if g.ctl.process(ctx) || g.drag.active {
		g.pop.active = false
	}

	// TODO debug
	// if m, haveMouse := ctx.Input.LastMouse(false); haveMouse && m.State.IsMotion() {
	// 	if posd := g.pos.At(m.Point.Add(g.ctl.view.Min)); posd.zero() {
	// 		g.pop.active = false
	// 	} else {
	// 		g.inspect(posd.Entity())
	// 		g.pop.setAt(m.Point)
	// 		g.pop.active = true
	// 	}
	// }

	ctx.Output.Clear()
	g.ren.drawRegionInto(g.ctl.view, &ctx.Output.Grid)

	if g.drag.active {
		dr := g.drag.r.Canon()
		eachCell(&ctx.Output.Grid, dr, func(cell anansi.Cell) {
			dc := uint32(0x1000)
			if cell.X == dr.Min.X ||
				cell.Y == dr.Min.Y ||
				cell.X == dr.Max.X-1 ||
				cell.Y == dr.Max.Y-1 {
				dc = 0x2000
			}
			// TODO better brighten function
			if r := cell.Rune(); r == 0 {
				cell.SetRune(' ') // TODO this shouldn't be necessary, test and fix anansi.Screen
			}
			a := cell.Attr()
			c, _ := a.BG()
			cr, cg, cb, ca := c.RGBA()
			cell.SetAttr(a.SansBG() | ansi.RGBA(cr+dc, cg+dc, cb+dc, ca).BG())
		})
	} else if g.pop.active {
		g.pop.drawInto(&ctx.Output.Grid)
	}

	return err
}

func removeEntity(ents []ecs.Entity, i int) []ecs.Entity {
	copy(ents[i:], ents[i+1:])
	ents = ents[:len(ents)-1]
	return ents
}

type dragState struct {
	active bool
	r      image.Rectangle
}

func (ds *dragState) process(ctx *platform.Context) (r image.Rectangle) {
	for id, typ := range ctx.Input.Type {
		if typ == platform.EventMouse {
			m := ctx.Input.Mouse(id)
			if b, isPress := m.State.IsPress(); isPress && b == 0 {
				ds.r.Min = m.Point
				ctx.Input.Type[id] = platform.EventNone
			} else if m.State.IsDrag() {
				ds.r.Max = m.Point
				if ds.r.Min == image.ZP {
					ds.r.Min = m.Point
					ds.r.Max = m.Point
				}
				ds.active = true
				ctx.Input.Type[id] = platform.EventNone
			} else {
				if ds.active && m.State.IsRelease() {
					ds.r.Max = m.Point
					r = ds.r
					ctx.Input.Type[id] = platform.EventNone
				}
				ds.active = false
				ds.r = image.ZR
				break
			}
		}
	}
	return r
}

func (g *game) inspect(ent ecs.Entity) {
	if ent == g.inspecting {
		return
	}
	log.Printf("inspect %v", ent)
	g.inspecting = ent

	g.pop.buf.Reset()
	g.pop.buf.Grow(1024)
	describe(&g.pop.buf, ent, []descSpec{
		{gameInput, "Ctl", nil},
		{gameCollides, "Col", nil},
		{gamePosition, "Pos", func(ent ecs.Entity) fmt.Stringer { return g.pos.Get(ent) }},
		{gameRender, "Ren", func(ent ecs.Entity) fmt.Stringer { return g.ren.Get(ent) }},
	})
	g.pop.processBuf()
}

func eachCell(g *anansi.Grid, r image.Rectangle, f func(anansi.Cell)) {
	for p := r.Min; p.Y < r.Max.Y; p.Y++ {
		for p.X = r.Min.X; p.X < r.Max.X; p.X++ {
			f(g.Cell(p))
		}
	}
}
