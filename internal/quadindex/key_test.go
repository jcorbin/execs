package quadindex_test

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jcorbin/execs/internal/quadindex"
)

func TestKey(t *testing.T) {
	for _, tc := range []struct{ p, e image.Point }{
		{p: image.ZP},
		{p: image.Pt(1, 0)},
		{p: image.Pt(0, 1)},
		{p: image.Pt(-1, 0)},
		{p: image.Pt(0, -1)},
		{p: image.Pt(1, 1)},
		{p: image.Pt(-1, 1)},
		{p: image.Pt(1, -1)},
		{p: image.Pt(-1, -1)},

		// positive limit
		{p: image.Pt(0x1ffffffe, 0x3ffffffe)},
		{p: image.Pt(0x3fffffff, 0x3fffffff)},
		{p: image.Pt(0x40000000, 0x40000000), e: image.Pt(0x3fffffff, 0x3fffffff)},

		// negative limit
		{p: image.Pt(-0x3ffffffe, -0x3ffffffe)},
		{p: image.Pt(-0x3fffffff, -0x3fffffff)},
		{p: image.Pt(-0x40000000, -0x40000000), e: image.Pt(-0x3fffffff, -0x3fffffff)},
	} {
		t.Run(tc.p.String(), func(t *testing.T) {
			k := quadindex.MakeKey(tc.p)
			assert.True(t, k.Set())
			if tc.e != image.ZP {
				assert.Equal(t, tc.e, k.Pt())
			} else {
				assert.Equal(t, tc.p, k.Pt())
			}
			if t.Failed() {
				t.Logf("%016x", uint64(k))
			}
		})
	}
}

func TestKey_NextWithin(t *testing.T) {
	r := image.Rect(2, 2, 8, 8)
	k := quadindex.MakeKey(r.Min)

	lim := r.Dx() * r.Dy() * 4
	for i := 0; i < lim && k < quadindex.MakeKey(r.Max); i++ {
		nk, within := k.NextWithin(r)
		if within {
			t.Logf("[%v] %v", i, k)
			k++
		} else {
			t.Logf("[%v] SKIP %v => %v (%v)", i, k, nk, int(nk-k))
			if nk < k {
				t.FailNow()
			}
			k = nk
		}
	}

	t.Fail()
}
