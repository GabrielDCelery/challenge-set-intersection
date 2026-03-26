# Entities

Entity definitions, fields, and relationships. This is the intermediate step between requirements and schema — reason through what needs to exist and why before committing to a data model.

Note: this is a stateless CLI tool. There is no persistent schema. These "entities" are in-memory data structures that exist for the duration of a single program run.

---

## Dataset

Represents one parsed CSV file. It is the primary unit of input. A Dataset holds all the keys read from a file and is the source for all four computed statistics.

| Field         | Type              | Notes                                                               |
| ------------- | ----------------- | ------------------------------------------------------------------- |
| `file_path`   | string            | Absolute or relative path to the CSV file as provided by the caller |
| `total_count` | uint64            | Total number of key rows read (including duplicates)                |
| `key_counts`  | map[string]uint64 | Frequency map: key value -> number of occurrences in this file      |

**Derived from key_counts:**

- `distinct_count` = len(key_counts)

**Open:** Should `key_counts` store keys as raw strings (preserving leading zeros) or normalised integers? See D2 in `02-decisions.md`.

---

## Key

Represents a single UDPRN value (or other key type in future) read from a CSV row.

| Field   | Type   | Notes                                                     |
| ------- | ------ | --------------------------------------------------------- |
| `value` | string | Raw string value from the CSV cell, trimmed of whitespace |

**Open:** Is an empty key value (blank cell) a parse error, or should it be silently skipped? This affects files with trailing newlines or malformed rows.

---

## IntersectionResult

Represents the computed statistics from comparing two Datasets. This is the output entity — it holds only counts, never individual key values. No PII or identifiable data ever enters this structure.

| Field              | Type   | Notes                                                              |
| ------------------ | ------ | ------------------------------------------------------------------ |
| `total_count_a`    | uint64 | Total key count for dataset A (including duplicates)               |
| `total_count_b`    | uint64 | Total key count for dataset B (including duplicates)               |
| `distinct_count_a` | uint64 | Count of distinct keys in dataset A                                |
| `distinct_count_b` | uint64 | Count of distinct keys in dataset B                                |
| `distinct_overlap` | uint64 | Count of keys appearing in both datasets (regardless of frequency) |
| `total_overlap`    | uint64 | Sum of count_in_A × count_in_B across all shared keys              |

**Open:** If approximation is used (see D5 in `02-decisions.md`), each count field should carry an associated error bound. Should the error bound be part of this struct, or represented as a separate wrapper?

---

## IntersectionAlgorithm

The interface the computation layer implements. Accepts N connectors and returns an `IntersectionResult`. Owns all decisions about memory strategy, parallelism, and accuracy — none of these concerns leak into the connector or writer layers.

```go
type IntersectionAlgorithm interface {
    Compute(datasets []KeyIterator) (IntersectionResult, error)
}
```

**Current implementation:** `PairwiseAlgorithm` — requires exactly two datasets, exact counts, frequency map held in memory.

**Future implementations:**
- `NWayAlgorithm` — N datasets, all pairwise and multi-way region breakdowns
- `ApproximateAlgorithm` — probabilistic counts via HyperLogLog/MinHash for very large datasets

---

## ConnectorStats

Accumulated statistics from a single connector run. Populated by the connector during iteration and readable at any point via `Stats()`. The algorithm checks this after each batch to enforce the `max_error_rate` threshold.

| Field         | Type       | Notes                                                                 |
| ------------- | ---------- | --------------------------------------------------------------------- |
| `rows_read`   | uint64     | Total rows yielded by the connector, including skipped rows           |
| `rows_skipped`| uint64     | Rows skipped due to malformed data                                    |
| `errors`      | []RowError | Per-row error details (row number and reason) for skipped rows        |

**RowError fields:**

| Field       | Type   | Notes                                      |
| ----------- | ------ | ------------------------------------------ |
| `row_number`| uint64 | 1-based row number in the source           |
| `reason`    | string | Human-readable description of the problem |

**Error rate** = `rows_skipped / rows_read`. If this exceeds `max_error_rate` after a batch the algorithm aborts with a non-zero exit code.

---

## ParseConfig

Configuration supplied by the user at runtime, controlling how files are parsed.

| Field         | Type     | Notes                                                                                                                                                                                                       |
| ------------- | -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `file_path_a` | string   | Path to the first CSV file                                                                                                                                                                                  |
| `file_path_b` | string   | Path to the second CSV file                                                                                                                                                                                 |
| `key_columns` | []string | **Required.** Ordered list of column names to use as the key. If multiple, values are joined with `\|` delimiter to form a composite key string. Omitting this field is a hard error — there is no default. |
| `has_header`  | bool     | Whether the first row is a header to skip (default: true)                                                                                                                                                   |

**Composite key construction:** for a row with `udprn=30433784` and `email=alice@example.com`, `key_columns=["udprn","email"]` produces the key string `"30433784|alice@example.com"`. The delimiter `|` is chosen as it does not appear in UDPRN or common email values. The intersection algorithm then operates on these opaque strings with no further changes.

**Note:** Column resolution is always by name (matched against the header row), not by index. This is more robust to column reordering and makes the config self-documenting. If any named column is not present in the file header, the program exits with an error before processing any rows.
