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
	wcWaiting
	wcBody
	wcSoul
	wcItem
	wcAI
)

const (
	renderMask   = wcPosition | wcGlyph
	playMoveMask = wcPosition | wcInput | wcSoul
	charMask     = wcPosition | wcCollide | wcName | wcGlyph | wcBody
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

	Names     []string
	Positions []point.Point
	Glyphs    []rune
	BG        []termbox.Attribute
	FG        []termbox.Attribute
	timers    []timer
	bodies    []*body
	items     []interface{}

	// TODO: consider making this an ecs.Relation
	coll  []ecs.EntityID
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
	// timerSetType FIXME
	timerCallback
)

type collision struct {
	sourceID, targetID ecs.EntityID
}

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

	f, err := os.Create(fmt.Sprintf("%v.log", time.Now().Format(time.RFC3339)))
	if err != nil {
		return nil, err
	}
	w.logger = log.New(f, "", 0)
	w.log("logging to %q", f.Name())

	w.RegisterAllocator(wcName|wcPosition|wcGlyph|wcBG|wcFG|wcBody|wcItem|wcTimer, w.allocWorld)
	w.RegisterCreator(wcBody, w.createBody, w.destroyBody)
	w.RegisterCreator(wcItem, nil, w.destroyItem)

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
			name := w.getName(id, "???")
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
			if w.bodies[i] == nil {
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
		parts := make([]string, 0, len(bo.Entities))
		for _, root := range bo.Roots() {
			bo.Descend(root, func(ent ecs.Entity, level int) bool {
				parts = append(parts, fmt.Sprintf("%s:%v", bo.DescribePart(ent), bo.hp[ent.ID()]))
				return true
			})
		}
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
	it = w.Iter(ecs.All(renderMask))
	for it.Next() {
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

	it = w.Iter(ecs.Clause(wcPosition, wcGlyph|wcBG))
	for it.Next() {
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

func (w *world) Close() error { return nil }

func (w *world) HandleKey(v *view.View, k view.KeyEvent) error {
	if k.Key == termbox.KeyEsc {
		return view.ErrStop
	}

	v.ClearLog()

	// run timers
	it := w.Iter(ecs.All(wcTimer))
	for it.Next() {
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
		// 	w.Entities[it.ID()] = w.timers[it.ID()].t
		case timerCallback:
			f := w.timers[it.ID()].f
			if f != nil {
				w.timers[it.ID()].f = nil
				f(ent)
			}
		}
	}

	// collect collidables
	w.prepareCollidables()

	// ai chase target; last one wins for now TODO
	var target point.Point

	// apply player move
	if move, ok := key2move(k); ok {
		it := w.Iter(ecs.All(playMoveMask))
		for it.Next() {
			target = w.move(it.ID(), move)
		}
	}

	// chase player
	it = w.Iter(ecs.All(aiMoveMask))
	for it.Next() {
		move := target.Sub(w.Positions[it.ID()]).Sign()
		w.move(it.ID(), move)
	}

	// collisions deal damage
	for _, coll := range w.colls {
		if w.Entities[coll.sourceID]&combatMask != combatMask {
			continue
		}
		if w.Entities[coll.targetID]&combatMask != combatMask {
			continue
		}
		w.attack(coll.sourceID, coll.targetID)
	}

	// count remaining souls
	if w.Iter(ecs.All(wcSoul)).Count() == 0 {
		w.log("game over")
		return view.ErrStop
	}

	// maybe spawn
	// TODO: randomize position?
	if _, occupied := w.collides(0, point.Zero); !occupied {
		sum := 0
		it := w.Iter(ecs.All(wcBody))
		for it.Next() {
			sum += w.bodies[it.ID()].HP()
		}

		var enemy ecs.Entity
		if it := w.Iter(ecs.All(charMask | wcWaiting)); it.Next() {
			enemy = it.Entity()
		} else {
			enemy = w.newChar("enemy", 'X')
			enemy.Add(wcWaiting)
		}
		bo := w.bodies[enemy.ID()]
		if hp := bo.HP(); w.rng.Intn(sum+hp) < hp {
			enemy.Delete(wcWaiting)
			enemy.Add(wcInput | wcAI)
			w.log("%s enters the world @%v stats: %+v",
				w.Names[enemy.ID()],
				w.Positions[enemy.ID()],
				bo.Stats())
		}
	}

	return nil
}

func (w *world) setTimer(id ecs.EntityID, n int, a timerAction) *timer {
	w.Ref(id).Add(wcTimer)
	w.timers[id] = timer{n: n, a: a}
	return &w.timers[id]
}

func (w *world) addBox(box point.Box, glyph rune) {
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

func (w *world) getName(id ecs.EntityID, deflt string) string {
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
	it := w.Iter(ecs.All(collMask))
	if n := it.Count(); cap(w.coll) < n {
		w.coll = make([]ecs.EntityID, 0, n)
	} else {
		w.coll = w.coll[:0]
	}
	for it.Next() {
		w.coll = append(w.coll, it.ID())
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

func (w *world) collides(id ecs.EntityID, pos point.Point) (ecs.EntityID, bool) {
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

func (w *world) move(id ecs.EntityID, move point.Point) point.Point {
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
		w, err := newWorld(v)
		if err != nil {
			return nil, err
		}

		pt := point.Point{X: 12, Y: 8}

		w.addBox(point.Box{TopLeft: pt.Neg(), BottomRight: pt}, '#')
		player := w.newChar("you", 'X')
		player.Add(wcInput | wcSoul)
		w.log("%s enter the world @%v stats: %+v", w.Names[player.ID()], w.Positions[player.ID()], w.bodies[player.ID()].Stats())

		return w, nil
	}); err != nil {
		log.Fatal(err)
	}
}
