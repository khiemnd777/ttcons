# Current States

## [Active] Khôi phục full frontend suite sau shared table viewport

- Mục tiêu: đưa `npm run test` về xanh sau khi shared `consflow-table-viewport` thay thế utility overflow/sticky cục bộ; giữ nguyên giao diện, RBAC, workflow và API.
- Đã làm: tái lập bốn lỗi độc lập; thêm CSS text loader cho Activity Topic in-memory bundle và cập nhật ba assertion theo shared viewport từ commit `e80f352`. Test hẹp đạt 16/16, full `npm run test` đạt 425/425 và `npm run build` đạt trên working tree này.
- Sẽ làm: không còn việc thay đổi mã trong plan; browser không chạy vì không thay đổi UI.
- Phạm vi: `frontend/tests` và shared plan state; không đổi frontend source runtime hoặc mapping.
