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

	frozen bool
	pt     []point.Point
	ix     index
}

type index struct {
	def []bool
	key []uint64
	ix  []int
}

// Init ialize the EPS wrt a given core and component type that
// represents "has a position".
func (eps *EPS) Init(core *ecs.Core, t ecs.ComponentType) {
	eps.core = core
	eps.t = t
	eps.core.RegisterAllocator(eps.t, eps.alloc)
	eps.core.RegisterCreator(eps.t, eps.create)
	eps.core.RegisterDestroyer(eps.t, eps.destroy)

	eps.pt = []point.Point{point.Zero}
	eps.ix.def = []bool{false}
	eps.ix.key = []uint64{0}
	eps.ix.ix = []int{-1}
}

// Get the position of an entity; the bool argument is true only if
// the entity actually has a position.
func (eps *EPS) Get(ent ecs.Entity) (point.Point, bool) {
	if ent == ecs.NilEntity {
		return point.Zero, false
	}
	id := eps.core.Deref(ent)
	return eps.pt[id], eps.ix.def[id]
}

// Set the position of an entity, adding the eps's component if
// necessary.
func (eps *EPS) Set(ent ecs.Entity, pt point.Point) {
	id := eps.core.Deref(ent)
	eps.frozen = true
	if !eps.ix.def[id] {
		ent.Add(eps.t)
	}
	eps.pt[id] = pt
	eps.ix.key[id] = zorderKey(pt)
	eps.frozen = false
	sort.Sort(eps.ix) // TODO: worth a fix-one algorithm?
}

// At returns a slice of entities at a given point.
func (eps *EPS) At(pt point.Point) (ents []ecs.Entity) {
	k := zorderKey(pt)
	i, m := eps.ix.searchRun(k)
	if m > 0 {
		ents = make([]ecs.Entity, m)
		for j := 0; j < m; i, j = i+1, j+1 {
			xi := eps.ix.ix[i+1]
			ents[j] = eps.core.Ref(ecs.EntityID(xi))
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
	eps.ix.def = append(eps.ix.def, false)
	eps.ix.key = append(eps.ix.key, 0)
	eps.ix.ix = append(eps.ix.ix, i)
}

func (eps *EPS) create(id ecs.EntityID, t ecs.ComponentType) {
	eps.ix.def[id] = true
	eps.ix.key[id] = zorderKey(eps.pt[id])
	if !eps.frozen {
		sort.Sort(eps.ix) // TODO: worth a fix-one algorithm?
	}
}

func (eps *EPS) destroy(id ecs.EntityID, t ecs.ComponentType) {
	eps.pt[id] = point.Zero
	eps.ix.def[id] = false
	eps.ix.key[id] = 0
	if !eps.frozen {
		sort.Sort(eps.ix) // TODO: worth a fix-one algorithm?
	}
}

func (ix index) Len() int { return len(ix.ix) - 1 }

func (ix index) Less(i, j int) bool {
	xi, xj := ix.ix[i+1], ix.ix[j+1]
	if !ix.def[xi] {
		return true
	} else if !ix.def[xj] {
		return false
	}
	return ix.key[xi] < ix.key[xj]
}

func (ix index) Swap(i, j int) {
	i++
	j++
	ix.ix[i], ix.ix[j] = ix.ix[j], ix.ix[i]
}

func (ix index) search(key uint64) int {
	return sort.Search(ix.Len(), func(i int) bool {
		xi := ix.ix[i+1]
		return ix.def[xi] && ix.key[xi] >= key
	})
}

func (ix index) searchRun(key uint64) (i, m int) {
	i = ix.search(key)
	for j, n := i, ix.Len(); j < n; j++ {
		if xi := ix.ix[j+1]; !ix.def[xi] || ix.key[xi] != key {
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
