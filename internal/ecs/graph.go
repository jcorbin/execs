package ecs

// Graph is an auto-relation: one where both the A-side and B-side ar
// the same Core system.
type Graph Relation

// NewGraph creates a new graph relation for the given Core system.
func NewGraph(core *Core) *Relation {
	return NewRelation(core, core)
}

// Roots returns a slice of Entities that have no in-relation (i.e. there's no
// relation `a R b for all a in the result`).
func (rel *Graph) Roots(
	tcl TypeClause,
	where func(ent, a, b Entity, r RelationType) bool,
) []Entity {
	// TODO: leverage index if available
	tcl.All |= relType
	it := rel.Iter(tcl)
	triset := make(map[EntityID]bool, it.Count())
	n := 0
	for it.Next() {
		ent := it.Entity()
		i := ent.ID() - 1
		r := RelationType(rel.Entities[i] & ^relType)
		a := rel.aCore.Ref(rel.aids[i])
		b := rel.aCore.Ref(rel.bids[i])
		if where == nil || where(ent, a, b, r) {
			aid, bid := rel.aids[i], rel.bids[i]
			if _, def := triset[aid]; !def {
				triset[aid] = true
				n++
			}
			if in := triset[bid]; in {
				triset[bid] = false
				n--
			}
		}
	}

	result := make([]Entity, 0, n)
	for id, in := range triset {
		if in {
			result = append(result, rel.aCore.Ref(id))
		}
	}
	return result
}

// Leaves returns a slice of Entities that have no out-relation (i.e. there's no
// relation `a R b for all b in the result`).
func (rel *Graph) Leaves(
	tcl TypeClause,
	where func(ent, a, b Entity, r RelationType) bool,
) []Entity {
	// TODO: leverage index if available
	tcl.All |= relType
	it := rel.Iter(tcl)
	triset := make(map[EntityID]bool, it.Count())
	n := 0
	for it.Next() {
		ent := it.Entity()
		i := ent.ID() - 1
		r := RelationType(rel.Entities[i] & ^relType)
		a := rel.aCore.Ref(rel.aids[i])
		b := rel.aCore.Ref(rel.bids[i])
		if where == nil || where(ent, a, b, r) {
			aid, bid := rel.aids[i], rel.bids[i]
			if _, def := triset[bid]; !def {
				triset[bid] = true
				n++
			}
			if in := triset[aid]; in {
				triset[aid] = false
				n--
			}
		}
	}

	result := make([]Entity, 0, n)
	for id, in := range triset {
		if in {
			result = append(result, rel.aCore.Ref(id))
		}
	}
	return result
}

// TODO: graph traversal like DFS, CoDFS, BFS, CoBFS
