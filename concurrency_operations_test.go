package shardmap

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoadOrStore(t *testing.T) {
	m := NewShardMap[string, int]()

	if got, loaded := m.LoadOrStore("key", 1); loaded || got != 1 {
		t.Fatalf("first LoadOrStore = (%d, %v), want (1, false)", got, loaded)
	}
	if got, loaded := m.LoadOrStore("key", 2); !loaded || got != 1 {
		t.Fatalf("second LoadOrStore = (%d, %v), want (1, true)", got, loaded)
	}
	if got := m.Len(); got != 1 {
		t.Fatalf("Len = %d, want 1", got)
	}
}

func TestConcurrentLoadOrStoreHasSingleWinner(t *testing.T) {
	const workers = 64
	m := NewShardMap[string, int]()
	start := make(chan struct{})
	results := make(chan struct {
		value  int
		loaded bool
	}, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(value int) {
			defer wg.Done()
			<-start
			actual, loaded := m.LoadOrStore("shared", value)
			results <- struct {
				value  int
				loaded bool
			}{actual, loaded}
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)

	winner, ok := m.Get("shared")
	if !ok {
		t.Fatal("shared key was not stored")
	}
	winners := 0
	for result := range results {
		if result.value != winner {
			t.Fatalf("LoadOrStore returned %d, stored winner is %d", result.value, winner)
		}
		if !result.loaded {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("non-loaded result count = %d, want 1", winners)
	}
	if got := m.Len(); got != 1 {
		t.Fatalf("Len = %d, want 1", got)
	}
}

func TestConcurrentLoadOrComputeStoresOneResult(t *testing.T) {
	const workers = 32
	m := NewShardMap[string, int]()
	start := make(chan struct{})
	releaseCallbacks := make(chan struct{})
	callbackEntered := make(chan struct{}, workers)
	results := make(chan struct {
		value  int
		loaded bool
	}, workers)

	var calls atomic.Int64
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(value int) {
			defer wg.Done()
			<-start
			actual, loaded := m.LoadOrCompute("shared", func() int {
				calls.Add(1)
				callbackEntered <- struct{}{}
				<-releaseCallbacks
				return value
			})
			results <- struct {
				value  int
				loaded bool
			}{actual, loaded}
		}(i)
	}
	close(start)
	for i := 0; i < workers; i++ {
		select {
		case <-callbackEntered:
		case <-time.After(2 * time.Second):
			t.Fatalf("only %d/%d callbacks entered", i, workers)
		}
	}
	close(releaseCallbacks)
	wg.Wait()
	close(results)

	stored, ok := m.Get("shared")
	if !ok {
		t.Fatal("shared key was not stored")
	}
	winners := 0
	for result := range results {
		if result.value != stored {
			t.Fatalf("LoadOrCompute returned %d, stored value is %d", result.value, stored)
		}
		if !result.loaded {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("non-loaded result count = %d, want 1", winners)
	}
	if got := calls.Load(); got != workers {
		t.Fatalf("callback calls = %d, want %d", got, workers)
	}
}

func TestConcurrentComputeIsAtomic(t *testing.T) {
	const (
		workers    = 32
		increments = 1000
	)
	m := NewShardMap[string, int]()
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < increments; j++ {
				m.Compute("counter", func(old int, loaded bool) int {
					return old + 1
				})
			}
		}()
	}
	wg.Wait()

	if got, ok := m.Get("counter"); !ok || got != workers*increments {
		t.Fatalf("counter = %d, ok=%v, want %d", got, ok, workers*increments)
	}
	if got := m.Len(); got != 1 {
		t.Fatalf("Len = %d, want 1", got)
	}
}

func TestSwap(t *testing.T) {
	m := NewShardMap[string, int]()
	if old, loaded := m.Swap("key", 1); loaded || old != 0 {
		t.Fatalf("first Swap = (%d, %v), want (0, false)", old, loaded)
	}
	if old, loaded := m.Swap("key", 2); !loaded || old != 1 {
		t.Fatalf("second Swap = (%d, %v), want (1, true)", old, loaded)
	}
	if got := m.Len(); got != 1 {
		t.Fatalf("Len = %d, want 1", got)
	}
}

func TestRangeStopsEarly(t *testing.T) {
	m := NewShardMap[int, int]()
	for i := 0; i < 100; i++ {
		m.Set(i, i)
	}

	visited := 0
	m.Range(func(key, value int) bool {
		visited++
		return false
	})
	if visited != 1 {
		t.Fatalf("Range callback count = %d, want 1", visited)
	}
}

func TestRangeCallbackDoesNotBlockShardWrites(t *testing.T) {
	m := NewShardMap[string, int]()
	m.Set("key", 1)
	callbackEntered := make(chan struct{})
	releaseCallback := make(chan struct{})
	rangeDone := make(chan struct{})
	go func() {
		defer close(rangeDone)
		m.Range(func(key string, value int) bool {
			close(callbackEntered)
			<-releaseCallback
			return true
		})
	}()

	select {
	case <-callbackEntered:
	case <-time.After(time.Second):
		t.Fatal("Range callback did not start")
	}

	writeDone := make(chan struct{})
	go func() {
		m.Set("key", 2)
		close(writeDone)
	}()
	select {
	case <-writeDone:
	case <-time.After(time.Second):
		t.Fatal("Set was blocked by a Range callback")
	}
	close(releaseCallback)
	<-rangeDone
}
