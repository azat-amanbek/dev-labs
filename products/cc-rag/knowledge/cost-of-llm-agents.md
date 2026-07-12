# What actually drives the cost of an LLM coding agent

## When
Someone blames cost/limits on the plugin stack, a big system prompt, or "too many
skills." Before optimizing, measure — the intuitive culprit is usually wrong.

## Method / findings
- **The loaded prefix is cheap.** First-turn `cache_creation` (system + tools +
  plugins) measured ~3k tokens per session. Even a heavy plugin stack is a rounding
  error.
- **The real driver is context growth over long sessions.** Cache-write ran ~1.7M
  tokens/session — ~500× the prefix. As a conversation grows, the expanding context
  is re-written to cache repeatedly. `cache_read` in the hundreds of millions confirms
  a large context re-read every turn.
- **Lever = session hygiene, not plugin hygiene.** `/clear`, shorter sessions,
  compaction cut context growth. Disabling plugins barely moves the bill.

## Measurement trick
Per-session cost from summed transcripts is **inflated** — `--resume`/compaction
copy history into a new file, so it double-counts. To measure the prefix cleanly, take
the **first assistant turn's `cache_creation_input_tokens`** (a single value, immune to
double-counting).

## Cache economics cheat-sheet
- cache-read ≈ 0.1× input; cache-write ≈ 1.25× input (5-min TTL); output ≈ 5× input.
- Watch: **cache hit rate** (high = stable prefix), **cache churn** (write$/total,
  high = prefix changing/growing), **output share** (high → lower effort / terser).

## В бою
"Why is this costing so much?" → don't guess, instrument. Prefix is cheap; long
sessions are expensive. On a flat subscription, dollars are a *shadow price* — the real
constraint is rate/context limits, not money.
