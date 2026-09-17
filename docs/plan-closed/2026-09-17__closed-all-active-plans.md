# [Closed] Active plans archive — 2026-09-17

Closed by user request on 2026-09-17. The archived statuses and limitations remain authoritative: closing these plans does not claim pending browser/UAT work is accepted.

## [Closed] Utility Dock / Project Workspace Footer Alignment

Status: TECHNICALLY VERIFIED; NOT UX ACCEPTED.

### Will be done

- No remaining implementation work. Browser scroll verification remains a separate, user-authorized UAT step.

### Done

- Confirmed that AppShell's Project Workspace focus wrapper added document-flow end clearance around the viewport-minimum, sticky-sidebar workspace.
- Removed that outer clearance while preserving the fixed Utility Dock and standard-shell main clearance.
- Added focused regression coverage and updated the frontend, central routing and root-cause maps.
- Verified with `node --experimental-strip-types --test tests/utilityDock.test.ts` and `npm run build` in `frontend`.

## [Closed] Theo dõi đơn vị vật tư vật lý và phần dư tái sử dụng

Status: TECHNICALLY VERIFIED; NOT UX ACCEPTED.

### Will be done

- Không còn hạng mục triển khai trong phạm vi đã duyệt. Kiểm thử trình duyệt/UAT vẫn là bước riêng cần được cho phép rõ ràng; hiện trạng UI là `NOT UX ACCEPTED`.

### Done

- Đã xác nhận luồng hiện tại là Vật tư -> Kho -> Công trình -> Kho, với `stock_movements` là sổ kho chuẩn và `site_material_balances` là số dư tại công trình.
- Trước triển khai, hệ thống chỉ xử lý số lượng tổng hợp; chưa có định danh, kích thước, nguồn gốc tách/cắt và trạng thái tái sử dụng cho từng Đoạn/Cuộn/Cây/Tấm.
- Đã duyệt review package, Mockup as Text, phạm vi triển khai và kế hoạch xác minh.
- Đã thêm migration V144 với chế độ `aggregate`/`physical_unit`, dạng Đoạn/Cuộn/Cây/Tấm, sổ `material_stock_units`, lịch sử bất biến, liên kết biến động kho, phân bổ snapshot/surplus và lượng hao hụt tại công trình.
- Đã triển khai nhập, xuất, chuyển, hoàn kho/nhà thầu theo đúng đơn vị vật lý; tách một phần giữ quan hệ gốc/cha, chỉ phần `usable` được tái xuất và phần không dùng được vào cách ly.
- Đã triển khai khởi tạo có đối soát cho tồn cũ, khóa đồng thời theo vật tư, chặn điều chỉnh tổng hợp có thể làm lệch sổ con và giữ phiếu vật lý đã ghi sổ bất biến về dữ liệu tồn.
- Đã tích hợp phân bổ dùng/hao hụt trong ghi nhận vật tư của Đầu việc và phân bổ hoàn phần dư; số lượng sổ tổng hợp và sổ đơn vị được cập nhật trong cùng giao dịch.
- Đã bổ sung tab `Phần dư tái sử dụng`, chi tiết lịch sử đơn vị, form danh mục/chứng từ/khởi tạo và các chỉ số tái xuất/cách ly; giữ nhánh chỉ đọc `Vật tư` của Thiết kế không truy cập dữ liệu kho.
- Đã cập nhật bản đồ module/backend/frontend/domain và thêm kiểm thử trình bày tập trung.
- Đã xác minh migration V1-V144 trên PostgreSQL tạm thời và ca tích hợp nhập 10 -> xuất 6 -> hoàn 4 -> tái xuất 3; kết quả tồn kho vật lý/tồn công trình/sổ kho đều bằng 5 theo kỳ vọng.
- Đã chạy `make migrate-check`, `go test ./...`, frontend build và 6/6 kiểm thử tập trung. Toàn bộ frontend suite còn 1 lỗi nền không thuộc phạm vi tại `projectEvidenceColumns.test.ts`; 480/481 kiểm thử qua.
