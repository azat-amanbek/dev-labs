package main

// Shared hashing. No external deps: an inlined, allocation-free FNV-1a followed
// by the splitmix64 finalizer so the bits avalanche well. HLL leans on the high
// bits (leading-zero count) while Bloom/CMS split the word into two independent
// halves for double hashing — both need good distribution across all 64 bits.

const (
	fnvOffset64 = 1469598103934665603
	fnvPrime64  = 1099511628211
)

// fnv1a hashes s without allocating (indexing bytes, not ranging runes).
func fnv1a(s string) uint64 {
	h := uint64(fnvOffset64)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime64
	}
	return h
}

// mix64 is the splitmix64 finalizer: strong avalanche, no dependencies.
func mix64(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return x
}

// hash64 is the single 64-bit hash used by HyperLogLog.
func hash64(s string) uint64 { return mix64(fnv1a(s)) }

// hashPair derives two independent hashes for Kirsch-Mitzenmacher double hashing
// (h_i = h1 + i*h2). h2 is forced odd so successive probes always step to a fresh
// slot instead of collapsing onto h1.
func hashPair(s string) (h1, h2 uint64) {
	h := hash64(s)
	h1 = h
	h2 = mix64(h) | 1
	return
}
