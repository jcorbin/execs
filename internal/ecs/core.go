package ecs

// ComponentType is a bit field indicating presence of an entity's components.
type ComponentType uint64

const (
	// ComponentNone means "free" or "unallocated" entity.
	ComponentNone ComponentType = 0
)

// Core is the core of the Entity Component System, tracking the entities in
// play. It is meant to be embedded in the root of a collection of game data.
type Core struct {
	Entities []ComponentType

	// TODO: type registration, alloc/free hooks
}

// CountAll counts how many entities have all of the given components.
func (core Core) CountAll(mask ComponentType) int {
	n := 0
	for _, t := range core.Entities {
		if t&mask == mask {
			n++
		}
	}
	return n
}

// CountAny counts how many entities have at least one of the given components.
func (core Core) CountAny(mask ComponentType) int {
	n := 0
	for _, t := range core.Entities {
		if t&mask != 0 {
			n++
		}
	}
	return n
}

// IterAll returns an iterator over entities with all of the given components.
func (core Core) IterAll(mask ComponentType) Iterator {
	return Iterator{
		ents:   core.Entities,
		filter: allTypeFilter(mask),
	}
}

// IterAny returns an iterator over entities with any of the given components.
func (core Core) IterAny(mask ComponentType) Iterator {
	return Iterator{
		ents:   core.Entities,
		filter: anyTypeFilter(mask),
	}
}

type typeFilter interface {
	test(ComponentType) bool
}

type allTypeFilter ComponentType
type anyTypeFilter ComponentType

func (t allTypeFilter) test(ot ComponentType) bool { return ot&ComponentType(t) == ComponentType(t) }
func (t anyTypeFilter) test(ot ComponentType) bool { return ot&ComponentType(t) != 0 }

// Iterator iterates over entities of specific type(s) within a Core.
type Iterator struct {
	ents   []ComponentType
	i      int
	filter typeFilter
}

// Reset resets the iterator, causing it to iterates over again.
func (it *Iterator) Reset() { it.i = 0 }

// Count returns a count of how many entities are yet to come.
func (it *Iterator) Count() int {
	ents, test := it.ents, it.filter.test
	n := 0
	for i := it.i; i < len(ents); i++ {
		if test(ents[i]) {
			n++
		}
	}
	return n
}

// Next returns the next entity id, its type, and true if there are any
// entities yet to iterate; otherwise false is return with a 0 id and
// ComponentNone type.
func (it *Iterator) Next() (int, ComponentType, bool) {
	ents, test, i := it.ents, it.filter.test, it.i
	for i < len(ents) {
		if test(ents[i]) {
			it.i = i + 1
			return i, ents[i], true
		}
		i++
	}
	return 0, ComponentNone, false
}
