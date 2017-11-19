package ecs

// Cursor supports iterating over relations; see Relation.Cursor.
type Cursor struct {
	rel *Relation

	it    Iterator
	where func(ent, a, b Entity, r RelationType) bool

	ent Entity
	a   Entity
	r   RelationType
	b   Entity
}

// Count scans ahead and returns a count of how many records are to come.
func (cur Cursor) Count() int {
	if cur.where == nil {
		return cur.it.Count()
	}

	n := 0
	it := cur.it
	for it.Next() {
		ent := it.Entity()
		i := ent.ID() - 1
		r := RelationType(cur.rel.Entities[i] & ^relType)
		a := cur.rel.aCore.Ref(cur.rel.aids[i])
		b := cur.rel.aCore.Ref(cur.rel.bids[i])
		if cur.where(ent, a, b, r) {
			n++
		}
	}
	return n
}

// Scan advances the cursor return false if the scan is done, true otherwise.
func (cur *Cursor) Scan() bool {
	for cur.it.Next() {
		cur.ent = cur.it.Entity()
		i := cur.ent.ID() - 1
		cur.r = RelationType(cur.rel.Entities[i] & ^relType)
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
func (cur Cursor) Entity() Entity { return cur.ent }

// R returns the current relation type.
func (cur Cursor) R() RelationType { return cur.r }

// A returns the current a-side entity.
func (cur Cursor) A() Entity { return cur.a }

// B returns the current b-side entity.
func (cur Cursor) B() Entity { return cur.b }
