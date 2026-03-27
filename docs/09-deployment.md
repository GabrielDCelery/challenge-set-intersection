# Deployment

How this program gets built and distributed. In its current form this is a CLI tool submitted as a challenge — but the design accounts for production deployment as a pipeline job.

---

## Deployment Target

**Current:** A compiled Go binary run locally against the provided `data/` CSV files. The submission includes source code and a Makefile with `build`, `test`, and `run` targets.

**Production:** A job within a data pipeline or orchestrator. The binary is the same — what changes is how it is invoked, how config is supplied, and how secrets are injected. No code changes are required to move from local to production.

---

## Environments

| Environment | Purpose                                      |
| ----------- | -------------------------------------------- |
| local       | Development, testing, and challenge submission |
| production  | Pipeline job — config and secrets injected by orchestrator |

---

## CI/CD

A GitHub Actions workflow runs on every push:

1. `go build` — verifies the binary compiles
2. `go test ./...` — runs the full test suite
3. `govulncheck` — scans dependencies for known vulnerabilities

The Makefile targets (`build`, `test`, `run`) mirror the CI steps so local and CI behaviour are identical.

---

## Containerisation

Deferred — not required for the challenge submission. A Dockerfile would be useful to guarantee the reviewer can run the program without installing Go. If added:

- Multi-stage build: build stage compiles the binary, runtime stage is minimal (`scratch` or `alpine`)
- The config file is mounted at runtime — not baked into the image
- The `data/` directory is mounted as a volume

If containerised, `trivy` should be added to CI to scan the image for OS-level vulnerabilities. See `13-tooling.md`.

---

## Network Access

REST connectors make outbound HTTPS calls to configured API endpoints. No inbound network access. HTTPS is required — plaintext HTTP must not be used. See `08-security.md`.

---

## Secrets and Environment Variables

Secrets are injected via environment variables and referenced in the YAML config using `${VAR}` syntax — expanded at parse time via `os.Getenv`. See D13 in `02-decisions.md`.

| Variable         | Description                          | Required                          |
| ---------------- | ------------------------------------ | --------------------------------- |
| `${VAR_NAME}`    | Any variable referenced in config    | Required if referenced in config  |

Example — REST connector auth token:

```yaml
datasets:
  - connector: rest
    auth_token: ${REST_AUTH_TOKEN}
```

In production, variables are injected by the orchestrator or a secrets manager (AWS Secrets Manager, HashiCorp Vault, Kubernetes secrets). Locally, they are set in the shell before running.

---

## Build and Run Instructions

These appear in the README. Summary:

```sh
make build          # compiles the binary
make test           # runs the test suite
make run            # runs against data/A_f.csv and data/B_f.csv with config/default.yaml
govulncheck ./...   # scans dependencies for vulnerabilities
```

---

## Open Questions

- Should the binary be distributed as a pre-built release artifact (e.g. GitHub Release), or is building from source sufficient for the challenge submission?
