package shardmap

import (
	"math"
	"testing"
)

func TestHashKeySupportsEveryKeyType(t *testing.T) {
	assertStableHash(t, false)
	assertStableHash(t, "key")
	assertStableHash(t, int(-1))
	assertStableHash(t, int8(-2))
	assertStableHash(t, int16(-3))
	assertStableHash(t, int32(-4))
	assertStableHash(t, int64(-5))
	assertStableHash(t, uint(1))
	assertStableHash(t, uint8(2))
	assertStableHash(t, uint16(3))
	assertStableHash(t, uint32(4))
	assertStableHash(t, uint64(5))
	assertStableHash(t, uintptr(6))
	assertStableHash(t, float32(1.5))
	assertStableHash(t, float64(2.5))
	assertStableHash(t, complex64(complex(1.5, -2.5)))
	assertStableHash(t, complex(3.5, -4.5))

	if hashKey(float32(0)) != hashKey(float32(math.Copysign(0, -1))) {
		t.Fatal("+0 and -0 must use the same float32 hash")
	}
}

func assertStableHash[K Key](t *testing.T, key K) {
	t.Helper()
	if first, second := hashKey(key), hashKey(key); first != second {
		t.Fatalf("hashKey(%v) is unstable: %d != %d", key, first, second)
	}
}
