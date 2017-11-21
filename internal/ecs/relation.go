package ecs

// RelationFlags specifies options for the A or B dimension in a Relation.
type RelationFlags uint32

const (
	// RelationCascadeDestroy causes destruction of an entity relation to
	// destroy related entities within the flagged dimension.
	RelationCascadeDestroy RelationFlags = 1 << iota

	// RelationRestrictDeletes TODO: cannot abort a destroy at present
)

// Relation contains entities that represent relations between entities in two
// (maybe different) Cores. Users may attach arbitrary data to these relations
// the same way you would with Core.
//
// NOTE: the high Type bit (bit 64) is reserved.
type Relation struct {
	Core
	aCore, bCore *Core
	aFlag, bFlag RelationFlags
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
func NewRelation(
	aCore *Core, aFlags RelationFlags,
	bCore *Core, bFlags RelationFlags,
) *Relation {
	rel := &Relation{}
	rel.Init(aCore, aFlags, bCore, bFlags)
	return rel
}

// Init initializes the entity relation; useful for embedding.
func (rel *Relation) Init(
	aCore *Core, aFlags RelationFlags,
	bCore *Core, bFlags RelationFlags,
) {
	rel.aCore, rel.aFlag = aCore, aFlags
	rel.bCore, rel.bFlag = bCore, bFlags
	rel.RegisterAllocator(relType, rel.allocRel)
	rel.RegisterDestroyer(relType, rel.destroyRel)
	rel.aCore.RegisterDestroyer(NoType, rel.destroyFromA)
	if rel.aCore != rel.bCore {
		rel.bCore.RegisterDestroyer(NoType, rel.destroyFromB)
	}
}

// RelationType specified the type of a relation, it's basically a
// ComponentType where the highest bit is reserved.
type RelationType uint64

// NoRelType is the RelationType equivalent of NoType.
const NoRelType RelationType = 0

// All returns true only if all of the masked type bits are set.
func (t RelationType) All(mask RelationType) bool { return t&mask == mask }

// Any returns true only if at least one of the masked type bits is set.
func (t RelationType) Any(mask RelationType) bool { return t&mask != 0 }

const relType ComponentType = 1 << (63 - iota)

// AnyRel is a convenience for Any(ComponentType(t)).
func AnyRel(t RelationType) TypeClause { return Any(ComponentType(t)) }

// AllRel is a convenience for All(ComponentType(t)).
func AllRel(t RelationType) TypeClause { return All(ComponentType(t)) }

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
	if aid := rel.aids[i]; aid != 0 {
		if rel.aFlag&RelationCascadeDestroy != 0 {
			rel.aCore.setType(aid, NoType)
		}
		rel.aids[i] = 0
	}
	if bid := rel.bids[i]; bid != 0 {
		if rel.bFlag&RelationCascadeDestroy != 0 {
			rel.bCore.setType(bid, NoType)
		}
		rel.bids[i] = 0
	}
	if !rel.fix {
		if rel.aix != nil {
			fix(len(rel.aix), i, rel.aixLess, rel.aixSwap)
		}
		if rel.bix != nil {
			fix(len(rel.bix), i, rel.bixLess, rel.bixSwap)
		}
	}
}

func (rel *Relation) destroyFromA(aid EntityID, t ComponentType) {
	for i, t := range rel.types {
		if t.All(relType) && rel.aids[i] == aid {
			rel.setType(EntityID(i+1), NoType)
		}
	}
}

func (rel *Relation) destroyFromB(bid EntityID, t ComponentType) {
	for i, t := range rel.types {
		if t.All(relType) && rel.bids[i] == bid {
			rel.setType(EntityID(i+1), NoType)
		}
	}
}

// A returns a reference to the A-side entity for the given relation entity.
func (rel *Relation) A(ent Entity) Entity {
	if ent.Type().All(relType) {
		return rel.aCore.Ref(rel.aids[rel.Deref(ent)-1])
	}
	return NilEntity
}

// B returns a reference to the B-side entity for the given relation entity.
func (rel *Relation) B(ent Entity) Entity {
	if ent.Type().All(relType) {
		return rel.bCore.Ref(rel.bids[rel.Deref(ent)-1])
	}
	return NilEntity
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
	return &iterCursor{rel: rel, it: it, where: where}
}

// LookupA returns a Cursor that will iterate over relations involving one or
// more given A entities.
func (rel *Relation) LookupA(tcl TypeClause, ids ...EntityID) Cursor {
	if rel.aix == nil {
		return rel.scanLookup(tcl, false, ids)
	}
	return rel.indexLookup(tcl, ids, rel.aids, rel.aix)
}

// LookupB returns a Cursor that will iterate over relations involving one or
// more given B entities.
func (rel *Relation) LookupB(tcl TypeClause, ids ...EntityID) Cursor {
	if rel.bix == nil {
		return rel.scanLookup(tcl, true, ids)
	}
	return rel.indexLookup(tcl, ids, rel.bids, rel.bix)
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
) int {
	n := 0
	if fixIndex := rel.deferIndexing(); fixIndex != nil {
		defer fixIndex()
	}
	for cur := rel.Cursor(tcl, where); cur.Scan(); {
		ent := cur.Entity()
		oa, ob, or := cur.A(), cur.B(), cur.R()
		n++ // TODO: differetiate updated vs destroyed?
		na, nb := set(ent, oa, ob, or)
		if na == NilEntity || nb == NilEntity {
			rel.setType(cur.Entity().ID(), NoType)
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
	return n
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
	for cur := rel.Cursor(tcl, where); cur.Scan(); {
		rel.setType(cur.Entity().ID(), NoType)
	}
}
