# Consflow - Construction Workflow Operating System

Consflow là web app quản lý vòng đời công trình theo dynamic workflow / pipeline cho xây dựng, hoàn thiện nội thất, sửa chữa, cải tạo và fit-out.

Trọng tâm của hệ thống là quản lý điều kiện để công việc được phép xảy ra: đủ người, đủ quyền, đủ vật tư, đủ ngân sách, đủ phê duyệt, đủ nghiệm thu, đủ chứng từ và đủ điều kiện tài chính.

## Tech Stack

- Backend: Go, Fiber, PostgreSQL, Redis, JWT, RBAC, custom SQL migrations, Ent schema scaffold
- Frontend: React, TypeScript, Vite, TailwindCSS, TanStack Query, React Router, React Hook Form, Zod
- Local infra: Docker Compose với PostgreSQL, Redis, backend và admin frontend

## Cấu trúc

```txt
backend/
  cmd/server/main.go
  internal/core              # handler -> service -> repository
  internal/shared            # config, db, cache, middleware, response
  ent/schema                 # Ent schema scaffold cho core entities
  migrations/V1__init.sql
  seed/seed.sql
frontend/
  src/app/router.tsx
  src/layouts/AppShell.tsx
  src/components
  src/modules
deploy/k8s                     # portable multi-node application baseline
docker-compose.yml
AGENTS.md
.agents/skills                 # Repo-scoped Codex skills
.codex/agents                  # Project-scoped Codex subagents
```

## Chạy bằng Docker

```bash
make up
```

Lệnh vận hành:

```bash
make up       # down -> build -> run
make down
make restart  # down -> build -> run
make log      # xem log toàn bộ stack
make log SERVICE=admin
```

Sau khi stack chạy:

- Admin frontend: http://localhost:5173
- Backend API: http://localhost:8080/api
- PostgreSQL: localhost:5432, database `consflow`, user/password `consflow`

Production Compose là topology single-host có kiểm soát. Baseline Kubernetes cho hai replica `admin`/`backend`, external managed PostgreSQL/Redis/S3, migration Job và các rollout gate nằm tại [`deploy/k8s/README.md`](deploy/k8s/README.md); manifests cố ý fail-closed và không tự deploy cluster.

Seed data thủ công nếu cần reset dữ liệu:

```bash
cd backend
make seed
```

## Chạy local từng phần

```bash
docker compose up -d postgres redis

cd backend
make migrate
make seed
make dev
```

Admin frontend chạy bằng Docker trong stack chính qua `make up`.

## Codex local repo agents

Repo này có bộ hướng dẫn và agent artifacts local:

```bash
bash .agents/scripts/validate.sh
```

- `AGENTS.md`: quy ước repo cho Codex
- `.agents/skills`: skills theo chuẩn repo-scope
- `.codex/agents`: custom subagents project-scope
- `docs/agents/README.md`: ghi chú vận hành agent

Rule bắt buộc cho agent:

- Feature, bugfix hoặc thay đổi code phải trình review package và chờ confirm trước khi triển khai.
- Thay đổi UI/giao diện phải có "Mockup as Text" và chờ confirm trước khi triển khai.

## Tài khoản demo

Tất cả dùng mật khẩu `123456`.

- admin@consflow.local
- pm@consflow.local
- qs@consflow.local
- accounting@consflow.local
- warehouse@consflow.local
- supervisor@consflow.local
- director@consflow.local
- procurement@consflow.local
- qaqc@consflow.local
- construction@consflow.local
- subcontractor@consflow.local
- design@consflow.local
- sales@consflow.local
- legal@consflow.local
- contract@consflow.local

Fixture kiểm tra đăng nhập bị từ chối: `client@consflow.local`. Tài khoản này ở trạng thái `inactive`, role `client` là đối tác ngoài App và không có quyền ứng dụng.

## Module chính

- Login, Dashboard
- Công trình và thành viên công trình
- Pipeline Builder: template, node, step, approval rule, fail path, publish, clone
- Pipeline Run / Planning: clone template vào công trình, submit step, request approval, advance/block node
- BOQ / Dự toán và sinh procurement từ BOQ
- Procurement, Warehouse, Operations, Acceptance
- Rework / Replanning, Handover
- Accounting, Payables, Receivables, Payments
- Staff, Roles, Permissions
- Audit Log / Workflow Playback
- Reports và Settings

## API Summary

Backend response chuẩn:

```json
{
  "success": true,
  "data": {},
  "message": "OK"
}
```

Các nhóm route chính:

- `POST /api/auth/login`, `GET /api/auth/me`
- `GET/POST /api/projects`, `GET/PUT/DELETE /api/projects/:id`
- `GET/POST /api/pipeline-templates`, `POST /api/pipeline-templates/:id/publish`
- `POST /api/projects/:id/start-pipeline`, `GET /api/projects/:id/pipeline-run`
- `POST /api/pipeline-run-steps/:id/submit`, `POST /api/pipeline-run-nodes/:id/advance`
- `GET /api/approvals`, `POST /api/approvals/:id/approve`
- `GET/POST /api/projects/:id/boq`, `POST /api/projects/:id/boq/generate-procurement`
- `GET/POST /api/procurement-requests`
- `GET /api/materials`, `GET /api/stock`, `POST /api/stock/issue`
- `GET/POST /api/projects/:id/daily-reports`, `GET/POST /api/projects/:id/issues`
- `GET/POST /api/projects/:id/acceptances`, `POST /api/acceptances/:id/fail`
- `GET/POST /api/projects/:id/reworks`, `GET/POST /api/projects/:id/replannings`
- `GET/POST /api/projects/:id/handover`, `POST /api/handover/:id/check`
- `GET /api/payables`, `GET /api/receivables`, `GET /api/payments`
- `GET /api/staff`, `GET /api/roles`, `GET /api/permissions`
- `GET /api/audit-logs`, `GET /api/reports/dashboard`

## Development Notes

- Custom migration runner tạo đầy đủ nhóm bảng domain trong prompt và ghi lịch sử vào `schema_migrations`.
- Seed data dùng UUID cố định để demo flow ổn định.
- Ent schema hiện scaffold các entity lõi; repository SQL đang chạy runtime để bao phủ nhanh toàn bộ domain rộng.
- Business rules demo đã có ở backend: block node thi công khi chưa đủ vật tư hoặc chưa thu đủ 30%, nghiệm thu fail tạo punch list/rework, handover closure checker.
