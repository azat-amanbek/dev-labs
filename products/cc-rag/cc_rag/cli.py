"""cc-rag CLI: index / query / eval. Claude Code can call `rag query` via Bash."""
from __future__ import annotations

import argparse
import os

from .embed import Embedder
from .eval import run as eval_run
from .index import build, default_data_dir, search


def _default_projects() -> str:
    p = "/mnt/c/Users/aamanbek/.claude/projects"
    return p if os.path.isdir(p) else os.path.expanduser("~/.claude/projects")


def _default_memory() -> str:
    return "/mnt/c/Users/aamanbek/.claude/projects/C--Users-aamanbek-Desktop-startup/memory"


def _queries_path() -> str:
    return os.path.join(os.path.dirname(os.path.dirname(__file__)), "queries.yaml")


def main(argv=None):
    ap = argparse.ArgumentParser(prog="rag", description="retrieval over your Claude history + memory")
    ap.add_argument("--data", default=default_data_dir())
    ap.add_argument("--model", default=None, help="override embedding model")
    sub = ap.add_subparsers(dest="cmd", required=True)

    pi = sub.add_parser("index", help="(re)build the index")
    pi.add_argument("--projects", default=_default_projects())
    pi.add_argument("--memory", default=_default_memory())

    pq = sub.add_parser("query", help="search the index")
    pq.add_argument("text")
    pq.add_argument("-k", type=int, default=5)

    pe = sub.add_parser("eval", help="recall@k / MRR over queries.yaml")
    pe.add_argument("--queries", default=_queries_path())
    pe.add_argument("-k", type=int, default=5)

    a = ap.parse_args(argv)
    emb = Embedder(a.model)

    if a.cmd == "index":
        n, model = build(a.projects, a.memory, a.data, emb)
        print(f"indexed {n} chunks with {model} -> {a.data}")
    elif a.cmd == "query":
        for score, m in search(a.text, a.data, emb, k=a.k):
            snippet = " ".join(m["text"].split())
            if len(snippet) > 240:
                snippet = snippet[:240] + "…"
            print(f"[{score:.3f}] {m['source']}\n    {snippet}\n")
    elif a.cmd == "eval":
        r = eval_run(a.queries, a.data, emb, k=a.k)
        print(f"n={r['n']}  recall@{a.k}={r['recall']:.2f}  mrr={r['mrr']:.2f}")
        for q, rank in r["rows"]:
            print(f"  {'✓' if rank else '✗'} rank={rank or '-':<3} {q}")


if __name__ == "__main__":
    main()
