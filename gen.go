package main

import (
	"image"
	"log"
	"math/rand"
	"time"

	"github.com/jcorbin/execs/internal/ecs"
)

type worldGenConfig struct {
	Log bool

	Floor  renderStyle
	Wall   renderStyle
	Door   renderStyle
	Player renderStyle

	RoomSize    image.Rectangle
	MinHallSize int
	MaxHallSize int
	ExitDensity int
}

type worldGen struct {
	rooms *rooms

	builder
	worldGenConfig

	ecs.ArrayIndex
	data []genRoom
	tick int
}

func (gen *worldGen) logf(mess string, args ...interface{}) {
	if gen.Log {
		log.Printf(mess, args...)
	}
}

func (gen *worldGen) EntityCreated(ent ecs.Entity, _ ecs.Type) {
	i := gen.ArrayIndex.Insert(ent)
	for i >= len(gen.data) {
		if i < cap(gen.data) {
			gen.data = gen.data[:i+1]
		} else {
			gen.data = append(gen.data, genRoom{})
		}
	}
	gen.data[i] = genRoom{
		exits: gen.data[i].exits[:0],
		walls: gen.data[i].walls[:0],
	}
}

func (gen *worldGen) Get(ent ecs.Entity) (h genRoomHandle) {
	if i, def := gen.ArrayIndex.Get(ent); def {
		h.load(gen, i, ent.ID)
	}
	return h
}

func (gen *worldGen) GetID(id ecs.ID) (h genRoomHandle) {
	i, def := gen.ArrayIndex.GetID(id)
	if def {
		h.load(gen, i, id)
	}
	return h
}

func (room *genRoomHandle) load(gen *worldGen, i int, id ecs.ID) {
	room.gen = gen
	room.i = i
	room.genRoom = &gen.data[i]
	room.r = gen.rooms.GetID(id)
}

func (gen *worldGen) run() bool {
	const deadline = 3 * time.Millisecond
	t0 := time.Now()
	for i := 0; i < len(gen.data); i++ {
		if gen.data[i].tick >= gen.tick {
			continue
		}
		if id := gen.ArrayIndex.ID(i); id != 0 {
			var room genRoomHandle
			if room.load(gen, i, id); !room.done {
				gen.createRoom(room)
				room.done = true
			} else {
				ok := gen.elaborateRoom(room)
				if ok {
					ok = len(room.exits) < room.maxExits
				}
				if !ok {
					gen.Entity(i).DeleteType(gameGen)
				}
			}
		}
		gen.data[i].tick = gen.tick
		t1 := time.Now()
		if t1.Sub(t0) > deadline {
			return true
		}
	}
	gen.tick++
	return len(gen.data) > 0
}

func (gen *worldGen) create(depth int, enter image.Point, r image.Rectangle) genRoomHandle {
	ent := gen.Scope.Create(gameRoom | gameGen)
	room := gen.GetID(ent.ID)
	if room.gen == nil {
		panic("missing new genRoom data")
	}

	room.depth = depth
	room.enter = enter
	*room.r = r
	gen.logf("gen %v %+v", ent, room)
	return room
}

func (gen *worldGen) createRoom(room genRoomHandle) {
	gen.logf("room @%v", room.r)
	room.maxExits = room.r.Dx() * room.r.Dy() / gen.ExitDensity

	// create room
	gen.reset()
	gen.style = gen.Floor
	gen.fill(room.r.Inset(1))
	gen.style = gen.Wall
	gen.rectangle(*room.r)
	room.collectWalls(gen)

	if room.enter == image.ZP {
		// create spawn in non-enterable rooms
		mid := room.r.Min.Add(room.r.Size().Div(2))
		spawn := gen.g.Create(gameSpawnPoint)
		gen.g.pos.Get(spawn).SetPoint(mid)
	} else {
		// entrance door
		for i, wall := range room.walls {
			if pt := wall.Point(); pt == room.enter {
				copy(gen.built[i:], gen.built[i+1:])
				gen.built = gen.built[:len(gen.built)-1]
				wall.apply(gen.Floor)
				gen.createDoorway(room.enter)
				room.exits = append(room.exits, room.enter)
				break
			}
		}
	}
}

func (gen *worldGen) elaborateRoom(room genRoomHandle) (ok bool) {
	gen.logf("elaborate %v", room.r)
	var pos, dir image.Point
	wall := room.chooseDoorWall(gen)
	if ok = !wall.zero(); ok {
		pos = wall.Point()
		wall.apply(gen.Floor)
		gen.createDoorway(pos)
		room.exits = append(room.exits, pos)
		if pos, dir, ok = gen.buildHallway(room, pos); ok {
			gen.create(room.depth+1, pos, gen.placeNextRoom(pos, dir))
		}
	}
	return ok
}

func (gen *worldGen) placeNextRoom(enter, dir image.Point) image.Rectangle {
	const placeAttempts = 10

	var r image.Rectangle
	for i := 0; ; i++ {
		if i >= placeAttempts {
			r = image.ZR
			break
		}
		r = gen.placeRoom(enter, dir, gen.chooseRoomSize())
		if !gen.anyWithin(r) {
			break
		}
	}
	return r
}

func (gen *worldGen) createDoorway(pt image.Point) renderable {
	gen.logf("doorway @%v", pt)
	door := gen.g.ren.create(pt, gen.Door)
	// TODO set door behavior
	return door
}

func (gen *worldGen) buildHallway(room genRoomHandle, pos image.Point) (_, dir image.Point, _ bool) {
	// TODO hallways with turns

	dir = room.wallNormal(pos)
	n := rand.Intn(gen.MaxHallSize-gen.MinHallSize) + gen.MinHallSize

	gen.logf("hallway dir:%v n:%v", dir, n)
	orth := orthNormal(dir)

	// +1 spot for the landing
	r := image.Rectangle{pos, pos.Add(dir.Mul(n + 2))}.Canon()
	// TODO care about checking for wall cells too?
	for q := gen.g.pos.Within(r); q.Next(); {
		// TODO may care to filter entity type
		// ent := q.handle().Entity()
		return pos, dir, false
	}

	gen.reset()
	for i := 0; i < n; i++ {
		pos = pos.Add(dir)
		gen.style = gen.Floor
		gen.point(pos)
		gen.style = gen.Wall
		gen.point(pos.Add(orth))
		gen.point(pos.Sub(orth))
	}

	// advance to entrance
	pos = pos.Add(dir)

	return pos, dir, true
}

func (gen *worldGen) anyWithin(r image.Rectangle) bool {
	for q := gen.g.pos.Within(r); q.Next(); {
		ent := q.handle().Entity()
		switch ent.Type() {
		case gen.Floor.t, gen.Wall.t, gen.Door.t:
			return true
		}
	}
	return false
}

func (gen *worldGen) chooseRoomSize() image.Point {
	return image.Pt(
		gen.RoomSize.Min.X+rand.Intn(gen.RoomSize.Dx()),
		gen.RoomSize.Min.Y+rand.Intn(gen.RoomSize.Dy()),
	)
}

func (gen *worldGen) placeRoom(enter, dir, sz image.Point) (r image.Rectangle) {
	// TODO better placement
	r.Min = enter
	if dir.Y == 0 {
		if dir.X == -1 {
			r.Min.X -= sz.X - 1
		}
		if d := rand.Intn(sz.Y - 2); d > 0 {
			r.Min.Y -= d
		}
	} else { // dir.X == 0
		if d := rand.Intn(sz.X - 2); d > 0 {
			r.Min.X -= d
		}
		if dir.Y == -1 {
			r.Min.Y -= sz.Y - 1
		}
	}
	r.Max = r.Min.Add(sz)
	return r
}

type genRoomHandle struct {
	gen *worldGen
	i   int

	r *image.Rectangle
	*genRoom
}

type genRoom struct {
	done     bool
	depth    int
	tick     int
	maxExits int
	enter    image.Point
	exits    []image.Point
	walls    []renderable
}

func (room genRoomHandle) wallNormal(p image.Point) (dir image.Point) {
	if p.X == room.r.Min.X {
		dir.X = -1
	} else if p.Y == room.r.Min.Y {
		dir.Y = -1
	} else if p.X == room.r.Max.X-1 {
		dir.X = 1
	} else if p.Y == room.r.Max.Y-1 {
		dir.Y = 1
	}
	return dir
}

func (room genRoomHandle) collectWalls(gen *worldGen) {
	if room.walls == nil {
		room.walls = make([]renderable, 0, len(gen.built))
	}
	for _, wall := range gen.built {
		if wall.Entity().Type() == gen.Wall.t {
			if pt := wall.Point(); !isCorner(pt, *room.r) {
				room.walls = append(room.walls, wall)
			}
		}
	}
}

func (room genRoomHandle) chooseDoorWall(gen *worldGen) (rend renderable) {
	var j int
	for i, wall := range room.walls {
		if !wall.zero() && room.sharesWallWithExit(wall.Point()) {
			continue
		}
		if rend.zero() || rand.Intn(i+1) <= 1 {
			j, rend = i, wall
		}
	}
	if !rend.zero() {
		copy(room.walls[j:], room.walls[j+1:])
		room.walls = room.walls[:len(room.walls)-1]
	}
	return rend
}

func (room genRoomHandle) sharesWallWithExit(p image.Point) bool {
	for j := range room.exits {
		if room.exits[j].X == p.X || room.exits[j].Y == p.Y {
			return true
		}
	}
	return false
}

type builder struct {
	g     *game
	pos   image.Point
	ids   []ecs.ID
	built []renderable

	style renderStyle
}

func (bld *builder) reset() {
	bld.ids = bld.ids[:0]
	bld.built = bld.built[:0]
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

func (bld *builder) point(p image.Point) renderable {
	bld.pos = p
	return bld.create()
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

func (bld *builder) create() renderable {
	rend := bld.g.ren.create(bld.pos, bld.style)
	bld.ids = append(bld.ids, rend.ID())
	bld.built = append(bld.built, rend)
	return rend
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
