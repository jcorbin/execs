package ecs

// ArrayIndex manages a simple single-scoped index for homogenous array data.
type ArrayIndex struct {
	Scope *Scope
	id    []ID
	ix    map[ID]int
	free  []int
}

// Len returns how many id slots have been allocated.
func (ai *ArrayIndex) Len() int { return len(ai.id) }

// ID returns the entity ID stored for the given array index.
func (ai *ArrayIndex) ID(i int) ID {
	if i < len(ai.id) {
		return ai.id[i]
	}
	return 0
}

// Create index entries for the given entity, re-using from the free list if
// possible. Returns the array index that should be used for the new entity.
func (ai *ArrayIndex) Create(ent Entity) (i int) {
	if ai.Scope == nil {
		ai.Scope = ent.Scope
	} else if ent.Scope != ai.Scope {
		panic("multi-scope use of ArrayIndex")
	}
	if ai.ix == nil {
		ai.ix = make(map[ID]int, 64)
	}
	if j := len(ai.free) - 1; j >= 0 {
		i = ai.free[j]
		ai.free = ai.free[:j]
		ai.id[i] = ent.ID
	} else {
		i = len(ai.id)
		ai.id = append(ai.id, ent.ID)
	}
	ai.ix[ent.ID] = i
	return i
}

// Destroy index entries for the given entities, returning the old index and a
// boolean that is true only if the entity had been defined.
func (ai *ArrayIndex) Destroy(ent Entity) (i int, def bool) {
	if ai.Scope != nil && ent.Scope != ai.Scope {
		panic("multi-scope use of ArrayIndex")
	}
	i, def = ai.ix[ent.ID]
	if def {
		ai.id[i] = 0
		delete(ai.ix, ent.ID)
		ai.free = append(ai.free, i)
	}
	return i, def
}

// Get returns the index defined for the given entity and true, only if the
// entity has been created under this ArrayIndex.
func (ai *ArrayIndex) Get(ent Entity) (i int, def bool) {
	if ai.Scope != nil && ent.Scope != ai.Scope {
		return 0, false
	}
	i, def = ai.ix[ent.ID]
	return i, def
}

// TODO func (ai *ArrayIndex) compact(
// 	cop func(destI, destJ, srcI, srcJ int)
// )
