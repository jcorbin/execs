package ecs

// Graph is an auto-relation: one where both the A-side and B-side are the
// same Core system.
type Graph struct {
	Relation
}

// NewGraph creates a new graph relation for the given Core system.
func NewGraph(core *Core) *Graph {
	G := &Graph{
		Relation: Relation{
			aCore: core,
			bCore: core,
		},
	}
	G.RegisterAllocator(relType, G.allocRel)
	G.RegisterCreator(relType, nil, G.destroyRel)
	return G
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
		ent := it.Entity()
		i := ent.ID() - 1
		r := RelationType(G.Entities[i] & ^relType)
		a := G.aCore.Ref(G.aids[i])
		b := G.aCore.Ref(G.bids[i])
		if where == nil || where(ent, a, b, r) {
			aid, bid := G.aids[i], G.bids[i]
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
		ent := it.Entity()
		i := ent.ID() - 1
		r := RelationType(G.Entities[i] & ^relType)
		a := G.aCore.Ref(G.aids[i])
		b := G.aCore.Ref(G.bids[i])
		if where == nil || where(ent, a, b, r) {
			aid, bid := G.aids[i], G.bids[i]
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
			result = append(result, G.aCore.Ref(id))
		}
	}
	return result
}

// TODO: graph traversal like DFS, CoDFS, BFS, CoBFS
