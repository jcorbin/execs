package main

import "sync"

type Entity struct {
	sync.Mutex
	Components map[SystemID][]ComponentID
	Children   []Entity
	Parent     *Entity
}

type ComponentID int
type SystemID int

type System interface {
	SetID(SystemID)
	Destroy(ComponentID)
}

func (ent *Entity) AddComponent(
	sys System,
)

type ECS struct {
	Systems map[SystemID]System
}

func (ent *Ent) AddCopmonent(sid SystemID, cid ComponentID) func() {
	if sid == 0 {
		panic("unknown system")
	}
	ent.Lock()
	ent.Components[sid] = append(ent.Components[sid], cid)
	return ent.Unlock
}

func (ent *Ent) AddUniqueCopmonent(sid SystemID, cid ComponentID) (func(), bool) {
	if sid == 0 {
		panic("unknown system")
	}
	ent.Lock()
	cs := ent.Components[sid]
	if len(cs) > 0 {
		return ent.Unlock, false
	}
	ent.Components[sid] = append(cs, cid)
	return ent.Unlock, true
}

func (ent *Ent) RemoveCopmonent(sid SystemID, cid ComponentID) func() {
	if sid == 0 {
		panic("unknown system")
	}
	ent.Lock()
	cs := ent.Components[sid]
	for i := 0; i < len(cs); i++ {
		if cs[i] == cid {
			if j := i + 1; j < len(cs) {
				copy(cs[i:], cs[j:])
			}
			cs = cs[:len(cs)-1]
			i--
		}
	}
	ent.Components[sid] = cs[:i]
	return ent.Unlock
}

func (ecs ECS) AddSystem(sys System) {
	if ecs.Systems == nil {
		ecs.Systems = make(map[SystemID]System)
	}
	sid := SystemID(len(ecs.Systems) + 1)
	sys.SetID(sid)
	ecs.Systems[sid] = sys
}
