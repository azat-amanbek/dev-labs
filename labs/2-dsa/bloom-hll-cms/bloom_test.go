package main

import (
	"fmt"
	"math"
	"testing"
)

// A Bloom filter must never report a false negative: every added key tests true.
func TestBloomNoFalseNegatives(t *testing.T) {
	cases := []struct {
		n int
		p float64
	}{
		{1000, 0.01},
		{50000, 0.001},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("n=%d/p=%g", tc.n, tc.p), func(t *testing.T) {
			b := OptimalBloom(tc.n, tc.p)
			for i := 0; i < tc.n; i++ {
				b.Add(fmt.Sprintf("member-%d", i))
			}
			for i := 0; i < tc.n; i++ {
				if !b.Test(fmt.Sprintf("member-%d", i)) {
					t.Fatalf("false negative for member-%d", i)
				}
			}
		})
	}
}

// The measured false-positive rate should track the target within a small
// multiple (probabilistic, so we allow slack rather than an exact match).
func TestBloomFalsePositiveRate(t *testing.T) {
	const n, trials = 20000, 100000
	const target = 0.01
	b := OptimalBloom(n, target)
	for i := 0; i < n; i++ {
		b.Add(fmt.Sprintf("in-%d", i))
	}
	fp := 0
	for i := 0; i < trials; i++ {
		if b.Test(fmt.Sprintf("out-%d", i)) {
			fp++
		}
	}
	got := float64(fp) / trials
	if got > target*2 {
		t.Fatalf("false-positive rate %.4f exceeds 2x target %.4f", got, target)
	}
}

func TestBloomTheoreticalFPRMatchesMeasured(t *testing.T) {
	const n, trials = 20000, 100000
	b := OptimalBloom(n, 0.01)
	for i := 0; i < n; i++ {
		b.Add(fmt.Sprintf("in-%d", i))
	}
	fp := 0
	for i := 0; i < trials; i++ {
		if b.Test(fmt.Sprintf("out-%d", i)) {
			fp++
		}
	}
	got := float64(fp) / trials
	want := b.TheoreticalFPR(n)
	if math.Abs(got-want) > 0.01 {
		t.Fatalf("measured fp %.4f far from theory %.4f", got, want)
	}
}
