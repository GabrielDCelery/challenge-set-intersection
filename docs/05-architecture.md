# Architecture

Infrastructure-level decisions — separate from domain/algorithm decisions in `02-decisions.md`. Informed by data consumers and entities.

---

## System Shape

This is a stateless, single-process CLI tool. There is no server, no database, no message queue, and no network communication. The entire system fits within a single binary invocation that:

1. Reads CLI arguments
2. Constructs two `KeyIterator` connectors from the specified sources
3. Streams keys from each connector in batches — no full dataset is ever loaded into memory
4. Computes four integer counts
5. Writes results to stdout
6. Exits

All architecture concerns here are about the internal structure of that process and how it scales with input size.

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

**Current connector:** `CSVFileConnector` — opens a local CSV file, reads the header row to resolve `key_columns` to column indices, and returns batches of `[][]string`.

**Future connectors** (not in scope for this implementation but the interface accommodates them without algorithm changes):
- `JSONConnector` — reads a JSON array or newline-delimited JSON, extracting configured fields per record
- `RESTConnector` — paginates through an API endpoint, one page per batch
- `DatabaseConnector` — wraps a database cursor, returning rows in batches
- `SFTPConnector` — streams a remote file over SFTP, delegating to the appropriate file format connector

---

## I/O Strategy

### Streaming vs Bulk Load

**Decision:** Stream both datasets via `KeyIterator`. Never load a full dataset into memory.

**Algorithm:**
1. Stream dataset A via its connector in batches — for each `[]string` row, join with `\x00` to form a map key, increment `map[string]uint64` frequency map
2. Stream dataset B via its connector in batches — for each `[]string` row, join with `\x00`, look up in the frequency map, and accumulate overlap counts
3. Compute the four metrics from the frequency map and accumulators

Memory usage is O(distinct keys in A). Dataset B is never stored — each row is consumed, looked up, and discarded.

**Alternatives considered:**

- Load both datasets fully into memory as slices — simpler code, but makes the solution incompatible with remote sources and large datasets; ruled out
- External sort-merge — O(1) extra memory, exact results, but requires temp disk space and adds significant I/O complexity; only warranted if distinct key count in A does not fit in RAM; deferred

**Why:** Streaming is the only approach compatible with the connector abstraction. A remote source (API, database, SFTP) cannot be bulk-loaded — it can only be iterated. Designing for streaming from the start means the algorithm works identically regardless of source.

---

## Scalability

### Read/Write Split

There are no writes in this system. All operations are reads from disk followed by in-memory computation.

- **Reads:** Two sequential file reads. No random access required. Throughput is bounded by disk read speed and CSV parse speed.
- **Writes:** One write to stdout at the end. Negligible.

### Memory Scaling

| Scenario                               | Memory required                   | Approach                   |
| -------------------------------------- | --------------------------------- | -------------------------- |
| Small files (< 1M rows, < 500K keys)   | < ~50MB (string map)              | In-memory frequency map    |
| Medium files (1M–50M rows)             | ~500MB–5GB depending on key width | In-memory if RAM allows    |
| Very large files (> 50M distinct keys) | Exceeds typical RAM               | External sort-merge or HLL |

### Known Hotspots

- **Hash map insertion** for file A: O(n) time and memory. For very high cardinality this is the dominant cost.
- **Key normalisation**: if keys are stored as strings, each key occupies more memory than an integer. For 8-digit UDPRNs stored as uint32, memory is ~4x smaller than string storage.
- **CSV parsing**: naive line-by-line string splitting is slower than a proper CSV parser that handles quoting. For large files, parser choice matters.

### What Is Not Addressed Yet

The following cannot be sized without answers to the open questions in `01-requirements.md`:

- Acceptable wall-clock runtime (OQ1)
- Whether external-sort or HyperLogLog is needed (depends on max distinct key count vs available RAM)
- Whether parallelism (concurrent file reads) is worth the implementation complexity

---

## Auth

This is a local CLI tool. There is no authentication or authorisation concern — the tool has access to whatever files the OS user running it has permission to read. No credentials, tokens, or roles.

---

## Privacy Boundary

Although this tool processes data that originated as PII (delivery addresses), the input files contain only anonymised UDPRN keys — not names, addresses, or any other identifiable information. The tool never logs, stores, or transmits individual key values. Output contains only aggregate counts. This is consistent with InfoSum's stated platform guarantee of never revealing identifiable information.

See `08-security.md` for the full data classification.
