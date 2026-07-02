---
name: consflow-handoff
description: Create a compact Consflow handoff for another Codex session, including current objective, touched files, commands run, validation status, blockers, and suggested repo skills or subagents.
---

# Consflow Handoff

Use this skill before pausing long work, handing to another agent, or documenting a partially completed implementation.

## Handoff Content

Include:

- Objective and latest user request.
- Current repo state and files changed.
- Important decisions and tradeoffs.
- Diagnosis status: reproduction signal, evidence, root cause or unresolved risk, and remaining clarification questions.
- Commands run and their outcomes.
- Validation still needed.
- Known blockers or risks.
- Suggested skills for the next session.
- Suggested subagents if parallel review is useful.

## Rules

- Do not duplicate large diffs or code blocks.
- Reference files by path.
- Redact secrets, tokens, passwords, and private personal data.
- Keep the handoff compact enough for a fresh session to act immediately.
- If saving to disk, prefer a temp file unless the user asks for a repo document.
- A handoff does not authorize implementation. Any next-session feature, bugfix, code-changing, or UI-changing work must still pass the repo review package, Mockup as Text when relevant, root-cause/clarification gate, and explicit user confirmation before editing.

## Suggested Skills Section

Use local names:

- `consflow-repo-architect`
- `consflow-backend-workflow`
- `consflow-frontend-workflow`
- `consflow-workflow-orchestration-domain`
- `consflow-project-planning-domain`
- `consflow-construction-operations-domain`
- `consflow-commercial-procurement-domain`
- `consflow-materials-warehouse-domain`
- `consflow-quality-acceptance-handover-domain`
- `consflow-accounting-finance-domain`
- `consflow-admin-governance-domain`
- `consflow-reporting-analytics-domain`
- `consflow-pipeline-domain` for broad legacy cross-domain guardrails
- `consflow-diagnose`
- `consflow-docker-ops`

## Output Expectations

Return the handoff body or the path where it was saved.
