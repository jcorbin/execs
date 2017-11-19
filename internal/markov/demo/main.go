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
	ecs.Core
	markov.Table
	word   []string
	lookup map[string]ecs.EntityID
}

func newCorpus() *corpus {
	c := &corpus{
		word:   []string{""},
		lookup: make(map[string]ecs.EntityID),
	}
	c.Table.Init(&c.Core)
	c.RegisterAllocator(componentWord, c.allocWord)
	c.RegisterCreator(componentWord, nil, c.destroyWord)
	return c
}

func (c *corpus) allocWord(id ecs.EntityID, t ecs.ComponentType) {
	c.word = append(c.word, "")
}

func (c *corpus) destroyWord(id ecs.EntityID, t ecs.ComponentType) {
	delete(c.lookup, c.word[id])
	c.word[id] = ""
}

func (c *corpus) ToEntity(s string) ecs.Entity {
	if id, def := c.lookup[s]; def {
		return c.Ref(id)
	}
	ent := c.AddEntity(componentWord)
	c.word[ent.ID()] = s
	c.lookup[s] = ent.ID()
	return ent
}

func (c *corpus) ToString(ent ecs.Entity) string {
	id := c.Deref(ent)
	if ent.Type().All(componentWord) {
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
	c := newCorpus()
	for _, s := range []string{
		"it was the best of times",
		"it was the worst of times",
		"power brings out the worst in men",
		"I always try to be my best",
		"the best of the best hoorah",
	} {
		c.Ingest(strings.Fields(s))
	}
	return c
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
