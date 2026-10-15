---
blueprint:
  type: adr
  id: art_adr-0002-png-first-format-support
  version: 1
  status: in_review
  inputs:
    artifacts:
      - { id: art_requirements, version: 1 }
  evidence:
    - "README.md:2 — says only 'An image', without naming any format"
  guesses:
    - "PNG as the first and mandatory format (see Q-02)"
    - "JPEG and GIF as later additions, not first-release scope (see Q-02)"
---

# ADR 0002: Support PNG first; add JPEG and GIF behind a format-adapter interface

## Context

The README says the tool takes "An image" but names no format. Formats differ radically in how (or whether) an invisible carrier exists: PNG has free-form ancillary chunks; JPEG has comment and APPn segments; a lossless GIF has extension blocks. Supporting every format on day one multiplies the parsing surface before the core idea is validated.

## Options considered

1. **PNG only, first release** — one lossless container to get right, with a clean adapter interface for later formats.
2. **All common raster formats at once** (PNG, JPEG, GIF, BMP, WebP) — maximal coverage, but each format is its own parser project and WebP/BMP add little value early.
3. **JPEG only** — the most common format in the wild, but its lossy pipeline makes "displays identically" awkward: re-encoding a JPEG almost never reproduces the original bytes, whereas a comment-segment edit avoids re-encode entirely.

## Decision

**PNG is the first supported format** and the reference implementation. The format adapter in the container diagram (`c4-container.md`) defines the contract — `detect`, `parse`, `embed`, `extract`, `serialize` — and JPEG (comment/APP segment) and GIF (comment extension) are added later by implementing that interface, without touching the embedder/extractor. Unsupported formats are rejected with a clear message (R-09).

## Consequences

- **+** PNG is lossless and its ancillary-chunk mechanism gives a carrier that provably never touches rendered pixels (feeds ADR 0003).
- **+** One format done correctly establishes the adapter contract the others will follow.
- **−** Users with JPEGs get a "not supported yet" error in the first release — the most likely early complaint.
- **−** PNG-only scope is a guess; the README does not even imply PNG (Q-02).
