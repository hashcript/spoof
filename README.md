# spoof
 An image and an arbitrary hexstring and outputs an adjusted file that displays identically to the human eye (when opened in image viewers) 

`spoof` takes a PNG or JPEG image and a hexstring. It writes a copy of the image whose **SHA-512 digest starts with that hexstring**. The copy decodes to exactly the same pixels as the original.

It works by adding a small block that image viewers ignore, just before the end of the file: a private `spOf` chunk in PNG files, or a `COM` segment holding `spoof:<counter>` in JPEG files. It then tries counter values until the file's hash starts with the hexstring. All other bytes, including metadata such as EXIF, are kept unchanged. The design is described in [`docs/`](docs/).

## Install

You need Go 1.22 or newer. The tool uses only the standard library.

```sh
go install github.com/hashcript/spoof/cmd/spoof@latest
# or, from a clone of this repository:
go build -o spoof ./cmd/spoof
```

To build for other platforms, set `GOOS` and `GOARCH`, e.g. `GOOS=windows GOARCH=amd64 go build ./cmd/spoof`.

## Usage

```
spoof [flags] <hexstring> <input> <output>
```

```sh
$ spoof 0x24ab photo.png photo-24ab.png
24ab5f0c…                          # full SHA-512 of photo-24ab.png
$ shasum -a 512 photo-24ab.png
24ab5f0c…  photo-24ab.png
```

- **hexstring**: the digits the digest must start with. A leading `0x` is optional, case does not matter, and an odd number of digits is allowed.
- **input**: a PNG or JPEG file. It is never modified. The format is detected from the file's content, not its extension.
- **output**: where the adjusted file is written. spoof writes to a temporary file and renames it into place, so a partial file is never left behind. It refuses to write over the input.

On success, spoof prints the output file's full SHA-512 digest in hex on stdout. Before writing, it re-hashes the output and checks that it decodes to the same pixels as the input.

| Flag | Default | Meaning |
|---|---|---|
| `--timeout` | `1h` | Give up after this long. `0` means no limit. |
| `--max-digits` | `12` | Refuse hexstrings longer than this many digits (at most 128). |
| `--workers` | one per CPU | Number of parallel search goroutines. With `--workers 1` the output is the same on every run. |
| `--force` | off | Overwrite the output file if it already exists. |

Flags may come before or after the positional arguments.

### Exit codes

| Code | Meaning |
|---|---|
| 0 | Success; the digest was printed. |
| 1 | Unsupported, malformed or too-large input; I/O or verification failure. |
| 2 | Usage error: bad hexstring or flags, output is the input, or output exists without `--force`. |
| 3 | No match found within `--timeout`. |
| 130 | Interrupted (Ctrl-C). |

### How long does it take?

Each extra digit makes the search 16 times longer: on average an n-digit prefix needs 16ⁿ attempts. Each attempt hashes only the last block or two of the file, so the image's size does not affect the speed. On a typical multi-core laptop, up to 5 digits are near instant, 6 digits take about a second, and 8 digits take minutes. 10 digits take hours, and spoof warns above 10. Longer prefixes are impractical.

### Things to know

- Only a **prefix** of the digest is matched. Matching a full SHA-512 digest is computationally infeasible.
- The output is 28 bytes (PNG) or 26 bytes (JPEG) larger than the input. Running spoof on its own output replaces the earlier block, so blocks do not stack.
- Metadata is kept as it is, including any GPS location or camera serial number stored in EXIF. Remove it with another tool first if you plan to share the image.
- Inputs that are not well-formed are rejected: bad PNG CRCs, data after the PNG `IEND` or JPEG `EOI` (such as the extra images some phones append), or files over 256 MiB or 2²⁷ pixels.
- Other formats (GIF, WebP, HEIC, …) are not supported.

## Development

```sh
go test ./...                                                          # offline; generates its own test images
go test -run XXX -fuzz FuzzParsePNG -fuzztime 30s ./internal/format    # fuzz the PNG parser
go test -run XXX -fuzz FuzzParseJPEG -fuzztime 30s ./internal/format   # fuzz the JPEG parser
```

| Package | Role |
|---|---|
| `cmd/spoof` | Command-line entry point: arguments, orchestration, exit codes |
| `internal/prefix` | Hexstring parsing and nibble-wise digest matching |
| `internal/format` | Format detection and the PNG and JPEG adapters, which build the nonce slot |
| `internal/search` | Parallel nonce search, reusing the hash state of the unchanged head of the file |
| `internal/verify` | Checks the digest and that the decoded pixels are identical before writing |
| `internal/output` | Output path checks and the atomic write |
