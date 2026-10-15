---
blueprint:
  type: adr
  id: art_0002-supported-formats-png-jpeg
  version: 1
  status: in_review
  inputs:
    artifacts:
      - { id: art_requirements, version: 1 }
      - { id: art_c4-container, version: 1 }
  evidence:
    - "README.md line 2: 'An image … displays identically … when opened in image viewers' (no format named)"
  guesses:
    - "PNG and JPEG cover the users' needs for the first release (see Q-03)"
    - "Each supported format must have a block that viewers ignore, so pixels stay identical (see Q-04)"
---

# ADR-0002: Support PNG and JPEG first, behind a format-adapter interface

Status: accepted (reconstructed from code)

> No code exists, and the README does not name formats. This records the recommended scope. Confirm via Q-03.

## Context

The approach in ADR-0003 needs every supported format to provide:
1. a way to add bytes that all mainstream viewers ignore, so decoded pixels do not change (R-04);
2. a position for those bytes near the end of the file, so the hash midstate can be reused (R-13);
3. a structure simple enough to parse safely from untrusted input (R-18).

## Options considered

| Format | Ignorable container | Late placement possible? | Assessment |
|---|---|---|---|
| **PNG** | Ancillary chunks: lowercase first letter of the type, e.g. a private `spOf` chunk or `tEXt` | Yes, just before `IEND` (needs a per-nonce CRC-32) | Very clean. The spec requires decoders to skip unknown ancillary chunks |
| **JPEG** | `COM` segment (`FF FE`), or `APPn` | Yes, just before `EOI` (widely tolerated); or after `SOI` (strictly legal, but loses the midstate benefit) | Ubiquitous. Some placement nuance (Q-10) |
| GIF | Comment extension (`21 FE`) | Yes, before the trailer `3B` | Easy to add later; less common for photos |
| WebP | Unknown RIFF chunks | Yes, but the RIFF size header changes with the payload length | Feasible later |
| BMP / TIFF / HEIC | Trailing bytes / private tags / boxes | Varies | Viewer behaviour is less uniform; defer |
| Any format: append after end-of-image | Trailing garbage | Yes | Simple, but some tools flag or strip it. Kept only as a possible fallback |

## Decision

Support **PNG and JPEG** in the first release. Each lives behind a `FormatAdapter` interface (`Detect`, `Parse`, `NonceSlot`, `Assemble`). Reject every other format with a clear "unsupported format" error (R-09). GIF and WebP are the next candidates.

## Consequences

- This covers the large majority of images people share, with two small, well-specified parsers.
- A new format only needs a new adapter. The search engine and verifier do not change.
- The verifier needs a decoder for each format. Go's stdlib provides PNG, JPEG and GIF; WebP decoding needs `golang.org/x/image`.
- Users with HEIC or WebP files get a clear error, not a silently broken file.
