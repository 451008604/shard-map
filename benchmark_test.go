package shardmap

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

func fillShard(n int) *ShardMap[int, int] {
	m := NewShardMap[int, int]()
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

func BenchmarkShardMap_Set(b *testing.B) {
	m := NewShardMap[int, int]()
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

func BenchmarkShardMap_Get(b *testing.B) {
	m := fillShard(10000)
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

func BenchmarkShardMap_Delete(b *testing.B) {
	m := fillShard(b.N)
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

// benchConcurrent 使用固定数量的 worker 平分 b.N 次操作。
// worker 和 iteration 可用于生成无需共享原子计数器的确定性键序列。
func benchConcurrent(b *testing.B, workers int, op func(worker, iteration int)) {
	b.ResetTimer()
	var wg sync.WaitGroup
	wg.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func(worker int) {
			defer wg.Done()
			for iteration := worker; iteration < b.N; iteration += workers {
				op(worker, iteration)
			}
		}(worker)
	}
	wg.Wait()
}

// ─── 并发写入梯度 ───

func BenchmarkConcurrentWrite_ShardMap(b *testing.B) {
	for _, g := range goroutineCounts {
		b.Run(fmt.Sprintf("g=%d", g), func(b *testing.B) {
			m := NewShardMap[int, int]()
			benchConcurrent(b, g, func(worker, iteration int) {
				m.Set(iteration%10000, iteration)
			})
		})
	}
}

func BenchmarkConcurrentWrite_MutexMap(b *testing.B) {
	for _, g := range goroutineCounts {
		b.Run(fmt.Sprintf("g=%d", g), func(b *testing.B) {
			m := newMutexMap[int, int]()
			benchConcurrent(b, g, func(worker, iteration int) {
				m.Set(iteration%10000, iteration)
			})
		})
	}
}

// ─── 并发读取梯度 ───

func BenchmarkConcurrentRead_ShardMap(b *testing.B) {
	for _, g := range goroutineCounts {
		b.Run(fmt.Sprintf("g=%d", g), func(b *testing.B) {
			m := fillShard(10000)
			benchConcurrent(b, g, func(worker, iteration int) {
				m.Get(iteration % 10000)
			})
		})
	}
}

func BenchmarkConcurrentRead_MutexMap(b *testing.B) {
	for _, g := range goroutineCounts {
		b.Run(fmt.Sprintf("g=%d", g), func(b *testing.B) {
			m := fillMutex(10000)
			benchConcurrent(b, g, func(worker, iteration int) {
				m.Get(iteration % 10000)
			})
		})
	}
}

// ─── 不同读写比例基准（固定高并发 256 goroutines）───

func BenchmarkMixedRatio_ShardMap(b *testing.B) {
	for _, writePct := range []int{0, 10, 50, 100} {
		b.Run(fmt.Sprintf("write=%d%%", writePct), func(b *testing.B) {
			m := fillShard(10000)
			benchConcurrent(b, 256, func(worker, iteration int) {
				if iteration%100 < writePct {
					m.Set(iteration%10000, iteration)
				} else {
					m.Get(iteration % 10000)
				}
			})
		})
	}
}

func BenchmarkMixedRatio_MutexMap(b *testing.B) {
	for _, writePct := range []int{0, 10, 50, 100} {
		b.Run(fmt.Sprintf("write=%d%%", writePct), func(b *testing.B) {
			m := fillMutex(10000)
			benchConcurrent(b, 256, func(worker, iteration int) {
				if iteration%100 < writePct {
					m.Set(iteration%10000, iteration)
				} else {
					m.Get(iteration % 10000)
				}
			})
		})
	}
}

// ─── Range 基准 ───

func BenchmarkShardMap_Range(b *testing.B) {
	for _, size := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			m := fillShard(size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.Range(func(k, v int) bool { return true })
			}
		})
	}
}
