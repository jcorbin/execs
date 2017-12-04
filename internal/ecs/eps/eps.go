package eps

import (
	"math"
	"sort"

	"github.com/jcorbin/execs/internal/ecs"
	"github.com/jcorbin/execs/internal/point"
)

// TODO: support movement on top of or within an EPS:
// - an ecs.Relation on positioned things:
//   - intent: direction & magnitude? jumping?
//   - outcome: collision (only if solid)? pre-compute "what's here"?

// EPS is an Entity Positioning System; (technically it's not an ecs.System, it
// just has a reference to an ecs.Core).
type EPS struct {
	core *ecs.Core
	t    ecs.ComponentType

	inval int
	pt    []point.Point
	ix    index
}

type epsFlag uint8

const (
	epsDef epsFlag = 1 << iota
	epsInval
)

// Init ialize the EPS wrt a given core and component type that
// represents "has a position".
func (eps *EPS) Init(core *ecs.Core, t ecs.ComponentType) {
	eps.core = core
	eps.t = t
	eps.core.RegisterAllocator(eps.t, eps.alloc)
	eps.core.RegisterCreator(eps.t, eps.create)
	eps.core.RegisterDestroyer(eps.t, eps.destroy)
}

// Get the position of an entity; the bool argument is true only if
// the entity actually has a position.
func (eps *EPS) Get(ent ecs.Entity) (point.Point, bool) {
	if ent == ecs.NilEntity {
		return point.Zero, false
	}
	id := eps.core.Deref(ent)
	return eps.pt[id-1], eps.ix.flg[id-1]&epsDef != 0
}

// Set the position of an entity, adding the eps's component if
// necessary.
func (eps *EPS) Set(ent ecs.Entity, pt point.Point) {
	id := eps.core.Deref(ent)
	if eps.ix.flg[id-1]&epsDef == 0 {
		ent.Add(eps.t)
	}
	eps.pt[id-1] = pt
	eps.ix.key[id-1] = zorderKey(pt)
	if flg := eps.ix.flg[id-1]; flg&epsInval == 0 {
		eps.ix.flg[id-1] = flg | epsInval
		eps.inval++
	}
}

// At returns a slice of entities at a given point.
func (eps *EPS) At(pt point.Point) (ents []ecs.Entity) {
	if eps.inval > 0 {
		eps.reindex()
	}
	k := zorderKey(pt)
	i, m := eps.ix.searchRun(k)
	if m > 0 {
		ents = make([]ecs.Entity, m)
		for j := 0; j < m; i, j = i+1, j+1 {
			xi := eps.ix.ix[i]
			ents[j] = eps.core.Ref(ecs.EntityID(xi + 1))
		}
	}
	return ents
}

// TODO: NN queries, range queries, etc
// func (eps *EPS) Near(pt point.Point, d uint) []ecs.Entity
// func (eps *EPS) Within(box point.Box) []ecs.Entity

func (eps *EPS) alloc(id ecs.EntityID, t ecs.ComponentType) {
	i := len(eps.pt)
	eps.pt = append(eps.pt, point.Zero)
	eps.ix.flg = append(eps.ix.flg, 0)
	eps.ix.key = append(eps.ix.key, 0)
	eps.ix.ix = append(eps.ix.ix, i)
}

func (eps *EPS) create(id ecs.EntityID, t ecs.ComponentType) {
	eps.ix.flg[id-1] |= epsDef
	eps.ix.key[id-1] = zorderKey(eps.pt[id-1])
	if flg := eps.ix.flg[id-1]; flg&epsInval == 0 {
		eps.ix.flg[id-1] = flg | epsInval
		eps.inval++
	}
}

func (eps *EPS) destroy(id ecs.EntityID, t ecs.ComponentType) {
	eps.pt[id-1] = point.Zero
	eps.ix.flg[id-1] &= ^epsDef
	eps.ix.key[id-1] = 0
	if flg := eps.ix.flg[id-1]; flg&epsInval == 0 {
		eps.ix.flg[id-1] = flg | epsInval
		eps.inval++
	}
}

func (eps *EPS) reindex() {
	// TODO: worth a fix-one algorithm?
	sort.Sort(eps.ix)
	for i := range eps.ix.flg {
		eps.ix.flg[i] &= ^epsInval
	}
	eps.inval = 0
}

type index struct {
	flg []epsFlag
	key []uint64
	ix  []int
}

type subindex struct {
	ixi   []int // subset of indices in...
	index       // ...the underlying index
}

func (ix index) Len() int      { return len(ix.ix) }
func (ix index) Swap(i, j int) { ix.ix[i], ix.ix[j] = ix.ix[j], ix.ix[i] }
func (ix index) Less(i, j int) bool {
	xi, xj := ix.ix[i], ix.ix[j]
	if ix.flg[xi]&epsDef == 0 {
		return ix.flg[xj]&epsDef != 0
	} else if ix.flg[xj]&epsDef == 0 {
		return false
	}
	return ix.key[xi] < ix.key[xj]
}

func (si subindex) Len() int           { return len(si.ixi) }
func (si subindex) Less(i, j int) bool { return si.index.Less(si.ixi[i], si.ixi[j]) }
func (si subindex) Swap(i, j int)      { si.index.Swap(si.ixi[i], si.ixi[j]) }

func (ix index) search(i, j int, key uint64) int {
	// adapted from sort.Search
	for i < j {
		h := int(uint(i+j) >> 1)
		if xi := ix.ix[h]; ix.flg[xi]&epsDef != 0 && ix.key[xi] >= key {
			j = h
		} else {
			i = h + 1
		}
	}
	return i
}

func (ix index) searchRun(key uint64) (i, m int) {
	i = ix.search(0, len(ix.ix), key)
	for j := i; j < len(ix.ix); j++ {
		if xi := ix.ix[j]; ix.flg[xi]&epsDef == 0 || ix.key[xi] != key {
			break
		}
		m++
	}
	return i, m
}

// TODO: evaluate hilbert instead of z-order
func zorderKey(pt point.Point) (z uint64) {
	// TODO: evaluate a table ala
	// https://graphics.stanford.edu/~seander/bithacks.html#InterleaveTableObvious
	x, y := truncInt32(pt.X), truncInt32(pt.Y)
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
