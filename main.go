package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
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
	wcWaiting
	wcBody
	wcSoul
	wcItem
	wcAI
)

const (
	renderMask   = wcPosition | wcGlyph
	playMoveMask = wcPosition | wcInput | wcSoul
	charMask     = wcName | wcGlyph | wcBody
	aiMoveMask   = wcPosition | wcInput | wcAI
	collMask     = wcPosition | wcCollide
	combatMask   = wcCollide | wcBody
)

type world struct {
	logFile *os.File
	logger  *log.Logger
	rng     *rand.Rand
	View    *view.View
	ecs.Core

	over         bool
	playerMove   point.Point
	enemyCounter int

	Names     []string
	Positions []point.Point
	Glyphs    []rune
	BG        []termbox.Attribute
	FG        []termbox.Attribute
	timers    []timer
	bodies    []*body
	items     []interface{}

	coll  []ecs.EntityID // TODO: use an index structure for this
	moves moves
}

type moves struct {
	ecs.Relation
	n []int
}

const (
	movN ecs.ComponentType = 1 << iota

	mrCollide ecs.RelationType = 1 << iota
	mrGoal
	mrAgro
	mrDamage
	mrKill
)

func (mov *moves) init(core *ecs.Core) {
	mov.Relation.Init(core, 0, core, 0)
	mov.n = []int{0}
	mov.RegisterAllocator(movN, mov.allocN)
}

func (mov *moves) allocN(id ecs.EntityID, t ecs.ComponentType) {
	mov.n = append(mov.n, 0)
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
	// timerSetType FIXME
	timerCallback
)

func newWorld(v *view.View) (*world, error) {
	w := &world{
		rng:  rand.New(rand.NewSource(rand.Int63())),
		View: v,

		// TODO: consider eliminating the padding for EntityID(0)
		Names:     []string{""},
		Positions: []point.Point{point.Point{}},
		Glyphs:    []rune{0},
		BG:        []termbox.Attribute{0},
		FG:        []termbox.Attribute{0},
		timers:    []timer{timer{}},
		bodies:    []*body{nil},
		items:     []interface{}{nil},
	}
	w.moves.init(&w.Core)

	f, err := os.Create(fmt.Sprintf("%v.log", time.Now().Format(time.RFC3339)))
	if err != nil {
		return nil, err
	}
	w.logger = log.New(f, "", 0)
	w.log("logging to %q", f.Name())

	w.RegisterAllocator(wcName|wcPosition|wcGlyph|wcBG|wcFG|wcBody|wcItem|wcTimer, w.allocWorld)
	w.RegisterCreator(wcBody, w.createBody)
	w.RegisterDestroyer(wcBody, w.destroyBody)
	w.RegisterDestroyer(wcItem, w.destroyItem)

	return w, nil
}

func (w *world) allocWorld(id ecs.EntityID, t ecs.ComponentType) {
	w.Names = append(w.Names, "")
	w.Positions = append(w.Positions, point.Point{})
	w.Glyphs = append(w.Glyphs, 0)
	w.BG = append(w.BG, 0)
	w.FG = append(w.FG, 0)
	w.bodies = append(w.bodies, nil)
	w.items = append(w.items, nil)
	w.timers = append(w.timers, timer{})
}

func (w *world) createBody(id ecs.EntityID, t ecs.ComponentType) {
	w.bodies[id] = newBody()
}

func (w *world) destroyBody(id ecs.EntityID, t ecs.ComponentType) {
	// TODO: could reset the body ecs for re-use
	if bo := w.bodies[id]; bo != nil {
		if !bo.Empty() {
			name := w.getName(w.Ref(id), "???")
			name = fmt.Sprintf("remains of %s", name)
			w.newItem(w.Positions[id], name, '%', bo)
			w.bodies[id] = nil
		}
	}
}

func (w *world) destroyItem(id ecs.EntityID, t ecs.ComponentType) {
	item := w.items[id]
	w.items[id] = nil
	switch v := item.(type) {
	case *body:
		for i := range w.bodies {
			if i > 0 && w.bodies[i] == nil {
				v.Clear()
				w.bodies[i] = v
				break
			}
		}
	}
}

func (w *world) Render(ctx *view.Context) error {
	ctx.SetHeader(
		fmt.Sprintf("%v souls v %v demons", w.Iter(ecs.All(wcSoul)).Count(), w.Iter(ecs.All(wcAI)).Count()),
	)

	it := w.Iter(ecs.All(wcSoul | wcBody))
	hpParts := make([]string, 0, it.Count())
	for it.Next() {
		bo := w.bodies[it.ID()]

		parts := make([]string, 0, bo.Len())
		for gt := bo.rel.Traverse(ecs.AllRel(brControl), ecs.TraverseDFS); gt.Traverse(); {
			ent := gt.Node()
			parts = append(parts, fmt.Sprintf("%s:%v", bo.DescribePart(ent), bo.hp[ent.ID()]))
		}

		// TODO: render a doll like
		//    _O_
		//   / | \
		//   = | =
		//    / \
		//  _/   \_

		hp, maxHP := bo.HPRange()
		hpParts = append(hpParts, fmt.Sprintf("HP(%v/%v):<%s>", hp, maxHP, strings.Join(parts, " ")))
		// hpParts = append(hpParts, fmt.Sprintf("HP(%v/%v)", hp, maxHP))
	}
	ctx.SetFooter(hpParts...)

	// collect world extent, and compute a viewport focus position
	var (
		bbox  point.Box
		focus point.Point
	)
	for it := w.Iter(ecs.All(renderMask)); it.Next(); {
		pos := w.Positions[it.ID()]
		if it.Type().All(wcSoul) {
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

	for it := w.Iter(ecs.Clause(wcPosition, wcGlyph|wcBG)); it.Next(); {
		pos := w.Positions[it.ID()].Add(offset)
		if !pos.Less(point.Zero) && !ctx.Grid.Size.Less(pos) {
			var (
				ch     rune
				fg, bg termbox.Attribute
			)

			var zVal uint8

			if it.Type().All(wcGlyph) {
				ch = w.Glyphs[it.ID()]
				zVal = 1
			}

			// TODO: move to hp update
			if it.Type().All(wcBody) && it.Type().Any(wcSoul|wcAI) {
				zVal = 255
				colors := soulColors
				if !it.Type().All(wcSoul) {
					zVal--
					colors = aiColors
				}

				hp, maxHP := w.bodies[it.ID()].HPRange()
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

			} else if it.Type().All(wcSoul) {
				zVal = 127
				fg = soulColors[0]
			} else if it.Type().All(wcAI) {
				zVal = 126
				fg = aiColors[0]
			} else if it.Type().All(wcItem) {
				zVal = 10
				fg = 35
			} else {
				zVal = 2
				if it.Type().All(wcFG) {
					fg = w.FG[it.ID()]
				}
			}

			if i := pos.Y*ctx.Grid.Size.X + pos.X; zVals[i] < zVal {
				zVals[i] = zVal
				if it.Type().All(wcBG) {
					bg = w.BG[it.ID()]
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

func (w *world) extent() point.Box {
	var bbox point.Box
	for it := w.Iter(ecs.All(renderMask)); it.Next(); {
		bbox = bbox.ExpandTo(w.Positions[it.ID()])
	}
	return bbox
}

func (w *world) Close() error { return nil }

func (w *world) HandleKey(v *view.View, k view.KeyEvent) error {
	// parse player move
	switch k.Key {
	case termbox.KeyEsc:
		return view.ErrStop
	case termbox.KeyArrowDown:
		w.playerMove = point.Point{X: 0, Y: 1}
	case termbox.KeyArrowUp:
		w.playerMove = point.Point{X: 0, Y: -1}
	case termbox.KeyArrowLeft:
		w.playerMove = point.Point{X: -1, Y: 0}
	case termbox.KeyArrowRight:
		w.playerMove = point.Point{X: 1, Y: 0}
	default:
		w.playerMove = point.Zero
	}

	w.Process()

	if w.over {
		return view.ErrStop
	}
	return nil
}

func (w *world) Process() {
	w.reset()             // reset state from last time
	w.tick()              // run timers
	w.applyMoves()        // player and ai
	w.processCollisions() // e.g. deal damage
	w.checkOver()         // no souls => done
	w.maybeSpawn()        // spawn more demons
}

func (w *world) reset() {
	w.View.ClearLog()

	// reset collisions, damage, and kills
	w.moves.Delete(ecs.AnyRel(mrCollide|mrDamage|mrKill), nil)

	// collect collidables
	w.prepareCollidables()
}

func (w *world) tick() {
	// run timers
	for it := w.Iter(ecs.All(wcTimer)); it.Next(); {
		if w.timers[it.ID()].n <= 0 {
			continue
		}
		w.timers[it.ID()].n--
		if w.timers[it.ID()].n > 0 {
			continue
		}
		ent := it.Entity()
		ent.Delete(wcTimer)
		switch w.timers[it.ID()].a {
		case timerDestroy:
			ent.Destroy()
			// FIXME
			// case timerSetType:
			//	 w.Ref(it.ID()).SetType(w.timers[it.ID()].t)
		case timerCallback:
			f := w.timers[it.ID()].f
			if f != nil {
				w.timers[it.ID()].f = nil
				f(ent)
			}
		}
	}
}

func (w *world) applyMoves() {
	// apply player move
	for it := w.Iter(ecs.All(playMoveMask)); it.Next(); {
		w.move(it.Entity(), w.playerMove)
	}

	// generate ai moves
	for it := w.Iter(ecs.All(aiMoveMask)); it.Next(); {
		if target, found := w.aiTarget(it.Entity()); found {
			// Move towards the target!
			w.move(it.Entity(), target.Sub(w.Positions[it.ID()]).Sign())
		} else {
			// No? give up and just randomly budge then!
			w.move(it.Entity(), point.Point{
				X: w.rng.Intn(3) - 1,
				Y: w.rng.Intn(3) - 1,
			})
		}
	}
}

func (w *world) aiTarget(ai ecs.Entity) (point.Point, bool) {
	// chase the thing we hate the most
	opp, hate := ecs.NilEntity, 0
	for cur := w.moves.LookupA(ecs.AllRel(mrAgro), ai.ID()); cur.Scan(); {
		if ent := cur.Entity(); ent.Type().All(movN) {
			// TODO: take other factors like distance into account
			if n := w.moves.n[ent.ID()]; n > hate {
				if b := cur.B(); b.Type().All(combatMask) {
					opp, hate = b, n
				}
			}
		}
	}
	if opp != ecs.NilEntity {
		w.log("%v hates %v the most", w.getName(ai, "?!?"), w.getName(opp, "!?!"))
		return w.Positions[opp.ID()], true
	}

	// revert to our goal...
	for cur := w.moves.LookupA(ecs.AllRel(mrGoal), ai.ID()); cur.Scan(); {
		if b := cur.B(); b.Type().All(wcPosition) {
			return w.Positions[b.ID()], true
		}
		// TODO: bogus goal, should delete it
	}

	// ... no goal, pick one randomly!
	myPos := w.Positions[ai.ID()]
	goal, sum := ecs.NilEntity, 0
	for it := w.Iter(ecs.All(collMask)); it.Next(); {
		if !it.Type().All(combatMask) {
			pos := w.Positions[it.ID()]
			diff := pos.Sub(myPos)
			quad := diff.X*diff.X + diff.Y*diff.Y
			sum += quad
			if w.rng.Intn(sum) < quad {
				goal = it.Entity()
			}
		}
	}
	if goal != ecs.NilEntity {
		w.moves.Insert(mrGoal, ai, goal)
		return w.Positions[goal.ID()], true
	}

	return point.Zero, false
}

func (w *world) processCollisions() {
	// collisions deal damage
	for cur := w.moves.Cursor(ecs.AllRel(mrCollide), func(ent, a, b ecs.Entity, r ecs.RelationType) bool {
		return a.Type().All(combatMask) && b.Type().All(combatMask)
	}); cur.Scan(); {
		w.attack(cur.A(), cur.B()) // TODO: subsume
		// TODO: store damage and kill relations, update agro relations
	}
}

func (w *world) checkOver() {
	// count remaining souls
	if w.Iter(ecs.All(wcSoul)).Count() == 0 {
		w.log("game over")
		w.over = true
	}
}

func (w *world) maybeSpawn() {
	// TODO: randomize position?
	hit := w.collides(w.Ref(0), point.Zero)
	if hit != ecs.NilEntity {
		return
	}

	var enemy ecs.Entity
	if it := w.Iter(ecs.All(charMask | wcWaiting)); it.Next() {
		enemy = it.Entity()
	} else {
		w.enemyCounter++
		enemy = w.newChar(fmt.Sprintf("enemy%d", w.enemyCounter), 'X')
		enemy.Add(wcWaiting)
	}
	bo := w.bodies[enemy.ID()]

	sum := 0
	for it := w.Iter(ecs.All(combatMask)); it.Next(); {
		if !it.Type().All(wcWaiting) {
			sum += w.bodies[it.ID()].HP()
		}
	}
	if hp := bo.HP(); w.rng.Intn(sum+hp) < hp {
		enemy.Delete(wcWaiting)
		enemy.Add(wcPosition | wcCollide | wcInput | wcAI)
		w.log("%s enters the world @%v stats: %+v",
			w.Names[enemy.ID()],
			w.Positions[enemy.ID()],
			bo.Stats())
	}
}

func (w *world) attack(src, targ ecs.Entity) {
	const numSpiritTurns = 5

	srcName := w.getName(src, "!?!")
	targName := w.getName(targ, "?!?")

	srcBo, targBo := w.bodies[src.ID()], w.bodies[targ.ID()]

	aPart := srcBo.chooseRandomPart(w.rng, func(ent ecs.Entity) int {
		if srcBo.dmg[ent.ID()] <= 0 {
			return 0
		}
		return 4*srcBo.dmg[ent.ID()] + 2*srcBo.armor[ent.ID()] + srcBo.hp[ent.ID()]
	})
	if aPart == ecs.NilEntity {
		w.log("%s has nothing to hit %s with.", srcName, targName)
		return
	}

	bPart := targBo.chooseRandomPart(w.rng, func(ent ecs.Entity) int {
		if targBo.hp[ent.ID()] <= 0 {
			return 0
		}
		switch ent.Type() & bcPartMask {
		case bcTorso:
			return 100 * (1 + targBo.maxHP[ent.ID()] - targBo.hp[ent.ID()])
		case bcHead:
			return 10 * (1 + targBo.maxHP[ent.ID()] - targBo.hp[ent.ID()])
		}
		return 0
	})
	if bPart == ecs.NilEntity {
		w.log("%s can find nothing worth hitting on %s.", srcName, targName)
		return
	}

	aEff, bEff := 1.0, 1.0

	for gt := srcBo.rel.Traverse(ecs.AllRel(brControl), ecs.TraverseCoDFS, aPart.ID()); gt.Traverse(); {
		id := gt.Node().ID()
		aEff *= float64(srcBo.hp[id]) / float64(srcBo.maxHP[id])
	}

	for gt := targBo.rel.Traverse(ecs.AllRel(brControl), ecs.TraverseCoDFS, bPart.ID()); gt.Traverse(); {
		id := gt.Node().ID()
		bEff *= float64(targBo.hp[id]) / float64(targBo.maxHP[id])
	}

	aDesc := srcBo.DescribePart(aPart)
	bDesc := targBo.DescribePart(bPart)

	if 1.0-bEff*w.rng.Float64()/aEff*w.rng.Float64() < -1.0 {
		w.log("%s's %s misses %s's %s", srcName, aDesc, targName, bDesc)
		return
	}

	dmg := srcBo.dmg[aPart.ID()]
	if dmg > 1 {
		dmg = dmg/2 + rand.Intn(dmg/2)
	} else if dmg == 1 {
		dmg = rand.Intn(1)
	}
	dmg -= targBo.armor[bPart.ID()]

	if dmg == 0 {
		w.log("%s's %s bounces off %s's %s", srcName, aDesc, targName, bDesc)
		return
	}

	if dmg < 0 {
		src, targ = targ, src
		srcBo, targBo = targBo, srcBo
		srcName, targName = targName, srcName
		aPart, bPart = bPart, aPart
		aEff, bEff = bEff, aEff
		aDesc, bDesc = bDesc, aDesc
		dmg = -dmg
	}

	targID := targ.ID()

	// dmg > 0
	part, destroyed := targBo.damagePart(bPart, dmg)
	w.log("%s's %s dealt %v damage to %s's %s", srcName, aDesc, dmg, targName, bDesc)
	if !destroyed {
		return
	}

	if severed := targBo.sever(bPart.ID()); severed != nil {
		w.newItem(w.Positions[targID], fmt.Sprintf("remains of %s", targName), '%', severed)
		if roots := severed.rel.Roots(ecs.AllRel(brControl), nil); len(roots) > 0 {
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
		targ.Destroy()
		w.log("%s destroyed by %s (headshot)", targName, srcName)
	} else {
		targ.Delete(wcBody)
		w.Glyphs[targID] = '⟡'
		w.setTimer(targ, numSpiritTurns, timerDestroy)
		w.log("%s was disembodied by %s (moving as spirit for %v turns)", targName, srcName, numSpiritTurns)
	}
}

func (w *world) setTimer(ent ecs.Entity, n int, a timerAction) *timer {
	ent.Add(wcTimer)
	w.timers[ent.ID()] = timer{n: n, a: a}
	return &w.timers[ent.ID()]
}

func (w *world) addBox(box point.Box, glyph rune) {
	// TODO: the box should be an entity, rather than each cell
	last, sz, pos := wallTable.Ref(1), box.Size(), box.TopLeft
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
			wall := w.AddEntity(wcPosition | wcCollide | wcGlyph | wcBG | wcFG)
			w.Glyphs[wall.ID()] = glyph
			w.Positions[wall.ID()] = pos
			c, _ := wallTable.toColor(last)
			w.BG[wall.ID()] = c
			w.FG[wall.ID()] = c + 1
			pos = pos.Add(r.d)
			last = wallTable.ChooseNext(w.rng, last)
		}
	}

	floorTable.genTile(w.rng, box, func(pos point.Point, bg termbox.Attribute) {
		floor := w.AddEntity(wcPosition | wcBG)
		w.Positions[floor.ID()] = pos
		w.BG[floor.ID()] = bg
	})
}

func (w *world) newItem(pos point.Point, name string, glyph rune, val interface{}) ecs.Entity {
	ent := w.AddEntity(wcPosition | wcName | wcGlyph | wcItem)
	w.Positions[ent.ID()] = pos
	w.Glyphs[ent.ID()] = glyph
	w.Names[ent.ID()] = name
	w.items[ent.ID()] = val
	return ent
}

func (w *world) newChar(name string, glyph rune) ecs.Entity {
	ent := w.AddEntity(charMask)
	w.Glyphs[ent.ID()] = glyph
	w.Positions[ent.ID()] = point.Zero
	w.Names[ent.ID()] = name
	w.bodies[ent.ID()].build(w.rng)
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

func (w *world) getName(ent ecs.Entity, deflt string) string {
	if !ent.Type().All(wcName) {
		return deflt
	}
	if w.Names[ent.ID()] == "" {
		return deflt
	}
	return w.Names[ent.ID()]
}

func (w *world) prepareCollidables() {
	// TODO: maintain a cleverer structure, like a quad-tree, instead
	it := w.Iter(ecs.All(collMask))
	if n := it.Count(); cap(w.coll) < n {
		w.coll = make([]ecs.EntityID, 0, n)
	} else {
		w.coll = w.coll[:0]
	}
	for it.Next() {
		w.coll = append(w.coll, it.ID())
	}
}

func (w *world) collides(ent ecs.Entity, pos point.Point) ecs.Entity {
	var id ecs.EntityID
	if ent != ecs.NilEntity {
		id = w.Deref(ent)
	}
	for _, hitID := range w.coll {
		if hitID != id {
			if hitPos := w.Positions[hitID]; hitPos.Equal(pos) {
				return w.Ref(hitID)
			}
		}
	}
	return ecs.NilEntity

	// TODO: binary search
	// i := sort.Search(len(w.coll), func(i int) bool {
	// 	return w.Positions[w.coll[i]].Less(pos)
	// })
	// if i < len(w.coll) {
	// 	return w.coll[i], w.coll[i] != id && w.Positions[w.coll[i]].Equal(pos)
	// }
	// return 0, false
}

func (w *world) move(ent ecs.Entity, move point.Point) point.Point {
	pos := w.Positions[ent.ID()]
	new := pos.Add(move)
	if hit := w.collides(ent, new); hit != ecs.NilEntity {
		w.moves.Insert(mrCollide, ent, hit)
	} else {
		pos = new
		w.Positions[ent.ID()] = pos
	}
	return pos
}

func main() {
	if err := view.JustKeepRunning(func(v *view.View) (view.Client, error) {
		w, err := newWorld(v)
		if err != nil {
			return nil, err
		}

		pt := point.Point{X: 12, Y: 8}

		w.addBox(point.Box{TopLeft: pt.Neg(), BottomRight: pt}, '#')
		player := w.newChar("you", 'X')
		player.Add(wcPosition | wcCollide | wcInput | wcSoul)
		w.log("%s enter the world @%v stats: %+v", w.Names[player.ID()], w.Positions[player.ID()], w.bodies[player.ID()].Stats())

		return w, nil
	}); err != nil {
		log.Fatal(err)
	}
}
