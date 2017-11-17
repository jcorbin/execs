package main

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/jcorbin/execs/internal/ecs"
	"github.com/jcorbin/execs/internal/markov"
)

const (
	componentWord ecs.ComponentType = 1 << iota
)

type corpus struct {
	markov.Table
	word   []string
	lookup map[string]int
}

func (c *corpus) AddEntity() ecs.Entity {
	ent := c.Table.AddEntity()
	c.word = append(c.word, "")
	return ent
}

func (c *corpus) ToEntity(s string) ecs.Entity {
	if id, def := c.lookup[s]; def {
		return c.Ref(id)
	}
	if c.lookup == nil {
		c.lookup = make(map[string]int, 1)
	}
	ent := c.AddEntity()
	ent.AddComponent(componentWord)
	id := ent.ID()
	c.word[id] = s
	c.lookup[s] = id
	return ent
}

func (c *corpus) ToString(ent ecs.Entity) string {
	if !c.Owns(ent) {
		panic("foreign entity")
	}
	if id := ent.ID(); c.Entities[id].All(componentWord) {
		return c.word[id]
	}
	return ""
}

func (c *corpus) Ingest(chain []string) {
	term := c.ToEntity("")
	last := term
	for _, s := range chain {
		ent := c.ToEntity(s)
		c.AddTransition(last, ent, 1)
		last = ent
	}
	c.AddTransition(last, term, 1)
}

func canned() *corpus {
	var c corpus
	for _, s := range []string{
		"it was the best of times",
		"it was the worst of times",
		"power brings out the worst in men",
		"I always try to be my best",
		"the best of the best hoorah",
	} {
		c.Ingest(strings.Fields(s))
	}
	return &c
}

func main() {
	rng := rand.New(rand.NewSource(rand.Int63()))
	c := canned()
	var parts []string
	for i := 0; i < 10; i++ {
		ent := c.ToEntity("")
		for {
			ent = c.ChooseNext(rng, ent)
			s := c.ToString(ent)
			if s == "" {
				break
			}
			parts = append(parts, s)
		}
		fmt.Printf("%s\n", strings.Join(parts, " "))
		parts = parts[:0]
	}
}
