---
blueprint:
  type: adr
  id: art_0001-language-go
  version: 1
  status: in_review
  inputs:
    artifacts:
      - { id: art_requirements, version: 1 }
      - { id: art_c4-container, version: 1 }
  evidence:
    - "README.md line 2: the workload is CPU-bound hash brute-forcing over an image file (implied by 'arbitrary hexstring' + 'adjusted file')"
    - "git ls-files: no source files, go.mod, package.json, pyproject.toml or Cargo.toml exist, so no language has been chosen yet"
  guesses:
    - "No language is mandated by the product owner (see Q-07)"
    - "Prefixes of about 8 hex digits must be practical, which rules out an interpreted per-attempt loop (see Q-06)"
---

# ADR-0001: Implement spoof in Go as a single static binary

Status: accepted (reconstructed from code)

> Nothing has been implemented yet. The README names no language. This ADR records the recommended decision so the implementation starts from it. Confirm or override it via Q-07.

## Context

- The core loop tries nonces until the file hash starts with the hexstring (R-05). Expected attempts are 16^n: about 65 k for 4 digits, about 4.3 × 10⁹ for 8 digits.
- The loop needs a fast SHA-512 with a cloneable midstate, cheap CRC-32 (for PNG), and easy multi-core use (R-13).
- The tool also needs safe binary parsing of untrusted files (R-18), a PNG/JPEG decoder for verification (R-15), and easy distribution to users on any OS (R-16).

## Options considered

| Option | Pros | Cons |
|---|---|---|
| **Python 3** (`hashlib`, `Pillow`) | Fastest to write; `hashlib` objects support `.copy()` for midstate | Interpreter overhead of about 1 µs per attempt. 8-digit prefixes take hours on one core; `multiprocessing` adds complexity. Needs a Python runtime and Pillow to verify |
| **Go** | Stdlib has `crypto/sha512` (state clone via `encoding.BinaryMarshaler`), `hash/crc32`, `image/png`, `image/jpeg`. Goroutines make the parallel search trivial. Memory-safe. Cross-compiles to static binaries | Somewhat slower SHA than hand-tuned C/Rust; more verbose than Python |
| **Rust** (`sha2`, `png`, `jpeg-decoder`, `rayon`) | Top performance; memory-safe; static binaries | More third-party crates; slower to develop for a small tool |
| **C/C++** (OpenSSL) | Top performance | Not memory-safe for parsing untrusted files; dependency and build friction on each OS |

## Decision

Use **Go**. Only the standard library is needed for hashing, CRC, PNG/JPEG decoding and concurrency. Ship one statically linked `spoof` binary per OS and architecture.

## Consequences

- No runtime to install. Release artefacts come from `GOOS`/`GOARCH` cross-compilation.
- Parsing is memory-safe and bounds-checked by default, which reduces the risk from hostile inputs (see `nfr-and-threats.md`).
- Throughput is a few million SHA-512 attempts per second per core, so 8-digit prefixes finish in minutes on a laptop. Much longer prefixes need GPU work, which is out of scope.
- If the product owner needs a Python library interface (Q-07), the search core could later be exposed as a library or rewritten. The adapter and search interfaces in `c4-container.md` keep that change local.
