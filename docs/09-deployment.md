# Deployment

How this program gets built and distributed. In its current form this is a CLI tool submitted as a challenge — but the design accounts for production deployment as a pipeline job.

---

## Deployment Target

**Current:** A Docker image built from source. The submission includes a Dockerfile, source code, and a Makefile with `build`, `test`, `docker-build`, and `docker-run` targets. The reviewer does not need Go installed — Docker is sufficient to build and run.

**Production:** The same Docker image run as a job within a data pipeline or orchestrator. What changes is how the image is invoked, how the config is supplied, and how secrets are injected. No code changes are required to move from local to production.

---

## Environments

| Environment | Purpose                                                    |
| ----------- | ---------------------------------------------------------- |
| local       | Development, testing, and challenge submission             |
| production  | Pipeline job — config and secrets injected by orchestrator |

---

## CI/CD

A GitHub Actions workflow runs on every push:

1. `go test ./...` — runs the full test suite
2. `govulncheck` — scans dependencies for known vulnerabilities
3. `docker build` — builds the multi-stage image
4. `trivy image` — scans the built image for OS-level vulnerabilities

The Makefile targets (`docker-build`, `docker-run`, `test`) mirror the CI steps so local and CI behaviour are identical.

---

## Containerisation

A Dockerfile is included. Multi-stage build — a build stage compiles the Go binary, a minimal runtime stage (`scratch` or `alpine`) packages it. No Go toolchain in the final image.

```sh
docker build -t set-intersection .
docker run --rm \
  -v $(pwd)/data:/data \
  -v $(pwd)/config.yaml:/config.yaml \
  -e REST_AUTH_TOKEN=... \
  set-intersection --config /config.yaml
```

The config file and data directory are mounted at runtime — nothing sensitive is baked into the image. Secrets are injected via environment variables (see D13).

`trivy` is added to CI to scan the image for OS-level vulnerabilities. See `13-tooling.md`.

---

## Network Access

REST connectors make outbound HTTPS calls to configured API endpoints. No inbound network access. HTTPS is required — plaintext HTTP must not be used. See `08-security.md`.

---

## Secrets and Environment Variables

Secrets are injected via environment variables and referenced in the YAML config using `${VAR}` syntax — expanded at parse time via `os.Getenv`. See D13 in `02-decisions.md`.

| Variable      | Description                       | Required                         |
| ------------- | --------------------------------- | -------------------------------- |
| `${VAR_NAME}` | Any variable referenced in config | Required if referenced in config |

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
make docker-build   # builds the Docker image
make docker-run     # runs the image against data/A_f.csv and data/B_f.csv with config/default.yaml
make test           # runs the test suite (requires Go installed)
make build          # compiles the binary locally (requires Go installed)
govulncheck ./...   # scans dependencies for vulnerabilities (requires Go installed)
```

Docker is the primary path — the reviewer does not need Go installed. The `build` and `test` targets are provided for contributors who have Go available locally.

---

## Open Questions

None.
