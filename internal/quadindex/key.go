package quadindex

import (
	"fmt"
	"image"
)

// Key is a quadtree key value.
type Key uint64

const (
	keySet Key = 1 << (64 - iota - 1)
	keyInval

	keyBits     = 64 - iota
	keyCompBits = keyBits / 2

	keyMask     = 1<<(keyBits+1) - 1
	keyCompMask = 1<<(keyCompBits+1) - 1

	minInt = -1<<(keyCompBits-1) + 1
	maxInt = 1<<(keyCompBits-1) - 1
)

func (k Key) String() string {
	if !k.Set() {
		return "Key(unset)"
	}
	if k.invalid() {
		return fmt.Sprintf("Key%v*", k.Pt())
	}
	return fmt.Sprintf("Key%v", k.Pt())
}

func (k Key) invalid() bool {
	return k&keyInval != 0
}

// Set returns true only if the key value is marked as "set" or defined; the
// point of an unset key value is meaningless; in practice you should only see
// zero key values that are unset.
func (k Key) Set() bool {
	return k&keySet != 0
}

// Pt reconstructs the (maybe truncated) image point encoded by the key value.
// Returns image.ZP if the key value is not "set".
func (k Key) Pt() (p image.Point) {
	if k&keySet == 0 {
		return image.ZP
	}
	x, y := combine(uint64(k&keyMask)), combine(uint64(k&keyMask)>>1)
	p.X = int(x)
	p.Y = int(y)
	return p
}

// NextWithin returns k and true if k is inside the given rectangle, the next
// smallest Key value that is and false otherwise.
func (k Key) NextWithin(r image.Rectangle) (Key, bool) {
	within := k.Pt().In(r)
	if !within {
		bigmin, _ := zdivide(
			uint64(k&keyMask),
			zkey(truncQuadComponent(r.Min.X), truncQuadComponent(r.Min.Y)),
			zkey(truncQuadComponent(r.Max.X-1), truncQuadComponent(r.Max.Y-1)),
		)
		k = Key(bigmin) | keySet
	}
	return k, within
}

// https://web.archive.org/web/20170422120027/http://docs.raima.com/rdme/9_1/Content/GS/POIexample.htm

// zdivide implements the algorithm defined in the Tropf and Herzog paper to
// find the minimum z-index (bigmin) in range greater than and the maximum
// z-index (litmax) in range smaller than a given point outside the range.
// NOTE The rangue values, rmin and rmax, are both inclusive.
func zdivide(p, rmin, rmax uint64) (bigmin, litmax uint64) {
	zmin, zmax := rmin, rmax
	for i := uint(keyBits); i > 0; i-- {

		bits, dim := i/2, i%2

		v := ((p & (1 << i)) >> (i - 2)) |
			((zmin & (1 << i)) >> (i - 1)) |
			((zmax & (1 << i)) >> i)

		switch v {
		case 0, 7: // (0, 0, 0), (1, 1, 1)
			continue
		case 3: // (0, 1, 1)
			return zmin, litmax
		case 4: // (1, 0, 0)
			return bigmin, zmax
		case 2, 6: // (0, 1, 0), (1, 1, 0)
			panic(fmt.Sprintf("inconceivable: min <= max v:%v @%v", v, i))
		}

		mask := ^(split(keyCompMask>>(keyCompBits-bits-1)) << dim)
		nmin := zmin&mask | (split(1<<bits) << dim)
		nmax := zmax&mask | (split((1<<bits)-1) << dim)
		if v == 1 { // (0, 0, 1)
			bigmin, zmax = nmin, nmax
		} else { // v == 5 (1, 0, 1)
			zmin, litmax = nmin, nmax
		}

	}
	return bigmin, litmax
}

// MakeKey encodes an image point, truncating it if necessary, returning its
// Corresponding key value.
func MakeKey(p image.Point) Key {
	z := zkey(truncQuadComponent(p.X), truncQuadComponent(p.Y))
	return Key(z) | keySet
}

func zkey(x, y uint32) (z uint64) {
	return split(x) | split(y)<<1
}

func split(value uint32) (z uint64) {
	z = uint64(value & keyCompMask)
	z = (z ^ (z << 32)) & 0x000000007fffffff
	z = (z ^ (z << 16)) & 0x0000ffff0000ffff
	z = (z ^ (z << 8)) & 0x00ff00ff00ff00ff // 11111111000000001111111100000000..
	z = (z ^ (z << 4)) & 0x0f0f0f0f0f0f0f0f // 1111000011110000
	z = (z ^ (z << 2)) & 0x3333333333333333 // 11001100..
	z = (z ^ (z << 1)) & 0x5555555555555555 // 1010...
	return z
}

func combine(z uint64) uint32 {
	z = z & 0x5555555555555555
	z = (z ^ (z >> 1)) & 0x3333333333333333
	z = (z ^ (z >> 2)) & 0x0f0f0f0f0f0f0f0f
	z = (z ^ (z >> 4)) & 0x00ff00ff00ff00ff
	z = (z ^ (z >> 8)) & 0x0000ffff0000ffff
	z = (z ^ (z >> 16)) & 0x000000007fffffff
	return uint32(z)
}

func truncQuadComponent(n int) uint32 {
	return uint32(n)
}
