# [Closed] Khôi phục full frontend suite sau shared table viewport — 2026-09-08

## [Closed] Khôi phục full frontend suite sau shared table viewport

- Mục tiêu: đưa `npm run test` về xanh sau khi shared `consflow-table-viewport` thay thế utility overflow/sticky cục bộ; giữ nguyên giao diện, RBAC, workflow và API.
- Đã làm: tái lập bốn lỗi độc lập; thêm CSS text loader cho Activity Topic in-memory bundle và cập nhật ba assertion theo shared viewport từ commit `e80f352`.
- Trạng thái đóng: TECHNICALLY VERIFIED — test hẹp đạt 16/16, full `npm run test` đạt 425/425 và `npm run build` đạt trên revision phát hành `7f98012`. NOT UX ACCEPTED vì không có thay đổi UI và browser/UAT không chạy.
- Phạm vi: `frontend/tests` và shared plan state; không đổi frontend source runtime hoặc mapping.
