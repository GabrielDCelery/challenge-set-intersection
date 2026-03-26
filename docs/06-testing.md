# Testing

## What to Test

The correctness of four computed counts is the entire functional surface of this program. The most important thing to protect is that the total overlap formula matches the spec's definition exactly, and that no count is inflated by double-counting or deflated by mishandling duplicates.

- The total overlap calculation must match the spec example: `count_in_A × count_in_B` per shared key, summed.
- Distinct overlap must count shared key types, not shared occurrences.
- Total count must include all rows, including duplicates.
- Distinct count must deduplicate correctly.
- Leading zeros in UDPRN keys must not be silently stripped.
- Key column selection must be configurable: the algorithm must produce correct results when `key_columns` in the YAML config names any valid column, not just `udprn`.
- Composite keys (multiple columns) must be constructed and compared correctly — two rows match only when all specified columns match.

---

## Testing Strategy

### Unit Testing

Pure functions that can be tested in isolation without file I/O:

- The overlap computation logic: given two frequency maps, assert the four output counts are correct.
- The CSV row parser: given a raw string line, assert the key is extracted correctly (handles quoting, whitespace trimming, leading zeros).
- The key normalisation function (if one exists): assert that `"08034283"` and `"08034283"` are equal, and `"08034283"` and `"8034283"` behave according to the decided normalisation rule.

### Integration Testing

Tests that exercise real file I/O with fixture CSV files:

- End-to-end run against small fixture files with known expected outputs.
- Files with only duplicate keys (e.g. file A = `[A, A, A]`, file B = `[A, A]`) — total overlap should be 2, distinct overlap should be 1.
- Files with zero overlap.
- Files with identical content — distinct overlap should equal distinct count of either file.
- Empty file (zero rows after header) — all counts should be zero.
- File with a single row.
- File with no header row — tests the `has_header: false` config path.

### End-to-End Testing

Run the compiled binary against the provided `data/A_f.csv` and `data/B_f.csv` and assert the output format and values are correct. This is the primary acceptance test.

---

## Key Scenarios

| Scenario                                                       | Type        | Why it matters                                                                    |
| -------------------------------------------------------------- | ----------- | --------------------------------------------------------------------------------- |
| Scenario                                                              | Type        | Why it matters                                                                    |
| --------------------------------------------------------------------- | ----------- | --------------------------------------------------------------------------------- |
| Spec worked example (A B C D D E F F vs A C C D F F F X Y)           | Unit        | Validates total overlap formula against the canonical definition                  |
| data/A_f.csv vs data/B_f.csv with `key_columns: [udprn]`             | E2E         | Primary acceptance test — real input files, named column selection                |
| Config file missing `key_columns`                                     | E2E         | Confirms the program exits with a clear error and non-zero exit code — no default |
| Single-column fixture, non-default column name                        | Integration | `key_columns: [email]` on a file with an `email` column — correct column is used  |
| Multi-column fixture, composite key                                   | Integration | `key_columns: [udprn, email]` — rows match only when both columns match           |
| Multi-column fixture, partial match only                              | Integration | Row shares `udprn` but not `email` — should NOT count as overlap                  |
| File A has all duplicates, file B has all unique                      | Integration | Ensures distinct count deduplicates and total overlap uses m×n                    |
| Zero overlap between two files                                        | Integration | Ensures overlap counts are 0, not negative or undefined                           |
| Both files are identical                                              | Integration | Distinct overlap == distinct count; total overlap == total count of either        |
| One or both files are empty (header only)                             | Integration | Ensures graceful handling of zero-row input                                       |
| File with leading-zero UDPRN keys                                     | Unit        | Ensures `08034283` is not parsed as integer `8034283`                             |
| `key_columns` names a column that does not exist in the file          | Integration | Ensures clear error message and non-zero exit code                                |
| File path does not exist                                              | Integration | Ensures clear error message and non-zero exit code                                |
| Missing `--config` flag                                               | E2E         | Confirms the program exits with a clear error — config is mandatory               |
| Very large file (if generated fixture)                                | Integration | Performance and memory ceiling validation                                         |

---

## Open Questions

- How should the total overlap for the actual data files be verified independently? Is there a reference answer to compare against?
- If approximation is used for large files, what tolerance is acceptable in test assertions? (e.g. assert result is within 2% of exact value)
- Should the test suite include a generated large-file fixture to validate performance characteristics, or is that out of scope for the challenge?
