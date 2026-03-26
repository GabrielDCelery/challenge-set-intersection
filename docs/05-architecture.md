# Architecture

Infrastructure-level decisions — separate from domain/algorithm decisions in `02-decisions.md`. See `04-entities.md` for interface and struct definitions.

---

## System Shape

A stateless, single-process CLI tool. The entire system fits within a single binary invocation:

1. Read YAML config
2. Construct N `KeyIterator` connectors from the configured sources
3. Construct an `IntersectionAlgorithm` from the configured algorithm type
4. Construct a `ResultWriter` from the configured output destination
5. Pass connectors to the algorithm — it owns streaming, memory, and computation
6. Pass the `IntersectionResult` to the `ResultWriter`
7. Exit

```
KeyIterator(s)  →  IntersectionAlgorithm  →  ResultWriter
  (source)           (computation,             (sink)
                      memory strategy)
```

Each layer knows nothing about the others. Connectors, algorithms, and writers are swapped independently via config.

---

## Data Flow

Each connector runs in its own goroutine, streaming batches into a dedicated frequency map. The algorithm waits for all goroutines to complete, then computes the four metrics by comparing the maps.

```
goroutine 1: connector A → frequency map A ─┐
goroutine 2: connector B → frequency map B ─┤→ compare maps → IntersectionResult → ResultWriter
     (both complete via sync.WaitGroup)     ┘
```

Cancellation propagates via a shared `context.Context` — if any goroutine exceeds `max_error_rate`, encounters a fatal error, or the timeout fires, all other goroutines stop cleanly. Partial `ConnectorStats` are flushed to stderr on exit.

This design extends naturally to N datasets — fan out to N goroutines, wait for all, compute intersections from N maps.

**Why parallel:** connectors are fully independent. Streaming sequentially wastes wall-clock time — a slow remote connector would block all subsequent ones. Parallelism is the algorithm's responsibility; the `KeyIterator` interface does not change (D9).

---

## Scalability

### Memory

Memory usage is O(distinct keys across all datasets). Raw rows are never stored — each row is consumed, joined into a frequency map key, and discarded.

Each entry in a `map[string]uint64` costs approximately:

```
bytes_per_entry ≈ (16 + L + 8) × 1.5
```

where L is the key length in bytes (sum of all `key_columns` field lengths plus one `\x00` delimiter per join). Derived from D10.

| Distinct keys per dataset | L=8 (e.g. single UDPRN) | L=29 (e.g. UDPRN + email) | Approach                  |
| ------------------------- | ----------------------- | ------------------------- | ------------------------- |
| 1M                        | ~48MB                   | ~79MB                     | `in_memory`               |
| 10M                       | ~480MB                  | ~790MB                    | `in_memory` if RAM allows |
| 50M                       | ~2.4GB                  | ~3.9GB                    | `spill_to_disk`           |
| 500M+                     | ~24GB+                  | ~39GB+                    | `pairwise_approximate`    |

These are estimates from the formula — actual usage should be verified with `runtime.ReadMemStats` at representative dataset sizes before setting `max_memory_mb` in config. See D10 for the full measurement plan.

Exact algorithms build frequency maps — the `cache` config controls where they live (`in_memory` or `spill_to_disk`). Approximate algorithms use HyperLogLog and MinHash directly — no frequency map, no cache config applies. See D8, D10.

### Known Hotspots

- **Concurrent map writes:** each goroutine writes to its own frequency map — no shared state, no locking during the streaming phase.
- **Hash map insertion:** O(n) time and memory per dataset. Dominant cost at high cardinality.
- **Key join:** `strings.Join(row, "\x00")` on every row — cheap but worth profiling at high throughput.
- **CSV parsing:** parser choice matters for large files — `encoding/csv` handles quoting and CRLF correctly and is faster than naive string splitting.

### Open

- Max expected distinct key count and acceptable wall-clock time are not yet known (OQ1). These determine whether `in_memory`, `spill_to_disk`, or `pairwise_approximate` is appropriate in practice. See D10 for the sizing and measurement strategy.

---

## Privacy Boundary

Input datasets contain only the key columns specified in config — the tool has no knowledge of what those keys represent or what other data the source organisation holds. Individual key values are never logged, stored, or transmitted. Output contains only aggregate counts. This holds regardless of connector type or key column choice. See `08-security.md` for the full data classification.

---

## Auth

This is a local CLI tool. No authentication or authorisation concern — the tool has access to whatever files the OS user running it has permission to read.
