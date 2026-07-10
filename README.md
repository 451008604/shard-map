# shard-map

`shard-map` 是一个基于 Go 泛型的并发安全分片 Map，适合读写并发较高的内存缓存、计数器和会话状态场景。

## 特性

- 32 个独立分片锁，降低读写竞争。
- 仅支持具有内建安全哈希的键类型：`string`、`bool`、整数、无符号整数、`uintptr`、浮点数和复数。
- 字符串哈希按逻辑长度逐块读取，不会访问字符串边界之外的内存。
- 零值可用；内部 map 与 `Range` 缓冲区按需初始化。
- `Range` 在锁外调用回调，避免长回调阻塞写入。

结构体、指针、interface，以及定义的新类型（例如 `type ID string`）不能作为键。语言别名可以使用，例如 `type ID = string`。

## 安装

```bash
go get github.com/451008604/shard-map
```

## 使用

```go
package main

import (
	"fmt"

	shardmap "github.com/451008604/shard-map"
)

func main() {
	var m shardmap.ShardMap[string, int]
	m.Set("foo", 42)

	if value, ok := m.Get("foo"); ok {
		fmt.Println(value)
	}

	m.Compute("requests", func(old int, loaded bool) int {
		if loaded {
			return old + 1
		}
		return 1
	})
}
```

`NewShardMap[K, V]()` 也可用于创建实例，但不是必需的。

## API 语义

| 方法 | 行为 |
| --- | --- |
| `Set` / `Get` / `Delete` | 单键并发安全读写。 |
| `LoadOrStore` | 原子地读取已有值或写入新值。 |
| `LoadOrCompute` | 计算函数在锁外执行；竞争时可能被调用多次，最终仅一个值被存储。 |
| `Compute` | 原子读-改-写；回调在对应分片写锁内执行，不能重入可能访问同一分片的方法。 |
| `Swap` | 原子替换并返回旧值及存在标志。 |
| `Len` | 无锁汇总分片计数；与并发写入同时调用时不是全局一致快照。 |
| `Range` | 逐分片快照遍历；回调返回 `false` 可提前停止，不保证全局一致快照。 |

## 验证与基准

在仓库根目录运行：

```bash
go test ./...
go test -race ./...
go vet ./...
go test -run '^$' -fuzz '^FuzzShardMapEqualStringsUseSameShard$' -fuzztime=20s
go test -bench=. -benchmem ./...
```

基准结果依赖 Go 版本、CPU、并发度和键分布。修改哈希或锁策略后，请重新测量，不要沿用历史数据。

## 许可证

MIT
