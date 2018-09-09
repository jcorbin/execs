package main

import (
	"image"

	"github.com/jcorbin/execs/internal/ecs"
)

type position struct {
	ecs.ArrayIndex
	// TODO quad-tree index
	pt []image.Point
}

type positioned struct {
	pos *position
	i   int
}

func (pos *position) Create(ent ecs.Entity, _ ecs.Type) {
	i := pos.ArrayIndex.Create(ent)
	for i >= len(pos.pt) {
		if i < cap(pos.pt) {
			pos.pt = pos.pt[:i+1]
		} else {
			pos.pt = append(pos.pt, image.ZP)
		}
	}
	pos.pt[i] = image.ZP
}

func (pos *position) Destroy(ent ecs.Entity, _ ecs.Type) {
	pos.ArrayIndex.Destroy(ent)
}

func (pos *position) Get(ent ecs.Entity) positioned {
	if i, def := pos.ArrayIndex.Get(ent); def {
		return positioned{pos, i}
	}
	return positioned{}
}

func (posd positioned) Point() image.Point {
	if posd.pos == nil {
		return image.ZP
	}
	return posd.pos.pt[posd.i]
}
func (posd positioned) SetPoint(pt image.Point) {
	if posd.pos != nil {
		posd.pos.pt[posd.i] = pt
	}
}
