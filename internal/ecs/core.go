package ecs

import "fmt"

// Scope is the core of what one might call a "world":
// - it is the frame of reference for entity IDs
// - it owns entity Type definition
// - and supports watching changes in such Entity Type data
type Scope struct {
	typs   []genType
	free   []ID
	watAll []Type
	watAny []Type
	wats   []Watcher
}

type genType struct {
	gen  uint8
	Type Type
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
func (sc *Scope) Watch(all, any Type, wat Watcher) {
	sc.watAll = append(sc.watAll, all)
	sc.watAny = append(sc.watAny, any)
	sc.wats = append(sc.wats, wat)
}

// RemoveWatcher removes a watcher from any/all Types registered by Watch.
func (sc *Scope) RemoveWatcher(wat Watcher) {
	j := 0
	for i := 0; i < len(sc.wats); i++ {
		if sc.wats[i] == wat {
			continue
		}
		if i != j {
			sc.watAll[j] = sc.watAll[i]
			sc.watAny[j] = sc.watAny[i]
			sc.wats[j] = sc.wats[i]
		}
		j++
	}
	sc.watAll = sc.watAll[:j]
	sc.watAny = sc.watAny[:j]
	sc.wats = sc.wats[:j]
}

// Create a new entity with the given Type, returning a handle to it.
//
// Fires any Watcher's whose all criteria are fully satisfied by the new Type,
// and whose any criteria (if non-zero) are have at least one bit satisfied.
func (sc *Scope) Create(newType Type) (ent Entity) {
	if newType != 0 {
		ent = Entity{sc, sc.create()}
		typ, seq := ent.typ()
		if typ.Type != 0 {
			panic(fmt.Sprintf("refusing to reuse an entity with non-zero type: %v", typ))
		}
		sc.typs[seq].Type = newType
		ent.dispatchCreate(newType, newType)
	}
	return ent
}

func (sc *Scope) create() ID {
	if i := len(sc.free) - 1; i >= 0 {
		id := sc.free[i]
		sc.free = sc.free[:i]
		return id
	}
	if len(sc.typs) == 0 {
		sc.typs = append(sc.typs, genType{0xff, 0})
	}
	sc.typs = append(sc.typs, genType{})
	return ID(len(sc.typs) - 1)
}

// Destroy the Entity; a convenience for SetType(0).
func (ent Entity) Destroy() bool {
	return ent.SetType(0)
}

func (ent Entity) typ() (genType, uint64) {
	gen, seq := ent.ID.genseq()
	if seq == 0 {
		panic("invalid use of seq-0 ID")
	}
	typ := ent.Scope.typs[seq]
	if gen != typ.gen {
		panic(fmt.Sprintf("mis-use of entity of generation %v, expected %v", gen, typ.gen))
	}
	return typ, seq
}

// Type returns the type of the entity. Panics if Entity's generation is out of
// sync with Scope's.
func (ent Entity) Type() Type {
	typ, _ := ent.typ()
	return typ.Type
}

// Seq returns the Entity's sequence number, validating it and the generation
// number. Component data managers should use this to map internal data
// (directly, indirectly, or otherwise) rather than the raw ID itself.
func (ent Entity) Seq() uint64 {
	_, seq := ent.typ()
	return seq
}

// SetType updates returns the type of the entity, calling any requisite
// watchers. Panics if Entity's generation is out of sync with Scope's.
//
// Setting the type to 0 will completely destroy the entity, marking its ID for
// future reuse. In a best-effort to prevent use-after-free bugs, the ID's
// generation number is incremented before returning it to the free list,
// invalidating any future use of the prior generation's handle.
func (ent Entity) SetType(newType Type) bool {
	if ent.Scope == nil || ent.ID == 0 {
		panic("invalid entity handle")
	}
	priorTyp, seq := ent.typ()

	typeChange := priorTyp.Type ^ newType
	if typeChange == 0 {
		return false
	}

	ent.Scope.typs[seq].Type = newType

	if destroyTyp := priorTyp.Type & typeChange; destroyTyp != 0 {
		ent.dispatchDestroy(newType, destroyTyp)
	}

	if newType == 0 {
		gen := priorTyp.gen + 1
		ent.Scope.typs[seq].gen = gen // further reuse of this Entity handle should panic
		ent.Scope.free = append(ent.Scope.free, ent.ID.setgen(gen))
		return true
	}

	if createTyp := newType & typeChange; createTyp != 0 {
		ent.dispatchCreate(newType, createTyp)
	}

	return true
}

func (ent Entity) dispatchCreate(newType, createdType Type) {
	for i := 0; i < len(ent.Scope.watAll); i++ {
		all := ent.Scope.watAll[i]
		any := ent.Scope.watAny[i]
		if (all == 0 || (newType&all == all && createdType&all != 0)) &&
			(any == 0 || createdType&any != 0) {
			ent.Scope.wats[i].Create(ent, createdType)
		}
	}
}

func (ent Entity) dispatchDestroy(newType, destroyedType Type) {
	for i := 0; i < len(ent.Scope.watAll); i++ {
		all := ent.Scope.watAll[i]
		any := ent.Scope.watAny[i]
		if (all == 0 || (newType&all != all && destroyedType&all != 0)) &&
			(any == 0 || destroyedType&any != 0) {
			ent.Scope.wats[i].Destroy(ent, destroyedType)
		}
	}
}
