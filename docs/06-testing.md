# Testing

Tests are organised around startup, the three layer boundaries, and end-to-end. Each has a single responsibility — tests exist to protect that responsibility, not just to achieve coverage.

---

## Startup

Startup is where the config is parsed, environment variables are expanded, and the three layers are constructed and wired together. Failures here are always hard failures — if startup does not complete cleanly, no data is processed. The most dangerous startup failure is a silent misconfiguration: a connector constructed with the wrong parameters, or an environment variable silently falling back to an empty string instead of failing loudly.

These tests exist to ensure that what gets constructed matches what was configured, and that every invalid or incomplete configuration is caught before the first row is read.

- Valid config → correct algorithm type, connectors, and writer constructed
- `${VAR}` environment variable references expanded correctly at parse time
- Unset environment variable referenced in config → hard error, non-zero exit
- `key_columns` missing from config → hard error, non-zero exit
- Invalid algorithm type → hard error, non-zero exit
- Invalid caching strategy → hard error, non-zero exit
- Input file is world-readable → hard error, non-zero exit before any rows are read
- Config file has permissions looser than `600` → hard error, non-zero exit before any rows are read
- Config file is malformed YAML → hard error, non-zero exit

---

## KeyIterator (connector layer)

The connector is the only point where raw source data enters the system. Everything downstream — the frequency map, the four computed metrics, the final output — is derived entirely from what the connector returns. This means any corruption at the connector boundary is invisible to the rest of the system: the algorithm has no way to detect that it received a wrong column, a stripped leading zero, or a silently dropped row. It simply computes on whatever it was given and produces a confident-looking result.

This is by design. The algorithm deliberately has no defensive checks against bad connector output — validating the source format is the connector's responsibility, not the algorithm's. The `KeyIterator` contract is the enforcement point. If a connector satisfies the contract, the algorithm can trust it completely. If it doesn't, there is nothing downstream to catch the failure.

This is particularly dangerous in a privacy-sensitive context where the output is aggregate counts with no individual records to cross-check against. If the connector resolves `key_columns` to the wrong column index because a source file has a different column order, the algorithm will compute the intersection of the wrong fields entirely — and the output will look structurally correct. If `08034283` is parsed as the integer `8034283`, keys that should match will not, and keys that should not match will. There is no error, no warning, just a quietly wrong count. The same applies to error reporting: if `ConnectorStats` understates `RowsSkipped`, the `max_error_rate` check passes when it shouldn't, and a degraded result gets written as if it were complete.

Finally, the `Close()` method returns only an error — not stats — so that `defer connector.Close()` works safely on all code paths. If stats were returned from `Close()`, deferred calls on error paths would discard them, leaking both the stats and potentially the underlying file handle. These tests verify that `Close()` behaves correctly on both normal and error paths.

These tests hold the connector to its contract: return the right fields, in the right order, as raw strings, and be honest about every row it could not process.

- Keys are returned as raw strings — leading zeros preserved (`08034283` != `8034283`)
- Column resolution is by name, not index — correct column extracted regardless of column order
- Composite keys return one element per configured column, in order
- Quoted fields and embedded commas are parsed correctly — `"Smith, John"` is one field, not two
- Header row is not returned as a key row — file with header and no data rows yields zero batches
- Malformed rows are skipped and recorded in `ConnectorStats`, not silently dropped or fatal
- Row with fewer fields than expected → soft failure, recorded in `ConnectorStats`
- Row with one or more empty `key_columns` fields → soft failure, recorded in `ConnectorStats`
- `Stats()` reflects accurate counts after each batch
- `Close()` releases resources without error on normal completion
- Context cancellation mid-stream → `NextBatch` returns cleanly without hanging or panicking
- Missing column name in header → hard error before any rows are processed
- File path does not exist → hard error before any rows are processed
- Empty file (header only) → zero batches, zero stats, no error

---

## IntersectionAlgorithm (computation layer)

The algorithm is where the four metrics are produced and where the entire purpose of the tool either succeeds or fails silently. Unlike the connector layer — where a wrong column or a stripped zero is at least a detectable category of error — the algorithm's failure modes are purely numerical. An off-by-one in the overlap formula, a frequency map that counts rows instead of distinct keys, or a duplicate that inflates the total overlap by the wrong multiplier all produce output that is structurally valid, plausible-looking, and completely wrong. There is no downstream check. The `ResultWriter` trusts the result it receives and writes it faithfully.

The formula itself — `m × n` per shared key, summed — is counterintuitive enough that it was ambiguous from the spec alone and required working through the canonical example to confirm. This makes it a prime candidate for an implementation error that passes basic smoke tests but produces wrong results for anything other than the simplest inputs. The spec worked example is therefore the most important single test in the suite: if it passes, the formula is correct.

The algorithm is also where concurrency is introduced. Each connector streams into its own frequency map in a separate goroutine. The maps are independent during streaming — no locking required — but the comparison phase must wait for all goroutines to complete. A race condition or premature comparison would produce non-deterministic results that are difficult to reproduce and impossible to detect from the output alone.

Finally, the `max_error_rate` check is the algorithm's only feedback mechanism about data quality. If it fails to abort when the threshold is exceeded, a result computed from heavily degraded input gets written as if it were complete and reliable.

- Total overlap = `m × n` per shared key, summed — validated against the spec worked example (A B C D D E F F vs A C C D F F F X Y → 11)
- Distinct overlap counts key types, not occurrences — `[A, A, A]` vs `[A, A]` → distinct overlap = 1, total overlap = 6
- Distinct count deduplicates correctly — frequency map collapses duplicates
- Total count includes all rows including duplicates
- Zero overlap → all overlap counts are 0, not negative or undefined
- Both datasets identical → distinct overlap == distinct count of either; total overlap == total count of either
- One dataset entirely unique, the other entirely duplicates → frequency map tracks counts correctly, not just presence
- Empty dataset (zero rows) → all counts are zero, no division-by-zero or panic
- Context cancellation → all goroutines stop cleanly, no partial result written
- `max_error_rate` exceeded → algorithm aborts with non-zero exit, partial stats flushed to stderr
- `max_error_rate` at exact threshold boundary → does not abort; one row over threshold → aborts
- Composite key matching → two rows match only when all configured columns match; partial match does not count

---

## ResultWriter (output layer)

The writer is the last thing that runs and the only thing the caller ever sees. By the time `Write()` is called, the computation is complete and cannot be revisited — if the writer omits a metric, rounds a number, suppresses an error bound, or fails to surface skipped row counts, the caller has no way to know. A result that is computed correctly but presented incorrectly is indistinguishable from a result that is wrong.

This is particularly important for the signals that indicate data quality problems. The total overlap and distinct overlap figures tell the caller whether the data is clean. But `ConnectorStats` — the count of rows skipped during ingestion — tells the caller whether the result itself can be trusted. If the writer silently drops skipped row counts, a result computed from 20% malformed input looks identical to one computed from clean data. The caller cannot make an informed decision about the result's reliability without that information.

The error bound has the same character. When an approximate algorithm is used, `ErrorBoundPct` is the only indication that the figures are estimates rather than exact counts. Suppressing it — or showing it when it should be zero for an exact result — misleads the caller about the nature of the computation.

These tests exist to ensure the writer is a faithful and complete messenger, nothing more and nothing less.

- All six metrics appear in output with correct labels
- `ErrorBoundPct` is shown when non-zero, omitted when zero
- Exact algorithm result with `ErrorBoundPct == 0` → no error bound shown
- Skipped row counts from `ConnectorStats` are included in output per source
- Zero skipped rows → skipped row counts are not shown or shown as zero, not omitted entirely
- Output is written to the configured destination without truncation

---

## End-to-End

The layer-level tests above verify each boundary in isolation using controlled inputs — mock connectors, fixed frequency maps, known result structs. What they cannot verify is that the layers compose correctly when wired together through real config parsing, real file I/O, and a real binary invocation. A bug at the seam between layers — a field that gets dropped during config parsing, a connector that is constructed with the wrong column indices, an algorithm type that is never selected because the config key is misspelled — would pass every layer test and only surface here.

End-to-end tests also serve a different audience. The layer tests are for the implementer — they verify internal contracts. The end-to-end tests are for the reviewer: run the binary, read the output, and decide whether the implementation is correct. This is the test that stands in for the acceptance criteria the spec implies but never states explicitly.

The failure modes being protected here are integration failures and configuration failures. An integration failure means the layers do not compose as designed. A configuration failure means the program accepts input it should reject, or rejects input it should accept. Both are invisible to layer-level tests.

- `data/A_f.csv` vs `data/B_f.csv` with `key_columns: [udprn]` → correct output format and values
- Missing `--config` flag → hard error, non-zero exit
- Config missing `key_columns` → hard error, non-zero exit
- `key_columns` names a column not present in the file → hard error, non-zero exit
- Config specifies a valid algorithm type → correct algorithm is selected and produces output
- Config specifies an invalid algorithm type → hard error, non-zero exit
- Timeout exceeded → non-zero exit, partial `ConnectorStats` flushed to stderr

---

## Open Questions

- How should the total overlap for `data/A_f.csv` vs `data/B_f.csv` be verified independently? Is there a reference answer to compare against?

**Resolved:** A generated large-file fixture is out of scope for this iteration. It would only be needed to validate `spill_to_disk` or `pairwise_approximate`, neither of which is implemented yet. Deferred alongside those features.
