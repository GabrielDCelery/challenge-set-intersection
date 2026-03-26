# Design Decisions

## Summary

**Domain Model**

| #   | Question                                                                    | Decision |
| --- | --------------------------------------------------------------------------- | -------- |
| D1  | How should duplicate keys be counted for total overlap (multiplicity rule)? | `m × n` (cartesian product) per shared key, summed |
| D2  | Should keys be treated as strings or normalised integers?                   | TBD      |
| D3  | How are multi-column CSV files handled — which column is the key?           | TBD      |

**Algorithm**

| #   | Question                                                             | Decision |
| --- | -------------------------------------------------------------------- | -------- |
| D4  | In-memory hash map vs streaming/external approach for large files?   | TBD      |
| D5  | Exact counts vs probabilistic approximation (HyperLogLog / MinHash)? | TBD      |
| D6  | Single-pass vs multi-pass over the files?                            | TBD      |

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

- `m × n` (cartesian product) — matches the spec example exactly; counts every record-pair match across both files; chosen
- `min(m, n)` per key — would give 1+1+1+2 = 5, not 11; answers a different question (max matchable pairs if each record can only be used once)
- `m + n` (sum of occurrences) — also not what the spec describes

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

## D4: In-memory hash map vs streaming approach

**Decision:** TBD

**Context:** The simplest correct implementation loads one file's keys into a hash map and streams the second file for comparison. For very large files (hundreds of millions of rows), RAM becomes a constraint.

**Alternatives considered:**

- In-memory hash map (hash set for distinct, frequency map for total overlap): O(n) memory on the smaller file. Fast, simple, exact. Feasible if the distinct key count fits in memory.
- Sort-merge join on disk: sort both files externally, then merge. O(1) extra memory beyond sort buffers, exact counts. Slower due to I/O.
- Probabilistic structures (HyperLogLog for cardinality, MinHash for overlap): sub-linear memory, approximate results. The spec permits this if accuracy is declared.

**Why:** TBD — gated on OQ1 (file size) and the accuracy requirements.

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

## D6: Single-pass vs multi-pass

**Decision:** TBD

**Context:** Total key count and total overlap both require knowing frequencies. Distinct count and distinct overlap require only presence.

**Alternatives considered:**

- Single pass with a frequency map: records count per key for both files, then computes all four metrics in one pass per file. Most efficient.
- Two passes per file (one for total count, one for distinct): simpler logic per pass but reads each file twice — wasteful for large files.

**Why:** Single pass is preferred — collect a frequency map on the first file, stream the second file once.

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
