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

## Scaling

### Vertical Scaling

The tool scales vertically by changing the algorithm and caching strategy in config — no code changes, no infrastructure changes, no redeployment. The three strategies form a deliberate progression:

`in_memory` is the default. Both frequency maps are held in RAM simultaneously. For typical datasets — millions of keys, key lengths in the tens of bytes — this fits comfortably in a single container's memory allocation. See D10 in `02-decisions.md` for the sizing formula and approximate thresholds.

`spill_to_disk` handles datasets that exceed available RAM. Instead of holding the full frequency map in memory, the algorithm streams chunks to a temporary directory on disk and merges them at comparison time. The trade-off is I/O throughput for memory headroom. The result is still exact. This is the right choice when the data is large but the runtime environment has fast local storage.

`pairwise_approximate` is the escape hatch for datasets that are both too large for RAM and too large for spill-to-disk to be practical. HyperLogLog and MinHash replace exact counting with probabilistic estimation — the output includes an `ErrorBoundPct` field to signal to the caller that the figures are estimates. This is appropriate when a ±1–2% error margin is acceptable, which is usually true for the aggregate counts this tool produces.

The sizing formula in D10 gives a concrete basis for choosing the strategy at deployment time. The config change is a one-line edit to `algorithm.caching_strategy` — the rest of the pipeline is unaffected.

### Horizontal Scaling

Horizontal scaling is not implemented in this version. The current design processes one pair of datasets in a single container.

The natural extension is a partition-and-merge approach. Each key is hashed to a partition, and each partition is assigned to a worker. The invariant that makes this correct is that consistent hashing guarantees equal keys always land in the same partition — so each worker sees all occurrences of the keys it owns, from both datasets, and can build a complete frequency map for its slice of the keyspace independently. Overlap is then computed per partition and summed. No worker needs to communicate with any other during computation.

This is compatible with the current design. `KeyIterator` already streams in batches — a partitioned worker would consume the same interface, routing each incoming batch to the appropriate partition worker rather than processing it directly. `IntersectionResult` carries per-dataset counts and an overlap figure that are additive across partitions, so the fan-in step is a straightforward sum. The algorithm interfaces do not need to change; what changes is the orchestration layer — a fan-out stage that distributes rows, a parallel computation stage, and a fan-in stage that aggregates partial results.

In practice this maps onto any job graph orchestrator: AWS Batch array jobs, Kubernetes Jobs with a coordinator, or a Spark/Flink pipeline stage. The current single-container design is the degenerate case — one partition, one worker.

This would be the right direction if the tool were to be embedded in a distributed pipeline (e.g. Spark, Flink, or a custom orchestrator job graph). For the current use case — two CSVs processed in a single batch job — vertical scaling via `spill_to_disk` or `pairwise_approximate` is simpler and sufficient.

---

## Image Distribution

The Docker image is the deployment artefact. In production it is pushed to a container registry (AWS ECR, GCR, or Docker Hub) and tagged by commit SHA or semantic version. The orchestrator pulls the image by tag at job invocation time.

```sh
docker build -t set-intersection:${GIT_SHA} .
docker tag set-intersection:${GIT_SHA} <registry>/set-intersection:${GIT_SHA}
docker push <registry>/set-intersection:${GIT_SHA}
```

The image is stateless and immutable — it contains only the compiled binary. All runtime state (config, data, secrets) is injected at invocation. This means any version can be re-run against any config without rebuilding, and rolling back to a previous version is a tag change in the orchestrator.

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
