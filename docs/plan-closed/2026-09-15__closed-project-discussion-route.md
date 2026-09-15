# [Closed] Tách Trao đổi thành mục riêng trong chi tiết Project

Status: IMPLEMENTED; TECHNICALLY VERIFIED; NOT UX ACCEPTED

Mục đích chính:
- Đưa `Trao đổi` thành không gian cấp một trong Project Workspace để phối hợp, quyết định và xử lý công việc mà không phụ thuộc tab của trang Tổng quan.

Done:
- Review package và Mockup as Text đã được người dùng duyệt.
- Đã thêm route lazy `/projects/:id/discussion`, mục cấp một `Trao đổi` chỉ trong Project Workspace và badge `discussion_unread` theo actor.
- Đã chuyển nguyên composition `ActivityThread` cùng workflow action, work rail/dialog, composer, attachment viewer và `ProjectStartupWizard` sang page riêng; không tạo trải nghiệm trao đổi song song.
- Đã gỡ tab nội bộ `Trao đổi`, chuyển các deep link sản phẩm và giữ redirect tương thích từ `?tab=activity` có bảo toàn query/hash còn lại.
- Không đổi activity API, cursor, quyền, audit, workflow hoặc schema.
- Đã cập nhật mapping trung tâm, frontend và Project Planning cho owner/route mới.
- Kiểm tra tập trung đạt 25/25; frontend TypeScript/Vite build và bundle budget đạt; `git diff --check` đạt.

Scope limitation:
- Browser/UAT không chạy; trạng thái UX là `NOT UX ACCEPTED`.
- Full frontend suite còn đúng một lỗi source-contract ngoài phạm vi tại `projectEvidenceColumns.test.ts`; `EvidenceGallery.tsx` và test đó không bị thay đổi trong plan này.
