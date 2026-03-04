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

## 许可证

MIT
