package main

import (
	"fmt"
	"image"

	"github.com/jcorbin/execs/internal/ecs"
	"github.com/jcorbin/execs/internal/quadindex"
)

type position struct {
	ecs.ArrayIndex
	qi quadindex.Index
	pt []image.Point
}

type positioned struct {
	pos *position
	i   int
}

func (pos *position) Create(ent ecs.Entity, _ ecs.Type) {
	i := pos.ArrayIndex.Insert(ent)
	for i >= len(pos.pt) {
		if i < cap(pos.pt) {
			pos.pt = pos.pt[:i+1]
		} else {
			pos.pt = append(pos.pt, image.ZP)
		}
	}
	pos.pt[i] = image.ZP
}

func (pos *position) Get(ent ecs.Entity) positioned {
	if i, def := pos.ArrayIndex.Get(ent); def {
		return positioned{pos, i}
	}
	return positioned{}
}

func (pos *position) GetID(id ecs.ID) positioned {
	if i, def := pos.ArrayIndex.GetID(id); def {
		return positioned{pos, i}
	}
	return positioned{}
}

func (pos *position) At(p image.Point) (pq positionQuery) {
	pq.pos = pos
	pq.Cursor = pos.qi.At(p)
	return pq
}

func (pos *position) Within(r image.Rectangle) (pq positionQuery) {
	pq.pos = pos
	pq.Cursor = pos.qi.Within(r)
	return pq
}

type positionQuery struct {
	pos *position
	quadindex.Cursor
}

func (pq *positionQuery) handle() positioned {
	if i := pq.I(); i >= 0 {
		return positioned{pq.pos, i}
	}
	return positioned{}
}

func (posd positioned) zero() bool { return posd.pos == nil }

func (posd positioned) Point() image.Point {
	if posd.pos == nil {
		return image.ZP
	}
	return posd.pos.pt[posd.i]
}

func (posd positioned) SetPoint(p image.Point) {
	if posd.pos != nil {
		posd.pos.pt[posd.i] = p
		posd.pos.qi.Update(posd.i, p)
	}
}

func (posd positioned) Entity() ecs.Entity { return posd.pos.Entity(posd.i) }
func (posd positioned) ID() ecs.ID         { return posd.pos.ID(posd.i) }

func (posd positioned) String() string {
	if posd.pos == nil {
		return fmt.Sprintf("no-position")
	}
	return fmt.Sprintf("pt: %v", posd.pos.pt[posd.i])
}
