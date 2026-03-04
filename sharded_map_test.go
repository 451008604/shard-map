package shardedmap

import (
	"sync"
	"testing"
)

func TestSetGet(t *testing.T) {
	m := New[string, int]()
	m.Set("foo", 42)
	if v, ok := m.Get("foo"); !ok || v != 42 {
		t.Fatalf("expected value 42, got %v, ok=%v", v, ok)
	}
}

func TestDelete(t *testing.T) {
	m := New[int, bool]()
	m.Set(1, true)
	m.Delete(1)
	if _, ok := m.Get(1); ok {
		t.Fatalf("key should be deleted")
	}
}

func TestLenAndRange(t *testing.T) {
	m := New[int, string]()
	m.Set(1, "a")
	m.Set(2, "b")
	m.Set(3, "c")
	if l := m.Len(); l != 3 {
		t.Fatalf("expected length 3, got %d", l)
	}
	seen := make(map[int]bool)
	m.Range(func(k int, v string) bool {
		seen[k] = true
		return true
	})
	if len(seen) != 3 {
		t.Fatalf("expected to see 3 keys in range, got %d", len(seen))
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := New[int, int]()
	var wg sync.WaitGroup
	// launch 10 writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := id*100 + j
				m.Set(key, key*2)
			}
		}(i)
	}
	// launch 10 readers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				_, _ = m.Get(j)
			}
		}()
	}
	wg.Wait()
	// simple sanity check: length should be 1000 (10 writers * 100 keys each)
	if l := m.Len(); l != 1000 {
		t.Fatalf("expected length 1000 after concurrent writes, got %d", l)
	}
}
