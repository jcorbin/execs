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
