package ecs

// Entity is a reference to an entity in a Core
type Entity struct {
	co *Core
	id EntityID
}

// NilEntity is the zero of Entity, representing "no entity, in no Core".
var NilEntity = Entity{}

// Type returns the type of the referenced entity, or NoType if the reference
// is empty.
func (ent Entity) Type() ComponentType {
	if ent.co == nil || ent.id == 0 {
		return NoType
	}
	return ent.co.Entities[ent.id-1]
}

// ID returns the ID of the referenced entity; it SHOULD only be called in a
// context where the caller is sure of ownership; when in doubt, use
// Core.Deref(ent) instead.
func (ent Entity) ID() EntityID {
	if ent.co == nil {
		return 0
	}
	return ent.id
}

// Deref unpacks an Entity reference, returning its ID; it panics if the Core
// doesn't own the Entity.
func (co *Core) Deref(e Entity) EntityID {
	if e.co == co {
		return e.id
	} else if e.co == nil {
		panic("nil entity")
	} else {
		panic("foreign entity")
	}
}

// Ref returns an Entity reference to the given ID; it is valid to return a
// reference to the zero entity, to represent "no entity, in this Core" (e.g.
// will Deref() to 0 EntityID).
func (co *Core) Ref(id EntityID) Entity { return Entity{co, id} }

// AddEntity adds an entity to a core, returning an Entity reference; it MAY
// re-use an unused Entities entry (one whose type is still NoType).
//
// Invokes all allocators if it allocates a new EntityID in Entities. Invokes
// any creator functions.
func (co *Core) AddEntity(nt ComponentType) Entity {
	ent := Entity{co, co.allocate()}
	co.Entities[ent.id-1] = nt
	for _, ef := range co.creators {
		if nt.All(ef.t) {
			ef.f(ent.id, nt)
		}
	}
	return ent
}

// Add sets bits in the entity's type, calling any creator functions that are
// newly satisfied by the new type.
func (ent Entity) Add(t ComponentType) {
	old := ent.co.Entities[ent.id-1]
	new := old | t
	ent.co.Entities[ent.id-1] = new
	for _, ef := range ent.co.creators {
		if new.All(ef.t) && !old.All(ef.t) {
			ef.f(ent.id, new)
		}
	}
}

// Delete clears bits in the entity's type, calling any destroyer functions
// that are no longer satisfied by the new type.
func (ent Entity) Delete(t ComponentType) {
	old := ent.co.Entities[ent.id-1]
	new := old & ^t
	ent.co.Entities[ent.id-1] = new
	for _, ef := range ent.co.destroyers {
		if old.All(ef.t) && !new.All(ef.t) {
			ef.f(ent.id, new)
		}
	}
}

// Destroy sets the entity's type to NoType, invoking any destroyer functions
// that match the prior type.`
func (ent Entity) Destroy() {
	old := ent.co.Entities[ent.id-1]
	ent.co.Entities[ent.id-1] = NoType
	for _, ef := range ent.co.destroyers {
		if old.All(ef.t) {
			ef.f(ent.id, NoType)
		}
	}
}

func (co *Core) allocate() EntityID {
	i := 0
	for ; i < len(co.Entities); i++ {
		if co.Entities[i] == NoType {
			return EntityID(i + 1)
		}
	}
	id := EntityID(i + 1)
	co.Entities = append(co.Entities, NoType)
	for _, ef := range co.allocators {
		ef.f(id, NoType)
	}
	return id
}
