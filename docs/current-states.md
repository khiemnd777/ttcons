# Current States

## CI release unblock — `v2026-09-21.1`

- Will be done: push the CI-test corrective commit, create the next release tag to trigger the full GitHub CI gate, then verify VPS deployment and HTTPS smoke checks only if CI succeeds.
- Done: GitHub release `v2026-09-21.1` was blocked before image build/VPS deploy by three backend integration failures. Diagnosis confirmed one persisted JSON decode defect and two stale assertions/fixtures. The decoder now treats persisted JSONB `null` as an empty allocation list; the stale V144 balance assertion and `workflow_form` fixture are aligned. All three regression tests passed against isolated PostgreSQL/Redis. Release `v2026-09-21.2` then passed backend CI but stopped at a stale frontend assertion that expected the retired standalone `Giai đoạn` column; the current UI intentionally owns a combined `Nguồn / Giai đoạn` column to represent design evidence and runtime stages. No production migration or deployment ran.
