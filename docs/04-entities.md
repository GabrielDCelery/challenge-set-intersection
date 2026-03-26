# Entities

The three interfaces that form the layer boundaries of the system, plus the supporting types that flow between them.

```
KeyIterator(s)  →  IntersectionAlgorithm  →  ResultWriter
```

Note: this is a stateless CLI tool. There is no persistent schema. These entities are in-memory interfaces and structs that exist for the duration of a single program run.

---

## KeyIterator

The input boundary. Each dataset source implements this interface. The algorithm has no knowledge of what lies behind it — CSV file, REST API, database cursor, or SFTP stream.

```go
type KeyIterator interface {
    NextBatch() (keys [][]string, done bool, err error)
    Stats() ConnectorStats
    Close() error
}
```

- `NextBatch()` returns a batch of rows. Each row is a `[]string` — one element per configured `key_columns` entry, in order. Returns `done=true` on the final batch.
- `Stats()` returns accumulated `ConnectorStats` at any point — safe to call after every batch. Used by the algorithm to enforce `max_error_rate`.
- `Close()` releases any held resources (file handles, connections). Safe to `defer`.

**Key representation:** keys are raw strings, leading zeros preserved (D2). For composite keys, the algorithm joins the `[]string` elements with `\x00` to form a single frequency map key — the delimiter cannot appear in real data so there is no collision risk (D3).

**Soft failures:** malformed rows are skipped by the connector. The row number and reason are recorded in `ConnectorStats.errors`. Processing continues (FR6).

**Current implementation:** `CsvKeyIterator` — reads a CSV file row by row, resolves `key_columns` to column indices from the header row.

---

## IntersectionAlgorithm

The computation boundary. Accepts N `KeyIterator` instances and returns an `IntersectionResult`. Owns all decisions about memory strategy, parallelism, and accuracy — none of these concerns leak into the connector or writer layers.

```go
type IntersectionAlgorithm interface {
    Compute(datasets []KeyIterator) (IntersectionResult, error)
}
```

Streams connectors in parallel — one goroutine per `KeyIterator`, managed internally. Cancellation propagates via shared context (D9). Checks `ConnectorStats` after each batch and aborts if `max_error_rate` is exceeded (FR6).

**Current implementation:** `pairwise_exact` — two datasets, exact counts, frequency map held in memory.

**Future implementations:**
- `pairwise_approximate` — HyperLogLog for distinct counts, MinHash for overlap; no frequency map; results include error bounds
- `nway_exact` — N datasets, full pairwise and multi-way breakdown
- `nway_approximate` — N datasets, probabilistic

**Note:** caching strategy (`in_memory`, `spill_to_disk`) applies to exact implementations only. Approximate implementations do not build frequency maps (D8, D10).

---

## ResultWriter

The output boundary. Accepts an `IntersectionResult` and writes it to a destination. The algorithm has no knowledge of where results go.

```go
type ResultWriter interface {
    Write(result IntersectionResult) error
}
```

**Current implementation:** `StdoutWriter` — formats results as a human-readable table. Includes error bounds when the result was produced by an approximate algorithm.

**Future implementations:** JSON writer, file writer, REST/webhook writer.

---

## IntersectionResult

The value that crosses from `IntersectionAlgorithm` to `ResultWriter`. Holds only aggregate counts — never individual key values. No PII or identifiable data ever enters this struct.

```go
type DatasetStats struct {
    Source        string
    TotalCount    uint64
    DistinctCount uint64
}

type IntersectionResult struct {
    Datasets        []DatasetStats
    DistinctOverlap uint64
    TotalOverlap    uint64
    ErrorBoundPct   float64
}
```

**DatasetStats** — one entry per input source, in the order datasets were provided:

| Field           | Type   | Notes                                                        |
| --------------- | ------ | ------------------------------------------------------------ |
| `source`        | string | Human-readable source identifier (e.g. file path, URL)       |
| `total_count`   | uint64 | Total key count including duplicates                         |
| `distinct_count`| uint64 | Count of distinct keys                                       |

**IntersectionResult** — overlap figures across all datasets:

| Field             | Type    | Notes                                                              |
| ----------------- | ------- | ------------------------------------------------------------------ |
| `datasets`        | []DatasetStats | Per-dataset counts, N entries for N input sources         |
| `distinct_overlap`| uint64  | Count of keys appearing in all datasets (regardless of frequency)  |
| `total_overlap`   | uint64  | Sum of count_in_A × count_in_B across all shared keys             |
| `error_bound_pct` | float64 | Error bound as a percentage (0 = exact result, no bound)           |

This shape works for 2 or N datasets without structural change — the `ResultWriter` iterates `datasets` to print per-source rows, then prints the overlap figures. When produced by an approximate algorithm, `error_bound_pct` is non-zero (e.g. `0.8` for ±0.8%) and is included in output by the `ResultWriter` (D8).

---

## ConnectorStats

Accumulated per-connector statistics. Populated during iteration and readable at any point via `KeyIterator.Stats()`.

```go
type RowError struct {
    RowNumber uint64
    Reason    string
}

type ConnectorStats struct {
    Source      string
    RowsRead    uint64
    RowsSkipped uint64
    Errors      []RowError
}
```

| Field          | Type       | Notes                                                          |
| -------------- | ---------- | -------------------------------------------------------------- |
| `source`       | string     | Human-readable source identifier — matches `DatasetStats.source` |
| `rows_read`    | uint64     | Total rows seen by the connector, including skipped rows       |
| `rows_skipped` | uint64     | Rows skipped due to malformed data                             |
| `errors`       | []RowError | Per-row error details for skipped rows                         |

**RowError:**

| Field        | Type   | Notes                                      |
| ------------ | ------ | ------------------------------------------ |
| `row_number` | uint64 | 1-based row number in the source           |
| `reason`     | string | Human-readable description of the problem |

**Error rate** = `rows_skipped / rows_read`. If this exceeds `max_error_rate` after a batch the algorithm aborts with a non-zero exit code and flushes stats to stderr.

---

## RunConfig

Parsed from the YAML config file. Drives construction of connectors, algorithm, and writer at startup. See D6 for the full config schema and alternatives considered.

```go
type RunConfig struct {
    Datasets   []DatasetConfig
    KeyColumns []string
    Algorithm  AlgorithmConfig
    Output     OutputConfig
    Run        RunControlConfig
}
```

| Field         | Type             | Notes                                                                        |
| ------------- | ---------------- | ---------------------------------------------------------------------------- |
| `datasets`    | []DatasetConfig  | One entry per input source. Two required for pairwise algorithms.            |
| `key_columns` | []string         | **Required.** Column names forming the key. Omitting is a hard error — no default. |
| `algorithm`   | AlgorithmConfig  | Type and caching strategy.                                                   |
| `output`      | OutputConfig     | Writer type and destination.                                                 |
| `run`         | RunControlConfig | Timeout and future resume settings.                                          |

---

### DatasetConfig

```go
type DatasetConfig struct {
    Connector    string
    PageSize     int
    MaxErrorRate float64
    // CSV-specific
    Path      string
    HasHeader bool
    // REST-specific
    URL        string
    AuthHeader string
    AuthToken  string
}
```

| Field            | Type    | Notes                                                              |
| ---------------- | ------- | ------------------------------------------------------------------ |
| `connector`      | string  | Connector type: `csv`, `rest`, `database`, `sftp`                  |
| `page_size`      | int     | Batch size returned per `NextBatch()` call                         |
| `max_error_rate` | float64 | Fraction of rows that may be skipped before the algorithm aborts   |
| `path`           | string  | CSV only — file path                                               |
| `has_header`     | bool    | CSV only — whether the first row is a header                       |
| `url`            | string  | REST only — endpoint URL                                           |
| `auth_header`    | string  | REST only — HTTP header name for auth token                        |
| `auth_token`     | string  | REST only — token value                                            |

---

### AlgorithmConfig

```go
type CacheConfig struct {
    Strategy    string
    MaxMemoryMB int
    SpillDir    string
}

type AlgorithmConfig struct {
    Type      string
    Cache     *CacheConfig // nil for approximate algorithms
    Precision int          // approximate algorithms only; range 4–18
}
```

| Field       | Type         | Notes                                                                          |
| ----------- | ------------ | ------------------------------------------------------------------------------ |
| `type`      | string       | `pairwise_exact`, `pairwise_approximate`, `nway_exact`, `nway_approximate`     |
| `cache`     | *CacheConfig | Exact algorithms only. `strategy`: `in_memory` or `spill_to_disk`             |
| `precision` | int          | Approximate algorithms only. Controls HyperLogLog accuracy (4–18). See D10.   |

---

### OutputConfig

```go
type OutputConfig struct {
    Writer string
}
```

| Field    | Type   | Notes                                              |
| -------- | ------ | -------------------------------------------------- |
| `writer` | string | Output destination: `stdout`. Future: `json`, `file`, `rest` |

---

### RunControlConfig

```go
type RunControlConfig struct {
    TimeoutSeconds int
}
```

| Field             | Type | Notes                                                                    |
| ----------------- | ---- | ------------------------------------------------------------------------ |
| `timeout_seconds` | int  | Cancels all connector goroutines and exits with non-zero code if exceeded |
