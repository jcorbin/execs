package main

import (
	"image"
	"testing"

	"github.com/jcorbin/execs/internal/ecs"
	"github.com/stretchr/testify/assert"
)

func Test_position(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []image.Point
	}{
		{
			name: "origin",
			data: []image.Point{image.Pt(0, 0)},
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
			var sc ecs.Scope
			var pos position
			sc.Watch(0, 0, &pos)

			// load
			for i, pt := range tc.data {
				idt := ecs.Type(i + 1)
				ent := sc.Create(idt)
				posd := pos.Get(ent)
				posd.SetPoint(pt)
			}

			// readback via At
			for i, pt := range tc.data {
				idt := ecs.Type(i + 1)
				found := false
				for q := pos.At(pt); q.next(); {
					posd := q.handle()
					ent := posd.Entity()
					if ent.Type() == idt {
						found = true
						break
					}
				}
				if !assert.True(t, found, "expected to find [%v]%v", i, pt) {
					t.Logf("q: %v", pos.At(pt))

					t.Logf("id,type,pi,X,Y,key,ix")
					for i := 0; i < pos.ArrayIndex.Len(); i++ {
						id := pos.ArrayIndex.ID(i)
						ent := sc.Entity(id)
						t.Logf(
							"%v,%v,%v,%v,%v,%016x,%v",
							id, ent.Type(), i,
							pos.pt[i].X, pos.pt[i].Y,
							pos.qi.ks[i], pos.qi.ix[i],
						)
					}

				}
			}

			// TODO At misses

			// TODO Within queries
		})
	}

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
