# Công trình mẫu — sổ tay vận hành hiện tại

Tài liệu giữ đường dẫn cũ để các liên kết tiếp tục hoạt động. Công trình `GOLDEN-LIVE-001` và các UUID cố định trước đây không còn được seed tạo. Luồng hiện tại dùng danh mục demo additive và SampleData v5 qua ứng dụng có xác thực.

## Chuẩn bị

Chỉ dùng môi trường local/disposable có DB và storage riêng. Áp dụng migration hiện tại trước khi bootstrap danh mục:

```bash
cd backend
make migrate
make seed
make seed-validate
make seed-fixtures-check
```

`make seed` và `seed-validate` có thể chọn PostgreSQL Compose. Với target cô lập tường minh, dùng quy trình URL riêng trong [seed policy](../../docs/ops/seed-policy.md). Seed không reset, không chạy lại migration và không tạo hồ sơ nghiệp vụ hoàn tất. Giữ PDF fixture để thử tải/đọc minh chứng qua ứng dụng.

## Tạo công trình mẫu

1. Đăng nhập actor có `sample_data.create`; mở **Cấu hình → Dữ liệu mẫu**.
2. Chọn pipeline published trong môi trường hiện tại. Rev9 chính thức có ID `40000000-0000-0000-0000-000000000028`; vẫn kiểm tra danh sách và blockers trước khi tạo.
3. Tạo tài khoản mẫu theo vai trò nếu thiếu. Quyền dựa trên role canonical của pipeline, không dựa tên/chức danh.
4. Chọn `draft` 0%, `planning` theo preset hoặc `completed` 100%; chỉ tạo khi hồ sơ tương thích.
5. Lưu project ID/code, template revision/hash, generator version, actor và idempotency key; mở công trình bằng ID trả về.

`draft` dùng cho thiết lập mới; `planning` dùng kiểm tra tiến độ mẫu được hỗ trợ; `completed` dùng đọc hồ sơ tổng hợp đã sinh. Hồ sơ hoàn tất là mô phỏng, không chứng minh vận hành hoặc thanh toán thật. Để trải nghiệm thủ công từ đầu, tạo công trình qua startup thông thường.

| API có xác thực và quyền sample_data.create | Nội dung |
|---|---|
| `GET /api/settings/sample-data/options` | Blueprint version, pipelines, roles, blockers, progress presets |
| `POST /api/settings/sample-data/users` | Body có `pipeline_template_id` |
| `POST /api/settings/sample-data/projects` | Body có `profile`, `progress_percent`, `pipeline_template_id`, `idempotency_key` UUID |

Retry giữ nguyên actor, payload và key; dùng key mới để tạo mẫu mới. Không cập nhật SQL để vượt blockers.

## Trải nghiệm vận hành thủ công

| Phần nghiệp vụ | Thao tác và bằng chứng |
|---|---|
| Khảo sát và thiết kế | Form/minh chứng yêu cầu, chọn nhánh thiết kế và duyệt đúng actor |
| BOQ và mua sắm | Duyệt BOQ, khóa giá, báo giá/so sánh và procurement qua lifecycle hợp lệ |
| Hợp đồng | Phiên bản, duyệt/ký/activate, mốc/phụ lục và snapshot điều khoản đã ký |
| Kế hoạch và thi công | Bàn giao kế hoạch, nhật ký, khối lượng, vật tư và minh chứng đúng nguồn |
| QA/QC và nghiệm thu | RFI/NCR/rework, tái kiểm và reconciliation; giữ lịch sử fail/pass |
| Kho và bàn giao | Nhập/xuất/trả vật tư tường minh, xử lý tồn công trường và checklist bàn giao |
| Bàn giao công trình và hồ sơ kế toán | Bàn giao nguồn hồ sơ, tiếp nhận/trả bổ sung có actor/lý do/audit |
| Đối soát MISA | Xác nhận căn cứ có nguồn/minh chứng, nhập MISA thử nghiệm và phân bổ trực tiếp vào căn cứ |

Đối soát giữ nguyên số tiền căn cứ để hiển thị chênh lệch; tổng phân bổ trên một giao dịch MISA không vượt số tiền giao dịch đó. Hủy phân bổ phải có lý do và giữ lịch sử. Luồng hiện tại đã bỏ đề nghị thanh toán, AP/AR, hóa đơn/VAT, sổ kế toán và đóng tài chính nội bộ. Bàn giao và hồ sơ mẫu hoàn tất không xác nhận đã thu/chi tiền.

## Vai trò và chạy lại

Seed có các tài khoản demo `admin`, `developer`, `pm`, `qs`, `accounting`, `warehouse`, `supervisor`, `director`, `procurement`, `qaqc`, `construction`, `subcontractor`, `design`, `sales`, `legal`, `contract` tại miền `consflow.local`. Membership công trình phải kiểm tra riêng. Tài khoản SampleData sinh dùng miền `dev.local`, tách với demo masters.

Không dùng Admin thay actor cần coverage. Chủ đầu tư ngoài App được xác nhận bằng hồ sơ/minh chứng; demo `client@consflow.local` inactive dùng kiểm tra từ chối đăng nhập. Chuyển role tuần tự và tuân thủ `Retry-After` khi API trả `429`.

Tạo công trình mới với key mới hoặc dùng môi trường disposable mới cho lần chạy độc lập. `make seed` không đưa công trình về Giai đoạn đầu và không xóa dữ liệu. Lưu request/response, actor, IDs, trạng thái trước/sau và audit trước khi xử lý lỗi.

Tham khảo [role-based UAT](../../docs/operations/golden-sample-role-based-uat.md) cho evidence và tiêu chí nghiệm thu. Tài liệu không khẳng định UAT đã chạy; browser verification cần yêu cầu rõ trong turn thực thi.
