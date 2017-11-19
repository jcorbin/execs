package ecs

// Relation contains entities that represent relations between entities in two
// (maybe different) Cores. Users may attach arbitrary data to these relations
// the same way you would with Core.
//
// NOTE: the high Type bit (bit 64) is reserved.
type Relation struct {
	Core
	aCore, bCore *Core
	aids         []EntityID
	bids         []EntityID
	fix          bool
	aix          []int
	bix          []int
}

// TODO: Where interface with WhereFunc convenience (would allow using indices more)
// TODO: secondary indices, uniqueness, keys, etc
// TODO: joins

// NewRelation creates a new relation for the given Core systems.
func NewRelation(aCore, bCore *Core) *Relation {
	rel := &Relation{}
	rel.Init(aCore, bCore)
	return rel
}

// Init initializes the entity relation; useful for embedding.
func (rel *Relation) Init(aCore, bCore *Core) {
	rel.aCore = aCore
	rel.bCore = bCore
	rel.RegisterAllocator(relType, rel.allocRel)
	rel.RegisterDestroyer(relType, rel.destroyRel)
}

// RelationType specified the type of a relation, it's basically a
// ComponentType where the highest bit is reserved.
type RelationType uint64

// All returns true only if all of the masked type bits are set.
func (t RelationType) All(mask RelationType) bool { return t&mask == mask }

// Any returns true only if at least one of the masked type bits is set.
func (t RelationType) Any(mask RelationType) bool { return t&mask != 0 }

const relType ComponentType = 1 << (63 - iota)

// RelType converts a RelationType into a ComponentType.
func RelType(t RelationType) ComponentType {
	return ComponentType(t) | relType
}

func (rel *Relation) allocRel(id EntityID, t ComponentType) {
	i := len(rel.aids)
	rel.aids = append(rel.aids, 0)
	rel.bids = append(rel.bids, 0)
	if rel.aix != nil {
		rel.aix = append(rel.aix, i)
	}
	if rel.bix != nil {
		rel.bix = append(rel.bix, i)
	}
}

func (rel *Relation) destroyRel(id EntityID, t ComponentType) {
	i := int(id) - 1
	rel.aids[i] = 0
	rel.bids[i] = 0
	if !rel.fix {
		if rel.aix != nil {
			fix(len(rel.aix), i, rel.aixLess, rel.aixSwap)
		}
		if rel.bix != nil {
			fix(len(rel.bix), i, rel.bixLess, rel.bixSwap)
		}
	}
}

// DestroyReferencesTo destroys all relations referencing the given ids; either
// id may be 0, in which case an A (resp. B) match is not done.
func (rel *Relation) DestroyReferencesTo(tcl TypeClause, aid, bid EntityID) {
	if aid == 0 && bid == 0 {
		return
	}
	tcl.All |= relType
	dedup := make(map[int]struct{}, len(rel.Entities))
	for i, t := range rel.Entities {
		if tcl.Test(t) {
			if _, seen := dedup[i]; seen {
				continue
			}
			if (aid > 0 && rel.aids[i] == aid) ||
				(bid > 0 && rel.bids[i] == bid) {
				dedup[i] = struct{}{}
				defer rel.Ref(EntityID(i + 1)).Destroy()
			}
		}
	}
}

// Insert relations under the given type clause. TODO: constraints, indices,
// etc.
func (rel *Relation) Insert(r RelationType, a, b Entity) Entity {
	if fixIndex := rel.deferIndexing(); fixIndex != nil {
		// TODO: a bit of over kill for single insertion
		defer fixIndex()
	}
	return rel.insert(r, a, b)
}

// InsertMany allows a function to insert many relations without incurring
// indexing cost; indexing is deferred until the with function returns, at
// which point indices are fixed.
func (rel *Relation) InsertMany(with func(func(r RelationType, a, b Entity) Entity)) {
	if fixIndex := rel.deferIndexing(); fixIndex != nil {
		defer fixIndex()
	}
	with(rel.insert)
}

func (rel *Relation) insert(r RelationType, a, b Entity) Entity {
	aid := rel.aCore.Deref(a)
	bid := rel.bCore.Deref(b)
	ent := rel.AddEntity(ComponentType(r) | relType)
	i := int(ent.ID()) - 1
	rel.aids[i] = aid
	rel.bids[i] = bid
	if rel.aix != nil {
		rel.aix[i] = i
	}
	if rel.bix != nil {
		rel.bix[i] = i
	}
	return ent
}

// Cursor returns a cursor that will scan over relations with given type and
// that meet the optional where clause.
func (rel *Relation) Cursor(
	tcl TypeClause,
	where func(ent, a, b Entity, r RelationType) bool,
) Cursor {
	tcl.All |= relType
	it := rel.Iter(tcl)
	return Cursor{rel: rel, it: it, where: where}
}

// LookupA returns a slice of B entities that are related to the given A
// entities under the given type clause.
func (rel *Relation) LookupA(tcl TypeClause, ids ...EntityID) []EntityID {
	if rel.aix == nil {
		// TODO: warn about falling back to Cursor scan?
		return rel.scanLookup(tcl, ids, rel.aids, rel.bids)
	}
	return rel.indexLookup(tcl, ids, rel.aids, rel.bids, rel.aix)
}

// LookupB returns a slice of A entities that are related to the given B
// entities under the given type clause.
func (rel *Relation) LookupB(tcl TypeClause, ids ...EntityID) []EntityID {
	if rel.aix == nil {
		// TODO: warn about falling back to Cursor scan?
		return rel.scanLookup(tcl, ids, rel.bids, rel.aids)
	}
	return rel.indexLookup(tcl, ids, rel.bids, rel.aids, rel.bix)
}

func (rel *Relation) scanLookup(
	tcl TypeClause,
	qids, aids, bids []EntityID,
) []EntityID {
	// TODO: if qids is big enough, build a set first
	tcl.All |= relType
	it := rel.Iter(tcl)
	rset := make(map[EntityID]struct{}, len(rel.Entities))
	for it.Next() {
		i := it.ID() - 1
		aid := aids[i]
		for _, id := range qids {
			if id == aid {
				rset[bids[i]] = struct{}{}
				break
			}
		}
	}
	result := make([]EntityID, 0, len(rset))
	for id := range rset {
		result = append(result, id)
	}
	return result
}

// Update relations specified by an optional where function and type
// clause. The actual update is performed by the set function, which should
// mutate any secondary data, and return the (possibly modified) relation. If
// either side of the returned relation is now 0, then the relation is
// destroyed.
func (rel *Relation) Update(
	tcl TypeClause,
	where func(ent, a, b Entity, r RelationType) bool,
	set func(ent, a, b Entity, r RelationType) (Entity, Entity),
) {
	if fixIndex := rel.deferIndexing(); fixIndex != nil {
		defer fixIndex()
	}
	cur := rel.Cursor(tcl, where)
	for cur.Scan() {
		ent := cur.Entity()
		oa, ob, or := cur.A(), cur.B(), cur.R()
		na, nb := set(ent, oa, ob, or)
		if na == NilEntity || nb == NilEntity {
			cur.Entity().Destroy()
			continue
		}
		i := ent.ID() - 1
		if na != oa {
			rel.aids[i] = na.ID()
		}
		if nb != ob {
			rel.bids[i] = nb.ID()
		}
	}
}

// Delete all relations matching the given type clause and optional where
// function; this is like Update with a set function that zeros the relation,
// but marginally faster / simpler.
func (rel *Relation) Delete(
	tcl TypeClause,
	where func(ent, a, b Entity, r RelationType) bool,
) {
	if fixIndex := rel.deferIndexing(); fixIndex != nil {
		defer fixIndex()
	}
	cur := rel.Cursor(tcl, where)
	for cur.Scan() {
		cur.Entity().Destroy()
	}
}
