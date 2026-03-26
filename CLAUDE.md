## What This Is

A CLI tool that compares two CSV datasets of anonymised UDPRN keys (Unique Delivery Point Reference Numbers) and computes four set intersection statistics: total key count per file, distinct key count per file, distinct overlap (keys appearing in both files), and total overlap (sum of min(count_in_A, count_in_B) across shared keys). Built for InfoSum's privacy-preserving data platform, the tool outputs only aggregate counts — never individual key values. A key design consideration is handling files that may be very large, potentially requiring streaming or probabilistic approximation.

## Key Files

- `docs/01-requirements.md` — functional and non-functional requirements, design clarifications, open questions
- `docs/02-decisions.md` — reasoning behind design choices and alternatives considered
- `docs/03-data-consumers.md` — who needs what view of which data and why
- `docs/04-entities.md` — entity definitions and field reasoning (intermediate step before schema)
- `docs/05-architecture.md` — infrastructure, scalability, and privacy boundary decisions
- `docs/06-testing.md` — what to test, testing strategy, and key scenarios
- `docs/07-observability.md` — logging, exit codes, and diagnostic output strategy
- `docs/08-security.md` — privacy constraints, data classification, and what must never appear in output
- `docs/09-deployment.md` — build and distribution strategy, Dockerfile considerations
- `docs/10-sequence.md` — walking skeleton, development slices, and sequencing reasoning
- `docs/13-tooling.md` — recommended packages and tools by concern with benefits and tradeoffs

## Data

Input CSV files are in the `data/` directory:

- `data/A_f.csv` — dataset A (single-column CSV of UDPRN keys with header)
- `data/B_f.csv` — dataset B (single-column CSV of UDPRN keys with header)

## Core Algorithm Note

Total overlap is defined as: for each key appearing in both files, contribute `min(count_in_A, count_in_B)` to the total. This is not a cartesian product and not a simple sum. See `docs/02-decisions.md` (D1) for the full reasoning.
