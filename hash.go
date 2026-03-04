package shardmap

import (
	"fmt"
	"hash/fnv"
	"math"
)

const (
	fnvOffsetBasis32 = uint32(2166136261)
	fnvPrime32       = uint32(16777619)
)

// fnvHash 通过类型分发对常见类型进行零分配 FNV-1a 哈希计算。
// 仅对未匹配的类型（struct、type alias 等）回退到 fmt.Fprintf。
func fnvHash[K comparable](key K) uint32 {
	switch k := any(key).(type) {
	case string:
		return fnvHashString(k)
	case int:
		return fnvHashUint64(uint64(k))
	case int8:
		return fnvHashByte(byte(k))
	case int16:
		return fnvHashUint16(uint16(k))
	case int32:
		return fnvHashUint32(uint32(k))
	case int64:
		return fnvHashUint64(uint64(k))
	case uint:
		return fnvHashUint64(uint64(k))
	case uint8:
		return fnvHashByte(k)
	case uint16:
		return fnvHashUint16(k)
	case uint32:
		return fnvHashUint32(k)
	case uint64:
		return fnvHashUint64(k)
	case uintptr:
		return fnvHashUint64(uint64(k))
	case float32:
		return fnvHashUint32(math.Float32bits(k))
	case float64:
		return fnvHashUint64(math.Float64bits(k))
	default:
		h := fnv.New32a()
		_, _ = fmt.Fprintf(h, "%v", key)
		return h.Sum32()
	}
}

func fnvHashString(s string) uint32 {
	h := fnvOffsetBasis32
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= fnvPrime32
	}
	return h
}

func fnvHashByte(b byte) uint32 {
	h := fnvOffsetBasis32
	h ^= uint32(b)
	h *= fnvPrime32
	return h
}

func fnvHashUint16(v uint16) uint32 {
	h := fnvOffsetBasis32
	h ^= uint32(byte(v))
	h *= fnvPrime32
	h ^= uint32(byte(v >> 8))
	h *= fnvPrime32
	return h
}

func fnvHashUint32(v uint32) uint32 {
	h := fnvOffsetBasis32
	h ^= v & 0xFF
	h *= fnvPrime32
	h ^= (v >> 8) & 0xFF
	h *= fnvPrime32
	h ^= (v >> 16) & 0xFF
	h *= fnvPrime32
	h ^= (v >> 24) & 0xFF
	h *= fnvPrime32
	return h
}

func fnvHashUint64(v uint64) uint32 {
	h := fnvOffsetBasis32
	h ^= uint32(v & 0xFF)
	h *= fnvPrime32
	h ^= uint32((v >> 8) & 0xFF)
	h *= fnvPrime32
	h ^= uint32((v >> 16) & 0xFF)
	h *= fnvPrime32
	h ^= uint32((v >> 24) & 0xFF)
	h *= fnvPrime32
	h ^= uint32((v >> 32) & 0xFF)
	h *= fnvPrime32
	h ^= uint32((v >> 40) & 0xFF)
	h *= fnvPrime32
	h ^= uint32((v >> 48) & 0xFF)
	h *= fnvPrime32
	h ^= uint32((v >> 56) & 0xFF)
	h *= fnvPrime32
	return h
}
