# Observability

Strategy for understanding program behaviour and diagnosing problems. This is a CLI tool in its current form — but in production it would run as a job within a pipeline or orchestrator. Observability decisions are made with both contexts in mind.

---

## Logging

All diagnostic output goes to stderr. Stdout carries only the final result. A clean run produces no stderr output — only the result on stdout. This makes it safe to pipe stdout to another tool without filtering noise.

`zerolog` is used for all diagnostic output — it writes structured JSON to any `io.Writer`, so redirecting from stderr to a log aggregator in production requires no code changes. See `13-tooling.md` for the full rationale.

### What is logged in the initial implementation

**Startup (before any layer runs):**

- Config file path loaded
- Source identifiers (file paths, URLs) and configured `key_columns`
- Algorithm type and caching strategy selected

**Connector layer (per source):**

- On completion: rows read, rows skipped, reasons for skipped rows, wall-clock time
- On soft failure: row number, reason, running skip rate
- On hard failure: source identifier, failure reason — program exits after this log line
- On cancellation (timeout or error in another goroutine): how far the connector got, partial `ConnectorStats`

**Algorithm layer:**

- On completion: wall-clock time for `Compute()`
- On `max_error_rate` exceeded: which source breached the threshold, final `ConnectorStats` for all sources

**Writer layer:**

- On completion: wall-clock time for `Write()`, output destination
- On failure: destination identifier, failure reason

**Job level:**

- On success: total job duration — no other stderr output
- On any failure: total job duration, failure category

### What is not logged

- Individual key values — never, under any circumstances (privacy boundary)
- Per-row debug output — too verbose for normal operation; use `runtime.ReadMemStats` profiling during development instead

**Deferred:** Progress indicators (rows processed / estimated total) are not implemented in this iteration. The timeout mechanism (`run.timeout_seconds`) is the primary safeguard for long-running processes.

---

## Metrics

This is a batch tool — there are no runtime metrics to collect separately. The structured log output from zerolog already captures the signals needed to understand a run:

**Connector layer** — one entry per source, since connectors run in parallel and a single aggregate hides which source is the bottleneck:

| Signal                     | Why it matters                                                            |
| -------------------------- | ------------------------------------------------------------------------- |
| Wall-clock time per source | Pinpoints which source is slow when connectors run in parallel            |
| Rows read per source       | Confirms the expected volume was processed                                |
| Rows skipped per source    | High skip rate on a specific source is actionable and source-attributable |

**Algorithm layer:**

| Signal             | Why it matters                                                                                                                |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------------- |
| Algorithm duration | Isolates CPU/memory cost of frequency map construction and comparison                                                         |
| Peak memory usage  | Validates whether `in_memory` strategy is viable at the target scale — measured via `runtime.ReadMemStats` during development |

**Writer layer:**

| Signal          | Why it matters                                                              |
| --------------- | --------------------------------------------------------------------------- |
| Writer duration | Expected to be negligible — a spike indicates an output destination problem |

**Job level:**

| Signal             | Why it matters                                  |
| ------------------ | ----------------------------------------------- |
| Total job duration | Primary SLA signal for the orchestrating system |

In production, total job duration and outcome are the signals the orchestrating system monitors. Per-layer and per-source timings are available in the structured log output for diagnosis when the SLA is breached.

---

## Traces

Not implemented in this iteration — the tool is a single process with no distributed components to correlate across. However, in production this tool is one step in a larger data pipeline. If that pipeline uses distributed tracing, this tool should participate so that a slow or failed run can be correlated back to the broader pipeline context — which upstream job produced the input, which downstream job is waiting, how long this step took relative to others.

The building blocks are already in place:

- `zerolog` emits structured JSON fields — a `trace_id` and `span_id` can be added to every log line with no structural changes
- A trace context (e.g. W3C `traceparent`) could be passed in via an environment variable or config field and propagated through the logger
- The `context.Context` already flows through `NextBatch` and `Compute` — a span could be derived from it at each layer boundary without touching the algorithm or connector logic

When distributed tracing is introduced to the surrounding pipeline, adding participation here is a configuration and wiring change, not an architectural one.
