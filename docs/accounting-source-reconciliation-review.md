# Đối soát MISA trực tiếp với hồ sơ công trình

Trạng thái: APPROVED và IMPLEMENTED — người dùng duyệt “Duyệt, triển khai đối soát trực tiếp hồ sơ” ngày 2026-10-05. Plan đã đóng theo yêu cầu ngày 2026-10-08; bằng chứng kỹ thuật/Docker local được giữ trong [historical closed plans](plan-closed/2026-10-08__closed-all-active-plans.md). Browser/UAT NOT_RUN, NOT UX ACCEPTED.

## Mục đích chính

Kế toán đối chiếu dữ liệu thu/chi đã nhập từ MISA với hồ sơ mua sắm và hợp đồng của công trình. Consflow không lập đề nghị thanh toán, không phê duyệt hoặc thực hiện thu/chi.

## Hiện trạng trước thay đổi và nguyên nhân phụ thuộc

- `accounting_operations_workspace.go` trả căn cứ, đề nghị, file MISA, ghép tiền và lịch sử.
- `accounting_misa.go` hiện bắt buộc `request_id` đã được tiếp nhận khi ghép giao dịch và xác nhận đối soát. Xóa nút Lập đề nghị đơn thuần sẽ khiến đối soát không dùng được với công trình mới.
- `AccountingOperationsDialog.tsx` hiện có tab Đề nghị theo đợt và các trường tạm ứng, thu hồi, giữ lại. Người dùng vừa xác nhận bỏ thao tác lập hồ sơ thanh toán hợp đồng và tập trung đối chiếu thu/chi.

## Hợp đồng đã duyệt và triển khai

1. Bỏ hàng đợi, lifecycle và thao tác lập/gửi/tiếp nhận đề nghị theo đợt; bỏ chức năng quản lý tạm ứng, thu hồi và giải phóng giữ lại.
2. Giữ căn cứ đối soát có hồ sơ nguồn, đối tác, chiều thu/chi, tiền tệ, giá trị và minh chứng được kế toán xác nhận. Căn cứ là số liệu để so sánh, không phải công nợ hoặc lệnh thanh toán.
3. Ghép giao dịch MISA trực tiếp với căn cứ. Chống ghép trùng/vượt số tiền giao dịch MISA; ghép một phần được phép và thể hiện rõ. Nếu số đã ghép khác giá trị căn cứ, hiển thị chênh lệch, không chặn công việc dự án hay coi đó là trạng thái chưa thanh toán.
4. Giữ nhập CSV, danh tính công trình/đối tác/tiền tệ/chiều giao dịch, thay thế file có kiểm soát, hủy ghép có lý do, fingerprint chống xác nhận dữ liệu đã đổi, concurrency và audit. File MISA vẫn là nguồn thu/chi chính thức, không giả đồng bộ trực tiếp.
5. Đổi contract API/schema từ `request_id` sang `basis_id` cho ghép và đối soát; xóa bảng/route/permission đề nghị đã bỏ. Dự án mới không có dữ liệu nghiệp vụ cần chuyển lịch sử. Migration dừng nếu gặp dữ liệu ngoài phạm vi.

## Mockup as Text

```text
ĐỐI SOÁT MISA · <Mã công trình>                         [×]
[Hồ sơ đối soát]  [Kết quả MISA]  [Lịch sử]

[Xác nhận căn cứ]                          [Nhập file MISA]
Hồ sơ nguồn                  Giá trị căn cứ   Đã ghép  Chênh lệch
<Mua sắm / Mốc hợp đồng>      <... VND>        <...>    <...>
<Đối tác> · Thu / Chi         [Mở hồ sơ] [Chọn đối chiếu]

Hồ sơ đang chọn: <Mã · Tên>
Giao dịch MISA [Chọn giao dịch phù hợp                 v]
Số tiền ghép  [.................................... VND]
                                      [Ghép với hồ sơ]
Các khoản đã ghép: <Ngày · Chứng từ · Số tiền> [Hủy ghép]
Ghi chú       [.......................................]
                                     [Xác nhận đối soát]

Chưa có căn cứ: Xác nhận căn cứ từ hồ sơ công trình.
Chưa nhập MISA: Nhập file để đối chiếu thu/chi.
Đang tải... / Không tải được dữ liệu [Thử lại]
```

Mobile xếp các cột thành thẻ; modal toàn màn hình theo Dialog hiện có. Inline validation và toast đóng được giữ nguyên. Không có nút Lập đề nghị, Gửi đề nghị hoặc giải phóng giữ lại.

Interaction → standard owner → entrypoints: dialog → `Dialog`; số tiền → `CurrencyInput`; file/minh chứng → `EvidenceRecordViewerDialog` và adapter đúng nguồn; nguồn hồ sơ → `AccountingHandoffDetailDialog`; entrypoints register kế toán và My Work. Không tạo viewer riêng.

## Kiểm chứng và giới hạn

- Unit/API/DB tests: quyền và truy cập chéo công trình, nguồn đủ điều kiện, nhiều giao dịch–nhiều căn cứ, chống ghép vượt giao dịch, chênh lệch căn cứ, thao tác lặp, cạnh tranh, thay file/hủy ghép, dữ liệu thay đổi sau xác nhận và rollback/audit.
- Đề nghị và endpoint cũ phải không còn; handoff và gate đóng hồ sơ vẫn giữ contract hiện hành.
- Frontend tests/build; cập nhật module/domain mappings. Browser/UAT chưa được yêu cầu nên NOT UX ACCEPTED.
- Files dự kiến: accounting_requests.go (thay bằng owner căn cứ), accounting_misa.go, accounting_operations_workspace.go, routes_finance.go, accounting_operations_test.go, AccountingOperationsDialog/AccountingRequestQueue/accountingOperations types, migration mới và các map tương ứng.

## Quyết định đã duyệt

1. Người dùng đã duyệt đối soát trực tiếp với căn cứ hồ sơ và Mockup as Text trên; loại bỏ toàn bộ bước lập đề nghị theo đợt.

Chênh lệch có dấu = đã ghép − giá trị căn cứ. source_available=false khi nguồn/minh chứng/quyền nguồn không còn đủ điều kiện; reviewed hiện tại=false, lịch sử giữ nguyên. Source-only detail không cấp handoff snapshot/actions. Quyền mới accounting_reconciliation.view/manage; giữ accounting_misa.reconcile. Canonical actor/permission/evidence được kiểm tra sau các điểm chờ khóa trên cùng transaction; tiền lịch sử/hiển thị giữ chuỗi thập phân. Standard Dialog/CurrencyInput/EvidenceRecordViewerDialog được tái sử dụng; frontend510tests/build và full backend PostgreSQL/Redis PASS. Docker local đã cóV151 và readiness/API checks PASS; không có browser/UAT acceptance.
