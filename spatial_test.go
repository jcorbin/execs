package main

import (
	"image"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jcorbin/execs/internal/ecs"
)

func Test_position(t *testing.T) {
	for _, tc := range []struct {
		name   string
		data   []image.Point
		zeros  []image.Point
		within map[image.Rectangle][]int
	}{
		{
			name:  "origin",
			data:  []image.Point{image.Pt(0, 0)},
			zeros: testPointRing(image.Rect(-1, -1, 1, 1)),
			within: map[image.Rectangle][]int{
				testRectAround(image.Pt(0, 0)):   {0}, // centered around only
				testRectAround(image.Pt(-1, -1)): nil, // Max exclusive
				testRectAround(image.Pt(1, 1)):   {0}, // Min inclusive
				testRectAround(image.Pt(42, 42)): nil, // far point
			},
		},
		{
			name: "four-quadrant square",
			data: []image.Point{
				image.Pt(8, 8),
				image.Pt(8, -8),
				image.Pt(-8, -8),
				image.Pt(-8, 8),
			},

			zeros: flattenPoints(
				testRingAround(image.Pt(8, 8)),
				testRingAround(image.Pt(8, -8)),
				testRingAround(image.Pt(-8, -8)),
				testRingAround(image.Pt(-8, 8)),
			),

			within: map[image.Rectangle][]int{
				testRectAround(image.Pt(8, 8)):   {0},
				testRectAround(image.Pt(8, -8)):  {1},
				testRectAround(image.Pt(-8, -8)): {2},
				testRectAround(image.Pt(-8, 8)):  {3},
				image.Rect(7, -9, 9, 9):          {0, 1},
				image.Rect(-9, -9, -7, 9):        {2, 3},
				image.Rect(-9, 7, 9, 9):          {0, 3},
				image.Rect(-9, -9, 9, -7):        {1, 2},
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

			// Within queries
			for r, is := range tc.within {
				var res []int
				for q := tp.pos.Within(r); q.next(); {
					posd := q.handle()
					ent := posd.Entity()
					res = append(res, int(ent.ID)-1)
				}
				sort.Ints(res)
				if !assert.Equal(t, is, res, "expected points within %v", r) {
					t.Logf("q: %v", tp.pos.Within(r))
					tp.dump()
				}
			}

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

func flattenPoints(ptss ...[]image.Point) []image.Point {
	n := 0
	for _, pts := range ptss {
		n += len(pts)
	}
	r := make([]image.Point, 0, n)
	for _, pts := range ptss {
		r = append(r, pts...)
	}
	return r
}

func testRectAround(pt image.Point) image.Rectangle {
	return image.Rectangle{
		pt.Sub(image.Pt(1, 1)),
		pt.Add(image.Pt(1, 1)),
	}
}

func testRingAround(pt image.Point) []image.Point {
	return testPointRing(testRectAround(pt))
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
