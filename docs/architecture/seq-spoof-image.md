---
blueprint:
  type: sequence
  id: art_seq-spoof-image
  version: 1
  status: in_review
  inputs:
    artifacts:
      - { id: art_c4-container, version: 1 }
  evidence:
    - "README.md line 2: the single end-to-end flow: take an image and a hexstring, output an adjusted file that displays identically"
  guesses:
    - "The goal is a SHA-512 prefix match on the whole output file (see Q-01, Q-02)"
    - "The nonce goes in a block near the end of the file, so the hash midstate can be reused (see ADR-0003, Q-10)"
    - "The first worker to find a match wins, so output is not deterministic (see Q-09)"
---

# Sequence: spoof an image to a target hash prefix

```mermaid
sequenceDiagram
    autonumber
    actor U as User
    participant CLI as spoof CLI
    participant FS as Filesystem
    participant DET as Format detector
    participant AD as Format adapter (PNG/JPEG)
    participant SE as Search engine
    participant W as Workers ×N
    participant V as Verifier
    participant OW as Output writer

    U->>CLI: spoof 0x24ab in.png out.png
    CLI->>CLI: normalise hexstring (strip 0x, lowercase, validate [0-9a-f]+)
    alt invalid hexstring or output path == input path
        CLI-->>U: error on stderr, exit 2
    end
    CLI->>FS: read in.png (size-capped)
    FS-->>CLI: bytes
    CLI->>DET: detect(bytes)
    DET-->>CLI: PNG adapter (or error → exit 1)
    CLI->>AD: parse(bytes)
    AD->>AD: walk chunks/segments with bounds checks
    AD-->>CLI: NonceSlot{head, tail template}
    CLI->>SE: search(prefix, slot, timeout)
    SE->>SE: midstate = SHA512(head)
    par worker k = 0..N-1
        SE->>W: start(k, stride N)
        loop nonce = k, k+N, ...
            W->>W: h = clone(midstate)
            W->>AD: encode(nonce) → nonce bytes + tail (e.g. CRC + IEND)
            W->>W: h.write(nonce bytes + tail); d = h.sum()
            W->>W: d has prefix?
        end
        W-->>SE: match(nonce, digest)
    end
    SE->>W: cancel remaining workers
    alt timeout reached
        SE-->>CLI: no match
        CLI-->>U: "no match within limit", exit 3
    end
    SE-->>CLI: nonce, digest
    CLI->>AD: assemble(nonce) → output bytes
    CLI->>V: verify(source, output, prefix)
    V->>V: SHA512(output) has prefix
    V->>V: decode(source) pixels == decode(output) pixels
    V-->>CLI: ok
    CLI->>OW: write(out.png, bytes)
    OW->>FS: create out.png.tmp, write, fsync
    OW->>FS: rename out.png.tmp → out.png
    CLI-->>U: digest (hex) on stdout, exit 0
```

## Notes

- **Why the midstate matters:** SHA-512 handles data in 128-byte blocks. If the nonce slot comes after almost all of the file, each attempt only has to hash the last block or two. The per-attempt cost then stays the same no matter how large the image is.
- **PNG tail:** the nonce goes in a private ancillary chunk just before `IEND`. Its CRC-32 covers the chunk type and data, so the CRC must be recomputed for every nonce. The CRC and the fixed 12-byte `IEND` chunk make up the tail.
- **JPEG tail:** the nonce goes in a `COM` segment just before `EOI` (`FF D9`). The `EOI` marker is the tail. The nonce is encoded with bytes that cannot contain `0xFF`, so it never looks like a marker.
- **Exit codes** (guess): 0 = success, 1 = unsupported or malformed input, 2 = usage error, 3 = limit reached.
- The verify step (R-15) guards against adapter bugs. Without it, the tool could produce a file that matches the hash but renders differently.
