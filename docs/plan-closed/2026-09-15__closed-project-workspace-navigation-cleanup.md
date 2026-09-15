# [Closed] Tối giản cây nghiệp vụ công trình

Status: IMPLEMENTED; TECHNICALLY VERIFIED; NOT UX ACCEPTED

Mục đích chính:
- Làm cây nghiệp vụ công trình gọn và chính xác hơn: nhóm điều hướng trực tiếp không gợi ý sai rằng có submenu, trong khi phân cấp submenu vẫn dễ nhận biết.

Done:
- Review package và Mockup as Text đã được người dùng duyệt ngày 15-09-2026.
- `Tổng quan` và `Trao đổi` không còn render chevron vì không có submenu.
- Nhóm có submenu vẫn giữ chevron; các mục con giữ thụt lề nhưng đã bỏ đường kẻ dọc bên trái.
- Đã bổ sung regression coverage cho conditional chevron, child indentation và việc không còn `border-l` trong renderer.
- Đã đồng bộ frontend UI utility map và Project Planning domain map.
- Trên local working tree: `npm run test:project-navigation` đạt 6/6; `npm run build` đạt; `git diff --check` đạt.

Scope limitation:
- Không chạy browser/UAT vì không được yêu cầu riêng trong lượt triển khai; trạng thái UX là `NOT UX ACCEPTED`.
- Không thay đổi route, API, quyền, badge, backend, cơ sở dữ liệu hay migration.
