---
blueprint:
  type: requirements
  id: art_requirements
  version: 1
  status: in_review
  inputs:
    artifacts: []
  evidence:
    - "README.md:1-2 — the entire system description: 'An image and an arbitrary hexstring and outputs an adjusted file that displays identically to the human eye (when opened in image viewers)'"
  guesses:
    - "The hexstring is meant to be recoverable from the output file, not merely present in it (see Q-01)"
    - "The tool is a command-line application (see Q-05)"
    - "The output is a new file; the input image is left untouched (see Q-08)"
    - "PNG is the first supported format (see Q-02)"
---

# Requirements for spoof

The repository has no code; the single source of truth is the one-line description in `README.md:2`. Requirements below are numbered, one line each; every functional requirement cites that line. Where the description is silent, the requirement is marked **(guessed)** and linked to a question in `questions.yaml`.

## Functional requirements

- **R-01** — The tool accepts an input image file as a parameter. *(README.md:2, "An image")*
- **R-02** — The tool accepts an arbitrary hexstring as a parameter. *(README.md:2, "an arbitrary hexstring")*
- **R-03** — The tool produces an adjusted output image file. *(README.md:2, "outputs an adjusted file")*
- **R-04** — The output file, when opened in an image viewer, displays identically to the human eye as the original image. *(README.md:2, "displays identically to the human eye (when opened in image viewers)")*
- **R-05** — The output file carries the hexstring within it. **(guessed, Q-01)** *(README.md:2, name and purpose imply the hexstring survives into the output)*
- **R-06** — The hexstring can be extracted back from the output file, round-tripping R-02. **(guessed, Q-01, Q-06)** *(README.md:2, implied)*
- **R-07** — The input image file is never modified in place; the result is written to a separate output path. **(guessed, Q-08)** *(README.md:2, "outputs an adjusted file" — implies a new file)*
- **R-08** — The tool validates the hexstring (even length, hexadecimal characters only) and rejects invalid input with a clear error. **(guessed, Q-07)** *(README.md:2, "arbitrary hexstring" — at minimum it must be a hexstring)*
- **R-09** — The tool detects the input image format and either processes it or rejects it with a clear error listing supported formats. **(guessed, Q-02)** *(README.md:2, "An image")*
- **R-10** — If the hexstring exceeds what the chosen image format can carry, the tool fails with a clear error rather than truncating. **(guessed, Q-07)** *(README.md:2, "arbitrary" — arbitrary length must have a defined behavior)*

## Non-functional requirements

- **N-01** — *Visual fidelity:* the decoded pixel data of the output is identical to the original — rendering equivalence must hold by construction, not by visual approximation. *(README.md:2, "displays identically")*
- **N-02** — *Losslessness:* the operation is reversible with respect to the image content: the original pixels can always be recovered from the output. *(README.md:2)*
- **N-03** — *No runtime dependencies:* the tool runs from a standard interpreter with no third-party packages to install. **(guessed, Q-04)**
- **N-04** — *Performance:* processing is O(size of file) with constant memory beyond streaming buffers; a typical image embeds in under a second. **(guessed)**
- **N-05** — *Portability:* the tool runs unchanged on Linux, macOS and Windows. *(README.md:2 says "image viewers" generatively, implying any desktop platform)*
- **N-06** — *Robustness:* malformed images or hexstrings produce a non-zero exit code and a diagnostic message, never a corrupt output file. **(guessed)**
- **N-07** — *Testability:* embedding followed by extraction is a pure round-trip property testable without a human in the loop. **(guessed, Q-06)*
