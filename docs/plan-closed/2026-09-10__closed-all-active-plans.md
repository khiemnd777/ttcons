# [Closed] Active plans archive — 2026-09-10

Closed by user request on 2026-09-10. The archived statuses and limitations remain authoritative: closing these plans does not claim pending browser/UAT work is accepted.

## [Closed] Hồ sơ thiết kế — gom cập nhật chat theo lô upload

- Status: TECHNICALLY VERIFIED, NOT UX ACCEPTED.
- Done: Người dùng đã duyệt review package và Mockup as Text. Đã xác nhận nguyên nhân: `DesignReleasePanel` gửi từng tệp riêng, `AddDesignReleaseFile` tạo một `work_update` với event `file:<evidence_id>` cho mỗi tệp, còn ActivityThread hiển thị đúng một bong bóng cho mỗi activity. Đã tạo một UUID batch cho mỗi lượt upload; vẫn lưu Evidence/audit theo từng tệp nhưng chỉ hoàn tất một snapshot `work_update` bất biến `files:<batch_id>` chứa đúng các tệp thành công theo thứ tự người dùng chọn. Thẻ `Cập nhật công việc` tải snapshot được bảo vệ, hiển thị một danh sách đánh số và mở từng tệp bằng shared `EvidenceReferenceViewButton`; trạng thái loading/error/retry được giữ ngay trong thẻ. Lượt retry dùng batch mới; activity phát hành/bàn giao, upload không có batch và snapshot lịch sử cũ giữ nguyên. Đã cập nhật module/backend/frontend/root-cause/domain maps và thêm regression coverage. Trên working tree từ commit gốc `df7a75e`: `GOCACHE="$PWD/.gocache" go test ./...` đạt; focused frontend tests đạt 31/31; `npm run build` đạt sau thay đổi source cuối cùng; `git diff --check` đạt.
- Will be done only if separately authorized: Browser/UAT trên trang công trình thật; trạng thái UX vẫn chưa được nghiệm thu.

## [Closed] Project Activity — badge Trao đổi và Thảo luận đếm chưa đọc

- Status: TECHNICALLY VERIFIED, NOT UX ACCEPTED.
- Done: Trước yêu cầu hiện tại, badge ngoài `Trao đổi` đã được thu hẹp từ tổng mọi hoạt động sang `discussion - decisions`. Người dùng đã duyệt review package mới và hai badge ngoài `Trao đổi`/trong `Thảo luận` hiện cùng dùng `activity_summary.discussion_unread` theo actor/công trình; read-through hiện có, badge `Quyết định`, badge `Nhật ký hệ thống`, zero/loading/error và responsive layout không đổi. Backend contract actor-scoped vẫn dựa trên `project_discussion_read_cursors`; không có API/migration mới. Đã cập nhật focused regression cùng module/frontend/root-cause/Project Planning maps. Trên working tree hiện tại, focused frontend test đạt 2/2, backend `TestActivityMentionsUnreadAndRoleScope` đạt, frontend production build đạt và `git diff --check` đạt. Full frontend suite có đúng một lỗi ngoài phạm vi tại `projectEvidenceColumns.test.ts`: test đòi cột `Giai đoạn` riêng trong khi source hiện có `Nguồn / Giai đoạn`; hai file đó không thuộc diff của thay đổi badge.
- Will be done only if separately authorized: Browser/UAT; lỗi full-suite ngoài phạm vi tại `projectEvidenceColumns.test.ts` cần một review package riêng trước khi xử lý.
