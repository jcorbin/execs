package ecs

import "sort"

// AddAIndex adds an index for A-side entity IDs.
func (rel *Relation) AddAIndex() {
	rel.aix = make([]int, len(rel.aids))
	for i := range rel.aids {
		rel.aix[i] = i
	}
	sortit(len(rel.aix), rel.aixLess, rel.aixSwap)
}

// AddBIndex adds an index for B-side entity IDs.
func (rel *Relation) AddBIndex() {
	rel.bix = make([]int, len(rel.bids))
	for i := range rel.bids {
		rel.bix[i] = i
	}
	sortit(len(rel.bix), rel.bixLess, rel.bixSwap)
}

func (rel *Relation) indexLookup(
	tcl TypeClause,
	qids, aids, bids []EntityID,
	aix []int,
) []EntityID {
	// TODO: tighter cardinality estimation
	rset := make(map[EntityID]struct{}, len(rel.types))
	for _, id := range qids {
		for i := sort.Search(len(aix), func(i int) bool {
			return aids[aix[i]] >= id
		}); i < len(aix) && aids[aix[i]] == id; i++ {
			if j := aix[i]; tcl.Test(rel.types[j]) {
				rset[bids[j]] = struct{}{}
			}
		}
	}
	result := make([]EntityID, 0, len(rset))
	for id := range rset {
		result = append(result, id)
	}
	return result
}

func (rel Relation) aixLess(i, j int) bool { return rel.aids[rel.aix[i]] < rel.aids[rel.aix[j]] }
func (rel Relation) bixLess(i, j int) bool { return rel.bids[rel.bix[i]] < rel.bids[rel.bix[j]] }

func (rel Relation) aixSwap(i, j int) { rel.aix[i], rel.aix[j] = rel.aix[j], rel.aix[i] }
func (rel Relation) bixSwap(i, j int) { rel.bix[i], rel.bix[j] = rel.bix[j], rel.bix[i] }

func (rel *Relation) deferIndexing() func() {
	if rel.aix == nil && rel.bix == nil {
		return nil
	}
	rel.fix = true
	return func() {
		if rel.aix != nil {
			sortit(len(rel.aix), rel.aixLess, rel.aixSwap)
		}
		if rel.bix != nil {
			sortit(len(rel.aix), rel.aixLess, rel.aixSwap)
		}
		rel.fix = false
	}
}

type tmpSort struct {
	n    int
	less func(i, j int) bool
	swap func(i, j int)
}

func (ts tmpSort) Len() int           { return ts.n }
func (ts tmpSort) Less(i, j int) bool { return ts.less(i, j) }
func (ts tmpSort) Swap(i, j int)      { ts.swap(i, j) }

func fix(
	i, n int,
	less func(i, j int) bool,
	swap func(i, j int),
) {
	// TODO: something more minimal, since we assume sorted order but for [i]
	sortit(n, less, swap)
}

func sortit(
	n int,
	less func(i, j int) bool,
	swap func(i, j int),
) {
	sort.Sort(tmpSort{n, less, swap})
}
