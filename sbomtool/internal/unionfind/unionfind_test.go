package unionfind

import "testing"

func TestBasicUnion(t *testing.T) {
	keys := []string{"a", "b"}
	uf := New(2)
	uf.Union(0, 1, keys)
	if uf.Find(0) != uf.Find(1) {
		t.Error("elements should be in same set")
	}
}

func TestTransitiveUnion(t *testing.T) {
	keys := []string{"a", "b", "c"}
	uf := New(3)
	uf.Union(0, 1, keys)
	uf.Union(1, 2, keys)
	if uf.Find(0) != uf.Find(2) {
		t.Error("a and c should be in same set via b")
	}
}

func TestDeterminism(t *testing.T) {
	keys := []string{"z", "a", "m"}
	uf1 := New(3)
	uf1.Union(0, 1, keys)
	uf1.Union(1, 2, keys)

	uf2 := New(3)
	uf2.Union(0, 1, keys)
	uf2.Union(1, 2, keys)

	for i := 0; i < 3; i++ {
		if uf1.Find(i) != uf2.Find(i) {
			t.Error("same operations should produce same roots")
		}
	}
}

func TestPathCompression(t *testing.T) {
	keys := []string{"a", "b", "c", "d"}
	uf := New(4)
	uf.Union(0, 1, keys)
	uf.Union(1, 2, keys)
	uf.Union(2, 3, keys)
	root := uf.Find(3)
	// After find, 3 should point directly to root
	if uf.parent[3] != root {
		t.Error("path compression should update parent")
	}
}

func TestEmpty(t *testing.T) {
	uf := New(0)
	// Should not panic
	_ = uf
}

func TestSingleElement(t *testing.T) {
	uf := New(1)
	if uf.Find(0) != 0 {
		t.Error("single element should be its own root")
	}
}

func TestOutOfBounds(t *testing.T) {
	uf := New(3)
	keys := []string{"a", "b", "c"}

	// Find with invalid indices returns -1
	if uf.Find(-1) != -1 {
		t.Error("Find(-1) should return -1")
	}
	if uf.Find(3) != -1 {
		t.Error("Find(n) where n >= size should return -1")
	}
	if uf.Find(100) != -1 {
		t.Error("Find(100) should return -1")
	}

	// Union with invalid indices is a no-op (should not panic)
	uf.Union(-1, 0, keys)
	uf.Union(0, -1, keys)
	uf.Union(3, 0, keys)
	uf.Union(0, 3, keys)
	uf.Union(100, 0, keys)

	// Union with nil keys doesn't panic
	uf.Union(0, 1, nil)

	// Verify state unchanged after invalid operations
	if uf.Find(0) != 0 || uf.Find(1) != 1 || uf.Find(2) != 2 {
		t.Error("invalid operations should not change state")
	}
}
