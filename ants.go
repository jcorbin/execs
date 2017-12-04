package main

import (
	"strings"

	"github.com/jcorbin/execs/internal/ecs"
	"github.com/jcorbin/execs/internal/point"
	termbox "github.com/nsf/termbox-go"
)

const (
	antW         = 2
	antM         = 0x03
	antL antRule = 1
	antR antRule = 2
)

var antHeads = []point.Point{
	point.Pt(0, 1),
	point.Pt(1, 0),
	point.Pt(0, -1),
	point.Pt(-1, 0),
}

type antRule uint64

func makeAntRule(parts ...antRule) antRule {
	var rule antRule
	for i := len(parts) - 1; i >= 0; i-- {
		rule = rule<<antW | parts[i]&antM
	}
	return rule
}

func (rule antRule) Len() (n int) {
	for rule != 0 {
		n++
		rule >>= antW
	}
	return n
}

func (rule antRule) String() string {
	parts := make([]string, 0, rule.Len())
	for rule != 0 {
		switch rule & antM {
		case 0:
			parts = append(parts, "F")
		case antL:
			parts = append(parts, "L")
		case antR:
			parts = append(parts, "R")
		case antL | antR:
			parts = append(parts, "LR")
		}
		rule >>= antW
	}
	return strings.Join(parts, " ")
}

func (rule antRule) For(st uint) antRule {
	for st = st % uint(rule.Len()); st > 0; st-- {
		rule >>= antW
	}
	return rule & antM
}

func (w *world) runAnts() {
	it := w.Iter(ecs.All(wcPosition | wcAnt))
	w.ui.perfDash.Note("Nants", "%d", it.Count())

	type createReq struct {
		pos   point.Point
		glyph rune
		rule  antRule
		head  uint
	}

	type moveReq struct {
		ent ecs.Entity
		pos point.Point
	}

	n := it.Count()
	createQ := make([]createReq, 0, n)
	moveQ := make([]moveReq, 0, n)
	deadQ := make(map[ecs.EntityID]struct{}, n)

	for it.Next() {
		if _, dead := deadQ[it.ID()]; dead {
			continue
		}
		pos, _ := w.pos.Get(it.Entity())

		here := w.pos.At(pos)

		// TODO: better collision -> death rule
		any := false
		for _, ent := range here {
			if id := ent.ID(); id != it.ID() && ent.Type().All(wcAnt) {
				deadQ[id] = struct{}{}
				any = true
			}
		}
		if any {
			deadQ[it.ID()] = struct{}{}
			continue
		}

		cell := ecs.NilEntity
		if cells := ecs.Filter(here, ecs.All(wcPosition|wcBG)); len(cells) > 0 {
			cell = cells[0]
		}

		rule := w.antRule[it.ID()]
		head := w.antHead[it.ID()] % 4
		st := w.antDeriveState(cell)
		switch rule.For(st) {
		case antL:
			head = (head + 3) % 4
		case antR:
			head = (head + 1) % 4
		case antL | antR:
			cloneHead := (head + 1) % 4
			createQ = append(createQ, createReq{
				pos.Add(antHeads[cloneHead]),
				w.Glyphs[it.ID()],
				rule,
				cloneHead,
			})
			head = (head + 3) % 4
		}
		w.antApplyState(cell, pos, st+1)
		w.antHead[it.ID()] = head

		moveQ = append(moveQ, moveReq{it.Entity(), pos.Add(antHeads[head])})
	}

	for id := range deadQ {
		w.Ref(id).Destroy()
	}
	for _, req := range createQ {
		ent := w.AddEntity(wcGlyph | wcAnt)
		w.pos.Set(ent, req.pos)
		w.Glyphs[ent.ID()] = req.glyph
		w.antRule[ent.ID()] = req.rule
		w.antHead[ent.ID()] = req.head
	}
	for _, req := range moveQ {
		w.pos.Set(req.ent, req.pos)
	}
}

func (w *world) antDeriveState(cell ecs.Entity) uint {
	if cell == ecs.NilEntity {
		return 0
	}
	if cell.Type().All(wcFloor | wcBG) {
		bg := w.BG[cell.ID()]
		if i := colorIndex(bg, floorColors); i >= 0 {
			return uint(i) + 1
		}
		return 0
	}
	if cell.Type().All(wcWall | wcBG) {
		bg := w.BG[cell.ID()]
		if i := colorIndex(bg, wallColors); i >= 0 {
			return uint(i) + 2
		}
		return 0
	}
	return 0
}

func (w *world) antApplyState(cell ecs.Entity, pos point.Point, st uint) {
	st = st % 9

	if st == 0 {
		cell.Destroy()
		return
	}

	var (
		t  = wcPosition | wcBG
		bg termbox.Attribute
		ch rune
	)
	if st <= 3 {
		t |= wcFloor
		bg = floorColors[st-1]
	} else {
		t |= wcCollide | wcSolid | wcGlyph | wcFG | wcWall
		bg = wallColors[st-2]
		ch = '#'
	}
	if cell == ecs.NilEntity {
		cell = w.AddEntity(t)
		w.pos.Set(cell, pos)
	} else {
		cell.SetType(t)
	}
	w.BG[cell.ID()] = bg
	w.FG[cell.ID()] = bg + 1
	if ch != 0 {
		w.Glyphs[cell.ID()] = ch
	}
}

func colorIndex(attr termbox.Attribute, attrs []termbox.Attribute) int {
	for i := range attrs {
		if attrs[i] == attr {
			return i
		}
	}
	return -1
}
