package main

import (
	"fmt"
	"testing"
)

// Count-Min never underestimates: the estimate is always >= the true count.
func TestCMSNeverUnderestimates(t *testing.T) {
	c := NewCMSError(0.001, 0.01)
	exact := map[string]uint64{}
	for i := 0; i < 100000; i++ {
		k := fmt.Sprintf("k-%d", i%2000) // 2000 distinct keys, skewed by modulo
		c.Add(k, 1)
		exact[k]++
	}
	for k, want := range exact {
		if got := c.Count(k); got < want {
			t.Fatalf("underestimate for %s: got %d want %d", k, got, want)
		}
	}
}

// The overestimate should be bounded by eps*N with high probability. We assert
// the mean absolute overestimate stays within eps*N as a stable check.
func TestCMSErrorBound(t *testing.T) {
	const eps, delta = 0.001, 0.01
	c := NewCMSError(eps, delta)
	exact := map[string]uint64{}
	var total uint64
	for i := 0; i < 200000; i++ {
		k := fmt.Sprintf("k-%d", i%1000)
		c.Add(k, 1)
		exact[k]++
		total++
	}
	bound := eps * float64(total)
	for k, want := range exact {
		over := float64(c.Count(k) - want)
		if over > bound {
			t.Fatalf("overestimate %.0f for %s exceeds eps*N=%.0f", over, k, bound)
		}
	}
}
