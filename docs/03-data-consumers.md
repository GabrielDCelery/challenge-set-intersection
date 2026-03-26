# Data Consumers

Who needs what view of the data and why. This drives entity design and query strategy.

---

## CLI User (Data Analyst / Developer)

**What they need:**
A summary of four counts for the two input datasets: total keys per file, distinct keys per file, total overlap, and distinct overlap. They do not need to see individual keys — only aggregate statistics.

**Why:**
The purpose of the tool is to quantify the overlap between two anonymised datasets without revealing PII. Individual key values must not be surfaced in output; only counts are returned.

**Freshness requirement:** On-demand (batch, synchronous) — the user runs the program and waits for a result.

**Key queries:**

- How many rows are in file A? How many in file B?
- How many distinct UDPRN values appear in each file?
- How many distinct keys appear in both files?
- What is the maximum possible matching volume between the two files (total overlap)?

---

## Platform Engineer / Reviewer

**What they need:**
Confidence that the algorithm is correct, that it handles edge cases (empty files, all-duplicate files, zero overlap, identical files), and that performance characteristics are understood for large inputs.

**Why:**
This is a challenge submission — correctness and clarity of approach are being evaluated alongside functional output.

**Freshness requirement:** Static — cares about correctness of the computed results, not real-time data.

**Key queries:**

- Is the total overlap calculation consistent with the spec's worked example?
- Does the program handle files with all-duplicate keys without over-counting?
- What happens if one or both files are empty?
- How does memory usage scale with file size? — resolved via D10: configurable caching strategy (`in_memory` or `spill_to_disk`), with a documented sizing formula for deriving thresholds from actual dataset characteristics.

---

## Future Pipeline Consumer (Hypothetical)

**What they need:**
If the output is ever piped into another tool or script, a machine-readable format (JSON) would be needed. This is not a current requirement.

**Why:**
InfoSum's platform generates insights programmatically — future integration could consume this tool's output as structured data rather than human-readable text.

**Freshness requirement:** On-demand batch.

**Key queries:**

- Structured counts keyed by metric name, suitable for downstream aggregation.

**How it would be served:** Output format is controlled via `output.writer` in the YAML config (D7). A JSON writer would be a new `ResultWriter` implementation — no algorithm or connector changes required.
