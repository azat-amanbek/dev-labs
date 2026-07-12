"""Tiny retrieval eval: recall@k and MRR over a hand-labeled query set.

Each query names an `expect` substring that a relevant chunk (text or citation)
should contain. Not a framework — just enough to make retrieval quality observable.
"""
from __future__ import annotations

import yaml

from .embed import Embedder
from .index import search


def run(queries_file: str, data_dir: str, emb: Embedder, k: int = 5) -> dict:
    with open(queries_file, encoding="utf-8") as f:
        qs = yaml.safe_load(f) or []
    hits, rr, rows = 0, 0.0, []
    for item in qs:
        expect = item["expect"].lower()
        rank = 0
        for i, (_score, m) in enumerate(search(item["q"], data_dir, emb, k=k), 1):
            if expect in (m["text"] + " " + m["source"]).lower():
                rank = i
                break
        if rank:
            hits += 1
            rr += 1.0 / rank
        rows.append((item["q"], rank))
    n = len(qs) or 1
    return {"n": len(qs), "recall": hits / n, "mrr": rr / n, "rows": rows}
