# Development Sequence

How to split the project into deliverable slices and where to start. Each slice should be shippable and build on the previous.

## Walking Skeleton

A program that accepts two hardcoded file paths, reads both CSVs, and prints four correct counts to stdout — no flags, no error handling, no large-file optimisation. Validates that the core algorithm is right before anything else is built.

---

## Slice 1 — Core Algorithm (In-Memory, Hardcoded Paths)

**What:** Implement the frequency map approach. Read both files into memory. Compute all four counts. Print them to stdout.

**Why here:** The algorithm is the only thing that matters. Getting the total overlap formula right against the spec example before adding any I/O or CLI complexity reduces the risk of building a scaffold around a broken core.

**Done when:** Running against `data/A_f.csv` and `data/B_f.csv` produces four counts that are manually verifiable. The spec example (A B C D D E F F vs A C C D F F F X Y → distinct overlap 4, total overlap 11) passes as a unit test.

**Risk:** Total overlap formula is subtle — `min(m, n)` per key, not `m * n` or `m + n`. Worth asserting against the spec example before moving on.

---

## Slice 2 — CLI Argument Parsing

**What:** Replace hardcoded file paths with runtime arguments. Add usage error handling (wrong number of args, file not found).

**Why here:** The core is proven. Now make it usable. This slice is the difference between a sketch and a runnable tool.

**Done when:** `./program data/A_f.csv data/B_f.csv` works. Running with no args or a missing file prints a clear error to stderr and exits with a non-zero code.

**Risk:** None significant — argument parsing is mechanical.

---

## Slice 3 — Robust CSV Parsing

**What:** Handle edge cases in CSV input: leading zeros preserved, whitespace trimming, blank lines skipped, header row handled correctly, multi-column files supported if key column is configurable.

**Why here:** The algorithm and CLI are working. Now harden the parser so the tool doesn't silently produce wrong results on real-world CSV variation.

**Done when:** Unit tests for the parser cover: leading zeros, blank lines, whitespace in cells, files with no header, files with multiple columns. All pass.

**Risk:** Deciding whether to use a full CSV parser library or a line-splitting approach — a library is safer for quoted fields and commas within values.

---

## Slice 4 — Test Suite

**What:** Write unit tests for the overlap computation and integration tests using fixture CSV files covering all key scenarios from `06-testing.md`.

**Why here:** The tool is functionally complete. Tests codify the correct behaviour before any optimisation changes the internals.

**Done when:** All scenarios in the key scenarios table in `06-testing.md` have a passing test. CI runs the test suite on push.

**Risk:** If a bug is found here that changes the algorithm, Slice 1 output may have been wrong — but better to find it now than never.

---

## Slice 5 — Large File Optimisation (If Required)

**What:** Determine whether the in-memory approach is sufficient for the expected file sizes (OQ1). If not, implement either an external sort-merge approach or a HyperLogLog-based approximation for distinct counts.

**Why here:** Premature optimisation. The correctness of the exact algorithm must be established first. Only then is it worth adding complexity for scale.

**Done when:** The program handles a generated fixture file of the maximum expected size within the acceptable time and memory bounds defined in the NFRs.

**Risk:** If approximation is used, the output format must change to communicate error bounds — this may require revisiting Slice 2's output design.

---

## Slice 6 — Documentation and Packaging

**What:** Write the README with build and run instructions. Add a Makefile or Taskfile. Optionally add a Dockerfile.

**Why here:** Everything else is done. Documentation written last reflects the actual implementation.

**Done when:** A reviewer with the language runtime installed can clone the repo, run `make build && make test`, and run the program against the provided data files following only the README.

**Risk:** None — this is polish, not logic.

---

## What to Defer

- **JSON output flag:** defer until Slice 6 or drop entirely — the spec says "display", and a structured output mode adds complexity without being required.
- **Support for more than two files:** defer unless explicitly confirmed in scope — the spec says "comparison of two datasets."
- **Progress indicators / verbose mode:** defer until Slice 5 (only useful for large-file runs).
- **HyperLogLog / MinHash approximation:** defer until Slice 5 — only needed if exact in-memory approach cannot handle the target file size.
