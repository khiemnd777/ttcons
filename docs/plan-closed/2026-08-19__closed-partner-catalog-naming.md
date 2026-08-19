# [Closed] Đổi tên Danh mục đối tác và nguồn cung vật tư

- Ngày đóng: 2026-08-19
- Trạng thái: Đã triển khai và kiểm tra kỹ thuật; `NOT UX ACCEPTED` do chưa chạy Browser/UAT.

## Will be done

- Đổi menu `Dữ liệu lõi` thành `Đối tác`, giữ nguyên route `/core-data` và quyền `catalog.view`.
- Đổi tiêu đề trang thành `Danh mục đối tác`, cập nhật mô tả và CTA thành `Thêm đối tác`.
- Đổi trường/cột nguồn cung của vật tư thành `Nhà cung cấp tham khảo`; đổi trường nhập tay thành `Tên nhà cung cấp khác` và thêm hướng dẫn ngắn.
- Cập nhật mapping frontend, Kho vật tư và Quản trị cho các nhãn UI mới.
- Chạy frontend tests/build và rà soát diff; không chạy browser theo quy định repo.

## Done

- Đã đối chiếu phạm vi: thay đổi chỉ ở copy/placeholder frontend; không đổi route, API, RBAC, dữ liệu hoặc snapshot nhà cung cấp.
- Đã đổi menu thành `Đối tác`, tiêu đề trang thành `Danh mục đối tác`, mô tả trang và CTA thành `Thêm đối tác`.
- Đã đổi trường/cột vật tư thành `Nhà cung cấp tham khảo`, trường nhập tay thành `Tên nhà cung cấp khác`, đồng thời thêm placeholder và helper text đã duyệt.
- Đã cho phép `SimpleForm` dùng placeholder riêng cho empty option của select; các select không cấu hình vẫn hiển thị `Chọn`.
- Đã thêm kiểm thử presentation và cập nhật module/frontend/domain mapping liên quan.
- Đã chạy focused presentation test: 2/2 passed.
- Đã chạy toàn bộ frontend test: 207/207 passed.
- Đã chạy `npm run build`: TypeScript và Vite production build passed trên working tree hiện tại.
- Browser/UAT không chạy theo quy định repo; trạng thái UI/UX là `NOT UX ACCEPTED`.
