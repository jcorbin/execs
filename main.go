package main

import (
	"fmt"
	"math/rand"
	"sort"

	termbox "github.com/nsf/termbox-go"
)

// inspired by
// https://www.gamedev.net/articles/programming/general-and-gameplay-programming/understanding-component-entity-systems-r3013/
// https://www.gamedev.net/articles/programming/general-and-gameplay-programming/implementing-component-entity-systems-r3382

type ComponentType uint64

const (
	ComponentNone ComponentType = 0
	ComponentName               = 1 << iota
	ComponentPosition
	ComponentCollide
	ComponentGlyph
	ComponentStats // TODO: break up
	ComponentInput
	ComponentSoul
	ComponentAI
)

type Position struct{ X, Y int }

var zeroPos = Position{}

func (pos Position) Less(other Position) bool {
	return pos.Y < other.Y || pos.X < other.X
}

func (pos Position) Equal(other Position) bool {
	return pos.X == other.X && pos.Y == other.Y
}

func (pos Position) Clamp(max Position) Position {
	if pos.X < 0 {
		pos.X = 0
	}
	if pos.Y < 0 {
		pos.Y = 0
	}
	if pos.X > max.X {
		pos.X = max.X
	}
	if pos.Y > max.Y {
		pos.Y = max.Y
	}
	return pos
}

func (pos Position) Div(n int) Position {
	pos.X /= n
	pos.Y /= n
	return pos
}

func (pos Position) Mul(n int) Position {
	pos.X *= n
	pos.Y *= n
	return pos
}

func (pos Position) Add(other Position) Position {
	pos.X += other.X
	pos.Y += other.Y
	return pos
}

func (pos Position) Sub(other Position) Position {
	pos.X -= other.X
	pos.Y -= other.Y
	return pos
}

func (pos Position) Abs() Position {
	if pos.X < 0 {
		pos.X = -pos.X
	}
	if pos.Y < 0 {
		pos.Y = -pos.Y
	}
	return pos
}

func sign(i int) int {
	if i < 0 {
		return -1
	}
	if i > 0 {
		return 1
	}
	return 0
}

func (pos Position) Sign() Position {
	pos.X = sign(pos.X)
	pos.Y = sign(pos.Y)
	return pos
}

type Stats struct {
	HP, MaxHP           int
	Str, Def, Dex, Luck int
}

type World struct {
	Entities  []ComponentType
	Names     []string
	Positions []Position
	Glyphs    []rune
	Stats     []Stats // TODO: this being dense is quite wasteful for walls

	logs []string

	// TODO: collect render state
	viewSize Position

	// TODO: collect collision system state
	coll  []int
	colls []collision

	// TODO: collect combat system state
	damaged []damage
	killed  []int
	killer  []int
}

type collision struct {
	sourceID int
	targetID int
}

type damage struct {
	collision
	n int
}

type Entity struct {
	w  *World
	id int
}

func (w *World) AddEntity() Entity {
	ent := Entity{w, len(w.Entities)}
	w.Entities = append(w.Entities, ComponentNone)
	w.Names = append(w.Names, "")
	w.Positions = append(w.Positions, Position{})
	w.Glyphs = append(w.Glyphs, 0)
	w.Stats = append(w.Stats, Stats{})
	return ent
}

func (e Entity) AddComponent(t ComponentType)    { e.w.Entities[e.id] |= t }
func (e Entity) RemoveComponent(t ComponentType) { e.w.Entities[e.id] &= ^t }
func (e Entity) Has(t ComponentType) bool        { return (e.w.Entities[e.id] & t) == t }
func (e Entity) Type() ComponentType             { return e.w.Entities[e.id] }

func (e Entity) Name() string {
	if !e.Has(ComponentName) {
		return ""
	}
	return e.w.Names[e.id]
}
func (e Entity) SetName(name string) {
	e.w.Entities[e.id] |= ComponentName
	e.w.Names[e.id] = name
}

func (e Entity) Position() Position {
	if !e.Has(ComponentPosition) {
		return Position{}
	}
	return e.w.Positions[e.id]
}
func (e Entity) SetPosition(pos Position) {
	e.w.Entities[e.id] |= ComponentPosition
	e.w.Positions[e.id] = pos
}

func (e Entity) Glyph() rune {
	if !e.Has(ComponentGlyph) {
		return 0
	}
	return e.w.Glyphs[e.id]
}
func (e Entity) SetGlyph(r rune) {
	e.w.Entities[e.id] |= ComponentGlyph
	e.w.Glyphs[e.id] = r
}

func (e Entity) Stats() Stats {
	if !e.Has(ComponentStats) {
		return Stats{}
	}
	return e.w.Stats[e.id]
}
func (e Entity) SetStats(st Stats) {
	e.w.Entities[e.id] |= ComponentStats
	e.w.Stats[e.id] = st
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

func rollStats() Stats {
	return Stats{
		HP: 20, MaxHP: 20,
		Str:  rollStat(),
		Def:  rollStat(),
		Dex:  rollStat(),
		Luck: rollStat(),
	}
}

func (w *World) addRenderable(pos Position, glyph rune) Entity {
	ent := w.AddEntity()
	ent.AddComponent(ComponentCollide)
	ent.SetGlyph(glyph)
	ent.SetPosition(pos)
	return ent
}

func (w *World) AddBox(tl, br Position, glyph rune) {
	// TODO: the box should be an entity, rather than each cell
	sz := br.Sub(tl).Abs()
	pos := tl
	for i := 0; i < sz.X; i++ {
		w.addRenderable(pos, glyph)
		pos.X++
	}
	for i := 0; i < sz.Y; i++ {
		w.addRenderable(pos, glyph)
		pos.Y++
	}
	for i := 0; i < sz.X; i++ {
		w.addRenderable(pos, glyph)
		pos.X--
	}
	for i := 0; i < sz.Y; i++ {
		w.addRenderable(pos, glyph)
		pos.Y--
	}
}

func (w *World) RollChar(name string, glyph rune) Entity {
	ent := w.addRenderable(Position{0, 0}, glyph)
	ent.SetName(name)
	ent.SetStats(rollStats())
	return ent
}

const (
	collMask     = ComponentPosition | ComponentCollide
	playMoveMask = ComponentPosition | ComponentInput | ComponentSoul
	aiMoveMask   = ComponentPosition | ComponentInput | ComponentAI
	combatMask   = ComponentCollide | ComponentStats
	renderMask   = ComponentPosition | ComponentGlyph
)

func (w *World) Move(move Position) {
	// soulless world are dead
	if w.CountAll(ComponentSoul) == 0 {
		return
	}

	// collect collidables
	w.prepareCollidables()

	// ai chase target; last one wins for now TODO
	var target Position

	// apply player move
	for id, t := range w.Entities {
		if t&playMoveMask == playMoveMask {
			target = w.move(id, move)
		}
	}

	// chase player
	for id, t := range w.Entities {
		if t&aiMoveMask == aiMoveMask {
			w.move(id, target.Sub(w.Positions[id]).Sign())
		}
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
				w.CombatLog(coll.sourceID, coll.targetID, "dealt %v damage to", dmg)
			} else {
				w.CombatLog(coll.targetID, coll.sourceID, "mitigated a hit from")
			}
		} else {
			w.CombatLog(coll.sourceID, coll.targetID, "missed")
		}
	}

	// damage decrements HP, maybe kills
	for _, dmg := range w.damaged {
		if w.Entities[dmg.targetID]&ComponentStats != ComponentStats {
			continue
		}
		hp := w.Stats[dmg.targetID].HP - dmg.n
		w.Stats[dmg.targetID].HP = hp
		if hp < 0 {
			w.killer = append(w.killer, dmg.sourceID)
			w.killed = append(w.killed, dmg.targetID)
		}
	}

	// bring out your dead
	for i, killed := range w.killed {
		if w.Entities[killed]&ComponentName != 0 {
			killer := w.killer[i]
			if w.Entities[killer]&ComponentName != 0 {
				w.Log("%s has died (killed by %s)", w.Names[killed], w.Names[killer])
			} else {
				w.Log("%s has died", w.Names[killed])
			}
		}
		w.Entities[killed] = ComponentNone
	}

	// count remaining souls
	if w.CountAll(ComponentSoul) == 0 {
		w.Log("game over")
		return
	}

	// maybe spawn
	if n := w.CountAll(combatMask); rand.Float64() < 1/float64(n) {
		// TODO: choose something random
		// TODO: position
		if _, occupied := w.collides(len(w.Entities), Position{0, 0}); !occupied {
			w.RollChar("enemy", 'x').AddComponent(ComponentInput | ComponentAI)
			w.Log("spawned an enemy")
		}
	}
}

func (w *World) Render() (rerr error) {
	buf := make([]termbox.Cell, w.viewSize.X*w.viewSize.Y)
	defer func() {
		if rerr == nil {
			termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
			copy(termbox.CellBuffer(), buf)
			rerr = termbox.Flush()
		}
	}()

	// collect world extent, and compute a viewport focus position
	var (
		topLeft, bottomRight Position
		focus                Position
		first                bool
	)
	for id, t := range w.Entities {
		if t&renderMask == renderMask {
			pos := w.Positions[id]
			if t&ComponentSoul != 0 {
				// TODO: centroid between all souls would be a way to move
				// beyond "last wins"
				focus = pos
			}
			if first {
				topLeft = pos
				bottomRight = pos
				first = false
				continue
			}
			if pos.X < topLeft.X {
				topLeft.X = pos.X
			}
			if pos.Y < topLeft.Y {
				topLeft.Y = pos.Y
			}
			if pos.X > bottomRight.X {
				bottomRight.X = pos.X
			}
			if pos.Y > bottomRight.Y {
				bottomRight.Y = pos.Y
			}
		}
	}

	// calculate viewport to center the focus point
	viewOffset := w.viewSize.Div(2).Sub(focus)

	// render all glyphs in view
	for id, t := range w.Entities {
		if t&renderMask == renderMask {
			pos := w.Positions[id].Add(viewOffset)
			if pos.Less(zeroPos) {
				continue
			}
			if w.viewSize.Less(pos) {
				continue
			}
			buf[pos.Y*w.viewSize.X+pos.X].Ch = w.Glyphs[id]
		}
	}

	// render status header
	status := fmt.Sprintf("%v souls v %v demons", w.CountAll(ComponentSoul), w.CountAll(ComponentAI))
	x := w.viewSize.X - len(status)
	if x < 0 {
		x = 0
	}
	for _, r := range status {
		buf[x].Ch = r
		x++
		if x >= w.viewSize.X {
			break
		}
	}

	// render log at bottom of screen
	// TODO: draw a nice frame around the log area
	if len(w.logs) > 0 {
		n := 5 // TODO: work with offset above to opportunistically expand
		if n > len(w.logs) {
			n = len(w.logs)
		}
		for i, y := len(w.logs)-n, w.viewSize.Y-n; i < len(w.logs); {
			mess := w.logs[i]

			x := 4
			off := y*w.viewSize.X + x
			for _, r := range mess {
				buf[off].Ch = r
				x++
				off++
				if x >= w.viewSize.X {
					break
				}
			}

			i++
			y++
		}
	}

	// if len(w.logs) > 0 {
	// 	for _, s := range w.logs {
	// 		fmt.Printf("- %s\n", s)
	// 	}
	// 	fmt.Printf("\n")
	// }

	return nil
}

func (w *World) CountAll(mask ComponentType) int {
	n := 0
	for _, t := range w.Entities {
		if t&mask == mask {
			n++
		}
	}
	return n
}

func (w *World) CountAny(mask ComponentType) int {
	n := 0
	for _, t := range w.Entities {
		if t&mask != 0 {
			n++
		}
	}
	return n
}

func (w *World) CombatLog(src, targ int, mess string, args ...interface{}) {
	a, b := w.Names[src], w.Names[targ]
	if a == "" {
		a = "!?!"
	}
	if b == "" {
		b = "?!?"
	}
	mess = fmt.Sprintf("%s %s %s", a, mess, b)
	w.Log(mess, args...)
}

func (w *World) Log(mess string, args ...interface{}) {
	w.logs = append(w.logs, fmt.Sprintf(mess, args...))
}

func (w *World) prepareCollidables() {
	// TODO: maintain a cleverer structure, like a quad-tree, instead
	if cap(w.coll) < len(w.Entities) {
		w.coll = make([]int, 0, len(w.Entities))
	} else {
		w.coll = w.coll[:0]
	}
	for id, t := range w.Entities {
		if t&collMask == collMask {
			w.coll = append(w.coll, id)
		}
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

func (w *World) collides(id int, pos Position) (int, bool) {
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

func (w *World) move(id int, move Position) Position {
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

func key2move(ev termbox.Event) (Position, bool) {
	switch ev.Key {
	case termbox.KeyArrowDown:
		return Position{0, 1}, true
	case termbox.KeyArrowUp:
		return Position{0, -1}, true
	case termbox.KeyArrowLeft:
		return Position{-1, 0}, true
	case termbox.KeyArrowRight:
		return Position{1, 0}, true
	}
	return Position{}, true
}

func termboxSize() Position {
	w, h := termbox.Size()
	return Position{w, h}
}

func signal(ch chan<- struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func main() {
	if err := func() error {
		if err := termbox.Init(); err != nil {
			return err
		}
		defer termbox.Close()

		termbox.SetInputMode(termbox.InputEsc)

		resized := make(chan struct{}, 1)
		redraw := make(chan struct{}, 1)
		done := make(chan error)
		moves := make(chan Position, 1)

		go func() {
		evloop:
			for {
				switch ev := termbox.PollEvent(); ev.Type {
				case termbox.EventKey:
					switch ev.Key {
					case termbox.KeyCtrlC:
						break evloop
					case termbox.KeyCtrlL:
						signal(redraw)
						continue
					}

					if move, ok := key2move(ev); ok {
						select {
						case moves <- move:
						default:
						}
						continue
					}

					fmt.Printf("KEY: %+v\n", ev)

				case termbox.EventResize:
					signal(resized)

				case termbox.EventError:
					done <- ev.Err
					break evloop
				}
			}
			close(done)
		}()

		for {
			var w World
			w.viewSize = termboxSize()

			w.AddBox(Position{-5, -5}, Position{5, 5}, '#')

			player := w.RollChar("you", '@')
			player.AddComponent(ComponentInput | ComponentSoul)
			w.Log("%s enter the world @%v", player.Name(), player.Position())

			w.Render()
			for w.CountAll(ComponentSoul) > 0 {
				select {

				case err := <-done:
					return err

				case <-resized:
					w.viewSize = termboxSize()

				case <-redraw:
					if err := termbox.Sync(); err != nil {
						return err
					}

				case move := <-moves:
					w.Move(move)
				}

				w.Render()
			}
		}
	}(); err != nil {
		panic(err)
	}
}
