# Tooling

**Language: Go.** Chosen to match InfoSum's primary backend language (Go proficiency is a stated requirement in their job description). All tool choices below are Go-specific.

---

## CSV Parsing

### `encoding/csv` (stdlib) — **chosen**

Handles quoted fields, embedded commas, and CRLF line endings correctly. No dependency. Stream-oriented — reads row by row without loading the full file. No automatic header detection — the first row is read manually and used to build the column-name-to-index map required by `--key-columns`.

---

## CLI Argument Parsing

### `flag` (stdlib) — **chosen**

Simple named flag parsing, no dependency. Sufficient for the two file path arguments and `--key-columns` flag. `cobra` would be overkill for a single-command tool.

---

## Probabilistic Data Structures (if large-file approximation is needed)

### HyperLogLog — for distinct count approximation

Counting distinct elements with ~1–2% error using kilobytes of memory regardless of cardinality.

**Library:** `axiomhq/hyperloglog` (more actively maintained than `clarkduvall/hyperloglog`)

### MinHash / Jaccard — for set similarity estimation

Estimates the fraction of shared elements between two sets without materialising either set. Gives a Jaccard similarity ratio, not a raw count — converting back requires knowing set sizes separately.

---

## Testing

### `testing` (stdlib) + `testify` — **chosen**

stdlib `testing` with table-driven tests covers all scenarios. `testify` (`assert` and `require` packages) reduces assertion boilerplate. Effectively standard in Go projects.

---

## Task Running

### Makefile — **chosen**

Universal — available on any Unix-like system without installation. Familiar to reviewers. Targets: `build`, `test`, `run`.
