// idempotency-guard: a Bloom filter as a "negative cache" in front of an exact
// ledger, in a payment-ingest scenario. Archetype: OLTP / correctness-critical.
//
// The lesson is NOT "how a Bloom filter works" — it's WHY you reach for one here:
// its error is one-sided in the SAFE direction, so it can cheaply skip work
// without ever risking correctness.
package main

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
)

// ---- Bloom filter: the "definitely-not" oracle (compact, inline on purpose) ----

type bloom struct {
	bits []uint64
	m    uint64 // total bits
	k    int    // probes per key
}

func newBloom(n int, fpr float64) *bloom {
	m := uint64(math.Ceil(-float64(n) * math.Log(fpr) / (math.Ln2 * math.Ln2)))
	k := int(math.Round(float64(m) / float64(n) * math.Ln2))
	if k < 1 {
		k = 1
	}
	return &bloom{bits: make([]uint64, (m+63)/64), m: m, k: k}
}

// Kirsch–Mitzenmacher: derive k probe positions from two 32-bit halves of one hash.
func (b *bloom) probes(key string) []uint64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	sum := h.Sum64()
	h1, h2 := uint32(sum), uint32(sum>>32)
	ps := make([]uint64, b.k)
	for i := 0; i < b.k; i++ {
		ps[i] = (uint64(h1) + uint64(i)*uint64(h2)) % b.m
	}
	return ps
}

func (b *bloom) add(key string) {
	for _, p := range b.probes(key) {
		b.bits[p/64] |= 1 << (p % 64)
	}
}

// maybe: true = MIGHT have been added; false = DEFINITELY was not (never wrong on false).
func (b *bloom) maybe(key string) bool {
	for _, p := range b.probes(key) {
		if b.bits[p/64]&(1<<(p%64)) == 0 {
			return false
		}
	}
	return true
}

// ---- the payment-ingest scenario ----

func main() {
	const (
		events  = 200_000
		dupRate = 0.30 // 30% of incoming events are replays of an earlier payment
		fpr     = 0.01 // Bloom target false-positive rate
	)
	rng := rand.New(rand.NewSource(42))

	ledger := make(map[string]struct{}) // EXACT source of truth (stands in for Postgres)
	filter := newBloom(events, fpr)      // negative cache in front of it

	var (
		fastSkips int // Bloom said "definitely new" -> no DB touch at all (the win)
		dbLookups int // Bloom said "maybe" -> we had to ask the exact ledger
		realDup   int // ...and it truly was a duplicate (correctly rejected)
		falsePos  int // ...but it was actually new (Bloom cried wolf: wasted lookup)
		missedDup int // duplicate that slipped through the fast path -> MUST stay 0
	)

	var issued []string // pool of already-seen payment ids, to replay as duplicates

	for i := 0; i < events; i++ {
		var id string
		if len(issued) > 0 && rng.Float64() < dupRate {
			id = issued[rng.Intn(len(issued))] // replay an old payment id
		} else {
			id = fmt.Sprintf("pay-%d", i)
		}

		if !filter.maybe(id) {
			// FAST PATH: Bloom is certain it's new -> skip the DB entirely.
			// Safety check: was it *actually* already in the ledger? (should never happen)
			if _, dup := ledger[id]; dup {
				missedDup++
			}
			ledger[id] = struct{}{}
			filter.add(id)
			fastSkips++
			issued = append(issued, id)
			continue
		}

		// SLOW PATH: Bloom says "maybe" -> confirm against the exact ledger.
		dbLookups++
		if _, dup := ledger[id]; dup {
			realDup++
			continue // correctly rejected duplicate
		}
		// false positive: it was new after all
		falsePos++
		ledger[id] = struct{}{}
		filter.add(id)
		issued = append(issued, id)
	}

	naive := events // without the guard, EVERY event costs one ledger lookup
	fmt.Printf("payment-ingest: %d events, %.0f%% duplicates\n\n", events, dupRate*100)
	fmt.Printf("  DB lookups WITHOUT guard : %d (one per event)\n", naive)
	fmt.Printf("  DB lookups WITH guard    : %d  (%.1f%% of naive)\n", dbLookups, 100*float64(dbLookups)/float64(naive))
	fmt.Printf("  fast-path skips (no DB)  : %d\n", fastSkips)
	fmt.Printf("    of the DB lookups: %d real duplicates, %d false positives (wasted)\n", realDup, falsePos)
	fmt.Printf("  false-positive rate      : %.4f (target %.4f)\n", float64(falsePos)/float64(fastSkips+dbLookups), fpr)
	fmt.Printf("\n  duplicates wrongly processed (MUST be 0): %d\n", missedDup)
	if missedDup == 0 {
		fmt.Println("  -> correctness held: a one-sided error never cost us a missed duplicate.")
	} else {
		fmt.Println("  -> BUG: Bloom produced a false negative, which is impossible if correct.")
	}
}
