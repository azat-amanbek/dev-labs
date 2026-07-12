# Bloom / HLL / Count-Min

**Status:** [~] in progress

## What
Three probabilistic structures built from scratch, stdlib only (`bloom.go`, `hll.go`,
`cms.go`), sharing one hash (`hash.go`: inlined FNV-1a + splitmix64 finalizer).
`main.go` runs all three and prints a memory-vs-exact comparison table.

- **Bloom filter** — configurable `m`, `k` (`OptimalBloom` sizes from n + target FPR).
  Adds 100k members, probes 100k absent keys, reports measured vs theoretical FPR.
- **HyperLogLog** — 12-bit registers (4096 × 1 byte). Estimates the cardinality of
  1,000,000 distinct keys via the bias-corrected harmonic mean + linear counting.
- **Count-Min sketch** — `d` rows × `w` counters, min-over-rows query. Estimates top-k
  frequencies over a deterministic Zipfian stream (`math/rand`, seed 42) and reports
  the overestimation (one-sided by construction — it never underestimates).

Observed (`go run .`):

| structure   | params         | sketch    | exact(min) | saving | accuracy                    |
|-------------|----------------|-----------|------------|--------|-----------------------------|
| Bloom       | m=958506 k=7   | 117 KiB   | 781 KiB    | 7×     | fpr 0.0096 (target 0.010)   |
| HyperLogLog | p=12, 4096 reg | 4.0 KiB   | 7.6 MiB    | 1953×  | 994,785 est, −0.52% err     |
| Count-Min   | w=2719 d=5     | 106 KiB   | 727 KiB    | 7×     | max over +0.63% over top-10 |

## Why (CTO lens)
All three trade a **bounded, tunable error for constant/sublinear memory** — the point is
that memory stops scaling with the data. HLL counts 1M distinct items in 4 KiB (~2000×
smaller than storing the keys) at ~1.6% standard error: that is how you get per-tenant
"unique users/devices" cardinality across billions of events without a per-key store.
Bloom turns an expensive existence check into 117 KiB of RAM — the "have I seen this
loan/txn id?" pre-filter that keeps 99% of lookups off Postgres/S3, at a false-positive
cost you dial in (never a false negative, so it's safe as a *pre*-filter). Count-Min gives
heavy-hitter frequencies (hot keys, rate-limit counters, top merchants) in fixed memory
with a one-sided error — you can never *undercount*, which is the right bias for abuse/limit
enforcement. The re-observed lesson recorded here: **the sketch only wins when the universe
is large.** At eps=0.0001 over a 100k-key universe the CMS was *bigger* than the exact map
(1× saving); relaxing to eps=0.001 restored a 7× win. Know your cardinality before you reach
for a sketch.

## Run
    go run .
    go test ./...

## Deep-dive next (on demand)
- HLL++ (Google, 2013) — 64-bit hashing, sparse representation, empirical bias correction
- Counting / scalable Bloom filters — support deletes and growth without rebuild
- Count-Min with conservative update + heavy-keeper — sharply lower overestimation for top-k
- register-width vs error trade-off: sweep p and plot measured error against 1.04/√m
