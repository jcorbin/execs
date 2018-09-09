package main

import (
	"fmt"
	"image"
	"sort"

	"github.com/jcorbin/anansi"
	"github.com/jcorbin/anansi/ansi"

	"github.com/jcorbin/execs/internal/ecs"
)

type render struct {
	ecs.ArrayIndex
	pt   []image.Point // TODO shouldn't own the position data
	z    []int
	cell []cell
	zord []int
}

type cell struct {
	r rune
	a ansi.SGRAttr
}

func (ren *render) drawRegionInto(view image.Rectangle, grid *anansi.Grid) {
	ren.rezort() // TODO invalidation based approach, try to defer to inter-frame bg work
	for _, i := range ren.zord {
		if pt := ren.pt[i]; pt.In(view) {
			pt = pt.Sub(view.Min)
			if c := grid.Cell(pt); c.Rune() == 0 {
				c.Set(ren.cell[i].r, ren.cell[i].a)
			} else {
				a := c.Attr()
				if _, bgSet := a.BG(); !bgSet {
					if color, haveBG := ren.cell[i].a.BG(); haveBG {
						c.SetAttr(a | color.BG())
					}
				}
			}
		}
	}
}

func (ren *render) rezort() {
	if ren.zord != nil {
		ren.zord = ren.zord[:0]
	}
	for i := 0; i < ren.Len(); i++ {
		if ren.ID(i) != 0 {
			ren.zord = append(ren.zord, i)
		}
	}
	sort.Slice(ren.zord, ren.zcmp)
}

func (ren *render) zcmp(i, j int) bool {
	return ren.z[ren.zord[i]] > ren.z[ren.zord[j]]
}

func (ren *render) Create(ent ecs.Entity, _ ecs.Type) {
	i := ren.ArrayIndex.Create(ent)
	for i >= len(ren.pt) {
		if i < cap(ren.pt) {
			ren.pt = ren.pt[:i+1]
			ren.z = ren.z[:i+1]
			ren.cell = ren.cell[:i+1]
		} else {
			ren.pt = append(ren.pt, image.ZP)
			ren.z = append(ren.z, 0)
			ren.cell = append(ren.cell, cell{})
		}
	}
	ren.pt[i] = image.ZP
	ren.z[i] = 0
	ren.cell[i] = cell{}
}

func (ren *render) Destroy(ent ecs.Entity, _ ecs.Type) {
	ren.ArrayIndex.Destroy(ent)
}

type renderable struct {
	ren *render
	i   int
}

func (ren *render) Get(ent ecs.Entity) renderable {
	if i, def := ren.ArrayIndex.Get(ent); def {
		return renderable{ren, i}
	}
	return renderable{}
}

func (rend renderable) Point() image.Point {
	if rend.ren == nil {
		return image.ZP
	}
	return rend.ren.pt[rend.i]
}
func (rend renderable) SetPoint(pt image.Point) {
	if rend.ren != nil {
		rend.ren.pt[rend.i] = pt
	}
}

func (rend renderable) Z() int {
	if rend.ren == nil {
		return 0
	}
	return rend.ren.z[rend.i]
}
func (rend renderable) SetZ(z int) {
	if rend.ren != nil {
		rend.ren.z[rend.i] = z
	}
}

func (rend renderable) Cell() (rune, ansi.SGRAttr) {
	if rend.ren == nil {
		return 0, 0
	}
	return rend.ren.cell[rend.i].r, rend.ren.cell[rend.i].a
}
func (rend renderable) SetCell(r rune, a ansi.SGRAttr) {
	if rend.ren != nil {
		rend.ren.cell[rend.i] = cell{r, a}
	}
}

func (rend renderable) String() string {
	return fmt.Sprintf("pt:%v z:%v rune:%q attr:%v",
		rend.ren.pt[rend.i],
		rend.ren.z[rend.i],
		rend.ren.cell[rend.i].r,
		rend.ren.cell[rend.i].a,
	)
}
