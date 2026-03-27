# Development Sequence

How to split the project into deliverable slices and where to start. Each slice builds on the previous and should leave the codebase in a working state. The three-layer architecture (KeyIterator → IntersectionAlgorithm → ResultWriter) drives the sequencing — the entry point is built first so that every subsequent layer has real input to work against.

---

## Slice 1 — Project Skeleton

**What:** Initialise the repo, set up the basic Makefile (`build`, `test`), and wire up GitHub Actions to run `go build` and `go test` on every push. Nothing works yet — this is scaffolding only.

**Why here:** CI from day one means every subsequent slice has a green build to work against. The pipeline should never be in a broken state that gets fixed at the end.

**Done when:** A `go build` passes. `go test ./...` runs (and passes with no tests). GitHub Actions reports green on push.

---

## Slice 2 — KeyIterator (CSV Connector)

**What:** Implement the CSV connector as a KeyIterator. Read a CSV file, resolve columns by name from the header row, return batches of keys as raw strings, and report ConnectorStats.

**Why here:** The KeyIterator is the entry point for all data. Building it first means the algorithm has real streamed input to work against from the start — no mocks, no hardcoded data.

**Done when:** A CSV file can be streamed batch by batch. Column resolution works by name regardless of column order. Leading zeros are preserved. ConnectorStats reflects accurate row counts.

---

## Slice 3 — Core Algorithm

**What:** Implement the frequency map approach. Feed two KeyIterators into the algorithm, build frequency maps, compute all four counts: total count per dataset, distinct count per dataset, distinct overlap, total overlap.

**Why here:** The KeyIterator is proven. Now build the computation that depends on it. Validating the formula against the spec example before adding any further complexity reduces the risk of building a scaffold around a broken core.

**Done when:** The spec example (A B C D D E F F vs A C C D F F F X Y → distinct overlap 4, total overlap 11) passes as a unit test. Running against `data/A_f.csv` and `data/B_f.csv` produces four counts that are manually verifiable.

**Risk:** The total overlap formula is `m × n` per shared key, summed — not `min(m, n)` and not `m + n`. Worth asserting against the spec example before moving on.

---

## Slice 4 — ResultWriter

**What:** Implement the stdout writer. Format and output all six fields from IntersectionResult: total count per dataset, distinct count per dataset, distinct overlap, total overlap, skipped row counts from ConnectorStats, and ErrorBoundPct when non-zero.

**Why here:** The algorithm produces a result — now make it visible. The writer is thin but it is the only thing the caller ever sees. Implementing it here keeps the output contract explicit before config and wiring are added.

**Done when:** All six fields appear in output with correct labels. ErrorBoundPct is omitted when zero. Skipped row counts from ConnectorStats are included per source.

---

## Slice 5 — Config and Wiring

**What:** Replace hardcoded paths with a YAML config file parsed via `--config`. Wire the three layers together through config: connector construction, algorithm selection, writer configuration. Expand `${VAR}` environment variable references at parse time.

**Why here:** The three layers work independently. Now connect them through the config plumbing that makes the tool configurable and runnable as a single command.

**Done when:** `./program --config config/default.yaml` works end-to-end. Missing `--config`, missing `key_columns`, unset environment variable references, and invalid algorithm type all produce hard errors on startup with a non-zero exit.

---

## Slice 6 — Hardening and Tests

**What:** Add all failure modes at each layer boundary and complete the test suite across all layers. Soft fails at the connector: empty key fields, malformed rows, fewer fields than expected — recorded in ConnectorStats, not fatal. Hard fails: world-readable input files, missing columns, file not found — rejected before any rows are read. Config file permissions checked at startup. End-to-end tests run the binary against real fixture files.

**Why here:** The happy path works. Now protect each boundary against the failure modes that produce quietly wrong results or silent data loss. These failures are invisible in the output — tests are the only protection. Hardening and tests are done together because hardening without tests to prove it works isn't really done.

**Done when:** All scenarios in `06-testing.md` have passing tests. World-readable input files are rejected before the first row is read. CI runs the full suite on every push.

---

## Slice 7 — Large File Strategies

**What:** Implement `spill_to_disk` for datasets that exceed available RAM, and `pairwise_approximate` using HyperLogLog and MinHash for datasets where exact computation is not practical. Strategy selected via `algorithm.caching_strategy` in config.

**Why here:** Premature optimisation. The correctness of the exact in-memory algorithm must be established first. Only then is it worth adding complexity for scale.

**Done when:** `spill_to_disk` produces the same result as `in_memory` on the existing fixture files. `pairwise_approximate` output includes a non-zero `ErrorBoundPct`. Sizing thresholds in D10 are validated against real memory usage.

**Risk:** `pairwise_approximate` changes the output contract — `ErrorBoundPct` must be non-zero and the caller must be able to distinguish an estimate from an exact result.

---

## Slice 8 — Packaging

**What:** Finalise the Dockerfile, add `docker-build` and `docker-run` Makefile targets, extend CI with Docker build and trivy scanning, and write the README.

**Why here:** The Makefile and CI skeleton exist from Slice 1 — this slice finalises them. Documentation written last reflects the actual implementation.

**Done when:** A reviewer with only Docker installed can clone the repo, run `make docker-build && make docker-run`, and see correct output against the provided data files following only the README.

---

## What to Defer

- **REST connector:** defer until after Slice 2 — the CSV connector validates the KeyIterator interface. REST is a second implementation of the same interface.
- **N-way intersection (more than two datasets):** defer unless explicitly confirmed in scope — the current design is N-dataset safe but the challenge specifies two.
- **Horizontal scaling:** out of scope — requires dedicated research and design. See `09-deployment.md`.
- **Progress indicators / verbose mode:** defer until large file strategies are implemented — only useful for large-file runs.
