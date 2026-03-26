# Requirements

## Functional Requirements

### Data Ingestion

The program must accept two datasets as input. Each dataset contains a header row followed by rows of key values. The program should not assume a fixed source — in the current implementation datasets are provided as local CSV files, but the ingestion layer must be designed as a pluggable connector so that future sources (REST API, database, SFTP) can be added without changing the algorithm.

- FR1: Dataset sources, key columns, and output destination are specified via a YAML config file passed with `--config`. A shorthand positional form (`program --key-columns udprn A_f.csv B_f.csv`) is supported as a convenience for two local CSV files and constructs an equivalent config internally.
- FR2: Each dataset source is consumed via a `KeyIterator` interface that streams batches of `[][]string` — one inner slice per row, one element per configured key column. The algorithm has no knowledge of the underlying source format.
- FR3: The CSV connector implements `KeyIterator`, reads the file row by row without loading it fully into memory, and resolves the configured `--key-columns` to column indices from the header row.
- FR4: The program handles datasets where key values may appear more than once (duplicates are valid input).
- FR5: Configuration errors (missing key column, source cannot be opened, missing config) are hard failures — the program exits with a non-zero exit code and a clear error message before processing any rows.
- FR6: Data errors (malformed rows) are soft failures — the connector skips the row, records it in `ConnectorStats`, and continues. The algorithm checks the error rate after each batch and aborts if it exceeds the configured `max_error_rate` threshold (set per dataset in the YAML config).
- FR7: Skipped row counts and error details are included in the final output so the caller is aware the results may be incomplete.

### Key Statistics

The core output is a set of counts derived from the two datasets. These counts must be computed accurately unless an approximation strategy is explicitly chosen (see NFRs).

- FR8: Report the total count of keys in each dataset (including duplicates).
- FR9: Report the count of distinct keys in each dataset.
- FR10: Report the total overlap — for a key appearing m times in dataset A and n times in dataset B, it contributes m×n to the total overlap, summed across all shared keys.
- FR11: Report the distinct overlap — the count of keys that appear in both datasets (regardless of frequency).

### Output

- FR12: Results are written via a `ResultWriter` interface — the algorithm has no knowledge of the output destination.
- FR13: The stdout writer formats results as a human-readable table with clearly labelled metrics, including skipped row counts from each connector.
- FR14: The output destination is configurable via the `output.writer` field in the YAML config (default: stdout).
- FR15: The output labels each metric clearly so the caller can tell which number corresponds to which statistic.
- FR16: If `run.timeout_seconds` is configured and exceeded, the program cancels all in-flight connector goroutines, flushes partial `ConnectorStats` to stderr, and exits with a non-zero exit code and a clear timeout message.
- FR17: Resume on failure is not supported in this iteration. A timed-out or failed run must be restarted from the beginning.

---

## Non-Functional Requirements

### Performance

- **Resolved:** Raw data ingestion is streaming — datasets are consumed via `KeyIterator` in batches and never fully loaded into memory.
- **Resolved:** Connectors stream in parallel — one goroutine per connector, managed by the algorithm. Wall-clock time is bounded by the slowest connector, not the sum of all connectors.
- **Resolved:** Frequency map memory is managed by the algorithm's configurable caching strategy — `in_memory` or `spill_to_disk` for exact algorithms, no map for approximate algorithms. See D11 in `02-decisions.md`.
- **Resolved:** Wall-clock time is controlled via `run.timeout_seconds` in the YAML config. If exceeded, all connector goroutines are cancelled via shared context, partial stats are flushed to stderr, and the program exits with a non-zero exit code. See D12.
- **Deferred:** Resume on failure — the architecture supports it via connector checkpointing and algorithm frequency map persistence, but it is not implemented in this iteration. See D12.
- **Deferred:** Connector-level buffer memory (e.g. a REST connector holding a page in memory during a `NextBatch()` call) is bounded by batch size and is the connector's responsibility. Acknowledged but not addressed in this iteration.

### Scalability

- **Resolved:** Raw data ingestion is not RAM-constrained — the streaming approach handles datasets of any size on the input side.
- **Resolved:** Frequency map memory is bounded by the caching strategy — `spill_to_disk` handles datasets that exceed available RAM with exact results. See D11.
- **Resolved:** N dataset extensibility is accommodated by the `nway_exact` and `nway_approximate` algorithm types. See D9.

### Availability

- This is a CLI tool, not a service — availability is not a concern beyond the program completing successfully or failing with a clear exit code and message.

### Accuracy

- **Resolved:** Approximation is a deliberate choice of algorithm type (`pairwise_approximate`, `nway_approximate`), not a forced fallback. The caller selects exact or approximate via `algorithm.type` in the YAML config. See D9.
- **Open:** If an approximate algorithm is used, what is the acceptable error margin? The spec flags this: "If approximations are used, ensure the accuracy of the values is appropriately represented." Output must include error bounds when an approximate algorithm is selected.

### Data Retention

- The program does not persist any output — all results are written to the configured `ResultWriter`. No intermediate files are retained unless `spill_to_disk` caching strategy is active, in which case temp files are written to `spill_dir` and cleaned up after the run.

---

## Design Clarifications

[Leave empty — populated as decisions are made and their implications become clear]

---

## Open Questions

- OQ1: What is the maximum expected distinct key count in a single dataset, and what is the acceptable wall-clock time? Together these determine whether `in_memory`, `spill_to_disk`, or an approximate algorithm is appropriate.
- OQ2: UDPRN is defined as an 8-digit numeric string — should leading zeros be preserved (i.e. is "08034283" distinct from "8034283")? The sample data includes leading zeros.
- OQ3: ~~Are there any other key types beyond UDPRN?~~ **Resolved** — the program supports any key type via `--key-columns`; the algorithm treats all keys as opaque strings regardless of their source or meaning.
- OQ4: ~~Is the output format fixed (stdout only)?~~ **Resolved** — output is abstracted behind a `ResultWriter` interface; stdout is the default writer, additional writers (file, JSON, API) can be added without algorithm changes.
- OQ5: ~~Should the program handle CSV files with multiple columns?~~ **Resolved** — multi-column support required; key columns specified via `--key-columns` flag.
- OQ6: ~~What is the target runtime environment?~~ **Resolved** — caching strategy (`in_memory` vs `spill_to_disk`) is configurable per run; the program adapts to the available RAM via config rather than assuming an environment.
- OQ7: If an approximate algorithm is used, what is the acceptable error margin? Required to determine whether approximate output is fit for purpose for a given use case.
