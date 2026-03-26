# Security

Security concerns, strategies, and tradeoffs. This is a local CLI tool — the threat model is narrow, but the privacy context (anonymised PII-derived data) is important to reason through explicitly.

---

## Threat Model

| Threat                                              | Likelihood | Impact | Mitigation                                                   |
| --------------------------------------------------- | ---------- | ------ | ------------------------------------------------------------ |
| Individual key values leaked in output or logs      | Low        | High   | Output only counts, never key values; no verbose key logging |
| Malicious CSV file causes excessive memory usage    | Low        | Medium | Document memory limits; consider row/key count cap           |
| Path traversal via file argument                    | Very low   | Low    | OS user already controls file access; no server involved     |
| Output captured and used to re-identify individuals | Low        | High   | Output is aggregate counts only — not re-identifiable        |

---

## Data Classification

| Data                     | Sensitivity   | Notes                                                                   |
| ------------------------ | ------------- | ----------------------------------------------------------------------- |
| UDPRN keys (input)       | Confidential  | Anonymised but derived from real-world delivery addresses               |
| Frequency counts per key | Confidential  | Held in memory only, never written to disk or stdout                    |
| Aggregate output counts  | Non-sensitive | Total and distinct counts reveal nothing about individual records       |
| File paths (CLI args)    | Low           | May appear in shell history; not a significant concern for a local tool |

---

## PII and Data Privacy

**What is PII in this system:** UDPRN values are not directly PII (they are address identifiers, not personal data), but they are derived from addresses and could in principle be used to infer location. The input files are described as having had PII removed before import.

**Key guarantee:** Individual key values must never appear in:

- stdout (output contains only counts)
- any log file
- any temporary file written to disk
- any error message (e.g. "key X was malformed" — log the row number, not the value)

**Strategy:**

- Process keys in memory only.
- Discard the frequency map after computation completes — do not retain it beyond the lifetime of a single run.
- In error messages, reference row numbers or file offsets, not the raw key value.

**Tradeoffs:**

- Avoiding key values in error messages makes debugging slightly harder (you cannot tell which specific value was malformed without looking at the file directly), but this is the right tradeoff given the privacy context.

---

## Encryption

### In Transit

Not applicable — no network communication.

### At Rest

The input files are read from disk as provided. The program does not write any intermediate files. No encryption concern within the tool itself. The security of the input CSV files at rest is the responsibility of the caller.

---

## Secrets Management

Not applicable — this tool has no credentials, API keys, or secrets of any kind.

---

## Open Questions

- Should the program refuse to run if the input file is world-readable (i.e. has loose permissions), or is that the caller's responsibility?
- Is there a compliance requirement (e.g. GDPR) that governs how long the input CSV files may be retained on disk after processing? That would be outside the scope of this tool but worth flagging to the platform team.
