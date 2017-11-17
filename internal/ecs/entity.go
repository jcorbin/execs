package ecs

// Entity is an opaque reference to an entity in an Core. It is primarily meant
// for external user convenience, and should not be used within systems.
type Entity struct {
	core *Core
	id   int
}

// InvalidEntity represents an invalid entity, the zero value.
var InvalidEntity = Entity{}

// AddComponent adds one or more component type to the entity (if it doesn't
// already have them).
func (e Entity) AddComponent(t ComponentType) { e.core.Entities[e.id] |= t }

// RemoveComponent removes a component type to the entity.
func (e Entity) RemoveComponent(t ComponentType) { e.core.Entities[e.id] &= ^t }

// Has tests if the entity has the component(s).
func (e Entity) Has(t ComponentType) bool { return (e.core.Entities[e.id] & t) == t }

// ID returns the entity's id; this is its index within first-level Core data
// collections.
func (e Entity) ID() int { return e.id }

// Type returns the entity's type (the combination of all its component types).
func (e Entity) Type() ComponentType {
	return e.core.Entities[e.id]
}

// AddEntity makes room for a new entity in the Core; however the entity's type
// is still ComponentNone upon return. The user should add one or more
// components to the entity after calling AddEntity.
//
// The user should not retain the Entity value after using it to construct the
// new entity; doing so is code smell for missing system code.
func (core *Core) AddEntity() Entity {
	ent := Entity{core, len(core.Entities)}
	// TODO: re-use
	core.Entities = append(core.Entities, ComponentNone)
	return ent
}

// Owns returns true if the given entity belongs to this Core.
func (core *Core) Owns(ent Entity) bool { return ent.core == core }

// Ref returns an entity reference for the given id.
func (core *Core) Ref(id int) Entity { return Entity{core: core, id: id} }
