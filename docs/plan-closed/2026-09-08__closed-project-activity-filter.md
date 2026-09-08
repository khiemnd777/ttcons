# [Closed] Rút gọn bộ lọc Trao đổi công trình — 2026-09-08

## [Closed] Rút gọn bộ lọc Trao đổi công trình

- Mục tiêu: bỏ tab `Tất cả` và composer dạng card tại luồng Trao đổi của công trình; mặc định mở `Thảo luận`, giữ `Quyết định` cùng `Nhật ký hệ thống` ở chế độ chỉ đọc.
- Đã làm: người dùng đã duyệt review package/mockup và chấp nhận tác động sau khi phân biệt activity `Quyết định` với phê duyệt workflow; `ActivityThread.tsx` chỉ còn ba nhóm, luôn gửi `kind` và chỉ mount composer dạng chat trong `Thảo luận`. Composer dạng card dùng tại Đầu việc và composer dạng chat của Utility Dock vẫn giữ nguyên; mapping liên quan đã được cập nhật. Bộ kiểm thử hẹp `projectActivityTabs`, `activityChatInput`, `utilityDock` pass 17/17 và frontend build pass trên working tree hiện tại.
- Trạng thái đóng: TECHNICALLY VERIFIED theo các kiểm tra ghi nhận; NOT UX ACCEPTED vì chưa chạy browser/UAT trong lượt này theo quy ước repository.
- Phạm vi: chỉ frontend presentation/filter selection. Không đổi API, phân quyền, dữ liệu, thông báo, hay trạng thái đọc.
