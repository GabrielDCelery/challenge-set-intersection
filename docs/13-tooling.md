# Tooling

Recommended packages and tools by concern. Multiple options are listed per category. No code snippets — this is a decision aid, not a tutorial.

This is a CLI tool for set intersection computation on CSV files. Language choice is not specified in the brief, so options are given for the languages listed in the project context (TypeScript/Node.js, Go, Python).

---

## Language

### Go

**Good for:** Compiled to a single static binary with no runtime dependency. Fast startup, excellent memory efficiency for hash maps, strong standard library CSV parser (`encoding/csv`). Easy to cross-compile for the reviewer's OS.

**Tradeoffs:** More verbose than Python for small scripts. No native HyperLogLog in stdlib (use a third-party package if needed).

### TypeScript (Node.js)

**Good for:** Familiar to the team. Strong CSV parsing ecosystem. Type safety helps reason about the frequency map and result types.

**Tradeoffs:** Requires Node.js installed on the reviewer's machine. Slower startup than Go. Memory usage for large maps is higher due to V8's object overhead.

### Python

**Good for:** Fast to write, excellent for data manipulation tasks. `csv` module in stdlib. Easy to generate large fixture files for performance testing.

**Tradeoffs:** Slower than Go for CPU-bound operations at scale. Requires Python runtime installed. Dictionary memory overhead is higher than a typed Go map.

---

## CSV Parsing

### Go: `encoding/csv` (stdlib)

**Good for:** Handles quoted fields, embedded commas, and CRLF line endings correctly. No dependency. Stream-oriented — reads row by row without loading the full file.

**Tradeoffs:** No automatic header detection — caller must skip the first row explicitly.

### Node.js: `csv-parse`

**Good for:** Full-featured, handles all CSV edge cases, supports streaming and async iteration. Widely used.

**Tradeoffs:** External dependency. Async API adds minor complexity compared to a sync stdlib parser.

### Node.js: `papaparse`

**Good for:** Simple API, auto-detects header rows, handles malformed CSV more gracefully than `csv-parse`.

**Tradeoffs:** Primarily a browser library — Node.js support works but feels like a secondary target.

### Python: `csv` (stdlib)

**Good for:** Handles standard CSV correctly, stream-oriented, no dependency.

**Tradeoffs:** Does not handle non-standard delimiters or encodings without configuration.

---

## CLI Argument Parsing

### Go: `flag` (stdlib)

**Good for:** Simple named flags and positional argument parsing. No dependency.

**Tradeoffs:** Less ergonomic than `cobra` for subcommands — not needed here.

### Go: `cobra`

**Good for:** If the tool needs subcommands or a richer CLI in future.

**Tradeoffs:** Overkill for a two-argument CLI.

### Node.js: `commander`

**Good for:** Clean flag and positional argument parsing. Well-maintained.

**Tradeoffs:** External dependency for something that could be done with `process.argv` parsing for a two-argument CLI.

### Python: `argparse` (stdlib)

**Good for:** Auto-generates usage messages, handles type coercion, supports optional flags cleanly.

**Tradeoffs:** Verbose for simple cases.

---

## Probabilistic Data Structures (if large-file approximation is needed)

### HyperLogLog — for distinct count approximation

**Good for:** Counting distinct elements with ~1–2% error using kilobytes of memory regardless of cardinality. Well-suited for very high distinct key counts.

**Tradeoffs:** Not exact — cannot replace the precise distinct count without accepting error. Requires communicating error bounds in output.

**Libraries:**

- Go: `axiomhq/hyperloglog` or `clarkduvall/hyperloglog`
- Python: `datasketch`
- Node.js: `hyperloglog` (npm)

### MinHash / Jaccard — for set similarity estimation

**Good for:** Estimating the fraction of shared elements between two sets without materialising either set fully.

**Tradeoffs:** Gives a similarity ratio (Jaccard index), not a raw overlap count. Converting back to a count requires knowing the set sizes separately. More complex to implement correctly than HyperLogLog.

---

## Testing

### Go: `testing` (stdlib) + `testify`

**Good for:** stdlib `testing` is sufficient for table-driven tests. `testify` adds `assert.Equal` and `require` helpers that reduce boilerplate.

**Tradeoffs:** `testify` is an external dependency, but it is effectively standard in Go projects.

### Node.js: `vitest`

**Good for:** Fast, TypeScript-native, excellent watch mode. Replaces Jest for most new TypeScript projects.

**Tradeoffs:** Newer than Jest — smaller ecosystem of extensions, but sufficient for a CLI tool.

### Python: `pytest`

**Good for:** Simple test discovery, parametrize decorator for table-driven tests, clean output.

**Tradeoffs:** External dependency, but pytest is the de facto standard.

---

## Task Running

### Makefile

**Good for:** Universal — works on any Unix-like system without installing anything. Familiar to most reviewers.

**Tradeoffs:** Syntax is arcane, tab-sensitivity is a footgun, poor Windows support.

### Taskfile (go-task)

**Good for:** YAML-based, cross-platform, cleaner syntax than Make, good for projects that need to run on Windows as well.

**Tradeoffs:** Requires `task` installed — less universally available than `make`.
