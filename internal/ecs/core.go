package ecs

// ComponentType is a bit field indicating presence of an entity's components.
type ComponentType uint64

const (
	// ComponentNone means "free" or "unallocated" entity.
	ComponentNone ComponentType = 0
)

// All returns true if the type has all of the mask types set.
func (t ComponentType) All(mask ComponentType) bool { return t&mask == mask }

// Any returns true if the type has any of the mask types set.
func (t ComponentType) Any(mask ComponentType) bool { return t&mask != 0 }

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

// Iter returns an iterator that will iterate over all entities with type t
// where t.All(all) is true if all is not 0 and t.Any(any) is true if any is
// not 0.  all of the given components.
func (core Core) Iter(all, any ComponentType) Iterator {
	return Iterator{
		ents: core.Entities,
		all:  all,
		any:  any,
	}
}

// IterAll returns an iterator over entities with all of the given components.
func (core Core) IterAll(mask ComponentType) Iterator { return core.Iter(mask, 0) }

// IterAny returns an iterator over entities with any of the given components.
func (core Core) IterAny(mask ComponentType) Iterator { return core.Iter(0, mask) }

// Iterator iterates over entities of specific type(s) within a Core.
type Iterator struct {
	ents     []ComponentType
	i        int
	all, any ComponentType
}

// Reset resets the iterator, causing it to iterates over again.
func (it *Iterator) Reset() { it.i = 0 }

func (it Iterator) test(t ComponentType) bool {
	if it.all != 0 && !t.All(it.all) {
		return false
	}
	if it.any != 0 && !t.Any(it.any) {
		return false
	}
	return true
}

// Count returns a count of how many entities are yet to come.
func (it Iterator) Count() int {
	ents := it.ents
	n := 0
	for i := it.i; i < len(ents); i++ {
		if it.test(ents[i]) {
			n++
		}
	}
	return n
}

// Next returns the next entity id, its type, and true if there are any
// entities yet to iterate; otherwise false is return with a 0 id and
// ComponentNone type.
func (it *Iterator) Next() (int, ComponentType, bool) {
	ents, i := it.ents, it.i
	for i < len(ents) {
		if it.test(ents[i]) {
			it.i = i + 1
			return i, ents[i], true
		}
		i++
	}
	return 0, ComponentNone, false
}
