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
go test -run '^$' -fuzz '^FuzzShardMapOperations$' -fuzztime=20s
go test -bench=. -benchmem ./...
```

基准结果依赖 Go 版本、CPU、并发度和键分布。修改哈希或锁策略后，请重新测量，不要沿用历史数据。

### Docker 基准结果

以下结果采集于 2026-07-20，环境为 Go 1.21.13、Linux/arm64、4 vCPU、`GOMAXPROCS=4`。镜像为
`golang:1.21.13-bookworm`（digest `sha256:c6a5b9308b3f3095e8fde83c8bf4d68bd101fce606c1a0a1394522542509dda9`）。
每个基准运行 5 次、每次至少 500 ms；表中为 `ns/op` 的中位数，越低越好。

复现命令：

```bash
docker run --rm --cpus=4 -e GOMAXPROCS=4 \
  -v "$PWD:/src:ro" -w /src \
  golang@sha256:c6a5b9308b3f3095e8fde83c8bf4d68bd101fce606c1a0a1394522542509dda9 \
  go test -run '^$' -bench=. -benchmem -benchtime=500ms -count=5 ./...
```

单线程操作：

| 操作 | ShardMap | 单锁 map |
| --- | ---: | ---: |
| Set | 229.2 | 171.6 |
| Get | 28.45 | 21.70 |
| Delete | 148.7 | 134.3 |

固定 worker 数的并发操作：

| Worker | ShardMap 写 | 单锁 map 写 | 写入加速比 | ShardMap 读 | 单锁 map 读 | 读取加速比 |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 41.49 | 29.33 | 0.71x | 31.65 | 23.86 | 0.75x |
| 4 | 55.00 | 85.83 | 1.56x | 24.69 | 88.75 | 3.59x |
| 16 | 59.78 | 119.1 | 1.99x | 25.75 | 85.73 | 3.33x |
| 64 | 69.48 | 158.7 | 2.28x | 25.95 | 84.31 | 3.25x |
| 256 | 70.69 | 155.3 | 2.20x | 28.19 | 81.70 | 2.90x |
| 1024 | 70.11 | 271.3 | 3.87x | 25.79 | 88.21 | 3.42x |

`Range` 遍历：

| 键数量 | ns/op | allocs/op |
| ---: | ---: | ---: |
| 100 | 2,176 | 0 |
| 1,000 | 13,301 | 0 |
| 10,000 | 112,513 | 0 |

加速比按“单锁 map / ShardMap”计算，大于 1 表示 ShardMap 更快。这些数字仅代表上述容器资源和当前测试键分布；单线程路径会承担分片选择和长度计数成本，并发收益会随 CPU、调度、读写比例及热点键分布变化。

## 许可证

MIT
