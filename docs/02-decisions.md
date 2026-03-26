# Design Decisions

## Summary

**Domain Model**

| #   | Question                                                                    | Decision                                           |
| --- | --------------------------------------------------------------------------- | -------------------------------------------------- |
| D1  | How should duplicate keys be counted for total overlap (multiplicity rule)? | `m × n` (cartesian product) per shared key, summed |
| D2  | Should keys be treated as strings or normalised integers?                   | TBD                                                |
| D3  | How are multi-column CSV files handled — which column is the key?           | `--key-columns` required flag, comma-separated column names, no default |

**Algorithm**

| #   | Question                                                             | Decision |
| --- | -------------------------------------------------------------------- | -------- |
| D4  | In-memory hash map vs streaming/external approach for large files?   | Streaming via `KeyIterator` interface — never bulk load |
| D5  | Exact counts vs probabilistic approximation (HyperLogLog / MinHash)? | TBD      |
| D6  | Single-pass vs multi-pass over the files?                            | Single-pass per dataset |

**System Boundaries**

| #   | Question                                                   | Decision |
| --- | ---------------------------------------------------------- | -------- |
| D7  | CLI argument parsing — positional args vs flags?           | TBD      |
| D8  | Output format — plain text table vs structured (JSON/CSV)? | TBD      |

---

## Domain Model

## D1: Total overlap multiplicity rule

**Decision:** `m × n` (cartesian product) per shared key, summed across all shared keys.

**Context:** The spec example resolves to `m × n`, not `min(m, n)`. Working through the example confirms this:

```
Key A: 1×1 = 1
Key C: 1×2 = 2
Key D: 2×1 = 2
Key F: 2×3 = 6
Total = 11  ✓
```

**Alternatives considered:**

Using `Dataset A: A B C D D E F F` and `Dataset B: A C C D F F F X Y`:

```
Shared key counts:
  A: 1 in A, 1 in B
  C: 1 in A, 2 in B
  D: 2 in A, 1 in B
  F: 2 in A, 3 in B

m × n:       (1×1) + (1×2) + (2×1) + (2×3) = 1 + 2 + 2 + 6 = 11  ← spec answer
min(m, n):   min(1,1) + min(1,2) + min(2,1) + min(2,3) = 1 + 1 + 1 + 2 = 5
m + n:       (1+1) + (1+2) + (2+1) + (2+3) = 2 + 3 + 3 + 5 = 13
```

- `m × n` (cartesian product) — counts every record-pair match across both files; chosen. Real world: a retailer and a bank both have records for the same address. The retailer has 2 records for it (e.g. two household members who are customers) and the bank has 3 (three account holders at that address). There are 6 possible retailer-bank record pairings for that address — `m × n = 6`. If instead those 2 retailer records are the same person entered twice (a duplicate), the result is still 6 — which is now inflated. This is intentional: a large divergence between total overlap and distinct overlap is the signal that duplicates exist and the data needs cleaning before any analysis is trusted.
- `min(m, n)` — models exclusive assignment where each record can only be matched once. Real world: a recruitment platform has 2 candidates with a given skill and 3 job openings requiring that skill. You can only place 2 candidates before you run out of people — `min(2,3) = 2`.
- `m + n` (sum of occurrences) — counts total occurrences of each shared key across both files combined. Real world: a fraud detection team wants to know how many times a suspicious postcode appears across two transaction logs in total — not how many pairs match, just the raw volume. If postcode `SW1A 1AA` appears 5 times in one log and 8 times in another, `m + n = 13` tells you how many transactions reference that postcode across both sources combined.

**Why:** `m × n` represents the number of row pairs across the two files that share the same key — equivalent to a join cardinality. This is meaningful in InfoSum's context: a person appearing twice in dataset A and three times in dataset B represents 6 linkable record pairs. It also serves as a data quality signal: in a clean dataset total overlap equals distinct overlap; significant divergence indicates duplicate records.

---

## D2: Key normalisation — string vs integer

**Decision:** TBD

**Context:** UDPRN values appear to be 8-digit numeric strings. The sample data contains values with leading zeros (e.g. `08034283`). Treating as integer would silently strip the leading zero and could cause incorrect joins if one file stores `08034283` and another stores `8034283`.

**Alternatives considered:**

- Store as raw string: preserves leading zeros, no normalisation ambiguity
- Parse as integer: loses leading zeros unless re-padded; risky unless the spec guarantees consistent formatting
- Normalise to zero-padded 8-char string: handles mixed formats but requires knowing the canonical width

**Why:** TBD — needs confirmation on whether leading zeros are semantically significant (OQ2).

---

## D3: Multi-column CSV handling

**Decision:** Support one or more named key columns, specified at runtime via a `--key-columns` flag (comma-separated column names). When multiple columns are specified, their values are concatenated with a delimiter to form a composite key string. The rest of the algorithm is unchanged.

**Context:** The current sample files are single-column CSVs with a `udprn` header, but the platform is designed to handle richer datasets where a row may be identified by multiple columns (e.g. `udprn`, `email`, `loyalty_card_id`). The solution must be configurable without code changes.

**Alternatives considered:**

- Always use the first column as the key — simple but not extensible; breaks on multi-column or reordered files
- Allow a `--key-column` flag (single column name) — handles the current data but requires code change to support composite keys later
- Allow `--key-columns` (comma-separated list) — handles both single and composite keys; chosen
- Require the key column to be named `udprn` — brittle, breaks for any other key type

**Why:** Composite key support is a stated requirement of the platform context. Making it a required flag keeps the algorithm generic and forces the caller to be explicit — key extraction is a configurable pre-processing step, and the intersection logic operates on opaque strings regardless of how many source columns were combined. There is no default: omitting `--key-columns` is a hard error. A default (first column, all columns) risks silently wrong results if the file structure changes or contains non-key columns; in a privacy-sensitive platform, silent incorrectness is unacceptable.

---

## D4: Streaming via KeyIterator

**Decision:** Both datasets are consumed via a `KeyIterator` interface — one key at a time, never bulk loaded into memory.

**Context:** The ingestion layer must support sources beyond local files (REST API, database, SFTP). Bulk loading is incompatible with remote sources that can only be iterated, and with datasets that exceed available RAM.

**Alternatives considered:**

- Bulk load both datasets into memory — incompatible with remote connectors and large datasets; ruled out
- Stream dataset A into a frequency map, stream dataset B for comparison — chosen; O(distinct keys in A) memory, single pass per dataset, works with any connector
- Sort-merge join on disk — O(1) extra memory, exact, but requires temp disk space and significant complexity; deferred unless in-memory approach proves insufficient

**Why:** Streaming is the only design compatible with the connector abstraction. The algorithm is identical regardless of whether the source is a local CSV, a paginated API, or a database cursor.

---

## D5: Exact vs approximate counts

**Decision:** TBD

**Context:** The spec explicitly raises the approximation question: "If approximations are used, ensure the accuracy of the values is appropriately represented." This is only relevant if D4 resolves to a probabilistic approach.

**Alternatives considered:**

- Exact counts via hash map or sort-merge: always correct, memory/time bounded
- HyperLogLog for distinct counts: ~1–2% error, very low memory (a few KB regardless of cardinality)
- MinHash / Jaccard estimation for overlap: estimates set similarity, not the raw overlap count directly

**Why:** TBD — if files fit in memory, exact is preferable and simpler to explain. Approximation adds complexity and requires communicating error bounds in output.

---

## D6: Single-pass per dataset

**Decision:** One pass per dataset — stream dataset A into a frequency map, then stream dataset B once to compute all four metrics.

**Context:** Total key count and total overlap both require knowing frequencies. Distinct count and distinct overlap require only presence. All four can be derived from a single frequency map pass per dataset.

**Alternatives considered:**

- Two passes per dataset (one for total count, one for distinct) — reads each source twice; wasteful for large files and incompatible with non-seekable remote sources
- Single pass with frequency map — chosen; collects all information needed in one pass per dataset

**Why:** Non-seekable remote sources (API streams, database cursors) cannot be rewound. A single-pass design is required for the connector abstraction to work correctly across all source types.

---

## D7: CLI argument parsing

**Decision:** TBD

**Alternatives considered:**

- Positional arguments: `program fileA.csv fileB.csv` — simple, no flag parsing needed
- Named flags: `program --file-a A_f.csv --file-b B_f.csv` — self-documenting, easier to extend with `--key-column` later

**Why:** TBD.

---

## D8: Output format

**Decision:** TBD

**Alternatives considered:**

- Plain text table: human-readable, matches the "display" framing in the spec
- JSON: machine-readable, useful if output is piped to another tool
- Both (flag-controlled): flexible but adds complexity

**Why:** TBD — the spec says "display", implying human-readable is the primary target.
