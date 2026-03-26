# Design Decisions

## Summary

**Domain Model**

| #   | Question                                                                    | Decision                                           |
| --- | --------------------------------------------------------------------------- | -------------------------------------------------- |
| D1  | How should duplicate keys be counted for total overlap (multiplicity rule)? | `m × n` (cartesian product) per shared key, summed |
| D2  | Should keys be treated as strings or normalised integers?                   | TBD                                                |
| D3  | How are multi-column CSV files handled — which column is the key?           | `--key-columns` required flag, comma-separated column names, no default |

**Algorithm**

| #   | Question                                                             | Decision |
| --- | -------------------------------------------------------------------- | -------- |
| D4  | In-memory hash map vs streaming/external approach for large files?   | Streaming via `KeyIterator` returning `[][]string` batches — never bulk load |
| D5  | Exact counts vs probabilistic approximation (HyperLogLog / MinHash)? | TBD — algorithm implementation detail, resolved per `IntersectionAlgorithm` type |
| D6  | Single-pass vs multi-pass over the files?                            | Single-pass per dataset |
| D9  | Should the intersection algorithm be pluggable?                      | Yes — `IntersectionAlgorithm` interface, implementations swapped via config |
| D10 | Should connectors stream sequentially or in parallel?                | Parallel — one goroutine per connector, algorithm owns concurrency |
| D11 | How should the algorithm manage memory for large frequency maps?     | Configurable caching strategy per algorithm — in-memory, spill-to-disk, or probabilistic |

**System Boundaries**

| #   | Question                                                   | Decision |
| --- | ---------------------------------------------------------- | -------- |
| D7  | CLI argument parsing — positional args vs flags?           | YAML config file via `--config`, with shorthand positional args for CSV convenience |
| D8  | Output format — plain text table vs structured (JSON/CSV)? | `ResultWriter` interface — stdout table by default, pluggable |

---

## Domain Model

## D1: Total overlap multiplicity rule

**Decision:** `m × n` (cartesian product) per shared key, summed across all shared keys.

**Context:** The spec example resolves to `m × n`, not `min(m, n)`. Working through the example confirms this:

```
Key A: 1×1 = 1
Key C: 1×2 = 2
Key D: 2×1 = 2
Key F: 2×3 = 6
Total = 11  ✓
```

**Alternatives considered:**

Using `Dataset A: A B C D D E F F` and `Dataset B: A C C D F F F X Y`:

```
Shared key counts:
  A: 1 in A, 1 in B
  C: 1 in A, 2 in B
  D: 2 in A, 1 in B
  F: 2 in A, 3 in B

m × n:       (1×1) + (1×2) + (2×1) + (2×3) = 1 + 2 + 2 + 6 = 11  ← spec answer
min(m, n):   min(1,1) + min(1,2) + min(2,1) + min(2,3) = 1 + 1 + 1 + 2 = 5
m + n:       (1+1) + (1+2) + (2+1) + (2+3) = 2 + 3 + 3 + 5 = 13
```

- `m × n` (cartesian product) — counts every record-pair match across both files; chosen. Real world: a retailer and a bank both have records for the same address. The retailer has 2 records for it (e.g. two household members who are customers) and the bank has 3 (three account holders at that address). There are 6 possible retailer-bank record pairings for that address — `m × n = 6`. If instead those 2 retailer records are the same person entered twice (a duplicate), the result is still 6 — which is now inflated. This is intentional: a large divergence between total overlap and distinct overlap is the signal that duplicates exist and the data needs cleaning before any analysis is trusted.
- `min(m, n)` — models exclusive assignment where each record can only be matched once. Real world: a recruitment platform has 2 candidates with a given skill and 3 job openings requiring that skill. You can only place 2 candidates before you run out of people — `min(2,3) = 2`.
- `m + n` (sum of occurrences) — counts total occurrences of each shared key across both files combined. Real world: a fraud detection team wants to know how many times a suspicious postcode appears across two transaction logs in total — not how many pairs match, just the raw volume. If postcode `SW1A 1AA` appears 5 times in one log and 8 times in another, `m + n = 13` tells you how many transactions reference that postcode across both sources combined.

**Why:** `m × n` represents the number of row pairs across the two files that share the same key — equivalent to a join cardinality. This is meaningful in InfoSum's context: a person appearing twice in dataset A and three times in dataset B represents 6 linkable record pairs. It also serves as a data quality signal: in a clean dataset total overlap equals distinct overlap; significant divergence indicates duplicate records.

---

## D2: Key normalisation — string vs integer

**Decision:** TBD

**Context:** UDPRN values appear to be 8-digit numeric strings. The sample data contains values with leading zeros (e.g. `08034283`). Treating as integer would silently strip the leading zero and could cause incorrect joins if one file stores `08034283` and another stores `8034283`.

**Alternatives considered:**

- Store as raw string: preserves leading zeros, no normalisation ambiguity
- Parse as integer: loses leading zeros unless re-padded; risky unless the spec guarantees consistent formatting
- Normalise to zero-padded 8-char string: handles mixed formats but requires knowing the canonical width

**Why:** TBD — needs confirmation on whether leading zeros are semantically significant (OQ2).

---

## D3: Multi-column CSV handling

**Decision:** Support one or more named key columns, specified at runtime via a `--key-columns` flag (comma-separated column names). When multiple columns are specified, their values are concatenated with a delimiter to form a composite key string. The rest of the algorithm is unchanged.

**Context:** The current sample files are single-column CSVs with a `udprn` header, but the platform is designed to handle richer datasets where a row may be identified by multiple columns (e.g. `udprn`, `email`, `loyalty_card_id`). The solution must be configurable without code changes.

**Alternatives considered:**

- Always use the first column as the key — simple but not extensible; breaks on multi-column or reordered files
- Allow a `--key-column` flag (single column name) — handles the current data but requires code change to support composite keys later
- Allow `--key-columns` (comma-separated list) — handles both single and composite keys; chosen
- Require the key column to be named `udprn` — brittle, breaks for any other key type

**Why:** Composite key support is a stated requirement of the platform context. Making it a required flag keeps the algorithm generic and forces the caller to be explicit — key extraction is a configurable pre-processing step, and the intersection logic operates on opaque strings regardless of how many source columns were combined. There is no default: omitting `--key-columns` is a hard error. A default (first column, all columns) risks silently wrong results if the file structure changes or contains non-key columns; in a privacy-sensitive platform, silent incorrectness is unacceptable.

---

## D4: Streaming via KeyIterator

**Decision:** Both datasets are consumed via a `KeyIterator` interface that returns batches of `[][]string` — never bulk loaded into memory.

```go
type KeyIterator interface {
    NextBatch() (keys [][]string, done bool, err error)
    Close() error
}
```

Each inner `[]string` is one row's key field values, one element per configured key column. The connector extracts the correct fields from the source format (CSV, JSON, database row) and returns them in column order. The algorithm joins each `[]string` with a null byte (`\x00`) to form a frequency map key — no delimiter ambiguity, no collision risk.

**Context:** The ingestion layer must support sources beyond local files (REST API, database, SFTP, JSON). Bulk loading is incompatible with remote sources that can only be iterated. Returning `[]string` per row rather than a pre-joined string keeps the interface format-agnostic and avoids baking a delimiter assumption into the connector contract.

**Alternatives considered:**

- Return `string` per key (pre-joined in the connector) — leaks the delimiter concern into the connector; each connector must agree on the same separator; ruled out in favour of `[][]string`
- Bulk load both datasets into memory — incompatible with remote connectors and large datasets; ruled out
- Sort-merge join on disk — O(1) extra memory, exact, but requires temp disk space and significant complexity; deferred unless in-memory approach proves insufficient

**Why:** `[][]string` keeps the connector interface clean and format-agnostic. The delimiter is a single implementation detail inside the algorithm, not a contract between components.

---

## D5: Exact vs approximate counts

**Decision:** TBD

**Context:** The spec explicitly raises the approximation question: "If approximations are used, ensure the accuracy of the values is appropriately represented." This is only relevant if D4 resolves to a probabilistic approach.

**Alternatives considered:**

- Exact counts via hash map or sort-merge: always correct, memory/time bounded
- HyperLogLog for distinct counts: ~1–2% error, very low memory (a few KB regardless of cardinality)
- MinHash / Jaccard estimation for overlap: estimates set similarity, not the raw overlap count directly

**Why:** TBD — if files fit in memory, exact is preferable and simpler to explain. Approximation adds complexity and requires communicating error bounds in output.

---

## D6: Single-pass per dataset

**Decision:** One pass per dataset — stream dataset A into a frequency map, then stream dataset B once to compute all four metrics.

**Context:** Total key count and total overlap both require knowing frequencies. Distinct count and distinct overlap require only presence. All four can be derived from a single frequency map pass per dataset.

**Alternatives considered:**

- Two passes per dataset (one for total count, one for distinct) — reads each source twice; wasteful for large files and incompatible with non-seekable remote sources
- Single pass with frequency map — chosen; collects all information needed in one pass per dataset

**Why:** Non-seekable remote sources (API streams, database cursors) cannot be rewound. A single-pass design is required for the connector abstraction to work correctly across all source types.

---

## D7: Configuration via YAML config file

**Decision:** Dataset sources, key columns, and output destination are specified via a YAML config file passed with `--config`. A shorthand positional form is also supported for the common case of two local CSV files.

**Full config form:**

```yaml
datasets:
  - connector: csv
    path: data/A_f.csv
    page_size: 1000
    max_error_rate: 0.05
  - connector: rest
    url: https://api.example.com/records
    auth_header: Authorization
    auth_token: Bearer xyz
    page_size: 1000
    max_error_rate: 0.05

key_columns: [udprn, email]

algorithm:
  type: pairwise
  cache:
    strategy: in_memory
    max_memory_mb: 512
    spill_dir: /tmp

output:
  writer: stdout
```

**Shorthand form (CSV only):**

```sh
program --key-columns udprn A_f.csv B_f.csv
```

Internally the shorthand constructs an equivalent CSV config — it is a convenience wrapper, not a separate code path.

**Alternatives considered:**

- Positional arguments only — works for CSV file paths but cannot express connector type, auth, pagination config, or output destination without an explosion of flags
- Named flags only (`--file-a`, `--file-b`, `--key-columns`, `--output`) — manageable for CSV but does not scale to REST or database connectors which need arbitrarily many parameters
- YAML config file — scales to any connector type and keeps all configuration in one place; chosen

**Why:** Different connectors have fundamentally different configuration shapes. A flag-based interface cannot accommodate this without becoming unwieldy. A config file makes each connector's parameters explicit and self-documenting, and mirrors how real data pipeline tools are configured.

---

## D8: Output via ResultWriter interface

**Decision:** Results are written via a `ResultWriter` interface. The default writer formats output as a human-readable table to stdout. The output destination is configurable via `--output`.

**Context:** Mirroring the `KeyIterator` connector pattern on the input side, the output side should be equally pluggable. Hardcoding stdout couples the algorithm to a single output destination and makes it impossible to write results to a file, database, or API without changing the algorithm.

**Alternatives considered:**

- Hardcode stdout — simple but not extensible; rules out file, API, and database output without algorithm changes
- Plain text table to stdout (default writer) — human-readable, matches the "display" framing in the spec; chosen as the default
- JSON writer — machine-readable, useful for piping to other tools or storing results; available as a future writer implementation
- Both via flag — handled naturally by the `ResultWriter` interface; no special casing needed

**Why:** Same reasoning as `KeyIterator` — the algorithm should have no opinion about where results go. A `ResultWriter` keeps that concern isolated in a single swappable component.

---

## D9: Pluggable IntersectionAlgorithm

**Decision:** The intersection computation is abstracted behind an `IntersectionAlgorithm` interface. The implementation is selected via the `algorithm.type` field in the YAML config.

```go
type IntersectionAlgorithm interface {
    Compute(datasets []KeyIterator) (IntersectionResult, error)
}
```

**Current implementation:** `PairwiseAlgorithm` — accepts exactly two datasets, builds frequency maps, computes the four metrics.

**Future implementations** (not in scope for this iteration but the interface accommodates them):
- `NWayAlgorithm` — computes all pairwise and multi-way region breakdowns for N datasets; required for Venn diagram output
- `ApproximateAlgorithm` — uses HyperLogLog for distinct counts and MinHash for overlap estimation; suited for very large datasets where exact counts are not feasible

**Why D5 is now an implementation detail:** exact vs approximate counting is no longer a system-level decision — it is a property of whichever algorithm implementation is selected. The `PairwiseAlgorithm` is exact. An `ApproximateAlgorithm` would be probabilistic. The rest of the system does not change.

**Alternatives considered:**

- Hardcode the pairwise algorithm — simple for the current spec but forces code changes to support N-way comparison or approximation; ruled out
- Flag-controlled approximation mode — adds conditional logic throughout the algorithm rather than isolating it in a separate implementation; ruled out
- Pluggable interface — chosen; each algorithm implementation owns its own memory strategy, parallelism, and accuracy tradeoffs

**Why:** The three-layer architecture (`KeyIterator → IntersectionAlgorithm → ResultWriter`) gives each layer a single, well-defined responsibility. The algorithm layer owns all computation concerns — memory, parallelism, accuracy — without any of those concerns leaking into the connector or writer layers.

---

## D11: Algorithm caching strategy

**Decision:** The algorithm's memory strategy is configurable via an `algorithm.cache` block in the YAML config. Three strategies are supported, selected based on dataset size and accuracy requirements:

| Strategy       | Memory usage                  | Accuracy  | When to use                                      |
| -------------- | ----------------------------- | --------- | ------------------------------------------------ |
| `in_memory`    | O(distinct keys per dataset)  | Exact     | Default — datasets fit within `max_memory_mb`   |
| `spill_to_disk`| O(batch size) working memory  | Exact     | Frequency map exceeds `max_memory_mb`            |
| `probabilistic`| O(1) — kilobytes regardless   | Approximate (~1-2% error) | Distinct key count is too large for disk spill |

**Config:**

```yaml
algorithm:
  type: pairwise
  cache:
    strategy: in_memory     # in_memory | spill_to_disk | probabilistic
    max_memory_mb: 512      # threshold before spill_to_disk kicks in
    spill_dir: /tmp         # only used when strategy: spill_to_disk
```

**How each strategy works:**

- `in_memory` — frequency map held entirely in RAM. Each goroutine writes to its own map concurrently with no locking. Fast, simple, exact. Default for the current implementation.
- `spill_to_disk` — frequency map written to a temp file in `spill_dir` in sorted key order. When the in-memory portion exceeds `max_memory_mb`, it is flushed and sorted. Final merge produces exact counts. More complex but RAM-bounded.
- `probabilistic` — uses HyperLogLog for distinct counts and MinHash for overlap estimation. Sub-linear memory, approximate results. Output includes error bounds. Resolves D5 for this strategy.

**Why the algorithm owns this:** caching strategy is a computation concern, not a connector or writer concern. The connector streams rows regardless of what the algorithm does with them. The writer formats results regardless of how they were computed. Keeping the strategy in the algorithm config block means it can be tuned independently of source or output format.

**Current implementation:** `in_memory` only. `spill_to_disk` and `probabilistic` are deferred.
