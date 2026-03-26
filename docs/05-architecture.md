# Architecture

Infrastructure-level decisions — separate from domain/algorithm decisions in `02-decisions.md`. Informed by data consumers and entities.

---

## System Shape

This is a stateless, single-process CLI tool. There is no server, no database, no message queue, and no network communication. The entire system fits within a single binary invocation that:

1. Reads CLI arguments
2. Streams two CSV files from local disk
3. Computes four integer counts in memory
4. Writes results to stdout
5. Exits

All architecture concerns here are about the internal structure of that process and how it scales with input size.

---

## I/O Strategy

### Streaming vs Bulk Load

**Decision:** TBD

**Alternatives considered:**

- Stream file A row by row into a frequency map, then stream file B row by row and compute overlap on the fly. Memory usage is O(distinct keys in A). Does not require both files to fit in memory simultaneously.
- Load both files fully into memory as slices, then compute. Simpler code, but doubles memory usage and offers no benefit.
- External sort-merge: sort both files on disk, then merge. O(1) extra memory, exact results, but requires temp disk space and adds I/O complexity. Only warranted if distinct key count in A does not fit in RAM.

**Why:** The streaming approach (file A into map, file B streamed) is the best default — it is exact, memory-efficient relative to bulk load, and requires only one pass per file.

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
