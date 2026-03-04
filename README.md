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
| Set | 263 ns/op, 0 allocs | 224 ns/op, 0 allocs | 分片哈希计算略有开销，差距很小 |
| Get | 37 ns/op, 0 allocs | 27 ns/op, 0 allocs | 单线程无锁竞争，差距较小 |
| Delete | 185 ns/op, 0 allocs | 170 ns/op, 0 allocs | 基本持平 |

> 通过内联 FNV-1a 类型分发哈希，单线程下已实现零分配，与单锁方案差距大幅缩小。

### 并发性能（16 线程）

| 操作 | ShardedMap | MutexMap | 倍率 |
|------|-----------|----------|------|
| 并发 Set | **50 ns/op** | 199 ns/op | **4.0x** |
| 并发 Get | **9 ns/op** | 35 ns/op | **3.9x** |
| 并发混合读写 (80%读+20%写) | **42 ns/op** | 37 ns/op | 1.1x |

> **并发场景全面优于单锁方案**。并发 Set 吞吐量达到单锁的 4 倍，并发 Get 提升 3.9 倍。混合读写场景下也已基本持平。

### Range 遍历

| 数据规模 | 耗时 | 内存分配 |
|----------|------|----------|
| 100 条 | 3,166 ns/op | 1,600 B/op, 32 allocs |
| 1,000 条 | 20,072 ns/op | 16,384 B/op, 32 allocs |
| 10,000 条 | 173,123 ns/op | 172,032 B/op, 32 allocs |

> Range 每次遍历固定 32 次分配（每个分片 1 个 entry slice），内存随数据量线性增长。

### 适用场景

- **推荐使用**：高并发读写场景（如缓存、计数器、会话存储）
- **无需使用**：单线程或极低并发场景，标准 `sync.RWMutex` 即可

## 许可证

MIT
