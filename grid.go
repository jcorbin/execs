package main

type point struct {
	x, y int
}

type gridTile struct {
	pos   point
	shape point
	data  []uint8
}

type gridSystem struct {
	sid             SystemID
	tiles           gridTile // XXX quad-tree
	entityPositions []point
	entities        []*Entity
}

func (gs gridSystem) SetID(sid SystemID) {
	if gs.sid != 0 {
		panic("gridSystem ID already set")
	}
	gs.sid = sid
}

func (gs *gridSystem) Destroy(cid ComponentID) {
	if j := int(cid) + 1; j < len(gs.entities) {
		copy(gs.entities[i:], gs.entities[j:])
		copy(gs.entityPositions[i:], gs.entityPositions[j:])
	}
	j := len(gs.entities) - 1
	gs.entities = gs.entities[:j]
	gs.entityPositions = gs.entityPositions[:j]
}

func (gs *gridSystem) AddPosition(ent *Entity, pt point) {
	cid := ComponentID(len(gs.entities))
	unlock, ok := ent.AddUniqueCopmonent(gs.sid, cid)
	defer unlock()
	if ok {
		gs.entities = append(gs.entities, ent)
		gs.entityPositions = append(gs.entityPositions, pt)
	}
}
