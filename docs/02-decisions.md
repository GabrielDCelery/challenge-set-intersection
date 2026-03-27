# Design Decisions

## Summary

**Domain Model**

| #   | Question                         | Decision                                                 |
| --- | -------------------------------- | -------------------------------------------------------- |
| D1  | Total overlap multiplicity rule? | `m × n` per shared key, summed                           |
| D2  | Keys as strings or integers?     | Raw strings — leading zeros preserved                    |
| D3  | Which column is the key?         | `key_columns` in YAML config — required, no default      |
| D12 | Empty key field in a row?        | Soft failure — row skipped, recorded in `ConnectorStats` |

**Algorithm**

| #   | Question                                    | Decision                                                     |
| --- | ------------------------------------------- | ------------------------------------------------------------ |
| D4  | Streaming vs bulk load?                     | Stream via `KeyIterator` (`[][]string` batches)              |
| D5  | Single-pass vs multi-pass?                  | Single-pass per dataset                                      |
| D8  | Pluggable algorithm?                        | `IntersectionAlgorithm` interface; type and cache orthogonal |
| D9  | Sequential vs parallel connector streaming? | Parallel — one goroutine per connector                       |
| D10 | Frequency map memory for exact algorithms?  | `in_memory` or `spill_to_disk`; n/a for approximate          |
| D11 | Long-running process control?               | `run.timeout_seconds`; resume deferred                       |

**System Boundaries**

| #   | Question                      | Decision                                                           |
| --- | ----------------------------- | ------------------------------------------------------------------ |
| D6  | Config mechanism?             | YAML via `--config`; mandatory, no shorthand                       |
| D7  | Output format?                | `ResultWriter` interface; stdout table by default                  |
| D13 | Secret injection into config? | Environment variable expansion — `${VAR}` resolved via `os.Getenv` |

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

**Decision:** Keys are stored and compared as raw strings. Leading zeros are preserved. No numeric parsing is performed.

**Context:** UDPRN is a Royal Mail identifier for a unique delivery point — it is an 8-digit code, not a number. `08034283` and `8034283` refer to different delivery points. Parsing as integer would silently strip the leading zero, corrupting the key and producing either a false non-match or a match against a completely different address. In a privacy-sensitive platform where accurate population overlap is the entire purpose, silent data corruption is unacceptable.

**Alternatives considered:**

- Store as raw string — preserves leading zeros, no normalisation ambiguity, consistent with treating all key types as opaque strings; chosen
- Parse as integer — loses leading zeros; would require re-padding to 8 digits to recover them, and only works if the canonical width is known and consistent across all sources; ruled out
- Normalise to zero-padded 8-char string — handles mixed formats (e.g. one source omits leading zeros, another preserves them) but requires knowing the canonical width upfront and adds a transformation step that could introduce its own bugs; ruled out for now, revisit if mixed-format sources are encountered

**Why:** Keys are already treated as opaque strings throughout the system — the connector returns raw string values, the algorithm joins them with `\x00`, the frequency map uses them as-is. Preserving leading zeros requires no extra code. Parsing as integer requires extra code and introduces risk of silent data corruption.

---

## D3: Multi-column key configuration

**Decision:** One or more key columns are specified via `key_columns` in the YAML config (list of column names). When multiple columns are specified, their values are joined with `\x00` to form a composite key string. The config is mandatory — omitting `key_columns` is a hard error.

**Context:** The current sample files are single-column CSVs with a `udprn` header, but the platform is designed to handle richer datasets where a row may be identified by multiple columns (e.g. `udprn`, `email`, `loyalty_card_id`). The solution must be configurable without code changes.

**Alternatives considered:**

- Always use the first column as the key — simple but not extensible; breaks on multi-column or reordered files; silent incorrectness
- Single column name config — handles the current data but requires code change to support composite keys later
- `key_columns` list in YAML config — handles both single and composite keys; explicit; chosen
- Require the key column to be named `udprn` — brittle, breaks for any other key type

**Why:** `key_columns` is required with no default — the caller must be explicit about which columns identify a record. A default risks silently wrong results if the file structure changes or contains non-key columns. In a privacy-sensitive platform, silent incorrectness is unacceptable.

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

## D5: Single-pass per dataset

**Decision:** One pass per dataset — stream dataset A into a frequency map, then stream dataset B once to compute all four metrics.

**Context:** Total key count and total overlap both require knowing frequencies. Distinct count and distinct overlap require only presence. All four can be derived from a single frequency map pass per dataset.

**Alternatives considered:**

- Two passes per dataset (one for total count, one for distinct) — reads each source twice; wasteful for large files and incompatible with non-seekable remote sources
- Single pass with frequency map — chosen; collects all information needed in one pass per dataset

**Why:** Non-seekable remote sources (API streams, database cursors) cannot be rewound. A single-pass design is required for the connector abstraction to work correctly across all source types.

---

## D6: Configuration via YAML config file

**Decision:** Dataset sources, key columns, and output destination are specified via a YAML config file passed with `--config`. The config file is mandatory — there is no shorthand or default form.

**Full config form:**

```yaml
datasets:
  - connector: csv
    page_size: 1000
    max_error_rate: 0.05
    params:
      path: /data/A_f.csv
  - connector: rest
    page_size: 1000
    max_error_rate: 0.05
    params:
      url: https://api.example.com/records
      auth_header: Authorization
      auth_token: ${REST_AUTH_TOKEN}

key_columns: [udprn, email]

# exact algorithm — use when dataset fits within memory constraints
algorithm:
  type: pairwise_exact
  cache:
    strategy: in_memory
    max_memory_mb: 512
    spill_dir: /tmp

# approximate algorithm — use when dataset is too large for exact counting
# algorithm:
#   type: pairwise_approximate
#   precision: 14   # 0.8% error, ~256KB memory

output:
  writer: stdout

run:
  timeout_seconds: 3600
```

**Alternatives considered:**

- Positional arguments only — works for CSV file paths but cannot express connector type, auth, pagination config, or output destination; does not scale
- Named flags only (`--file-a`, `--file-b`, `--key-columns`, `--output`) — manageable for CSV but does not scale to REST or database connectors which need arbitrarily many parameters
- Shorthand convenience form — weakens the design by introducing implicit behaviour; ruled out for consistency with the principle of explicit configuration
- YAML config file — scales to any connector type, explicit, self-documenting; chosen

**Why:** Different connectors have fundamentally different configuration shapes. A flag-based interface cannot accommodate this without becoming unwieldy. A config file makes each connector's parameters explicit and self-documenting, and mirrors how real data pipeline tools are configured.

---

## D7: Output via ResultWriter interface

**Decision:** Results are written via a `ResultWriter` interface. The default writer formats output as a human-readable table to stdout. The output destination is configurable via `--output`.

**Context:** Mirroring the `KeyIterator` connector pattern on the input side, the output side should be equally pluggable. Hardcoding stdout couples the algorithm to a single output destination and makes it impossible to write results to a file, database, or API without changing the algorithm.

**Alternatives considered:**

- Hardcode stdout — simple but not extensible; rules out file, API, and database output without algorithm changes
- Plain text table to stdout (default writer) — human-readable, matches the "display" framing in the spec; chosen as the default
- JSON writer — machine-readable, useful for piping to other tools or storing results; available as a future writer implementation
- Both via flag — handled naturally by the `ResultWriter` interface; no special casing needed

**Why:** Same reasoning as `KeyIterator` — the algorithm should have no opinion about where results go. A `ResultWriter` keeps that concern isolated in a single swappable component.

---

## D8: Pluggable IntersectionAlgorithm

**Decision:** The intersection computation is abstracted behind an `IntersectionAlgorithm` interface. The implementation is selected via the `algorithm.type` field in the YAML config. Algorithm type and caching strategy are orthogonal concerns — type determines how computation is done, caching determines where the frequency map lives (exact algorithms only).

```go
type IntersectionAlgorithm interface {
    Compute(datasets []KeyIterator) (IntersectionResult, error)
}
```

**Algorithm types:**

| Type                   | Datasets | Accuracy    | Frequency map? | Notes                                         |
| ---------------------- | -------- | ----------- | -------------- | --------------------------------------------- |
| `pairwise_exact`       | 2        | Exact       | Yes            | Current implementation                        |
| `pairwise_approximate` | 2        | ~1-2% error | No             | HyperLogLog for distinct, MinHash for overlap |
| `nway_exact`           | N        | Exact       | Yes            | Required for Venn diagram output              |
| `nway_approximate`     | N        | ~1-2% error | No             | Deferred                                      |

**Note:** Exact vs approximate is a property of the algorithm type, not a system-level flag. Approximate algorithms do not build frequency maps at all — they use HyperLogLog and MinHash directly. Caching strategy is irrelevant for approximate algorithms and only applies to exact ones.

**Alternatives considered:**

- Hardcode the pairwise algorithm — simple for the current spec but forces code changes to support N-way or approximation; ruled out
- Flag-controlled approximation mode — adds conditional logic throughout rather than isolating it in a separate implementation; ruled out
- Pluggable interface — chosen; each implementation owns its own computation approach and memory needs

**Why:** The three-layer architecture (`KeyIterator → IntersectionAlgorithm → ResultWriter`) gives each layer a single well-defined responsibility. Algorithm type and caching strategy being separate means you can change one without touching the other.

---

## D10: Caching strategy for exact algorithms

**Decision:** Exact algorithm implementations (`pairwise_exact`, `nway_exact`) support a configurable `cache` block controlling where the frequency map lives. Approximate algorithms have no cache block — they never build a frequency map.

| Strategy        | Memory usage                 | Accuracy | When to use                                         |
| --------------- | ---------------------------- | -------- | --------------------------------------------------- |
| `in_memory`     | O(distinct keys per dataset) | Exact    | Default — frequency map fits within `max_memory_mb` |
| `spill_to_disk` | O(batch size) working memory | Exact    | Frequency map exceeds `max_memory_mb`               |

**Config for exact algorithm:**

```yaml
algorithm:
  type: pairwise_exact
  cache:
    strategy: in_memory # in_memory | spill_to_disk
    max_memory_mb: 512 # threshold before spill_to_disk kicks in
    spill_dir: /tmp # only used when strategy: spill_to_disk
```

**Config for approximate algorithm:**

```yaml
algorithm:
  type: pairwise_approximate
  precision:
    14 # HyperLogLog precision — range 4–18, higher = more accurate, more memory
    # error rate ≈ 1.04 / √(2^precision); memory ≈ 2^precision × 8 bytes (verify against axiomhq/hyperloglog)
    # precision 10 ≈ 3.25% error, ~8KB
    # precision 18 ≈ 0.20% error, ~2MB
  # no cache block — approximate algorithms do not build frequency maps
```

Output always includes error bounds when an approximate algorithm is used — e.g. `distinct_overlap: 4823901 (± 0.8%)`. The acceptable error margin is the caller's decision based on their use case; the system's responsibility is to report it accurately.

**How each strategy works:**

- `in_memory` — frequency map held entirely in RAM. Each goroutine writes to its own map with no locking (each dataset has its own map). Fast, simple, exact. Default.
- `spill_to_disk` — when the in-memory map exceeds `max_memory_mb`, it is flushed to `spill_dir` in sorted key order. At the end all chunks are merged to produce exact counts. RAM-bounded but significantly slower due to disk I/O.

---

### Sizing Strategy

Rather than fixed thresholds, the right approach is to derive them from actual conditions. Each entry in a `map[string]uint64` costs:

| Component              | Size     | Explanation                                                                               |
| ---------------------- | -------- | ----------------------------------------------------------------------------------------- |
| Go string header       | 16 bytes | Pointer + length field, paid per entry regardless of content                              |
| String data            | L bytes  | Raw key bytes — includes `\x00` delimiters for composite keys                             |
| uint64 counter value   | 8 bytes  | Frequency count — fixed size regardless of value                                          |
| Go map bucket overhead | ~50%     | Amortised cost of Go's bucket-based hash map internals (overflow pointers, tophash bytes) |

```
bytes_per_entry ≈ (16 + L + 8) × 1.5
```

where L = sum of max field lengths for each column in `key_columns` + (number of columns − 1) for `\x00` delimiters. Multiply by distinct key count and divide by 1024² to get MB. Set `max_memory_mb` to ~80% of available RAM to leave headroom for both dataset maps and the Go runtime.

**Strategy decision:**

```
map_size_mb < available_ram × 0.8                 →  in_memory
map_size_mb > available_ram × 0.8                 →  spill_to_disk
spill_to_disk I/O cost unacceptable for use case  →  pairwise_approximate
```

The threshold for choosing `pairwise_approximate` over `spill_to_disk` is not a fixed number — it depends on the operator's latency requirements and tolerance for approximation. Benchmark `spill_to_disk` at the target dataset size and compare the wall-clock time against the acceptable run time before switching to approximate.

**Wall-clock time** is dominated by the slowest connector — benchmark each separately:

| Connector   | Primary bottleneck             | What to measure                              |
| ----------- | ------------------------------ | -------------------------------------------- |
| CSV (local) | Disk read + CSV parse speed    | Rows/sec at varying file sizes               |
| REST API    | Network latency + rate limits  | Pages/sec, rows/page, API rate limit ceiling |
| Database    | Query execution + cursor fetch | Rows/sec at varying `LIMIT` sizes            |
| SFTP        | Network bandwidth              | Effective MB/sec throughput                  |

Set `timeout_seconds` to ~2× the estimated wall-clock time (`distinct_key_count / rows_per_sec`) to allow for variance without cutting off legitimate runs.

**Current implementation:** `in_memory` only. `spill_to_disk` is deferred. Benchmarks have not yet been run — the above is the measurement plan.

---

## D11: Long-running process control

**Decision:** A configurable timeout is the primary mechanism for controlling long-running processes. Resume is acknowledged as a desirable future extension but is deferred.

**Config:**

```yaml
run:
  timeout_seconds: 3600
```

When the timeout is exceeded the program cancels all connector goroutines via a shared context, flushes any partial `ConnectorStats` to stderr, and exits with a non-zero exit code and a clear message indicating timeout.

**Why timeout:**

- Simple to implement — Go's `context.WithTimeout` propagates cancellation to all goroutines cleanly
- Protects against runaway jobs — a slow or unresponsive remote connector cannot block the process indefinitely
- Sufficient for the current use case — the provided datasets are small; timeout is a safety valve, not a primary flow

**Why not resume (yet):**
Resume requires two components working together:

1. The connector must checkpoint its position (byte offset, page cursor, last row ID) after each batch
2. The algorithm must checkpoint its partial frequency map to disk so already-processed rows do not need to be reprocessed

These are coupled — a connector checkpoint without an algorithm checkpoint means reprocessing all rows from the resumed position, producing incorrect counts. Implementing both correctly adds significant complexity without a clear requirement driving it now.

The `KeyIterator` interface is designed to accommodate resume in future — a `Checkpoint() error` and `Resume(checkpoint Checkpoint) error` method pair could be added to the interface without changing the algorithm or writer layers.

**Alternatives considered:**

- No timeout — process hangs indefinitely on a slow or unresponsive source; unacceptable
- Timeout only — chosen for now; simple, effective for the current scope
- Resume only — solves a different problem (recovery) without preventing runaway processes; incomplete without timeout
- Both — the right long-term answer; resume deferred until there is a concrete requirement for it

---

## D12: Empty key field handling

**Decision:** A row where one or more `key_columns` fields are empty is a soft failure — the row is skipped and recorded in `ConnectorStats`. Processing continues.

**Context:** An empty field in a key column means the composite key is incomplete. It cannot reliably identify an individual — a blank UDPRN or email is not a valid key. More importantly, multiple rows with the same empty field would produce spurious matches against each other, inflating overlap counts silently.

**Alternatives considered:**

- Hard failure — abort the entire run on the first empty field; too strict for real-world data which commonly has sparse or optional fields
- Treat empty as a valid key value — would cause blank-field rows to match each other across datasets, producing meaningless overlap counts; ruled out
- Soft failure — chosen; consistent with the handling of other malformed rows (FR6); caller is informed via `ConnectorStats`

**Why:** Treating an empty key field as a soft failure is consistent with the existing error handling model. The connector is responsible for deciding what constitutes a valid row — an incomplete key is not valid, but it is not a reason to abort the entire run.

---

## D13: Secret injection into config

**Decision:** Secrets in the YAML config (e.g. `auth_token` for REST connectors) are specified as environment variable references using `${VAR}` syntax. At config parse time the program expands these references via `os.Getenv`. The actual secret lives in the environment, injected by the orchestrator or a secrets manager.

**Example:**

```yaml
datasets:
  - connector: rest
    url: https://api.example.com/records
    auth_header: Authorization
    auth_token: ${REST_AUTH_TOKEN}
```

**Alternatives considered:**

- Plaintext in config file — simple but unacceptable for production; secrets committed to version control or stored on disk in plaintext
- Separate secrets config file — avoids plaintext in the main config but adds complexity: two files to manage, two parse paths, no standard format
- CLI flag for secrets — contradicts the mandatory config file design; also exposes secrets in process list (`ps aux`)
- Environment variable expansion in config — chosen; secrets stay out of the config file, the orchestrator or secrets manager controls injection, no code changes needed to rotate a secret

**Why:** The config file may be stored in version control or on a shared filesystem — it must never contain secrets directly. Environment variable expansion is the standard pattern for CLI tools running in pipelines and is compatible with all common secrets managers (AWS Secrets Manager, HashiCorp Vault, Kubernetes secrets) without any integration code.
