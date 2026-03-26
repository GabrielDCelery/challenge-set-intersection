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

---

## Non-Functional Requirements

### Performance

- How large can the input files be? The instructions call out scalability "in terms of number of rows" and "number of unique key values" — what is the upper bound? (e.g. millions of rows, tens of millions of distinct keys?)
- What is the acceptable wall-clock time for a single run at maximum file size? (e.g. under 10 seconds, under 1 minute?)
- **Resolved:** Raw data ingestion is streaming — datasets are consumed via `KeyIterator` in batches and never fully loaded into memory.
- **Open:** The frequency map for dataset A is held in memory and grows O(distinct keys in A). For very large datasets this is a hard memory constraint. If the distinct key count exceeds available RAM, an external sort-merge or probabilistic approach (HyperLogLog) will be required. See D5 in `02-decisions.md`.
- **Deferred:** Connector-level buffer memory (e.g. a REST connector holding a page in memory during a `NextBatch()` call, or buffering for retry) is bounded by batch size and is the connector's responsibility. This is acknowledged as a concern but not addressed in this iteration.

### Scalability

- **Resolved:** Raw data ingestion is not RAM-constrained — the streaming approach handles datasets of any size on the input side.
- **Open:** The frequency map is RAM-constrained. Must it support datasets where distinct key count exceeds available RAM? If so, see D5.
- Must it support more than two datasets in a future iteration? The current spec says two — confirm whether extensibility to N datasets is in scope.

### Availability

- This is a CLI tool, not a service — availability is not a concern beyond the program completing successfully or failing with a clear exit code and message.

### Accuracy

- If the dataset size forces an approximation (e.g. HyperLogLog for distinct counts, MinHash for set similarity), what is the acceptable error margin? The instructions explicitly flag this: "If approximations are used, ensure the accuracy of the values is appropriately represented."
- Must the total and distinct overlap counts be exact, or is a probabilistic estimate acceptable for very large files?

### Data Retention

- The program does not persist any output — all results are written to stdout. No intermediate files or caches are retained unless an explicit temp-file strategy is chosen for large-file processing.

---

## Design Clarifications

[Leave empty — populated as decisions are made and their implications become clear]

---

## Open Questions

- OQ1: What is the maximum expected distinct key count in a single dataset? This determines whether the in-memory frequency map is viable or whether an external sort-merge / HyperLogLog approach is needed.
- OQ2: UDPRN is defined as an 8-digit numeric string — should leading zeros be preserved (i.e. is "08034283" distinct from "8034283")? The sample data includes leading zeros.
- OQ3: ~~Are there any other key types beyond UDPRN?~~ **Resolved** — the program supports any key type via `--key-columns`; the algorithm treats all keys as opaque strings regardless of their source or meaning.
- OQ4: ~~Is the output format fixed (stdout only)?~~ **Resolved** — output is abstracted behind a `ResultWriter` interface; stdout is the default writer, additional writers (file, JSON, API) can be added without algorithm changes.
- OQ5: ~~Should the program handle CSV files with multiple columns?~~ **Resolved** — multi-column support required; key columns specified via `--key-columns` flag.
- OQ6: What is the target runtime environment and available RAM? This determines the practical ceiling for the in-memory frequency map.
