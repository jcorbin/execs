package main

import (
	"fmt"
	"math/rand"

	"github.com/jcorbin/execs/internal/ecs"
)

// TODO: more explicit guard against orphans

const (
	bcHP ecs.ComponentType = 1 << iota
	bcPart
	bcName

	bcRight
	bcLeft

	bcHead  // o
	bcTorso // O
	bcArm   // \ /
	bcHand  // w
	bcLeg   // |
	bcFoot  // ^
	bcTail
)

const (
	bcPartMask = bcHead | bcTorso | bcTail | bcArm | bcHand | bcLeg | bcFoot
	bcLocMask  = bcRight | bcLeft
)

const (
	brControl ecs.RelationType = 1 << iota
)

type body struct {
	ecs.Core

	fmt   []string
	maxHP []int
	hp    []int
	dmg   []int
	armor []int

	rel ecs.Graph
}

type bodyStats struct {
	HP, MaxHP int
	Damage    int
	Armor     int
}

type bodyParts []bodyPart

type bodyPart struct {
	Type ecs.ComponentType
	Name string
	Desc string
	bodyStats
}

func newBody() *body {
	bo := &body{
		// TODO: consider eliminating the padding for EntityID(0)
		fmt:   []string{""},
		maxHP: []int{0},
		hp:    []int{0},
		dmg:   []int{0},
		armor: []int{0},
	}
	bo.RegisterAllocator(bcPart, bo.allocPart)
	return bo
}

func (bo *body) allocPart(id ecs.EntityID, t ecs.ComponentType) {
	bo.fmt = append(bo.fmt, "")
	bo.maxHP = append(bo.maxHP, 0)
	bo.hp = append(bo.hp, 0)
	bo.dmg = append(bo.dmg, 0)
	bo.armor = append(bo.armor, 0)
}

func (bo *body) build(rng *rand.Rand) {
	head := bo.AddPart(bcHead, 5, 3, 4)
	torso := bo.AddPart(bcTorso, 8, 0, 2)

	rightArm := bo.AddPart(bcRight|bcArm, 5, 3, 2)
	leftArm := bo.AddPart(bcLeft|bcArm, 5, 3, 2)

	rightHand := bo.AddPart(bcRight|bcHand, 2, 5, 1)
	leftHand := bo.AddPart(bcLeft|bcHand, 2, 5, 1)

	rightLeg := bo.AddPart(bcRight|bcLeg, 6, 5, 3)
	leftLeg := bo.AddPart(bcLeft|bcLeg, 6, 5, 3)

	rightFoot := bo.AddPart(bcRight|bcFoot, 3, 6, 2)
	leftFoot := bo.AddPart(bcLeft|bcFoot, 3, 6, 2)

	bo.rel.InsertMany(func(insert func(r ecs.RelationType, a, b ecs.Entity) ecs.Entity) {
		insert(brControl, head, torso)
		insert(brControl, torso, rightArm)
		insert(brControl, torso, leftArm)
		insert(brControl, torso, rightLeg)
		insert(brControl, torso, leftLeg)
		insert(brControl, rightArm, rightHand)
		insert(brControl, leftArm, leftHand)
		insert(brControl, rightLeg, rightFoot)
		insert(brControl, leftLeg, leftFoot)
	})
}

func (bo *body) AddPart(t ecs.ComponentType, hp, dmg, armor int) ecs.Entity {
	ent := bo.AddEntity(bcHP | bcPart | t)
	id := ent.ID()
	bo.maxHP[id] = hp
	bo.hp[id] = hp
	bo.dmg[id] = dmg
	bo.armor[id] = armor
	return ent
}

func (bo *body) Stats() bodyStats {
	var s bodyStats
	it := bo.Iter(ecs.All(bcHP | bcPart))
	for it.Next() {
		s.HP += bo.hp[it.ID()]
		s.MaxHP += bo.maxHP[it.ID()]
		s.Damage += bo.dmg[it.ID()]
		s.Armor += bo.armor[it.ID()]
	}
	return s
}

func (bo *body) HPRange() (hp, maxHP int) {
	it := bo.Iter(ecs.All(bcHP))
	for it.Next() {
		hp += bo.hp[it.ID()]
		maxHP += bo.maxHP[it.ID()]
	}
	return
}

func (bo *body) HP() int {
	hp := 0
	it := bo.Iter(ecs.All(bcHP))
	for it.Next() {
		hp += bo.hp[it.ID()]
	}
	return hp
}

func (bo *body) Roots() []ecs.Entity {
	return bo.rel.Roots(ecs.All(ecs.RelType(brControl)), nil)
}

func (bo *body) Descend(ent ecs.Entity, f func(ent ecs.Entity, level int) bool) {
	id := bo.Deref(ent)
	tcl := ecs.All(ecs.RelType(brControl))
	bo.descend(tcl, id, 0, func(id ecs.EntityID, level int) bool {
		return f(bo.Ref(id), level)
	})
}

func (bo *body) descend(
	tcl ecs.TypeClause, id ecs.EntityID, level int,
	f func(id ecs.EntityID, level int) bool,
) {
	if !f(id, level) {
		return
	}
	for _, id := range bo.rel.LookupA(tcl, id) {
		bo.descend(tcl, id, level+1, f)
	}
}

func (bo *body) Ascend(ent ecs.Entity, f func(ent ecs.Entity) bool) {
	id := bo.Deref(ent)
	tcl := ecs.All(ecs.RelType(brControl))
	bo.ascend(tcl, id, func(id ecs.EntityID) bool {
		return f(bo.Ref(id))
	})
}

func (bo *body) ascend(
	tcl ecs.TypeClause, id ecs.EntityID,
	f func(id ecs.EntityID) bool,
) {
	if !f(id) {
		return
	}
	for _, id := range bo.rel.LookupB(tcl, id) {
		bo.ascend(tcl, id, f)
	}
}

func (bo *body) choosePart(want func(prior, ent ecs.Entity) bool) ecs.Entity {
	var choice ecs.Entity
	it := bo.Iter(ecs.All(bcPart | bcHP))
	for it.Next() {
		if want(choice, it.Entity()) {
			choice = it.Entity()
		}
	}
	return choice
}

func (bo *body) chooseRandomPart(rng *rand.Rand, score func(ent ecs.Entity) int) ecs.Entity {
	sum := 0
	return bo.choosePart(func(prior, ent ecs.Entity) bool {
		if w := score(ent); w > 0 {
			sum += w
			return prior == ecs.NilEntity || rng.Intn(sum) < w
		}
		return false
	})
}

func (bo *body) PartName(ent ecs.Entity) string {
	switch ent.Type() & bcPartMask {
	case bcHead:
		return "head"
	case bcTorso:
		return "torso"
	case bcArm:
		return "arm"
	case bcHand:
		return "hand"
	case bcLeg:
		return "leg"
	case bcFoot:
		return "foot"
	case bcTail:
		return "tail"
	}
	return ""
}

func (bo *body) DescribePart(ent ecs.Entity) string {
	s := bo.PartName(ent)
	if s == "" {
		s = "???"
	}
	switch ent.Type() & bcLocMask {
	case bcRight:
		s = "right " + s
	case bcLeft:
		s = "left " + s
	}
	if ent.Type().All(bcName) {
		s = fmt.Sprintf(bo.fmt[ent.ID()], s)
	}
	return s
}

func (w *world) attack(srcID, targID ecs.EntityID) {
	const numSpiritTurns = 5

	src, targ := w.bodies[srcID], w.bodies[targID]
	srcName := w.getName(srcID, "!?!")
	targName := w.getName(targID, "?!?")

	aPart := src.chooseRandomPart(w.rng, func(ent ecs.Entity) int {
		if src.dmg[ent.ID()] <= 0 {
			return 0
		}
		return 4*src.dmg[ent.ID()] + 2*src.armor[ent.ID()] + src.hp[ent.ID()]
	})
	if aPart == ecs.NilEntity {
		w.log("%s has nothing to hit %s with.", srcName, targName)
		return
	}

	bPart := targ.chooseRandomPart(w.rng, func(ent ecs.Entity) int {
		if targ.hp[ent.ID()] <= 0 {
			return 0
		}
		switch ent.Type() & bcPartMask {
		case bcTorso:
			return 100 * (1 + targ.maxHP[ent.ID()] - targ.hp[ent.ID()])
		case bcHead:
			return 10 * (1 + targ.maxHP[ent.ID()] - targ.hp[ent.ID()])
		}
		return 0
	})
	if bPart == ecs.NilEntity {
		w.log("%s can find nothing worth hitting on %s.", srcName, targName)
		return
	}

	aEff, bEff := 1.0, 1.0
	src.Ascend(aPart, func(ent ecs.Entity) bool {
		aEff *= float64(src.hp[ent.ID()]) / float64(src.maxHP[ent.ID()])
		return true
	})
	targ.Ascend(bPart, func(ent ecs.Entity) bool {
		bEff *= float64(targ.hp[ent.ID()]) / float64(targ.maxHP[ent.ID()])
		return true
	})

	aDesc := src.DescribePart(aPart)
	bDesc := targ.DescribePart(bPart)

	if 1.0-bEff*w.rng.Float64()/aEff*w.rng.Float64() < -1.0 {
		w.log("%s's %s misses %s's %s", srcName, aDesc, targName, bDesc)
		return
	}

	dmg := src.dmg[aPart.ID()]
	if dmg > 1 {
		dmg = dmg/2 + rand.Intn(dmg/2)
	} else if dmg == 1 {
		dmg = rand.Intn(1)
	}
	dmg -= targ.armor[bPart.ID()]

	if dmg == 0 {
		w.log("%s's %s bounces off %s's %s", srcName, aDesc, targName, bDesc)
		return
	}

	if dmg < 0 {
		src, targ = targ, src
		srcName, targName = targName, srcName
		aPart, bPart = bPart, aPart
		aEff, bEff = bEff, aEff
		aDesc, bDesc = bDesc, aDesc
		dmg = -dmg
	}

	// dmg > 0
	part, destroyed := targ.damagePart(bPart, dmg)
	w.log("%s's %s dealt %v damage to %s's %s", srcName, aDesc, dmg, targName, bDesc)
	if !destroyed {
		return
	}

	if severed := targ.sever(bPart.ID()); severed != nil {
		w.newItem(w.Positions[targID], fmt.Sprintf("remains of %s", targName), '%', severed)
		if roots := severed.Roots(); len(roots) > 0 {
			for _, ent := range roots {
				w.log("%s's %s has dropped on the floor", targName, severed.DescribePart(ent))
			}
		}
	}

	if bo := w.bodies[targID]; bo.Iter(ecs.All(bcPart)).Count() > 0 {
		w.log("%s's %s destroyed by %s's %s", targName, bDesc, srcName, aDesc)
		return
	}

	// head not destroyed -> become spirit
	if part.Type.All(bcHead) {
		w.Ref(targID).Destroy()
		w.log("%s destroyed by %s (headshot)", targName, srcName)
	} else {
		w.Glyphs[targID] = '⟡'
		w.Ref(targID).Delete(wcBody)
		w.bodies[targID] = nil
		w.setTimer(targID, numSpiritTurns, timerDestroy)
		w.log("%s was disembodied by %s (moving as spirit for %v turns)", targName, srcName, numSpiritTurns)
	}
}

func (bo *body) damagePart(ent ecs.Entity, dmg int) (bodyPart, bool) {
	hp := bo.hp[ent.ID()]
	hp -= dmg
	bo.hp[ent.ID()] = hp
	if hp > 0 {
		return bodyPart{}, false
	}
	part := bodyPart{
		Type: ent.Type(),
		Name: bo.PartName(ent),
		Desc: bo.DescribePart(ent),
		bodyStats: bodyStats{
			HP:     bo.hp[ent.ID()],
			MaxHP:  bo.maxHP[ent.ID()],
			Damage: bo.dmg[ent.ID()],
			Armor:  bo.armor[ent.ID()],
		},
	}
	return part, true
}

func (bo *body) sever(ids ...ecs.EntityID) *body {
	type rel struct {
		ent, a, b ecs.Entity
		r         ecs.RelationType
	}

	var (
		cont  body
		xlate = make(map[ecs.EntityID]ecs.EntityID)
		q     = append([]ecs.EntityID(nil), ids...)
		rels  = make([]rel, 0, len(bo.rel.Entities))
		relis = make(map[ecs.EntityID]struct{}, len(bo.rel.Entities))
	)

	for len(q) > 0 {
		id := q[0]
		copy(q, q[1:])
		q = q[:len(q)-1]
		t := bo.Entities[id]
		if !t.All(bcPart) {
			continue
		}

		if bo.hp[id] > 0 {
			eid := cont.AddEntity(t).ID()
			xlate[id] = eid
			cont.fmt[eid] = bo.fmt[id]
			cont.maxHP[eid] = bo.maxHP[id]
			cont.hp[eid] = bo.hp[id]
			cont.dmg[eid] = bo.dmg[id]
			cont.armor[eid] = bo.armor[id]
		}

		// collect affected relations for final processing
		cur := bo.rel.Cursor(ecs.All(ecs.RelType(brControl)), nil)
		for cur.Scan() {
			ent := cur.Entity()
			id := ent.ID()
			if _, seen := relis[id]; !seen {
				relis[id] = struct{}{}
				rels = append(rels, rel{ent, cur.A(), cur.B(), cur.R()})
			}
		}

		bo.Ref(id).Delete(bcPart)

		if len(q) == 0 && (bo.Iter(ecs.All(bcPart|bcHead)).Count() == 0 ||
			bo.Iter(ecs.All(bcPart|bcTorso)).Count() == 0) {
			it := bo.Iter(ecs.All(bcPart))
			for it.Next() {
				q = append(q, it.ID())
			}
		}
	}

	if len(cont.Entities) == 0 {
		return nil
	}

	// finish relation processing
	cont.rel.InsertMany(func(insert func(r ecs.RelationType, a ecs.Entity, b ecs.Entity) ecs.Entity) {
		for _, rel := range rels {
			if xa, def := xlate[rel.a.ID()]; def {
				a := cont.Ref(xa)
				if xb, def := xlate[rel.b.ID()]; def {
					b := cont.Ref(xb)
					insert(brControl, a, b)
				}
			}
			defer rel.ent.Destroy()
		}
	})

	return &cont
}
