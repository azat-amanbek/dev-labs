package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

// row is one line of the final comparison table.
type row struct {
	name    string
	params  string
	sketch  int    // sketch memory, bytes
	exact   int    // minimal exact-structure memory, bytes (fingerprint baseline)
	quality string // accuracy summary for this structure
}

func main() {
	fmt.Println("Probabilistic structures — sketch vs exact, from scratch (stdlib only)")
	fmt.Println()

	rows := []row{
		runBloom(),
		runHLL(),
		runCMS(),
	}

	fmt.Println("== comparison ==")
	fmt.Println("(exact = minimal exact structure storing 8-byte fingerprints per key;")
	fmt.Println(" a real set/map with string keys and bucket overhead costs several times more.)")
	fmt.Println()
	fmt.Printf("%-12s %-22s %12s %14s %9s  %s\n", "structure", "params", "sketch", "exact(min)", "saving", "accuracy")
	for _, r := range rows {
		fmt.Printf("%-12s %-22s %12s %14s %8.0fx  %s\n",
			r.name, r.params, humanBytes(r.sketch), humanBytes(r.exact),
			float64(r.exact)/float64(r.sketch), r.quality)
	}
}

// 1 — Bloom filter: measured false-positive rate at a target sizing.
func runBloom() row {
	const n, probes = 100_000, 100_000
	const target = 0.01
	b := OptimalBloom(n, target)

	for i := 0; i < n; i++ {
		b.Add(fmt.Sprintf("member-%d", i))
	}
	// Probe with keys guaranteed absent from the set.
	fp := 0
	for i := 0; i < probes; i++ {
		if b.Test(fmt.Sprintf("absent-%d", i)) {
			fp++
		}
	}
	measured := float64(fp) / probes
	fmt.Println("[1] Bloom filter")
	fmt.Printf("    n=%d target-fpr=%.3f -> m=%d bits, k=%d probes (%s)\n",
		n, target, b.m, b.k, humanBytes(b.Bytes()))
	fmt.Printf("    false positives: %d/%d = %.4f measured  vs %.4f theoretical\n\n",
		fp, probes, measured, b.TheoreticalFPR(n))

	return row{
		name:    "Bloom",
		params:  fmt.Sprintf("m=%d k=%d", b.m, b.k),
		sketch:  b.Bytes(),
		exact:   n * 8,
		quality: fmt.Sprintf("fpr %.4f (target %.3f)", measured, target),
	}
}

// 2 — HyperLogLog: cardinality of 1,000,000 distinct keys.
func runHLL() row {
	const n = 1_000_000
	h := NewHLL(12)
	for i := 0; i < n; i++ {
		h.Add(fmt.Sprintf("key-%d", i))
	}
	est := h.Count()
	relErr := (est - float64(n)) / float64(n)
	fmt.Println("[2] HyperLogLog")
	fmt.Printf("    p=12 -> %d registers (%s)\n", h.m, humanBytes(h.Bytes()))
	fmt.Printf("    exact=%d  estimate=%.0f  error=%+.2f%%  (std err ~%.2f%%)\n\n",
		n, est, relErr*100, 100*1.04/math.Sqrt(float64(h.m)))

	return row{
		name:    "HyperLogLog",
		params:  fmt.Sprintf("p=12 (%d reg)", h.m),
		sketch:  h.Bytes(),
		exact:   n * 8,
		quality: fmt.Sprintf("card %.0f, err %+.2f%%", est, relErr*100),
	}
}

// 3 — Count-Min sketch: top-k frequencies over a Zipfian stream.
func runCMS() row {
	const (
		events   = 1_000_000
		universe = 100_000
		topK     = 10
		eps      = 0.001
		delta    = 0.01
		skew     = 1.2
	)
	c := NewCMSError(eps, delta)
	exact := make(map[uint64]uint64, universe)

	// Deterministic Zipfian stream (fixed seed => reproducible observation).
	rng := rand.New(rand.NewSource(42))
	zipf := rand.NewZipf(rng, skew, 1.0, universe-1)
	for i := 0; i < events; i++ {
		id := zipf.Uint64()
		key := fmt.Sprintf("item-%d", id)
		c.Add(key, 1)
		exact[id]++
	}

	// Rank exact frequencies to find the true top-k.
	type kv struct {
		id    uint64
		count uint64
	}
	ranked := make([]kv, 0, len(exact))
	for id, cnt := range exact {
		ranked = append(ranked, kv{id, cnt})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].count > ranked[j].count })

	fmt.Println("[3] Count-Min sketch")
	fmt.Printf("    stream=%d events, universe=%d, Zipf s=%.1f -> w=%d d=%d (%s)\n",
		events, universe, skew, c.w, c.d, humanBytes(c.Bytes()))
	fmt.Printf("    top-%d frequencies (exact vs estimate):\n", topK)
	fmt.Printf("      %-12s %10s %10s %10s\n", "key", "exact", "estimate", "over")
	var maxOverAbs, maxOverRel float64
	for i := 0; i < topK && i < len(ranked); i++ {
		id, want := ranked[i].id, ranked[i].count
		got := c.Count(fmt.Sprintf("item-%d", id))
		over := float64(got - want)
		rel := over / float64(want) * 100
		if over > maxOverAbs {
			maxOverAbs = over
		}
		if rel > maxOverRel {
			maxOverRel = rel
		}
		fmt.Printf("      item-%-7d %10d %10d %+10d\n", id, want, got, got-want)
	}
	fmt.Printf("    max overestimate over top-%d: %.0f (%.4f%%); eps*N bound = %.0f\n\n",
		topK, maxOverAbs, maxOverRel, eps*float64(events))

	// Exact baseline: fingerprint -> count = 16 bytes per distinct key.
	return row{
		name:    "Count-Min",
		params:  fmt.Sprintf("w=%d d=%d", c.w, c.d),
		sketch:  c.Bytes(),
		exact:   len(exact) * 16,
		quality: fmt.Sprintf("max over %+.4f%% (top-%d)", maxOverRel, topK),
	}
}

func humanBytes(b int) string {
	switch {
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
