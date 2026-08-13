# [Closed] Workflow Signature, Seal And PDF Composition

## Scope closed

- Touch Signature Canvas Stability
- Reusable Workflow Seal Confirmations
- Workflow PDF Confirmation Composition
- Workflow PDF Preference Cache Stability

## Delivered

- Stabilized touch, pen and mouse signature drawing by resolving pointer coordinates before deferred React state updates; preserved immutable signature semantics and existing API contracts.
- Added a protected organization-wide reusable company-seal catalog backed by the existing Local/S3 storage abstraction, immutable seal applications, audit history, permissions, Settings management and fixed PDF rendering.
- Renamed the ambiguous workflow action to `Thêm xác nhận`, separated `Chữ ký cá nhân` from `Con dấu công ty`, and retained `Tạo PDF` as an independent one-click action.
- Grouped confirmations by purpose and added auto-saved `Hiển thị trong PDF` choices when a purpose contains both a signature and a seal. At least one type remains selected; single-type purposes stay automatically included.
- Added V94 for reusable seal assets/applications and V95 for mutable PDF confirmation preferences. Existing immutable signature, seal and PDF artifacts remain unchanged.
- Stored exact selected confirmation IDs and counts in immutable PDF export metadata and filtered PDF hashing/rendering to match the saved selection.
- Fixed the checkbox regression that replaced the canonical work-item-form query cache with a narrower mutation response and falsely rendered `Bước này chưa có hồ sơ nghiệp vụ`.
- Updated central, backend, frontend, root-cause, workflow and admin-governance mappings for the changed feature ownership and behavior.

## Verification at closure

- Frontend focused regressions and full suite passed; latest full run: 81/81 tests.
- Frontend TypeScript/Vite production build passed for the current implementation source.
- Backend `go test ./...` and the relevant isolated database-backed migration/integration checks passed during implementation.
- `git diff --check` passed before closure.
- Local Docker health/readiness and migrations through V95 were verified during implementation without resetting or reseeding persistent data.
- Browser/Edge UAT was not requested after the latest cache fix; UX status remains `NOT UX ACCEPTED`.

## Durable references

- Feature routing and ownership are maintained in `.agents/skills/consflow-module-memory/references/`.
- Workflow invariants are maintained in `.agents/skills/consflow-workflow-orchestration-domain/references/domain-map.md`.
- Seal administration permissions and governance are maintained in `.agents/skills/consflow-admin-governance-domain/references/domain-map.md`.
