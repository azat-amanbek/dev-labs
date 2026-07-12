package main

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"sort"
)

// Ring is a consistent-hash ring with virtual nodes.
type Ring struct {
	vnodes  int
	keys    []uint32          // sorted positions on the ring
	hashMap map[uint32]string // position -> physical node
}

func New(vnodes int) *Ring {
	return &Ring{vnodes: vnodes, hashMap: map[uint32]string{}}
}

func hash(s string) uint32 {
	h := sha1.Sum([]byte(s))
	return binary.BigEndian.Uint32(h[:4])
}

func (r *Ring) Add(nodes ...string) {
	for _, n := range nodes {
		for i := 0; i < r.vnodes; i++ {
			h := hash(fmt.Sprintf("%s#%d", n, i))
			r.keys = append(r.keys, h)
			r.hashMap[h] = n
		}
	}
	sort.Slice(r.keys, func(i, j int) bool { return r.keys[i] < r.keys[j] })
}

// Get returns the node owning key (first vnode clockwise).
func (r *Ring) Get(key string) string {
	if len(r.keys) == 0 {
		return ""
	}
	h := hash(key)
	idx := sort.Search(len(r.keys), func(i int) bool { return r.keys[i] >= h })
	if idx == len(r.keys) {
		idx = 0
	}
	return r.hashMap[r.keys[idx]]
}

func main() {
	const N = 100_000
	const vnodes = 150
	nodes := []string{"n1", "n2", "n3", "n4"}

	r := New(vnodes)
	r.Add(nodes...)

	before := make(map[string]string, N)
	count := map[string]int{}
	for i := 0; i < N; i++ {
		k := fmt.Sprintf("key-%d", i)
		n := r.Get(k)
		before[k] = n
		count[n]++
	}

	fmt.Printf("%d keys over %d nodes, %d vnodes each\n", N, len(nodes), vnodes)
	for _, n := range nodes {
		fmt.Printf("  %s: %6d  (%.1f%%)\n", n, count[n], 100*float64(count[n])/N)
	}

	r.Add("n5")
	moved := 0
	nc := map[string]int{}
	for i := 0; i < N; i++ {
		k := fmt.Sprintf("key-%d", i)
		n := r.Get(k)
		nc[n]++
		if before[k] != n {
			moved++
		}
	}

	fmt.Printf("\nafter adding n5:\n")
	for _, n := range append(nodes, "n5") {
		fmt.Printf("  %s: %6d  (%.1f%%)\n", n, nc[n], 100*float64(nc[n])/N)
	}

	fmt.Printf("\nkeys remapped: %d / %d = %.1f%%   (ideal 1/5 = 20.0%%)\n",
		moved, N, 100*float64(moved)/N)
	fmt.Printf("naive hash%%N would remap ~%.0f%% of all keys.\n", 100*(1-1.0/5))
}
