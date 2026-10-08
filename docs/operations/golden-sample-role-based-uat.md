# SampleData v5 — UAT theo vai trò

Runbook dùng công trình mẫu tạo qua API có xác thực và công trình thử thao tác thủ công trên môi trường disposable. Fixed Golden project IDs, chuỗi SQL Golden và validator cũ đã bỏ. Hướng dẫn tại [công trình mẫu runbook](../../backend/seed/GOLDEN_LIVE_RUNBOOK.md) và [seed policy](../ops/seed-policy.md).

## Target và chuẩn bị

Mỗi lần chạy cần DB/storage riêng, tên rõ ràng và operator chịu trách nhiệm. Xác minh target trước migrate, seed hoặc action API. Không dùng production, staging hoặc DB/storage local dùng chung cho UAT ghi.

```bash
psql "$UAT_DATABASE_URL" -v ON_ERROR_STOP=1 -Atqc \
  "select current_database(), current_user, inet_server_addr(), inet_server_port()"
git rev-parse HEAD
cd backend
make migrate-check
make seed-fixtures-check
```

Đối chiếu DB name với target đã duyệt, rồi bootstrap bằng URL tường minh:

```bash
DATABASE_URL="$UAT_DATABASE_URL" MIGRATIONS_DIR=migrations go run ./cmd/migrate
psql "$UAT_DATABASE_URL" -v ON_ERROR_STOP=1 \
  -f seed/seed.sql -f seed/validate_current_seed.sql
curl -fsS "$UAT_BASE_URL/api/health"
curl -fsS "$UAT_BASE_URL/api/ready"
```

Backend thử phải trỏ đúng DB/storage đó. Không dùng `make seed` khi cần target tường minh vì Makefile có nhánh Compose. Seed additive, không reset dữ liệu, không chép PDF vào storage và không tạo công trình.

## Bộ công trình thử

Đăng nhập actor có `sample_data.create`, đọc `GET /api/settings/sample-data/options`, kiểm tra `blueprint_version=5`, pipeline published, role coverage và blockers. Provision tài khoản thiếu qua `POST /api/settings/sample-data/users`; tạo qua `POST /api/settings/sample-data/projects` hoặc **Cấu hình → Dữ liệu mẫu**.

| Case | Hồ sơ | Mục đích |
|---|---|---|
| S0 | `draft`, 0% | Thiết lập mới, chưa có runtime hoàn tất |
| S1 | `planning`, preset từ options | Đọc tiến độ/runtime tại mức được hỗ trợ |
| S2 | `completed`, 100% | Đọc nguồn hồ sơ, nghiệm thu và bàn giao tổng hợp đã sinh |
| S3 | Retry cùng actor/payload/key | Cùng công trình, không nhân đôi hồ sơ/artifacts |
| S4 | Key cũ với actor/payload khác | Conflict, không tạo side effect |
| S5 | Thiếu quyền, template/role không tương thích | Từ chối rõ ràng, không tạo công trình một phần |

Mẫu mới dùng UUID/idempotency key riêng; ghi IDs trả về. S2 là synthetic, không chứng minh thanh toán hay xác nhận MISA. Không mutate S2 trong case chỉ đọc; dùng công trình riêng cho actions.

## Nghiệp vụ và vai trò

Quyền/membership hiện tại quyết định expected result; tên/chức danh không cấp quyền. Ghi actor, role key, permission và project scope trước positive/negative cases. Không dùng Admin thay coverage chuyên biệt.

| Phạm vi | Positive case và invariant |
|---|---|
| Khảo sát/thiết kế | Form/minh chứng đúng nguồn, Giai đoạn mở đúng điều kiện, nhánh và duyệt đúng actor |
| BOQ/mua sắm | Duyệt/khóa giá trước sinh procurement; retry không sinh trùng |
| Hợp đồng | Lifecycle phiên bản, ký và source immutable; không mở nghĩa vụ AP/AR nội bộ |
| Kho/thi công | Accepted quantity tách stock movement; nhật ký/minh chứng giữ nguồn |
| QA/QC/nghiệm thu | Fail/rework/tái kiểm giữ lịch sử và quyền quyết định |
| Bàn giao | Vật tư/quality/checklist đủ điều kiện; tiếp nhận/trả bổ sung có actor/lý do/audit |
| Căn cứ đối soát | `accounting_reconciliation.view/manage` theo actual grants; nguồn/minh chứng/amount bất biến, hiển thị chênh lệch |
| MISA | `accounting_misa.reconcile`; provenance import, đúng project/partner/direction/currency, cap giao dịch và revoke có lý do |

Với kế toán, kiểm tra nguồn toàn công ty theo observer permissions hiện tại và write ngoài quyền bị từ chối. Khả năng đọc không chứng minh được sửa nguồn. API quản trị/audit và write nghiệp vụ khác kiểm tra theo catalog quyền thực tế.

Endpoint đã bỏ của sổ kế toán/AP/AR/payment/invoice/tax/report tài chính/đề nghị thanh toán phải không còn đăng ký; không đưa vào positive journey. Bàn giao hiện tại không đòi nợ nội bộ về 0 và không dùng collection gate. Kiểm tra blockers vận hành còn hiệu lực; không sửa SQL để vượt gate.

Chuyển role tuần tự và tuân thủ `Retry-After`; `429` không chứng minh negative RBAC. Kiểm tra cùng-key tạo mẫu không trùng, cạnh tranh phân bổ không vượt MISA amount, lỗi giữa chừng rollback và không để artifacts không được tham chiếu.

## Evidence và nghiệm thu

Lưu theo `run-id`: source SHA/working-tree state, timestamp, operator/reviewer, DB name/storage target, migrations, generator version, template revision/hash, actor/role/scope, request/response đã che secrets, IDs, before/after, audit/activity và defect links. Không lưu password, token, cookie, DSN chứa credentials hoặc tài liệu nhạy cảm.

Case chỉ PASS khi expected/actual có bằng chứng và side effects đúng. Failed/blocked/skipped giữ trạng thái; không cộng thành PASS. Baseline chỉ đọc giữ nguyên; writes ngoài quyền, mất lịch sử/nguồn, nhân đôi hồ sơ, vượt MISA cap hoặc reset nhầm target là FAIL. Tạo môi trường disposable mới khi cần lần chạy trắng; seed không reset. Lưu evidence lỗi trước xử lý target.

API/DB checks là kiểm chứng kỹ thuật của môi trường thử, không tự chứng minh UX. Chỉ chạy Browser/Playwright/headless browser khi người dùng yêu cầu rõ trong turn thực thi. Khi chưa chạy, ghi `browser verification skipped` và `NOT UX ACCEPTED`. Runbook không khẳng định UAT đã hoàn tất.
