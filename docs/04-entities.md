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

**Soft failures:** malformed rows are skipped by the connector. The row number and reason are recorded in `ConnectorStats.Errors`. Processing continues (FR6).

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
    Source        string  // human-readable identifier (e.g. file path, URL)
    TotalCount    uint64  // total rows including duplicates
    DistinctCount uint64  // unique keys only
}

type IntersectionResult struct {
    Datasets        []DatasetStats // one entry per input source, N-dataset safe
    DistinctOverlap uint64         // keys appearing in all datasets, regardless of frequency
    TotalOverlap    uint64         // sum of count_in_A × count_in_B across shared keys
    ErrorBoundPct   float64        // 0 = exact; non-zero = approximate, e.g. 0.8 for ±0.8%
}
```

The `ResultWriter` iterates `Datasets` to print per-source rows, then prints the overlap figures. `ErrorBoundPct` is included in output only when non-zero (D8).

---

## ConnectorStats

Accumulated per-connector statistics. Populated during iteration and readable at any point via `KeyIterator.Stats()`. Error rate = `RowsSkipped / RowsRead` — if this exceeds `max_error_rate` after a batch the algorithm aborts with a non-zero exit code and flushes stats to stderr.

```go
type RowError struct {
    RowNumber uint64 // 1-based row number in the source
    Reason    string // human-readable description of the problem
}

type ConnectorStats struct {
    Source      string     // matches DatasetStats.Source for traceability
    RowsRead    uint64     // total rows seen, including skipped
    RowsSkipped uint64     // rows skipped due to malformed data
    Errors      []RowError // per-row detail for skipped rows
}
```

---

## RunConfig

Parsed from the YAML config file. Drives construction of connectors, algorithm, and writer at startup. See D6 for the full config schema and alternatives considered.

```go
type RunConfig struct {
    Datasets   []DatasetConfig  // one entry per input source; two required for pairwise algorithms
    KeyColumns []string         // required — omitting is a hard error, no default
    Algorithm  AlgorithmConfig
    Output     OutputConfig
    Run        RunControlConfig
}

type DatasetConfig struct {
    Connector    string   // "csv" | "rest" | "database" | "sftp"
    PageSize     int      // batch size per NextBatch() call
    MaxErrorRate float64  // fraction of skippable rows before aborting
    // CSV-specific
    Path      string
    HasHeader bool
    // REST-specific
    URL        string
    AuthHeader string // HTTP header name for the auth token
    AuthToken  string
}

type CacheConfig struct {
    Strategy    string // "in_memory" | "spill_to_disk"
    MaxMemoryMB int    // threshold before spilling
    SpillDir    string // temp directory for spill files
}

type AlgorithmConfig struct {
    Type      string       // "pairwise_exact" | "pairwise_approximate" | "nway_exact" | "nway_approximate"
    Cache     *CacheConfig // exact algorithms only; nil for approximate
    Precision int          // approximate algorithms only; HyperLogLog range 4–18, see D10
}

type OutputConfig struct {
    Writer string // "stdout"; future: "json", "file", "rest"
}

type RunControlConfig struct {
    TimeoutSeconds int // cancels all goroutines and exits non-zero if exceeded
}
```
