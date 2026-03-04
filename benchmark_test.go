package shardedmap

import (
	"fmt"
	"sync"
	"testing"
)

// ─── 对照组：单锁 map ───

type mutexMap[K comparable, V any] struct {
	sync.RWMutex
	m map[K]V
}

func newMutexMap[K comparable, V any]() *mutexMap[K, V] {
	return &mutexMap[K, V]{m: make(map[K]V)}
}

func (mm *mutexMap[K, V]) Set(key K, value V) {
	mm.Lock()
	mm.m[key] = value
	mm.Unlock()
}

func (mm *mutexMap[K, V]) Get(key K) (V, bool) {
	mm.RLock()
	v, ok := mm.m[key]
	mm.RUnlock()
	return v, ok
}

func (mm *mutexMap[K, V]) Delete(key K) {
	mm.Lock()
	delete(mm.m, key)
	mm.Unlock()
}

// ─── 辅助：预填充 ───

func fillSharded(n int) *ShardedMap[int, int] {
	m := New[int, int]()
	for i := 0; i < n; i++ {
		m.Set(i, i)
	}
	return m
}

func fillMutex(n int) *mutexMap[int, int] {
	m := newMutexMap[int, int]()
	for i := 0; i < n; i++ {
		m.Set(i, i)
	}
	return m
}

// ─── 单线程基准 ───

func BenchmarkShardedMap_Set(b *testing.B) {
	m := New[int, int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Set(i, i)
	}
}

func BenchmarkMutexMap_Set(b *testing.B) {
	m := newMutexMap[int, int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Set(i, i)
	}
}

func BenchmarkShardedMap_Get(b *testing.B) {
	m := fillSharded(10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Get(i % 10000)
	}
}

func BenchmarkMutexMap_Get(b *testing.B) {
	m := fillMutex(10000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Get(i % 10000)
	}
}

func BenchmarkShardedMap_Delete(b *testing.B) {
	m := fillSharded(b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Delete(i)
	}
}

func BenchmarkMutexMap_Delete(b *testing.B) {
	m := fillMutex(b.N)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Delete(i)
	}
}

// ─── 并发基准 ───

func BenchmarkShardedMap_ConcurrentSet(b *testing.B) {
	m := New[int, int]()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.Set(i, i)
			i++
		}
	})
}

func BenchmarkMutexMap_ConcurrentSet(b *testing.B) {
	m := newMutexMap[int, int]()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.Set(i, i)
			i++
		}
	})
}

func BenchmarkShardedMap_ConcurrentGet(b *testing.B) {
	m := fillSharded(10000)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.Get(i % 10000)
			i++
		}
	})
}

func BenchmarkMutexMap_ConcurrentGet(b *testing.B) {
	m := fillMutex(10000)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.Get(i % 10000)
			i++
		}
	})
}

func BenchmarkShardedMap_ConcurrentMixed(b *testing.B) {
	m := fillSharded(10000)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%10 < 8 {
				m.Get(i % 10000)
			} else {
				m.Set(i%10000, i)
			}
			i++
		}
	})
}

func BenchmarkMutexMap_ConcurrentMixed(b *testing.B) {
	m := fillMutex(10000)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%10 < 8 {
				m.Get(i % 10000)
			} else {
				m.Set(i%10000, i)
			}
			i++
		}
	})
}

// ─── Range 基准 ───

func BenchmarkShardedMap_Range(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			m := fillSharded(size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.Range(func(k, v int) bool { return true })
			}
		})
	}
}
