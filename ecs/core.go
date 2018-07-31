package ecs

import "fmt"

// Scope is the core of what one might call a "world":
// - it is the frame of reference for entity IDs
// - it owns entity Type definition
// - and supports watching changes in such Entity Type data
type Scope struct {
	typs   []Type
	gens   []uint8
	free   []ID
	watAll []Type
	wats   []Watcher
}

// ID identifies an individual entity under some Scope.
type ID uint64

const (
	idGenMask ID = 0xff00000000000000 // 8-bit generation
	idSeqMask ID = 0x00ffffffffffffff // 56-bit id
)

// String representation of the ID, clearly shows the sequence and generation
// numbers.
func (id ID) String() string {
	gen, seq := id>>56, id&idSeqMask
	if seq == 0 {
		if gen != 0 {
			return fmt.Sprintf("INVALID_ZeroID(gen:%d)", gen)
		}
		return "ZeroID"
	}
	return fmt.Sprintf("%d(gen:%d)", seq, gen)
}

// genseq returns the 8-bit generation number and 56-bit sequence numbers.
func (id ID) genseq() (uint8, uint64) {
	gen, seq := id>>56, id&idSeqMask
	if seq == 0 {
		panic("invalid use of seq-0 ID")
	}
	return uint8(gen), uint64(seq)
}

// setgen returns a copy of the ID with the 8 generations bits replaced with
// the given ones.
func (id ID) setgen(gen uint8) ID {
	seq := id & idSeqMask
	if seq == 0 {
		panic("invalid use of seq-0 ID")
	}
	return seq | (ID(gen) << 56)
}

// Type describe entity component composition: each entity in a Scope has a
// Type value describing which components it currently has; entities only exist
// if they have a non-zero type; each component within a scope must be
// registered with a distinct type bit.
type Type uint64

// Entity is a handle within a Scope's ID space.
type Entity struct {
	Scope *Scope
	ID    ID
}

// Watcher is a stakeholder in Entity's type changes, uses include: component
// data manager (de)allocation and logic systems updating their entity subject
// collections.
type Watcher interface {
	Create(Entity, Type)
	Destroy(Entity, Type)
}

// Watch changes in entity types, calling the given Watcher when all of the
// given bits are destroyed / created. If all is 0 then the Watcher is called
// when any type bits are destroyed/created.
//
// Watcher Create is called when all given bits have been added to an entities
// type; in other words, compound Create watching fires last.
//
// Conversely, Watcher Destroy is called when any of the given "all" bits is
// removed; in other words, compound Destroy watching fires early and often.
//
// If registered with all=0, the Watcher is passed any new/old type bits to
// Create/Destroy; otherwise it's passed the all mask with which it was
// registered.
//
// TODO also support an "any" bitmask?
func (sc *Scope) Watch(all Type, wat Watcher) {
	sc.watAll = append(sc.watAll, all)
	sc.wats = append(sc.wats, wat)
}

// Create a new entity with the given Type, returning a handle to it.
//
// Fires any Watcher's whose all criteria are fully satisfied by the new Type.
func (sc *Scope) Create(newTyp Type) (ent Entity) {
	if newTyp == 0 {
		return ent
	}
	ent.Scope = sc
	if i := len(sc.free) - 1; i >= 0 {
		ent.ID = sc.free[i]
		sc.free = sc.free[:i]
	} else {
		if len(sc.typs) == 0 {
			sc.typs = make([]Type, 1)
			sc.gens = make([]uint8, 1)
		}
		ent.ID = ID(len(sc.typs))
		sc.typs = append(sc.typs, 0)
		sc.gens = append(sc.gens, 0)
	}

	gen, seq := ent.ID.genseq()
	if oldTyp := sc.typs[seq]; oldTyp != 0 {
		panic(fmt.Sprintf("refusing to reuse an entity with non-zero type: %v", oldTyp))
	}
	if gen != sc.gens[seq] {
		panic(fmt.Sprintf("refusing to reuse an entity of generation %v, expected %v", gen, sc.gens[seq]))
	}
	sc.typs[seq] = newTyp
	ent.dispatchCreate(newTyp, newTyp)

	return ent
}

// Destroy the Entity; a convenience for SetType(0).
func (ent Entity) Destroy() bool { return ent.SetType(0) }

// Type returns the type of the entity. Panics if Entity's generation is out of
// sync with Scope's.
func (ent Entity) Type() Type {
	gen, seq := ent.ID.genseq()
	if seq == 0 {
		panic("invalid use of seq-0 ID")
	}
	if gen != ent.Scope.gens[seq] {
		panic(fmt.Sprintf("mis-use of entity of generation %v, expected %v", gen, ent.Scope.gens[seq]))
	}
	return ent.Scope.typs[seq]
}

// Seq returns the Entity's sequence number, validating it and the generation
// number. Component data managers should use this to map internal data
// (directly, indirectly, or otherwise) rather than the raw ID itself.
func (ent Entity) Seq() uint64 {
	gen, seq := ent.ID.genseq()
	if seq == 0 {
		panic("invalid use of seq-0 ID")
	}
	if gen != ent.Scope.gens[seq] {
		panic(fmt.Sprintf("mis-use of entity of generation %v, expected %v", gen, ent.Scope.gens[seq]))
	}
	return seq
}

// SetType updates returns the type of the entity, calling any requisite
// watchers. Panics if Entity's generation is out of sync with Scope's.
//
// Setting the type to 0 will completely destroy the entity, marking its ID for
// future reuse. In a best-effort to prevent use-after-free bugs, the ID's
// generation number is incremented before returning it to the free list,
// invalidating any future use of the prior generation's handle.
func (ent Entity) SetType(newTyp Type) bool {
	if ent.Scope == nil || ent.ID == 0 {
		panic("invalid entity handle")
	}

	gen, seq := ent.ID.genseq()
	if gen != ent.Scope.gens[seq] {
		panic(fmt.Sprintf("mis-use of entity of generation %v, expected %v", gen, ent.Scope.gens[seq]))
	}

	oldTyp := ent.Scope.typs[seq]
	xorTyp := oldTyp ^ newTyp
	if xorTyp == 0 {
		return false
	}

	ent.Scope.typs[seq] = newTyp

	if destroyTyp := oldTyp & xorTyp; destroyTyp != 0 {
		ent.dispatchDestroy(newTyp, destroyTyp)
	}

	if newTyp == 0 {
		gen++
		ent.Scope.gens[seq] = gen // further reuse of this Entity handle should panic
		ent.Scope.free = append(ent.Scope.free, ent.ID.setgen(gen))
		return true
	}

	if createTyp := newTyp & xorTyp; createTyp != 0 {
		ent.dispatchCreate(newTyp, createTyp)
	}

	return true
}

func (ent Entity) dispatchCreate(typ, newTyp Type) {
	for i, all := range ent.Scope.watAll {
		if all == 0 {
			ent.Scope.wats[i].Create(ent, newTyp)
		} else if newTyp&all != 0 && typ&all == all {
			ent.Scope.wats[i].Create(ent, all)
		}
	}
}

func (ent Entity) dispatchDestroy(typ, oldTyp Type) {
	for i, all := range ent.Scope.watAll {
		if all == 0 {
			ent.Scope.wats[i].Destroy(ent, oldTyp)
		} else if oldTyp&all != 0 && typ&all != all {
			ent.Scope.wats[i].Destroy(ent, all)
		}
	}
}
