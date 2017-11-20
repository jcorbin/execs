package ecs

// Cursor iterates through a Relation.
type Cursor interface {
	Scan() bool
	Count() int
	Entity() Entity
	R() RelationType
	A() Entity
	B() Entity
}

func (rel *Relation) scanLookup(
	tcl TypeClause,
	qids, aids, bids []EntityID,
) Cursor {
	// TODO: if qids is big enough, build a set first
	return rel.Cursor(tcl, func(ent, a, b Entity, r RelationType) bool {
		aid := a.ID()
		for _, id := range qids {
			if id == aid {
				return true
			}
		}
		return false
	})
}

// iterCursor supports iterating over relations; see Relation.iterCursor.
type iterCursor struct {
	rel *Relation

	it    Iterator
	where func(ent, a, b Entity, r RelationType) bool

	ent Entity
	a   Entity
	r   RelationType
	b   Entity
}

// Count scans ahead and returns a count of how many records are to come.
func (cur iterCursor) Count() int {
	if cur.where == nil {
		return cur.it.Count()
	}

	n := 0
	it := cur.it
	for it.Next() {
		ent := it.Entity()
		i := ent.ID() - 1
		r := RelationType(cur.rel.types[i] & ^relType)
		a := cur.rel.aCore.Ref(cur.rel.aids[i])
		b := cur.rel.aCore.Ref(cur.rel.bids[i])
		if cur.where(ent, a, b, r) {
			n++
		}
	}
	return n
}

// Scan advances the cursor return false if the scan is done, true otherwise.
func (cur *iterCursor) Scan() bool {
	for cur.it.Next() {
		cur.ent = cur.it.Entity()
		i := cur.ent.ID() - 1
		cur.r = RelationType(cur.rel.types[i] & ^relType)
		cur.a = cur.rel.aCore.Ref(cur.rel.aids[i])
		cur.b = cur.rel.aCore.Ref(cur.rel.bids[i])
		if cur.where == nil || cur.where(cur.ent, cur.a, cur.b, cur.r) {
			return true
		}
	}
	cur.ent = NilEntity
	cur.r = 0
	cur.a = NilEntity
	cur.b = NilEntity
	return false
}

// Entity returns the current relation entity.
func (cur iterCursor) Entity() Entity { return cur.ent }

// R returns the current relation type.
func (cur iterCursor) R() RelationType { return cur.r }

// A returns the current a-side entity.
func (cur iterCursor) A() Entity { return cur.a }

// B returns the current b-side entity.
func (cur iterCursor) B() Entity { return cur.b }
