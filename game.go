package main

import (
	"fmt"
	"image"
	"io"
	"log"

	"github.com/jcorbin/anansi"
	"github.com/jcorbin/anansi/ansi"
	"github.com/jcorbin/anansi/x/platform"

	"github.com/jcorbin/execs/internal/ecs"
)

type game struct {
	ag agentSystem

	// TODO shard(s)
	ecs.Scope
	ren render
	pos position

	// generation
	genning bool
	gen     worldGen

	// ui
	view image.Rectangle
	drag dragState
	pop  popup
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

	playerMoveKey     = "playerMove"
	playerCentroidKey = "playerCentroid"
	playerCountKey    = "playerCount"
)

func (g *game) describe(w io.Writer, ent ecs.Entity) {
	describe(w, ent, []descSpec{
		{gameInput, "Ctl", nil},
		{gameCollides, "Col", nil},
		{gamePosition, "Pos", g.describePosition},
		{gameRender, "Ren", g.describeRender},
	})
}

func (g *game) describeRender(ent ecs.Entity) fmt.Stringer   { return g.ren.Get(ent) }
func (g *game) describePosition(ent ecs.Entity) fmt.Stringer { return g.pos.Get(ent) }

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
	MinHallSize: 2,
	MaxHallSize: 8,
	ExitDensity: 25,
}

func newGame() *game {
	g := &game{}

	// TODO better shard construction
	g.pos.Scope = &g.Scope
	g.ren.Scope = &g.Scope

	// TODO better dep coupling
	g.ren.pos = &g.pos
	g.gen.g = g
	g.gen.worldGenConfig = worldConfig

	g.ag.watch(&g.Scope)
	g.ag.registerFunc(g.movePlayers, 0, gamePlayer)
	g.ag.registerFunc(g.spawnPlayers, 1, gameSpawnPoint)

	g.Scope.Watch(gamePosition, 0, &g.pos)
	g.Scope.Watch(gamePosition|gameRender, 0, &g.ren)

	// TODO agent-based gen
	// generate level
	g.gen.init()

	return g
}

func (g *game) Update(ctx *platform.Context) (err error) {
	// Ctrl-C interrupts
	if ctx.Input.HasTerminal('\x03') {
		err = errInt
	}

	// Ctrl-Z suspends
	if ctx.Input.CountRune('\x1a') > 0 {
		defer func() {
			if err == nil {
				err = ctx.Suspend()
			} // else NOTE don't bother suspending, e.g. if Ctrl-C was also present
		}()
	}

	// start/stop generation
	if !g.genning && ctx.Input.CountRune('\x07') > 0 {
		g.genning = true
		log.Printf("starting generation len(q):%v", len(g.gen.q))
	} else if g.genning && err == errInt {
		err = nil
		g.genning = false
		log.Printf("stopping generation")
	}

	// run generation
	if g.genning {
		if !g.gen.elaborate() {
			g.genning = false
			log.Printf("generation done")
		} else {
			log.Printf("gen up to %v entities", g.Scope.Len())
		}
	}

	// process any drag region
	if r := g.drag.process(ctx); r != image.ZR {
		r = r.Canon().Add(g.view.Min)
		n := 0
		for q := g.pos.Within(r); q.Next(); n++ {
			posd := q.handle()
			rend := g.ren.Get(posd.Entity())
			log.Printf("%v %v", posd, rend)
		}
		log.Printf("queried %v entities in %v", n, r)
		g.pop.active = false
	}

	// process control input
	agCtx := nopAgentContext
	if move, interacted := parseTotalMove(ctx.Input); interacted {
		agCtx = addAgentValue(agCtx, playerMoveKey, move)
		g.pop.active = false
	} else if g.drag.active {
		g.pop.active = false
	}

	// run agents
	agCtx, agErr := g.ag.update(agCtx, &g.Scope)
	if err == nil {
		err = agErr
	}

	// center view on player (if any)
	centroid, _ := agCtx.Value(playerCentroidKey).(image.Point)
	g.view = centerView(g.view, centroid, ctx.Output.Size)

	// Ctrl-mouse to inspect entities
	if m, haveMouse := ctx.Input.LastMouse(false); haveMouse && m.State.IsMotion() {
		any := false
		if m.State&ansi.MouseModControl != 0 {
			pq := g.pos.At(m.Point.Add(g.view.Min))
			if pq.Next() {
				any = true
				g.pop.buf.Reset()
				g.pop.buf.Grow(1024)
				g.describe(&g.pop.buf, pq.handle().Entity())
				for pq.Next() {
					_, _ = g.pop.buf.WriteString("\r\n\n")
					g.describe(&g.pop.buf, pq.handle().Entity())
				}
			}
			if any {
				g.pop.processBuf()
				g.pop.setAt(m.Point)
				g.pop.active = true
			} else {
				g.pop.active = false
			}
		}
	}

	ctx.Output.Clear()
	g.ren.drawRegionInto(g.view, &ctx.Output.Grid)

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

func eachCell(g *anansi.Grid, r image.Rectangle, f func(anansi.Cell)) {
	for p := r.Min; p.Y < r.Max.Y; p.Y++ {
		for p.X = r.Min.X; p.X < r.Max.X; p.X++ {
			f(g.Cell(p))
		}
	}
}
