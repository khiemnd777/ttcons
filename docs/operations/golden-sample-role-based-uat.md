# Golden Sample role-based UAT

Runbook này kiểm tra Golden Sample theo vai trò mà không làm bẩn fixture hoàn tất hoặc dữ liệu dùng chung. Mặc định chỉ dùng API, SQL chỉ đọc và các lệnh kiểm tra không có trình duyệt. Luồng nghiệp vụ đầy đủ của sandbox được mô tả tại [Golden Live runbook](../../backend/seed/GOLDEN_LIVE_RUNBOOK.md).

## Phạm vi và fixture

| Fixture | Mục đích | Chế độ | Reset |
|---|---|---|---|
| `GOLDEN-REAL-001` (`30000000-0000-0000-0000-000000000020`) | Baseline hoàn tất nhánh `Có thiết kế` | Chỉ đọc; không action API, SQL chỉ `SELECT` | Không |
| `GOLDEN-NO-DESIGN-001` (`30000000-0000-0000-0000-000000000021`) | Baseline hoàn tất nhánh `Không thiết kế` | Chỉ đọc; không action API, SQL chỉ `SELECT` | Không |
| `GOLDEN-LIVE-001` (`30000000-0000-0000-0000-000000000022`) | Sandbox thực thi từ node `Khảo sát` | Được mutate trên DB UAT cô lập | Có, theo chu kỳ bên dưới |

Hai fixture hoàn tất là mẫu đối chiếu của cùng một suite, không phải hai runtime cần chạy lại. Mọi thay đổi trên chúng là `FAIL` và phải dừng UAT.

## Cổng an toàn DB UAT

Chỉ chạy UAT ghi dữ liệu trên PostgreSQL local/disposable có tên riêng, ví dụ `consflow_uat_<run-id>`, cùng một upload directory riêng. Không dùng production, staging, DB local dùng chung hoặc thư mục upload dùng chung.

Trước mọi migrate/reset/action, ghi kết quả các lệnh sau vào evidence package và đối chiếu tên DB bằng mắt:

```bash
export UAT_DATABASE_URL='postgres://consflow:consflow@127.0.0.1:5432/consflow_uat_<run-id>?sslmode=disable'
export UAT_EXPECTED_DB='consflow_uat_<run-id>'
export UAT_BASE_URL='http://127.0.0.1:8080'
psql "$UAT_DATABASE_URL" -v ON_ERROR_STOP=1 -Atqc \
  "select current_database(), current_user, inet_server_addr(), inet_server_port()"
psql "$UAT_DATABASE_URL" -v ON_ERROR_STOP=1 -Atqc \
  "select current_database() = '$UAT_EXPECTED_DB'"
```

Chỉ tiếp tục khi dòng thứ hai trả `t` và metadata đúng target đã phê duyệt. Dừng ngay nếu URL rỗng, tên DB không khớp, target có dấu hiệu production/staging/shared hoặc chưa có người chịu trách nhiệm reset.

> `backend/Makefile` ưu tiên PostgreSQL Compose đang chạy và khi đó `make seed` luôn ghi vào DB `consflow`. Vì vậy không dùng `make seed` cho UAT cô lập khi Compose đang chạy. Reset cô lập phải dùng URL tường minh như dưới đây. Tất cả seed bắt đầu bằng `TRUNCATE ... CASCADE` và xóa toàn bộ dữ liệu trong target.

## Chuẩn bị và lệnh không dùng trình duyệt

Các lệnh kiểm tra không ghi DB:

```bash
git rev-parse HEAD
cd backend
make migrate-check
make seed-fixtures-check
psql "$UAT_DATABASE_URL" -v ON_ERROR_STOP=1 -f seed/validate_golden_project.sql
curl -fsS "$UAT_BASE_URL/api/health"
curl -fsS "$UAT_BASE_URL/api/ready"
```

Nếu cần tạo/reset DB disposable đã qua cổng an toàn, migrate rồi chạy chuỗi seed bằng đúng một `UAT_DATABASE_URL`:

```bash
cd backend
DATABASE_URL="$UAT_DATABASE_URL" make migrate
psql "$UAT_DATABASE_URL" -v ON_ERROR_STOP=1 \
  -f seed/seed.sql \
  -f seed/golden_project.sql \
  -f seed/golden_project_domains.sql \
  -f seed/golden_project_closure.sql \
  -f seed/golden_project_construction.sql \
  -f seed/golden_project_finance.sql \
  -f seed/golden_live_project.sql \
  -f seed/validate_golden_project.sql
```

Kết quả cuối phải có `GOLDEN_SAMPLE_VALIDATION_PASSED`. Không xóa thư mục upload bằng lệnh recursive; cấu hình backend dùng upload directory disposable riêng và chép fixture từ `backend/seed/uploads` theo quy trình môi trường đã duyệt.

## Chu kỳ Golden Live

| Chu kỳ | Nhánh | Reset Golden Live? | Quy tắc |
|---|---|---|---|
| C0 | Baseline | Có, trước UAT ghi đầu tiên | Seed toàn DB UAT cô lập, chạy validator, chụp metadata |
| C1 | `Có thiết kế` (`yes`) | Không trong chu kỳ | Chạy J0-J9, lưu evidence/sign-off trước reset |
| C2 | `Không thiết kế` (`no`) | Có, giữa C1 và C2 | Reset toàn DB UAT cô lập; không chạy song song với C1 |
| C3 | Negative RBAC/read-only | Không nếu mọi write bị từ chối | Nếu write ngoài dự kiến thành công: đánh `FAIL`, chụp evidence rồi reset target cô lập |

Không reset để “xóa” lỗi trước khi đã lưu request/response, actor, entity ID, audit và snapshot trước/sau. Không reset `GOLDEN-REAL-001` hoặc `GOLDEN-NO-DESIGN-001` riêng lẻ.

## Hành trình J0-J9

| ID | Hành trình | Bằng chứng tối thiểu |
|---|---|---|
| J0 | Đăng nhập, project scope, menu/API được phép và write bị cấm | actor/role, HTTP status, project membership, một positive và một negative permission check |
| J1 | Khảo sát, form/evidence, biên bản họp và chọn nhánh | exact form/answer/evidence IDs, work-item transitions, approval actor khác requester |
| J2 | Nhánh thiết kế hoặc bỏ qua thiết kế | nhánh đã chọn, node được mở, nhánh đối nghịch được skip, không có mutation trên fixture hoàn tất |
| J3 | BOQ, báo giá, khóa giá và sinh procurement | BOQ/quote/selection/procurement IDs, status trước/sau, không sinh trùng |
| J4 | Procurement, incoming acceptance và kho | PO/delivery/acceptance/document/movement IDs; accepted quantity tách khỏi quantity đã nhập kho |
| J5 | Hợp đồng, version, milestone, phụ lục và retention | actor/reviewer/signer, immutable signed version, AP/AR chỉ mở ở gate hợp lệ |
| J6 | Kế hoạch, handoff thi công, nhật ký và kiểm tra hiện trường | form revision/hash, generated work/check IDs, daily report/evidence, idempotent handoff |
| J7 | QA/QC, RFI/NCR/rework, nghiệm thu đầu ra | fail/pass history, NCR/rework/reinspection IDs, criterion-evidence links, không mất lịch sử |
| J8 | AP/AR, lịch, tạm ứng, hóa đơn, phân bổ và đối soát | ledger IDs/amounts/status, requester separation, không tạo fake invoice/payment/stock movement |
| J9 | Vật tư dư, checklist bàn giao, readiness, đóng tài chính và audit | blocker trước/sau, tám checklist, handover/audit IDs, AP/AR/pending payment về điều kiện đóng |

## Ma trận 15 vai trò

Ký hiệu: `P` hành động positive-path; `D` quyết định/duyệt bằng actor khác requester; `R` chỉ đọc/đối chiếu; `N` negative write phải bị từ chối; `-` không áp dụng. Đây là trách nhiệm tối thiểu, không cho phép dùng Admin thay cho kiểm tra role-specific.

| Vai trò / tài khoản | J0 | J1 | J2 | J3 | J4 | J5 | J6 | J7 | J8 | J9 |
|---|---|---|---|---|---|---|---|---|---|---|
| Admin `admin@consflow.local` | P | R | R | R | R | R | R | R | R | R |
| PM `pm@consflow.local` | P/N | P/D | P/D | P/D | R | P/D | P/D | R/D | R | P/D |
| QS `qs@consflow.local` | P/N | P | P | P | P | P | P | P | P/R | R |
| Kế toán `accounting@consflow.local` | P/N | N | N | R | P | P/D | N | R | P/D | P/D |
| Thủ kho `warehouse@consflow.local` | P/N | N | N | R | P | N | P | P/R | N | P |
| Giám sát `supervisor@consflow.local` | P/N | P | R | N | R | N | P | P | N | P |
| Giám đốc `director@consflow.local` | P/N | D | D | D | D | D | D | D | D | D |
| Mua hàng `procurement@consflow.local` | P/N | N | N | P | P | P | R | R | R | R |
| QA/QC `qaqc@consflow.local` | P/N | R | R | N | P | N | P | P/D | N | P/D |
| Đội thi công `construction@consflow.local` | P/N | P | N | N | R | N | P | P | N | P/R |
| Thầu phụ `subcontractor@consflow.local` | P/N | N | N | N | R | P/R | P | P | N | R |
| Chủ đầu tư `client@consflow.local` | P/N | P/D | D | D | N | D | R | D | R | D |
| Thiết kế `design@consflow.local` | P/N | P | P | R | N | N | P | R | N | R |
| Sale `sales@consflow.local` | P/N | P | P/R | P | R | P/R | R | R | R | R |
| Pháp lý `legal@consflow.local` | P/N | N | N | R | N | P/D | N | N | R | R |

Golden Live có actor/project member chuyên biệt cho `Thiết kế`, `Sale` và `Pháp lý`. Các step yêu cầu một trong ba vai trò này phải dùng đúng actor tương ứng; nếu account hoặc membership bị thiếu, ghi `BLOCKED-COVERAGE` và không dùng Admin để biến coverage thiếu thành `PASS`.

Rev3 có fixture riêng `GOLDEN-REV3-BOQ-001` (project 023) và actor `Bộ phận HĐ` là `contract@consflow.local`. Khi kiểm thử Rev3, actor này phải nhìn thấy work item hợp đồng được giao nhưng không được dùng để thay QS/Kế toán/Pháp lý/Mua hàng/PM/Giám đốc ở các quyết định cần tách vai trò. Project 022 và ma trận lịch sử 15 vai trò phía trên không được rebound sang Rev3.

Chuyển role tuần tự và tôn trọng `Retry-After` khi login/auth trả `429`. Một response rate-limit làm case bị nhiễu và phải chạy lại; không được tính là negative RBAC `PASS`.

## Evidence package

Mỗi lần chạy dùng một `run-id` và lưu ngoài thư mục source chứa secret. Không ghi password, bearer/refresh token, cookie, DSN đầy đủ hoặc nội dung file nhạy cảm.

```text
uat-evidence/<run-id>/
  00-run-metadata.md
  01-db-guard.txt
  02-fixture-baseline.txt
  03-role-matrix.csv
  C1-design/J0...J9/
  C2-no-design/J0...J9/
  90-negative-rbac.md
  99-signoff.md
```

`00-run-metadata.md` phải có commit SHA, timestamp/timezone, operator/reviewer, environment, base URL, DB name (không có credentials), upload target, fixture IDs và browser authorization `yes/no`. Mỗi case ghi: case ID, cycle/journey, actor/role, precondition, action/endpoint, expected/actual HTTP status, entity IDs, before/after status, audit/activity ID, SQL/API evidence file, result và defect link. File upload ghi tên fixture, MIME, size và checksum; không nhúng secret.

## Tiêu chí PASS/FAIL

- Một cell là `PASS` khi mọi `P/D/R/N` trong cell có evidence và kết quả đúng; `BLOCKED` không được tính là pass.
- Một journey là `PASS` khi tất cả cell áp dụng pass, blocker dự kiến xuất hiện đúng lúc, requester separation/audit được giữ và không có side effect cấm.
- Suite chỉ `PASS` khi C1 và C2 cùng pass, validator pass trước/sau, hai completed fixture không đổi, nhánh đối nghịch được skip, mutation có actor/timestamp/ID truy vết và mọi balance/approval/history invariant còn đúng.
- Suite là `FAIL` ngay khi có cross-project/cross-role write, mutation completed fixture, bypass gate, approval sai actor, mất lịch sử, trùng side effect, fake invoice/payment/stock movement, reset nhầm target hoặc evidence không đủ để tái hiện.
- Defect chưa xử lý hoặc actor chuyên biệt bị thiếu/sai membership phải ghi `BLOCKED` kèm owner và điều kiện unblock; không hạ chuẩn bằng Admin hoặc SQL trực tiếp.

## Cổng ủy quyền trình duyệt

Roadmap hoặc review package đã duyệt không phải là quyền chạy trình duyệt. Chỉ dùng Browser, Chrome, Playwright hoặc headless browser khi người dùng yêu cầu rõ browser verification ngay trong turn thực thi đó. Khi chưa có quyền, chạy API/DB/non-browser checks, ghi `browser_authorized: no` và báo `browser verification skipped`. Sau khi được phép, vẫn phải qua cổng DB UAT cô lập và không được dùng browser session để truy cập production/shared data.
