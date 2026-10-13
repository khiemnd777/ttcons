# Nghiệm thu development — các nút xuất BM-GS

Phiên kiểm tra bắt đầu 09-10-2026, kết thúc 10-10-2026 (Asia/Ho_Chi_Minh).

**Kết luận:** development tiêu chuẩn đã cập nhật. Đã bấm các nút ở hồ sơ hiện có và tải thành công 18 XLSX, bao phủ BM-GS-01 đến BM-GS-08. Đã mở 10 workbook từ các file tải thực tế trong Google Sheets. Đây là kiểm chứng có giới hạn trên dữ liệu synthetic; anh chưa duyệt hiển thị, và chưa chứng nhận toàn bộ nghiệp vụ/client.

## Còn thiếu / giới hạn

1. **Chi tiết lần gửi BM02: NOT_RUN.** Development hiện có hai báo cáo nháp tại nguồn; chưa có lần gửi và người nhận hợp lệ. Không tạo lần gửi giả hoặc gửi cho người khác để vượt điều kiện này.
2. **Nguồn nhập vật tư BM02: chưa được chứng minh trên development.** Fixture liên kết phiếu nhập với Đầu việc báo cáo ngày bị API từ chối do sai loại chủ sở hữu. Không suy diễn vật tư đã sử dụng thành vật tư nhập trường. Regression trên PostgreSQL UAT ở lượt trước là bằng chứng riêng.
3. **Thời điểm chụp BM07 bằng camera thật: NOT_RUN.** Ảnh thử hiện nhập từ file và không có `captured_at`; ô thời điểm chụp giữ trống. Không coi giờ upload là giờ chụp.
4. **Excel desktop, in giấy, tải cực đại, toàn bộ quy trình và dữ liệu client: NOT_RUN.** BM03/BM08 được xuất từ hồ sơ thử; chưa chứng minh ký/chốt nghiệm thu và hoàn tất bàn giao xuyên suốt.
5. Fixture báo cáo ngày đầu tiên có PNG sai checksum. Xuất ảnh từ revision1 bị từ chối với `decode image: png: invalid format: invalid checksum`. Đã bổ sung ảnh hợp lệ qua API chuẩn và sửa nguồn theo amendment revision2; dữ liệu/lịch sử cũ giữ nguyên. Revision1 và xuất toàn bộ minh chứng của chính Đầu việc chứa PNG hỏng không thuộc kết quả thành công dưới đây.
6. Fixture đầu BM01/02/05 có loại Giai đoạn không khớp guard Thi công, nên Operations ở chế độ theo dõi. Việc tạo/sửa biểu mẫu qua API chuẩn vẫn hoạt động; chỉ kiểm chứng đọc/xuất các báo cáo đã có tại Operations. Fixture riêng BM04/07 dùng đúng Giai đoạn Thi công. Không sửa guard hoặc dữ liệu chính thức để chạy thử.

## Nút đã kiểm tra

| BM | Vị trí | Nhãn thực tế | Kết quả |
|---|---|---|---|
| 01 | Đầu việc → phiếu khảo sát | Xuất báo cáo | Tải được BM01 |
| 02 | Thi công → Nhật ký | Xuất báo cáo | Workbook có hai ngày 08/09; lọc riêng ngày08 chỉ còn ngày08 |
| 02 | Chi tiết báo cáo ngày | Xuất báo cáo ngày | Tải được BM02 |
| 02 | Báo cáo → Nháp tại nguồn | Xuất báo cáo | Tải được workbook hai ngày |
| 02 | Báo cáo → chi tiết lần gửi | Xuất báo cáo | NOT_RUN — chưa có lần gửi hợp lệ |
| 02 | Đầu việc → phiếu báo cáo ngày | Xuất báo cáo ngày | Tải được BM02 revision2 |
| 03 | Đầu việc → checklist và gói nghiệm thu thầu phụ | Xuất báo cáo | Hai hồ sơ thử, tải được BM03 |
| 04 | Thi công → Cần xử lý | Xuất báo cáo | Tải được BM04 danh sách |
| 04 | Chi tiết việc Cần xử lý | Xuất báo cáo | Tải được BM04 một việc |
| 05 | Đầu việc → phiếu rủi ro | Xuất báo cáo | Tải được BM05; đang sửa chưa lưu thì nút bị vô hiệu |
| 05 | Phiếu khảo sát; phiếu báo cáo ngày; chi tiết ngày | Xuất báo cáo rủi ro | Cả ba vị trí tải được BM05 |
| 06 | Phiếu báo cáo ngày; chi tiết ngày | Xuất báo cáo hiện trường | Cả hai vị trí tải được BM06 |
| 07 | Đầu việc → Sổ minh chứng | Xuất báo cáo | Tải được BM07 có ảnh nguồn |
| 08 | Bàn giao → hồ sơ đang xem | Xuất báo cáo | Tải được BM08 |

Danh sách đường dẫn file tải, URL nguồn và nhãn: [browser-downloads.json](acceptance-evidence/bm-gs-reuse-development-2026-10-09/browser-downloads.json). Các XLSX nguyên vẹn nằm trong [thư mục exports](acceptance-evidence/bm-gs-reuse-development-2026-10-09/exports). [xlsx-inspection.json](acceptance-evidence/bm-gs-reuse-development-2026-10-09/xlsx-inspection.json) ghi SHA256, ZIP/XML, ô nguồn và 32 kiểm tra đạt; 18 bản sao có cùng SHA256 với file tải gốc. Không sửa workbook sau tải để làm nghiệm thu.

## Kết quả Google Sheets và tái sử dụng nguồn

- Đọc các vùng `A1:K80` của mọi sheet trong 10 workbook nhập thực tế: không có `effectiveValue.errorValue`, bao gồm `#VALUE!`. Dữ liệu đọc và spreadsheet IDs: [google-readback-final.json](acceptance-evidence/bm-gs-reuse-development-2026-10-09/google-readback-final.json).
- **BM07:** ảnh hiển thị trong F8, giữ tỷ lệ, vừa hàng/cột. Menu của F8 có `Put image over cells`, xác nhận Google Sheets nhận đây là ảnh trong ô. Không chọn lệnh chuyển ảnh; workbook không bị chỉnh nội dung. [Mở workbook BM07](https://docs.google.com/spreadsheets/d/1c9TtjL_YAJSUSCWAjgRAnBLzACB4Bgx0ZyAN1oSJFAs/edit?gid=707024086).
- **BM04:** C8 lấy đúng `[Synthetic] Khu B — ô kiểm tra 04` từ phiếu kiểm tra liên kết dù việc xử lý không có vị trí riêng. B8 mã lỗi trống; A13 tính thực tế trên Google Sheets: tổng1 / đã khắc phục0 / đang xử lý1.
- **BM03:** G9 vẫn là `Không đạt`; Comment có sẵn xuất ở A16 dưới nhãn ghi chú nguồn. Trạng thái checklist hoàn tất không biến thành chất lượng đạt.
- **BM08:** D14 vẫn `Không đạt`, F14 ngày11-10-2026, kết luận vẫn chưa đủ điều kiện. Ghi chú có sẵn xuất riêng A27; các mục hồ sơ chưa đánh giá giữ trống.
- **BM06:** I8 là số0; J8 `Đang làm` lấy từ đúng Đầu việc phát sinh liên kết và được chốt trong revision2. G8 ngày giao thiếu thì giữ trống.
- **BM01/BM05:** không nhúng ảnh vào mẫu không yêu cầu ảnh. **BM02:** các tiêu chí chưa được đánh giá tại nguồn giữ trống; file xuất không tự đánh giá để lấp ô.
- Phụ lục ảnh BM02/BM04/BM06 đã mở và thấy ảnh hiển thị. BM02 workbook hai ngày giữ hai sheet báo cáo và một phụ lục ảnh; hai ngày dùng cùng ảnh thử được khử trùng media nhưng giữ hai neo ảnh trong workbook gốc.

## Ảnh chụp để review

Ảnh thật từ development5173 và Google Sheets; desktop1280×720, màn hình nhỏ390×844; Sheets100%. Không phải mockup.

### Phiếu báo cáo ngày — ba nút

![Ba nút xuất desktop](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm02-desktop-final.jpg)

![Ba nút xuất màn hình nhỏ](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm02-mobile-buttons.jpg)

### BM01

![Nút BM01](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm01-source.jpg)

![BM01 trên Google Sheets](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm01-google.jpg)

### BM02 — Nhật ký, danh sách và chi tiết

![Nhật ký nhiều ngày](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm02-journal.jpg)

![Nhật ký lọc ngày08](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm02-journal-filter08.jpg)

![Chi tiết báo cáo ngày](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm02-detail.jpg)

![Danh sách Báo cáo](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm02-reports-list.jpg)

![BM02 workbook hai ngày](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm02-two-days-google.jpg)

![BM02 phần nguồn đã có](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm02-two-days-source-google.jpg)

![BM02 phụ lục ảnh](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm02-images-google.jpg)

### BM03

![Phiếu nghiệm thu](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm03-source.jpg)

![BM03 giữ ghi chú độc lập](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm03-google-100.jpg)

![BM03 gói thầu phụ](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm03-client-google.jpg)

### BM04

![Danh sách cần xử lý](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm04-list.jpg)

![Chi tiết cần xử lý](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm04-detail.jpg)

![BM04 vị trí và tổng hợp](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm04-google.jpg)

### BM05

![Phiếu rủi ro](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm05-source.jpg)

![BM05 trên Google Sheets](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm05-google.jpg)

![Đang sửa chưa lưu không xuất](acceptance-evidence/bm-gs-reuse-development-2026-10-09/unsaved-export-disabled.jpg)

### BM06

![BM06 trạng thái và tiến độ nguồn](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm06-google.jpg)

### BM07

![Sổ minh chứng có nút xuất](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm07-source.jpg)

![Ảnh trong ô F8](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm07-google.jpg)

![Menu xác nhận ảnh trong ô](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm07-image-cell-menu.jpg)

### BM08

![Hồ sơ bàn giao](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm08-source.jpg)

![BM08 giữ ghi chú và đánh giá](acceptance-evidence/bm-gs-reuse-development-2026-10-09/bm08-google.jpg)

## Phiên bản và môi trường đã kiểm tra

- `main`, HEAD `75dc44707785d399ed2408959bf784928a544e96`, giữ toàn bộ thay đổi chưa commit. Manifest959 file nguồn/config/test SHA256 `f658859b6fb567530ca64c25c0c56df76542898ef7313848cf7d9513cf89e97e`; đọc lại cuối phiên không có file nguồn thay đổi.
- `make restart` chạy đúng down→build→run, exit0; containers tạo09-10-2026 lúc23:08:21+07. Frontend5173 / API8080, PostgreSQL16 database`consflow`; 157 migrations, max numericV157. Backend image`sha256:e418941bda7f6221fd2d2060eb87cd4a51307a0c8a6da1c82ac7d91bf3ce0b1a`. Frontend development chạy Vite từ source bind-mounted sau rebuild. Không có migration mới trong sửa tái sử dụng nguồn.
- Kiểm tra cuối phiên `/api/health`200/ok, `/api/ready`200/databaseok; PostgreSQL/Redis khỏe. Các môi trường UAT khác giữ nguyên.
- Trước restart và sau restart trước fixture:3project/17form; sau fixture API chuẩn:5project/20form. Không resetDB. Backup1,120,940byte tại`/private/tmp/consflow-before-reuse-development-20261009.dump`; chưa thử restore nên không chứng nhận khả năng phục hồi.
- Lệnh thực tế: `docker compose --env-file .env config --quiet`, `make restart`, `docker compose --env-file .env ps --format json`, health/ready, truy vấn số migration/dữ liệu và so sánh source manifest. Nghiệm thu gồm 18 lần tải từ nút UI, 32 kiểm tra ZIP/XML, đọc ô Google Sheets và ảnh chụp thực tế. Evidence máy đọc: [runtime-evidence.json](acceptance-evidence/bm-gs-reuse-development-2026-10-09/runtime-evidence.json).
- Bộ Go 84.441s, frontend 34 tests/build và vet/build đã đạt ở lượt implementation trước trên cùng source; không chạy lại các test đó trong lượt cập nhật/nghiệm thu này. Các test DB trước chạy trên UAT schema cô lập; không thay thế kiểm chứng development ở đây.
- Phiên này chỉ cập nhật runtime, tạo dữ liệu thử và bằng chứng/tài liệu. Không thay đổi hành vi ứng dụng, nên không có mapping chức năng mới cần cập nhật. Không deploy production, commit hay push.
- GPT-6.1 Sol High đã đối chiếu độc lập báo cáo, 18 file/SHA256, 10 workbook/ô dữ liệu và ảnh BM07/menu ảnh trong ô. Không phát hiện P0–P2 trong phạm vi bằng chứng; giữ nguyên các mục NOT_RUN và yêu cầu anh review UX. Reviewer không thực hiện browser/DB trong lượt audit này.

**Bước tiếp theo:** anh review ảnh chụp. Để chốt nghiệm thu đầy đủ còn cần nguồn nhập vật tư đúng chủ sở hữu, người nhận/lần gửi báo cáo hợp lệ và camera thật; các mục này chưa được tính đạt.
