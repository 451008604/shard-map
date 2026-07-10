package shardmap

import (
	"math"
	"math/bits"
)

const (
	m1 = 0xa0761d6478bd642f
	m2 = 0xe7037ed1a0b428db
	m3 = 0x8ebc6af09c88c6e3
	m4 = 0x1d8e4e27c47d124f
)

// Key is the set of key types with a built-in, equality-compatible hash.
// Defined types, structs, pointers, and interfaces are intentionally excluded.
type Key interface {
	bool | string | int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | uintptr | float32 | float64 | complex64 | complex128
}

// hashKey returns the same hash for supported keys that compare equal in Go.
// Every supported primitive type uses a dedicated fast path.
func hashKey[K Key](key K) uint32 {
	switch k := any(key).(type) {
	case bool:
		if k {
			return mixHash(1)
		}
		return mixHash(0)
	case string:
		return wyhashString(k)
	case int:
		return mixHash(uint64(k))
	case int8:
		return mixHash(uint64(k))
	case int16:
		return mixHash(uint64(k))
	case int32:
		return mixHash(uint64(k))
	case int64:
		return mixHash(uint64(k))
	case uint:
		return mixHash(uint64(k))
	case uint8:
		return mixHash(uint64(k))
	case uint16:
		return mixHash(uint64(k))
	case uint32:
		return mixHash(uint64(k))
	case uint64:
		return mixHash(k)
	case uintptr:
		return mixHash(uint64(k))
	case float32:
		return hashFloat32(k)
	case float64:
		return hashFloat64(k)
	case complex64:
		return combineHash(hashFloat32(real(k)), hashFloat32(imag(k)))
	case complex128:
		return combineHash(hashFloat64(real(k)), hashFloat64(imag(k)))
	default:
		return 0
	}
}

func hashFloat32(f float32) uint32 {
	if f == 0 { // Go considers -0 and +0 equal.
		return mixHash(0)
	}
	return mixHash(uint64(math.Float32bits(f)))
}

func hashFloat64(f float64) uint32 {
	if f == 0 { // Go considers -0 and +0 equal.
		return mixHash(0)
	}
	return mixHash(math.Float64bits(f))
}

func combineHash(a, b uint32) uint32 {
	return mixHash(uint64(a)<<32 | uint64(b))
}

func mixHash(v uint64) uint32 {
	hi, lo := bits.Mul64(v^m1, m2)
	return uint32(hi ^ lo)
}

// wyhashString is a bounds-safe wyhash-style string hash. Every 64-bit load
// is guarded by len(s) >= 8; the final partial word is assembled byte by byte.
func wyhashString(s string) uint32 {
	n := len(s)
	h := mix(uint64(n)^m1, m2)
	for len(s) >= 8 {
		h = mix(h^readU64String(s)^m3, m4)
		s = s[8:]
	}
	if len(s) > 0 {
		var tail uint64
		for i := range s {
			tail |= uint64(s[i]) << (8 * i)
		}
		h = mix(h^tail^m2, m3)
	}
	return uint32(mix(h^uint64(n), m1))
}

func readU64String(s string) uint64 {
	return uint64(s[0]) |
		uint64(s[1])<<8 |
		uint64(s[2])<<16 |
		uint64(s[3])<<24 |
		uint64(s[4])<<32 |
		uint64(s[5])<<40 |
		uint64(s[6])<<48 |
		uint64(s[7])<<56
}

func mix(a, b uint64) uint64 {
	hi, lo := bits.Mul64(a, b)
	return hi ^ lo
}
