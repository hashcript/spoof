---
blueprint:
  type: c4-context
  id: art_c4-context
  version: 1
  status: in_review
  inputs:
    artifacts:
      - { id: art_requirements, version: 1 }
  evidence:
    - "README.md line 2: the actors and neighbours named: a user supplying 'an image and an arbitrary hexstring', an output file, and 'image viewers'"
    - "README.md: no server, network service, database or third-party API is mentioned"
  guesses:
    - "spoof is a local command-line tool with no network dependencies (see Q-07)"
    - "The output's hash is checked by some external hash tool or system that only looks at a hex prefix (see Q-01, Q-02)"
---

# spoof: System context (C4 level 1)

```mermaid
flowchart LR
    user["👤 User<br/><i>Person</i><br/>Runs spoof with an image<br/>and a hexstring"]
    spoof["spoof<br/><i>Software system (CLI)</i><br/>Writes an adjusted image whose<br/>hash starts with the hexstring<br/>and whose pixels are unchanged"]
    fs[("Local filesystem<br/><i>External</i><br/>Source image in,<br/>spoofed image out")]
    viewer["Image viewers<br/><i>External software</i><br/>Browsers, OS previewers, editors"]
    hasher["Hash consumers<br/><i>External software</i><br/>sha512sum, dedup / integrity<br/>systems that check a digest"]

    user -- "spoof &lt;hex&gt; in.png out.png<br/>(argv, stdout/stderr, exit code)" --> spoof
    spoof -- "reads source, writes output<br/>(file I/O)" --> fs
    user -- "opens output" --> viewer
    viewer -- "reads output file" --> fs
    hasher -- "hashes output file" --> fs
    user -. "compares digest prefix" .-> hasher
```

## Notes

- **The system boundary is one local process.** The README mentions no server, network, account or storage beyond the image files themselves.
- **Image viewers do not talk to spoof.** They are the acceptance oracle: the output must render exactly like the input in them (R-04).
- **Hash consumers** (guessed, Q-01) are whatever checks the digest prefix. The README cut-off leaves their identity and the algorithm unknown (Q-02).
- Trust boundary: the input image is **untrusted data**, because it may come from anywhere. The user and the local filesystem are trusted. See `nfr-and-threats.md`.
