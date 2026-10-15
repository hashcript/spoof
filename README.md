# spoof

An image and an arbitrary hexstring and outputs an adjusted file that
displays identically to the human eye (when opened in image viewers).

`spoof` embeds a hexstring into a PNG image without touching a single
rendered byte: the payload rides in an ancillary chunk that image viewers
ignore, so the output is pixel-for-pixel identical to the original — by
construction, not by visual approximation. The hexstring can be extracted
back out at any time, making the whole operation a lossless round trip.

## Requirements

- Python 3.6 or newer. No third-party packages — the tool is a single
  standard-library-only file (see `docs/adr/0001-python-stdlib-cli.md`).
- Input images must be PNG (the first supported format; JPEG and GIF are
  planned behind the same interface — see `docs/adr/0002-png-first-format-support.md`).

## Usage

Run the tool directly (no install step):

```sh
python3 spoof.py embed cat.png deadbeef
# -> writes cat_spoof.png

python3 spoof.py embed cat.png 0xDEADBEEF -o tagged.png
python3 spoof.py embed cat.png "de ad be ef" -o tagged.png --force

python3 spoof.py extract cat_spoof.png
# -> prints the hexstring (lowercase, e.g. deadbeef) to stdout
```

### `embed IMAGE HEXSTRING`

Embeds the hexstring into a copy of the image.

- `HEXSTRING` must contain only hexadecimal digits (`0-9`, `a-f`, `A-F`) in
  pairs. Whitespace is ignored and an optional `0x`/`0X` prefix is allowed.
- `-o, --output PATH` — where to write the result. Default:
  `<input>_spoof.png`. The input file is never modified.
- `--force` — overwrite the output file if it already exists (by default
  the tool refuses, rather than clobber an existing file).

After writing, the tool re-parses its own output and verifies the payload
round-trips before reporting success.

### `extract IMAGE`

Prints the carried hexstring to stdout (lowercase; the value round-trips,
the letter case does not) and exits `0`. If the image carries no payload —
or is not a valid PNG — it exits non-zero with a diagnostic on stderr.

Exit codes: `0` on success, `1` on any error (invalid hexstring, unsupported
or malformed image, oversized payload, missing file). On failure no output
file is written.

## How it works

The payload bytes are stored in a private ancillary PNG chunk named `spOf`,
placed immediately after `IHDR`. PNG viewers ignore ancillary chunks they
do not recognise, so the image data (`IDAT` and everything else a viewer
renders) is copied through byte-for-byte with recomputed-verified CRCs —
"displays identically" is a byte-level guarantee, checkable by a diff, not a
perceptual one (see `docs/adr/0003-lossless-metadata-carriage.md`).

Notes and limits:

- **Reversibility** — deleting the `spOf` chunk recovers the original file
  exactly; `extract` reads the chunk back out.
- **Capacity** — the payload must fit a single PNG chunk (at most
  2^31−1 bytes). Larger payloads fail with a clear error rather than being
  truncated.
- **Re-embedding replaces** — embedding into an image that already carries
  a payload replaces it; it does not stack.
- **This is carriage, not secrecy** — anyone inspecting the chunks can see
  the payload, and re-encoding/upload pipelines may strip ancillary chunks.
- Malformed PNGs (bad CRC, truncation, missing `IEND`, trailing data) are
  rejected; the tool never leaves a partial output file behind.

## Library use

The CLI is a thin shell over an importable module:

```python
import spoof

payload = spoof.parse_hexstring("0xdeadbeef")
spoof.embed("cat.png", payload, "cat_spoof.png")   # writes the copy
spoof.extract("cat_spoof.png")                     # -> b'\xde\xad\xbe\xef'
spoof.detect_format("cat.png")                     # -> "PNG"
```

## Tests

The test suite runs offline with the standard library only:

```sh
python3 -m unittest discover -s tests
```

## Design documents

- `docs/requirements/requirements.md` — numbered functional and
  non-functional requirements
- `docs/adr/` — Python stdlib choice, PNG-first format support,
  lossless metadata carriage
- `docs/architecture/` — C4 context/container diagrams and the
  embed sequence
