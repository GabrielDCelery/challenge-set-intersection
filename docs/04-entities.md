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
| `total_overlap`    | uint64 | Sum of min(count_in_A, count_in_B) across all shared keys          |

**Open:** If approximation is used (see D5 in `02-decisions.md`), each count field should carry an associated error bound. Should the error bound be part of this struct, or represented as a separate wrapper?

---

## ParseConfig

Configuration supplied by the user at runtime, controlling how files are parsed.

| Field         | Type   | Notes                                                                   |
| ------------- | ------ | ----------------------------------------------------------------------- |
| `file_path_a` | string | Path to the first CSV file                                              |
| `file_path_b` | string | Path to the second CSV file                                             |
| `key_column`  | string | Name of the column to use as the key (default: first column or `udprn`) |
| `has_header`  | bool   | Whether the first row is a header to skip (default: true)               |

**Open:** Should `key_column` be a column name (string match against header) or a column index (zero-based integer)? Name matching is more robust but requires a header row.
