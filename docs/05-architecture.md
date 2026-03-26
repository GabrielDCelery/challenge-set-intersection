# Architecture

Infrastructure-level decisions — separate from domain/algorithm decisions in `02-decisions.md`. Informed by data consumers and entities.

---

## System Shape

This is a stateless, single-process CLI tool. There is no server, no database, no message queue, and no network communication. The entire system fits within a single binary invocation that:

1. Reads CLI arguments
2. Constructs two `KeyIterator` connectors from the specified sources
3. Streams keys from each connector one at a time — no full dataset is ever loaded into memory
4. Computes four integer counts
5. Writes results to stdout
6. Exits

All architecture concerns here are about the internal structure of that process and how it scales with input size.

---

## Connector Layer

The ingestion layer is decoupled from the algorithm via a `KeyIterator` interface:

```go
type KeyIterator interface {
    Next() (key string, done bool, err error)
    Close() error
}
```

The algorithm only calls `Next()` and `Close()`. It has no knowledge of whether the underlying source is a local CSV file, a paginated REST API, a database cursor, or an SFTP stream. Each source type is a separate connector that implements this interface.

**Current connector:** `CSVFileConnector` — opens a local CSV file, resolves the key columns from the header row, and yields one composite key string per call to `Next()`.

**Future connectors** (not in scope for this implementation but the interface accommodates them without algorithm changes):
- `RESTConnector` — paginates through an API endpoint, yielding one key per record
- `DatabaseConnector` — wraps a database cursor
- `SFTPConnector` — streams a remote CSV over SFTP

---

## I/O Strategy

### Streaming vs Bulk Load

**Decision:** Stream both datasets via `KeyIterator`. Never load a full dataset into memory.

**Algorithm:**
1. Stream dataset A via its connector — build a frequency map `map[string]uint64` of key → count
2. Stream dataset B via its connector — for each key, look it up in the frequency map and accumulate overlap counts
3. Compute the four metrics from the frequency map and accumulators

Memory usage is O(distinct keys in A). Dataset B is never stored — each key is consumed, looked up, and discarded.

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
