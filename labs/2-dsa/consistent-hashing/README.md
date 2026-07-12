# Consistent hashing

**Status:** [x] working depth

## What
Hash ring with virtual nodes. ring.go distributes 100k keys over 4 nodes,
adds a 5th, and measures how many keys are remapped.

## Why (CTO lens)
Adding a node remaps only ~K/N keys (measured 18%, ideal 20%) instead of ~80%
for naive "hash mod N". This is why resharding Redis Cluster / cache partitions
/ sharded storage does not trigger a full data reshuffle. Virtual nodes (150 per
physical) keep load skew within ~+/-3% instead of wild imbalance.

## Run
    go run .

## Deep-dive next (on demand)
- bounded-load consistent hashing (Google, 2017) - caps hot-node overload
- rendezvous (HRW) hashing - no ring, per-key max-weight, simpler failover
- jump consistent hash - O(1) memory, no vnodes, but nodes only append
