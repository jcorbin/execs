package ecs

// Iter returns a new iterator over the Core's Entities. If all is non-0, then
// the iterator is limeted to entities that have all of those type bits set.
// Similarly if any non-0, then the iterator is limited to entities that at
// least one of those type bits set.
func (co *Core) Iter(all, any ComponentType) Iterator { return Iterator{co, 0, all, any} }

// IterAll is a convenient way of saying Iter(all, 0).
func (co *Core) IterAll(all ComponentType) Iterator { return Iterator{co, 0, all, 0} }

// IterAny is a convenient way of saying Iter(0, any).
func (co *Core) IterAny(any ComponentType) Iterator { return Iterator{co, 0, 0, any} }

// Iterator points into a Core's Entities, iterating over them with optional
// type filter criteria.
type Iterator struct {
	co  *Core
	i   int
	all ComponentType
	any ComponentType
}

// Next advances the iterator to point at the next matching entity, and
// returns true if such an entity was found; otherwise iteration is done, and
// false is returned.
func (it *Iterator) Next() bool {
	for ; it.i < len(it.co.Entities); it.i++ {
		t := it.co.Entities[it.i]
		if it.all != 0 && !t.All(it.all) {
			continue
		}
		if it.any != 0 && !t.Any(it.any) {
			continue
		}
		return true
	}
	return false
}

// Reset resets the iterator, causing it to start over.
func (it *Iterator) Reset() { it.i = 0 }

// Count counts how many entities remain to be iterated, without advancing the
// iterator.
func (it Iterator) Count() int {
	n := 0
	for it.Next() {
		n++
	}
	return n
}

// Type returns the type of the current entity, or NoType if iteration is
// done.
func (it Iterator) Type() ComponentType {
	if it.i < len(it.co.Entities) {
		return it.co.Entities[it.i]
	}
	return NoType
}

// ID returns the type of the current entity, or 0 if iteration is done.
func (it Iterator) ID() EntityID {
	if it.i < len(it.co.Entities) {
		return EntityID(it.i + 1)
	}
	return 0
}

// Entity returns a reference to the current entity, or NilEntity if
// iteration is done.
func (it Iterator) Entity() Entity {
	if it.i < len(it.co.Entities) {
		return Entity{it.co, EntityID(it.i + 1)}
	}
	return NilEntity
}
