package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/jcorbin/execs/internal/ecs"
	"github.com/jcorbin/execs/internal/moremath"
	"github.com/jcorbin/execs/internal/point"
	"github.com/jcorbin/execs/internal/view"
	termbox "github.com/nsf/termbox-go"
)

// TODO: spirit possession
// TODO: movement based on body status
// TODO: factor out something like ecs.Relation

const (
	wcName ecs.ComponentType = 1 << iota
	wcTimer
	wcPosition
	wcCollide
	wcSolid
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
	charMask     = wcName | wcGlyph | wcBody | wcSolid
	aiMoveMask   = wcPosition | wcInput | wcAI
	collMask     = wcPosition | wcCollide
	combatMask   = wcCollide | wcBody
)

type worldItem interface {
	interact(pr prompt, w *world, item, ent ecs.Entity) (prompt, bool)
}

type destroyableItem interface {
	worldItem
	destroy(w *world)
}

type world struct {
	logFile *os.File
	logger  *log.Logger
	rng     *rand.Rand
	View    *view.View
	ecs.Core

	over         bool
	enemyCounter int

	prompt prompt

	Names     []string
	Positions []point.Point
	Glyphs    []rune
	BG        []termbox.Attribute
	FG        []termbox.Attribute
	timers    []timer
	bodies    []*body
	items     []worldItem

	coll  []ecs.EntityID // TODO: use an index structure for this
	moves moves
}

type moves struct {
	ecs.Relation
	n []int
	p []point.Point
}

const (
	movN ecs.ComponentType = 1 << iota
	movP

	mrCollide ecs.RelationType = 1 << iota
	mrHit
	mrItem
	mrGoal
	mrAgro
	mrPending

	movPending = movP | ecs.ComponentType(mrPending)
)

func (mov *moves) init(core *ecs.Core) {
	mov.Relation.Init(core, 0, core, 0)
	mov.n = []int{0}
	mov.p = []point.Point{point.Zero}
	mov.RegisterAllocator(movN|movP, mov.allocData)
	mov.RegisterDestroyer(movN, mov.deleteN)
	mov.RegisterDestroyer(movP, mov.deleteP)
}

func (mov *moves) allocData(id ecs.EntityID, t ecs.ComponentType) {
	mov.n = append(mov.n, 0)
	mov.p = append(mov.p, point.Zero)
}

func (mov *moves) deleteN(id ecs.EntityID, t ecs.ComponentType) { mov.n[id] = 0 }
func (mov *moves) deleteP(id ecs.EntityID, t ecs.ComponentType) { mov.p[id] = point.Zero }

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
	f, err := os.Create(fmt.Sprintf("%v.log", time.Now().Format(time.RFC3339)))
	if err != nil {
		return nil, err
	}
	w := &world{
		rng:    rand.New(rand.NewSource(rand.Int63())),
		View:   v,
		logger: log.New(f, "", 0),
	}
	w.init()
	w.log("logging to %q", f.Name())
	return w, nil
}

func (w *world) init() {
	// TODO: consider eliminating the padding for EntityID(0)
	w.Names = []string{""}
	w.Positions = []point.Point{point.Point{}}
	w.Glyphs = []rune{0}
	w.BG = []termbox.Attribute{0}
	w.FG = []termbox.Attribute{0}
	w.timers = []timer{timer{}}
	w.bodies = []*body{nil}
	w.items = []worldItem{nil}

	w.prompt.action = make([]promptAction, 0, 10)

	w.moves.init(&w.Core)

	w.RegisterAllocator(wcName|wcPosition|wcGlyph|wcBG|wcFG|wcBody|wcItem|wcTimer, w.allocWorld)
	w.RegisterCreator(wcBody, w.createBody)
	w.RegisterDestroyer(wcBody, w.destroyBody)
	w.RegisterDestroyer(wcItem, w.destroyItem)
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
	if bo := w.bodies[id]; bo != nil {
		bo.Clear()
	}
}

func (w *world) destroyItem(id ecs.EntityID, t ecs.ComponentType) {
	item := w.items[id]
	w.items[id] = nil
	if des, ok := item.(destroyableItem); ok {
		des.destroy(w)
	}
}

func (bo *body) destroy(w *world) {
	for i := range w.bodies {
		if i > 0 && w.bodies[i] == nil {
			bo.Clear()
			w.bodies[i] = bo
			break
		}
	}
}

func (w *world) Render(ctx *view.Context) error {
	ctx.SetHeader(
		fmt.Sprintf("%v souls v %v demons", w.Iter(ecs.All(wcSoul)).Count(), w.Iter(ecs.All(wcAI)).Count()),
	)

	it := w.Iter(ecs.All(wcSoul | wcBody))
	promptLines := w.prompt.render("")
	footParts := make([]string, 0, it.Count()*3+len(promptLines)+1)

	if len(promptLines) > 0 {
		footParts = append(footParts, promptLines...)
		footParts = append(footParts, "")
	}

	for it.Next() {
		bo := w.bodies[it.ID()]

		armor, damage := 0, 0
		hpParts := make([]string, 0, bo.Len())
		armorParts := make([]string, 0, bo.Len())
		damageParts := make([]string, 0, bo.Len())
		for gt := bo.rel.Traverse(ecs.AllRel(brControl), ecs.TraverseDFS); gt.Traverse(); {
			ent := gt.Node()
			id := ent.ID()
			// desc := bo.DescribePart(ent)
			hpParts = append(hpParts, fmt.Sprintf("%v", bo.hp[id]))
			if n := bo.armor[id]; n > 0 {
				armorParts = append(armorParts, fmt.Sprintf("%v", n))
				armor += n
			}
			if n := bo.dmg[id]; n > 0 {
				damageParts = append(damageParts, fmt.Sprintf("%v", n))
				damage += n
			}
		}

		// TODO: render a doll like
		//    _O_
		//   / | \
		//   = | =
		//    / \
		//  _/   \_

		hp, maxHP := bo.HPRange()
		footParts = append(footParts,
			fmt.Sprintf("HP(%v/%v): %s", hp, maxHP, strings.Join(hpParts, " ")),
			fmt.Sprintf("Armor(%v): %s", armor, strings.Join(armorParts, " ")),
			fmt.Sprintf("Damage(%v): %s", damage, strings.Join(damageParts, " ")),
		)
	}

	ctx.SetFooter(footParts...)

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
	if k.Key == termbox.KeyEsc {
		return view.ErrStop
	}

	// maybe run prompt
	if w.prompt.handle(k.Ch) {
		return nil
	}

	// parse player move
	var move point.Point
	switch k.Key {
	case termbox.KeyArrowDown:
		move = point.Point{X: 0, Y: 1}
	case termbox.KeyArrowUp:
		move = point.Point{X: 0, Y: -1}
	case termbox.KeyArrowLeft:
		move = point.Point{X: -1, Y: 0}
	case termbox.KeyArrowRight:
		move = point.Point{X: 1, Y: 0}
	default:
		move = point.Zero
	}

	for it := w.Iter(ecs.All(playMoveMask)); it.Next(); {
		w.addPendingMove(it.Entity(), move)
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
	w.generateAIMoves()   // give AI a chance!
	w.applyMoves()        // resolve moves
	w.buildItemMenu()     // what items are here
	w.processCollisions() // e.g. deal damage
	w.checkOver()         // no souls => done
	w.maybeSpawn()        // spawn more demons
}

func (w *world) reset() {
	w.prompt.reset()

	w.View.ClearLog()

	// reset collisions
	w.moves.Delete(ecs.AnyRel(mrCollide), nil)

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

func (w *world) addPendingMove(ent ecs.Entity, move point.Point) ecs.Entity {
	rel := w.moves.Insert(mrPending, ent, ent)
	rel.Add(movP)
	w.moves.p[rel.ID()] = move
	return rel
}

func (w *world) generateAIMoves() {
	for it := w.Iter(ecs.All(aiMoveMask)); it.Next(); {
		if target, found := w.aiTarget(it.Entity()); found {
			// Move towards the target!
			w.addPendingMove(it.Entity(), target.Sub(w.Positions[it.ID()]).Sign())
		} else {
			// No? give up and just randomly budge then!
			w.log("%s> Hmm...", w.getName(it.Entity(), "???"))
			w.addPendingMove(it.Entity(), point.Point{
				X: w.rng.Intn(3) - 1,
				Y: w.rng.Intn(3) - 1,
			})
		}
	}
}

func (w *world) applyMoves() {
	// TODO: better resolution strategy based on connected components
	w.moves.InsertMany(func(insert func(r ecs.RelationType, a ecs.Entity, b ecs.Entity) ecs.Entity) {
		w.moves.Update(ecs.All(movPending), nil, func(r ecs.RelationType, ent, a, b ecs.Entity) (ecs.RelationType, ecs.Entity, ecs.Entity) {
			new := w.Positions[a.ID()].Add(w.moves.p[ent.ID()])
			if hit := w.collides(a, new); len(hit) > 0 {
				for _, other := range hit {
					if other.Type().All(wcSolid) {
						return mrCollide | mrHit, a, other
					}
					if other.Type().All(wcItem) {
						insert(mrCollide|mrItem, a, other)
					}
				}
			}
			w.Positions[a.ID()] = new
			return ecs.NoRelType, a, b
		})
	})
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
		rel, goal := cur.Entity(), cur.B()
		if goal.Type().All(wcPosition) {
			myPos := w.Positions[ai.ID()]
			goalPos := w.Positions[goal.ID()]
			id := rel.ID()
			if !rel.Type().All(movN | movP) {
				rel.Add(movN | movP)
				w.moves.p[id] = myPos
				return goalPos, true
			}
			if lastPos := w.moves.p[id]; lastPos != myPos {
				w.moves.n[id] = 0
				w.moves.p[id] = myPos
				return goalPos, true
			}
			w.moves.n[id]++
			if w.moves.n[id] < 3 {
				return goalPos, true
			}
		}

		w.log("%s> giving up on %v @%v",
			w.getName(ai, "anon"),
			w.getName(goal, "???"),
			w.Positions[goal.ID()],
		)

		rel.Destroy() // bogus or stuck goal
	}

	// ... no goal, pick one!
	if goal := w.chooseAIGoal(ai); goal != ecs.NilEntity {
		w.moves.Insert(mrGoal, ai, goal)
		w.log("%s> going to boop %v @%v",
			w.getName(ai, "anon"),
			w.getName(goal, "???"),
			w.Positions[goal.ID()],
		)
		return w.Positions[goal.ID()], true
	}

	return point.Zero, false
}

func (w *world) chooseAIGoal(ai ecs.Entity) ecs.Entity {
	// TODO: doesn't always cause progress, get stuck on the edge sometimes
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
	return goal
}

func (w *world) processCollisions() {
	for cur := w.moves.Cursor(ecs.RelClause(mrCollide, mrHit), nil); cur.Scan(); {
		a, b := cur.A(), cur.B()

		if a.Type().All(combatMask) && b.Type().All(combatMask) {
			// combat collisions deal damage
			w.attack(a, b)
		}

		// am ai?
		if a.Type().All(aiMoveMask) {
			// have booped?
			for gc := w.moves.LookupA(ecs.AllRel(mrGoal), a.ID()); gc.Scan(); {
				if gc.B() == b {
					w.log("%s> booped %v @%v",
						w.getName(a, "anon"),
						w.getName(b, "???"),
						w.Positions[b.ID()],
					)
					gc.Entity().Destroy()
					break
				}
			}
		}
	}
}

func (w *world) buildItemMenu() {
	if pr, ok := w.itemPrompt(w.prompt); ok {
		w.prompt = pr
	} else if w.prompt.mess != "" {
		w.prompt.reset()
	}
}

func (w *world) itemPrompt(pr prompt) (prompt, bool) {
	// TODO: once we have a proper spatial index, stop relying on
	// collision relations for this
	prompting := false
	for cur := w.moves.Cursor(ecs.RelClause(mrCollide, mrItem), nil); cur.Scan(); {
		if !prompting {
			pr = pr.makeSub("Items Here")
			prompting = true
		}
		ent, item := cur.A(), cur.B()
		if !pr.addAction(
			func(pr prompt) (prompt, bool) { return w.interactWith(pr, ent, item) },
			w.getName(item, "unknown item"),
		) {
			break
		}
	}
	return pr, prompting
}

func (w *world) interactWith(pr prompt, ent, item ecs.Entity) (prompt, bool) {
	if it := w.items[item.ID()]; it != nil {
		return w.items[item.ID()].interact(pr, w, item, ent)
	}
	return pr.unwind(), false
}

type bodyRemains struct {
	w    *world     // the world it's in
	bo   *body      // the body it's in
	part ecs.Entity // the part
	item ecs.Entity // its container
	ent  ecs.Entity // what's interacting with it
}

func (bo *body) interact(pr prompt, w *world, item, ent ecs.Entity) (prompt, bool) {
	if !ent.Type().All(wcBody) {
		w.log("you have no body!")
		return pr, false
	}

	pr = pr.makeSub(w.getName(item, "unknown item"))

	for it := bo.Iter(ecs.All(bcPart)); len(pr.action) < cap(pr.action) && it.Next(); {
		part := it.Entity()
		rem := bodyRemains{w, bo, part, item, ent}
		// TODO: inspect menu when more than just scavengable

		// any part can be scavenged
		if !pr.addAction(
			rem.scavenge,
			rem.describeScavenge(),
		) {
			break
		}

	}

	return pr, true
}

func (rem bodyRemains) describeScavenge() string {
	return fmt.Sprintf("scavenge %s (armor:%+d damage:%+d)",
		rem.bo.DescribePart(rem.part),
		rem.bo.armor[rem.part.ID()],
		rem.bo.dmg[rem.part.ID()])
}

func (rem bodyRemains) scavenge(pr prompt) (prompt, bool) {
	entBo := rem.w.bodies[rem.ent.ID()]

	imp := make([]string, 0, 2)
	if armor := entBo.armor[rem.part.ID()]; armor > 0 {
		recv := rem.w.chooseAttackedPart(rem.ent)
		entBo.armor[recv.ID()] += armor
		imp = append(imp, fmt.Sprintf("%s armor +%v", entBo.DescribePart(recv), armor))
	}
	if damage := entBo.dmg[rem.part.ID()]; damage > 0 {
		recv := rem.w.chooseAttackerPart(rem.ent)
		entBo.dmg[recv.ID()] += damage
		imp = append(imp, fmt.Sprintf("%s damage +%v", entBo.DescribePart(recv), damage))
	}
	rem.part.Destroy()

	rem.w.log("%s gained %v from %s",
		rem.w.getName(rem.ent, "unknown"),
		strings.Join(imp, " and "),
		rem.w.getName(rem.item, "unknown"),
	)

	pr, ok := rem.w.itemPrompt(pr.unwind())
	if !ok {
		return pr, ok
	}

	if rem.bo.Len() == 0 {
		rem.item.Destroy()
		return pr, ok
	}

	return rem.bo.interact(pr, rem.w, rem.item, rem.ent)
}

// func (w *world) graftBodyPart(soul ecs.Entity, rem *body, part, item ecs.Entity) {
// 	defer func() { w.prompt = *w.prompt.prior }()

// 		bo := w.bodies[soul.ID()]

// 		attach, limit := ecs.NoType, 0
// 		switch part.Type() & bcPartMask {
// 		case bcRight | bcUpperArm, bcLeft | bcUpperArm:
// 			attach, limit = bcTorso, 2
// 		case bcRight | bcForeArm:
// 			attach, limit = bcRight|bcUpperArm, 1
// 		case bcLeft | bcForeArm:
// 			attach, limit = bcLeft|bcUpperArm, 1
// 		case bcRight | bcHand:
// 			attach, limit = bcRight|bcForeArm, 1
// 		case bcLeft | bcHand:
// 			attach, limit = bcLeft|bcForeArm, 1
// 		case bcRight | bcThigh, bcLeft | bcThigh:
// 			attach, limit = bcTorso, 2
// 		case bcRight | bcCalf:
// 			attach, limit = bcRight|bcThigh, 1
// 		case bcLeft | bcCalf:
// 			attach, limit = bcLeft|bcThigh, 1
// 		case bcRight | bcFoot:
// 			attach, limit = bcRight|bcCalf, 1
// 		case bcLeft | bcFoot:
// 			attach, limit = bcLeft|bcCalf, 1
// 		case bcTail:
// 			attach, limit = bcTorso, 3
// 		default:
// 			w.log("don't know how to graft a %s", body.DescribePart(part))
// 		}

// 		for it := bo.Iter(ecs.All(attach)); it.Next(); {
// 			for cur := bo.rel.LookupA(ecs.AllRel(brControl), it.ID()); cur.Scan(); {
// 				// TODO: should be a cursor w/ where .Count()
// 				n := 0
// 				if sub := cur.B(); sub.Type() == part.Type() {
// 					n++
// 				}
// 				if n < limit {
// 					par := it.Entity()
// 					// TODO: import the sub graph, insert relation from par to it
// 				}

// 			}
// 		}
// }

func (w *world) firstSoulBody() ecs.Entity {
	if it := w.Iter(ecs.All(wcBody | wcSoul)); it.Next() {
		return it.Entity()
	}
	return ecs.NilEntity
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
	if len(w.collides(w.Ref(0), point.Zero)) > 0 {
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
			bo := w.bodies[it.ID()]
			if it.Type().All(wcSoul) {
				hp, maxHP := bo.HPRange()
				dmg := maxHP - hp
				sum += 10*maxHP + 100*dmg
			} else {
				sum += bo.HP()
			}
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
	aPart, bPart := w.checkAttackHit(src, targ)
	if aPart == ecs.NilEntity || bPart == ecs.NilEntity {
		return
	}

	srcBo, targBo := w.bodies[src.ID()], w.bodies[targ.ID()]
	rating := srcBo.partHPRating(aPart) / targBo.partHPRating(bPart)
	rand := (1 + w.rng.Float64()) / 2 // like an x/2 + 1D(x/2) XXX reconsider
	dmg := int(moremath.Round(float64(srcBo.dmg[aPart.ID()]) * rating * rand))
	dmg -= targBo.armor[bPart.ID()]
	if dmg == 0 {
		w.log("%s's %s bounces off %s's %s",
			w.getName(src, "!?!"), srcBo.DescribePart(aPart),
			w.getName(targ, "?!?"), targBo.DescribePart(bPart))
		return
	}

	if dmg < 0 {
		w.dealAttackDamage(targ, bPart, src, aPart, -dmg)
	} else {
		w.dealAttackDamage(src, aPart, targ, bPart, dmg)
	}
}

func (w *world) dealAttackDamage(src, aPart, targ, bPart ecs.Entity, dmg int) {
	// TODO: store damage and kill relations

	srcBo, targBo := w.bodies[src.ID()], w.bodies[targ.ID()]
	dealt, part, destroyed := targBo.damagePart(bPart, dmg)
	if !destroyed {
		w.log("%s's %s dealt %v damage to %s's %s",
			w.getName(src, "!?!"), srcBo.DescribePart(aPart),
			dealt,
			w.getName(targ, "?!?"), targBo.DescribePart(bPart),
		)
		// TODO: Relation.Upsert would be nice
		if w.moves.Update(
			ecs.AllRel(mrAgro),
			func(r ecs.RelationType, ent, a, b ecs.Entity) bool {
				return a == targ && b == src
			},
			func(r ecs.RelationType, ent, a, b ecs.Entity) (ecs.RelationType, ecs.Entity, ecs.Entity) {
				w.moves.n[ent.ID()] += dmg
				return r, a, b
			},
		) == 0 {
			ent := w.moves.Insert(mrAgro, targ, src)
			ent.Add(movN)
			w.moves.n[ent.ID()] = dmg
		}
		return
	}

	// TODO: drop armor scraps on the floor instead
	if imp := w.reclaimDestroyedPart(src, part); len(imp) > 0 {
		w.log("%s gained %v from %s's %s",
			w.getName(src, "!?!"),
			strings.Join(imp, " and "),
			w.getName(targ, "?!?"), part.Desc,
		)
	}

	targID := targ.ID()

	if severed := targBo.sever(bPart.ID()); severed != nil {
		targName := w.getName(targ, "nameless")
		w.newItem(w.Positions[targID], fmt.Sprintf("remains of %s", targName), '%', severed)
		if severed.Len() > 0 {
			w.log("%s's remains have dropped on the floor", targName)
		}
	}

	w.log("%s's %s destroyed by %s's %s",
		w.getName(targ, "?!?"), targBo.DescribePart(bPart),
		w.getName(src, "!?!"), srcBo.DescribePart(aPart),
	)

	if bo := w.bodies[targID]; bo.Iter(ecs.All(bcPart)).Count() > 0 {
		return
	}

	// maybe become spirit
	if spi := targBo.spiritScore(); spi == 0 {
		targ.Destroy()
		w.log("%s destroyed by %s", w.getName(targ, "?!?"), w.getName(src, "!?!"))
	} else {
		targ.Delete(wcBody)
		w.Glyphs[targID] = '⟡'
		numSpiritTurns := 5 * spi
		w.setTimer(targ, numSpiritTurns, timerDestroy)
		w.log("%s was disembodied by %s (moving as spirit for %v turns)", w.getName(targ, "?!?"), w.getName(src, "!?!"), numSpiritTurns)
	}
}

func (w *world) reclaimDestroyedPart(ent ecs.Entity, part bodyPart) []string {
	bo := w.bodies[ent.ID()]
	imp := make([]string, 0, 2)
	if part.Armor > 0 {
		recv := w.chooseAttackedPart(ent)
		bo.armor[recv.ID()] += part.Armor
		imp = append(imp, fmt.Sprintf("%s armor +%v", bo.DescribePart(recv), part.Armor))
	}
	if part.Damage > 0 {
		recv := w.chooseAttackerPart(ent)
		bo.dmg[recv.ID()] += part.Damage
		imp = append(imp, fmt.Sprintf("%s damage +%v", bo.DescribePart(recv), part.Damage))
	}
	return imp
}

func (w *world) checkAttackHit(src, targ ecs.Entity) (ecs.Entity, ecs.Entity) {
	aPart := w.chooseAttackerPart(src)
	if aPart == ecs.NilEntity {
		w.log("%s has nothing to hit %s with.", w.getName(src, "!?!"), w.getName(targ, "?!?"))
		return ecs.NilEntity, ecs.NilEntity
	}
	bPart := w.chooseAttackedPart(targ)
	if bPart == ecs.NilEntity {
		w.log("%s can find nothing worth hitting on %s.", w.getName(src, "!?!"), w.getName(targ, "?!?"))
		return ecs.NilEntity, ecs.NilEntity
	}
	return aPart, bPart
}

func (w *world) chooseAttackerPart(ent ecs.Entity) ecs.Entity {
	bo := w.bodies[ent.ID()]
	return bo.chooseRandomPart(w.rng, func(part ecs.Entity) int {
		if bo.dmg[part.ID()] <= 0 {
			return 0
		}
		return 4*bo.dmg[part.ID()] + 2*bo.armor[part.ID()] + bo.hp[part.ID()]
	})
}

func (w *world) chooseAttackedPart(ent ecs.Entity) ecs.Entity {
	bo := w.bodies[ent.ID()]
	return bo.chooseRandomPart(w.rng, func(part ecs.Entity) int {
		id := part.ID()
		hp := bo.hp[id]
		if hp <= 0 {
			return 0
		}
		maxHP := bo.maxHP[id]
		armor := bo.armor[id]
		stick := 1 + maxHP - hp
		switch part.Type() & bcPartMask {
		case bcTorso:
			stick *= 100
		case bcHead:
			stick *= 10
		}
		return stick - armor
	})
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
			wall := w.AddEntity(wcPosition | wcCollide | wcSolid | wcGlyph | wcBG | wcFG)
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

func (w *world) newItem(pos point.Point, name string, glyph rune, val worldItem) ecs.Entity {
	ent := w.AddEntity(wcPosition | wcCollide | wcName | wcGlyph | wcItem)
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

func (w *world) collides(ent ecs.Entity, pos point.Point) []ecs.Entity {
	var id ecs.EntityID
	if ent != ecs.NilEntity {
		id = w.Deref(ent)
	}
	var r []ecs.Entity
	for _, hitID := range w.coll {
		if hitID != id {
			if hitPos := w.Positions[hitID]; hitPos.Equal(pos) {
				r = append(r, w.Ref(hitID))
			}
		}
	}
	return r

	// TODO: binary search
	// i := sort.Search(len(w.coll), func(i int) bool {
	// 	return w.Positions[w.coll[i]].Less(pos)
	// })
	// if i < len(w.coll) {
	// 	return w.coll[i], w.coll[i] != id && w.Positions[w.coll[i]].Equal(pos)
	// }
	// return 0, false
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
