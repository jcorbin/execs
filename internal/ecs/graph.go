package ecs

// Graph is an auto-relation: one where both the A-side and B-side are the
// same Core system.
type Graph struct {
	Relation
}

// NewGraph creates a new graph relation for the given Core system.
func NewGraph(core *Core, flags RelationFlags) *Graph {
	G := &Graph{}
	G.Init(core, flags)
	return G
}

// Init initializes the graph relation; useful for embedding.
func (G *Graph) Init(core *Core, flags RelationFlags) {
	G.Relation.Init(core, flags, core, flags)
}

// Roots returns a slice of Entities that have no in-relation (i.e. there's no
// relation `a R b for all a in the result`).
func (G *Graph) Roots(
	tcl TypeClause,
	where func(ent, a, b Entity, r RelationType) bool,
) []Entity {
	// TODO: leverage index if available
	tcl.All |= relType
	it := G.Iter(tcl)
	triset := make(map[EntityID]bool, it.Count())
	n := 0
	for it.Next() {
		i := it.ID() - 1

		if where != nil && !where(
			it.Entity(),
			G.aCore.Ref(G.aids[i]),
			G.aCore.Ref(G.bids[i]),
			RelationType(it.Type() & ^relType),
		) {
			continue
		}

		aid, bid := G.aids[i], G.bids[i]
		if _, def := triset[aid]; !def {
			triset[aid] = true
			n++
		}
		if in := triset[bid]; in {
			n--
		}
		triset[bid] = false
	}

	result := make([]Entity, 0, n)
	for id, in := range triset {
		if in {
			result = append(result, G.aCore.Ref(id))
		}
	}
	return result
}

// Leaves returns a slice of Entities that have no out-relation (i.e. there's no
// relation `a R b for all b in the result`).
func (G *Graph) Leaves(
	tcl TypeClause,
	where func(ent, a, b Entity, r RelationType) bool,
) []Entity {
	// TODO: leverage index if available
	tcl.All |= relType
	it := G.Iter(tcl)
	triset := make(map[EntityID]bool, it.Count())
	n := 0
	for it.Next() {
		i := it.ID() - 1

		if where != nil && !where(
			it.Entity(),
			G.aCore.Ref(G.aids[i]),
			G.aCore.Ref(G.bids[i]),
			RelationType(G.Entities[i] & ^relType),
		) {
			continue
		}

		aid, bid := G.aids[i], G.bids[i]
		if _, def := triset[bid]; !def {
			triset[bid] = true
			n++
		}
		if in := triset[aid]; in {
			n--
		}
		triset[aid] = false
	}

	result := make([]Entity, 0, n)
	for id, in := range triset {
		if in {
			result = append(result, G.aCore.Ref(id))
		}
	}
	return result
}

// TODO: graph traversal like DFS, CoDFS, BFS, CoBFS
