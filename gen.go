package main

import (
	"image"
	"log"
	"math/rand"

	"github.com/jcorbin/execs/internal/ecs"
)

type worldGenConfig struct {
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
	builder
	worldGenConfig

	atFloor ecs.Entity
	atWall  ecs.Entity
	atDoor  ecs.Entity

	q []genRoom
}

func (gen *worldGen) init() {
	if len(gen.q) > 0 {
		gen.q = gen.q[:0]
	}
	room := genRoom{r: image.Rectangle{image.ZP, gen.chooseRoomSize()}}
	log.Printf("init %v", room.r)
	room.create(gen, image.ZP)
	gen.q = append(gen.q, room)
}

func (gen *worldGen) elaborate() bool {
	if len(gen.q) == 0 {
		return false
	}
	room := gen.q[0]
	gen.q = gen.q[:copy(gen.q, gen.q[1:])]
	room.elaborate(gen)
	return true
}

func (room genRoom) elaborate(gen *worldGen) {
	const placeAttempts = 10

	log.Printf("elaborate %v", room.r)
	// choose and build exit door
	doorway := room.chooseDoorWall(gen)
	if doorway.zero() {
		return
	}
	exit := doorway.Point()
	pos, dir, clear := room.hallway(gen, exit)
	if !clear {
		return
	}
	gen.doorway(doorway, exit)
	room.exits = append(room.exits, exit)

	// record exit
	if len(room.exits) < cap(room.exits) {
		// room can be further elaborated
		gen.q = append(gen.q, room)
	}

	// entrance clear?
	enter := pos.Add(dir)
	if gen.at(enter) {
		// otherwise, cap hallway. TODO maybe doorway back into a room.
		gen.fillWallAt()
		return
	}

	// place and create next room
	room = genRoom{depth: room.depth + 1}
	for i := 0; ; i++ {
		if i >= placeAttempts {
			room.r = image.ZR
			break
		}
		room.r = gen.placeRoom(enter, dir, gen.chooseRoomSize())
		if !gen.anyWithin(room.r) {
			break
		}
	}
	room.create(gen, enter)

	// further elaborate if large enough
	if len(room.exits) < cap(room.exits) {
		gen.q = append(gen.q, room)
	}
}

func (room *genRoom) create(gen *worldGen, enter image.Point) {
	// create room
	gen.room(room)
	if maxExits := room.r.Dx() * room.r.Dy() / gen.ExitDensity; maxExits > 1 {
		room.exits = make([]image.Point, 0, maxExits)
	}

	if enter == image.ZP {
		// create spawn in non-enterable rooms
		mid := room.r.Min.Add(room.r.Size().Div(2))
		gen.g.pos.Get(gen.g.Create(gameSpawnPoint)).SetPoint(mid)
	} else {
		// entrance door
		for i, wall := range room.walls {
			if pt := wall.Point(); pt == enter {
				copy(gen.built[i:], gen.built[i+1:])
				gen.built = gen.built[:len(gen.built)-1]
				gen.doorway(wall, enter)
				room.exits = append(room.exits, enter)
				break
			}
		}
	}
}

func (room *genRoom) hallway(gen *worldGen, pos image.Point) (_, dir image.Point, clear bool) {
	dir = room.wallNormal(pos)
	n := rand.Intn(gen.MaxHallSize-gen.MinHallSize) + gen.MinHallSize
	orth := orthNormal(dir)
	log.Printf("hallway dir:%v n:%v", dir, n)

	gen.reset()
	for i := 0; i < n; i++ {
		pos = pos.Add(dir)
		if gen.at(pos) {
			return pos, dir, false
		}

		gen.style = gen.Floor
		gen.point(pos)

		// TODO deconflict?

		gen.style = gen.Wall
		gen.point(pos.Add(orth))
		gen.point(pos.Sub(orth))
	}
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

func (gen *worldGen) fillWallAt() {
	if gen.atDoor != ecs.ZE {
		gen.atDoor.SetType(0)
	}
	if gen.atFloor != ecs.ZE && gen.atWall == ecs.ZE {
		gen.g.ren.Get(gen.atFloor).apply(gen.Wall)
		gen.atFloor, gen.atWall = ecs.ZE, gen.atFloor
	}
}

func (gen *worldGen) room(room *genRoom) {
	log.Printf("room @%v", room.r)
	gen.reset()
	gen.style = gen.Floor
	gen.fill(room.r.Inset(1))
	gen.style = gen.Wall
	gen.rectangle(room.r)
	room.collectWalls(gen)
}

func (gen *worldGen) at(p image.Point) (any bool) {
	for q := gen.g.pos.At(p); q.Next(); {
		ent := q.handle().Entity()
		switch ent.Type() {
		case gen.Floor.t:
			gen.atFloor = ent
			any = true
		case gen.Wall.t:
			gen.atWall = ent
			any = true
		case gen.Door.t:
			gen.atDoor = ent
			any = true
		}
	}
	return any
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

func (gen *worldGen) doorway(floor renderable, p image.Point) renderable {
	log.Printf("doorway @%v", p)
	floor.apply(gen.Floor)
	door := gen.g.ren.create(p, gen.Door)
	// TODO set door behavior
	return door
}

type genRoom struct {
	depth int
	r     image.Rectangle
	exits []image.Point
	walls []renderable
}

func (room *genRoom) wallNormal(p image.Point) (dir image.Point) {
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

func (room *genRoom) collectWalls(gen *worldGen) {
	if room.walls == nil {
		room.walls = make([]renderable, 0, len(gen.built))
	}
	for _, wall := range gen.built {
		if wall.Entity().Type() == gen.Wall.t {
			if pt := wall.Point(); !isCorner(pt, room.r) {
				room.walls = append(room.walls, wall)
			}
		}
	}
}

func (room *genRoom) chooseDoorWall(gen *worldGen) (rend renderable) {
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

func (room *genRoom) sharesWallWithExit(p image.Point) bool {
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
	built []renderable

	style renderStyle
}

func (bld *builder) reset() {
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
	rend := bld.g.ren.create(bld.pos, bld.style)
	bld.built = append(bld.built, rend)
	return rend
}

func (bld *builder) fill(r image.Rectangle) {
	for bld.moveTo(r.Min); bld.pos.Y < r.Max.Y; bld.pos.Y++ {
		for bld.pos.X = r.Min.X; bld.pos.X < r.Max.X; bld.pos.X++ {
			rend := bld.g.ren.create(bld.pos, bld.style)
			bld.built = append(bld.built, rend)
		}
	}
}

func (bld *builder) lineTo(p image.Point, n int) {
	for i := 0; i < n; i++ {
		rend := bld.g.ren.create(bld.pos, bld.style)
		bld.built = append(bld.built, rend)
		bld.pos = bld.pos.Add(p)
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
