# UAT sau sửa hiệu năng — 2026-10-02

**Mục đích chính:** Phó Quản Lý mở bộ lọc công trình/Giám sát nhanh, tìm đúng công việc và báo cáo toàn công ty, giữ đúng quyền quan sát và xác nhận của từng người.

**Kết quả: PASS trong phạm vi kiểm tra bản sửa hiệu năng và các regression dưới đây.** Chưa phải nghiệm thu của client hoặc toàn bộ hệ thống; thiết bị thật, PDF và UI tạo/sửa nguồn chưa kiểm tra lại. Tính năng tổng thể vẫn NOT UX ACCEPTED theo các giới hạn đã ghi.

## Giới hạn và quan sát cần giữ lại

- Sau các lượt tải lại trang liên tục, UI có lúc báo “Mất kết nối · Làm mới thủ công”; log backend ghi HTTP429 ở `/api/me/activity-attention/stream`. Giao diện tự trở lại “Đang trực tuyến”, không restart hoặc đổi cấu hình. Một probe trực tiếp nhận HTTP200; đây chỉ là kiểm tra handshake, không chứng minh stream ổn định lâu dài.
- Chưa xác minh riêng vòng đời/release connection sau reload hoặc độ ổn định realtime lâu dài. Guard giới hạn kết nối trong `activity_stream.go::acquireActivityStream` và retry/backoff trong `client.ts::streamSSE` là các owner cần kiểm tra nếu mở scope này. Không kết luận đây là regression do tối ưu options, cũng không coi handshake200 là đã sửa vấn đề realtime.
- Lượt này dùng báo cáo/receipt fixture đã có để kiểm tra hiển thị, lịch sử, protected viewer và scope; không tạo revision mới hoặc lặp lại chu trình yêu cầu bổ sung/submit/ack. Chu trình mutation đã được kiểm tra ở UAT trước; không trình bày kết quả trước đó thành kết quả mới.
- Responsive dùng viewport390px, không phải điện thoại thật; PDF/download, camera, bàn phím mềm và UI tạo/sửa báo cáo không thuộc lần retest này. Không triển khai production hoặc chuyển tài khoản Cường/Nguyệt.

## Thời gian, source và runtime

- Khoảng **23:26–23:36 ICT** ngày 2026-10-02. User yêu cầu rõ “UAT check”, cho phép browser trong lượt này. Browser Codex IAB; desktop1440×900, mobile390×844.
- Working tree chưa commit trên `main`, HEAD `465eb76`. `supervision.go` SHA256 vẫn `3d12367ab4afacbcb7969402d55f5d56b913541fd4bbd68d01f8fc0a2fe68ed9`, khớp bản đã tối ưu. Không sửa mã ứng dụng trong UAT.
- Stack local đã được rebuild trước lần kiểm tra này: backend image **`sha256:982a193c534a6eabb81e9fe3c70796d3d9e170223572016cf7fc091714d5ae60`**, bắt đầu23:25:14 ICT. Đây là image hiện đang chạy, khác image trong checkpoint sửa hiệu năng trước đó. Frontend là Vite dev/source bind tại5173; backend8080, main PostgreSQL/Redis chạy healthy, main DB V148. Không reset DB hoặc seed.
- Tái sử dụng đúng ba tài khoản fixture `uat-dm-1002-6ff17d` và công trình/báo cáo cũ; chỉ bật lại trong thời gian test. Phó Quản Lý vẫn có **0 project membership**. Không đọc/sửa credentials tài khoản thật.
- Mapping đã đối chiếu: central/backend/frontend/triage, Admin Governance/Construction Operations/Reporting. Owner chính `SupervisionOptions`, `supervisionSupervisorOptionsSQL`, `SupervisionWorkspace`, shared WorkItem/WorkReport dialogs; đọc consumer/options query trước thao tác. Chỉ cập nhật bằng chứng kiểm chứng, không đổi behavior/API nên không cần sửa mapping trong lượt này.

## Kết quả đã quan sát

| Tình huống | Kết quả |
| --- | --- |
| Mở tab Công việc Giám sát lần đầu | Từ click tới option UAT Giám sát có trong DOM: **140ms**; không còn loading sau khi sẵn sàng. |
| Reload trang với cache query mới | Hai lần từ reload tới option sẵn sàng: **333ms / 234ms**. Bao gồm tải trang/auth/React và overhead automation, không phải timing mạng thuần. |
| Options qua API thật | **8,42 / 6,41 / 7,21ms**; 840 công trình và 5 Giám sát active. |
| Data trước/sau | Công trình và thứ tự giống checkpoint sau sửa; Giám sát giống sau khi loại đúng tài khoản Giám sát UAT vừa bật lại. Chênh lệch này do trạng thái fixture, không phải scope mở rộng. |
| Option sample | Trong các option có tag UAT chỉ có A/B/C, không có SAMPLE. |
| Lọc công trình mới C + Giám sát | Đúng một công việc C mới, Sẵn sàng; không membership cho Deputy. |
| Phân trang tìm tag UAT | Trang đầu25 dòng; Xem thêm đủ29 dòng; hết CTA Xem thêm. |
| Lọc B + Giám sát | Đúng một công việc B, Sẵn sàng. |
| Shared WorkItem detail | Banner “Theo dõi công việc · Chỉ xem”; không có thao tác bắt đầu/hoàn tất/duyệt; minh chứng read-only. |
| Report filter | Đã xem có A/B; Chưa xem ra empty state đúng trên mobile; không loading/error kéo dài. |
| A lần2 | Nội dung55%, 1 ảnh, hai receipt riêng của Deputy/PM đúng actor `(tôi)` khi đổi tài khoản. |
| A lần1 | Nội dung40%, 0 ảnh, cảnh báo snapshot lịch sử, không có action review, có Về bản hiện tại. |
| Protected image | Viewer chuẩn mở ảnh xanh test của lần2, không lỗi tải ảnh; quan sát bằng screenshot. |
| Responsive390px | Page client/scroll390/390; dialog356/356, không tràn ngang. |
| PM UI | Hiển thị PM, không có hai tab company supervision; trang Báo cáo có đúng hai báo cáo được phép và receipt PM `(tôi)`. |
| Negative API | PM/Supervisor gọi options403; Deputy đọc sample403, finance403, bắt đầu công việc B403. |
| Console | Snapshot warning/error cuối lượt không có bản ghi; điều này không phủ định HTTP429 đã thấy trong backend log. |

## Cleanup và bằng chứng

- Logout, reset viewport, đóng tab tạm. Ba user UAT trở lại inactive; toàn bộ13 session hiện có revoked; DB xác nhận zero unrevoked session. Token API của cả ba role trả401, token file tạm đã gỡ. Không xóa lịch sử/receipt fixture.
- `git diff --check`: PASS sau cập nhật tài liệu. Không chạy lại build/unit tests vì mã ứng dụng không thay đổi; lượt này bổ sung bằng chứng browser/runtime cho source đã có technical checkpoint.
- Ảnh/JSON đã lưu tại `/Users/khiemnguyen/.codex/visualizations/2026/10/02/01a0fc1e-bd41-7350-97cc-6a24777bf4bc/consflow-deputy-uat-after-fix/`: `filters-ready-desktop.jpg`, `work-readonly.jpg`, `report-receipts-desktop.jpg`, `protected-image.jpg`, `report-mobile-390.jpg`, `filters-empty-mobile.jpg`, `pm-report-receipts.jpg`, `browser-metrics.json`, `api-checks.json`.
- UAT-DM-01 có browser retest PASS trong phạm vi hiệu năng. Plan được đóng theo yêu cầu của người dùng ngày 2026-10-02; xem [bản lưu plan](plan-closed/2026-10-02__closed-deputy-manager-supervision.md). Các giới hạn nghiệm thu nêu trên không được đánh dấu hoàn tất.
