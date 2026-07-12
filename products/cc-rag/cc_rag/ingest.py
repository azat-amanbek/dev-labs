"""Ingest: turn Claude Code transcripts + memory files into retrievable chunks.

Keeps only human/assistant text (drops tool_use / tool_result noise), splits long
messages, and attaches a human-readable citation to each chunk.
"""
from __future__ import annotations

import glob
import json
import os
from dataclasses import dataclass


@dataclass
class Chunk:
    text: str
    source: str  # citation, e.g. "session 8eb8108f · 2026-07-11 · user"
    kind: str    # "transcript" | "memory"
    ref: str     # session id or file name


def _text_from_content(content) -> str:
    """A Claude message's content is a str or a list of blocks; keep only text."""
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = []
        for b in content:
            if isinstance(b, dict) and b.get("type") == "text":
                parts.append(b.get("text", ""))
            elif isinstance(b, str):
                parts.append(b)
        return "\n".join(p for p in parts if p)
    return ""


def _split(text: str, size: int = 1200, overlap: int = 150) -> list[str]:
    text = text.strip()
    if len(text) <= size:
        return [text] if text else []
    out, i = [], 0
    while i < len(text):
        out.append(text[i:i + size])
        i += size - overlap
    return out


def iter_transcripts(projects_dir: str):
    for f in glob.glob(os.path.join(projects_dir, "*", "*.jsonl")):
        session = os.path.splitext(os.path.basename(f))[0]
        try:
            lines = open(f, encoding="utf-8").read().splitlines()
        except OSError:
            continue
        for line in lines:
            if not line.strip():
                continue
            try:
                e = json.loads(line)
            except json.JSONDecodeError:
                continue
            if e.get("type") not in ("user", "assistant"):
                continue
            text = _text_from_content((e.get("message") or {}).get("content"))
            if len(text.strip()) < 20:  # drop "ok", acks, empty tool turns
                continue
            role = e.get("type")
            ts = (e.get("timestamp") or "")[:10]
            for c in _split(text):
                yield Chunk(c, f"session {session[:8]} · {ts} · {role}", "transcript", session)


def iter_memory(memory_dir: str):
    for f in glob.glob(os.path.join(memory_dir, "*.md")):
        name = os.path.basename(f)
        try:
            text = open(f, encoding="utf-8").read()
        except OSError:
            continue
        for c in _split(text):
            yield Chunk(c, f"memory/{name}", "memory", name)


def collect(projects_dir: str, memory_dir: str) -> list[Chunk]:
    chunks = list(iter_transcripts(projects_dir))
    if os.path.isdir(memory_dir):
        chunks += list(iter_memory(memory_dir))
    return chunks
