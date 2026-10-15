---
blueprint:
  type: adr
  id: art_adr-0003-lossless-metadata-carriage
  version: 1
  status: in_review
  inputs:
    artifacts:
      - { id: art_requirements, version: 1 }
  evidence:
    - "README.md:2 — the hard promise 'displays identically to the human eye (when opened in image viewers)'"
  guesses:
    - "Metadata carriage satisfies the requirement; the product owner may actually want LSB steganography (see Q-03)"
    - "A recoverable round-trip is required, not mere presence of bytes (see Q-01)"
---

# ADR 0003: Carry the hexstring in a lossless metadata carrier, not in pixels

## Context

The requirement "displays identically to the human eye (when opened in image viewers)" can be met two ways: *provably*, by never touching the bytes a viewer renders, or *approximately*, by altering pixels imperceptibly (classic LSB steganography). The name "spoof" suggests the output should pass as the original; an approximation invites per-viewer, per-display disagreement about "identical".

## Options considered

1. **Lossless metadata carriage** — write the payload into a non-rendered region of the container (e.g. a PNG ancillary `tEXt`-family chunk), copying all rendered data byte-for-byte.
2. **LSB steganography** — spread payload bits across least-significant pixel bits. Invisible to the eye, larger capacity per pixel, but pixels genuinely differ, so "identical" is only perceptual and any pixel diff or lossless re-encode can reveal or destroy it.
3. **Appended trailing bytes after the image's end marker** — simplest of all, but many viewers, hosts and re-saves strip trailing data, and it is the first place steganalysis looks.

## Decision

Use **lossless metadata carriage as the primary mechanism**: the payload is encoded into a format-native ancillary chunk, and pixel data is streamed through unchanged. "Displays identically" then holds by construction — the rendered bytes are identical, not merely similar — and extraction is a deterministic parse of the same chunk (round-trip, N-07). For a future format lacking metadata capacity, LSB becomes the documented fallback behind the same embed/extract interface.

## Consequences

- **+** Visual identity (R-04) is a byte-level guarantee, checkable in an automated test by comparing decoded pixels — no human judgment involved.
- **+** The original image is perfectly recoverable: delete the chunk and the file is the original again (N-02).
- **+** No per-pixel math; embedding is a cheap container splice, fast on any image (N-04).
- **−** Capacity is bounded by chunk-size limits rather than pixels; an "arbitrary" hexstring could overflow (R-10, Q-07).
- **−** The payload is discoverable by anyone who inspects chunks — this is carriage, not secrecy. If the goal is steganographic covertness, this decision must be revisited (Q-03, Q-09).
- **−** Some re-encoding/hosting pipelines strip ancillary chunks; the payload is not guaranteed to survive transport (Q-09).
