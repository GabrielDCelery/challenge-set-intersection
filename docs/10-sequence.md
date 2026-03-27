# Development Sequence

How to split the project into deliverable slices and where to start. Each slice should be shippable and build on the previous. The three-layer architecture (KeyIterator → IntersectionAlgorithm → ResultWriter) provides the natural seam for sequencing — get the algorithm right first, then wire the layers together, then harden each boundary.

---

## Slice 1 — Core Algorithm

**What:** Implement the frequency map approach with hardcoded inputs. Build the IntersectionAlgorithm and ResultWriter layers. Compute all four counts and print them to stdout.

**Why here:** The algorithm is the only thing that matters. Getting the total overlap formula right against the spec example before adding any I/O or config complexity reduces the risk of building a scaffold around a broken core.

**Done when:** The spec example (A B C D D E F F vs A C C D F F F X Y → distinct overlap 4, total overlap 11) passes as a unit test. Running against `data/A_f.csv` and `data/B_f.csv` produces four counts that are manually verifiable.

**Risk:** The total overlap formula is `m × n` per shared key, summed — not `min(m, n)` and not `m + n`. Worth asserting against the spec example before moving on.

---

## Slice 2 — KeyIterator and Config

**What:** Implement the CSV connector as a KeyIterator. Wire the three layers together through YAML config parsed via `--config`. Column resolution by name, batch streaming, ConnectorStats.

**Why here:** The algorithm is proven. Now build the connector layer and the config plumbing that wires everything together. This is the difference between a sketch and a configurable, runnable tool.

**Done when:** `./program --config config/default.yaml` works end-to-end. Config specifies the two input files and `key_columns`. Missing `--config` flag, missing `key_columns`, or a file that does not exist all produce a hard error on startup with a non-zero exit.

**Risk:** Column resolution by name (not index) must be established here — getting this wrong silently produces incorrect results with no downstream indication.

---

## Slice 3 — Connector Hardening

**What:** Handle edge cases at the connector boundary: leading zeros preserved, quoted fields and embedded commas parsed correctly, empty `key_columns` fields treated as soft failures and recorded in ConnectorStats, malformed rows skipped and counted, world-readable input files rejected before any rows are read.

**Why here:** The layers are wired. Now harden the point where raw source data enters the system. Corruption at the connector boundary is invisible downstream — the algorithm has no way to detect it.

**Done when:** Unit tests for the KeyIterator cover all scenarios in `06-testing.md` — leading zeros, quoted fields, empty key fields, malformed rows, missing columns, world-readable file permissions. All pass.

**Risk:** None of these failures are visible in the output — they produce quietly wrong counts. The tests are the only protection.

---

## Slice 4 — Test Suite

**What:** Complete the test suite across all layer boundaries: startup, KeyIterator, IntersectionAlgorithm, ResultWriter, and end-to-end. Each layer tested in isolation using controlled inputs.

**Why here:** The tool is functionally complete. Tests codify the correct behaviour at each boundary before any changes to the internals.

**Done when:** All scenarios in `06-testing.md` have a passing test. CI runs the full suite on every push.

**Risk:** If a bug is found here that changes the algorithm, Slice 1 output may have been wrong — better to find it now than never.

---

## Slice 5 — Large File Strategies

**What:** Implement `spill_to_disk` for datasets that exceed available RAM, and `pairwise_approximate` using HyperLogLog and MinHash for datasets where exact computation is not practical. Strategy is selected via `algorithm.caching_strategy` in config.

**Why here:** Premature optimisation. The correctness of the exact in-memory algorithm must be established first. Only then is it worth adding complexity for scale.

**Done when:** `spill_to_disk` produces the same result as `in_memory` on the existing fixture files. `pairwise_approximate` output includes a non-zero `ErrorBoundPct`. The sizing thresholds in D10 are validated against real memory usage.

**Risk:** `pairwise_approximate` changes the output contract — `ErrorBoundPct` must be non-zero and the caller must be able to distinguish an estimate from an exact result.

---

## Slice 6 — Packaging

**What:** Write the README with build and run instructions. Finalise the Makefile (`build`, `test`, `docker-build`, `docker-run`). Write the Dockerfile. Set up GitHub Actions CI.

**Why here:** Everything else is done. Documentation and packaging written last reflects the actual implementation.

**Done when:** A reviewer with only Docker installed can clone the repo, run `make docker-build && make docker-run`, and see correct output against the provided data files following only the README.

**Risk:** None — this is polish, not logic.

---

## What to Defer

- **REST connector:** defer until after Slice 2 — the CSV connector validates the KeyIterator interface. REST is a second implementation of the same interface.
- **N-way intersection (more than two datasets):** defer unless explicitly confirmed in scope — the current design is N-dataset safe but the challenge specifies two.
- **Horizontal scaling:** out of scope — requires dedicated research and design. See `09-deployment.md`.
- **Progress indicators / verbose mode:** defer until Slice 5 — only useful for large-file runs.
