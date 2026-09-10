# [Closed] Đồng bộ avatar hiện tại vào Trao đổi — 2026-09-08

## [Closed] Đồng bộ avatar hiện tại vào Trao đổi

- Mục tiêu: hiển thị ảnh avatar đã upload của tài khoản đang đăng nhập trong Project Activity và Utility Dock, trong khi giữ initials cho thành viên khác.
- Đã làm: người dùng đã duyệt review package và Mockup as Text; xác nhận `ActivityThread.tsx` và `UtilityDock.tsx` gọi `actorInitials` trực tiếp, chưa dùng `UserAvatar`. Đã tái dùng `UserAvatar` cho tin nhắn của actor hiện tại, bổ sung kích thước compact và regression test nguồn. Không thêm peer-avatar endpoint; người khác tiếp tục dùng initials.
- Trạng thái đóng: TECHNICALLY VERIFIED với 21/21 test presentation liên quan và `npm run build` frontend trên working tree đã cập nhật. NOT UX ACCEPTED vì chưa chạy browser/UAT trong lượt này.
- Phạm vi: presentation-only ở Project Activity/Utility Dock. Không đổi API, RBAC, audit, read cursor hay migration.
