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

## [Closed] Icon cho tab Tổng quan công trình

- Status: TECHNICALLY VERIFIED, NOT UX ACCEPTED.
- Done: Người dùng đã duyệt review package và Mockup as Text bằng `OK duyệt triển khai`. Bảy tab của `ProjectDetailPage` nhận icon Lucide 16px: `LayoutDashboard`, `GitBranch`, `ListTodo`, `MessageSquare`, `FileText`, `ScrollText` và `Users`. Icon là decorative (`aria-hidden`), nên nhãn tab vẫn là tên truy cập được; URL, badge, active state, quyền, API, dữ liệu và workflow giữ nguyên. Đã thêm `projectDetailTabs.test.ts`, cập nhật assertion Activity Tabs tương ứng và cập nhật module/frontend mapping. Trên HEAD `102d5a9` cùng working tree thời điểm kiểm tra: focused frontend tests đạt 3/3, `npm run build` và `git diff --check` đạt.
- Will be done only if separately authorized: Browser/UAT, do quy định repository không cho phép kiểm tra trình duyệt nếu chưa được yêu cầu rõ ràng.

## [Closed] Hiển thị người có thể duyệt yêu cầu mua

- Status: TECHNICALLY VERIFIED, NOT UX ACCEPTED.
- Done: Root cause đã được tái hiện: cache `['work-item', taskId]` còn tươi giữ `isFetchedAfterMount=false`, làm `taskUnavailable=true` và ẩn sai action dù source API trả `capabilities.approve=true`. Dữ liệu local xác nhận hồ sơ hiện không có người duyệt được chỉ định; Admin, PM và Mua hàng đều có quyền `procurement.approve`. Backend `GET /procurement-requests/:id` đã trả `approval_context` cho trạng thái chờ duyệt; helper chỉ lấy tài khoản App còn hoạt động, loại external và recheck từng ứng viên bằng `CanAccessProjectPermission`. Dialog hiển thị người có thể duyệt, tên/vai trò, quyền của người đang xem, trạng thái không có ứng viên và structured-approval blocker. Task context dùng `staleTime: 0`, `refetchOnMount: 'always'` và loading/ready/error rõ ràng nên warm cache không còn khóa sai CTA. Đã tái sử dụng `Dialog`, `RecordSection`, `IdentityReferenceLabel`, `ProcurementStatus` và `ToastProvider`; không có migration, thay đổi role grant, trạng thái mua sắm, stock, acceptance, payable hoặc finance. Mapping trung tâm/backend/frontend/root-cause/Commercial Procurement/Admin Governance đã được cập nhật. Trên HEAD `102d5a9` cùng working tree thời điểm kiểm tra: `gofmt`, `git diff --check`, backend focused tests, `go test ./...`, frontend focused tests và `npm run build` đạt.
- Known limitation: Full frontend suite còn một lỗi ngoài phạm vi tại `projectEvidenceColumns.test.ts`, test kỳ vọng `Giai đoạn` trong khi source không đổi bởi plan này đang hiển thị `Nguồn / Giai đoạn`.
- Will be done only if separately authorized: Browser/UAT.

## [Closed] Fix vòng lặp điều kiện hoàn tất yêu cầu mua

- Status: TECHNICALLY VERIFIED, NOT UX ACCEPTED.
- Done: Đã tái hiện bằng dữ liệu local: Work Item `e942ea39-b4fc-4964-afb1-74d158d39359` bị `procurement.require_approved` chặn bởi `UAT-DISC-MS-02` ở `waiting_approval`. Hard gate theo phạm vi công trình được giữ nguyên; backend trả danh sách blocker, blocker đầu tiên, lý do cụ thể và deep link chuẩn tới yêu cầu mua. Riêng `procurement.require_approved` được hỗ trợ business source trên runtime chính thức engine 2 cũ, không nới điều kiện revision cho action khác. Khi `Hoàn tất` thất bại, Work Item tải lại chi tiết, hiển thị `details.reason`, đóng dialog và mở đúng yêu cầu mua qua `ProcurementPage` + `ProcurementDialog` chuẩn. Regression tests và mapping trung tâm/backend/frontend/root-cause/domain đã được cập nhật. Trên HEAD `102d5a9` cùng working tree thời điểm kiểm tra: `gofmt`, `git diff --check`, backend focused integration tests, `go test -p 1 ./...`, frontend focused tests và `npm run build` đạt.
- Known limitation: Full frontend suite còn một lỗi ngoài phạm vi tại `projectEvidenceColumns.test.ts`, test kỳ vọng `Giai đoạn` trong khi source không đổi bởi plan này đang hiển thị `Nguồn / Giai đoạn`.
- Will be done only if separately authorized: Browser/UAT từ Work Item mở đúng yêu cầu mua đang chặn, duyệt lần lượt blocker, rồi hoàn tất Work Item.
