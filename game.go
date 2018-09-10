package main

import (
	"fmt"
	"image"
	"log"
	"math/rand"

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
}

const (
	gamePosition ecs.Type = 1 << iota
	gameRender
	gameCollides
	gameInput
	gameSpawn

	// gamePosition // TODO separate from gameRender

	gameWall       = gamePosition | gameRender | gameCollides
	gameFloor      = gamePosition | gameRender
	gameSpawnPoint = gamePosition | gameSpawn
	gameCharacter  = gamePosition | gameRender | gameCollides
	gamePlayer     = gameCharacter | gameInput
)

func newGame() *game {
	g := &game{}

	g.Scope.Watch(gamePosition, 0, &g.pos)
	g.Scope.Watch(gameRender, 0, &g.ren)
	g.Scope.Watch(gamePlayer, 0, &g.ctl)
	g.Scope.Watch(gameSpawnPoint, 0, &g.spawn)

	// TODO better dep coupling
	g.ren.pos = &g.pos
	g.ctl.pos = &g.pos

	var (
		wallStyle = style(gameWall, 5, '#', ansi.SGRAttrBold|
			ansi.RGB(0x20, 0x20, 0x20).BG()|
			ansi.RGB(0x30, 0x30, 0x30).FG())
		floorStyle = style(gameFloor, 4, '·',
			ansi.RGB(0x10, 0x10, 0x10).BG()|
				ansi.RGB(0x18, 0x18, 0x18).FG())
		playerStyle = style(gamePlayer, 10, '@', ansi.SGRAttrBold|
			ansi.RGB(0x60, 0x80, 0xa0).FG(),
		)
	)

	// TODO re-evaluate builder abstraction
	gen := worldGen{
		g:        g,
		roomSize: image.Rect(5, 3, 21, 13),
		wall: builder{
			g:     g,
			style: wallStyle,
		},
		floor: builder{
			g:     g,
			style: floorStyle,
		},
	}

	// create room walls
	gen.room(image.Rectangle{image.ZP, gen.chooseRoomSize()})
	g.pos.Get(g.Create(gameSpawnPoint)).SetPoint(gen.r.Min.Add(gen.r.Size().Div(2)))

	// carve doorway
	if door, pos := gen.chooseWall(); door != ecs.ZE {
		// door
		log.Printf("doorway @%v", pos)
		gen.floor.applyTo(door)
		// TODO actual door entity

		// hallway
		dir := gen.wallNormal(pos)
		if n := rand.Intn(5) + 1; n > 0 {
			pos = gen.hallway(pos, dir, n)
		}

		// next room
		sz := gen.chooseRoomSize()
		enter := pos.Add(dir)
		gen.room(gen.placeRoom(enter, dir, sz))

		// TODO
		// log.Printf("doorway @%v", enter)
		// gen.floor.applyTo(door)
		// TODO actual door entity

		// TODO loop

	}

	// place characters
	spawnPos := g.pos.Get(g.spawn.Scope.Entity(g.spawn.ID(0)))
	log.Printf("spawn player @%v", spawnPos)
	playerStyle.createAt(g, spawnPos.Point())

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

type worldGen struct {
	g *game

	roomSize image.Rectangle
	floor    builder
	wall     builder

	r image.Rectangle
}

func (gen *worldGen) room(r image.Rectangle) {
	log.Printf("room @%v", r)
	gen.r = r
	gen.wall.reset()
	gen.floor.reset()
	gen.wall.rectangle(gen.r)
	gen.floor.fill(gen.r.Inset(1))
}

func (gen *worldGen) hallway(p, dir image.Point, n int) image.Point {
	log.Printf("hallway n:%v dir:%v", n, dir)
	orth := orthNormal(dir)
	gen.wall.reset()
	gen.floor.reset()
	for i := 0; i < n; i++ {
		p = p.Add(dir)
		// TODO deconflict?
		gen.floor.point(p)
		gen.wall.point(p.Add(orth))
		gen.wall.point(p.Sub(orth))
	}
	return p
}

func (gen *worldGen) wallNormal(p image.Point) (dir image.Point) {
	if p.X == gen.r.Min.X {
		dir.X = -1
	} else if p.Y == gen.r.Min.Y {
		dir.Y = -1
	} else if p.X == gen.r.Max.X-1 {
		dir.X = 1
	} else if p.Y == gen.r.Max.Y-1 {
		dir.Y = 1
	}
	return dir
}

func (gen *worldGen) chooseRoomSize() image.Point {
	return image.Pt(
		gen.roomSize.Min.X+rand.Intn(gen.roomSize.Dx()),
		gen.roomSize.Min.Y+rand.Intn(gen.roomSize.Dy()),
	)
}

func (gen *worldGen) placeRoom(enter, dir, sz image.Point) (r image.Rectangle) {
	// TODO better placement
	r.Min = enter
	if dir.Y == 0 {
		if dir.X == 1 {
			r.Max.X = r.Min.X + sz.X
		} else { // dir.X == -1
			r.Max.X = r.Min.X
			r.Min.X -= sz.X
		}
		if d := rand.Intn(sz.Y) - 2; d > 0 {
			r.Min.Y -= d
		}
		r.Max.Y = r.Min.Y + sz.Y
	} else { // dir.X == 0
		if dir.Y == 1 {
			r.Max.Y = r.Min.Y + sz.Y
		} else { // dir.Y == -1
			r.Max.Y = r.Min.Y
			r.Min.Y -= sz.Y
		}
		if d := rand.Intn(sz.X) - 2; d > 0 {
			r.Min.X -= d
		}
		r.Max.X = r.Min.X + sz.X
	}
	return r
}

func (gen *worldGen) chooseWall() (ent ecs.Entity, pos image.Point) {
	if gen.r != image.ZR {
		var j int
		for i, wall := range gen.wall.ents {
			if pt := gen.g.pos.Get(wall).Point(); !isCorner(pt, gen.r) {
				if ent == ecs.ZE || rand.Intn(i+1) <= 1 {
					j, ent, pos = i, wall, pt
				}
			}
		}
		if ent != ecs.ZE {
			copy(gen.wall.ents[j:], gen.wall.ents[j+1:])
			gen.wall.ents = gen.wall.ents[:len(gen.wall.ents)-1]
		}
	}
	return ent, pos
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

func (bld *builder) point(p image.Point) {
	bld.pos = p
	bld.create()
}

func (bld *builder) fill(r image.Rectangle) {
	for bld.moveTo(r.Min); bld.pos.Y < r.Max.Y; bld.pos.Y++ {
		for bld.pos.X = r.Min.X; bld.pos.X < r.Max.X; bld.pos.X++ {
			bld.create()
		}
	}
}

func (bld *builder) lineTo(p image.Point, n int) {
	for i := 0; i < n; i++ {
		bld.create()
		bld.pos = bld.pos.Add(p)
	}
}

func (bld *builder) create() {
	ent := bld.style.createAt(bld.g, bld.pos)
	bld.ents = append(bld.ents, ent)
}

func (bld *builder) applyTo(ent ecs.Entity) {
	bld.style.applyTo(bld.g, ent)
	bld.ents = append(bld.ents, ent)
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
	posd := g.pos.Get(ent)
	rend := g.ren.Get(ent)
	posd.SetPoint(pos)
	rend.SetZ(st.z)
	rend.SetCell(st.r, st.a)
	return ent
}

func (st buildStyle) applyTo(g *game, ent ecs.Entity) {
	ent.SetType(st.t)
	rend := g.ren.Get(ent)
	rend.SetZ(st.z)
	rend.SetCell(st.r, st.a)
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

func isCorner(p image.Point, r image.Rectangle) bool {
	return (p.X == r.Min.X && p.Y == r.Min.Y) ||
		(p.X == r.Min.X && p.Y == r.Max.Y-1) ||
		(p.X == r.Max.X-1 && p.Y == r.Min.Y) ||
		(p.X == r.Max.X-1 && p.Y == r.Max.Y-1)
}

func orthNormal(p image.Point) image.Point {
	if p.X == 0 {
		return image.Pt(1, 0)
	}
	if p.Y == 0 {
		return image.Pt(0, 1)
	}
	return image.ZP
}
