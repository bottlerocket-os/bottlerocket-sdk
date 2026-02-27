// Package unionfind implements a disjoint-set data structure with path compression.
package unionfind

// UnionFind implements a disjoint-set data structure.
type UnionFind struct {
	parent []int
}

// New creates a UnionFind with n elements.
func New(n int) *UnionFind {
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	return &UnionFind{parent: parent}
}

// Find returns the root of the set containing x, with path compression.
// Returns -1 if x is out of bounds.
func (uf *UnionFind) Find(x int) int {
	if x < 0 || x >= len(uf.parent) {
		return -1
	}
	if uf.parent[x] != x {
		uf.parent[x] = uf.Find(uf.parent[x])
	}
	return uf.parent[x]
}

// Union merges the sets containing x and y.
// Uses keys for lexicographic comparison to ensure deterministic root selection.
// This trades optimal tree height (rank-based union) for deterministic output,
// which is acceptable given our small dataset sizes.
// No-op if x or y is out of bounds.
func (uf *UnionFind) Union(x, y int, keys []string) {
	if x < 0 || x >= len(uf.parent) || y < 0 || y >= len(uf.parent) {
		return
	}
	px, py := uf.Find(x), uf.Find(y)
	if px != py {
		// keys must have same length as parent
		if px == -1 || py == -1 || keys == nil || px >= len(keys) || py >= len(keys) {
			return
		}
		if keys[px] < keys[py] {
			uf.parent[py] = px
		} else {
			uf.parent[px] = py
		}
	}
}
