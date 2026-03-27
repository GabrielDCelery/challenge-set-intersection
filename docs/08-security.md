# Security

Security concerns, strategies, and tradeoffs. This is a CLI tool in its current form — but in production it runs as a job within a pipeline or orchestrator. The threat model is narrow, but the privacy context (anonymised PII-derived data) is important to reason through explicitly.

---

## Threat Model

| Threat                                         | Mitigation                                                                                             |
| ---------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| Individual key values leaked in output or logs | Output contains only counts; key values are never logged — row numbers are used in error messages      |
| Malicious input causes excessive memory usage  | Configurable `max_memory_mb`; `spill_to_disk` bounds RAM usage; `pairwise_approximate` as escape hatch |
| Path traversal via config file paths           | In production, config is provided by the orchestrator; OS permissions apply                            |
| Output used to re-identify individuals         | Aggregate counts only — not re-identifiable                                                            |
| Auth token intercepted in transit              | HTTPS required for all REST connector endpoints — plaintext HTTP must not be used                      |

---

## Data Classification and Privacy

Key values in this system are address identifiers (UDPRN) — not directly PII, but derived from real-world delivery addresses and capable of inferring location. Input files are described as having had direct PII removed before import. Regardless, the system treats all key values as confidential and applies the same guarantees to any key type configured via `key_columns`.

### Values extracted from `key_columns` fields

**Sensitivity:** Confidential

The raw field values extracted from each source row for the configured `key_columns` — in the current data files these are UDPRN identifiers, but the guarantee applies to any key type. Anonymised but potentially sensitive depending on the key type (a UDPRN can infer a delivery address; an email is directly identifying). Must never appear in stdout, any log line, any error message, or any temporary file. Error messages reference row numbers, not key values — this makes debugging slightly harder but is the right tradeoff in a privacy-sensitive context. The program checks that input files are not world-readable before processing; loose permissions cause a hard error before the first row is read.

### Frequency counts per key

**Sensitivity:** Confidential

Held in memory for `in_memory` strategy. For `spill_to_disk`, chunks are written to `spill_dir` as temp files — these require the same access controls as the input and are deleted after the run. The caller is responsible for ensuring `spill_dir` is on an access-controlled, non-shared filesystem.

### Auth tokens (config)

**Sensitivity:** Confidential

REST connector `auth_token` is specified in the YAML config as an environment variable reference — `${REST_AUTH_TOKEN}` — expanded at parse time via `os.Getenv`. The actual token lives in the environment, injected by the orchestrator or a secrets manager. The config file itself contains no secrets and can be stored safely in version control. See D13 for the full rationale.

`auth_token` must never appear in logs — zerolog fields containing tokens must be explicitly excluded. The config file must not be world-readable regardless, as it may contain other sensitive configuration.

### Aggregate output counts

**Sensitivity:** Non-sensitive

Total and distinct counts reveal nothing about individual records. Safe to share with either party without privacy concern.

### File paths (config)

**Sensitivity:** Low

Appear in config and logs. Not sensitive in themselves but may reveal information about the deployment environment.

---

## Encryption

### In Transit

HTTPS is required for all REST connector endpoints. The `auth_token` and all key data returned by the API travel over an encrypted connection. Plaintext HTTP must not be used — the connector should refuse to connect to a non-HTTPS endpoint.

### At Rest

Input files and `spill_dir` temp files are the responsibility of the caller. The program itself does not encrypt them. In a production deployment these should reside on encrypted volumes with access restricted to the process owner.

---

## Open Questions

- Is there a compliance requirement (e.g. GDPR) that governs how long the input CSV files may be retained on disk after processing? Outside the scope of this tool but worth flagging to the platform team.
