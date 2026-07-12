# cc-rag

Retrieval-first RAG over **your own Claude Code history + memory** — local,
brute-force, no vector DB, no LLM/API in the core. Ask "what did I decide about X /
how did we do Y" and get ranked, cited chunks from your past sessions.

## What / Why (CTO lens)

The corpus is your `~/.claude/projects/**/*.jsonl` transcripts + `memory/*.md`.
Claude Code can query it via Bash (`rag query "..."`) to teach itself from prior
work. Deliberately thin: embeddings live in a `.npy` and search is a matrix
multiply — a vector database earns its keep only past ~100k vectors (YAGNI).

## Run

    uv sync
    uv run rag index                 # build the index (downloads the embed model once)
    uv run rag query "что решили про хранилище"
    uv run rag eval                  # recall@k / MRR over queries.yaml

## Design (thin by choice)

- **Local multilingual embeddings** (`multilingual-e5-small` via fastembed) — RU + EN.
- **Brute-force cosine** over a normalized matrix; no pgvector.
- **One seam:** `Embedder` (swap the model/backend). Everything else is plain functions.
- `data/` (the index) holds private transcript text — gitignored.

## Deferred (when a driver appears)

- hybrid retrieval (vector + keyword) — add if eval shows vector misses exact terms
- MCP tool — if calling the CLI over Bash gets clunky
- write-back learning loop (v2) — feeds the *existing* memory, not a new silo
- cloud (pgvector on RDS) — when access from multiple machines matters
