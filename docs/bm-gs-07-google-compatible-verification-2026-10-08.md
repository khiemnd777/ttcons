# BM-GS-07 — nghiệm thu file xuất tương thích Google Sheets

Ngày: 2026-10-08. **IMPLEMENTED / TECHNICALLY VERIFIED / GOOGLE RENDER ACCEPTANCE PASS trong UAT tổng hợp.** Microsoft Excel Place in Cell, hiển thị Excel, in thực tế, ảnh client thật và peak memory ở giới hạn tối đa chưa được xác minh. Kết quả này chỉ áp dụng BM07, không đóng toàn bộ kế hoạch BM-GS.

## Quyết định đã duyệt

Người dùng duyệt sửa bộ xuất BM07 hiện có để tạo ảnh nhúng `oneCellAnchor` trực tiếp từ nút `Xuất báo cáo`, tính kích thước theo dữ liệu báo cáo thực tế. Không thêm thao tác chọn/chèn ảnh hoặc xử lý file sau tải. Đây là thay đổi định dạng có chủ đích: XLSX chứa drawing neo vào ô; đường import Google đã kiểm chứng chuyển drawing thành ảnh native trong ô. Không coi đây là Microsoft Excel Place in Cell.

Bản thử bốn ảnh trước đó chỉ là file đã biến đổi bằng script ngoài ứng dụng. Bằng chứng nghiệm thu dưới đây là file mới tải từ nút thật sau rebuild, được import nguyên file; không dùng script biến đổi hoặc chèn lại ảnh trên Google.

## Source, build và môi trường

- Working tree chưa commit trên `main`, HEAD `75dc44707785d399ed2408959bf784928a544e96`.
- Hai application files thay đổi so với manifest 100 files của lần kiểm tra trước: `backend/internal/core/bm_gs_cell_images.go` và `bm_gs_cell_images_test.go`.
- Writer SHA256 `f787ee2822d7b311d340239484349e8a7f32f9b611df097c17ebb2f163e1966a`; tests `1768726523d6b33fa1ab5c11c574e1bccc20a10356b70bafdf6a70eb88acfc97`.
- Fingerprint 100 files `f51c896a4408fc0f3e992f76905e8d0c230699efdab5f82d7c1d72c13a766793`; thuật toán và từng file nằm trong [source manifest](acceptance-evidence/bm-gs-native-2026-10-08/bm07-google-compatible/tested-source-state.json). Frontend và các application files khác không đổi.
- macOS, Go/Excelize 2.9.1 hiện có; không thêm dependency/migration. Cache Go: `GOPATH=/private/tmp/consflow-gopath`, `GOMODCACHE=/private/tmp/consflow-gomod`, `GOCACHE=/private/tmp/consflow-bmgs-go-cache`.
- Production entrypoint và guarded task server đều build thành công trên source đã freeze. Task binary build lúc **16:46:39 UTC**, restart **23:47:06 ICT**, PID13742/session14348; API `127.0.0.1:18082`, UI `127.0.0.1:15174`.
- Guarded launcher chỉ kết nối DB tổng hợp `bmgs_native_browser_20261008` tại PostgreSQL loopback55532, giữ runtime routes/JWT/RBAC/storage; không chạy production workers. Read-only DB inspection xác nhận 156 migrations/max V156. Không sửa schema/dữ liệu hồ sơ hoặc normal/production database.
- Actor: Admin UAT. WorkItem `777ced83-58f0-4b0b-bf1d-a7b82202eceb`; bốn ảnh ngang/dọc/vuông/cao, hai ảnh khác nhau có cùng filename. Đây là dữ liệu tổng hợp, không phải ảnh công trình/client.

## Implementation

- Giữ exact source/evidence/revision binding và xử lý ảnh có sẵn. `bmgsBindCellImages` tính chiều cao theo độ rộng F thực tế, tỷ lệ ảnh và text lân cận.
- Excelize tạo media/content types/worksheet relationships. Adapter `encoding/xml` có types chuẩn hóa riêng drawing mới thành `oneCellAnchor` với offset0 và kích thước EMU theo ô thực tế. Không dùng regex để sửa worksheet hoặc copy số dòng/chiều cao của fixture bốn ảnh.
- Ô ảnh trống, không có rich-value parts hoặc stored error cells. Local media bytes và alternative metadata nguồn được giữ trong XLSX. Việc Google giữ alternative metadata chưa xác minh.
- Duplicate IDs/ZIP parts/relationships, targets ẩn/không trống/không hợp lệ, vượt giới hạn hoặc sparse media/drawing collision gây lỗi rõ ràng. Drawing/media sheet khác được giữ nguyên. Các mẫu ngoài BM07 tiếp tục dùng appendix ảnh hiện có.
- Không sửa frontend, route, quyền hoặc persistence. Tái sử dụng nguyên `EvidencePanel`, `NativeExcelExportButton`, protected API download và toast chuẩn. Không cần frontend mapping update vì các entrypoint/hành vi UI không đổi; central/backend/triage/Construction Operations/Reporting maps đã cập nhật.

## Kiểm tra kỹ thuật

| Kiểm tra | Kết quả thực tế |
| --- | --- |
| `go test ./internal/core -run '^TestBMGS' -count=1` | Implementation PASS1.727s; independent High audit PASS1.544s |
| `go vet ./internal/core` | PASS ở implementation và audit |
| `go build -mod=readonly -o <task-output>/server-production ./cmd/server` | PASS, backend module |
| `go build -mod=readonly -o <task-output>/native-server-compatible ./server` | PASS, guarded runtime module |
| Scope diff/format check | PASS; final source hashes giữ nguyên sau build/acceptance |
| Behavioral package regressions | 51 ảnh, nhiều sheet/đổi tên, tùy biến chiều rộng/chiều cao, tỷ lệ cực đoan, long text, filename trùng/revision khác, identity/hidden/nonblank targets, collision/ZIP/byte bounds và giữ ảnh các sheet/mẫu khác |
| Independent Sol High audit | Không có P0/P1/P2 finding trong frozen replacement delta; reviewer không chạy browser/DB |

Tests resolve worksheet → drawing → relationship → đúng bytes ảnh từng dòng; không chỉ tìm chuỗi XML. Fixture tests không thay bằng chứng browser/Google bên dưới. Frontend build không chạy lại vì không có thay đổi frontend; maximum-capacity memory benchmark không chạy.

## File tải trực tiếp từ nút

Browser click `Xuất báo cáo` trên Sổ minh chứng của đúng WorkItem; nút đổi `Đang xuất...`, rồi trở lại và có toast `Đã tải biểu mẫu Excel`. Download event trả `/Users/khiemnguyen/Downloads/BM-GS-07 (3).xlsx`.

- [Actual workbook giữ nguyên](acceptance-evidence/bm-gs-native-2026-10-08/bm07-google-compatible/actual-export.xlsx): **86,319 bytes**, SHA256 **`c0782ea449b1364810594177654ac13301fccdc8519abcd2cd953aa190b75da2`**.
- Một tab BM-GS-07, F8–F11 trống và bốn embedded oneCellAnchor, không rich metadata/stored errors/appendix.
- Bốn media hashes bằng chính media của file hệ thống lỗi trước đó: khác biệt là cấu trúc ghi/neo ảnh. Bốn file ảnh gốc cũng giữ nguyên checksum.
- F rộng24; row heights thực tế114/228.5/130.5/240pt. Extents ảnh ngang168×94.50px, dọc168×298.67px, vuông168×168px, cao87.09×314px. Các ảnh nằm trong ô, giữ tỷ lệ; dimensions là kết quả tính động.
- [Inspection](acceptance-evidence/bm-gs-native-2026-10-08/bm07-google-compatible/actual-export-inspection.json), [original hash checks](acceptance-evidence/bm-gs-native-2026-10-08/bm07-google-compatible/original-file-checks.json).

## Google Sheets thực tế

[BM07 — file tải trực tiếp từ Xuất báo cáo](https://docs.google.com/spreadsheets/d/1vjzyxWF5ImPpx7Atjr03w2QaYH6RaCNREkJv6rEkjJk/edit?gid=349641001).

- Connector import dùng nguyên đường dẫn actual download, `native_google_sheets`; response `converted:true`, MIME native Google Sheets. Một tab BM-GS-07/sheetId349641001.
- Bounded CellData read A1:G19 có **0 errorValue**, giữ nội dung nguồn và header. F8–F11 được API trả như cells trống; riêng điều đó không chứng minh ảnh trong ô.
- Authenticated Edge đã kiểm riêng F8, F9, F10, F11. Mỗi context menu `Image` có **`Add alt text to cell`** và **`Put image over cells`**, xác nhận native Google cell-image semantics. Không bấm chuyển ảnh hoặc sửa nội dung sheet.
- Screenshots tại **100% sheet zoom** cho thấy ảnh ngang/dọc/vuông/cao đúng dòng, đủ bốn marker góc, tỷ lệ nguyên vẹn và không che text ở các cột lân cận. Các vùng cuộn bao phủ đủ bốn ảnh/khung; ảnh chụp footer xác nhận các nhãn dòng14. Ảnh tổng quan tại **90%** chỉ phục vụ xem cả bốn ảnh cùng lúc; không thay thế kiểm tra100%.

Bằng chứng: [upper100%](acceptance-evidence/bm-gs-native-2026-10-08/bm07-google-compatible/google-upper-100.jpg), [lower100%](acceptance-evidence/bm-gs-native-2026-10-08/bm07-google-compatible/google-lower-100.jpg), [footer100%](acceptance-evidence/bm-gs-native-2026-10-08/bm07-google-compatible/google-footer-100.jpg), [overview90%](acceptance-evidence/bm-gs-native-2026-10-08/bm07-google-compatible/google-overview-90.jpg), [import response](acceptance-evidence/bm-gs-native-2026-10-08/bm07-google-compatible/bm07_final_import.json), [CellData readback](acceptance-evidence/bm-gs-native-2026-10-08/bm07-google-compatible/bm07_final_cells.json), F8/F9/F10/F11 native-menu text trong cùng evidence directory.

## Giới hạn và phần còn lại

- Đây là một actual UI download với bốn ảnh tổng hợp. 51 ảnh/multi-sheet/arbitrary sizes đã kiểm bằng package tests, chưa chạy thêm browser fixtures cho các trường hợp đó.
- Kết quả không chứng nhận mọi phiên bản Google importer, Microsoft Excel hoặc chất lượng ảnh client thật. Không gọi drawing là Microsoft Place in Cell.
- Whole BM-GS plan còn lỗi BM04 và các template Sheets khác chưa nghiệm thu đầy đủ. Khoảng thiếu ảnh BM01/BM05 từng ghi nhận đã được rút lại ngày 2026-10-09 sau kiểm tra file client: hai mẫu không yêu cầu ảnh. Không sửa application trong lần đính chính này.
- Không commit/push/deploy hoặc đóng plan. Existing Vietnamese TechNote Level3 được cập nhật kết quả này; Previous/Next/Replay và saved-state kiểm bằng mock-DOM, không tính đó là product acceptance.
