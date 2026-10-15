---
blueprint:
  type: adr
  id: art_0003-metadata-nonce-brute-force
  version: 1
  status: in_review
  inputs:
    artifacts:
      - { id: art_requirements, version: 1 }
      - { id: art_c4-container, version: 1 }
  evidence:
    - "README.md line 2: 'outputs an adjusted file that displays identically to the human eye (when opened in image viewers)': the file bytes change, the rendering does not"
  guesses:
    - "The hexstring is a required SHA-512 prefix of the output file (see Q-01, Q-02)"
    - "Pixel-level changes are not acceptable, even invisible ones (see Q-04)"
    - "A late-placed private chunk / COM segment is acceptable to users (see Q-10)"
---

# ADR-0003: Brute-force a nonce in an ignorable metadata block near the end of the file

Status: accepted (reconstructed from code)

> No code exists. This records the core algorithm implied by the README, assuming Q-01's answer is "hash prefix".

## Context

A cryptographic hash cannot be inverted. The only practical way to make a file's hash start with a chosen n-digit hexstring is to try many variants of the file until one matches (expected 16^n attempts). Each variant must render identically (R-04), and the search should be as cheap per attempt as possible (R-13).

## Options considered

1. **Change pixels invisibly** (e.g. flip least-significant bits). The image is visually the same but not pixel-identical. JPEG would need re-encoding, which is lossy and changes the whole file, so attempts are expensive. Rejected under assumption Q-04.
2. **Re-encode with different encoder settings** (compression level, filter choice). There are few variants, each is expensive, and lossless only for PNG. Rejected.
3. **Change existing metadata** (e.g. EXIF timestamps). This changes data the user cares about, and not every file has metadata. Rejected.
4. **Add a nonce block early in the file** (e.g. after the PNG `IHDR` or JPEG `SOI`). This is fully spec-compliant, but every attempt rehashes the whole file: cost O(file size) per attempt.
5. **Add a nonce block as late as possible** (PNG private ancillary chunk before `IEND`; JPEG `COM` before `EOI`), hash everything before it once, then clone that midstate for each attempt. Cost per attempt is O(1), about 1–2 SHA-512 blocks.

## Decision

Use **option 5**:
- Parse the file into blocks. Insert a nonce slot right before the final end marker.
- Compute `midstate = SHA512(head)` once. Workers clone it and hash `nonce bytes ‖ tail`, where the tail includes the PNG CRC-32 and `IEND`, or the JPEG `EOI`.
- Nonces are fixed-width ASCII hex counters. Workers stride through disjoint ranges.
- Before writing, re-hash the full output and compare its decoded pixels with the source (R-15).
- If a strict validator rejects the late JPEG `COM` (Q-10), fall back to option 4 for JPEG.

## Consequences

- The output is pixel-identical by construction, and verified. All original metadata is kept (R-12).
- Cost per attempt does not depend on image size, so throughput is limited only by SHA-512 speed and core count.
- The output is about 20–40 bytes larger than the input. Re-running spoof on its own output adds a second nonce block, unless the adapter replaces an existing spoof block (guess; worth doing).
- Runtime is probabilistic: there is no upper bound, only an expectation. A timeout is needed (R-14).
- The trick only defeats checks that compare a hash *prefix*. It does nothing against full-hash comparisons (see threats).
