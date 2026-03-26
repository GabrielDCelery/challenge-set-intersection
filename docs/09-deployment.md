# Deployment

How this program gets built and distributed. This is a CLI tool — "deployment" means producing a runnable binary or script and documenting how to build and run it.

---

## Deployment Target

**Hosting:** Local machine / developer workstation. No server, no container registry, no cloud infrastructure.

**Distribution:** A compiled binary (if using Go or TypeScript compiled to a binary), or a script with a documented dependency setup step (if using Python or Node.js).

**Why this target fits:** The spec asks for "source code and instructions on how to build and run" — this is a challenge submission, not a production service.

---

## Environments

| Environment | Purpose                        | Notable differences               |
| ----------- | ------------------------------ | --------------------------------- |
| local       | Development and submission run | Uses the provided data/ CSV files |

No staging or production environments exist for this tool.

---

## CI/CD

**Decision:** TBD

For a challenge submission, a basic GitHub Actions workflow to run tests on push is sufficient. It validates that the build and test steps work on a clean machine.

**Suggested pipeline steps:** install dependencies → build → test → (optionally) lint

---

## Containerisation

**Decision:** TBD

A Dockerfile is useful here because it guarantees the reviewer can run the program without installing the language runtime on their machine.

If provided:

- Use a multi-stage build: build stage compiles the binary, runtime stage is a minimal image (e.g. `scratch` or `alpine`).
- The container should accept file paths as arguments — the data/ directory should be mounted as a volume, not baked into the image.

Example invocation with Docker:

```
docker build -t set-intersection .
docker run --rm -v $(pwd)/data:/data set-intersection /data/A_f.csv /data/B_f.csv
```

---

## Infrastructure Provisioning

Not applicable — no cloud resources to provision.

---

## Data Migrations

Not applicable — no persistent schema.

---

## Network Access

Not applicable — no network communication.

---

## Secrets and Environment Variables

| Env Var       | Description | Required | Default |
| ------------- | ----------- | -------- | ------- |
| None required | —           | —        | —       |

---

## Build and Run Instructions

These should appear in the README. At minimum:

1. How to install dependencies (if any)
2. How to build the binary (if compiled)
3. How to run against the provided data files
4. How to run the tests

**Open:** What language and toolchain is being used? The build instructions depend on this. See `13-tooling.md`.

---

## Open Questions

- Should the program be distributed as a single compiled binary (no runtime dependency) or as a script that requires the language runtime installed?
- Is a Makefile or Taskfile warranted to simplify build and test commands for the reviewer?
