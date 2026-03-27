# Tooling

**Language: Go.** Chosen to match InfoSum's primary backend language (Go proficiency is a stated requirement in their job description). All tool choices below are Go-specific.

Tools are grouped by concern: runtime (what the binary uses while running), quality (what verifies correctness and safety), and build and deploy (what packages and ships it).

---

# Runtime

## CSV Parsing

### `encoding/csv` (stdlib) — **chosen**

Handles quoted fields, embedded commas, and CRLF line endings correctly. No dependency. Stream-oriented — reads row by row without loading the full file. No automatic header detection — the first row is read manually and used to build the column-name-to-index map from the `key_columns` YAML config value.

No third-party library is needed here. The stdlib parser handles every edge case this tool requires, and adding a dependency for CSV parsing would introduce supply chain risk with no benefit.

---

## CLI Argument Parsing

### `flag` (stdlib) — **chosen**

Simple named flag parsing, no dependency. Sufficient for the single `--config` flag.

`cobra` is the standard choice for multi-command CLIs with subcommands, persistent flags, and shell completion — none of which apply here. Pulling in cobra for a single flag would add a dependency, increase binary size, and signal to the reader that the CLI is more complex than it is.

---

## YAML Parsing

### `gopkg.in/yaml.v3` — **chosen**

Standard YAML parsing library for Go. Handles anchors, multi-document files, and complex types correctly. Well-established and actively maintained — the de facto choice for Go projects that need YAML config parsing. Used at startup to parse the `--config` file into the `RunConfig` struct before the three layers are constructed.

`viper` is a popular alternative that handles YAML, environment variables, and flags in a unified config system. It is over-engineered for this use case — environment variable expansion is handled explicitly via `os.Getenv` at parse time (D13), and the additional abstraction layer would obscure rather than simplify the config loading path.

---

## Logging

### `zerolog` (`github.com/rs/zerolog`) — **chosen**

Zero-allocation structured JSON logger. Writes to any `io.Writer` — stderr by default, trivially redirected to a log aggregator in production without code changes. Simple chainable API with no boilerplate for a single-binary tool.

`zap` (Uber) is excellent for high-throughput services that need sampling, named loggers, and multiple configurable sinks. For a single-binary batch job that runs once and exits, none of those features are relevant — and zap's setup cost (choosing between the sugared and non-sugared API, configuring zapcore) adds boilerplate without benefit.

`slog` (stdlib, Go 1.21+) is a closer call. It has no dependency, produces structured JSON, and is sufficient for basic use. The deciding factor is zerolog's `io.Writer` interface — in production, redirecting logs to a structured aggregator (CloudWatch Logs, Datadog) is a matter of swapping the writer with no changes to log call sites. Zerolog's zero-allocation guarantee is a secondary benefit; the cleaner production migration path is the primary reason.

---

# Quality

## Linting

### `golangci-lint` — **chosen**

Aggregates multiple Go linters in a single pass — `staticcheck`, `errcheck`, `govet`, and others. Zero configuration for basic use; a `.golangci.yml` can pin specific linters if needed. Runs in CI alongside `go test` and `govulncheck`. Catches error handling gaps, shadowed variables, and other issues that the compiler does not flag.

Running linters individually is the alternative — but `golangci-lint` is faster (runs linters in parallel, caches results) and has become the standard way to run Go static analysis in CI. There is no meaningful reason to prefer individual linter invocations over it.

---

## Testing

### `testing` (stdlib) + `testify` — **chosen**

stdlib `testing` with table-driven tests covers all scenarios. `testify` (`assert` and `require` packages) reduces assertion boilerplate without adding complexity — a failed assertion prints a clear diff rather than a raw `if got != want` message. Effectively standard in Go projects.

`gomock` and `mockery` were considered for mocking connector interfaces in algorithm tests. Ruled out — the KeyIterator interface is simple enough that a small hand-written stub is clearer than generated mocks, and avoids adding a code generation step to the build.

---

## Supply Chain Verification

### `govulncheck` — **chosen**

Go's official vulnerability scanner (`golang.org/x/vuln/cmd/govulncheck`). Checks dependencies against the Go vulnerability database maintained by the Go team. Zero configuration, runs in CI alongside tests. Catches known vulnerabilities in both direct and transitive dependencies.

`nancy` (Sonatype) scans `go.sum` against OSS Index, but requires a third-party service dependency and is not Go-specific. For a pure Go project, `govulncheck` queries a database curated specifically for Go modules by the Go team — narrower scope, higher signal, no external service required.

### `trivy` (Aqua Security) — **chosen** for image scanning

Broader scope than `govulncheck`: scans container images for OS-level vulnerabilities in addition to application dependencies. Used in CI to scan the Docker image after it is built. Complements `govulncheck` — the two tools cover different layers of the supply chain.

### `dependabot` — **chosen**

GitHub's automated dependency update PRs with vulnerability alerts. Zero configuration for GitHub-hosted repos — enables automatic PRs when a dependency has a known vulnerability or a new version is available. Complements `govulncheck` in CI by keeping dependencies current between scans.

---

# Build and Deploy

## Containerisation

### Dockerfile — **chosen**

Multi-stage build: a build stage compiles the Go binary, a minimal runtime stage (`scratch` or `alpine`) packages it. The resulting image has no Go toolchain dependency and is safe to distribute. The config file and data directory are mounted at runtime — nothing sensitive is baked into the image.

`scratch` produces the smallest possible image but requires the binary to be statically linked and provides no shell for debugging. `alpine` adds a minimal shell and CA certificates, which is useful if the REST connector needs to verify TLS certificates. The choice between them depends on whether a debug shell in the runtime image is acceptable — for a production deployment, `scratch` is preferred.

---

## Task Running

### Makefile — **chosen**

Universal — available on any Unix-like system without installation. Familiar to any reviewer. Targets: `build`, `test`, `run`, `docker-build`, `docker-run`.

`mise` is a stronger choice for a team repository — it pins Go version and tool versions in a single `mise.toml`, eliminating "works on my machine" problems when multiple contributors are on different Go versions. Not chosen here because it requires installation and adds friction for a reviewer who just wants to run the tool. Worth adopting if this moves to a shared codebase.

---

# Deferred

## Probabilistic Data Structures

Not implemented in this version. Required for the `pairwise_approximate` strategy in Slice 7. Library choices need further investigation before adoption.

- **HyperLogLog** — for distinct count approximation. Candidate library: `axiomhq/hyperloglog`. Needs verification against `clarkduvall/hyperloglog` on maintenance and API fit.
- **MinHash** — for set similarity estimation. No library selected yet — needs evaluation against the same criteria as the HyperLogLog choice: active maintenance, pure Go, no unnecessary dependencies.
