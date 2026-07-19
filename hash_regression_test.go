package shardmap

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
)

// equalStringWithTail returns a string whose logical contents are content but
// whose backing allocation continues with tail. Hashing must depend only on
// the logical string, not bytes following its declared length.
func equalStringWithTail(content, tail string) string {
	backing := content + tail
	return backing[:len(content)]
}

func TestSignedZeroKeysUseSameHash(t *testing.T) {
	if hashKey(float64(0)) != hashKey(math.Copysign(0, -1)) {
		t.Fatal("+0 and -0 must use the same float64 hash")
	}
	if hashKey(complex(0, 0)) != hashKey(complex(math.Copysign(0, -1), 0)) {
		t.Fatal("complex values equal under Go comparison must use the same hash")
	}
}

func TestLoadOrComputeCallbackRunsWithoutShardLock(t *testing.T) {
	m := NewShardMap[string, int]()
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.LoadOrCompute("key", func() int {
			m.Set("key", 2)
			return 1
		})
	}()

	select {
	case <-done:
		if got, ok := m.Get("key"); !ok || got != 2 {
			t.Fatalf("unexpected value after reentrant callback: got %d, ok=%v", got, ok)
		}
	case <-time.After(time.Second):
		t.Fatal("LoadOrCompute callback deadlocked")
	}
}

func TestZeroValueShardMap(t *testing.T) {
	var m ShardMap[string, int]
	m.Set("key", 1)
	if got, ok := m.Get("key"); !ok || got != 1 {
		t.Fatalf("zero-value map lookup failed: got %d, ok=%v", got, ok)
	}
}

func TestShardMapEqualStringsUseSameShard(t *testing.T) {
	// Lengths 17-31 and 33-47 exercise the wyhashString reads that cross the
	// declared string boundary. 32 and 48 are the adjacent safe boundaries.
	for _, length := range []int{17, 23, 24, 31, 33, 39, 40, 47} {
		t.Run("length="+strconv.Itoa(length), func(t *testing.T) {
			content := strings.Repeat("k", length)
			stored := equalStringWithTail(content, strings.Repeat("A", 16))
			lookup := equalStringWithTail(content, strings.Repeat("B", 16))
			if stored != lookup {
				t.Fatal("test setup produced unequal strings")
			}

			m := NewShardMap[string, int]()
			m.Set(stored, 42)
			if got, ok := m.Get(lookup); !ok || got != 42 {
				t.Fatalf("equal key lookup failed: got %d, ok=%v", got, ok)
			}
		})
	}
}

func FuzzShardMapEqualStringsUseSameShard(f *testing.F) {
	// This seed is safe (length 32 with identical tails). Mutations must retain
	// the equal-key property while exercising the boundary ranges.
	f.Add("key", "AAAAAAAAAAAAAAAA", "AAAAAAAAAAAAAAAA", byte(15))

	f.Fuzz(func(t *testing.T, seed, leftTail, rightTail string, length byte) {
		n := 17 + int(length%31) // Exercise the range 17-47.
		content := repeatedToLength(seed, n)
		stored := equalStringWithTail(content, padTail(leftTail, "A"))
		lookup := equalStringWithTail(content, padTail(rightTail, "B"))
		if stored != lookup {
			t.Fatal("test setup produced unequal strings")
		}

		m := NewShardMap[string, int]()
		m.Set(stored, 1)
		if _, ok := m.Get(lookup); !ok {
			t.Fatalf("equal key was routed to a different shard (length=%d)", n)
		}
	})
}

func repeatedToLength(seed string, n int) string {
	if seed == "" {
		seed = "x"
	}
	return strings.Repeat(seed, n/len(seed)+1)[:n]
}

func padTail(tail, fallback string) string {
	if len(tail) >= 16 {
		return tail
	}
	return tail + strings.Repeat(fallback, 16-len(tail))
}
