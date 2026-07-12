# Designing a retrieval-first RAG (the pragmatic defaults)

## When
Building search/RAG over a personal or mid-size corpus. Resist the urge to reach for
the heavyweight stack first.

## Decisions
- **Brute-force cosine beats a vector DB under ~100k vectors.** Hold normalized
  embeddings in a matrix (`.npy`); search is one matmul, instant. pgvector/Qdrant earn
  their keep only past that scale, or when you need concurrency/SQL/persistence. Adding
  one for a few thousand vectors is pure ceremony (YAGNI).
- **Retrieval-first.** Return ranked, cited chunks + quality metrics before adding any
  generation step. The interesting, learnable part is retrieval; generation is a thin
  later layer (or hand it to the calling agent).
- **Match the embedding model to the corpus language.** Mixed RU+EN → a *multilingual*
  model, or non-Latin text embeds poorly. Local (fastembed/ONNX) keeps it free + private.
- **Model-specific input format matters.** e5 models need `query:` / `passage:`
  prefixes; other models don't (the prefix becomes junk tokens). Make it conditional.
- **Corpus quality > cleverness.** Indexing raw transcripts pulls in injected
  skill-docs and chatter as noise. Prefer a **distilled** corpus (decisions, methods)
  over raw history. Garbage in the corpus caps retrieval quality no matter the model.

## Eval
Use recall@k / MRR, but a **small eval set (n < ~20) is noise** — a single query flips
the score by 0.2. Do not tune (prefixes, model, hybrid) on a 5-query set; when the
instrument can't adjudicate, follow the principled default and enlarge the eval first.

## В бою
"Should I add a vector DB / hybrid / reranker?" → only if eval on a real query set shows
the simpler version failing. Start thin; let data justify each added moving part.
