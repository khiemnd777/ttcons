# Current States

## CI release unblock — `v2026-09-21.1`

- Will be done: no further release action; keep this record open until the user asks to close the plan.
- Done: GitHub release `v2026-09-21.1` was blocked before image build/VPS deploy by three backend integration failures. Diagnosis confirmed one persisted JSON decode defect and two stale assertions/fixtures. The decoder now treats persisted JSONB `null` as an empty allocation list; the stale V144 balance assertion and `workflow_form` fixture are aligned. All three regression tests passed against isolated PostgreSQL/Redis. Release `v2026-09-21.2` then passed backend CI but stopped at a stale frontend assertion that expected the retired standalone `Giai đoạn` column; the current UI intentionally owns a combined `Nguồn / Giai đoạn` column to represent design evidence and runtime stages. Release `v2026-09-21.3` (commit `e0ce728c2d5cfcf377a0c61f28ac5366978372c0`) passed the full GitHub CI gate, built and pushed immutable images, deployed to VPS, and passed public HTTPS, gateway health and `/api/ready` checks. No production migration ran.
