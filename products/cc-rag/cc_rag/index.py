"""Brute-force vector index: embeddings in a .npy, metadata in json. No DB.

At a few thousand chunks a full matrix multiply is instant, so a vector database
would be pure ceremony (YAGNI). Swap in pgvector here only if the corpus outgrows RAM.
"""
from __future__ import annotations

import json
import os
from dataclasses import asdict

import numpy as np

from .embed import Embedder
from .ingest import collect


def default_data_dir() -> str:
    return os.path.join(os.path.dirname(os.path.dirname(__file__)), "data")


def build(projects_dir: str, memory_dir: str, data_dir: str, emb: Embedder, batch: int = 256):
    chunks = collect(projects_dir, memory_dir)
    if not chunks:
        raise SystemExit("no chunks found — check --projects / --memory paths")
    texts = [c.text for c in chunks]
    parts = [emb.passages(texts[i:i + batch]) for i in range(0, len(texts), batch)]
    mat = np.vstack(parts).astype(np.float32)
    os.makedirs(data_dir, exist_ok=True)
    np.save(os.path.join(data_dir, "emb.npy"), mat)
    with open(os.path.join(data_dir, "meta.json"), "w", encoding="utf-8") as f:
        json.dump([asdict(c) for c in chunks], f, ensure_ascii=False)
    return len(chunks), emb.model_name


def load(data_dir: str):
    mat = np.load(os.path.join(data_dir, "emb.npy"))
    with open(os.path.join(data_dir, "meta.json"), encoding="utf-8") as f:
        meta = json.load(f)
    return mat, meta


def search(query: str, data_dir: str, emb: Embedder, k: int = 5):
    mat, meta = load(data_dir)
    q = emb.query(query)          # normalized
    scores = mat @ q             # cosine (both sides normalized)
    idx = np.argsort(-scores)[:k]
    return [(float(scores[i]), meta[i]) for i in idx]
