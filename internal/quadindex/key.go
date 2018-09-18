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
	var x, y uint32
	for i := uint(0); i < keyCompBits; i++ {
		x |= uint32((k & (1 << (2 * i))) >> i)
		y |= uint32((k & (1 << (2*i + 1))) >> (i + 1))
	}
	p.X = int(x) + minInt
	p.Y = int(y) + minInt
	return p
}

// MakeKey encodes an image point, truncating it if necessary, returning its
// Corresponding key value.
func MakeKey(p image.Point) Key {
	x, y := truncQuadComponent(p.X), truncQuadComponent(p.Y)
	var z uint64
	for i := uint(0); i < keyCompBits; i++ {
		z |= uint64(x&(1<<i)) << i
		z |= uint64(y&(1<<i)) << (i + 1)
	}
	return Key(z) | keySet
}

func truncQuadComponent(n int) uint32 {
	if n < minInt {
		n = minInt
	}
	if n > maxInt {
		n = maxInt
	}
	return uint32(n - minInt)
}
