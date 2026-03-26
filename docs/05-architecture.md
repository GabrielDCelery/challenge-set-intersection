# Architecture

Infrastructure-level decisions — separate from domain/algorithm decisions in `02-decisions.md`. Informed by data consumers and entities.

---

## System Shape

This is a stateless, single-process CLI tool. There is no server, no database, no message queue, and no network communication. The entire system fits within a single binary invocation that:

1. Reads YAML config
2. Constructs N `KeyIterator` connectors from the specified sources
3. Constructs an `IntersectionAlgorithm` from the configured algorithm type
4. Constructs a `ResultWriter` from the configured output destination (default: stdout)
5. Passes the connectors to the algorithm — the algorithm owns streaming, memory, and computation
6. Passes the `IntersectionResult` to the `ResultWriter`
7. Exits

The three layers are fully decoupled:

```
KeyIterator(s)  →  IntersectionAlgorithm  →  ResultWriter
  (source)           (computation,             (sink)
                      memory strategy)
```

Each layer knows nothing about the others. Connectors, algorithms, and writers can be swapped independently via config.

---

## Connector Layer

The ingestion layer is decoupled from the algorithm via a `KeyIterator` interface:

```go
type KeyIterator interface {
    NextBatch() (keys [][]string, done bool, err error)
    Close() error
}
```

Each call to `NextBatch()` returns a batch of rows. Each row is a `[]string` — one element per configured key column, in the order specified by `key_columns`. The algorithm has no knowledge of the underlying source format (CSV, JSON, database row) or the separator used between fields. The connector is responsible for extracting the correct fields and returning them in the correct order.

**Example:** with `key_columns: ["udprn", "email"]`, a batch might look like:

```
[
  ["30433784", "alice@example.com"],
  ["71842328", "bob@example.com"],
]
```

A CSV connector and a JSON connector receiving the same data return identical output — the algorithm sees no difference.

**Frequency map key:** the algorithm joins the `[]string` slice with a null byte (`\x00`) to form a map key — e.g. `"30433784\x00alice@example.com"`. The null byte cannot appear in any real key value, eliminating collision risk. The join happens in one place inside the algorithm, not in the connector.

**Batching:** batch size is a connector implementation detail. A local CSV connector may return rows as fast as it can parse them. A REST connector returns one page per batch. The algorithm loops over whatever size batch it receives.

**Interface:**

```go
type RowError struct {
    RowNumber uint64
    Reason    string
}

type ConnectorStats struct {
    RowsRead    uint64
    RowsSkipped uint64
    Errors      []RowError
}

type KeyIterator interface {
    NextBatch() (keys [][]string, done bool, err error)
    Stats()     ConnectorStats
    Close()     error
}
```

`Stats()` can be called after each batch. The algorithm checks the error rate after every batch and aborts if it exceeds the configured `max_error_rate` threshold. `Close()` returns only an error so `defer connector.Close()` works as normal.

**Current connector:** `CSVFileConnector` — opens a local CSV file, reads the header row to resolve `key_columns` to column indices, returns batches of `[][]string`, skips malformed rows and records them in `Stats()`.

**Future connectors** (not in scope for this implementation but the interface accommodates them without algorithm changes):
- `JSONConnector` — reads a JSON array or newline-delimited JSON, extracting configured fields per record
- `RESTConnector` — paginates through an API endpoint, one page per batch
- `DatabaseConnector` — wraps a database cursor, returning rows in batches
- `SFTPConnector` — streams a remote file over SFTP, delegating to the appropriate file format connector

---

## I/O Strategy

### Streaming vs Bulk Load

**Decision:** Stream all datasets via `KeyIterator` in parallel — one goroutine per connector. Never load a full dataset into memory.

**Algorithm (PairwiseAlgorithm):**
1. Launch one goroutine per connector — each streams its dataset independently and builds its own frequency map concurrently
2. Wait for all goroutines to complete
3. Compute the four metrics by comparing the completed frequency maps
4. Check `ConnectorStats` error rates — abort if any connector exceeded `max_error_rate`

Memory usage is O(distinct keys across all datasets). No dataset is stored as raw rows — each row is consumed, joined with `\x00`, inserted into the frequency map, and discarded.

**Why parallel:** connectors are fully independent — a CSV file read and a REST API paginated fetch share no state. Streaming them sequentially wastes wall-clock time for no benefit. Parallelism is natural in Go via goroutines and is the algorithm's responsibility to manage — the connector interface does not change.

**Alternatives considered:**

- Sequential streaming — simpler but wastes time; a slow remote connector blocks all subsequent connectors; ruled out
- Load both datasets fully into memory as slices — incompatible with remote sources and large datasets; ruled out
- External sort-merge — O(1) extra memory, exact results, but requires temp disk space and significant I/O complexity; deferred

---

### Concurrent Streaming Design

Each connector runs in its own goroutine, streaming batches into a dedicated frequency map. The algorithm waits for all goroutines via a `sync.WaitGroup` or equivalent, collecting errors via a channel. If any goroutine exceeds `max_error_rate` or encounters a fatal error it signals cancellation via a shared context — all other goroutines stop cleanly.

```
goroutine 1: connector A → frequency map A
goroutine 2: connector B → frequency map B
     ↓ (both complete)
main: compare maps → IntersectionResult → ResultWriter
```

This design extends naturally to N datasets — fan out to N goroutines, wait for all, then compute intersections from N maps.

---

## Scalability

### Read/Write Split

There are no writes in this system. All operations are concurrent reads followed by in-memory computation.

- **Reads:** N concurrent connector streams. No random access required. Throughput per connector is bounded by its source speed (disk, network, database cursor).
- **Writes:** One write to the configured output destination at the end. Negligible.

### Memory Scaling

| Scenario                                      | Memory required                          | Approach                        |
| --------------------------------------------- | ---------------------------------------- | ------------------------------- |
| Small datasets (< 1M rows, < 500K keys each)  | < ~50MB per frequency map                | In-memory frequency maps        |
| Medium datasets (1M–50M rows)                 | ~500MB–5GB per map depending on key width| In-memory if RAM allows         |
| Very large datasets (> 50M distinct keys)     | Exceeds typical RAM                      | Algorithm caching strategy (D9) |

### Known Hotspots

- **Concurrent map writes:** each goroutine writes to its own dedicated frequency map — no shared state, no locking required during the streaming phase.
- **Hash map insertion:** O(n) time and memory per dataset. For very high cardinality this is the dominant cost.
- **Key join:** `strings.Join(row, "\x00")` on every row — cheap but worth noting for very high throughput scenarios.
- **CSV parsing:** naive line-by-line string splitting is slower than a proper CSV parser that handles quoting. For large files, parser choice matters.

### What Is Not Addressed Yet

- Acceptable wall-clock runtime (OQ1)
- Whether the algorithm's caching strategy (spill to disk, HyperLogLog) is needed — depends on max distinct key count vs available RAM (OQ1, OQ6)

---

## Writer Layer

The output layer is decoupled from the algorithm via a `ResultWriter` interface:

```go
type ResultWriter interface {
    Write(result IntersectionResult) error
    Close() error
}
```

The algorithm calls `Write()` once with the computed `IntersectionResult`. The writer decides how to format and deliver it. The output destination is selected at construction time via the `--output` flag.

**Current writer:** `StdoutWriter` — formats the result as a human-readable table and writes to stdout.

**Future writers** (not in scope for this implementation but the interface accommodates them without algorithm changes):
- `JSONFileWriter` — serialises the result as JSON to a file
- `RESTWriter` — POSTs the result to an API endpoint
- `DatabaseWriter` — inserts the result into a database table

---

## Auth

This is a local CLI tool. There is no authentication or authorisation concern — the tool has access to whatever files the OS user running it has permission to read. No credentials, tokens, or roles.

---

## Privacy Boundary

Although this tool processes data that originated as PII (delivery addresses), the input files contain only anonymised UDPRN keys — not names, addresses, or any other identifiable information. The tool never logs, stores, or transmits individual key values. Output contains only aggregate counts. This is consistent with InfoSum's stated platform guarantee of never revealing identifiable information.

See `08-security.md` for the full data classification.
