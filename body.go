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
	rel ecs.Graph

	fmt   []string
	maxHP []int
	hp    []int
	dmg   []int
	armor []int
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
	bo.rel.Init(&bo.Core, 0)
	bo.RegisterAllocator(bcPart, bo.allocPart)
	return bo
}

func (bo *body) Clear() {
	bo.rel.Clear()
	bo.Core.Clear()
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
	for it := bo.Iter(ecs.All(bcHP | bcPart)); it.Next(); {
		s.HP += bo.hp[it.ID()]
		s.MaxHP += bo.maxHP[it.ID()]
		s.Damage += bo.dmg[it.ID()]
		s.Armor += bo.armor[it.ID()]
	}
	return s
}

func (bo *body) HPRange() (hp, maxHP int) {
	for it := bo.Iter(ecs.All(bcHP)); it.Next(); {
		hp += bo.hp[it.ID()]
		maxHP += bo.maxHP[it.ID()]
	}
	return
}

func (bo *body) HP() int {
	hp := 0
	for it := bo.Iter(ecs.All(bcHP)); it.Next(); {
		hp += bo.hp[it.ID()]
	}
	return hp
}

func (bo *body) choosePart(want func(prior, ent ecs.Entity) bool) ecs.Entity {
	var choice ecs.Entity
	for it := bo.Iter(ecs.All(bcPart | bcHP)); it.Next(); {
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

func (bo *body) damagePart(ent ecs.Entity, dmg int) (int, bodyPart, bool) {
	hp := bo.hp[ent.ID()]
	if dmg < hp {
		bo.hp[ent.ID()] = hp - dmg
		return dmg, bodyPart{}, false
	}
	bo.hp[ent.ID()] = 0
	return hp, bodyPart{
		Type: ent.Type(),
		Name: bo.PartName(ent),
		Desc: bo.DescribePart(ent),
		bodyStats: bodyStats{
			HP:     bo.hp[ent.ID()],
			MaxHP:  bo.maxHP[ent.ID()],
			Damage: bo.dmg[ent.ID()],
			Armor:  bo.armor[ent.ID()],
		},
	}, true
}

func (bo *body) spiritScore() int {
	n := 0
	for it := bo.Iter(ecs.All(bcHead)); it.Next(); {
		n += bo.hp[it.ID()]
	}
	return n
}

func (bo *body) partHPRating(ent ecs.Entity) float64 {
	rating := 1.0
	for gt := bo.rel.Traverse(ecs.AllRel(brControl), ecs.TraverseCoDFS, ent.ID()); gt.Traverse(); {
		id := gt.Node().ID()
		rating *= float64(bo.hp[id]) / float64(bo.maxHP[id])
	}
	return rating
}

func (bo *body) sever(ids ...ecs.EntityID) *body {
	type rel struct {
		ent, a, b ecs.Entity
		r         ecs.RelationType
	}

	var (
		cont  = newBody()
		xlate = make(map[ecs.EntityID]ecs.EntityID)
		q     = append([]ecs.EntityID(nil), ids...)
		n     = bo.rel.Len()
		rels  = make([]rel, 0, n)
		relis = make(map[ecs.EntityID]struct{}, n)
	)

	for len(q) > 0 {
		id := q[0]
		copy(q, q[1:])
		q = q[:len(q)-1]

		ent := bo.Ref(id)
		if !ent.Type().All(bcPart) {
			continue
		}

		if bo.hp[id] > 0 {
			eid := cont.AddEntity(ent.Type()).ID()
			xlate[id] = eid
			cont.fmt[eid] = bo.fmt[id]
			cont.maxHP[eid] = bo.maxHP[id]
			cont.hp[eid] = bo.hp[id]
			cont.dmg[eid] = bo.dmg[id]
			cont.armor[eid] = bo.armor[id]
		}

		// collect affected relations for final processing
		cur := bo.rel.Cursor(ecs.AllRel(brControl), nil)
		for cur.Scan() {
			ent := cur.Entity()
			id := ent.ID()
			if _, seen := relis[id]; !seen {
				relis[id] = struct{}{}
				rels = append(rels, rel{ent, cur.A(), cur.B(), cur.R()})
			}
		}

		ent.Delete(bcPart)

		if len(q) == 0 && (bo.Iter(ecs.All(bcPart|bcHead)).Count() == 0 ||
			bo.Iter(ecs.All(bcPart|bcTorso)).Count() == 0) {
			it := bo.Iter(ecs.All(bcPart))
			for it.Next() {
				q = append(q, it.ID())
			}
		}
	}

	if cont.Len() == 0 {
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

	return cont
}
