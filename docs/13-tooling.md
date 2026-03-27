# Tooling

**Language: Go.** Chosen to match InfoSum's primary backend language (Go proficiency is a stated requirement in their job description). All tool choices below are Go-specific.

---

## CSV Parsing

### `encoding/csv` (stdlib) — **chosen**

Handles quoted fields, embedded commas, and CRLF line endings correctly. No dependency. Stream-oriented — reads row by row without loading the full file. No automatic header detection — the first row is read manually and used to build the column-name-to-index map from the `key_columns` YAML config value.

---

## CLI Argument Parsing

### `flag` (stdlib) — **chosen**

Simple named flag parsing, no dependency. Sufficient for the single `--config` flag. `cobra` would be overkill for a single-command tool.

---

## Probabilistic Data Structures (if large-file approximation is needed)

### HyperLogLog — for distinct count approximation

Counting distinct elements with ~1–2% error using kilobytes of memory regardless of cardinality.

**Library:** `axiomhq/hyperloglog` (more actively maintained than `clarkduvall/hyperloglog`)

### MinHash / Jaccard — for set similarity estimation

Estimates the fraction of shared elements between two sets without materialising either set. Gives a Jaccard similarity ratio, not a raw count — converting back requires knowing set sizes separately.

---

## Logging

### `zerolog` (`github.com/rs/zerolog`) — **chosen**

Zero-allocation structured JSON logger. Writes to any `io.Writer` — stderr by default, trivially redirected to a log aggregator in production without code changes. Simple chainable API with no boilerplate for a single-binary tool.

**Alternatives considered:**

- `zap` (Uber) — excellent for high-throughput services with complex logging pipelines, but over-engineered for this use case. Requires more configuration, more boilerplate, and is designed around use cases we don't have (sampling, named loggers, multiple sinks).
- `slog` (stdlib, Go 1.21+) — no dependency, structured JSON output, sufficient for basic use. Ruled out because zerolog's zero-allocation guarantee and `io.Writer` flexibility make the production migration path cleaner with minimal added cost.

---

## Testing

### `testing` (stdlib) + `testify` — **chosen**

stdlib `testing` with table-driven tests covers all scenarios. `testify` (`assert` and `require` packages) reduces assertion boilerplate. Effectively standard in Go projects.

---

## Supply Chain Verification

### `govulncheck` — **chosen**

Go's official vulnerability scanner (`golang.org/x/vuln/cmd/govulncheck`). Checks dependencies against the Go vulnerability database maintained by the Go team. Zero configuration, runs in CI alongside tests. Catches known vulnerabilities in both direct and transitive dependencies.

**Alternatives considered:**

- `nancy` (Sonatype) — scans `go.sum` against OSS Index; requires a third-party service dependency with no clear advantage over `govulncheck` for a pure Go project; ruled out
- `trivy` (Aqua Security) — broader scope: container images, filesystems, IaC; out of scope until deployment uses containers; revisit if a Dockerfile is added

### `dependabot` — **chosen**

GitHub's automated dependency update PRs with vulnerability alerts. Zero configuration for GitHub-hosted repos — enables automatic PRs when a dependency has a known vulnerability or a new version is available. Complements `govulncheck` in CI by keeping dependencies current between scans.

---

## Task Running

### Makefile — **chosen**

Universal — available on any Unix-like system without installation. Familiar to reviewers. Targets: `build`, `test`, `run`.
