package main

import (
	"math"
	"math/bits"
)

// HLL is a HyperLogLog cardinality estimator. It counts distinct items in
// fixed memory (2^p one-byte registers) by tracking, per register, the maximum
// number of leading zeros seen in the hash — a proxy for "how rare has the
// rarest bit-pattern been", which scales with cardinality.
type HLL struct {
	p   uint    // register-index bits
	m   uint32  // number of registers = 2^p
	reg []uint8 // each holds rho = leading-zero-run length + 1
}

// NewHLL builds an estimator with 2^p registers. p=12 -> 4096 registers = 4 KiB.
func NewHLL(p uint) *HLL {
	m := uint32(1) << p
	return &HLL{p: p, m: m, reg: make([]uint8, m)}
}

func (h *HLL) Add(key string) {
	x := hash64(key)
	idx := x >> (64 - h.p) // top p bits select the register
	// Remaining bits, left-aligned. The guard bit at position (p-1) bounds rho
	// to its max (64-p+1) when the remaining bits happen to be all zero.
	w := (x << h.p) | (1 << (h.p - 1))
	rho := uint8(bits.LeadingZeros64(w)) + 1
	if rho > h.reg[idx] {
		h.reg[idx] = rho
	}
}

// Count estimates the cardinality using the standard bias-corrected harmonic
// mean, with linear counting for the small-cardinality range. The large-range
// correction is unnecessary here because we hash to 64 bits, so collisions in
// the hash space are negligible at these scales.
func (h *HLL) Count() float64 {
	m := float64(h.m)
	sum := 0.0
	zeros := 0
	for _, r := range h.reg {
		sum += 1.0 / float64(uint64(1)<<r)
		if r == 0 {
			zeros++
		}
	}
	est := alpha(h.m) * m * m / sum
	if est <= 2.5*m && zeros > 0 {
		return m * math.Log(m/float64(zeros)) // linear counting
	}
	return est
}

// Bytes is the register array footprint.
func (h *HLL) Bytes() int { return len(h.reg) }

// alpha is the bias-correction constant for m registers.
func alpha(m uint32) float64 {
	switch m {
	case 16:
		return 0.673
	case 32:
		return 0.697
	case 64:
		return 0.709
	default:
		return 0.7213 / (1 + 1.079/float64(m))
	}
}
