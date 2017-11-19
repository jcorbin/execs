package ecs

// Core is the core of an Entity Component System: it manages the entity IDs
// and types. IDs are off-by-one indices in the Entities slice (since the 0 ID
// is invalid, ID 1 maps to Entities[0]). Types are simply the values in the slice
type Core struct {
	Entities []ComponentType

	allocators, creators, destroyers []entityFunc
}

type entityFunc struct {
	t ComponentType
	f func(EntityID, ComponentType)
}

// EntityID is the ID of an Entity in a Core; the 0 value is an invalid ID,
// meaning "null entity".
type EntityID int

// ComponentType represents the type of an Entity in a Core.
type ComponentType uint64

// NoType means that the Entities slot is unused.
const NoType ComponentType = 0

// All returns true only if all of the masked type bits are set. If the mask is
// NoType, always returns false.
func (t ComponentType) All(mask ComponentType) bool { return mask != NoType && t&mask == mask }

// Any returns true only if at least one of the masked type bits is set. If the
// mask is NoType, always returns true.
func (t ComponentType) Any(mask ComponentType) bool { return mask == NoType || t&mask != 0 }

// Empty returns true only if there are no active entities.
func (co *Core) Empty() bool {
	for _, t := range co.Entities {
		if t != NoType {
			return false
		}
	}
	return true
}

// Clear destroys all active entities.
func (co *Core) Clear() {
	for i, t := range co.Entities {
		if t != NoType {
			co.setType(EntityID(i+1), NoType)
		}
	}
}

// RegisterAllocator registers an allocator function; it panics if any
// allocator is registered that overlaps the given type.
//
// Allocators are called when the Core grows its entity capacity. An allocator
// must create space in each of its data collections so that the given id has
// corresponding element(s).
func (co *Core) RegisterAllocator(t ComponentType, allocator func(EntityID, ComponentType)) {
	for _, ef := range co.allocators {
		if ef.t.Any(t) {
			panic("aspect type conflict")
		}
	}
	co.allocators = append(co.allocators, entityFunc{t, allocator})
}

// RegisterCreator registers a creator function. The Type may overlap any
// number of other creator Types, so each should be written cooperatively.
//
// Creators are called when an Entity has all of its Type bits added to it;
// they may initialize static data, allocate dynamic data, or do other Type
// specific things.
//
// Any creators registered against NoType trigger simply at entity creation
// time; they will be called when an entity transitions from NoType to any
// arbitrary type. NOTE: this may or may not be proximate to allocation time!
func (co *Core) RegisterCreator(t ComponentType, creator func(EntityID, ComponentType)) {
	co.creators = append(co.creators, entityFunc{t, creator})
}

// RegisterDestroyer registers a destroyer function. The Type may overlap any
// number of other destroyer Types, so each should be written cooperatively.
//
// Destroyers are called when an Entity has any of its Type bits removed from
// it; they may clear static data, de-allocate dynamic data, or do other Type
// specific things. NOTE: destroyers must not de-allocate static data.
//
// Any destroyers registered against NoType trigger at entity deletion time;
// they will be called when an entity transitions to NoType.
func (co *Core) RegisterDestroyer(t ComponentType, destroyer func(EntityID, ComponentType)) {
	co.destroyers = append(co.destroyers, entityFunc{t, destroyer})
}
