# Golden Live - Sổ tay trải nghiệm vận hành

## Truy cập

- Project: `GOLDEN-LIVE-001` - `Golden Live – Tự vận hành công trình`
- URL: `http://localhost:5173/projects/30000000-0000-0000-0000-000000000022`
- Template: `40000000-0000-0000-0000-000000000020`
- Mật khẩu chung: `123456`

Golden Live bắt đầu ở node `Khảo sát`, tiến độ 0%. Không có form đã nộp, evidence, approval hoặc chứng từ nghiệp vụ phát sinh.

## Tài khoản theo vai trò

| Vai trò | Tài khoản |
|---|---|
| Admin | `admin@consflow.local` |
| PM | `pm@consflow.local` |
| QS | `qs@consflow.local` |
| Kế toán | `accounting@consflow.local` |
| Thủ kho | `warehouse@consflow.local` |
| Giám sát | `supervisor@consflow.local` |
| Giám đốc | `director@consflow.local` |
| Mua hàng | `procurement@consflow.local` |
| QA/QC | `qaqc@consflow.local` |
| Đội thi công | `construction@consflow.local` |
| Thầu phụ | `subcontractor@consflow.local` |
| Chủ đầu tư | `client@consflow.local` |
| Thiết kế | `design@consflow.local` |
| Sale | `sales@consflow.local` |
| Pháp lý | `legal@consflow.local` |

Admin có thể thao tác hầu hết nghiệp vụ nhưng không được dùng thay actor chuyên biệt khi kiểm chứng coverage. Khi requester separation được bật, dùng PM gửi yêu cầu và Giám đốc hoặc vai trò phù hợp để duyệt.

Khi chuyển liên tiếp nhiều tài khoản trong UAT, chạy tuần tự và tôn trọng `Retry-After` nếu API trả `429`; không tính một response rate-limit là bằng chứng negative RBAC.

## Kịch bản công trình

- Khách hàng: Công ty An Phát.
- Địa điểm: Tầng 8, Quận 3, TP.HCM.
- Phạm vi: hoàn thiện văn phòng, MEP, trần, sơn, sàn, pantry và chiếu sáng.
- Ngân sách dự kiến: 2.850.000.000 VND.
- Giá trị hợp đồng dự kiến: 4.200.000.000 VND.
- Thời gian dự kiến: 03/08/2026 - 30/01/2027.

## Luồng vận hành đề nghị

### 1. Khảo sát

- PM/Giám sát/QS nhập năm form khảo sát.
- Giám sát upload evidence ảnh hiện trạng cho step evidence-only.
- Hoàn tất sáu work item, yêu cầu duyệt node và dùng tài khoản khác để duyệt.

### 2. Họp báo cáo khảo sát

- Nhập khối lượng sơ bộ, layout, 3D, bản vẽ thi công và biên bản họp.
- Tại form `client_design_branch_decision`, chọn `yes` để đi nhánh Có thiết kế hoặc `no` để đi nhánh Không thiết kế.
- Sau khi chuyển node, engine sẽ tự skip nhánh còn lại. Muốn thử lại nhánh khác phải reset seed.

### 3-5. Thiết kế và BOQ

- Dùng PM/Chủ đầu tư để review và xác nhận thiết kế.
- Dùng QS tạo BOQ, khóa định mức và thu thập báo giá.
- Tạo ít nhất hai báo giá nhà cung cấp, submit/qualify, so sánh và chọn giá.
- Chỉ generate procurement sau khi BOQ đã duyệt và final price được khóa.

### 6-8. Đầu vào, đàm phán và hợp đồng

- Mua hàng tạo procurement, PO, ngày giao dự kiến và thực hiện lifecycle submit/approve/order.
- QS/PM tạo hợp đồng chủ đầu tư, thầu phụ và nhà cung cấp.
- Thực hiện version review/approve/sign/activate, milestone, phụ lục và retention.
- PM gửi yêu cầu duyệt; Giám đốc duyệt với comment.

### 9. QA/QC và thi công

- QA/QC tạo ITP, submit, approve và activate.
- Giám sát nộp nhật ký thi công với nhân lực, khối lượng, vật tư và evidence.
- Tạo một RFI pass và một RFI fail để trải nghiệm NCR.
- Với RFI fail: nhập root cause, corrective action, hoàn tất rework, tạo RFI tái kiểm rồi đóng NCR.

### 10. Nghiệm thu đầu vào và thầu phụ

- Tạo incoming acceptance cho từng procurement delivery.
- Kho chỉ thay đổi khi thực hiện explicit stock receive/issue/return.
- Thực hiện checklist và reconciliation thầu phụ; giữ chênh lệch khối lượng và quyết định xử lý.
- Tạo AP, lịch thanh toán, tạm ứng, hóa đơn đầu vào và payment allocation.

### 11. Nghiệm thu đầu ra

- Tạo nghiệm thu có điều kiện hoặc fail để thử punch list/rework.
- Tái nghiệm thu pass nhưng giữ nguyên lịch sử lần trước.
- Thực hiện owner/client reconciliation, hồ sơ quyết toán, AR, hóa đơn đầu ra và thu tiền nhiều kỳ.

### 12. Bàn giao và đóng tài chính

- Xử lý hết vật tư dư và đóng site balance.
- Đối soát AP/AR về 0, không còn payment pending.
- Xác minh đủ tám checklist bàn giao bằng acceptance/evidence/document/system source.
- Submit, check readiness và complete handover.

## Blocker có chủ đích của engine

- Thiếu exact submitted form hoặc required answer.
- Thiếu evidence/photo/document bắt buộc.
- Work item hoặc run step chưa hoàn tất.
- Node chưa được phê duyệt.
- BOQ chưa duyệt/chưa khóa giá hoặc procurement chưa đủ lifecycle.
- NCR/rework/issue còn mở.
- Vật tư dư chưa xử lý.
- AP/AR còn số dư hoặc payment còn pending.
- Handover checklist chưa verified.

Không sửa database để vượt blocker; hoàn thành nghiệp vụ nguồn qua UI/API.

## Reset

Để đưa Golden Live về node Khảo sát và dữ liệu trắng:

```bash
cd backend
make seed
```

Lệnh này reset toàn bộ dữ liệu demo local, tái tạo Golden completed suite và Golden Live, sau đó chạy validator. Không chạy seed trên production.
