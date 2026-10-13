# BM06/BM07 — Kiểm chứng Development ngày 13-10-2026

**Trạng thái:** UX ACCEPTED trong phạm vi bên dưới. Anh xác nhận `Chốt nghiệm thu BM06/BM07 trong phạm vi đã kiểm` ngày13-10-2026. Các giới hạn NOT_RUN giữ nguyên; không mở rộng nghiệm thu sang camera thực tế, mọi nguồn lịch sử hoặc toàn bộ quy trình. BM05 giữ nguyên phạm vi nghiệm thu trước đó.

**Mục đích chính:** xuất dữ liệu từ hồ sơ thi công và minh chứng đã có vào đúng biểu mẫu client; tên dự án dễ đọc và ảnh BM07 vừa ô. Người dùng tiếp tục dùng các nút hiện có.

## Thay đổi đã duyệt

- BM06 gộp vùng tên dự án B5:J5; BM07 gộp B5:G5 trước hàm căn chiều cao dòng hiện có. Chiều cao dòng 5 của cả ba file thực tế là 33 pt. BM05 B5:K5 giữ nguyên.
- Chỉ thêm tám dòng renderer trong `backend/internal/core/bm_gs_workbook.go`, mở rộng hồi quy trong `bm_gs_risk_header_test.go`. Không đổi hàm fit chung, độ rộng bảng, mapping dữ liệu, frontend, API, schema, field hoặc lịch sử.
- Cập nhật central/backend/triage/Construction Operations maps. Download vẫn qua `NativeExcelExportButton` trong `WorkflowFormPanel`, `ConstructionReportDetailDialog`, `EvidencePanel`; xem minh chứng vẫn thuộc shared `EvidenceRecordViewerDialog`. Không tạo UI/module/chọn mẫu/chọn ảnh mới.

## Nguồn và file thực tế

| File giữ nguyên byte tải | Nút và nguồn | Kết quả |
|---|---|---|
| [BM06-form.xlsx](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm06-bm07-development-2026-10-13/BM06-form.xlsx) | `Xuất báo cáo hiện trường` tại Đầu việc 4717be95-6d19-4c5e-972a-32fa825a0d8d; phiếu 35416810-63ab-4175-8340-5be6e13bb51b | 14.248 byte; native daily v5 đã chốt, R2; đủ 10 cột gốc |
| [BM06-daily.xlsx](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm06-bm07-development-2026-10-13/BM06-daily.xlsx) | Cùng nút tại chi tiết Nhật ký bd131f5c-ccf0-4c3f-b862-7d2dfc551eb4 | 14.219 byte; đúng cùng báo cáo ngày 09-10-2026 |
| [BM07.xlsx](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm06-bm07-development-2026-10-13/BM07.xlsx) | `Xuất báo cáo` trong Sổ minh chứng của Đầu việc 2c969a28-945f-4abe-bf9a-8d44c722699d | 38.562 byte; một tab BM-GS-07; đủ ba ảnh nguồn trong F8–F10 |

Tất cả là nguồn synthetic Development. BM06 công trình DEV-BM-REUSE-20261009 vẫn ở luồng thử cũ, không phải bằng chứng đi xuyên toàn bộ Rev10. BM07 công trình DEV-BM04-REUSE-20261009, ba ảnh đã gửi và chưa được duyệt chất lượng.

### BM06

- Hàng 8 giữ đúng mã SYN-GV-06, công việc, vị trí, người nhận/liên hệ, hạn 2026-10-10 và tiến độ công việc ghi nhận `0`. Không lấy tiến độ tổng báo cáo để thay cho tiến độ từng công việc.
- Thầu phụ E8 và ngày giao G8 trống vì nguồn không ghi hai dữ kiện này. Không suy từ owner role PM hoặc ngày báo cáo.
- R2 trong `workflow_form_events.metadata_json.native_daily_sources` lưu “Đang làm” với đúng generated-task a3fc7b9e-feb1-4b0a-850e-9dc40194e8ef và đúng row_id 8e48a7ac-27d4-428f-8589-2098b5da1be2. Task hiện tại cũng đang in_progress. Hai bản J8 cùng “Đang làm” trong fixture này; không suy rằng lịch sử và trạng thái hiện tại luôn giống nhau.
- 10 header gốc và ba vị trí ký giữ nguyên. Ảnh đã liên kết 864d4247-3f3e-48be-97b7-5dafd16b6e9d được giữ trong tab phụ `Hình ảnh`; BM06 gốc không có cột ảnh. Hai tab phụ ghi đúng nguồn riêng của phiếu và báo cáo ngày.
- Hồi quy reopened BM06 phân biệt thiếu/blank/invalid progress với zero. Browser/Google lượt này kiểm nguồn một dòng có zero; chưa chạy thêm nguồn nhiều dòng với missing progress.

### BM07

- Xuất tất cả ba ảnh hợp lệ đang gắn đúng Đầu việc, theo created_at/id. Không upload, chọn lại ảnh hoặc sửa metadata khi export.
- F8 = Gradient 8057abdc-e188-4605-b996-77f339d961a2; F9 = Lần 1 dd9e5c97-a54d-43ea-8ea4-e380788b6a1b; F10 = Lần 2 9fdb5b88-b61e-42a0-bac2-3f84946a1930.
- Drawing oneCellAnchor, zero offsets, exact source identity và media relationship được kiểm. Extents nằm trong chiều rộng F và chiều cao từng hàng. Ảnh F8 xuất 1280×822 từ 1400×900; hai ảnh còn lại 900×560 và pixel RGB giữ nguyên. Cả ba giữ tỷ lệ trong sai số làm tròn, không che ô bên cạnh.
- Google Sheets mở đúng file chưa chỉnh sửa, tại 100%. Cả F8/F9/F10 có menu `Image → Put image over cells`, xác nhận ảnh hiện tại nằm trong ô. Chỉ mở/đóng menu, không chọn thao tác đổi ảnh.
- Ba ảnh nhập từ file không có captured_at; C8–C10 giữ trống. Không dùng thời gian upload, mtime hoặc ngày ghi trên nội dung ảnh làm thời điểm chụp.
- Workbook 37,7 KiB; derivative ảnh 5.244/16.017/15.833 byte, cạnh dài tối đa 1280 px. Chưa kiểm ảnh công trường thật hoặc tải tối đa. Không tuyên bố mọi PNG đều giảm số byte: hai PNG nhỏ này được re-encode lớn hơn một chút, pixel giữ nguyên.
- XLSX lưu drawing anchors; kiểm chứng này xác nhận Google native cell images, không xác nhận Microsoft Excel Place in Cell hoặc in ấn. Không có tab `Hình ảnh` riêng cho BM07.

## Google Sheets và ảnh chụp

Cả ba workbook được nhập nguyên file tải, tạo native Google Sheets mới, private chỉ owner. Không chỉnh cell, style, chia sẻ hoặc source file sau tải.

- [BM06 từ phiếu](https://docs.google.com/spreadsheets/d/100AyGBpby2WmD7GL3W7QF33heHGh9I1UhJBESz_jpWA/edit#gid=599767050): main A1:J15 + ảnh A1:H25, 350 cell so sánh.
- [BM06 từ Nhật ký](https://docs.google.com/spreadsheets/d/10LVtY38c6GjaDcD5TDWVtTZnRs8dO0PE-BQdlNXNDWA/edit#gid=66816009): cùng phạm vi, 350 cell so sánh.
- [BM07](https://docs.google.com/spreadsheets/d/1aDiTayUB-5mA-yiFRsdZgcdfckP_Ctt3oh73PFrdkE0/edit#gid=662639595): A1:G16; 109 cell thường so sánh; ba ô ảnh kiểm riêng bằng XML và UI.

Tổng 809 cell thường khớp workbook, không có effectiveValue.errorValue hoặc #VALUE. Header selections B5:J5/B5:G5 và toàn bộ vùng có dữ liệu/chữ ký đã chụp ở Sheets100%.

![Nút BM06 ở phiếu](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm06-bm07-development-2026-10-13/bm06-form-button.jpg)

![Nút BM06 ở chi tiết Nhật ký](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm06-bm07-development-2026-10-13/bm06-daily-button.jpg)

![BM06 Google Sheets](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm06-bm07-development-2026-10-13/bm06-form-google-main.jpg)

![Nút BM07 trong Sổ minh chứng](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm06-bm07-development-2026-10-13/bm07-source-button.jpg)

![BM07 ảnh trong ô](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm06-bm07-development-2026-10-13/bm07-google-main.jpg)

Các ảnh tab phụ BM06 và menu từng ảnh BM07 nằm trong cùng thư mục evidence.

## Build, checks và bảo toàn

- Source: HEAD 75dc44707785d399ed2408959bf784928a544e96 + dirty working tree đã có; 979 file nguồn/test/migration hash. Chỉ hai file renderer/test trên thay đổi so với baseline BM05. Manifest hiện tại khớp build.
- Development localhost5173 / API8081 / DBconsflow / V158. Dùng đúng Compose env-file đang chạy, không đổi config. Backend image sha256:09c379a9cb12d6e6d945221fa47b9b1a5389607a3a752d8ce5be844d4e1876ea tạo 12:32:31 ICT, start12:32:36. Bốn container khác giữ nguyên ID/image/start/ports. Frontend nguồn không đổi, không build lại.
- RED tái hiện thiếu merge và dòng tên dài 347,55 pt BM06 /184,2 pt BM07. GREEN: tên ngắn/thiếu33 pt, tên dài frozen35,7 pt; giữ source JSON, data geometry và BM07 drawing/media bytes. BM05 và năm template khác giữ geometry đã duyệt.
- `go test ./internal/core -run 'TestBMGS|TestNativeBMReuse' -count=1`: pure workbook/projection regression PASS2,012s với temp Go caches; scoped gofmt clean. DB-backed checks và bộ tạo QA workbook SKIPPED vì CONSFLOW_TEST_DATABASE_URL/BMGS_QA_DIR không cấu hình; worker xác nhận. Sol High source review độc lập PASS0,691s, không có P0–P2 mới trong delta.
- `docker compose --env-file /private/tmp/consflow-bm02-development-20261012.env build backend`: PASS; scoped escalation cho Buildx. `up -d --no-deps backend`: PASS. `/api/ready` và frontendHTTP200. Lần probe sai `/health/ready` trả404 đã được sửa theo route gốc; không phải lỗi readiness của ứng dụng.
- `verify.py`: PASS55/55. 2.894 dòng cũ từ baseline business/construction/history/audit giữ nguyên; không mất/sửa dòng. 149 file storage nguyên checksum; form pin/answers/status/R2, events và ba evidence nguyên vẹn; V158 giữ nguyên.
- Chỉ thêm bốn `native_source_exported` audit: BM06 phiếu, BM06 Nhật ký và hai lần BM07. BM07 bấm lại một lần để giữ file bằng download event. `sources.revision_no=0` là reference current-source của các nút này; R2 phiếu được chứng minh riêng từ instance/events, không gọi field audit đó là R2.
- Sol High final evidence audit độc lập import `verify()` read-only: PASS55/55; xem ba main Google, ba menu F8/F9/F10, ba ảnh nút browser và hai tab ảnh BM06; xác nhận audit/container/source/history, không có P0–P2 mới trong phạm vi duyệt. [final-review.md](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm06-bm07-development-2026-10-13/final-review.md). Reviewer không chạy lại browser, Google hoặc runtime.

## Phần chưa kiểm và bước tiếp

**NOT_RUN:** camera vật lý/thời điểm shutter mới, ảnh công trường thật, portrait/square/tall trong lượt này, tải tối đa, tài khoản quyền khác, mobile, Excel desktop và print. Lịch sử không thay đổi theo baseline; không thao tác gửi lại hoặc sửa phiên bản nguồn để kiểm trong lượt này.

**Bước tiếp:** BM06/BM07 đã được anh chốt. Anh xác nhận ba điểm bổ sung không cần thiết: review PDF tiến độ BM02, camera thực tế BM07 và kiểm thử mở rộng là NOT_REQUIRED cho đợt này. Không còn yêu cầu nghiệm thu bổ sung; giới hạn bằng chứng giữ nguyên. Không tự đóng plan hoặc duyệt công việc ngoài phạm vi.
