package main

import (
	"image"
	"testing"

	"github.com/jcorbin/execs/internal/ecs"
	"github.com/stretchr/testify/assert"
)

func Test_position(t *testing.T) {
	for _, tc := range []struct {
		name  string
		data  []image.Point
		zeros []image.Point
	}{
		{
			name:  "origin",
			data:  []image.Point{image.Pt(0, 0)},
			zeros: testPointRing(image.Rect(-1, -1, 1, 1)),
		},
		{
			name: "four-quadrant square",
			data: []image.Point{
				image.Pt(8, 8),
				image.Pt(8, -8),
				image.Pt(-8, -8),
				image.Pt(-8, 8),
			},
		},

		{
			name: "3x3 square",
			data: testPointRect(image.Rect(0, 0, 3, 3)),
		},

		{
			name: "center point on a 3x3 square",
			data: append(
				testPointRect(image.Rect(0, 0, 3, 3)),
				image.Pt(1, 1)),
		},

		{
			name: "3x3 inset on a 5x5 square",
			data: append(
				testPointRect(image.Rect(0, 0, 5, 5)),
				testPointRect(image.Rect(1, 1, 4, 4))...),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tp := newTestPos(t, tc.data)

			// readback via At
			for i, pt := range tc.data {
				idt := ecs.Type(i + 1)
				found := false
				for q := tp.pos.At(pt); q.next(); {
					posd := q.handle()
					ent := posd.Entity()
					if ent.Type() == idt {
						found = true
						break
					}
				}
				if !assert.True(t, found, "expected to find [%v]%v", i, pt) {
					t.Logf("q: %v", tp.pos.At(pt))
					tp.dump()
				}
			}

			// At zeros
			for _, pt := range tc.zeros {
				if q := tp.pos.At(pt); !assert.False(t, q.next(), "expected zero at %v", pt) {
					t.Logf("q: %v", tp.pos.At(pt))
					tp.dump()
				}
			}

			// TODO Within queries
		})
	}

}

type testPos struct {
	*testing.T
	ecs.Scope
	pos position
}

func newTestPos(t *testing.T, data []image.Point) *testPos {
	var tp testPos
	tp.T = t
	tp.Watch(0, 0, &tp.pos)
	for i, pt := range data {
		tp.create(ecs.Type(i+1), pt)
	}
	return &tp
}

func (tp *testPos) create(t ecs.Type, pt image.Point) ecs.Entity {
	ent := tp.Create(t)
	posd := tp.pos.Get(ent)
	posd.SetPoint(pt)
	return ent
}

func (tp *testPos) dump() {
	tp.Logf("id,type,pi,X,Y,key,ix")
	for i := 0; i < tp.pos.ArrayIndex.Len(); i++ {
		id := tp.pos.ArrayIndex.ID(i)
		ent := tp.Entity(id)
		tp.Logf(
			"%v,%v,%v,%v,%v,%016x,%v",
			id, ent.Type(), i,
			tp.pos.pt[i].X, tp.pos.pt[i].Y,
			tp.pos.qi.ks[i], tp.pos.qi.ix[i],
		)
	}
}

func testPointRing(r image.Rectangle) (pts []image.Point) {
	pt := r.Min
	pts = make([]image.Point, 0, 2*r.Dx()+2*r.Dy())
	for _, st := range []struct {
		d image.Point
		n int
	}{
		{image.Pt(1, 0), r.Dx()},
		{image.Pt(0, 1), r.Dy()},
		{image.Pt(-1, 0), r.Dx()},
		{image.Pt(0, -1), r.Dy()},
	} {
		for i := 0; i < st.n; i++ {
			pts = append(pts, pt)
			pt = pt.Add(st.d)
		}
	}
	return pts
}

func testPointRect(r image.Rectangle) (pts []image.Point) {
	pts = make([]image.Point, 0, r.Dx()*r.Dy())
	for pt := r.Min; pt.Y < r.Max.Y; pt.Y++ {
		for pt.X = r.Min.X; pt.X < r.Max.X; pt.X++ {
			pts = append(pts, pt)
		}
	}
	return pts
}
