package main

import "math"

// CMS is a Count-Min sketch: d independent rows of w counters. Each item is
// hashed into one counter per row and incremented; a query returns the minimum
// across its d counters. Collisions can only ADD to a counter, so the estimate
// never underestimates — it is a one-sided (over)estimate of the true count.
type CMS struct {
	d, w   int
	counts [][]uint64
}

// NewCMS builds a sketch with w counters per row and d rows.
func NewCMS(w, d int) *CMS {
	if w < 1 {
		w = 1
	}
	if d < 1 {
		d = 1
	}
	c := &CMS{d: d, w: w, counts: make([][]uint64, d)}
	for i := range c.counts {
		c.counts[i] = make([]uint64, w)
	}
	return c
}

// NewCMSError sizes a sketch so that, with probability >= 1-delta, the
// overestimate is at most eps*N (N = total mass). w = e/eps, d = ln(1/delta).
func NewCMSError(eps, delta float64) *CMS {
	w := int(math.Ceil(math.E / eps))
	d := int(math.Ceil(math.Log(1 / delta)))
	return NewCMS(w, d)
}

func (c *CMS) Add(key string, n uint64) {
	h1, h2 := hashPair(key)
	for i := 0; i < c.d; i++ {
		pos := (h1 + uint64(i)*h2) % uint64(c.w)
		c.counts[i][pos] += n
	}
}

// Count returns the estimated frequency of key (min over the d rows).
func (c *CMS) Count(key string) uint64 {
	h1, h2 := hashPair(key)
	min := ^uint64(0)
	for i := 0; i < c.d; i++ {
		pos := (h1 + uint64(i)*h2) % uint64(c.w)
		if v := c.counts[i][pos]; v < min {
			min = v
		}
	}
	return min
}

// Bytes is the counter-matrix footprint (8 bytes per uint64 counter).
func (c *CMS) Bytes() int { return c.d * c.w * 8 }
