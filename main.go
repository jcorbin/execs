package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jcorbin/execs/internal/ecs"
	"github.com/jcorbin/execs/internal/point"
	"github.com/jcorbin/execs/internal/view"
	termbox "github.com/nsf/termbox-go"
)

// TODO: spirit possession
// TODO: agro system
// TODO: movement based on body status
// TODO: more body parts: thigh/calf, forearm/upper, neck, fingers, toes, organs, joints, items
// TODO: factor out something like ecs.Relation

const (
	wcName ecs.ComponentType = 1 << iota
	wcTimer
	wcPosition
	wcCollide
	wcGlyph
	wcBG
	wcFG
	wcInput
	wcBody
	wcSoul
	wcItem
	wcAI
)

const (
	renderMask   = wcPosition | wcGlyph
	playMoveMask = wcPosition | wcInput | wcSoul
	aiMoveMask   = wcPosition | wcInput | wcAI
	collMask     = wcPosition | wcCollide
	combatMask   = wcCollide | wcBody
)

type world struct {
	logFile *os.File
	logger  *log.Logger
	View    *view.View
	ecs.Core

	Names     []string
	Positions []point.Point
	Glyphs    []rune
	BG        []termbox.Attribute
	FG        []termbox.Attribute
	timers    []timer
	bodies    []*body
	items     []interface{}

	coll  []int
	colls []collision
}

type timer struct {
	n int
	a timerAction
	t ecs.ComponentType
	f func(ecs.Entity)
}

type timerAction uint8

const (
	timerDestroy timerAction = iota
	timerSetType
	timerCallback
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

type collision struct {
	sourceID int
	targetID int
}

func (w *world) AddEntity() ecs.Entity {
	ent := w.Core.AddEntity()
	w.Names = append(w.Names, "")
	w.Positions = append(w.Positions, point.Point{})
	w.Glyphs = append(w.Glyphs, 0)
	w.BG = append(w.BG, 0)
	w.FG = append(w.FG, 0)
	w.bodies = append(w.bodies, nil)
	w.items = append(w.items, nil)
	w.timers = append(w.timers, timer{})
	return ent
}

func (w *world) DestroyEntity(id int) {
	w.Entities[id] = ecs.ComponentNone
	w.bodies[id] = nil
}

func (w *world) Render(ctx *view.Context) error {
	ctx.SetHeader(
		fmt.Sprintf("%v souls v %v demons", w.CountAll(wcSoul), w.CountAll(wcAI)),
	)

	it := w.IterAll(wcSoul | wcBody)
	hpParts := make([]string, 0, it.Count())
	for id, _, ok := it.Next(); ok; id, _, ok = it.Next() {
		bo := w.bodies[id]
		parts := make([]string, 0, len(bo.Entities))
		bo.Descend(0, 0, func(id, level int) bool {
			parts = append(parts, fmt.Sprintf("%s:%v", bo.DescribePart(id), bo.hp[id]))
			return true
		})
		hp, maxHP := bo.HPRange()

		// TODO: render a doll like
		//    _O_
		//   / | \
		//   = | =
		//    / \
		//  _/   \_

		hpParts = append(hpParts, fmt.Sprintf("HP(%v/%v):<%s>", hp, maxHP, strings.Join(parts, " ")))
		// hpParts = append(hpParts, fmt.Sprintf("HP(%v/%v)", hp, maxHP))
	}
	ctx.SetFooter(hpParts...)

	// collect world extent, and compute a viewport focus position
	var (
		bbox  point.Box
		focus point.Point
	)
	it = w.IterAll(renderMask)
	for id, t, ok := it.Next(); ok; id, t, ok = it.Next() {
		pos := w.Positions[id]
		if t&wcSoul != 0 {
			// TODO: centroid between all souls would be a way to move beyond
			// "last wins"
			focus = pos
		}
		bbox = bbox.ExpandTo(pos)
	}

	// center clamped grid around focus
	offset := bbox.TopLeft.Add(bbox.Size().Div(2)).Sub(focus)
	ofbox := bbox.Add(offset)
	if ofbox.TopLeft.X < 0 {
		offset.X -= ofbox.TopLeft.X
	}
	if ofbox.TopLeft.Y < 0 {
		offset.Y -= ofbox.TopLeft.Y
	}

	if sz := ofbox.Size().Min(ctx.Avail); !sz.Equal(ctx.Grid.Size) {
		ctx.Grid = view.MakeGrid(sz)
	} else {
		for i := range ctx.Grid.Data {
			ctx.Grid.Data[i] = termbox.Cell{}
		}
	}

	zVals := make([]uint8, len(ctx.Grid.Data))

	it = w.Iter(wcPosition, wcGlyph|wcBG)
	for id, t, ok := it.Next(); ok; id, t, ok = it.Next() {
		pos := w.Positions[id].Add(offset)
		if !pos.Less(point.Zero) && !ctx.Grid.Size.Less(pos) {
			var (
				ch     rune
				fg, bg termbox.Attribute
			)

			var zVal uint8

			if t.All(wcGlyph) {
				ch = w.Glyphs[id]
				zVal = 1
			}

			// TODO: move to hp update
			if t.All(wcBody) && t.Any(wcSoul|wcAI) {
				zVal = 255
				colors := soulColors
				if !t.All(wcSoul) {
					zVal--
					colors = aiColors
				}

				hp, maxHP := w.bodies[id].HPRange()
				// XXX range error FIXME
				i := 1 + (len(colors)-2)*hp/maxHP
				if i < 0 {
					fg = colors[1]
					w.log("lo color index hp=%v maxHP=%v", hp, maxHP)
				} else if i >= len(colors) {
					fg = colors[len(colors)-1]
					w.log("hi color index hp=%v maxHP=%v", hp, maxHP)
				} else {
					fg = colors[i]
				}

			} else if t.All(wcSoul) {
				zVal = 127
				fg = soulColors[0]
			} else if t.All(wcAI) {
				zVal = 126
				fg = aiColors[0]
			} else if t.All(wcItem) {
				zVal = 10
				fg = 35
			} else {
				zVal = 2
				if t.All(wcFG) {
					fg = w.FG[id]
				}
			}

			if i := pos.Y*ctx.Grid.Size.X + pos.X; zVals[i] < zVal {
				zVals[i] = zVal
				if t.All(wcBG) {
					bg = w.BG[id]
				}
				if fg != 0 {
					fg++
				}
				if bg != 0 {
					bg++
				}
				ctx.Grid.Merge(pos.X, pos.Y, ch, fg, bg)
			}
		}
	}

	return nil
}

func (w *world) Close() error { return nil }

func (w *world) HandleKey(v *view.View, k view.KeyEvent) error {
	if k.Key == termbox.KeyEsc {
		return view.ErrStop
	}

	v.ClearLog()

	rng := rand.New(rand.NewSource(rand.Int63()))

	// run timers
	it := w.IterAll(wcTimer)
	for id, _, ok := it.Next(); ok; id, _, ok = it.Next() {
		if w.timers[id].n <= 0 {
			continue
		}
		w.timers[id].n--
		if w.timers[id].n > 0 {
			continue
		}
		w.Entities[id] &= ^wcTimer
		switch w.timers[id].a {
		case timerDestroy:
			w.DestroyEntity(id)
		case timerSetType:
			w.Entities[id] = w.timers[id].t
		case timerCallback:
			f := w.timers[id].f
			if f != nil {
				w.timers[id].f = nil
				f(w.Ref(id))
			}
		}
	}

	// collect collidables
	w.prepareCollidables()

	// ai chase target; last one wins for now TODO
	var target point.Point

	// apply player move
	if move, ok := key2move(k); ok {
		it := w.IterAll(playMoveMask)
		for id, _, ok := it.Next(); ok; id, _, ok = it.Next() {
			target = w.move(id, move)
		}
	}

	// chase player
	it = w.IterAll(aiMoveMask)
	for id, _, ok := it.Next(); ok; id, _, ok = it.Next() {
		w.move(id, target.Sub(w.Positions[id]).Sign())
	}

	// collisions deal damage
	for _, coll := range w.colls {
		if w.Entities[coll.sourceID]&combatMask != combatMask {
			continue
		}
		if w.Entities[coll.targetID]&combatMask != combatMask {
			continue
		}
		w.attack(rng, coll.sourceID, coll.targetID)
	}

	// count remaining souls
	if w.CountAll(wcSoul) == 0 {
		w.log("game over")
		return view.ErrStop
	}

	// maybe spawn
	// TODO: randomize position?
	if _, occupied := w.collides(len(w.Entities), point.Zero); !occupied {
		sum := 0
		it := w.IterAll(wcBody)
		for id, _, ok := it.Next(); ok; id, _, ok = it.Next() {
			sum += w.bodies[id].HP()
		}
		bo := newBody(rng)
		if hp := bo.HP(); rng.Intn(sum+hp) < hp {
			enemy := w.newChar("enemy", 'X', bo)
			enemy.AddComponent(wcInput | wcAI)
			id := enemy.ID()
			w.log("%s enters the world @%v stats: %+v", w.Names[id], w.Positions[id], w.bodies[id].Stats())
		}
	}

	return nil
}

func (w *world) setTimer(id, n int, a timerAction) *timer {
	w.Entities[id] |= wcTimer
	w.timers[id] = timer{n: n, a: a}
	return &w.timers[id]
}

func (w *world) addBox(box point.Box, glyph rune) {
	rng := rand.New(rand.NewSource(rand.Int63()))

	// TODO: the box should be an entity, rather than each cell
	last, sz, pos := wallTable.Ref(0), box.Size(), box.TopLeft
	for _, r := range []struct {
		n int
		d point.Point
	}{
		{n: sz.X, d: point.Point{X: 1}},
		{n: sz.Y, d: point.Point{Y: 1}},
		{n: sz.X, d: point.Point{X: -1}},
		{n: sz.Y, d: point.Point{Y: -1}},
	} {
		for i := 0; i < r.n; i++ {
			wall := w.AddEntity()
			wall.AddComponent(
				wcPosition | wcCollide |
					wcGlyph | wcBG | wcFG)
			id := wall.ID()
			w.Glyphs[id] = glyph
			w.Positions[id] = pos
			c, _ := wallTable.toColor(last)
			w.BG[id] = c
			w.FG[id] = c + 1
			pos = pos.Add(r.d)
			last = wallTable.ChooseNext(rng, last)
		}
	}

	floorTable.genTile(rng, box, func(pos point.Point, bg termbox.Attribute) {
		floor := w.AddEntity()
		floor.AddComponent(wcPosition | wcBG)
		w.Positions[floor.ID()] = pos
		w.BG[floor.ID()] = bg
	})
}

func (w *world) newItem(pos point.Point, name string, glyph rune, val interface{}) ecs.Entity {
	ent := w.AddEntity()
	ent.AddComponent(wcPosition | wcName | wcGlyph | wcItem)
	id := ent.ID()
	w.Positions[id] = pos
	w.Glyphs[id] = glyph
	w.Names[id] = name
	w.items[id] = val
	return ent
}

func (w *world) newChar(name string, glyph rune, bo *body) ecs.Entity {
	rng := rand.New(rand.NewSource(rand.Int63()))

	ent := w.AddEntity()
	ent.AddComponent(
		wcPosition | wcCollide |
			wcName | wcGlyph | wcBody)

	id := ent.ID()
	w.Glyphs[id] = glyph
	w.Positions[id] = point.Zero
	w.Names[id] = name
	if bo == nil {
		bo = newBody(rng)
	}
	w.bodies[id] = bo
	return ent
}

func (w *world) log(mess string, args ...interface{}) {
	s := fmt.Sprintf(mess, args...)
	for _, rule := range []struct{ old, new string }{
		{"you's", "your"},
		{"them's", "their"},
		{"you has", "you have"},
		{"you was", "you were"},
	} {
		s = strings.Replace(s, rule.old, rule.new, -1)
	}
	w.View.Log(s)
	w.logger.Printf(s)
}

func (w *world) getName(id int, deflt string) string {
	if w.Entities[id]&wcName == 0 {
		return deflt
	}
	if w.Names[id] == "" {
		return deflt
	}
	return w.Names[id]
}

func (w *world) prepareCollidables() {
	// TODO: maintain a cleverer structure, like a quad-tree, instead
	it := w.IterAll(collMask)
	if n := it.Count(); cap(w.coll) < n {
		w.coll = make([]int, 0, n)
	} else {
		w.coll = w.coll[:0]
	}
	for id, _, ok := it.Next(); ok; id, _, ok = it.Next() {
		w.coll = append(w.coll, id)
	}

	if n := len(w.coll) * len(w.coll); cap(w.colls) < n {
		w.colls = make([]collision, 0, n)
	} else {
		w.colls = w.colls[:0]
	}
	sort.Slice(w.colls, func(i, j int) bool {
		return w.Positions[w.coll[i]].Less(w.Positions[w.coll[j]])
	})
}

func (w *world) collides(id int, pos point.Point) (int, bool) {
	for _, hitID := range w.coll {
		if hitID != id {
			if hitPos := w.Positions[hitID]; hitPos.Equal(pos) {
				return hitID, true
			}
		}
	}
	return 0, false

	// TODO: binary search
	// i := sort.Search(len(w.coll), func(i int) bool {
	// 	return w.Positions[w.coll[i]].Less(pos)
	// })
	// if i < len(w.coll) {
	// 	return w.coll[i], w.coll[i] != id && w.Positions[w.coll[i]].Equal(pos)
	// }
	// return 0, false
}

func (w *world) move(id int, move point.Point) point.Point {
	pos := w.Positions[id]
	new := pos.Add(move)
	if hitID, hit := w.collides(id, new); hit {
		coll := collision{id, hitID}
		w.colls = append(w.colls, coll)
	} else {
		pos = new
		w.Positions[id] = pos
	}
	return pos
}

func d6() int { return rand.Intn(5) + 1 }

func rollStat() int {
	a, b, c, d := d6(), d6(), d6(), d6()
	if d < c {
		c, d = d, c
	}
	if c < b {
		b, c = c, b
	}
	if b < a {
		a, b = b, a
	}
	return c + c + b
}

func key2move(k view.KeyEvent) (point.Point, bool) {
	switch k.Key {
	case termbox.KeyArrowDown:
		return point.Point{X: 0, Y: 1}, true
	case termbox.KeyArrowUp:
		return point.Point{X: 0, Y: -1}, true
	case termbox.KeyArrowLeft:
		return point.Point{X: -1, Y: 0}, true
	case termbox.KeyArrowRight:
		return point.Point{X: 1, Y: 0}, true
	}
	return point.Zero, true
}

func main() {
	if err := view.JustKeepRunning(func(v *view.View) (view.Client, error) {
		var w world
		w.View = v

		// TODO: world.rng probably
		// rng := rand.New(rand.NewSource(rand.Int63()))

		f, err := os.Create(fmt.Sprintf("%v.log", time.Now().Format(time.RFC3339)))
		if err != nil {
			return nil, err
		}
		w.logger = log.New(f, "", 0)
		w.log("logging to %q", f.Name())

		pt := point.Point{X: 12, Y: 8}

		w.addBox(point.Box{TopLeft: pt.Neg(), BottomRight: pt}, '#')
		player := w.newChar("you", 'X', nil)
		player.AddComponent(wcInput | wcSoul)
		id := player.ID()
		w.log("%s enter the world @%v stats: %+v", w.Names[id], w.Positions[id], w.bodies[id].Stats())

		// bo := w.bodies[id]
		// bo.Descend(0, 0, func(id, level int) bool {
		// 	if level == 0 {
		// 		w.log("the %s is connected to the...", bo.DescribePart(id))
		// 	} else {
		// 		w.log("%s...%s; is connected to the...", strings.Repeat("  ", level), bo.DescribePart(id))
		// 	}
		// 	return true
		// })

		return &w, nil
	}); err != nil {
		log.Fatal(err)
	}
}
