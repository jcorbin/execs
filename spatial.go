package main

import (
	"fmt"
	"image"
	"math"
	"sort"

	"github.com/jcorbin/execs/internal/ecs"
)

type position struct {
	ecs.ArrayIndex
	qi quadIndex
	pt []image.Point
}

type positioned struct {
	pos *position
	i   int
}

func (pos *position) Create(ent ecs.Entity, _ ecs.Type) {
	i := pos.ArrayIndex.Create(ent)
	for i >= len(pos.pt) {
		if i < cap(pos.pt) {
			pos.pt = pos.pt[:i+1]
			pos.qi.ix = pos.qi.ix[:i+1]
			pos.qi.ks = pos.qi.ks[:i+1]
		} else {
			pos.pt = append(pos.pt, image.ZP)
			pos.qi.ix = append(pos.qi.ix, 0)
			pos.qi.ks = append(pos.qi.ks, 0)
		}
	}
	pos.pt[i] = image.ZP
	pos.qi.ix[i] = i
	pos.qi.ks[i] = 0 // pos.qi.key(image.ZP)
}

func (pos *position) Destroy(ent ecs.Entity, _ ecs.Type) {
	pos.ArrayIndex.Destroy(ent)
}

func (pos *position) Get(ent ecs.Entity) positioned {
	if i, def := pos.ArrayIndex.Get(ent); def {
		return positioned{pos, i}
	}
	return positioned{}
}

func (pos *position) At(p image.Point) (posd positioned) {
	if i, ok := pos.qi.search(pos.qi.key(p)); ok {
		posd.pos, posd.i = pos, i
	}
	return posd
}

func (pos *position) Within(r image.Rectangle) (pq positionQuery) {
	pq.pos = pos
	pq.quadQuery = pos.qi.rangeSearch(r)
	pq.quadQuery.pt = pos.pt // TODO eliminate need in quadQuery.next
	return pq
}

type positionQuery struct {
	quadQuery
	pos *position
}

func (pq *positionQuery) handle() positioned {
	if pq.i < pq.imax {
		return positioned{pq.pos, pq.i}
	}
	return positioned{}
}

func (posd positioned) zero() bool { return posd.pos == nil }

func (posd positioned) Point() image.Point {
	if posd.pos == nil {
		return image.ZP
	}
	return posd.pos.pt[posd.i]
}

func (posd positioned) SetPoint(p image.Point) {
	if posd.pos != nil {
		posd.pos.pt[posd.i] = p
		posd.pos.qi.update(posd.i, p)
	}
}

func (posd positioned) Entity() ecs.Entity {
	return posd.pos.Scope.Entity(posd.pos.ID(posd.i))
}

func (posd positioned) String() string {
	return fmt.Sprintf("pt: %v", posd.pos.pt[posd.i])
}

type quadIndex struct {
	ix []int
	ks []uint64
}

type quadQuery struct {
	pt         []image.Point
	r          image.Rectangle
	kmin, kmax uint64
	imin, imax int
	i          int
}

func (qi quadIndex) Len() int           { return len(qi.ix) }
func (qi quadIndex) Less(i, j int) bool { return qi.ks[qi.ix[i]] < qi.ks[qi.ix[j]] }
func (qi quadIndex) Swap(i, j int)      { qi.ix[i], qi.ix[j] = qi.ix[j], qi.ix[i] }

func (qi quadIndex) key(p image.Point) uint64 { return zorderKey(p) }

func (qi quadIndex) update(i int, p image.Point) {
	qi.ks[i] = qi.key(p)
	sort.Sort(qi)
}

func (qi quadIndex) search(k uint64) (int, bool) {
	ii := sort.Search(len(qi.ix), func(ii int) bool {
		i := qi.ix[ii]
		return qi.ks[i] >= k
	})
	if ii < len(qi.ix) {
		if i := qi.ix[ii]; qi.ks[i] == k {
			return i, true
		}
	}
	return 0, false
}

func (qi quadIndex) rangeSearch(r image.Rectangle) (qq quadQuery) {
	var ok bool
	qq.r = r
	qq.kmin = qi.key(r.Min)
	qq.kmax = qi.key(r.Max)
	qq.imin, ok = qi.search(qq.kmin)
	qq.imax, _ = qi.search(qq.kmax)
	if ok {
		qq.i = qq.imin - 1
	} else {
		qq.i = len(qi.ix)
	}
	return qq
}

func (qq *quadQuery) next() bool {
	for qq.i++; qq.i < qq.imax; qq.i++ {
		// TODO skip directly by computing BIGMIN rather than scanning
		if pt := qq.pt[qq.i]; pt.In(qq.r) {
			return true
		}
	}
	return false
}

// TODO: evaluate hilbert instead of z-order
func zorderKey(p image.Point) (z uint64) {
	// TODO: evaluate a table ala
	// https://graphics.stanford.edu/~seander/bithacks.html#InterleaveTableObvious
	x, y := truncInt32(p.X), truncInt32(p.Y)
	for i := uint(0); i < 32; i++ {
		z |= (x&(1<<i))<<i | (y&(1<<i))<<(i+1)
	}
	return z
}

func truncInt32(n int) uint64 {
	if n < math.MinInt32 {
		return 0
	}
	if n > math.MaxInt32 {
		return math.MaxUint32
	}
	return uint64(uint32(n - math.MinInt32))
}
