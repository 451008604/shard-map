package shardmap

import "testing"

func FuzzShardMapOperations(f *testing.F) {
	f.Add([]byte{0, 1, 10, 1, 1, 0, 2, 1, 0, 3, 1, 0})
	f.Add([]byte{4, 2, 7, 5, 2, 9, 6, 2, 0, 7, 0, 0})

	f.Fuzz(func(t *testing.T, operations []byte) {
		// 限制单个输入的工作量，让 fuzz 吞吐量保持稳定。
		if len(operations) > 3*1024 {
			operations = operations[:3*1024]
		}
		m := NewShardMap[uint8, uint8]()
		model := make(map[uint8]uint8)
		for i := 0; i+2 < len(operations); i += 3 {
			op, key, value := operations[i]%8, operations[i+1], operations[i+2]
			switch op {
			case 0:
				m.Set(key, value)
				model[key] = value
			case 1:
				got, ok := m.Get(key)
				want, exists := model[key]
				if ok != exists || got != want {
					t.Fatalf("Get(%d) = (%d, %v), want (%d, %v)", key, got, ok, want, exists)
				}
			case 2:
				m.Delete(key)
				delete(model, key)
			case 3:
				got, loaded := m.LoadOrStore(key, value)
				want, exists := model[key]
				if !exists {
					want = value
					model[key] = value
				}
				if loaded != exists || got != want {
					t.Fatalf("LoadOrStore(%d) = (%d, %v), want (%d, %v)", key, got, loaded, want, exists)
				}
			case 4:
				got, loaded := m.LoadOrCompute(key, func() uint8 { return value })
				want, exists := model[key]
				if !exists {
					want = value
					model[key] = value
				}
				if loaded != exists || got != want {
					t.Fatalf("LoadOrCompute(%d) = (%d, %v), want (%d, %v)", key, got, loaded, want, exists)
				}
			case 5:
				old, exists := model[key]
				got, loaded := m.Swap(key, value)
				model[key] = value
				if loaded != exists || got != old {
					t.Fatalf("Swap(%d) = (%d, %v), want (%d, %v)", key, got, loaded, old, exists)
				}
			case 6:
				want := model[key] + value
				got := m.Compute(key, func(old uint8, loaded bool) uint8 { return old + value })
				model[key] = want
				if got != want {
					t.Fatalf("Compute(%d) = %d, want %d", key, got, want)
				}
			case 7:
				assertMapMatchesModel(t, m, model)
			}
			if got := m.Len(); got != len(model) {
				t.Fatalf("Len = %d, want %d", got, len(model))
			}
		}
		assertMapMatchesModel(t, m, model)
	})
}

func assertMapMatchesModel(t *testing.T, m *ShardMap[uint8, uint8], model map[uint8]uint8) {
	t.Helper()
	seen := make(map[uint8]uint8, len(model))
	m.Range(func(key, value uint8) bool {
		if _, duplicate := seen[key]; duplicate {
			t.Fatalf("Range returned duplicate key %d", key)
		}
		seen[key] = value
		return true
	})
	if len(seen) != len(model) {
		t.Fatalf("Range returned %d entries, want %d", len(seen), len(model))
	}
	for key, want := range model {
		if got, ok := seen[key]; !ok || got != want {
			t.Fatalf("Range[%d] = (%d, %v), want (%d, true)", key, got, ok, want)
		}
	}
}
