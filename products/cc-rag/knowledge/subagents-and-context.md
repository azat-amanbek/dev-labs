# Subagents & context management (as architecture)

## When
A session is getting long and expensive, or work spans several mostly-independent
pieces. Also: any time you're tempted to reach for the heavy subagent workflow.

## Principle
- **Long single sessions bloat context**, which drives both cost (context re-cached
  every turn) and quality loss (the model reasons worse buried in a huge context).
- **The architectural fix is subagent-driven work:** a fresh, bounded context per task;
  the subagent never inherits your history — you construct exactly the brief it needs;
  artifacts move as **files**, not pasted text (pasted text stays resident in the
  orchestrator's context forever); the orchestrator stays thin, coordinating only.
- This is **context management as architecture** — the same lever as `/clear` and
  compaction, applied structurally.

## Calibration (don't cargo-cult it)
- Heavy subagent-driven process (implementer + reviewer per task, worktree, ledger) is
  **for executing a real multi-task plan**, and needs a design + plan first (brainstorm
  → plan → execute). It is **overkill for a ~300-line tool** — that's better built in one
  focused inline pass than fragmented across dispatches that each rebuild context.
- **Match process weight to task size.** Spawn subagents for genuinely bounded/
  parallel/context-heavy work; do small, coupled work inline.
- Pick the cheapest model that fits each subagent role; specify it explicitly (an omitted
  model inherits the expensive session default).

## В бору
"Should I spawn subagents for this?" → is the work bounded and independent, and large
enough that a fresh cold context pays off? If it's small or tightly coupled, inline wins.
