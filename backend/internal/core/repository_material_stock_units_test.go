package core

import "testing"

func TestDecodeStockUnitAllocationsTreatsPersistedJSONNullAsEmpty(t *testing.T) {
	for _, value := range []any{nil, "null", []byte("null")} {
		allocations, err := decodeStockUnitAllocations(value)
		if err != nil {
			t.Fatalf("decodeStockUnitAllocations(%T) error = %v", value, err)
		}
		if len(allocations) != 0 {
			t.Fatalf("decodeStockUnitAllocations(%T) = %#v, want empty", value, allocations)
		}
	}
}
