package ecs

import "sort"

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

// NewRelation creates a new relation for the given Core systems.
func NewRelation(aCore, bCore *Core) *Relation {
	rel := &Relation{
		aCore: aCore,
		bCore: bCore,
	}
	rel.RegisterAllocator(relType, rel.allocRel)
	rel.RegisterCreator(relType, nil, rel.destroyRel)
	return rel
}

// AddAIndex adds an index for A-side entity IDs.
func (rel *Relation) AddAIndex() {
	rel.aix = make([]int, len(rel.aids))
	for i := range rel.aids {
		rel.aix[i] = i
	}
	sortit(len(rel.aix), rel.aixLess, rel.aixSwap)
}

// AddBIndex adds an index for B-side entity IDs.
func (rel *Relation) AddBIndex() {
	rel.bix = make([]int, len(rel.bids))
	for i := range rel.bids {
		rel.bix[i] = i
	}
	sortit(len(rel.bix), rel.bixLess, rel.bixSwap)
}

// RelationType specified the type of a relation, it's basically a
// ComponentType where the highest bit is reserved.
type RelationType uint64

// All returns true only if all of the masked type bits are set.
func (t RelationType) All(mask RelationType) bool { return t&mask == mask }

// Any returns true only if at least one of the masked type bits is set.
func (t RelationType) Any(mask RelationType) bool { return t&mask != 0 }

const relType ComponentType = 1 << (63 - iota)

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

func (rel *Relation) indexLookup(
	tcl TypeClause,
	qids, aids, bids []EntityID,
	aix []int,
) []EntityID {
	// TODO: tighter cardinality estimation
	rset := make(map[EntityID]struct{}, len(rel.Entities))
	for _, id := range qids {
		for i := sort.Search(len(aix), func(i int) bool {
			return aids[aix[i]] >= id
		}); i < len(aix) && aids[aix[i]] == id; i++ {
			if j := aix[i]; tcl.Test(rel.Entities[j]) {
				rset[bids[j]] = struct{}{}
			}
		}
	}
	result := make([]EntityID, 0, len(rset))
	for id := range rset {
		result = append(result, id)
	}
	return result
}

func (rel Relation) aixLess(i, j int) bool { return rel.aids[rel.aix[i]] < rel.aids[rel.aix[j]] }
func (rel Relation) bixLess(i, j int) bool { return rel.bids[rel.bix[i]] < rel.bids[rel.bix[j]] }

func (rel Relation) aixSwap(i, j int) { rel.aix[i], rel.aix[j] = rel.aix[j], rel.aix[i] }
func (rel Relation) bixSwap(i, j int) { rel.bix[i], rel.bix[j] = rel.bix[j], rel.bix[i] }

func (rel *Relation) deferIndexing() func() {
	if rel.aix == nil && rel.bix == nil {
		return nil
	}
	rel.fix = true
	return func() {
		if rel.aix != nil {
			sortit(len(rel.aix), rel.aixLess, rel.aixSwap)
		}
		if rel.bix != nil {
			sortit(len(rel.aix), rel.aixLess, rel.aixSwap)
		}
		rel.fix = false
	}
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

// TODO: Where interface with WhereFunc convenience (would allow using indices more)
// TODO: secondary indices, uniqueness, keys, etc
// TODO: joins

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

type tmpSort struct {
	n    int
	less func(i, j int) bool
	swap func(i, j int)
}

func (ts tmpSort) Len() int           { return ts.n }
func (ts tmpSort) Less(i, j int) bool { return ts.less(i, j) }
func (ts tmpSort) Swap(i, j int)      { ts.swap(i, j) }

func fix(
	i, n int,
	less func(i, j int) bool,
	swap func(i, j int),
) {
	// TODO: something more minimal, since we assume sorted order but for [i]
	sortit(n, less, swap)
}

func sortit(
	n int,
	less func(i, j int) bool,
	swap func(i, j int),
) {
	sort.Sort(tmpSort{n, less, swap})
}
