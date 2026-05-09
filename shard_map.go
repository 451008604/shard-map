package shardmap

import (
	"sync"
	"sync/atomic"
)

const (
	shardCount = 32
	shardMask  = shardCount - 1
)

// 编译期断言：shardCount 必须为 2 的幂。
var _ [0]struct{} = [shardCount & (shardCount - 1)]struct{}{}

// shard 包含一个读写锁、内部 map 和原子长度计数器。
// 使用 padding 确保每个 shard 占据独立的缓存行（64 字节），
// 避免高并发下相邻 shard 的伪共享（false sharing）。
type shard[K comparable, V any] struct {
	sync.RWMutex
	m   map[K]V
	len atomic.Int64
	_   [24]byte // padding: RWMutex(24) + map(8) + len(8) + padding(24) = 64 bytes
}

// entry 用于 Range 快照分片数据。
type entry[K comparable, V any] struct {
	key   K
	value V
}

// ShardMap 为一个拥有 32 个分片的并发安全 map。
// 每个分片拥有独立的读写锁，以降低竞争并实现高并发读写。
// 键通过 FNV-1a 哈希均匀分布到各分片。
type ShardMap[K comparable, V any] struct {
	shards [shardCount]*shard[K, V]
	pool   sync.Pool // 复用 Range 遍历时的 entry slice，减少 GC 压力
}

// NewShardMap 创建一个空的 ShardMap 实例。
func NewShardMap[K comparable, V any]() *ShardMap[K, V] {
	sm := &ShardMap[K, V]{}
	sm.pool.New = func() any {
		s := make([]entry[K, V], 0, 256)
		return &s
	}
	for i := 0; i < shardCount; i++ {
		sm.shards[i] = &shard[K, V]{
			m: make(map[K]V),
		}
	}
	return sm
}

// getShard 返回键所在的分片（0~31）。
func (sm *ShardMap[K, V]) getShard(key K) *shard[K, V] {
	return sm.shards[fnvHash(key)&shardMask]
}

// Set 将键值对写入对应分片，使用写锁保证互斥。
func (sm *ShardMap[K, V]) Set(key K, value V) {
	s := sm.getShard(key)
	s.Lock()
	if _, exists := s.m[key]; !exists {
		s.len.Add(1)
	}
	s.m[key] = value
	s.Unlock()
}

// Get 从对应分片读取键值，使用读锁以支持并发读取。
func (sm *ShardMap[K, V]) Get(key K) (V, bool) {
	s := sm.getShard(key)
	s.RLock()
	e, ok := s.m[key]
	s.RUnlock()
	return e, ok
}

// Delete 删除对应分片中的键，使用写锁。
func (sm *ShardMap[K, V]) Delete(key K) {
	s := sm.getShard(key)
	s.Lock()
	if _, exists := s.m[key]; exists {
		s.len.Add(-1)
		delete(s.m, key)
	}
	s.Unlock()
}

// LoadOrStore 原子地获取或存储键值对。
// 如果键已存在，返回现有值和 true；否则存储新值并返回新值和 false。
// 使用先读后写模式避免不必要的写锁竞争。
func (sm *ShardMap[K, V]) LoadOrStore(key K, value V) (actual V, loaded bool) {
	s := sm.getShard(key)
	s.RLock()
	if v, ok := s.m[key]; ok {
		s.RUnlock()
		return v, true
	}
	s.RUnlock()

	s.Lock()
	if v, ok := s.m[key]; ok {
		s.Unlock()
		return v, true
	}
	s.m[key] = value
	s.len.Add(1)
	s.Unlock()
	return value, false
}

// LoadOrCompute 原子地获取或计算键值对。
// 如果键已存在，返回现有值和 true；否则调用 fn 计算值，存储并返回。
// fn 可能不会被调用（如果另一个 goroutine 先插入了值）。
func (sm *ShardMap[K, V]) LoadOrCompute(key K, fn func() V) (actual V, loaded bool) {
	s := sm.getShard(key)
	s.RLock()
	if v, ok := s.m[key]; ok {
		s.RUnlock()
		return v, true
	}
	s.RUnlock()

	s.Lock()
	if v, ok := s.m[key]; ok {
		s.Unlock()
		return v, true
	}
	v := fn()
	s.m[key] = v
	s.len.Add(1)
	s.Unlock()
	return v, false
}

// Compute 原子地对键执行读-修改-写操作。
// fn 接收旧值和是否存在标志，返回新值。
// 新值总是被存储，fn 的返回值不应为零值（除非有意存储零值）。
func (sm *ShardMap[K, V]) Compute(key K, fn func(old V, loaded bool) V) V {
	s := sm.getShard(key)
	s.Lock()
	old, loaded := s.m[key]
	newVal := fn(old, loaded)
	if !loaded {
		s.len.Add(1)
	}
	s.m[key] = newVal
	s.Unlock()
	return newVal
}

// Swap 原子地替换键的值，返回旧值和是否存在的标志。
func (sm *ShardMap[K, V]) Swap(key K, value V) (previous V, loaded bool) {
	s := sm.getShard(key)
	s.Lock()
	old, ok := s.m[key]
	if !ok {
		s.len.Add(1)
	}
	s.m[key] = value
	s.Unlock()
	return old, ok
}

// Len 返回整个 ShardMap 中所有键的总数。
// 使用原子计数器，无需获取任何锁。
func (sm *ShardMap[K, V]) Len() int {
	total := int64(0)
	for _, s := range sm.shards {
		total += s.len.Load()
	}
	return int(total)
}

// Range 以并发安全的方式遍历所有键值对。
// 每个分片的数据在持有读锁期间被复制出来，回调函数在释放读锁后执行，
// 避免长耗时回调阻塞写操作或导致死锁。
// 使用 sync.Pool 复用 entry slice，减少内存分配和 GC 压力。
func (sm *ShardMap[K, V]) Range(fn func(key K, value V) bool) {
	for _, s := range sm.shards {
		s.RLock()
		entriesPtr := sm.pool.Get().(*[]entry[K, V])
		entries := (*entriesPtr)[:0]
		for k, v := range s.m {
			entries = append(entries, entry[K, V]{key: k, value: v})
		}
		s.RUnlock()
		for _, e := range entries {
			if !fn(e.key, e.value) {
				sm.pool.Put(entriesPtr)
				return
			}
		}
		sm.pool.Put(entriesPtr)
	}
}
