---
blueprint:
  type: nfr
  id: art_nfr-and-threats
  version: 1
  status: in_review
  inputs:
    artifacts:
      - { id: art_c4-container, version: 1 }
  evidence:
    - "README.md:2 — 'displays identically to the human eye (when opened in image viewers)' is both the primary NFR and the trust-boundary statement"
  guesses:
    - "Threat priorities below assume a personal, offline, non-adversarial-use tool (see Q-09)"
    - "No code exists yet, so all 'risks noticed' are design-stage risks, not code bugs (see Q-01, Q-03, Q-07)"
---

# NFRs and threats — spoof

## Non-functional characteristics

| # | Characteristic | Target | Notes |
|---|---|---|---|
| N-01 | Visual fidelity | Byte-identical rendered data | Guaranteed by the carrier choice (ADR 0003), not by approximation |
| N-02 | Reversibility | Original pixels always recoverable | Removing the payload chunk restores the input exactly |
| N-03 | Runtime footprint | Python 3 stdlib only, no installs | ADR 0001 **(guessed, Q-04)** |
| N-04 | Performance | O(file size), one linear pass; sub-second for typical images | Streaming I/O; no per-pixel work |
| N-05 | Portability | Linux / macOS / Windows, same behavior | README implies arbitrary viewers/platforms |
| N-06 | Robustness | Bad input → non-zero exit, clear message, **no output file written** | Truncated/corrupt images, invalid hexstring, oversized payload |
| N-07 | Testability | `extract(embed(img, h)) == h` as a pure property test | Core byte-transforms need no filesystem; pixel-identity compared automatically |
| N-08 | Verifiability | Every run self-checks by re-extracting before reporting success | Makes the R-04 promise checkable per invocation **(guessed, Q-06)** |

## STRIDE-lite threats

| Threat | Applies how | Severity | Mitigation / note |
|---|---|---|---|
| **Spoofing** | The output is designed to pass as the original image — that is the product. A file so adjusted can be presented as untouched evidence or as the "same" photo with hidden content. | Medium (societal) | Inherent to purpose. Document clearly in the tool's own output (e.g. mention the payload in a listing mode) so honest users can always tell an adjusted file from an original. Needs the product owner's intent (Q-09). |
| **Tampering** | Anyone with the output can strip or replace the payload chunk, or embed a different payload in someone else's image; the carrier carries no integrity/authenticity. | Low | Out of scope for v1; an optional checksum/keyed tag could detect (not prevent) it. |
| **Repudiation** | An adjusted file leaves no record of who created it or when. | Low | A debug/listing mode suffices for a personal tool. |
| **Information disclosure** | The payload is a covert channel: an innocuous-looking image can smuggle arbitrary bytes past cursory inspection. | **High** | Payload is plaintext-in-chunk, so *any* inspection tool reveals it (no real secrecy — see ADR 0003). If secrecy is the intent, the whole approach changes (Q-03, Q-09). |
| **Denial of service** | Hostile inputs: a hexstring sized in gigabytes (argument-length limits help but `-` file input wouldn't), a crafted PNG with a chunk-length field near 2³¹ (allocation bomb), or an image that is actually a zip bomb in disguise. | Medium | Enforce a maximum carrier/chunk size (R-10); stream with bounded buffers; never trust the length field without bounding it. |
| **Elevation of privilege** | None: the tool runs as the user, touches only the paths it is given, no network, no install hooks. | Low | Keep it that way: no shell-outs, no auto-update, no execution of embedded content. |

## Risks and open concerns noticed during analysis

No code exists, so there are no code bugs to report — but these design-stage risks are worth recording:

1. **"Identical" is only as strong as the weakest viewer** (README.md:2). Metadata carriage makes it byte-true (ADR 0003), but if the product owner actually wants LSB embedding (Q-03), visual identity becomes probabilistic and viewer-dependent, and R-04 should be rewritten.
2. **Payload fragility in transit.** Re-encoding, many social/media platforms, and some editors strip ancillary chunks silently. If images are meant to be shared and later extracted, the payload will vanish without warning (Q-09).
3. **"Arbitrary" vs capacity.** README promises an arbitrary hexstring, but any finite carrier has a limit. The failure mode (error vs truncate, R-10) must be a conscious choice (Q-07).
4. **Dual-use nature.** The name and purpose ("spoof") mean the tool could be used to conceal data in ways that look deceptive. Worth a visible notice in `--help` and documentation; whether that suffices depends on the intended use (Q-09).
5. **Chunk-collision risk (future formats).** When JPEG/GIF support arrives (ADR 0002), naïve comment-segment handling can collide with existing metadata (EXIF/XMP) — the adapter contract should mandate non-destructive coexistence, or embedding will silently destroy a real photo's EXIF.
