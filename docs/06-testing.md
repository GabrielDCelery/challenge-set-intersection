# Testing

Tests are organised around the three layer boundaries. Each layer has a single responsibility — tests exist to protect that responsibility, not just to achieve coverage.

---

## KeyIterator (connector layer)

**What we're protecting:** the connector correctly extracts the configured key columns from the source and returns them as raw strings, regardless of source format. Malformed rows are soft failures. The connector never silently corrupts data.

- Keys are returned as raw strings — leading zeros preserved (`08034283` != `8034283`)
- Column resolution is by name, not index — correct column extracted regardless of column order
- Composite keys return one element per configured column, in order
- Malformed rows are skipped and recorded in `ConnectorStats`, not silently dropped or fatal
- `Stats()` reflects accurate counts after each batch
- `Close()` releases resources without error on normal completion
- Missing column name in header → hard error before any rows are processed
- File path does not exist → hard error before any rows are processed
- Empty file (header only) → zero batches, zero stats, no error

---

## IntersectionAlgorithm (computation layer)

**What we're protecting:** the four metrics are computed correctly for any input the connector produces. The formula is exact, duplicates are handled correctly, and edge cases do not produce nonsensical results.

- Total overlap = `m × n` per shared key, summed — validated against the spec worked example (A B C D D E F F vs A C C D F F F X Y → 11)
- Distinct overlap counts key types, not occurrences — `[A, A, A]` vs `[A, A]` → distinct overlap = 1, total overlap = 6
- Distinct count deduplicates correctly — frequency map collapses duplicates
- Total count includes all rows including duplicates
- Zero overlap → all overlap counts are 0, not negative or undefined
- Both datasets identical → distinct overlap == distinct count of either; total overlap == total count of either
- Empty dataset (zero rows) → all counts are zero, no division-by-zero or panic
- `max_error_rate` exceeded → algorithm aborts with non-zero exit, partial stats flushed to stderr
- Composite key matching → two rows match only when all configured columns match; partial match does not count

---

## ResultWriter (output layer)

**What we're protecting:** the writer faithfully represents the `IntersectionResult` without omission or distortion. Error bounds are surfaced when present. Skipped row counts are visible to the caller.

- All six metrics appear in output with correct labels
- `ErrorBoundPct` is shown when non-zero, omitted when zero
- Skipped row counts from `ConnectorStats` are included in output
- Output is written to the configured destination without truncation

---

## End-to-End

**What we're protecting:** the full pipeline — config parsing, connector construction, algorithm, writer — works correctly against real inputs. This is the primary acceptance test.

- `data/A_f.csv` vs `data/B_f.csv` with `key_columns: [udprn]` → correct output format and values
- Missing `--config` flag → hard error, non-zero exit
- Config missing `key_columns` → hard error, non-zero exit
- `key_columns` names a column not present in the file → hard error, non-zero exit

---

## Open Questions

- How should the total overlap for `data/A_f.csv` vs `data/B_f.csv` be verified independently? Is there a reference answer to compare against?
- Should the test suite include a generated large-file fixture to validate performance and memory characteristics, or is that out of scope for the challenge?
