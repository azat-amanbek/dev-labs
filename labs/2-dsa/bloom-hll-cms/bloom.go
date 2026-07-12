package main

import "math"

// Bloom is a classic bit-array Bloom filter with k hash probes. It answers
// set membership with no false negatives and a tunable false-positive rate:
// "definitely not present" or "probably present".
type Bloom struct {
	bits []uint64 // packed bit array
	m    uint64   // number of bits
	k    int      // number of hash probes
}

// NewBloom builds a filter with m bits and k probes.
func NewBloom(m uint64, k int) *Bloom {
	if m == 0 {
		m = 1
	}
	if k < 1 {
		k = 1
	}
	return &Bloom{bits: make([]uint64, (m+63)/64), m: m, k: k}
}

// OptimalBloom sizes a filter to hold n items at target false-positive rate p,
// using the standard m = -n*ln(p)/ln(2)^2 and k = (m/n)*ln(2).
func OptimalBloom(n int, p float64) *Bloom {
	m := uint64(math.Ceil(-float64(n) * math.Log(p) / (math.Ln2 * math.Ln2)))
	k := int(math.Round(float64(m) / float64(n) * math.Ln2))
	return NewBloom(m, k)
}

func (b *Bloom) Add(key string) {
	h1, h2 := hashPair(key)
	for i := 0; i < b.k; i++ {
		pos := (h1 + uint64(i)*h2) % b.m
		b.bits[pos>>6] |= 1 << (pos & 63)
	}
}

// Test reports whether key is probably present. False means definitely absent.
func (b *Bloom) Test(key string) bool {
	h1, h2 := hashPair(key)
	for i := 0; i < b.k; i++ {
		pos := (h1 + uint64(i)*h2) % b.m
		if b.bits[pos>>6]&(1<<(pos&63)) == 0 {
			return false
		}
	}
	return true
}

// Bytes is the memory footprint of the bit array.
func (b *Bloom) Bytes() int { return len(b.bits) * 8 }

// TheoreticalFPR is the predicted false-positive rate after n insertions:
// (1 - e^{-kn/m})^k.
func (b *Bloom) TheoreticalFPR(n int) float64 {
	return math.Pow(1-math.Exp(-float64(b.k)*float64(n)/float64(b.m)), float64(b.k))
}
