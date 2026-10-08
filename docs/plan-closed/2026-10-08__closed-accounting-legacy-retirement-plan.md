# [Closed] Dọn kế toán nội bộ cũ và đối soát trực tiếp hồ sơ

Ngày: 2026-10-05, Asia/Ho_Chi_Minh. Trạng thái: IMPLEMENTED và TECHNICALLY VERIFIED trên working tree/Docker local. Browser/UAT NOT_RUN, NOT UX ACCEPTED.

## Mục đích chính và quyết định đã duyệt

Consflow quản lý công trình, tiến độ, mua sắm, kho, chất lượng và báo cáo vận hành. Kế toán đối chiếu kết quả thu/chi MISA với hồ sơ nguồn. Người dùng xác nhận chưa chạy production, không có lịch sử nghiệp vụ; duyệt “Chốt, triển khai” và “Duyệt, triển khai đối soát trực tiếp hồ sơ”.

- Không chặn thi công theo số tiền đã thu.
- Bỏ mở nghĩa vụ công nợ, lập đề nghị theo đợt và giải phóng giữ lại. Giữ điều khoản/giá trị/mốc hợp đồng làm thông tin dự án.
- Bỏ báo cáo dòng tiền, lợi nhuận và thuế cũ. Giữ tiến độ, mua sắm, kho, nhân sự và đối soát MISA.
- Dọn xuyên UI/API/service/repository/schema/permissions/seed. Không khôi phục sổ mặc định, không giữ cột sổ nullable hoặc lớp đọc kế toán cũ.
- Giữ bàn giao hồ sơ kế toán và gate đóng hồ sơ Rev9. Tiếp nhận hồ sơ không chứng minh thanh toán.

## Bằng chứng lỗi và thay đổi

Request `ca963357-9e01-49b4-9ec9-f14a03797b0c` tạo `PRJ-78805` thất bại tại INSERT, PostgreSQL23503 `projects_accounting_book_fk`. V73 bắt buộc sổ mặc định đã bị xóa trong reset local; không có công trình được tạo. V150 xóa FK/default/cột cùng mọi consumer. Regression tái hiện lỗi ở V149 rồi chứng minh INSERT thành công sau V150 khi không có sổ.

Bootstrap: central/backend/frontend/triage maps và accounting/project/workflow/commercial/materials/quality/reporting domains. First searches: `accounting_book_id|CreateProjectWithStartup|nodeBlockers|client_payment_min_percent|ProjectFinanceMode|accounting_requests|basis_id`.

| Xóa | Giữ và owner hiện hành |
|---|---|
| Sổ, tài khoản, kỳ, bút toán, AP/AR, thanh toán, hóa đơn/VAT, budget ledger | Project startup/import, actor, audit/outbox; project_startup.go và project_excel_repository.go |
| Chi phí vật tư/cash requests nội bộ, gate tiền thu | Mua sắm/BOQ/phê duyệt/kho/nghiệm thu; repository_pipeline.go và repository_handover.go |
| Mở nghĩa vụ, release giữ lại, proposal lifecycle/queue | Điều khoản/mốc hợp đồng đủ điều kiện; repository_commercial.go |
| Ledger/debt screens, money reports/dashboard metrics/CSV | AccountingHandoffPage, operational Reports/Statistics/Dashboard |
| Đề nghị theo đợt và request_id matching | accounting_reconciliation.go, accounting_misa.go: basis_id matching/review |
| 12 SQL Golden runtime/ledger fixtures | Seed master-only additive; 8 PDF fixtures; sample v5 theo nguồn Rev9 thật |
| 10 definitions lỗi thời khỏi catalog chạy | Rev9 published; nội dung definitions và Forms giữ nguyên |

## Manifest schema và an toàn local

- V150: 31 bảng kế toán cũ, view/functions/triggers/FK/cột liên quan; bỏ permission/settings cũ và rút exact standard/demo/Rev1–Rev8. DROP RESTRICT, không CASCADE.
- V151: bỏ requests/events, đổi request_bases thành accounting_reconciliation_bases; allocations/reviews dùng basis_id và same-project FK. Giữ MISA originals/history/audit.
- Migrations dừng atomically khi gặp dữ liệu nghiệp vụ ngoài phạm vi, book bất thường hoặc kỳ đã đóng. Giữ toàn bộ checksum V1–V149; fresh install và empty upgrade cùng schema cuối.
- Không chạy seed/reset tài khoản/database chính. Sao lưu mới và rehearsal bản sao trước áp dụng Docker local; thông tin thực thi cuối ghi trong docs/plan-closed/2026-10-08__closed-all-active-plans.md.
- Không có production operation. Backup database không thay thế backup storage; storage hiện có được giữ.

## Contract đối soát đã triển khai

Căn cứ được kế toán xác nhận có snapshot nguồn/minh chứng bất biến, công trình, đối tác, chiều thu/chi, tiền tệ và giá trị so sánh. Căn cứ không tạo công nợ/lệnh thanh toán.

Ghép nhiều–nhiều, từng phần được phép. Tổng ghép không vượt giao dịch MISA; có thể vượt giá trị căn cứ. Chênh lệch có dấu = đã ghép − căn cứ. Mỗi write khóa công trình/record, operation UUID chống lặp, tiền numeric chính xác, audit cùng transaction. Hủy ghép cần lý do; thay file phải hủy ghép cũ.

Nguồn/minh chứng/quyền nguồn không còn hợp lệ: source_available=false, reviewed hiện tại=false, không ghép/xác nhận mới; snapshot và lịch sử vẫn giữ. Fingerprint đổi làm xác nhận cũ stale.

Permissions: accounting_reconciliation.view/manage và accounting_misa.reconcile hiện có. Source detail với reconciliation.view chỉ trả nguồn hiện tại khi có procurement.view/contract.view; không trả handoff snapshot/actions nếu thiếu accounting_handoff.view.

CSV là format Consflow quy định, tối đa2MB/2000rows; không tuyên bố native MISA parser/API sync.

## UI đã duyệt và standard reuse

Mockup đối soát trong [review đã duyệt](accounting-source-reconciliation-review.md). Tạo công trình giữ wizard Thông tin → Chọn nhân sự → Thiết lập; không có trường sổ. Sample bỏ bước sổ, giữ Nháp/Lập kế hoạch/Hoàn thành và blockers thực tế.

- Dialog/form → Dialog, FormDrawer, SimpleForm, Field và toast đóng được.
- Tiền → CurrencyInput/semantic currency.
- Hồ sơ nguồn → AccountingHandoffDetailDialog với adapter quyền nguồn.
- Photo/PDF/Word/Excel/workflow PDF → EvidenceRecordViewerDialog và protected source adapter đúng ID/revision.
- My Work giữ hàng đợi bàn giao hồ sơ; bỏ queue đề nghị thanh toán.
- Mobile sử dụng layout/modal responsive hiện có. Browser chưa chạy.

## Kiểm chứng và trạng thái công việc

Đã làm: implementation xuyên tầng, cập nhật fixtures theo schema/Rev9 hiện hành, tests đối soát/quyền/concurrency/idempotency/audit, sample8profiles/sourcegates, fresh/upgrade/data guards/seed, maps và docs.

Bằng chứng current working tree trên base0c111ac: frontend510tests PASS, build PASS4.55s; backend make test/vet/build PASS. Full go test ./... -count=1 trên PostgreSQL/Redis mới PASS (core188.242s, migrations85.192s). Các lần fullsuite trước có lỗi fixture cũ/sharedDB/time-boundary, được ghi riêng là FAIL; assertions nghiệp vụ hiện hành được giữ. Review độc lập cuối không còn blocker sau sửa quyền chờ khóa và precision history.

Docker local đã rebuild/restart theo down → build → run và custom runner áp dụng2/skipped149; main có151migrations. Backup mới được restore/rehearse;10 bảng identity/role/form/definition có counts/content fingerprints không đổi. Original PRJ-78805 INSERT thành công rồi rollback; main vẫn0projects/runs và2users. Readiness/admin HTTP200, authenticated permissions/current APIs/Dashboard/retired404/logout kiểm tra PASS. Chi tiết binary/source/backup/migration hashes và giới hạn trong 2026-10-08__closed-all-active-plans.md. Không còn việc implementation/non-browser technical trong scope; Plan đóng theo yêu cầu ngày 2026-10-08; kết quả commit/push được xác minh khi bàn giao.

Giới hạn: tests isolated không chứng minh browser/UAT. NOT UX ACCEPTED; production không được truy cập.


Closure date: 2026-10-08, Asia/Ho_Chi_Minh. Historical implementation and verification record only. Browser/UAT remains NOT_RUN, NOT UX ACCEPTED. Runtime snapshots above retain their original verification dates.
