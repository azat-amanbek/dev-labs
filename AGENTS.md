# AGENTS.md — dev-labs

Personal practice monorepo: full-stack architecture mastery (`labs/`) + shipping
small products (`products/`). Owner is a fintech CTO — treat as peer-level, move
fast, focus on the architecturally hard parts. Read this before doing any work here.

This is the **single canonical instruction file** (both Air and Claude Code read it).
`CLAUDE.md` is only a pointer here — do not duplicate guidance into it.

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

## Self-consult — recall-first

Before a non-trivial **design, architecture, or method** decision in this repo, first
consult the distilled knowledge base: from `products/cc-rag/` run

    uv run rag query "<the situation in a few words>"

and apply any relevant retrieved method/decision (name which one). This trades a cheap
retrieval for expensive re-derivation and keeps decisions consistent with prior ones —
that is the efficiency win (retrieval replaces re-thinking; it does **not** mean
injecting on every turn). Skip it for trivial or unrelated tasks. When you make a new
durable method/decision, add or update a `products/cc-rag/knowledge/*.md` doc so the
next recall finds it.

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

## Repo map & how to run

- Three areas: `labs/` (numbered tracks `0-os` … `5-strategy`, each subtopic its own
  dir), `products/` (`_template/` seeds a new product), `sandbox/` (`infra/` = shared
  local deps, `scratch/` = gitignored throwaway).
- **Each Go lab is its own module** (`go.mod`, `package main`) — no `go.work` ties them.
  Run from inside the lab dir. Go `1.26.5`.

      go run .            # run a lab from its directory
      go test ./...       # its tests

  Alternative entrypoints may be guarded by `//go:build ignore` (each with its own
  `func main`, excluded from the default build); run one explicitly: `go run <file>.go`.
- Shared deps (Postgres 16 + Redis 7):

      cd sandbox/infra && docker compose up -d    # pg :5432, redis :6379
      docker compose down

  DSN `postgres://dev:dev@localhost:5432/playground`. If a lab ships a `schema.sql`, load
  it first: `psql "<dsn>" -f schema.sql`.
- Product scaffold: copy `products/_template/`, then `make up` / `make down` / `make logs`.
- Gotcha: the `exposure-ledger` lab needs `github.com/lib/pq` + a running Postgres with
  its `schema.sql` applied before `go run .`.
- **Do not commit compiled binaries** — labs build to a binary named after the dir; use
  `go run` and let `.gitignore` keep them out.

## Docs & status

- Every lab has a `README.md` with three fields: **what / why (CTO lens) / status**.
- When a topic moves, update the `labs/STATUS.md` heat-map: `[ ]` → `[~]` → `[x]`.

## Git / workflow

- Private repo `git@github.com:azat-amanbek/dev-labs.git`, default branch `main`.
- Air (Local Workspace + Full Access) writes **directly into the `main` working tree** —
  it does NOT isolate in a separate worktree in this mode. Review the uncommitted diff
  before committing; don't hand-edit the same files while an Air task is running.
- Never commit secrets. `.env` is gitignored; `products/_template/.env.example` is the
  tracked sample.
- Windows git needs a `safe.directory` exception for the WSL UNC path (already set) — do
  not re-`git init` from the Windows side.
