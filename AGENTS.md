# AGENTS.md — dev-labs

Personal practice monorepo: full-stack architecture mastery (`labs/`) + shipping
small products (`products/`). Owner is a fintech CTO — treat as peer-level, move
fast, focus on the architecturally hard parts. Read this before doing any work here.

## Prime directive: two hands, do not mix them

- **`labs/**` = LEARNING.** Hand-coded by the human (GoLand / by hand). The friction
  *is* the learning. **Agents: do NOT implement labs unless explicitly asked.** You
  may explain, review, or generate a scaffold ONLY on an explicit request for that lab.
- **`products/**` = SHIPPING.** Agent-built work is welcome. Full autonomy within the
  task's worktree; human reviews the diff before merge.
- **`sandbox/**` = throwaway infra + scratch.** `sandbox/scratch/` is gitignored.

## Methodology — the scientific loop

> **PLACEHOLDER — replace this whole section with Azat's own "scientific method"**
> **formulation (the one taught to corp Claude).** The version below is a stand-in;
> overwrite it verbatim with yours so work↔home stay consistent.

Treat every non-trivial change as an experiment:

1. **Hypothesis** — state the design and expected behavior/economics *before* coding.
   Where does it sit in the generative dimensions (consistency↔availability, read/write
   profile, latency, scale, correctness-criticality, …)? Architecture is derived from
   coordinates, not memorized.
2. **Prediction** — write the test(s) first. A failing test is the falsifiable prediction.
3. **Experiment** — implement the minimum needed to make the prediction pass.
4. **Observation** — actually RUN it and read real output/metrics. No "done" without
   observed behavior.
5. **Review / falsification** — actively try to break it, request code review, and
   record what the numbers actually showed.

Never report success you have not observed. If a step was skipped, say so plainly.

## Go conventions

- Idiomatic Go, standard layout. No external deps in a lab unless the lab is *about* a
  library. Prefer the stdlib.
- `go run .` and `go test ./...` must pass from the package directory.
- Errors: wrap with `%w`, never swallow silently. Tests are table-driven.
- Keep files/packages focused — one clear purpose each. A file that grows large is a
  signal it does too much.

## System-design lens (the whole point of this repo)

- Derive architecture from the ~12 generative dimensions; don't cargo-cult patterns.
- For anything under `labs/3-system-design/`, name which archetype family it belongs to
  (serving / oltp / analytics / coordination / specialized) and why.
- Always name the cost-to-serve and reliability trade-off. This is a lending/fintech
  brain — money movement, ledgers, correctness, high reliability.

## Docs & status

- Every lab has a `README.md` with three fields: **what / why (CTO lens) / status**.
- When a topic moves, update the `labs/STATUS.md` heat-map: `[ ]` → `[~]` → `[x]`.

## Git / workflow

- Private repo `git@github.com:azat-amanbek/dev-labs.git`, default branch `main`.
- Air runs agents in git worktrees — keep each task's changes scoped; human reviews the
  diff before merging to `main`.
- Never commit secrets. `.env` is gitignored; `products/_template/.env.example` is the
  tracked sample.
- Windows git needs a `safe.directory` exception for the WSL UNC path (already set) — do
  not re-`git init` from the Windows side.
