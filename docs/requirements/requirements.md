---
blueprint:
  type: requirements
  id: art_requirements
  version: 1
  status: in_review
  inputs:
    artifacts: []
  evidence:
    - "README.md line 1: project name 'spoof'"
    - "README.md line 2: 'An image and an arbitrary hexstring and outputs an adjusted file that displays identically to the human eye (when opened in image viewers)'"
    - "git ls-files: README.md is the only tracked file; there is no code, build config, licence or test yet (single 'Initial commit')"
  guesses:
    - "The hexstring is the required prefix of a cryptographic hash of the output file; the README sentence is cut off before saying what the hexstring is for (see Q-01)"
    - "The hash algorithm is SHA-512 (see Q-02)"
    - "The tool is a command-line program taking hexstring, input path and output path (see Q-05)"
    - "Supported input formats are PNG and JPEG (see Q-03)"
    - "'Displays identically' means decoded pixels are bit-for-bit identical, not merely perceptually similar (see Q-04)"
    - "Prefix lengths up to 8 hex digits must finish in reasonable time on a laptop (see Q-06)"
---

# spoof: Requirements

The repository has no code yet. The only source is the one-sentence description in `README.md`. That sentence starts with a stray space, has no verb, and stops before it says what the hexstring is for. Every requirement below that goes beyond the literal README text is marked **(guess)** and linked to a question in `questions.yaml`.

## Functional requirements

| ID | Requirement | Source |
|----|-------------|--------|
| R-01 | The tool accepts an input image file. | README.md L2 ("An image") |
| R-02 | The tool accepts an arbitrary hexstring from the user. | README.md L2 ("an arbitrary hexstring") |
| R-03 | The tool writes an adjusted output file derived from the input image. | README.md L2 ("outputs an adjusted file") |
| R-04 | The output file displays identically to the human eye in ordinary image viewers. **(guess: identical decoded pixels, Q-04)** | README.md L2 ("displays identically … when opened in image viewers") |
| R-05 | The output file's hash starts with the given hexstring. **(guess: Q-01; SHA-512, Q-02)** | README.md L2 (sentence truncated) |
| R-06 | The tool is invoked from the command line as `spoof <hexstring> <input> <output>`. **(guess, Q-05)** | README.md (no interface specified) |
| R-07 | The hexstring may have an optional `0x` prefix, is case-insensitive, and may have an odd number of digits, which are then matched as nibbles. **(guess, Q-05)** | README.md L2 ("arbitrary hexstring") |
| R-08 | The tool detects the input format from its magic bytes and supports at least PNG and JPEG. **(guess, Q-03)** | README.md L2 ("image", "image viewers") |
| R-09 | The tool rejects unsupported or malformed input with a clear error and a non-zero exit code. **(guess)** | Implied by R-08 |
| R-10 | The tool never modifies the input file. It writes the output to a separate path. **(guess)** | README.md L2 ("outputs an adjusted file") |
| R-11 | On success, the tool prints the digest of the output file so the user can check it. **(guess)** | Implied by R-05 |
| R-12 | The tool keeps existing image metadata (e.g. EXIF) and only adds what the adjustment needs. **(guess, Q-08)** | README.md L2 ("adjusted", not "re-encoded") |

## Non-functional requirements

| ID | Requirement | Source |
|----|-------------|--------|
| R-13 | Performance: the search uses every CPU core and avoids rehashing the unchanged part of the file on each attempt. Expected work is 16^n attempts for an n-digit prefix. **(guess, Q-06)** | Follows from R-05 |
| R-14 | Bounded runtime: the user can set a time or prefix-length limit, and the tool stops cleanly with a message when it hits it. **(guess, Q-06)** | Follows from R-05 / R-13 |
| R-15 | Correctness: before writing the output, the tool checks that the output's digest has the required prefix and that the output decodes to the same pixels as the input. **(guess)** | Follows from R-04 / R-05 |
| R-16 | Portability: the tool ships as one self-contained executable for Linux, macOS and Windows, with no runtime dependencies. **(guess, Q-07)** | README.md (no platform specified) |
| R-17 | Offline and local only: no network access and no telemetry. **(guess)** | README.md (no external systems mentioned) |
| R-18 | Robustness: the parser handles hostile or corrupted input safely, with bounded memory and no crashes on bad chunk or segment lengths. **(guess)** | Follows from R-01 / R-09 |
| R-19 | Safe output: the output is written atomically (temp file, then rename). Writing over the input path is refused. **(guess)** | Follows from R-10 |
