# set-intersection

A CLI tool that compares two CSV datasets of anonymised UDPRN keys and computes four set intersection statistics: total key count per file, distinct key count per file, distinct overlap (keys appearing in both files), and total overlap (sum of `count_in_A × count_in_B` across shared keys). Built for privacy-preserving data analysis — the tool outputs only aggregate counts, never individual key values.

## Quick Start

```sh
git clone <repo>
cd set-intersection
```

Place your input CSV files in the `data/` directory:

- `data/A_f.csv` — dataset A (single-column CSV with a `udprn` header)
- `data/B_f.csv` — dataset B (single-column CSV with a `udprn` header)

Then build and run:

```sh
make docker-build
make docker-run
```

No Go installation required — Docker is sufficient to build and run.

Results are written to stdout, diagnostic logs (skipped rows, timing) to stderr. To separate them:

```sh
# results only
make docker-run 2>/dev/null

# logs only
make docker-run 1>/dev/null

# run logs to file
make docker-run 2> debug.log
```

## Development

```sh
make test    # run the test suite (requires Go)
make build   # compile the binary locally (requires Go)
```

## Docs

- [`docs/00-domain.md`](docs/00-domain.md) — domain context, UDPRN keys, and what the four metrics represent
- [`docs/01-requirements.md`](docs/01-requirements.md) — functional and non-functional requirements, design clarifications, open questions
- [`docs/02-decisions.md`](docs/02-decisions.md) — reasoning behind design choices and alternatives considered
- [`docs/03-data-consumers.md`](docs/03-data-consumers.md) — who needs what view of which data and why
- [`docs/04-entities.md`](docs/04-entities.md) — first-class interface definitions (KeyIterator, IntersectionAlgorithm, ResultWriter) and supporting types
- [`docs/05-architecture.md`](docs/05-architecture.md) — infrastructure, scalability, and privacy boundary decisions
- [`docs/06-testing.md`](docs/06-testing.md) — what to test, testing strategy, and key scenarios
- [`docs/07-observability.md`](docs/07-observability.md) — logging, metrics, and tracing strategy
- [`docs/08-security.md`](docs/08-security.md) — privacy constraints, data classification, and what must never appear in output
- [`docs/09-deployment.md`](docs/09-deployment.md) — build and distribution strategy, Dockerfile considerations
- [`docs/10-sequence.md`](docs/10-sequence.md) — development slices, sequencing reasoning, and what to defer
- [`docs/13-tooling.md`](docs/13-tooling.md) — recommended packages and tools by concern with benefits and tradeoffs
