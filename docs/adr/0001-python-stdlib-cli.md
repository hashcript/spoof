---
blueprint:
  type: adr
  id: art_adr-0001-python-stdlib-cli
  version: 1
  status: in_review
  inputs:
    artifacts:
      - { id: art_requirements, version: 1 }
  evidence:
    - "README.md:2 — no language, runtime or dependency is specified anywhere in the repository"
  guesses:
    - "Python 3 over Go/Rust/C (see Q-04)"
    - "Standard library only, including for PNG chunk handling (zlib/crc via stdlib) (see Q-04)"
---

# ADR 0001: Implement as a Python 3, stdlib-only CLI

## Context

The repository contains only a README; nothing constrains the implementation language, runtime, or dependencies. The tool's problem space is byte-level file manipulation: parse an image container, splice bytes into it, re-serialize it. Such code benefits from a language with cheap string/bytes handling and easy distribution as a single script.

## Options considered

1. **Python 3, standard library only** — `zlib` and CRC computation cover PNG chunk re-serialization; `argparse` gives the CLI; runs anywhere a Python 3 interpreter exists; zero install footprint.
2. **Go or Rust** — single static binary, faster on huge files, but heavier scaffolding for a small utility and a compile step for every user platform.
3. **C/C++** — maximal control and speed, but slow to write safely and the memory-safety risk is needless here.
4. **Node.js/TypeScript** — viable, but adds a runtime and package ecosystem for no gain over Python in this domain.

## Decision

Implement spoof as a **single Python 3 command-line tool using only the standard library**. The core (format adapter, embedder, extractor) lives in an importable module so it doubles as a library; the CLI is a thin `argparse` front end.

## Consequences

- **+** Trivial to run and distribute: copy one file, no `pip install`, no builds.
- **+** Fast iteration for a tool that is pure byte-shuffling and has no performance-critical inner loop beyond streaming I/O.
- **+** Stdlib `zlib`/CRC support is exactly what PNG chunk work needs.
- **−** Slower than Go/Rust on very large images; acceptable since the operation is one linear pass (N-04).
- **−** Users must already have Python 3; on Windows this is occasionally friction.
- Language choice is still open for the product owner to confirm (Q-04).
