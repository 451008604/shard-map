// large_concurrency_test.go
package shardmap

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestLargeConcurrentReadWriteAndRange(t *testing.T) {
	const (
		numWriters    = 20   // 写入协程数量
		numReaders    = 20   // 读取协程数量
		numRangeRuns  = 10   // 同时遍历的次数
		keysPerWriter = 5000 // 每个写入协程插入的键数量
	)

	totalKeys := numWriters * keysPerWriter
	m := NewShardMap[int, int]()

	var wg sync.WaitGroup

	// ---------- 写入 ----------
	wg.Add(numWriters)
	for w := range numWriters {
		go func(writerID int) {
			defer wg.Done()
			base := writerID * keysPerWriter
			for i := range keysPerWriter {
				k := base + i
				m.Set(k, k*2)
			}
		}(w)
	}

	// ---------- 读取 ----------
	wg.Add(numReaders)
	for range numReaders {
		go func() {
			defer wg.Done()
			// 顺序遍历所有键，覆盖全部键
			for i := range totalKeys {
				_, _ = m.Get(i)
			}
		}()
	}

	// ---------- 遍历 ----------
	var visited int64
	wg.Add(numRangeRuns)
	for range numRangeRuns {
		go func() {
			defer wg.Done()
			m.Range(func(k, v int) bool {
				atomic.AddInt64(&visited, 1)
				if v != k*2 {
					t.Fatalf("unexpected value for key %d: got %d, want %d", k, v, k*2)
				}
				return true
			})
		}()
	}

	wg.Wait()

	if got := m.Len(); got != totalKeys {
		t.Fatalf("length mismatch: want %d, got %d", totalKeys, got)
	}
	// 在高并发环境下，Range 可能在所有写入完成前就已经遍历，
	// 因此实际遍历到的键数可能小于 totalKeys * numRangeRuns。
	// 为了保证测试的有效性，只需要确认遍历确实执行并访问了若干键。
	if visited == 0 {
		t.Fatalf("Range did not visit any keys")
	}
}
