package main

import (
	"fmt"
	"math/rand"

	"github.com/jcorbin/execs/internal/ecs"
)

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

type entRel struct{ a, b int }

type body struct {
	ecs.Core

	fmt   []string
	maxHP []int
	hp    []int
	dmg   []int
	armor []int

	controls []entRel
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

func newBody(rng *rand.Rand) *body {
	var bo body

	// TODO: use rng

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

	bo.controls = append(bo.controls,
		entRel{a: head.ID(), b: torso.ID()},
		entRel{a: torso.ID(), b: rightArm.ID()},
		entRel{a: torso.ID(), b: leftArm.ID()},
		entRel{a: torso.ID(), b: rightLeg.ID()},
		entRel{a: torso.ID(), b: leftLeg.ID()},
		entRel{a: rightArm.ID(), b: rightHand.ID()},
		entRel{a: leftArm.ID(), b: leftHand.ID()},
		entRel{a: rightLeg.ID(), b: rightFoot.ID()},
		entRel{a: leftLeg.ID(), b: leftFoot.ID()},
	)

	return &bo
}

func (bo *body) AddPart(t ecs.ComponentType, hp, dmg, armor int) ecs.Entity {
	ent := bo.AddEntity()
	ent.AddComponent(bcHP | bcPart | t)
	id := ent.ID()
	bo.maxHP[id] = hp
	bo.hp[id] = hp
	bo.dmg[id] = dmg
	bo.armor[id] = armor
	return ent
}

func (bo *body) AddEntity() ecs.Entity {
	ent := bo.Core.AddEntity()
	bo.fmt = append(bo.fmt, "")
	bo.maxHP = append(bo.maxHP, 0)
	bo.hp = append(bo.hp, 0)
	bo.dmg = append(bo.dmg, 0)
	bo.armor = append(bo.armor, 0)
	return ent
}

func (bo *body) Stats() bodyStats {
	var s bodyStats
	bo.Descend(0, 0, func(id, level int) bool {
		s.HP += bo.hp[id]
		s.MaxHP += bo.maxHP[id]
		s.Damage += bo.dmg[id]
		s.Armor += bo.armor[id]
		return true
	})
	return s
}

func (bo *body) HPRange() (hp, maxHP int) {
	it := bo.IterAll(bcHP)
	for id, _, ok := it.Next(); ok; id, _, ok = it.Next() {
		hp += bo.hp[id]
		maxHP += bo.maxHP[id]
	}
	return
}

func (bo *body) HP() int {
	hp := 0
	it := bo.IterAll(bcHP)
	for id, _, ok := it.Next(); ok; id, _, ok = it.Next() {
		hp += bo.hp[id]
	}
	return hp
}

func (bo *body) Roots() []int {
	f := make(map[int]struct{}, len(bo.Entities))
	for id, t := range bo.Entities {
		if t != ecs.ComponentNone {
			f[id] = struct{}{}
		}
	}
	for _, rel := range bo.controls {
		delete(f, rel.b)
	}
	r := make([]int, 0, len(f))
	for id := range f {
		r = append(r, id)
	}
	return r
}

func (bo *body) Descend(id, level int, f func(id, level int) bool) {
	if !f(id, level) {
		return
	}
	for _, rel := range bo.controls {
		if rel.a == id {
			bo.Descend(rel.b, level+1, f)
		}
	}
}

func (bo *body) Ascend(id int, f func(id int) bool) {
	if !f(id) {
		return
	}
	for _, rel := range bo.controls {
		if rel.b == id {
			bo.Ascend(rel.a, f)
		}
	}
}

func (bo *body) choosePart(want func(prior, id, level int) bool) ecs.Entity {
	choice := -1
	bo.Descend(0, 0, func(id, level int) bool {
		if !bo.Entities[id].All(bcPart) {
			return false
		}
		if want(choice, id, level) {
			choice = id
		}
		return true
	})
	if choice < 0 {
		return ecs.InvalidEntity
	}
	return bo.Ref(choice)
}

func (bo *body) chooseRandomPart(rng *rand.Rand, score func(id, level int) int) ecs.Entity {
	sum := 0
	return bo.choosePart(func(prior, id, level int) bool {
		if w := score(id, level); w > 0 {
			sum += w
			return prior < 0 || rng.Intn(sum) < w
		}
		return false
	})
}

func (bo *body) PartName(id int) string {
	switch bo.Entities[id] & bcPartMask {
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
	return fmt.Sprintf("?%08x?", []ecs.ComponentType{
		bo.Entities[id],
		bo.Entities[id] & bcPartMask,
	})
	// return ""
}

func (bo *body) DescribePart(id int) string {
	s := bo.PartName(id)
	if s == "" {
		s = "???"
	}
	switch bo.Entities[id] & bcLocMask {
	case bcRight:
		s = "right " + s
	case bcLeft:
		s = "left " + s
	}
	if bo.Entities[id].All(bcName) {
		s = fmt.Sprintf(bo.fmt[id], s)
	}
	return s
}

func (w *world) attack(rng *rand.Rand, srcID, targID int) {
	const numSpiritTurns = 5

	src, targ := w.bodies[srcID], w.bodies[targID]
	srcName := w.getName(srcID, "!?!")
	targName := w.getName(targID, "?!?")

	aPart := src.chooseRandomPart(rng, func(id, level int) int {
		if src.dmg[id] <= 0 {
			return 0
		}
		return 4*src.dmg[id] + 2*src.armor[id] + src.hp[id]
	})
	if aPart == ecs.InvalidEntity {
		w.log("%s has nothing to hit %s with.", srcName, targName)
		return
	}

	bPart := targ.chooseRandomPart(rng, func(id, level int) int {
		if targ.hp[id] <= 0 {
			return 0
		}
		switch targ.Entities[id] & bcPartMask {
		case bcTorso:
			return 100 * (1 + targ.maxHP[id] - targ.hp[id])
		case bcHead:
			return 10 * (1 + targ.maxHP[id] - targ.hp[id])
		}
		return 0
	})
	if bPart == ecs.InvalidEntity {
		w.log("%s can find nothing worth hitting on %s.", srcName, targName)
		return
	}

	aEff, bEff := 1.0, 1.0
	src.Ascend(aPart.ID(), func(id int) bool {
		aEff *= float64(src.hp[id]) / float64(src.maxHP[id])
		return true
	})
	targ.Ascend(bPart.ID(), func(id int) bool {
		bEff *= float64(targ.hp[id]) / float64(targ.maxHP[id])
		return true
	})

	aDesc := src.DescribePart(aPart.ID())
	bDesc := targ.DescribePart(bPart.ID())

	if 1.0-bEff*rng.Float64()/aEff*rng.Float64() < -1.0 {
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
	part, destroyed := targ.damagePart(bPart.ID(), dmg)
	w.log("%s's %s dealt %v damage to %s's %s", srcName, aDesc, dmg, targName, bDesc)
	if !destroyed {
		return
	}

	if severed := targ.sever(bPart.ID()); severed != nil {
		w.newItem(w.Positions[targID], fmt.Sprintf("remains of %s", targName), '%', severed)
		if roots := severed.Roots(); len(roots) > 0 {
			for _, id := range roots {
				w.log("%s's %s has dropped on the floor", targName, severed.DescribePart(id))
			}
		}
	}

	if bo := w.bodies[targID]; bo.CountAll(bcPart) > 0 {
		w.log("%s's %s destroyed by %s's %s", targName, bDesc, srcName, aDesc)
		return
	}

	// head not destroyed -> become spirit
	if part.Type.All(bcHead) {
		w.DestroyEntity(targID)
		w.log("%s destroyed by %s (headshot)", targName, srcName)
	} else {
		w.Glyphs[targID] = '⟡'
		w.Entities[targID] &= ^wcBody
		w.bodies[targID] = nil
		w.setTimer(targID, numSpiritTurns, timerDestroy)
		w.log("%s was disembodied by %s (moving as spirit for %v turns)", targName, srcName, numSpiritTurns)
	}
}

func (bo *body) damagePart(id, dmg int) (bodyPart, bool) {
	hp := bo.hp[id]
	hp -= dmg
	bo.hp[id] = hp
	if hp > 0 {
		return bodyPart{}, false
	}
	part := bodyPart{
		Type: bo.Entities[id],
		Name: bo.PartName(id),
		Desc: bo.DescribePart(id),
		bodyStats: bodyStats{
			HP:     bo.hp[id],
			MaxHP:  bo.maxHP[id],
			Damage: bo.dmg[id],
			Armor:  bo.armor[id],
		},
	}
	return part, true
}

func (bo *body) sever(ids ...int) *body {
	var (
		cont  body
		xlate = make(map[int]int)
		q     = append([]int(nil), ids...)
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
			eid := cont.AddEntity().ID()
			xlate[id] = eid
			cont.Entities[eid] = t
			cont.fmt[eid] = bo.fmt[id]
			cont.maxHP[eid] = bo.maxHP[id]
			cont.hp[eid] = bo.hp[id]
			cont.dmg[eid] = bo.dmg[id]
			cont.armor[eid] = bo.armor[id]
		}

		// collect affected relations, will translate later
		for i := 0; i < len(bo.controls)-1; i++ {
			rel := bo.controls[i]
			if rel.a == id || rel.b == id {
				j := len(bo.controls) - 1
				bo.controls[i] = bo.controls[j]
				bo.controls = bo.controls[:j]
				i--
				cont.controls = append(cont.controls, rel)
			}
		}

		bo.Entities[id] &= ^bcPart

		if len(q) == 0 && (bo.CountAll(bcPart|bcHead) == 0 || bo.CountAll(bcPart|bcTorso) == 0) {
			for id, t := range bo.Entities {
				if t.All(bcPart) {
					q = append(q, id)
				}
			}
		}
	}

	if len(cont.Entities) == 0 {
		return nil
	}

	// now translate any collected relations
	for i := 0; i < len(cont.controls); i++ {
		if xa, def := xlate[cont.controls[i].a]; def {
			if xb, def := xlate[cont.controls[i].b]; def {
				cont.controls[i] = entRel{a: xa, b: xb}
				continue
			}
		}
		j := len(cont.controls) - 1
		cont.controls[i] = cont.controls[j]
		cont.controls = cont.controls[:j]
		i--
	}

	return &cont
}
