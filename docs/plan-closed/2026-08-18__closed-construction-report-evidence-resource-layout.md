# [Closed] Báo cáo thi công: preview minh chứng và nguồn lực một cột

Closed: 2026-08-18
Reason: User requested closing the completed plan.

## Will be done

- Sửa vòng đời object URL để ảnh mới chọn hiển thị ổn định trong React StrictMode.
- Cho phép xem ảnh/tệp của minh chứng đã lưu qua endpoint bảo vệ hiện có.
- Đổi tiêu đề `Checklist báo cáo` thành `Công việc trong ngày`.
- Xếp Nhân công / tổ đội, Vật tư sử dụng và Ghi chú hiện trường theo một cột; hai danh sách nguồn lực dùng bảng edit inline.
- Bổ sung regression tests, cập nhật module/frontend/root-cause/domain mapping, chạy frontend tests và build.
- Thay hành động mở ảnh raw bằng viewer dùng chung trong ứng dụng cho cả ảnh mới chọn và ảnh đã lưu; giữ raw protected action cho tài liệu không phải ảnh.
- Tách composition `Xem minh chứng` đầy đủ thành viewer dùng chung: ảnh đã lưu phải có metadata bản ghi như màn chi tiết; ảnh mới chọn dùng cùng bố cục với trạng thái `Chưa upload` và không dựng giả metadata persistence.

## Done

- Đã chẩn đoán và được người dùng duyệt review package cùng Mockup as Text.
- Đã xác nhận backend `/evidence/:id/file` và permission scope hiện có đủ cho preview; không cần đổi API hoặc database.
- Đã sửa preview tệp mới chọn bằng object URL được tạo và thu hồi trong cùng effect, đồng thời thêm trạng thái lỗi đọc ảnh và liên kết mở tệp.
- Đã thêm thumbnail, trạng thái tải/lỗi/thử lại và hành động mở minh chứng đã lưu qua protected file route.
- Đã đổi tiêu đề thành `Công việc trong ngày`; chuyển Nhân công / tổ đội và Vật tư sử dụng thành hai bảng edit inline toàn chiều rộng, xếp một cột phía trên Ghi chú hiện trường.
- Đã cập nhật regression tests và các module/frontend/root-cause/construction-domain mapping liên quan.
- Đã chạy `npm test` sau thay đổi viewer: 158 test đạt, 0 test lỗi.
- Đã chạy `npm run build`: TypeScript và Vite build đạt trên working tree hiện tại.
- Đã chạy `npm run check:bundle-budget`: entry 122,1 KiB/250 KiB và 118 chunks, đạt ngân sách.
- Chưa chạy browser/UAT theo quy định repository; trạng thái giao diện là `NOT UX ACCEPTED` cho đến khi có kiểm thử trực tiếp.
- Đã được người dùng duyệt review package sửa luồng xem ảnh và Mockup as Text; viewer phải giữ nguyên dữ liệu form khi đóng.
- Đã tách `EvidenceViewerDialog` dùng chung và nối vào EvidenceGallery, EvidenceCapture và báo cáo thi công; ảnh dùng viewer trong ứng dụng, tài liệu không phải ảnh giữ hành động mở tệp.
- Đã chạy focused viewer/protected-file regression: 9 test đạt, 0 test lỗi; kiểm tra cả StrictMode object URL, loading/retry/decode error và Escape của dialog lồng nhau.
- Build hiện tại sau thay đổi viewer đạt; bundle budget đạt với entry 122,1 KiB/250 KiB và 118 chunks.
- Đã được người dùng duyệt review package sửa abstraction viewer: form báo cáo phải dùng đúng composition `Xem minh chứng`, không chỉ dùng chung modal shell.
- Đã tách `EvidenceRecordViewerDialog` dùng chung khỏi `EvidenceGallery`; gallery và form báo cáo dùng cùng component cho bản ghi đã lưu, còn ảnh mới chọn dùng `EvidenceDraftViewerDialog` cùng bố cục với trạng thái `Chưa upload` và chỉ hiển thị metadata đang tồn tại.
- Đã giữ thao tác duyệt/từ chối theo opt-in của gallery; form báo cáo không nhận các thao tác review. Protected file route, object URL và nested Escape không đổi.
- Đã chạy regression test sau khi dùng chung composition đầy đủ: 158 test đạt, 0 test lỗi.
- Đã chạy lại `npm run build` trên source cuối: TypeScript và Vite build đạt, 1.826 modules được transform; bundle budget đạt với entry 122,1 KiB/250 KiB và 118 chunks.

## Closure

- Closed on 2026-08-18 at the user's request.
- This file is a historical record only. Durable ownership, viewer behavior and construction-report invariants remain in the Consflow mapping files.
