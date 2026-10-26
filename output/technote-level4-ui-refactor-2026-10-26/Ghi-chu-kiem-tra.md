# TechNote Level 4 — Kiểm tra refactor giao diện

> **Bản ghi lịch sử trước khắc phục.** Người dùng sau đó đã duyệt package xử lý các phát hiện. Kết quả hiện tại:56 ca kiểm tra lại hợp lệ PASS,0 FAIL,0 BLOCKED; còn giới hạn người dùng đại diện/thiết bị thật và các ca lưu tùy chọn dùng HTTP fixtures. Xem [ghi chú cập nhật](/Users/khiemnguyen/Works/ttcons/ttcons/docs/ui-text-refactor.md), [receipt sau khắc phục](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/ui-text-refactor-2026-10-26/fix-summary.json) và video `sau-khac-phuc/TechNote-Level4-Sau-khac-phuc.mp4`. Nhãn P1 mobile trong bản ghi dưới đây đã được đánh giá lại thành P2 trước khi sửa; không dùng làm trạng thái hiện tại.

**Mục đích:** kiểm tra trang, bảng, dialog, rail và trợ giúp đã sửa trên desktop/mobile; đối chiếu Pipeline, Workflow Form và các bề mặt nhúng với baseline.

**Kết luận:** đã chạy browser local, nhưng **CHƯA NGHIỆM THU TOÀN PHẠM VI / NOT UX ACCEPTED**. Có 128 ca đã đăng ký: **122 PASS, 4 FAIL, 2 BLOCKED**. PASS chỉ có nghĩa đạt các tiêu chí hiển thị/tương tác của ca, không chứng minh luồng ghi dữ liệu hoặc nghiệm thu của người dùng thực.

## Điểm chưa đạt và bị chặn

| Điểm | Bằng chứng đã xác minh | Phân loại / hướng tiếp theo |
|---|---|---|
| Mobile tràn ngang tại Audit | documentWidth820, viewport cấu hình390; baseline cũng820 | P1, lỗi có sẵn. Cần chẩn đoán CSS/containment và review package trước sửa |
| Mobile tràn ngang tại Nhật ký sự kiện | documentWidth814, viewport390; baseline cũng814 | P1, lỗi có sẵn. Không coi thao tác zoom nhỏ tự động là responsive đạt |
| Mobile tràn ngang tại Thảo luận | documentWidth540, viewport390; baseline cũng540. Hai ca: rail và thẻ Pipeline nhúng | P1, hai ca trên cùng bề mặt. Nội dung component Pipeline vẫn khớp; responsive chưa đạt |
| Catalog thông báo HTTP500 | Hai ca tải xong ở desktop/mobile; dialog hiện “Chưa có nhóm thông báo đang hoạt động.” | BLOCKED. Backend image local không đổi; nguyên nhân gốc chưa xác định, chưa quy kết cho refactor frontend |
| Còn mô tả tĩnh | Thanh trên cùng; hai thẻ Profile; đầu hai dialog báo cáo | P2, mục tiêu dọn copy còn thiếu. Đề xuất rút/bỏ phần lặp hoặc chuyển help; AppShell phải áp ngoài vùng loại trừ |

Vị trí copy còn lại:

- [AppShell.tsx:268](/Users/khiemnguyen/Works/ttcons/ttcons/frontend/src/layouts/AppShell.tsx:268).
- [ProfilePage.tsx:323](/Users/khiemnguyen/Works/ttcons/ttcons/frontend/src/modules/profile/ProfilePage.tsx:323) và [ProfilePage.tsx:347](/Users/khiemnguyen/Works/ttcons/ttcons/frontend/src/modules/profile/ProfilePage.tsx:347).
- [ConstructionReportDetailDialog.tsx:241](/Users/khiemnguyen/Works/ttcons/ttcons/frontend/src/modules/operations/ConstructionReportDetailDialog.tsx:241).
- [WorkReportReviewDialog.tsx:280](/Users/khiemnguyen/Works/ttcons/ttcons/frontend/src/modules/work-reports/WorkReportReviewDialog.tsx:280).

## Đã kiểm chứng

- Desktop1440×1000:63 PASS,1 BLOCKED. Mobile390×844:59 PASS,4 FAIL,1 BLOCKED; bộ chính có touch emulation, sáu ca bổ sung dùng resize viewport. Không có JavaScript pageerror trong bộ chính; bộ bổ sung không gắn pageerror listener riêng.
- Mở trang các nhóm nghiệp vụ, bảng và card; mở/đóng dialog Profile, mật khẩu, ảnh đại diện, nhân sự/import, con dấu, import/export công trình, BOQ, nghiệm thu, sửa lỗi và báo cáo. Không bấm lưu/gửi/duyệt/xóa nghiệp vụ hoặc cấp quyền mới.
- ContextHelp tại Profile/Thống kê/Kế toán/Sửa lỗi: nút, Enter/Space, chạm, Escape, click ngoài và focus rời vùng. Escape đóng help, trả focus về nút và giữ dialog cha mở. Giới hạn ảnh và nhãn field vẫn hiện.
- 14 đối chiếu browser cùng nội dung nhìn thấy: Pipeline Builder, Workflow Forms, Pipeline công trình, tab Quy trình, Workflow Form nhúng, thẻ khởi tạo trong MyWork và thẻ quy trình trong Thảo luận; mỗi bề mặt ở hai viewport.
- Source comparison112 file +12 hàm được bảo vệ nguyên văn với baseline. Sai khác screenshot toàn trang ở hover/focus hoặc shell được ghi trong receipt; không tuyên bố mọi pixel giống nhau. Nội dung Form nhúng có crop đối chiếu riêng.
- Giới hạn mobile của Pipeline Builder có ở baseline và được giữ nguyên; việc giữ nguyên không phải nghiệm thu khả năng dùng trình thiết kế trên điện thoại.

## Môi trường và tính hợp lệ

- Baseline `fba39cdcf72a2d6714d5eac3d15eba8cdf4014c9`; bản hiện tại là cùng HEAD với working tree refactor chưa commit. Fingerprint source/tests `8a1cd72fb268ba252380169c795606d969eecdd2a081aaf8d2ded65e64182e87`.
- Chrome156 qua Playwright, Chromium sandbox bật. Frontend hiện tại15173, baseline15174; cả hai build cùng API local18081 và dependencies hiện có. Baseline lấy bằng git archive trong /private/tmp, không tạo Git worktree.
- DB `consflow_guide_20261022`, dữ liệu giả định đã có; image backend `sha256:7e1f8a8df43c3dbfcbfa8ebbc3efa8e5afc1990beffb9867b2e70f557f32694f`. Khởi động lại riêng local services, `AUTO_MIGRATE=false`; DB160 migration trước và sau. Auth/session và event logs có thể ghi theo hoạt động ứng dụng; không tuyên bố database không có bất kỳ ghi nào.
- Admin local dùng các ca phổ thông; Developer local có sẵn quyền dùng Import nhân sự. Không thêm quyền để làm test đạt. Không production, commit, push hoặc deploy.
- Lượt quét đầu reload liên tục gặp auth/SSE rate limit và bị loại. Harness đổi sang điều hướng SPA, kiểm tra đúng route, đóng dialog giữa ca và tôn trọng Retry-After; không tắt rate limit. Các ca chuyển về login không được tính PASS.
- Phép đo mobile đầu dùng innerWidth đã bỏ lọt việc Chrome tự giãn layout viewport. Đã sửa sang viewport cấu hình và chạy lại bốn ca với baseline; phân loại cuối là FAIL. Receipt cuối và video chỉ dùng số đã sửa.

## Chưa kiểm chứng

Không có người dùng đại diện hoặc điện thoại vật lý; chưa thử camera, đo thời gian thao tác, mọi trạng thái validation hay mọi biến thể Form. Không thực hiện luồng lưu/phê duyệt/xóa nghiệp vụ. InitialPasswordChange cần tài khoản must_change_password chưa được kiểm tra trực quan. Recovery Requests đã xem bằng Developer; dialog chi tiết khi có yêu cầu và nguồn RecordSource hợp lệ chưa có ca tương ứng (đã thử trạng thái không khả dụng). NOT_RUN=0 trong JSON chỉ áp dụng các ca đã đăng ký, không có nghĩa toàn bộ ứng dụng đã được kiểm tra.

## Video và bằng chứng

- MP4 `TechNote-Level4-Kiem-tra-giao-dien.mp4`:1280×720,195,5giây (khoảng3phút16giây), hình từ ảnh UAT thật; lời đọc tổng hợp Linh vi_VN, phụ đề burn-in và fileSRT24 đoạn;8 chương. Đã kiểm tra track hình/âm thanh, waveform24 đoạn không im lặng và trích ba khung hình từ MP4 để xem lại.
- `phu-de.srt`, `kich-ban.md`, `chuong.json` và `video-verification.json` cùng thư mục. Video là ghi chú giải thích bằng ảnh, không giả làm bản ghi thao tác liên tục.
- Kết quả từng ca/ảnh/hash: `docs/acceptance-evidence/ui-text-refactor-2026-10-26/browser-summary.json`; raw attempts và review sheets nằm cùng evidence directory.

## Bước tiếp theo — đề xuất, cần duyệt trước sửa

1. Lập review package xử lý ba bề mặt mobile và copy còn lại, giữ nguyên Pipeline/Form cùng component nhúng.
2. Chẩn đoán riêng catalog thông báo HTTP500; sau khi xử lý, chạy lại các ca FAIL/BLOCKED và nghiệm thu bằng người dùng/thiết bị phù hợp.
