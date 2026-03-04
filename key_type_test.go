// key_type_test.go
package shardedmap

import "testing"

// 辅助函数，用于测试任意可比较键类型 K 和任意值类型 V 的基本操作
func testKeyType[K comparable, V comparable](t *testing.T, keys []K, values []V) {
    if len(keys) != len(values) {
        t.Fatalf("keys and values length mismatch")
    }
    m := New[K, V]()
    // 写入
    for i, k := range keys {
        m.Set(k, values[i])
    }
    // 读取并验证
    for i, k := range keys {
        if v, ok := m.Get(k); !ok || v != values[i] {
            t.Fatalf("Get failed for key %v: got %v (ok=%v), want %v", k, v, ok, values[i])
        }
    }
    // 长度检查
    if got := m.Len(); got != len(keys) {
        t.Fatalf("Len mismatch: want %d, got %d", len(keys), got)
    }
    // 遍历检查
    seen := make(map[K]bool)
    m.Range(func(k K, v V) bool {
        // 找到对应的索引并比较值
        found := false
        for i, kk := range keys {
            if kk == k {
                if v != values[i] {
                    t.Fatalf("Range value mismatch for key %v: got %v, want %v", k, v, values[i])
                }
                found = true
                break
            }
        }
        if !found {
            t.Fatalf("Range visited unexpected key %v", k)
        }
        seen[k] = true
        return true
    })
    if len(seen) != len(keys) {
        t.Fatalf("Range visited %d keys, want %d", len(seen), len(keys))
    }
    // 删除并检查
    for _, k := range keys {
        m.Delete(k)
    }
    if got := m.Len(); got != 0 {
        t.Fatalf("After Delete Len mismatch: want 0, got %d", got)
    }
}

func TestKeyTypeCompatibility(t *testing.T) {
    t.Run("string", func(t *testing.T) {
        keys := []string{"a", "b", "c", "d"}
        vals := []int{1, 2, 3, 4}
        testKeyType(t, keys, vals)
    })
    t.Run("int", func(t *testing.T) {
        keys := []int{10, 20, 30, 40}
        vals := []string{"x", "y", "z", "w"}
        testKeyType(t, keys, vals)
    })
    t.Run("int64", func(t *testing.T) {
        keys := []int64{1001, 2002, 3003}
        vals := []bool{true, false, true}
        testKeyType(t, keys, vals)
    })
    t.Run("uint", func(t *testing.T) {
        keys := []uint{1, 2, 3, 4, 5}
        vals := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
        testKeyType(t, keys, vals)
    })
    t.Run("float64", func(t *testing.T) {
        keys := []float64{1.5, 2.5, 3.5}
        vals := []rune{'a', 'b', 'c'}
        testKeyType(t, keys, vals)
    })
    t.Run("struct", func(t *testing.T) {
        type structKey struct {
            PartA int
            PartB string
        }
        keys := []structKey{{1, "one"}, {2, "two"}, {3, "three"}}
        vals := []int{100, 200, 300}
        testKeyType(t, keys, vals)
    })
    t.Run("typeAlias", func(t *testing.T) {
        type MyString string
        keys := []MyString{"alpha", "beta", "gamma"}
        vals := []int{7, 8, 9}
        testKeyType(t, keys, vals)
    })
}
