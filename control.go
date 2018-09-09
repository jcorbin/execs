package main

import (
	"image"

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
	i := ctl.ArrayIndex.Create(player)
	for i >= len(ctl.player) {
		if i < cap(ctl.player) {
			ctl.player = ctl.player[:i+1]
		} else {
			ctl.player = append(ctl.player, ecs.Entity{})
		}
	}
	ctl.player[i] = player
}

func (ctl *control) Destroy(ent ecs.Entity, _ ecs.Type) {
	ctl.ArrayIndex.Destroy(ent)
}

func (ctl *control) Update(ctx *platform.Context) {
	// TODO support numpad and vi movement keys
	if move := ctx.Input.TotalCursorMovement(); move != image.ZP {
		// TODO other options beyond apply-to-all
		for _, player := range ctl.player {
			// TODO proper movement system
			posd := ctl.pos.Get(player)
			if pos := posd.Point().Add(move); ctl.pos.collides(player, pos) == ecs.ZE {
				posd.SetPoint(pos)
			}
		}
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
}

// TODO proper movement / collision system
func (pos *position) collides(ent ecs.Entity, p image.Point) ecs.Entity {
	if ent.Type()&gameCollides != 0 {
		if hitPosd := pos.At(p); !hitPosd.zero() {
			if other := hitPosd.Entity(); other.Type()&gameCollides != 0 {
				return other
			}
		}
	}
	return ecs.Entity{}
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
