package main

import (
	"image"

	"github.com/jcorbin/anansi/ansi"
	"github.com/jcorbin/anansi/x/platform"

	"github.com/jcorbin/execs/internal/ecs"
)

type control struct {
	view image.Rectangle
	pos  *position

	ecs.ArrayIndex
	player []ecs.Entity
}

func (ctl *control) Create(player ecs.Entity, _ ecs.Type) {
	i := ctl.ArrayIndex.Insert(player)
	for i >= len(ctl.player) {
		if i < cap(ctl.player) {
			ctl.player = ctl.player[:i+1]
		} else {
			ctl.player = append(ctl.player, ecs.Entity{})
		}
	}
	ctl.player[i] = player
}

func (ctl *control) process(ctx *platform.Context) bool {
	move, interacted := parseTotalMove(ctx.Input)

	if move != image.ZP {
		// TODO other options beyond apply-to-all
		for _, player := range ctl.player {
			// TODO proper movement system
			posd := ctl.pos.Get(player)
			if pos := posd.Point().Add(move); ctl.pos.collides(player, pos) == ecs.ZE {
				posd.SetPoint(pos)
			}
		}
		interacted = true // NOTE should already have been true; TODO maybe differentiate moved=true
	}

	if centroid := ctl.playerCentroid(); ctl.view == image.ZR {
		offset := centroid.Sub(ctx.Output.Size.Div(2))
		ctl.view = image.Rectangle{offset, ctx.Output.Size.Add(offset)}
	} else if ctl.view.Size() != ctx.Output.Size {
		ds := ctl.view.Size().Sub(ctx.Output.Size).Div(2)
		ctl.view.Min = ctl.view.Min.Sub(ds)
		ctl.view.Max = ctl.view.Min.Add(ctx.Output.Size)
	} else if adj := ctl.viewAdjust(centroid); adj != image.ZP {
		ctl.view = ctl.view.Add(adj)
	}

	return interacted
}

func parseTotalMove(in *platform.Events) (move image.Point, interacted bool) {
	for id := range in.Type {
		if dp, any := parseMove(in, id); any {
			move = move.Add(dp)
			interacted = true
		}
	}
	return move, interacted
}

func parseMove(in *platform.Events, id int) (_ image.Point, interacted bool) {
	defer func() {
		if interacted {
			in.Type[id] = platform.EventNone
		}
	}()

	// TODO support numpad

	switch in.Type[id] {
	case platform.EventEscape:
		esc := in.Escape(id)
		if d, isMove := ansi.DecodeCursorCardinal(esc.ID, esc.Arg); isMove {
			return d, true
		}

	case platform.EventRune:
		switch in.Rune(id) {
		case 'y':
			return image.Pt(-1, -1), true
		case 'u':
			return image.Pt(1, -1), true
		case 'n':
			return image.Pt(1, 1), true
		case 'b':
			return image.Pt(-1, 1), true
		case 'h':
			return image.Pt(-1, 0), true
		case 'j':
			return image.Pt(0, 1), true
		case 'k':
			return image.Pt(0, -1), true
		case 'l':
			return image.Pt(1, 0), true
		case '.':
			return image.ZP, true
		}
	}

	return image.ZP, false
}

// TODO proper movement / collision system
func (pos *position) collides(ent ecs.Entity, p image.Point) (hit ecs.Entity) {
	if ent.Type()&gameCollides != 0 {
		n := 0
		for q := pos.At(p); q.next(); {
			hitPosd := q.handle()
			other := hitPosd.Entity()
			typ := other.Type()
			// log.Printf("q:%v coll check %v type:%v", q, other, typ)
			if typ&gameCollides != 0 {
				// TODO better than last wins
				hit = other
			}
			n++
		}
		// FIXME
		// if hit != ecs.ZE {
		// 	log.Printf("%v at %v hit:%v type:%v", n, p, hit, hit.Type())
		// } else {
		// 	log.Printf("%v at %v hit:none", n, p)
		// }
	}
	return hit
}

func (ctl *control) viewAdjust(pt image.Point) (adj image.Point) {
	mid := ctl.centerRegion()
	dmin := pt.Sub(mid.Min)
	dmax := pt.Sub(mid.Max)
	if dmin.X < 0 {
		adj.X = dmin.X
	} else if dmax.X > 0 {
		adj.X = dmax.X
	}
	if dmin.Y < 0 {
		adj.Y = dmin.Y
	} else if dmax.Y > 0 {
		adj.Y = dmax.Y
	}
	return adj
}

func (ctl *control) centerRegion() image.Rectangle {
	mid := ctl.view
	ds := mid.Size().Div(8)
	mid.Min = mid.Min.Add(ds)
	mid.Max = mid.Max.Sub(ds)
	return mid
}

func (ctl *control) playerCentroid() (centroid image.Point) {
	for _, player := range ctl.player {
		centroid = centroid.Add(ctl.pos.Get(player).Point())
	}
	return centroid.Div(len(ctl.player))
}
