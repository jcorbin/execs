package ecs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jcorbin/execs/internal/ecs"
)

const (
	scData ecs.ComponentType = 1 << iota
	scD2
)

type stuff struct {
	ecs.Core

	d1 []int
	d2 [][]int
}

func newStuff() *stuff {
	s := &stuff{
		d1: []int{0},
		d2: [][]int{nil},
	}
	s.RegisterAllocator(scData, s.allocData)
	s.RegisterCreator(scD2, s.createD2, s.destroyD2)
	return s
}

func (s *stuff) allocData(id ecs.EntityID, t ecs.ComponentType) {
	s.d1 = append(s.d1, 0)
	s.d2 = append(s.d2, nil)
}

func (s *stuff) createD2(id ecs.EntityID, t ecs.ComponentType) {
	if s.d2[id] == nil {
		s.d2[id] = make([]int, 0, 5)
	}
}

func (s *stuff) destroyD2(id ecs.EntityID, t ecs.ComponentType) {
	s.d2[id] = s.d2[id][:0]
}

func TestBasics(t *testing.T) {
	s := newStuff()
	assert.True(t, s.Empty())

	e1 := s.AddEntity(scData)
	assert.False(t, s.Empty())

	assert.Nil(t, s.d2[e1.ID()])
	e1.Add(scD2)
	assert.NotNil(t, s.d2[e1.ID()])
	assert.Equal(t, 0, len(s.d2[e1.ID()]))

	s.d2[e1.ID()] = append(s.d2[e1.ID()], 3, 1, 4)
	assert.Equal(t, 3, len(s.d2[e1.ID()]))

	e2 := s.AddEntity(scData | scD2)
	assert.NotNil(t, s.d2[e2.ID()])
	assert.Equal(t, 0, len(s.d2[e2.ID()]))

	e1.Delete(scD2)
	assert.Equal(t, 0, len(s.d2[e1.ID()]))
	assert.NotNil(t, s.d2[e1.ID()])

	e1.Destroy()

	e3 := s.AddEntity(scData | scD2)
	assert.Equal(t, e1.ID(), e3.ID())
}
