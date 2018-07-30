package ecs

type Entities map[*Scope][]ID

func (es Entities) Create(ent Entity, _ Type) {
	es[ent.Scope] = append(es[ent.Scope], ent.ID)
}

func (es Entities) Destroy(ent Entity, _ Type) {
	es[ent.Scope] = removeID(es[ent.Scope], ent.ID)
}

func removeID(ids []ID, id ID) []ID {
	for i := range ids {
		if ids[i] == id {
			if j := i + 1; j < len(ids) {
				return ids[:i+copy(ids[i:], ids[j:])]
			}
			return ids[:i]
		}
	}
	return ids
}
