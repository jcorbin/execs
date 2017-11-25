package main

import (
	"fmt"
	"strings"

	"github.com/jcorbin/execs/internal/ecs"
	"github.com/jcorbin/execs/internal/point"
	"github.com/jcorbin/execs/internal/view"
	termbox "github.com/nsf/termbox-go"
)

type ui struct {
	View *view.View

	view.Logs
	prompt prompt
	bar    prompt
}

func (ui *ui) init(v *view.View) {
	ui.View = v
	ui.Logs.Init(1000)
	ui.bar.action = make([]promptAction, 0, 5)
	ui.prompt.action = make([]promptAction, 0, 10)
}

func (ui *ui) handle(k view.KeyEvent) (proc, handled bool, err error) {
	defer func() {
		if !handled {
			ui.prompt.reset()
			ui.bar = ui.bar.unwind()
		}
	}()

	if k.Key == termbox.KeyEsc {
		return false, true, view.ErrStop
	}

	if prompting, ok := ui.prompt.handle(k.Ch); ok {
		return !prompting, true, nil
	}

	if pr, prompting, ok := ui.bar.run(k.Ch); ok {
		ui.bar = pr
		return !prompting, true, nil
	}

	return false, false, nil
}

func (w *world) HandleKey(k view.KeyEvent) (rerr error) {
	proc, handled, err := w.ui.handle(k)
	if err != nil {
		return err
	}

	player := w.findPlayer()

	if player != ecs.NilEntity && w.ui.bar.prior == nil {
		defer func() {
			if rerr != nil {
				return
			}
			w.ui.bar.removeAction("Inspect")
			if itemPrompt, haveItemsHere := w.itemPrompt(w.prompt, player); haveItemsHere {
				w.ui.bar.addAction(itemPrompt.activate, "Inspect")
			}
		}()
	}

	// special keys
	if !handled {
		switch k.Ch {
		case ',':
			if player != ecs.NilEntity {
				if itemPrompt, haveItemsHere := w.itemPrompt(w.prompt, player); haveItemsHere {
					w.prompt, _ = itemPrompt.activate(w.prompt.unwind())
				}
			}
			proc, handled = false, true
		case '_':
			if player != ecs.NilEntity {
				if player.Type().All(wcCollide) {
					player.Delete(wcCollide)
					w.Glyphs[player.ID()] = '~'
				} else {
					player.Add(wcCollide)
					w.Glyphs[player.ID()] = 'X'
				}
			}
			proc, handled = true, true
		}
	}

	// parse player move
	if !handled {
		if move, ok := parseMove(k); ok {
			for it := w.Iter(ecs.All(playMoveMask)); it.Next(); {
				w.addPendingMove(it.Entity(), move)
			}
			proc, handled = true, true
		}
	}

	// default to resting
	if !handled {
		for it := w.Iter(ecs.All(playMoveMask)); it.Next(); {
			w.addCharge(it.Entity())
		}
		proc, handled = true, true
	}

	if proc {
		w.Process()
	}

	if w.over {
		return view.ErrStop
	}
	return nil
}

func parseMove(k view.KeyEvent) (point.Point, bool) {
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
	switch k.Ch {
	case 'y':
		return point.Point{X: -1, Y: -1}, true
	case 'u':
		return point.Point{X: 1, Y: -1}, true
	case 'n':
		return point.Point{X: 1, Y: 1}, true
	case 'b':
		return point.Point{X: -1, Y: 1}, true
	case 'h':
		return point.Point{X: -1, Y: 0}, true
	case 'j':
		return point.Point{X: 0, Y: 1}, true
	case 'k':
		return point.Point{X: 0, Y: -1}, true
	case 'l':
		return point.Point{X: 1, Y: 0}, true
	case '.':
		return point.Zero, true
	}
	return point.Zero, false
}

func (ui ui) promptParts() []string {
	if parts := ui.prompt.render(""); len(parts) > 0 {
		return parts
	}
	if ui.bar.prior != nil {
		if parts := ui.bar.render(""); len(parts) > 0 {
			return parts
		}
	}
	return nil
}

func (ui ui) barParts() []string {
	if ui.prompt.mess != "" {
		return nil
	}
	if ui.bar.prior != nil {
		return nil
	}
	if len(ui.bar.action) == 0 {
		return nil
	}
	parts := make([]string, len(ui.bar.action))
	i := 0
	parts[i] = fmt.Sprintf(".<%s[%d]", ui.bar.action[i].mess, i+1)
	for i++; i < len(ui.bar.action); i++ {
		parts[i] = fmt.Sprintf("<%s[%d]", ui.bar.action[i].mess, i+1)
	}
	return parts
}

func (w *world) Render(termGrid view.Grid) error {
	hud := view.HUD{
		Logs:  w.ui.Logs,
		World: w.renderViewport(termGrid.Size),
	}

	hud.Logs.Max = (termGrid.Size.Y - hud.World.Size.Y - 3) / 2
	if hud.Logs.Max < hud.Logs.Min {
		hud.Logs.Max = hud.Logs.Min
	}

	hud.HeaderF("^%v souls v %v demons", w.Iter(ecs.All(wcSoul)).Count(), w.Iter(ecs.All(wcAI)).Count())

	for it := w.Iter(ecs.All(wcSoul | wcBody)); it.Next(); {
		bo := w.bodies[it.ID()]

		// TODO: a charSummary Renderable

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

		charge := 0
		for cur := w.moves.LookupA(ecs.All(movCharge), it.ID()); cur.Scan(); {
			charge += w.moves.n[cur.Entity().ID()]
		}

		// TODO: render a doll like
		//    _O_
		//   / | \
		//   = | =
		//    / \
		//  _/   \_

		hp, maxHP := bo.HPRange()

		hud.FooterF(".>Damage(%v): %s", damage, strings.Join(damageParts, " "))
		hud.FooterF(".>Armor(%v): %s", armor, strings.Join(armorParts, " "))
		hud.FooterF(".>HP(%v/%v): %s", hp, maxHP, strings.Join(hpParts, " "))
		hud.FooterF(".>Charge: %v", charge)
	}

	// TODO: action bar Renderable
	if parts := w.ui.barParts(); len(parts) > 0 {
		for i := 0; i < len(parts); i++ {
			hud.FooterF(parts[i])
		}
	}

	// TODO: prompt should be a Renderable
	if parts := w.ui.promptParts(); len(parts) > 0 {
		for i := len(parts) - 1; i >= 0; i-- {
			hud.FooterF(".<" + parts[i])
		}
	}

	hud.Render(termGrid)
	return nil
}

func (w *world) renderViewport(max point.Point) view.Grid {
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

	// TODO: re-use
	grid := view.MakeGrid(ofbox.Size().Min(max))
	zVals := make([]uint8, len(grid.Data))

	for it := w.Iter(ecs.Clause(wcPosition, wcGlyph|wcBG)); it.Next(); {
		pos := w.Positions[it.ID()].Add(offset)
		if pos.Less(point.Zero) || !pos.Less(grid.Size) {
			continue
		}
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
			hp, maxHP := w.bodies[it.ID()].HPRange()
			if !it.Type().All(wcSoul) {
				zVal--
				fg = safeColorsIX(aiColors, 1+(len(aiColors)-2)*hp/maxHP)
			} else {
				fg = safeColorsIX(soulColors, 1+(len(soulColors)-2)*hp/maxHP)
			}
		} else if it.Type().All(wcSoul) {
			zVal = 127
			fg = soulColors[0]
		} else if it.Type().All(wcAI) {
			zVal = 126
			fg = aiColors[0]
		} else if it.Type().All(wcItem) {
			zVal = 10
			fg = itemColors[len(itemColors)-1]
			if dur, ok := w.items[it.ID()].(durableItem); ok {
				fg = itemColors[0]
				if hp, maxHP := dur.HPRange(); maxHP > 0 {
					fg = safeColorsIX(itemColors, (len(itemColors)-1)*hp/maxHP)
				}
			}
		} else {
			zVal = 2
			if it.Type().All(wcFG) {
				fg = w.FG[it.ID()]
			}
		}

		i := pos.Y*grid.Size.X + pos.X
		if i < 0 || i >= len(zVals) {
			// TODO: debug
			continue
		}

		if zVals[i] < zVal {
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
			grid.Merge(pos.X, pos.Y, ch, fg, bg)
		}
	}

	return grid
}

func (w *world) itemPrompt(pr prompt, ent ecs.Entity) (prompt, bool) {
	// TODO: once we have a proper spatial index, stop relying on
	// collision relations for this
	prompting := false
	for cur := w.moves.Cursor(
		ecs.RelClause(mrCollide, mrItem),
		func(r ecs.RelationType, rel, a, b ecs.Entity) bool { return a == ent },
	); cur.Scan(); {
		if !prompting {
			pr = pr.makeSub("Items Here")
			prompting = true
		}
		item := cur.B()
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
		if !pr.addAction(rem.scavenge, rem.describeScavenge()) {
			break
		}

	}

	return pr, true
}

type prompt struct {
	prior  *prompt
	mess   string
	action []promptAction
}

type promptAction struct {
	mess string
	run  func(prompt) (prompt, bool)
}

func (pr prompt) render(prefix string) []string {
	if pr.mess == "" {
		return nil
	}
	lines := make([]string, 0, 1+len(pr.action))
	lines = append(lines, fmt.Sprintf("%s%s: (Press Number, 0 to exit menu)", prefix, pr.mess))
	for i, act := range pr.action {
		lines = append(lines, fmt.Sprintf("%s%d) %s", prefix, i+1, act.mess))
	}
	return lines
}

func (pr *prompt) handle(ch rune) (prompting, ok bool) {
	if pr.mess == "" {
		return false, false
	}
	if new, prompting, ok := pr.run(ch); ok {
		*pr = new
		return prompting, ok
	}
	pr.reset()
	return false, false
}

func (pr *prompt) reset() {
	*pr = pr.unwind()
	pr.mess = ""
	pr.action = pr.action[:0]
}

func (pr *prompt) activate(prior prompt) (prompt, bool) {
	return prompt{
		prior:  &prior,
		mess:   pr.mess,
		action: pr.action,
	}, true
}

func (pr *prompt) addAction(
	run func(prompt) (prompt, bool),
	mess string, args ...interface{},
) bool {
	if len(pr.action) < cap(pr.action) {
		pr.action = append(pr.action, promptAction{mess, run})
		return true
	}
	return false
}

func (pr *prompt) removeAction(mess string) {
	for i := range pr.action {
		if pr.action[i].mess == mess {
			pr.action = append(pr.action[:i], pr.action[i+1:]...)
			break
		}
	}
}

func (pr prompt) run(ch rune) (_ prompt, prompting, ok bool) {
	n := int(ch - '0')
	if n < 0 || n > 9 {
		return pr, false, false
	}
	if i := n - 1; i < 0 {
		return pr.pop(), false, true
	} else if i < len(pr.action) {
		pr, prompting := pr.action[i].run(pr)
		return pr, prompting, true
	}
	return pr, false, true
}

func (pr prompt) pop() prompt {
	if pr.prior != nil {
		return *pr.prior
	}
	return pr
}

func (pr prompt) unwind() prompt {
	for pr.prior != nil {
		pr = *pr.prior
	}
	return pr
}

func (pr prompt) makeSub(mess string, args ...interface{}) prompt {
	return prompt{
		prior:  &pr,
		mess:   fmt.Sprintf(mess, args...),
		action: make([]promptAction, 0, 10),
	}
}

func safeColorsIX(colors []termbox.Attribute, i int) termbox.Attribute {
	if i < 0 {
		return colors[1]
	}
	if i >= len(colors) {
		return colors[len(colors)-1]
	}
	return colors[i]
}
