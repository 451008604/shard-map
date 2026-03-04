# shardedmap

一个基于 Go 泛型的高性能并发安全分片 Map 实现。

## 特性

- **泛型支持** — 键类型为任意 `comparable`，值类型为任意 `any`
- **分片降低锁竞争** — 内部将数据分散到 32 个分片，每个分片拥有独立的 `sync.RWMutex`，大幅降低并发场景下的锁争用
- **FNV-1a 哈希分布** — 使用 FNV-1a 32 位哈希将键均匀映射到各分片
- **读写分离** — `Get` 使用读锁，`Set`/`Delete` 使用写锁，支持高并发读取

## 安装

```bash
go get shardedmap
```

## 快速开始

```go
package main

import (
    "fmt"
    "shardedmap"
)

func main() {
    m := shardedmap.New[string, int]()

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
| `New[K, V]()` | 创建一个空的 `ShardedMap` 实例 |
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
go test -bench=. -benchmem -count=3 ./...
```

### 单线程性能

ShardedMap 与单锁 `sync.RWMutex` Map 对比：

| 操作 | ShardedMap | MutexMap | 说明 |
|------|-----------|----------|------|
| Set | 365 ns/op, 112 B/op | 235 ns/op, 61 B/op | 单线程下分片哈希有额外开销 |
| Get | 108 ns/op, 15 B/op | 27 ns/op, 0 B/op | 单线程无锁竞争，单锁更快 |
| Delete | 278 ns/op, 15 B/op | 179 ns/op, 0 B/op | 同上 |

> 单线程场景下 ShardedMap 因 FNV 哈希计算（`fmt.Fprintf`）产生额外分配，性能低于单锁方案。

### 并发性能（16 线程）

| 操作 | ShardedMap | MutexMap | 倍率 |
|------|-----------|----------|------|
| 并发 Set | **50 ns/op** | 202 ns/op | **4.0x** |
| 并发 Get | **25 ns/op** | 42 ns/op | **1.7x** |
| 并发混合读写 (80%读+20%写) | 85 ns/op | 38 ns/op | 0.4x |

> **并发写入是 ShardedMap 的核心优势**，在 16 线程并发 Set 下吞吐量达到单锁方案的 4 倍。并发读取同样有 1.7 倍提升。混合读写场景中，由于读占比高且哈希分配开销固定，单锁方案的单次操作延迟更低。

### Range 遍历

| 数据规模 | 耗时 | 内存分配 |
|----------|------|----------|
| 100 条 | 3,826 ns/op | 1,664 B/op, 64 allocs |
| 1,000 条 | 23,883 ns/op | 16,640 B/op, 64 allocs |
| 10,000 条 | 179,522 ns/op | 172,032 B/op, 64 allocs |

> Range 每次遍历固定 64 次分配（32 个分片 × 2 个 slice），内存随数据量线性增长。

### 适用场景

- **推荐使用**：高并发写入密集型场景（如缓存、计数器、会话存储）
- **无需使用**：单线程或低并发、读远大于写的场景，标准 `sync.RWMutex` 即可

## 许可证

MIT
