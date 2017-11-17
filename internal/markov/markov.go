package markov

import (
	"encoding/json"
	"errors"
	"math/rand"
	"sort"

	"github.com/jcorbin/execs/internal/ecs"
)

const componentTransition ecs.ComponentType = 1<<63 - iota

// Table implements the core of a markov transition table for use in an ECS.
//
// FIXME say more.
type Table struct {
	ecs.Core

	weights [][]int
	next    [][]int
}

// AddEntity adds an entity add reserves space in the table for it.
func (t *Table) AddEntity() ecs.Entity {
	ent := t.Core.AddEntity()
	ent.AddComponent(componentTransition)
	t.weights = append(t.weights, nil)
	t.next = append(t.next, nil)
	return ent
}

// AddTransition adds an entity transition to the table.
func (t *Table) AddTransition(a, b ecs.Entity, weight int) {
	if !t.Owns(a) {
		panic("foreign a entity")
	}
	if !t.Owns(b) {
		panic("foreign b entity")
	}

	aid, bid := a.ID(), b.ID()
	next, weights := t.next[aid], t.weights[aid]

	i := sort.Search(len(next), func(i int) bool { return next[i] >= bid })
	if i < len(next) && next[i] == bid {
		weights[i] += weight
		return
	}

	n := len(next) + 1

	if n <= cap(next) {
		next = next[:n]
	} else {
		next = append(next, 0)
	}
	copy(next[i+1:], next[i:])
	next[i] = bid
	t.next[aid] = next

	if n <= cap(weights) {
		weights = weights[:n]
	} else {
		weights = append(weights, 0)
	}
	copy(weights[i+1:], weights[i:])
	weights[i] = weight
	t.weights[aid] = weights
}

// ChooseNext returns a randomly chosen entity that was previously added as a
// transition.
func (t *Table) ChooseNext(rng *rand.Rand, ent ecs.Entity) ecs.Entity {
	if !t.Owns(ent) {
		panic("foreign entity")
	}
	id := ent.ID()
	i, sum := -1, 0
	for j, w := range t.weights[id] {
		sum += w
		if rng.Intn(sum) <= w {
			i = j
		}
	}
	if i < 0 {
		return ecs.InvalidEntity
	}
	id = t.next[id][i]
	return t.Ref(id)
}

type serd struct {
	ID      int   `json:"id"`
	Next    []int `json:"next"`
	Weights []int `json:"weights"`
}

// MarhsalJSON marshal's the markov transition data into a json array.
func (t *Table) MarhsalJSON() ([]byte, error) {
	it := t.IterAll(componentTransition)
	data := make([]serd, 0, it.Count())
	for id, _, ok := it.Next(); ok; id, _, ok = it.Next() {
		data = append(data, serd{
			ID:      id,
			Next:    t.next[id],
			Weights: t.weights[id],
		})
	}
	return json.Marshal(data)
}

// UnmarshalJSON unmarshal's markov transition data into this table; table must
// be empty.
func (t *Table) UnmarshalJSON(d []byte) error {
	if len(t.Entities) > 0 {
		return errors.New("markov table already has data")
	}
	var data []serd
	if err := json.Unmarshal(d, &data); err != nil {
		return err
	}

	n := -1
	for _, dat := range data {
		if dat.ID >= n {
			n = dat.ID + 1
		}
	}
	t.next = make([][]int, n)
	t.weights = make([][]int, n)
	for _, dat := range data {
		t.next[dat.ID] = dat.Next
		t.weights[dat.ID] = dat.Weights
	}
	return nil
}
