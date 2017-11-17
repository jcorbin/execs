package main

import (
	"fmt"
	"log"
	"math/rand"
	"sort"

	"github.com/jcorbin/execs/internal/ecs"
	"github.com/jcorbin/execs/internal/point"
	"github.com/jcorbin/execs/internal/view"
	termbox "github.com/nsf/termbox-go"
)

const (
	componentName ecs.ComponentType = 1 << iota
	componentPosition
	componentCollide
	componentGlyph
	componentBG
	componentFG
	componentHP
	componentStats
	componentInput
	componentSoul
	componentAI
)

const (
	renderMask   = componentPosition | componentGlyph
	playMoveMask = componentPosition | componentInput | componentSoul
	aiMoveMask   = componentPosition | componentInput | componentAI
	collMask     = componentPosition | componentCollide
	combatMask   = componentCollide | componentStats
)

const maxHP = 20

var (
	aiColors    = []termbox.Attribute{124, 160, 196, 202, 208, 214}
	soulColors  = []termbox.Attribute{19, 20, 21, 27, 33, 39}
	wallColors  = []termbox.Attribute{233, 234, 235, 236, 237, 238, 239}
	floorColors = []termbox.Attribute{232, 233, 234}

	wallTable  colorTable
	floorTable colorTable
)

func init() {
	wallTable.addLevelTransitions(wallColors, 4, 1, 4, 3, 2)
	floorTable.addLevelTransitions(floorColors, 6, 1, 6, 2, 1)
}

type world struct {
	View *view.View
	ecs.Core

	Names     []string
	Positions []point.Point
	Glyphs    []rune
	BG        []termbox.Attribute
	FG        []termbox.Attribute
	HP        []int
	Stats     []stats

	coll  []int
	colls []collision

	damaged []damage
	killed  []int
	killer  []int
}

type stats struct {
	Str, Def, Dex, Luck int
}

type collision struct {
	sourceID int
	targetID int
}

type damage struct {
	collision
	n int
}

func (w *world) AddEntity() ecs.Entity {
	ent := w.Core.AddEntity()
	w.Names = append(w.Names, "")
	w.Positions = append(w.Positions, point.Point{})
	w.Glyphs = append(w.Glyphs, 0)
	w.BG = append(w.BG, 0)
	w.FG = append(w.FG, 0)
	w.HP = append(w.HP, 0)
	w.Stats = append(w.Stats, stats{})
	return ent
}

func (w *world) Render(ctx *view.Context) error {
	ctx.SetHeader(
		fmt.Sprintf("%v souls v %v demons", w.CountAll(playMoveMask), w.CountAll(aiMoveMask)),
	)

	it := w.IterAll(componentSoul | componentHP)
	hpParts := make([]string, 0, it.Count())
	for id, _, ok := it.Next(); ok; id, _, ok = it.Next() {
		hpParts = append(hpParts, fmt.Sprintf("HP:%v", w.HP[id]))
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
		if t&componentSoul != 0 {
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

	it = w.Iter(componentPosition, componentGlyph|componentBG)
	for id, t, ok := it.Next(); ok; id, t, ok = it.Next() {
		pos := w.Positions[id].Add(offset)
		if !pos.Less(point.Zero) && !ctx.Grid.Size.Less(pos) {
			var (
				ch     rune
				fg, bg termbox.Attribute
			)

			if t.All(componentGlyph) {
				ch = w.Glyphs[id]
			}

			// TODO: move to hp update
			switch t & (componentSoul | componentAI | componentHP) {
			case componentSoul | componentHP:
				fg = soulColors[1+(len(soulColors)-2)*w.HP[id]/maxHP]
			case componentAI | componentHP:
				fg = aiColors[1+(len(soulColors)-2)*w.HP[id]/maxHP]
			case componentSoul:
				fg = soulColors[0]
			case componentAI:
				fg = aiColors[0]
			default:
				if t.All(componentFG) {
					fg = w.FG[id]
				}
			}

			if t.All(componentBG) {
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

	return nil
}

func (w *world) Close() error { return nil }

func (w *world) HandleKey(v *view.View, k view.KeyEvent) error {
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
	it := w.IterAll(aiMoveMask)
	for id, _, ok := it.Next(); ok; id, _, ok = it.Next() {
		w.move(id, target.Sub(w.Positions[id]).Sign())
	}

	// reset damage system
	if len(w.damaged) > 0 {
		w.damaged = w.damaged[:0]
	}
	if len(w.killer) > 0 {
		w.killer = w.killer[:0]
	}
	if len(w.killed) > 0 {
		w.killed = w.killed[:0]
	}

	// collisions deal damage
	for _, coll := range w.colls {
		if w.Entities[coll.sourceID]&combatMask != combatMask {
			continue
		}
		if w.Entities[coll.targetID]&combatMask != combatMask {
			continue
		}
		src, targ := &w.Stats[coll.sourceID], &w.Stats[coll.targetID]
		if 1+rand.Intn(src.Luck) > 1+rand.Intn(targ.Dex) {
			dmg := 1 + rand.Intn(src.Str) - 1 + rand.Intn(targ.Def)
			if dmg > 0 {
				w.damaged = append(w.damaged, damage{coll, dmg})
				w.combatLog(coll.sourceID, coll.targetID, "dealt %v damage to", dmg)
			} else {
				w.combatLog(coll.targetID, coll.sourceID, "mitigated a hit from")
			}
		} else {
			w.combatLog(coll.sourceID, coll.targetID, "missed")
		}
	}

	// damage decrements HP, maybe kills
	for _, dmg := range w.damaged {
		if w.Entities[dmg.targetID]&componentHP != componentHP {
			continue
		}
		hp := w.HP[dmg.targetID] - dmg.n
		w.HP[dmg.targetID] = hp
		if hp < 0 {
			w.killer = append(w.killer, dmg.sourceID)
			w.killed = append(w.killed, dmg.targetID)
		}
	}

	// bring out your dead
	for i, killed := range w.killed {
		killer := w.killer[i]
		w.combatLog(killed, killer, "killed by")
		w.Entities[killed] &= ^(componentCollide | componentInput | componentHP)
	}

	// count remaining souls
	if w.CountAll(playMoveMask) == 0 {
		w.log("game over")
		return view.Stop
	}

	// maybe spawn
	if n := w.CountAll(combatMask); rand.Float64() < 1/float64(n) {
		// TODO: choose something random
		// TODO: position
		if _, occupied := w.collides(len(w.Entities), point.Zero); !occupied {
			enemy := w.rollChar("enemy", 'x')
			enemy.AddComponent(componentInput | componentAI)
			w.log("%s enters the world @%v stats: %+v", w.Names[enemy.ID()], w.Positions[enemy.ID()], w.Stats[enemy.ID()])
		}
	}

	return nil
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
				componentPosition | componentCollide |
					componentGlyph | componentBG | componentFG)
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
		floor.AddComponent(componentPosition | componentBG)
		w.Positions[floor.ID()] = pos
		w.BG[floor.ID()] = bg
	})
}

func (w *world) rollChar(name string, glyph rune) ecs.Entity {
	ent := w.AddEntity()
	ent.AddComponent(
		componentPosition | componentCollide |
			componentName | componentGlyph |
			componentHP | componentStats)
	id := ent.ID()
	w.Glyphs[id] = glyph
	w.Positions[id] = point.Zero
	w.Names[id] = name
	w.HP[id] = maxHP
	w.Stats[id] = stats{
		Str:  rollStat(),
		Def:  rollStat(),
		Dex:  rollStat(),
		Luck: rollStat(),
	}
	return ent
}

func (w *world) log(mess string, args ...interface{}) {
	w.View.Log(mess, args...)
}

func (w *world) getName(id int, deflt string) string {
	if w.Entities[id]&componentName == 0 {
		return deflt
	}
	if w.Names[id] == "" {
		return deflt
	}
	return w.Names[id]
}

func (w *world) combatLog(src, targ int, mess string, args ...interface{}) {
	a := w.getName(src, "!?!")
	b := w.getName(targ, "?!?")
	mess = fmt.Sprintf("%s %s %s", a, mess, b)
	w.log(mess, args...)
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

		const size = 8
		pt := point.Point{X: size, Y: size}

		w.addBox(point.Box{TopLeft: pt.Neg(), BottomRight: pt}, '#')
		player := w.rollChar("you", '@')
		player.AddComponent(componentInput | componentSoul)
		w.log("%s enter the world @%v stats: %+v", w.Names[player.ID()], w.Positions[player.ID()], w.Stats[player.ID()])

		return &w, nil
	}); err != nil {
		log.Fatal(err)
	}
}
