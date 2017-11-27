package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"

	termbox "github.com/nsf/termbox-go"

	"github.com/jcorbin/execs/internal/ecs"
	"github.com/jcorbin/execs/internal/moremath"
	"github.com/jcorbin/execs/internal/point"
	"github.com/jcorbin/execs/internal/view"
	"github.com/jcorbin/execs/internal/view/hud/prompt"
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
	wcFloor
	wcWall
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
	interact(pr prompt.Prompt, w *world, item, ent ecs.Entity) (prompt.Prompt, bool)
}

type durableItem interface {
	worldItem
	HPRange() (int, int)
}

type destroyableItem interface {
	worldItem
	destroy(w *world)
}

type world struct {
	ui

	logFile *os.File
	logger  *log.Logger
	rng     *rand.Rand

	over         bool
	enemyCounter int

	ecs.Core

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
	mrMoveRange

	movCharge  = ecs.ComponentType(mrPending) | movN
	movPending = ecs.ComponentType(mrPending) | movN | movP
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
	n, m int
	f    func(ecs.Entity)
}

func newWorld(v *view.View) (*world, error) {
	// f, err := os.Create(fmt.Sprintf("%v.log", time.Now().Format(time.RFC3339)))
	// if err != nil {
	// 	return nil, err
	// }
	w := &world{
		rng: rand.New(rand.NewSource(rand.Int63())),
		// logger: log.New(f, "", 0),
	}
	w.init(v)
	// w.log("logging to %q", f.Name())
	return w, nil
}

func (w *world) init(v *view.View) {
	w.ui.init(v)

	// TODO: consider eliminating the padding for EntityID(0)
	w.Names = []string{""}
	w.Positions = []point.Point{point.Point{}}
	w.Glyphs = []rune{0}
	w.BG = []termbox.Attribute{0}
	w.FG = []termbox.Attribute{0}
	w.timers = []timer{timer{}}
	w.bodies = []*body{nil}
	w.items = []worldItem{nil}

	w.RegisterAllocator(wcName|wcPosition|wcGlyph|wcBG|wcFG|wcBody|wcItem|wcTimer, w.allocWorld)
	w.RegisterCreator(wcInput, w.createInput)
	w.RegisterCreator(wcBody, w.createBody)
	w.RegisterDestroyer(wcTimer, w.destroyTimer)
	w.RegisterDestroyer(wcBody, w.destroyBody)
	w.RegisterDestroyer(wcItem, w.destroyItem)
	w.RegisterDestroyer(wcInput, w.destroyInput)

	w.moves.init(&w.Core)
}

var movementRangeLabels = []string{"Walk", "Lunge"}

func newRangeChooser(w *world, ent ecs.Entity) *rangeChooser {
	return &rangeChooser{
		w:   w,
		ent: ent,
	}
}

type rangeChooser struct {
	w   *world
	ent ecs.Entity
}

func (rc *rangeChooser) label() string {
	n := rc.w.getMovementRange(rc.ent)
	return movementRangeLabels[n-1]
}

func (rc *rangeChooser) RunPrompt(prior prompt.Prompt) (next prompt.Prompt, required bool) {
	next = prior.Sub("Set Movement Range")
	for i, label := range movementRangeLabels {
		n := i + 1
		r := '0' + rune(n)
		run := prompt.Func(func(prior prompt.Prompt) (next prompt.Prompt, required bool) {
			return rc.chosen(prior, n)
		})
		if n == 1 {
			next.AddAction(r, run, "%s (%d cell, consumes %d charge)", label, n, n)
		} else {
			next.AddAction(r, run, "%s (%d cells, consumes %d charges)", label, n, n)
		}
	}
	return next, true
}

func (rc *rangeChooser) chosen(prior prompt.Prompt, n int) (next prompt.Prompt, required bool) {
	rc.w.setMovementRange(rc.ent, n)
	next = prior.Unwind()
	next.SetActionMess(0, rc, movementRangeLabels[n-1])
	return next, false
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

func (w *world) createInput(id ecs.EntityID, t ecs.ComponentType) {
	w.setMovementRange(w.Ref(id), 1)
}

func (w *world) createBody(id ecs.EntityID, t ecs.ComponentType) {
	w.bodies[id] = newBody()
}

func (w *world) destroyTimer(id ecs.EntityID, t ecs.ComponentType) {
	w.timers[id] = timer{}
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

func (w *world) destroyInput(id ecs.EntityID, t ecs.ComponentType) {
	if name := w.Names[id]; name != "" {
		// TODO: restore attribution
		// w.log("%s destroyed by %s", w.getName(targ, "?!?"), w.getName(src, "!?!"))
		w.log("%s has been destroyed", name)
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

func (w *world) extent() point.Box {
	var bbox point.Box
	for it := w.Iter(ecs.All(renderMask)); it.Next(); {
		bbox = bbox.ExpandTo(w.Positions[it.ID()])
	}
	return bbox
}

func (w *world) Close() error { return nil }

func (w *world) Process() {
	w.moves.Delete(ecs.AnyRel(mrCollide), nil)
	w.tick()               // run timers
	w.prepareCollidables() // collect collidables
	w.generateAIMoves()    // give AI a chance!
	w.applyMoves()         // resolve moves
	w.processAIItems()     // nom nom
	w.processCombat()      // e.g. deal damage
	w.checkOver()          // no souls => done
	w.maybeSpawn()         // spawn more demons
}

func (w *world) tick() {
	// run timers
	for it := w.Iter(ecs.All(wcTimer)); it.Next(); {
		timer := &w.timers[it.ID()]

		if timer.n <= 0 {
			continue
		}
		timer.n--
		if timer.n > 0 {
			continue
		}

		ent := it.Entity()
		timer.f(ent)
		if timer.m != 0 {
			// refresh interval timer
			timer.n = timer.m
		} else {
			// one shot timer
			ent.Delete(wcTimer)
		}
	}
}

const maxChargeFromResting = 4

func (w *world) addCharge(ent ecs.Entity) {
	if !ent.Type().All(wcInput) {
		return // who asked you
	}
	w.moves.UpsertOne(mrPending, ent, ent, func(rel ecs.Entity) {
		rel.Add(movN)
		rel.Delete(movP)
		id := rel.ID()
		if n := w.moves.n[id]; n < maxChargeFromResting {
			w.moves.n[id] = n + 1
		}
	}, nil)
}

func (w *world) addPendingMove(ent ecs.Entity, move point.Point) {
	if !ent.Type().All(wcInput) {
		return // who asked you
	}
	w.moves.UpsertOne(mrPending, ent, ent,
		func(rel ecs.Entity) {
			rel.Add(movP | movN)
			id := rel.ID()
			w.moves.p[id] = w.moves.p[id].Add(move)
			if n := w.moves.n[id]; n < maxChargeFromResting {
				w.moves.n[id] = n + 1
			}
		},
		func(accum, next ecs.Entity) {
			if next.Type().All(movN) {
				w.moves.n[accum.ID()] += w.moves.n[next.ID()]
			}
		})
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

const maxMovementRange = 2

func (w *world) setMovementRange(a ecs.Entity, n int) {
	_ = w.Deref(a)
	if n > maxMovementRange {
		n = maxMovementRange
	}
	w.moves.UpsertOne(mrMoveRange, a, a, func(ent ecs.Entity) {
		ent.Add(movN)
		w.moves.n[ent.ID()] = n
	}, nil)
}

func (w *world) getMovementRange(a ecs.Entity) int {
	id := w.Deref(a)
	for cur := w.moves.LookupA(ecs.AllRel(mrMoveRange), id); cur.Scan(); {
		if ent := cur.Entity(); ent.Type().All(movN) {
			if n := w.moves.n[ent.ID()]; n <= maxMovementRange {
				return n
			}
		}
	}
	return maxMovementRange
}

func (w *world) getCharge(ent ecs.Entity) (charge int) {
	for cur := w.moves.LookupA(ecs.All(movCharge), w.Deref(ent)); cur.Scan(); {
		charge += w.moves.n[cur.Entity().ID()]
	}
	return charge
}

func (w *world) applyMoves() {
	// TODO: better resolution strategy based on connected components
	w.moves.UpsertMany(ecs.All(movPending), nil, func(
		r ecs.RelationType, ent, a, b ecs.Entity,
		emit func(r ecs.RelationType, a, b ecs.Entity) ecs.Entity,
	) {
		if ent == ecs.NilEntity {
			return
		}

		// TODO: replace these with a "what's here" query
		defer func() {
			pos := w.Positions[a.ID()]
			for it := w.Iter(ecs.All(wcItem | wcPosition)); it.Next(); {
				if w.Positions[it.ID()] == pos {
					emit(mrCollide|mrItem, a, it.Entity())
				}
			}
		}()

		// can we actually affect a move?
		pend, n := w.moves.p[ent.ID()], w.moves.n[ent.ID()]
		if a.Type().All(wcBody) {
			rating := w.bodies[a.ID()].movementRating()
			pend = pend.Mul(int(moremath.Round(rating * float64(n))))
			if pend.SumSQ() == 0 {
				emit(r, a, b)
				return
			}
		}

		// move until we collide or exceeding lunging distance
		limit := w.getMovementRange(a)
		pos := w.Positions[a.ID()]
		i := 0
		for ; i < n && i < limit; i++ {
			new := pos.Add(pend.Sign())
			if hit := w.collides(a, new); len(hit) > 0 {
				for _, b := range hit {
					if b.Type().All(wcSolid) {
						hitRel := emit(mrCollide|mrHit, a, b)
						if m := n - i; m > 1 {
							hitRel.Add(movN)
							w.moves.n[hitRel.ID()] = m
						}
						w.Positions[a.ID()] = pos
						return
					}
				}
			}
			pos = new
		}
		n -= i
		w.Positions[a.ID()] = pos

		// moved without hitting anything
		ent = emit(r, a, b)
		w.moves.p[ent.ID()], w.moves.n[ent.ID()] = point.Zero, n
	})
}

func (bo *body) movementRating() float64 {
	ratings := make(map[ecs.EntityID]float64, 6)
	for it := bo.Iter(ecs.Any(bcFoot | bcCalf | bcThigh)); it.Next(); {
		rating := 1.0
		for gt := bo.rel.Traverse(ecs.AllRel(brControl), ecs.TraverseCoDFS, it.ID()); gt.Traverse(); {
			id := gt.Node().ID()
			delete(ratings, id)
			rating *= float64(bo.hp[id]) / float64(bo.maxHP[id])
		}
		if it.Type().All(bcCalf) {
			rating *= 2 / 3
		} else if it.Type().All(bcThigh) {
			rating *= 1 / 3
		}
		ratings[it.ID()] = rating
	}
	rating := 0.0
	for _, r := range ratings {
		rating += r
	}
	return rating / 2.0
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
		return w.Positions[opp.ID()], true
	}

	// revert to our goal...
	goalPos, found := point.Zero, false
	w.moves.UpsertMany(
		ecs.AllRel(mrGoal),
		func(r ecs.RelationType, ent, a, b ecs.Entity) bool { return a == ai },
		func(
			r ecs.RelationType, rel, _, goal ecs.Entity,
			emit func(r ecs.RelationType, a, b ecs.Entity,
			) ecs.Entity) {
			if goal == ecs.NilEntity {
				// no goal, pick one!
				if goal := w.chooseAIGoal(ai); goal != ecs.NilEntity {
					emit(mrGoal, ai, goal)
					goalPos, found = w.Positions[goal.ID()], true
				}
				return
			}

			if !goal.Type().All(wcPosition) {
				// no position, drop it
				return
			}

			myPos := w.Positions[ai.ID()]
			goalPos = w.Positions[goal.ID()]
			if id := rel.ID(); !rel.Type().All(movN | movP) {
				rel.Add(movN | movP)
				w.moves.p[id] = myPos
			} else if lastPos := w.moves.p[id]; lastPos != myPos {
				w.moves.n[id] = 0
				w.moves.p[id] = myPos
			} else {
				w.moves.n[id]++
				if w.moves.n[id] >= 3 {
					// stuck trying to get that one, give up
					return
				}
			}
			found = true

			// see if we can do better
			alt := w.chooseAIGoal(ai)
			score := w.scoreAIGoal(ai, goal)
			score *= 16 // inertia bonus
			altScore := w.scoreAIGoal(ai, alt)
			if w.rng.Intn(score+altScore) < altScore {
				goal, goalPos = alt, w.Positions[alt.ID()]
			}

			// keep or update
			emit(mrGoal, ai, goal)
		})

	return goalPos, found
}

func (w *world) scoreAIGoal(ai, goal ecs.Entity) int {
	const quadLimit = 144

	myPos := w.Positions[ai.ID()]
	goalPos := w.Positions[goal.ID()]
	score := goalPos.Sub(myPos).SumSQ()
	if score > quadLimit {
		return 0
	}
	if goal.Type().All(wcItem) {
		return (quadLimit - score) * quadLimit
	}
	return score
}

func (w *world) chooseAIGoal(ai ecs.Entity) ecs.Entity {
	// TODO: doesn't always cause progress, get stuck on the edge sometimes
	goal, sum := ecs.NilEntity, 0
	for it := w.Iter(ecs.All(collMask)); it.Next(); {
		if it.Type().All(combatMask) {
			continue
		}
		if score := w.scoreAIGoal(ai, it.Entity()); score > 0 {
			sum += score
			if sum <= 0 || w.rng.Intn(sum) < score {
				goal = it.Entity()
			}
		}
	}
	return goal
}

func (w *world) processAIItems() {
	type ab struct{ a, b ecs.EntityID }
	goals := make(map[ab]ecs.Entity)
	for cur := w.moves.Cursor(
		ecs.AllRel(mrGoal),
		func(r ecs.RelationType, ent, a, b ecs.Entity) bool {
			return a.Type().All(aiMoveMask)
		},
	); cur.Scan(); {
		goals[ab{cur.A().ID(), cur.B().ID()}] = cur.Entity()
	}

	for cur := w.moves.Cursor(
		ecs.RelClause(mrCollide, mrItem|mrHit),
		func(r ecs.RelationType, ent, a, b ecs.Entity) bool {
			_, isGoal := goals[ab{a.ID(), b.ID()}]
			return isGoal
		},
	); cur.Scan(); {
		ai, b := cur.A(), cur.B()

		if b.Type().All(wcItem) {
			// can haz?
			if pr, ok := w.itemPrompt(prompt.Prompt{}, ai); ok {
				w.runAIInteraction(pr, ai)
			}
			// can haz moar?
			if pr, ok := w.itemPrompt(prompt.Prompt{}, ai); !ok || pr.Len() == 0 {
				goals[ab{ai.ID(), b.ID()}].Destroy()
			}
		} else {
			// have booped?
			goals[ab{ai.ID(), b.ID()}].Destroy()
		}
	}
}

func (w *world) runAIInteraction(pr prompt.Prompt, ai ecs.Entity) {
	for n := pr.Len(); n > 0; n = pr.Len() {
		next, prompting, valid := pr.Run(w.rng.Intn(n))
		if !valid || !prompting {
			return
		}
		pr = next
	}
}

func (w *world) processCombat() {
	// TODO: make this an upsert that transmutes hits into damage/kill relations
	for cur := w.moves.Cursor(
		ecs.AllRel(mrCollide|mrHit),
		func(r ecs.RelationType, ent, a, b ecs.Entity) bool {
			return a.Type().All(combatMask) && b.Type().All(combatMask)
		},
	); cur.Scan(); {
		src, targ := cur.A(), cur.B()

		aPart, bPart := w.checkAttackHit(src, targ)
		if aPart == ecs.NilEntity || bPart == ecs.NilEntity {
			continue
		}

		srcBo, targBo := w.bodies[src.ID()], w.bodies[targ.ID()]
		rating := srcBo.partHPRating(aPart) / targBo.partHPRating(bPart)
		if cur.Entity().Type().All(movN) {
			mult := w.moves.n[cur.Entity().ID()]
			rating *= float64(mult)
		}
		rand := (1 + w.rng.Float64()) / 2 // like an x/2 + 1D(x/2) XXX reconsider
		dmg := int(moremath.Round(float64(srcBo.dmg[aPart.ID()]) * rating * rand))
		dmg -= targBo.armor[bPart.ID()]
		if dmg == 0 {
			if soulInvolved(src, targ) {
				w.log("%s's %s bounces off %s's %s",
					w.getName(src, "!?!"), srcBo.DescribePart(aPart),
					w.getName(targ, "?!?"), targBo.DescribePart(bPart))
			}
			continue
		}

		if dmg < 0 {
			w.dealAttackDamage(targ, bPart, src, aPart, -dmg)
		} else {
			w.dealAttackDamage(src, aPart, targ, bPart, dmg)
		}
	}
}

func (w *world) findPlayer() ecs.Entity {
	if it := w.Iter(ecs.All(playMoveMask)); it.Next() {
		return it.Entity()
	}
	return ecs.NilEntity
}

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
	pos := point.Zero // TODO: randomize?
	for it := w.Iter(ecs.All(collMask)); it.Next(); {
		if w.Positions[it.ID()].Equal(pos) {
			return
		}
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

	totalAgro := 0
	for cur := w.moves.Cursor(ecs.AllRel(mrAgro), nil); cur.Scan(); {
		totalAgro += w.moves.n[cur.Entity().ID()]
	}

	totalHP := 0
	totalDmg := 0
	combatCount := 0
	for it := w.Iter(ecs.All(combatMask | wcInput)); it.Next(); {
		if !it.Type().All(wcWaiting) {
			combatCount++
			bo := w.bodies[it.ID()]
			if it.Type().All(wcSoul) {
				hp, maxHP := bo.HPRange()
				dmg := maxHP - hp
				totalHP += maxHP
				totalDmg += dmg
			} else {
				totalHP += bo.HP()
			}
		}
	}

	agroDeficit := combatCount*30 - totalAgro
	sum := totalHP + totalDmg - agroDeficit
	if sum < 0 {
		sum = 0
	}
	if hp := bo.HP(); w.rng.Intn(sum+hp) < hp {
		enemy.Delete(wcWaiting)
		enemy.Add(wcPosition | wcCollide | wcInput | wcAI)
	}
}

func soulInvolved(a, b ecs.Entity) bool {
	// TODO: ability to say "those are soul remains"
	return a.Type().All(wcSoul) || b.Type().All(wcSoul)
}

func (w *world) dealAttackDamage(src, aPart, targ, bPart ecs.Entity, dmg int) {
	// TODO: store damage and kill relations

	srcBo, targBo := w.bodies[src.ID()], w.bodies[targ.ID()]
	dealt, _, destroyed := targBo.damagePart(bPart, dmg)
	if !destroyed {
		if soulInvolved(src, targ) {
			w.log("%s's %s dealt %v damage to %s's %s",
				w.getName(src, "!?!"), srcBo.DescribePart(aPart),
				dealt,
				w.getName(targ, "?!?"), targBo.DescribePart(bPart),
			)
		}

		// TODO: decouple damage -> agro into a separate upsert after the
		// damage proc; requires damage/kill relations
		w.moves.UpsertOne(
			mrAgro, targ, src,
			func(ent ecs.Entity) {
				ent.Add(movN)
				w.moves.n[ent.ID()] += dmg
			},
			func(accum, next ecs.Entity) {
				if next.Type().All(movN) {
					w.moves.n[accum.ID()] += w.moves.n[next.ID()]
				}
			},
		)
		return
	}

	if soulInvolved(src, targ) {
		w.log("%s's %s destroyed by %s's %s",
			w.getName(targ, "?!?"), targBo.DescribePart(bPart),
			w.getName(src, "!?!"), srcBo.DescribePart(aPart),
		)
	}

	targID := targ.ID()

	severed := targBo.sever(bPart.ID())
	if severed != nil {
		targName := w.getName(targ, "nameless")
		name := fmt.Sprintf("remains of %s", targName)
		item := w.newItem(w.Positions[targID], name, '%', severed)
		w.setInterval(item, 5, w.decayRemains)
		if severed.Len() > 0 {
			w.log("%s's remains have dropped on the floor", targName)
		}
	}

	if bo := w.bodies[targID]; bo.Iter(ecs.All(bcPart)).Count() > 0 {
		return
	}

	// may become spirit
	if severed != nil {
		heads, spi := severed.allHeads(), 0
		for _, head := range heads {
			spi += severed.hp[head.ID()]
		}
		if spi > 0 {
			targ.Delete(wcBody | wcCollide)
			w.Glyphs[targID] = '⟡'
			for _, head := range heads {
				head.Add(bcDerived)
				severed.derived[head.ID()] = targ
			}
			if soulInvolved(src, targ) {
				w.log("%s was disembodied by %s", w.getName(targ, "?!?"), w.getName(src, "!?!"))
			}
			return
		}
	}

	targ.Destroy()
}

func (w *world) decayRemains(item ecs.Entity) {
	rem := w.items[item.ID()].(*body)
	for it := rem.Iter(ecs.All(bcPart | bcHP)); it.Next(); {
		id := it.ID()
		rem.hp[id]--
		if rem.hp[id] <= 0 {
			part := it.Entity()
			w.log(
				"%s from %s has decayed away to nothing",
				rem.DescribePart(part),
				w.getName(item, "unknown item"),
			)
			part.Destroy()
		}
	}
	if rem.Len() == 0 {
		w.dirtyFloorTile(w.Positions[item.ID()])
		// TODO: do something neat after max dirty (spawn something
		// creepy)
		item.Destroy()
	}
}

func (w *world) dirtyFloorTile(pos point.Point) (ecs.Entity, bool) {
	for it := w.Iter(ecs.All(wcPosition | wcBG | wcFloor)); it.Next(); {
		if id := it.ID(); w.Positions[id] == pos {
			bg := w.BG[id]
			for i := range floorColors {
				if floorColors[i] == bg {
					j := i + 1
					canDirty := j < len(floorColors)
					if canDirty {
						w.BG[id] = floorColors[j]
					}
					return it.Entity(), canDirty
				}
			}
		}
	}
	return ecs.NilEntity, false
}

func (w *world) checkAttackHit(src, targ ecs.Entity) (ecs.Entity, ecs.Entity) {
	aPart := w.chooseAttackerPart(src)
	if aPart == ecs.NilEntity {
		if soulInvolved(src, targ) {
			w.log("%s has nothing to hit %s with.", w.getName(src, "!?!"), w.getName(targ, "?!?"))
		}
		return ecs.NilEntity, ecs.NilEntity
	}
	bPart := w.chooseAttackedPart(targ)
	if bPart == ecs.NilEntity {
		if soulInvolved(src, targ) {
			w.log("%s can find nothing worth hitting on %s.", w.getName(src, "!?!"), w.getName(targ, "?!?"))
		}
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

func (w *world) setTimer(ent ecs.Entity, n int, f func(ecs.Entity)) *timer {
	ent.Add(wcTimer)
	w.timers[ent.ID()] = timer{n: n, f: f}
	return &w.timers[ent.ID()]
}

func (w *world) setInterval(ent ecs.Entity, n int, f func(ecs.Entity)) *timer {
	ent.Add(wcTimer)
	w.timers[ent.ID()] = timer{n: n, m: n, f: f}
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
			wall := w.AddEntity(wcPosition | wcCollide | wcSolid | wcGlyph | wcBG | wcFG | wcWall)
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
		floor := w.AddEntity(wcPosition | wcBG | wcFloor)
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
	w.ui.Log(s)
	if w.logger != nil {
		w.logger.Printf(s)
	}
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
	if !ent.Type().All(wcCollide) {
		return nil
	}
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

		w.ui.bar.addAction(newRangeChooser(w, player))

		return w, nil
	}); err != nil {
		log.Fatal(err)
	}
}
