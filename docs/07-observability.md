# Observability

Strategy for understanding program behaviour and diagnosing problems. This is a CLI tool — there is no production service to monitor. Observability here means making a single invocation debuggable and its progress visible for large inputs.

---

## Logging

### Strategy

This tool does not need structured logging to a log aggregator. What it needs is:

- **stderr for diagnostic output** — progress indicators, warnings, and errors go to stderr so they do not pollute the stdout result.
- **stdout for results only** — the four computed counts, cleanly formatted.
- **Verbosity flag** — a `-v` or `--verbose` flag to enable diagnostic output (rows read, time elapsed, memory used). Off by default.

Diagnostic messages worth emitting at verbose level:

- Number of rows read per file after parsing completes.
- Wall-clock time for each phase (file A parse, file B parse, intersection compute).
- If approximation is used: the estimated error bound alongside each count.

### Tradeoffs

- **Too little:** silent failure is hard to debug — a malformed file that skips all rows would produce zeros with no explanation.
- **Too much:** noise on stderr for a simple tool is annoying. Default should be quiet.

### Open Questions

- Should skipped/malformed rows (e.g. blank lines, rows with wrong column count) be counted and reported as a warning, or silently ignored?
- If a file is very large and processing is slow, should the program emit a progress indicator (rows processed / estimated total)?

---

## Metrics

This is a batch CLI tool — there are no runtime metrics to collect. The output of the program is itself the "metric."

If performance profiling is needed during development:

| What to measure           | Why it matters                                          |
| ------------------------- | ------------------------------------------------------- |
| Wall-clock time per phase | Identifies whether bottleneck is I/O or computation     |
| Peak memory usage         | Validates whether in-memory approach is viable at scale |
| Rows parsed per second    | Baseline for estimating runtime on larger files         |

These are development-time concerns, not production metrics.

---

## Alerting

Not applicable — this is a CLI tool, not a running service.

---

## Tracing

Not applicable — single process, synchronous, no distributed components.

---

## Exit Codes

The program should use standard exit codes to allow scripting and CI integration:

| Exit code | Meaning                                        |
| --------- | ---------------------------------------------- |
| 0         | Success — all four counts computed and printed |
| 1         | General error — file not found, parse failure  |
| 2         | Usage error — invalid arguments                |
