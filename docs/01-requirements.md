# Requirements

## Functional Requirements

### Data Ingestion

The program must accept two datasets as input. Each dataset contains a header row followed by rows of key values. The program should not assume a fixed source — in the current implementation datasets are provided as local CSV files, but the ingestion layer must be designed as a pluggable connector so that future sources (REST API, database, SFTP) can be added without changing the algorithm.

- FR1: The user can specify two dataset sources via command-line arguments (or a config/flag mechanism).
- FR2: Each dataset source is consumed via a `KeyIterator` interface that streams one key string at a time. The algorithm has no knowledge of the underlying source.
- FR3: The CSV connector implements `KeyIterator`, reads the file row by row without loading it fully into memory, and resolves the key string from the configured `--key-columns`.
- FR4: The program handles datasets where key values may appear more than once (duplicates are valid input).
- FR5: The program reports an error clearly if a source cannot be opened or a key cannot be read, and exits with a non-zero exit code.

### Key Statistics

The core output is a set of counts derived from the two datasets. These counts must be computed accurately unless an approximation strategy is explicitly chosen (see NFRs).

- FR6: Report the total count of keys in each dataset (including duplicates).
- FR7: Report the count of distinct keys in each dataset.
- FR8: Report the total overlap — for a key appearing m times in dataset A and n times in dataset B, it contributes m×n to the total overlap, summed across all shared keys.
- FR9: Report the distinct overlap — the count of keys that appear in both datasets (regardless of frequency).

### Output

- FR10: Results are displayed to stdout in a human-readable format.
- FR11: The output labels each metric clearly so the caller can tell which number corresponds to which statistic.

---

## Non-Functional Requirements

### Performance

- How large can the input files be? The instructions call out scalability "in terms of number of rows" and "number of unique key values" — what is the upper bound? (e.g. millions of rows, tens of millions of distinct keys?)
- What is the acceptable wall-clock time for a single run at maximum file size? (e.g. under 10 seconds, under 1 minute?)
- Is streaming/constant-memory processing required, or is loading the full key set into memory acceptable?

### Scalability

- Must the program support files that do not fit in RAM? If so, an external-sort or streaming HyperLogLog approach is needed rather than in-memory hash sets.
- Must it support more than two files in a future iteration? The current spec says two — confirm whether extensibility to N files is in scope.

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

- OQ1: What is the maximum expected file size (rows and distinct key count)? This determines whether an in-memory hash set approach is viable or whether streaming/approximation is required.
- OQ2: UDPRN is defined as an 8-digit numeric string — should leading zeros be preserved (i.e. is "08034283" distinct from "8034283")? The sample data includes leading zeros.
- OQ3: Are there any other key types beyond UDPRN that the program must support in this iteration, or is UDPRN the sole key type?
- OQ4: Is the output format fixed (stdout only), or is writing results to a file also required?
- OQ5: Should the program handle CSV files with multiple columns, or can it assume a single-column CSV of keys?
- OQ6: What is the target runtime environment — local developer machine, CI pipeline, or a server? This affects memory assumptions.
