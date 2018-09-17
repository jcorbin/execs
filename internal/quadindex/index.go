package quadindex

import (
	"fmt"
	"image"
	"sort"
)

// Index implements a linear quadtree.
type Index struct {
	index
}

// Get the key value stored for the given index.
func (qi *Index) Get(i int) Key {
	if i >= len(qi.ks) {
		return 0
	}
	return qi.ks[i]
}

// Update the point associated with the given index.
func (qi *Index) Update(i int, p image.Point) {
	if i >= qi.index.Len() {
		qi.alloc(i)
	}
	qi.ks[i] = MakeKey(p) | keySet
	sort.Sort(&qi.index)
}

// Delete the point associated with the given index.
func (qi *Index) Delete(i int, p image.Point) {
	qi.ks[i] = 0
	sort.Sort(&qi.index)
}

// At returns a query cursor for all stored indices at the given point.
func (qi *Index) At(p image.Point) (qq Cursor) {
	qq.index = &qi.index
	qq.kmin = MakeKey(p)
	qq.kmax = qq.kmin
	qq.iimin = qi.search(qq.kmin)
	qq.iimax = qq.iimin
	if qq.iimin < len(qi.ix) {
		qq.iimax = qi.run(qq.iimax, qq.kmin)
		qq.ii = qq.iimin - 1
	} else {
		qq.ii = len(qi.ix)
	}
	return qq
}

// Within returns a query cursor for all stored indices within the given region.
func (qi *Index) Within(r image.Rectangle) (qq Cursor) {
	qq.index = &qi.index
	qq.r = r
	qq.kmin = MakeKey(r.Min)
	qq.kmax = MakeKey(r.Max)
	qq.iimin = qi.search(qq.kmin)
	qq.iimax = qq.iimin
	if qq.iimin < len(qi.ix) {
		qq.iimax = qi.run(qi.search(qq.kmax), qq.kmax)
		qq.ii = qq.iimin - 1
	} else {
		qq.ii = len(qi.ix)
	}
	return qq
}

// Cursor is a point or region query on an Index.
type Cursor struct {
	*index
	r            image.Rectangle
	kmin, kmax   Key
	iimin, iimax int
	ii           int
}

func (qq Cursor) String() string {
	return fmt.Sprintf("quadCursor(%v := range iimin:%v iimax:%v kmin:%v kmax:%v)",
		qq.ii, qq.iimin, qq.iimax, qq.kmin, qq.kmax)
}

// I returns the cursor index, or -1 if the cursor is done (Next() returns
// false forever).
func (qq *Cursor) I() int {
	if qq.ii < qq.iimax {
		return qq.ix[qq.ii]
	}
	return -1
}

// Next advances the cursor if possible and returns true, false otherwise.
func (qq *Cursor) Next() bool {
	for qq.ii++; qq.ii < qq.iimax; qq.ii++ {
		if qq.ks[qq.ix[qq.ii]] > qq.kmax {
			qq.ii = qq.iimax + 1
			return false
		}
		if qq.r == image.ZR {
			return true
		} else if qq.ks[qq.ix[qq.ii]].Pt().In(qq.r) {
			return true
		}
	}
	return false
}

type index struct {
	ix []int
	ks []Key
}

func (qi index) Len() int             { return len(qi.ix) }
func (qi index) Less(ii, jj int) bool { return qi.ks[qi.ix[ii]] < qi.ks[qi.ix[jj]] }
func (qi index) Swap(ii, jj int)      { qi.ix[ii], qi.ix[jj] = qi.ix[jj], qi.ix[ii] }

func (qi *index) alloc(i int) {
	for i >= len(qi.ix) {
		if i < cap(qi.ix) {
			j := len(qi.ix)
			qi.ix = qi.ix[:i+1]
			for ; j <= i; j++ {
				qi.ix[j] = j
			}
		} else {
			qi.ix = append(qi.ix, len(qi.ix))
		}
	}
	qi.ix[i] = i

	for i >= len(qi.ks) {
		if i < cap(qi.ks) {
			j := len(qi.ks)
			qi.ks = qi.ks[:i+1]
			for ; j <= i; j++ {
				qi.ks[j] = 0
			}
		} else {
			qi.ks = append(qi.ks, 0)
		}
	}
	qi.ks[i] = 0
}

func (qi *index) search(k Key) int {
	return qi.narrow(0, len(qi.ix), k)
}

func (qi *index) narrow(ii, jj int, k Key) int {
	k |= keySet
	for ii < jj {
		h := int(uint(ii+jj) >> 1) // avoid overflow when computing h
		// ii ≤ h < jj
		if qi.ks[qi.ix[h]] < k {
			ii = h + 1 // preserves qi.ks[qi.ix[ii-1]] < k
		} else {
			jj = h // preserves qi.ks[qi.ix[jj]] >= k
		}
	}
	return ii
}

func (qi *index) run(ii int, k Key) int {
	for ii < len(qi.ix) && qi.ks[qi.ix[ii]] == k {
		ii++
	}
	return ii
}
