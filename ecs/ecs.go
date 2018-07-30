package ecs

// Scope is the core of what one might call a "world":
// - it is the frame of reference for entity IDs
// - TODO types
// - TODO component managers
// - TODO systems
type Scope struct {
	typs []Type
	free []ID

	comp [64]Component

	mats []Type
	wats []Watcher
}

// ID identifies an individual entity under some Scope.
type ID uint64

// Type describe entity component composition: each entity in a Scope has a
// Type value describing which components it currently has; entities only exist
// if they have a non-zero type; each component within a scope must be
// registered with a distinct type bit.
type Type uint64

type Entity struct {
	Scope *Scope
	ID    ID
}

// TODO what's the difference between a Component and a Watcher really?

type Component interface {
	Create(Entity)
	Destroy(Entity)
}

type Watcher interface {
	Created(Entity)
	Destroyed(Entity)
}

func NewScope(n int) *Scope {
	if n <= 0 {
		n = 1024
	}
	var sc Scope
	sc.typs = make([]Type, 1, 1+n)
	sc.free = make([]ID, 0, n)
	return &sc
}

func (sc *Scope) AddComponent(co Component) Type {
	typ := Type(1)
	for i := 0; i < len(sc.comp); i++ {
		if sc.comp[i] == nil {
			sc.comp[i] = co
			return typ
		}
		typ <<= 1
	}
	panic("cannot add component, scope full")
}

func (sc *Scope) AddWatcher(mat Type, wat Watcher) {
	sc.mats = append(sc.mats, mat)
	sc.wats = append(sc.wats, wat)
}

func (sc *Scope) Create(Type) Entity
func (ent Entity) SetType(Type) bool
func (ent Entity) Destroy() bool

//// core entity tracking logic for system implementors; just a starting place,
//   some systems may care to maintain particular component-derived invariants
//   over their entity collections, and so would need to implement Watcher
//   directly.

type Entities struct {
	Scope *Scope
	IDs   []ID
}

type ManyEntities map[*Scope][]ID

func (es *Entities) Created(ent Entity) {
	if es.Scope == ent.Scope {
		es.IDs = append(es.IDs, ent.ID)
	}
}
func (es *Entities) Destroyed(ent Entity) {
	if es.Scope == ent.Scope {
		es.IDs = removeID(es.IDs, ent.ID)
	}
}

func (me ManyEntities) Created(ent Entity) { me[ent.Scope] = append(me[ent.Scope], ent.ID) }
func (me ManyEntities) Destroyed(ent Entity) {
	if ids := me[ent.Scope]; len(ids) > 0 {
		me[ent.Scope] = removeID(ids, ent.ID)
	}
}

func removeID(ids []ID, id ID) []ID {
	for i := range ids {
		if ids[i] == id {
			if j := i + 1; j < len(ids) {
				return ids[:i+copy(ids[i:], ids[j:])]
			}
			return ids[:i]
		}
	}
	return ids
}
