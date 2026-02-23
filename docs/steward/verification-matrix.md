# Steward Verification Matrix

This matrix defines the minimum automated verification coverage for steward features.

## Automated Suites

### Unit / package tests

- `go test ./internal/steward`
- `go test ./internal/api`

Coverage focus:

- queue and runtime state transitions
- policy tuning guardrails and bounds
- redaction behavior and markers
- circuit-breaker behavior

### Repo-wide regression

- `go test ./...`

Purpose:

- prevent steward changes from regressing unrelated memory/API codepaths

### E2E (manual/stack-required)

- `go test -tags e2e ./tests/e2e -run TestRecallBenchmark -v -count=1`
- `go test -tags e2e ./tests/e2e -v -count=1`

Notes:

- requires a running local stack (Postgres + Ollama + Contextify server)
- steward UI/control scenarios are validated against the live `/steward` console

### Benchmark Artifacts

- Recall benchmark JSON artifact: `artifacts/recall-benchmark-report.json`
- Steward verification matrix JSON artifact: `artifacts/steward-verification-matrix.json`

## Machine-Readable Verification Artifact

Run locally:

```bash
make verify-steward
```

This generates:

- `artifacts/steward-verification-matrix.json`

The artifact contains:

- generation timestamp
- step-level pass/fail status
- per-step timing metadata
- notes on remaining manual/e2e checks

## CI Integration

`Backend CI` uploads the steward verification matrix artifact on every run (best-effort, `if: always()`).
