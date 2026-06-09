---
name: consflow-diagnose
description: Diagnose Consflow bugs and performance regressions with a reproducible feedback loop across Docker, Go API, PostgreSQL migrations, React admin UI, and workflow state transitions.
---

# Consflow Diagnose

This skill adapts the diagnosis loop used in the reference skill catalog to Consflow.

## Phase 1: Build A Feedback Loop

Create the smallest repeatable pass/fail signal before fixing.

Good loops for this repo:

- `go test ./...` or a focused Go test under `backend`.
- `cd frontend && npm run build` for UI/type/build failures.
- `curl` against `http://localhost:8080/api/...` after `make up`.
- Browser check against `http://localhost:5173` for admin workflows.
- SQL check with `psql` for migration or data integrity issues.
- Docker log check with `make log SERVICE=backend` or `make log SERVICE=admin`.

Do not proceed with a speculative fix if the bug can be reproduced locally but no loop has been built.

## Phase 2: Reproduce

Confirm the loop matches the reported symptom:

- same route or UI workflow
- same actor/role when auth matters
- same project or workflow state when domain state matters
- same error message, wrong state, or missing UI output

## Phase 3: Rank Hypotheses

Write 3 to 5 falsifiable hypotheses before editing. Each one should predict what evidence would prove or disprove it.

Prefer boundaries that distinguish likely causes:

- frontend API call vs backend route
- handler parsing vs service transition
- service logic vs SQL repository helper
- migration/schema mismatch vs seed data
- Docker env/config vs local env

## Phase 4: Instrument Narrowly

Use targeted logs or temporary probes only at boundaries tied to a hypothesis. Tag temporary logs with a unique marker and remove them before finishing.

## Phase 5: Fix And Lock It Down

- Before implementing a bugfix or code change, present the diagnosis summary, proposed fix, files likely to change, validation plan, and wait for explicit user confirmation.
- If the fix changes UI, include a "Mockup as Text" and wait for explicit user confirmation before editing UI code.
- Add a regression test when a stable seam exists.
- Use a new migration for schema fixes.
- Re-run the original feedback loop.
- Re-run the narrow build/test command for the touched layer.

## Output Expectations

- State the reproduction loop.
- State the winning hypothesis.
- List files changed.
- Report validation results.
- Note any missing regression seam.
