"""Local multilingual embeddings via fastembed (ONNX, no torch).

The one deliberate seam: swap this class to change the embedding model/backend.
e5 models expect "query:" / "passage:" prefixes and are trained for cosine, so we
L2-normalize (cosine then reduces to a dot product at search time).
"""
from __future__ import annotations

import numpy as np
from fastembed import TextEmbedding

DEFAULT_MODEL = "intfloat/multilingual-e5-small"  # RU + EN


def resolve_model(preferred: str = DEFAULT_MODEL) -> str:
    """Pick the preferred model if available, else the smallest multilingual one."""
    models = TextEmbedding.list_supported_models()
    names = {m["model"] for m in models}
    if preferred in names:
        return preferred
    multis = [m for m in models if "multilingual" in m["model"].lower()]
    if multis:
        multis.sort(key=lambda m: m.get("size_in_GB", 99))
        return multis[0]["model"]
    return models[0]["model"]


class Embedder:
    def __init__(self, model: str | None = None):
        self.model_name = resolve_model(model or DEFAULT_MODEL)
        self._m = TextEmbedding(model_name=self.model_name)
        # "query:"/"passage:" prefixes are e5-specific; they hurt other models.
        self._e5 = "e5" in self.model_name.lower()

    def _embed(self, texts: list[str], prefix: str) -> np.ndarray:
        if self._e5:
            texts = [prefix + t for t in texts]
        vecs = list(self._m.embed(texts))
        arr = np.asarray(vecs, dtype=np.float32)
        norms = np.linalg.norm(arr, axis=1, keepdims=True)
        norms[norms == 0] = 1.0
        return arr / norms

    def passages(self, texts: list[str]) -> np.ndarray:
        return self._embed(texts, "passage: ")

    def query(self, text: str) -> np.ndarray:
        return self._embed([text], "query: ")[0]
