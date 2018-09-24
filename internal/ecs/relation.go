package ecs

// EntityRelation relates entities within two (maybe the same) Scope.
// Relations themselves are entities within the Scope of an EntityRelation.
// Related entity IDs are component data within an EntityRelation.
// Domain specific data may be added to a relation by embedding EntityRelation
// and adding component data around it.
type EntityRelation struct {
	A, B *Scope

	Scope

	// uses direct indexing into
	aid []ID
	bid []ID

	aindex map[ID][]ID // a ID -> relation IDs
	bindex map[ID][]ID // b ID -> relation IDs
}

// EntityRelation type constants.
const (
	TypeEntityRelation Type = 1 << iota
)

// Init ialize an EntityRelation between the given two scopes.
// If B is nil or equal to A, the outcome is the same: an auto-relation between
// entities within the same scope (i.e. a graph).
func (er *EntityRelation) Init(A, B *Scope) {
	if er.A != nil || er.B != nil {
		panic("invalid EntityRelation re-initialization")
	}
	if A == nil {
		panic("must provide an A relation ")
	}

	er.Scope.Watch(TypeEntityRelation, 0, er)

	er.A = A
	er.A.Watch(0, 0, EntityDestroyedFunc(er.onADestroyed))
	er.aindex = make(map[ID][]ID)

	if B == nil || B == A {
		er.B = nil
		er.bindex = nil
	} else {
		er.B = B
		er.B.Watch(0, 0, EntityDestroyedFunc(er.onBDestroyed))
		er.bindex = make(map[ID][]ID)
	}

}

func (er *EntityRelation) onADestroyed(ae Entity, _ Type) { er.DeleteA(ae) }
func (er *EntityRelation) onBDestroyed(be Entity, _ Type) { er.DeleteB(be) }

// EntityCreated allocates and clears ID storage space for the given relation
// entity.
func (er *EntityRelation) EntityCreated(re Entity, _ Type) {
	i := int(re.Seq())
	for i >= len(er.aid) {
		if i < cap(er.aid) {
			er.aid = er.aid[:i+1]
		} else {
			er.aid = append(er.aid, 0)
		}
	}
	er.aid[i] = 0
	for i >= len(er.bid) {
		if i < cap(er.bid) {
			er.bid = er.bid[:i+1]
		} else {
			er.bid = append(er.bid, 0)
		}
	}
	er.bid[i] = 0
}

// EntityDestroyed clears any stored IDs for the given relation entity.
func (er *EntityRelation) EntityDestroyed(re Entity, _ Type) {
	i := re.Seq()
	aid := er.aid[i]
	bid := er.bid[i]
	er.aid[i] = 0
	er.bid[i] = 0
	er.aindex[aid] = withoutID(er.aindex[aid], re.ID)
	if er.B == nil {
		er.aindex[aid] = withoutID(er.aindex[bid], re.ID)
	} else {
		er.bindex[aid] = withoutID(er.bindex[bid], re.ID)
	}
	// TODO support cascading destroy
}

// Insert creates a relation entity between the given A and B entities.
// The typ argument may provide additional type bits when being used as part of
// a larger EntityRelation-embedding struct.
func (er *EntityRelation) Insert(aid, bid ID, typ Type) Entity {
	re := er.Scope.Create(TypeEntityRelation | typ)
	i := re.Seq()
	er.aid[i] = aid
	er.bid[i] = bid
	er.aindex[aid] = append(er.aindex[aid], re.ID)
	if er.B != nil {
		er.bindex[bid] = append(er.bindex[bid], re.ID)
	} else if bid != aid {
		er.aindex[bid] = append(er.aindex[bid], re.ID)
	}
	return re
}

// DeleteA destroys any relation entities associated with the given A-side entity.
func (er *EntityRelation) DeleteA(ae Entity) {
	if ae.Scope != er.A {
		panic("invalid A-side entity")
	}
	ids := er.aindex[ae.ID]
	delete(er.aindex, ae.ID)
	for _, id := range ids {
		Ent(&er.Scope, id).Destroy()
	}
}

// DeleteB destroys any relation entities associated with the given B-side entity.
func (er *EntityRelation) DeleteB(be Entity) {
	if er.B == nil {
		er.DeleteA(be)
		return
	}
	if be.Scope != er.B {
		panic("invalid B-side entity")
	}
	ids := er.bindex[be.ID]
	delete(er.bindex, be.ID)
	for _, id := range ids {
		Ent(&er.Scope, id).Destroy()
	}
}

// LookupA returns the set of relation entities for a given A-side entity.
func (er *EntityRelation) LookupA(aid ID) Entities {
	return Entities{&er.Scope, er.aindex[aid]}
}

// LookupB returns the set of relation entities for a given B-side entity.
func (er *EntityRelation) LookupB(bid ID) Entities {
	if er.B == nil {
		return Entities{&er.Scope, er.aindex[bid]}
	}
	return Entities{&er.Scope, er.bindex[bid]}
}
