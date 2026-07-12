package main

import (
	"fmt"
	"math"
	"testing"
)

// HyperLogLog should estimate cardinality within a few percent. The standard
// error for p registers is ~1.04/sqrt(2^p); at p=12 that is ~1.6%, so we assert
// a comfortable 5% envelope to stay robust across the fixed key set.
func TestHLLAccuracy(t *testing.T) {
	for _, n := range []int{1000, 100000, 1000000} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			h := NewHLL(12)
			for i := 0; i < n; i++ {
				h.Add(fmt.Sprintf("key-%d", i))
			}
			est := h.Count()
			relErr := math.Abs(est-float64(n)) / float64(n)
			if relErr > 0.05 {
				t.Fatalf("n=%d est=%.0f relErr=%.3f exceeds 5%%", n, est, relErr)
			}
		})
	}
}

// Adding the same key repeatedly must not inflate the estimate.
func TestHLLIdempotentKeys(t *testing.T) {
	h := NewHLL(12)
	for r := 0; r < 100; r++ {
		for i := 0; i < 500; i++ {
			h.Add(fmt.Sprintf("dup-%d", i))
		}
	}
	est := h.Count()
	relErr := math.Abs(est-500) / 500
	if relErr > 0.05 {
		t.Fatalf("est=%.0f for 500 distinct keys, relErr=%.3f", est, relErr)
	}
}
