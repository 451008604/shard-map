# shard-map

一个基于 Go 泛型的高性能并发安全分片 Map 实现。

## 特性

- **泛型支持** — 键类型为任意 `comparable`，值类型为任意 `any`
- **分片降低锁竞争** — 内部将数据分散到 32 个分片，每个分片拥有独立的 `sync.RWMutex`，大幅降低并发场景下的锁争用
- **高性能哈希** — 使用 wyhash 风格的乘法混合函数，整数键哈希比 FNV-1a 快 10 倍，字符串键快 3 倍
- **读写分离** — `Get` 使用读锁，`Set`/`Delete` 使用写锁，支持高并发读取
- **原子操作** — 提供 `LoadOrStore`、`LoadOrCompute`、`Compute`、`Swap` 等原子复合操作
- **无锁长度查询** — `Len()` 使用原子计数器，无需获取任何锁
- **零分配遍历** — `Range` 使用 `sync.Pool` 复用 entry slice，消除内存分配
- **缓存行对齐** — 每个分片占据独立的 64 字节缓存行，避免伪共享

## 安装

```bash
go get github.com/451008604/shard-map
```

## 快速开始

```go
package main

import (
    "fmt"
    "github.com/451008604/shard-map"
)

func main() {
    m := shardmap.NewShardMap[string, int]()

    // 写入
    m.Set("foo", 42)
    m.Set("bar", 100)

    // 读取
    if v, ok := m.Get("foo"); ok {
        fmt.Println("foo =", v) // foo = 42
    }

    // 删除
    m.Delete("bar")

    // 长度（无锁）
    fmt.Println("len =", m.Len()) // len = 1

    // 遍历
    m.Range(func(key string, value int) bool {
        fmt.Printf("%s -> %d\n", key, value)
        return true // 返回 false 可提前终止遍历
    })

    // 原子操作
    actual, loaded := m.LoadOrStore("key", 100)
    fmt.Printf("actual=%d, loaded=%v\n", actual, loaded)
}
```

## API

### 基本操作

| 方法 | 说明 |
|------|------|
| `NewShardMap[K, V]()` | 创建一个空的 `ShardMap` 实例 |
| `Set(key, value)` | 写入或更新键值对 |
| `Get(key)` | 读取键对应的值，返回 `(value, ok)` |
| `Delete(key)` | 删除指定键 |
| `Len()` | 返回所有分片中键的总数（无锁，使用原子计数器） |
| `Range(fn)` | 遍历所有键值对，回调返回 `false` 时提前终止 |

### 原子操作

| 方法 | 说明 |
|------|------|
| `LoadOrStore(key, value)` | 原子地获取或存储。键存在返回 `(existing, true)`，否则存储并返回 `(new, false)` |
| `LoadOrCompute(key, fn)` | 原子地获取或计算。键存在返回 `(existing, true)`，否则调用 `fn()` 计算并存储 |
| `Compute(key, fn)` | 原子地读-修改-写。`fn` 接收 `(old, loaded)` 返回新值 |
| `Swap(key, value)` | 原子地替换值，返回 `(previous, loaded)` |

#### 使用示例

```go
// 缓存模式：仅在键不存在时计算
actual, loaded := m.LoadOrCompute("expensive-key", func() int {
    return expensiveComputation()
})

// 计数器模式：原子递增
m.Compute("counter", func(old int, loaded bool) int {
    if loaded {
        return old + 1
    }
    return 1
})

// 条件更新
previous, loaded := m.Swap("state", newState)
```

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

测试环境：Apple M1 Pro (10 核), Go 1.21, darwin/arm64

```bash
go test -bench=. -benchmem ./...
```

### 单线程性能

ShardMap 与单锁 `sync.RWMutex` Map 对比：

| 操作 | ShardMap | MutexMap | 说明 |
|------|-----------|----------|------|
| Set | 199 ns/op, 0 allocs | 135 ns/op, 0 allocs | 单线程下分片哈希有额外计算开销 |
| Get | 28 ns/op, 0 allocs | 20 ns/op, 0 allocs | 单线程无锁竞争，差距较小 |
| Delete | 135 ns/op, 0 allocs | 91 ns/op, 0 allocs | 同上 |

> 单线程下 ShardMap 因分片哈希有额外开销，但已实现零堆分配。分片锁的设计目标不是优化单线程，而是并发扩展性。

### 并发写入 — 随并发度扩展（goroutine 梯度）

| goroutines | ShardMap | MutexMap | 倍率 |
|------------|-----------|----------|------|
| 1 | 89 ns/op | 246 ns/op | **2.8x** |
| 4 | 88 ns/op | 248 ns/op | **2.8x** |
| 16 | 88 ns/op | 245 ns/op | **2.8x** |
| 64 | 84 ns/op | 273 ns/op | **3.3x** |
| 256 | 84 ns/op | 278 ns/op | **3.3x** |
| 1024 | 84 ns/op | 284 ns/op | **3.4x** |

> ShardMap 写入延迟在高并发下保持稳定（~84 ns），而 MutexMap 从 246 上升到 284 ns。**并发度越高，分片锁优势越明显**。

### 并发读取 — 随并发度扩展

| goroutines | ShardMap | MutexMap | 倍率 |
|------------|-----------|----------|------|
| 1 | 76 ns/op | 146 ns/op | **1.9x** |
| 4 | 75 ns/op | 145 ns/op | **1.9x** |
| 16 | 76 ns/op | 146 ns/op | **1.9x** |
| 64 | 76 ns/op | 146 ns/op | **1.9x** |
| 256 | 75 ns/op | 146 ns/op | **1.9x** |
| 1024 | 76 ns/op | 147 ns/op | **1.9x** |

> 读取场景下 ShardMap 稳定保持 1.9 倍优势，通过减少 CPU 缓存行争用实现。

### 不同读写比例（固定 256 goroutines 高并发）

| 写入占比 | ShardMap | MutexMap | 倍率 |
|----------|-----------|----------|------|
| 0%（纯读） | 80 ns/op | 149 ns/op | **1.9x** |
| 10% | 80 ns/op | 75 ns/op | 0.9x |
| 50% | 86 ns/op | 177 ns/op | **2.1x** |
| 100%（纯写） | 85 ns/op | 276 ns/op | **3.2x** |

> **写入占比越高，分片锁优势越大**。纯写场景下 ShardMap 达到 3.2 倍吞吐量。

### Range 遍历

| 数据规模 | 耗时 | 内存分配 |
|----------|------|----------|
| 100 条 | 1,967 ns/op | 0 B/op, 0 allocs |
| 1,000 条 | 11,474 ns/op | 0 B/op, 0 allocs |
| 10,000 条 | 153,435 ns/op | 262,288 B/op, 32 allocs |

> Range 使用 `sync.Pool` 复用 entry slice。小规模遍历实现零分配，大规模遍历仅在 slice 扩容时分配。

### 适用场景

- **推荐使用**：高并发读写场景，尤其是写入密集型（缓存、计数器、会话存储）
- **推荐使用**：需要原子复合操作的场景（`LoadOrStore`、`Compute`）
- **无需使用**：单线程或极低并发场景，标准 `sync.RWMutex` 即可

## 优化历史

### v2.0 — 性能优化

1. **哈希函数重写**
   - 整数键：wyhash 风格乘法混合（10x 提速）
   - 字符串键：wyhash（3x 提速）
   - 其他类型：unsafe 原始字节哈希（50x 提速，消除 `fmt.Fprintf` 分配）

2. **原子操作**
   - 新增 `LoadOrStore`、`LoadOrCompute`、`Compute`、`Swap`
   - 使用先读后写模式避免不必要的写锁竞争

3. **无锁 `Len()`**
   - 每个分片维护 `atomic.Int64` 计数器
   - `Len()` 从 32 次锁获取变为 32 次原子加载

4. **零分配 `Range`**
   - 使用 `sync.Pool` 复用 entry slice
   - 消除每次遍历的 32 次内存分配

5. **缓存行对齐**
   - 每个分片结构体填充至 64 字节
   - 避免高并发下的伪共享

## 许可证

MIT
