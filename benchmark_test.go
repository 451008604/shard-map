package shardedmap

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
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

// ─── 并发度梯度基准 ───
// 通过设置不同的 GOMAXPROCS 值模拟不同并发度，
// 展示随并发度增长，分片锁相比单锁的扩展性优势。

var goroutineCounts = []int{1, 4, 16, 64, 256, 1024}

// benchConcurrent 通用并发测试框架。
// op 为每个 goroutine 执行的操作，接收全局递增 ID 作为参数。
func benchConcurrent(b *testing.B, goroutines int, op func(id int)) {
	b.SetParallelism(goroutines / runtime.GOMAXPROCS(0))
	if goroutines/runtime.GOMAXPROCS(0) < 1 {
		b.SetParallelism(1)
	}
	b.ResetTimer()
	var counter atomic.Int64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			id := int(counter.Add(1))
			op(id)
		}
	})
}

// ─── 并发写入梯度 ───

func BenchmarkConcurrentWrite_ShardedMap(b *testing.B) {
	for _, g := range goroutineCounts {
		b.Run(fmt.Sprintf("g=%d", g), func(b *testing.B) {
			m := New[int, int]()
			benchConcurrent(b, g, func(id int) {
				m.Set(id%10000, id)
			})
		})
	}
}

func BenchmarkConcurrentWrite_MutexMap(b *testing.B) {
	for _, g := range goroutineCounts {
		b.Run(fmt.Sprintf("g=%d", g), func(b *testing.B) {
			m := newMutexMap[int, int]()
			benchConcurrent(b, g, func(id int) {
				m.Set(id%10000, id)
			})
		})
	}
}

// ─── 并发读取梯度 ───

func BenchmarkConcurrentRead_ShardedMap(b *testing.B) {
	for _, g := range goroutineCounts {
		b.Run(fmt.Sprintf("g=%d", g), func(b *testing.B) {
			m := fillSharded(10000)
			benchConcurrent(b, g, func(id int) {
				m.Get(id % 10000)
			})
		})
	}
}

func BenchmarkConcurrentRead_MutexMap(b *testing.B) {
	for _, g := range goroutineCounts {
		b.Run(fmt.Sprintf("g=%d", g), func(b *testing.B) {
			m := fillMutex(10000)
			benchConcurrent(b, g, func(id int) {
				m.Get(id % 10000)
			})
		})
	}
}

// ─── 不同读写比例基准（固定高并发 256 goroutines）───

func BenchmarkMixedRatio_ShardedMap(b *testing.B) {
	for _, writePct := range []int{0, 10, 50, 100} {
		b.Run(fmt.Sprintf("write=%d%%", writePct), func(b *testing.B) {
			m := fillSharded(10000)
			benchConcurrent(b, 256, func(id int) {
				if id%100 < writePct {
					m.Set(id%10000, id)
				} else {
					m.Get(id % 10000)
				}
			})
		})
	}
}

func BenchmarkMixedRatio_MutexMap(b *testing.B) {
	for _, writePct := range []int{0, 10, 50, 100} {
		b.Run(fmt.Sprintf("write=%d%%", writePct), func(b *testing.B) {
			m := fillMutex(10000)
			benchConcurrent(b, 256, func(id int) {
				if id%100 < writePct {
					m.Set(id%10000, id)
				} else {
					m.Get(id % 10000)
				}
			})
		})
	}
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
