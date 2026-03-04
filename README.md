# sharded-map

一个基于 Go 泛型的高性能并发安全分片 Map 实现。

## 特性

- **泛型支持** — 键类型为任意 `comparable`，值类型为任意 `any`
- **分片降低锁竞争** — 内部将数据分散到 32 个分片，每个分片拥有独立的 `sync.RWMutex`，大幅降低并发场景下的锁争用
- **FNV-1a 哈希分布** — 使用 FNV-1a 32 位哈希将键均匀映射到各分片
- **读写分离** — `Get` 使用读锁，`Set`/`Delete` 使用写锁，支持高并发读取

## 安装

```bash
go get github.com/451008604/sharded-map
```

## 快速开始

```go
package main

import (
    "fmt"
    sharded_map "github.com/451008604/sharded-map"
)

func main() {
    m := sharded_map.NewShardedMap[string, int]()

    // 写入
    m.Set("foo", 42)
    m.Set("bar", 100)

    // 读取
    if v, ok := m.Get("foo"); ok {
        fmt.Println("foo =", v) // foo = 42
    }

    // 删除
    m.Delete("bar")

    // 长度
    fmt.Println("len =", m.Len()) // len = 1

    // 遍历
    m.Range(func(key string, value int) bool {
        fmt.Printf("%s -> %d\n", key, value)
        return true // 返回 false 可提前终止遍历
    })
}
```

## API

| 方法 | 说明 |
|------|------|
| `NewShardedMap[K, V]()` | 创建一个空的 `ShardedMap` 实例 |
| `Set(key, value)` | 写入或更新键值对 |
| `Get(key)` | 读取键对应的值，返回 `(value, ok)` |
| `Delete(key)` | 删除指定键 |
| `Len()` | 返回所有分片中键的总数 |
| `Range(fn)` | 遍历所有键值对，回调返回 `false` 时提前终止 |

## 支持的键类型

任何满足 `comparable` 约束的类型均可作为键，包括但不限于：

- `string`、`int`、`int64`、`uint`、`float64`
- 自定义类型别名（如 `type MyString string`）
- 可比较的结构体

## 运行测试

```bash
go test -v -race ./...
```

## 基准测试

测试环境：Intel Core i7-10700K @ 3.80GHz, 16 线程, Go 1.22, linux/amd64

```bash
go test -bench=. -benchmem ./...
```

### 单线程性能

ShardedMap 与单锁 `sync.RWMutex` Map 对比：

| 操作 | ShardedMap | MutexMap | 说明 |
|------|-----------|----------|------|
| Set | 372 ns/op, 0 allocs | 269 ns/op, 0 allocs | 单线程下分片哈希有额外计算开销 |
| Get | 38 ns/op, 0 allocs | 29 ns/op, 0 allocs | 单线程无锁竞争，差距较小 |
| Delete | 287 ns/op, 0 allocs | 223 ns/op, 0 allocs | 同上 |

> 单线程下 ShardedMap 因 FNV-1a 分片哈希有额外开销，但已实现零堆分配。分片锁的设计目标不是优化单线程，而是并发扩展性。

### 并发写入 — 随并发度扩展（goroutine 梯度）

| goroutines | ShardedMap | MutexMap | 倍率 |
|------------|-----------|----------|------|
| 1 | 50 ns/op | 197 ns/op | **3.9x** |
| 4 | 48 ns/op | 202 ns/op | **4.2x** |
| 16 | 52 ns/op | 199 ns/op | **3.8x** |
| 64 | 41 ns/op | 229 ns/op | **5.6x** |
| 256 | 36 ns/op | 226 ns/op | **6.3x** |
| 1024 | 35 ns/op | 215 ns/op | **6.1x** |

> ShardedMap 写入延迟随并发度增加反而下降（50→35 ns），而 MutexMap 从 197 上升到 215+ ns。**并发度越高，分片锁优势越明显**，在 256 goroutines 时达到 6.3 倍。

### 并发读取 — 随并发度扩展

| goroutines | ShardedMap | MutexMap | 倍率 |
|------------|-----------|----------|------|
| 1 | 23 ns/op | 42 ns/op | **1.8x** |
| 16 | 24 ns/op | 39 ns/op | **1.6x** |
| 256 | 25 ns/op | 44 ns/op | **1.8x** |
| 1024 | 26 ns/op | 44 ns/op | **1.7x** |

> 读取场景下 `RWMutex` 的读锁本身允许并发，所以单锁方案不会严重退化。ShardedMap 通过减少 CPU 缓存行争用（32 个独立锁 vs 1 个锁的 reader count 原子操作），稳定保持 1.7~1.8 倍优势。

### 不同读写比例（固定 256 goroutines 高并发）

| 写入占比 | ShardedMap | MutexMap | 倍率 |
|----------|-----------|----------|------|
| 0%（纯读） | 25 ns/op | 48 ns/op | **1.9x** |
| 10% | 44 ns/op | 54 ns/op | **1.2x** |
| 50% | 46 ns/op | 71 ns/op | **1.5x** |
| 100%（纯写） | 32 ns/op | 236 ns/op | **7.4x** |

> **写入占比越高，分片锁优势越大**。纯写场景下 ShardedMap 达到 7.4 倍吞吐量。即使在纯读场景，也有 1.9 倍优势。

### Range 遍历

| 数据规模 | 耗时 | 内存分配 |
|----------|------|----------|
| 100 条 | 3,204 ns/op | 1,600 B/op, 32 allocs |
| 1,000 条 | 20,044 ns/op | 16,384 B/op, 32 allocs |
| 10,000 条 | 173,998 ns/op | 172,032 B/op, 32 allocs |

> Range 每次遍历固定 32 次分配（每个分片 1 个 entry slice），内存随数据量线性增长。

### 适用场景

- **推荐使用**：高并发读写场景，尤其是写入密集型（缓存、计数器、会话存储）
- **无需使用**：单线程或极低并发场景，标准 `sync.RWMutex` 即可

## 许可证

MIT
