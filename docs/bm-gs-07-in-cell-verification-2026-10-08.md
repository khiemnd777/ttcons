# BM-GS-07 — ảnh trong ô, 2026-10-08

> Bản ghi lịch sử của phương án Microsoft rich-value đã bị lỗi khi import Google Sheets. Phương án này đã được thay bằng writer tương thích Google sau khi người dùng duyệt. Trạng thái hiện tại và file tải từ nút mới: [BM07 Google-compatible verification](bm-gs-07-google-compatible-verification-2026-10-08.md). Không dùng các hashes/kết quả bên dưới như bằng chứng của source hiện tại.

**Trạng thái: IMPLEMENTED / TECHNICALLY VERIFIED cho code và cấu trúc file; NOT UX ACCEPTED.** Google Sheets chuyển đổi file tải từ UI thành `#VALUE!` tại F8–F11, không hiển thị ảnh. Microsoft Excel, kiểm tra canh ảnh/crop bằng ứng dụng có hỗ trợ và in thực tế chưa chạy.

Mục đích chính: từ **Đầu việc → Minh chứng**, người dùng bấm **Xuất báo cáo** để nhận sổ ảnh theo mẫu client, với ảnh đã lưu nằm trong ô của đúng dòng. Không thêm chọn mẫu, chọn nguồn hoặc upload ảnh lúc xuất.

## Phạm vi được duyệt và thay đổi

- Phê duyệt `Duyệt triển khai` sau yêu cầu đặt ảnh trong cell và canh cell vừa ảnh. Chỉ sửa BM07; giữ header cột F, cột khác, native button, quyền và nguồn. BM04 và quan hệ ảnh của bốn form khảo sát/rủi ro vẫn ngoài phạm vi.
- `bm_gs_export_projection.go` và `service_native_bm_gs_exports.go` dùng chung định danh nguồn/ảnh/revision. `bm_gs_workbook.go` bỏ appendix chỉ cho BM07. `bm_gs_cell_images.go` nối từng dòng bằng khóa chính xác và ghi Microsoft local-image rich metadata vào F8/F9…; không có drawing hoặc URL ảnh ngoài.
- Giữ header template nguyên bản `TÊN FILE ÂNH THAM CHIẾU` (typo đã có trong client). Tên file/ID/revision/ghi chú ảnh phái sinh nằm trong alternative metadata của ảnh.
- Cột F rộng 24 ký tự. Chiều cao do ảnh yêu cầu từ 54 đến 240 pt, theo tỷ lệ ảnh, cộng 6 pt dự phòng; text lân cận có thể yêu cầu chiều cao lớn hơn. Không sửa hình gốc. Kiểm tra cấu trúc cho biết các kích thước đã ghi; chưa chứng minh hình thực tế vừa/căn đúng/không crop trong Excel.
- GPT-6.1 Sol Medium triển khai; Sol High review read-only. Finding đổi header ngoài phạm vi đã được sửa và có regression so với template. Review cuối không còn finding actionable trong delta đã đọc; reviewer không tự chạy tests hoặc browser.

## Nguồn và môi trường kiểm tra

- Working tree chưa commit trên `main`, HEAD `75dc44707785d399ed2408959bf784928a544e96`. Manifest 100 backend/frontend files lúc **14:44:29 UTC**: combined `ce9532fc33e7efa85f47ed1e994ef0c4b994c093ed335050adcfb1d16b99f7fa`; backend `d2734c1b2111f7d3930c479a5dd6b16370c713358c328f475604ac663eceaa5e`. Frontend giữ fingerprint `aceb30e7d9589baaf2461ae7d904f0afa694dab9626ac5b1d935599dcbabe6ff` của lần nghiệm thu trước.
- macOS arm64, Go 1.25, cache Go riêng trong `/private/tmp`; Excelize vẫn 2.9.1. Không đổi dependencies hoặc migrations trong sửa BM07.
- API task riêng được build lại từ source cuối và khởi động **21:39:30 ICT**, port18082; Vite15174. PostgreSQL16 DB `bmgs_native_browser_20261008` ở loopback55532, 156 migrations của UAT đã có; Redis DB2/3 và protected storage task. Launcher dùng core routes/JWT/RBAC/storage thật, bỏ normal background workers/observability. Không phải main stack/production.
- Đầu việc synthetic mới `777ced83-58f0-4b0b-bf1d-a7b82202eceb` chứa bốn ảnh ngang/dọc/vuông/cao. Hai ảnh đầu trùng filename nhưng khác Evidence ID/checksum. Agent fixture dùng canonical upload và protected download, kiểm bytes/hash; SQL chỉ tạo/đặt nguồn mới trong task DB. Không sửa ảnh/phiếu cũ hoặc schema.

## Kiểm tra đã chạy

| Kiểm tra | Kết quả | Giới hạn |
|---|---|---|
| `go test ./internal/core -run '^TestBMGS' -count=1` với task cache | PASS, 1.641s | Renderer/projection pure tests; không thay broad DB regression của lượt trước |
| `go build ./internal/core`, current production entrypoint `go build ./cmd/server`, guarded UAT launcher build | PASS | Build cuối sau sửa header; binary production chỉ build, không chạy/deploy |
| `go vet ./internal/core`, scoped format/diff và `git diff --check` | PASS | Không phải kiểm chứng UX |
| Dòng/ảnh đúng ID, trùng tên, revision khác, đảo thứ tự tập ảnh, metadata chain và bytes | PASS | Dữ liệu tổng hợp; native Excel reader của Excelize không nhận diện trường Text optional hợp lệ |
| Bốn nguồn gộp/đổi tên sheet, text12 dòng, 51 dòng và print area, thiếu/trùng/orphan/corrupt image | PASS | Metadata/bố cục XML, không phải ảnh screenshot từ Excel |
| BM02 giữ appendix và các mẫu khác giữ template/header/print setup | PASS trong scope `TestBMGS` | Không chạy lại toàn bộ nghiệm thu tám mẫu |
| Bấm `Xuất báo cáo` từ native EvidencePanel có4 ảnh, actual JWT task runtime | File tải được; nút trở về trạng thái thường | Một scenario admin/synthetic, không phải ma trận RBAC mới |
| Google native Sheets conversion của actual UI download | **FAIL**: F8–F11 `#VALUE!`, không có ảnh | Chưa đạt hiển thị/canh ảnh; không kết luận mọi phiên bản Google Sheets đều không hỗ trợ |
| TechNote8 bước, Previous/Next/Replay, bounds và state restoration | PASS mock DOM | Layout nguồn không đổi; không chứng minh workbook hoặc kiểm thử người dùng |

Regression giữ row identity, text dài, source grouping và native metadata có assertion nội dung/bytes. Phần service chỉ chia sẻ hàm key có giá trị như trước; route/auth/DB/schema không đổi. Vì frontend component/interaction không đổi, không cần cập nhật frontend mapping hoặc chạy lại frontend build; central/backend/triage/Construction Operations maps đã được cập nhật.

## File xuất thực tế

[Workbook tải từ UI](acceptance-evidence/bm-gs-native-2026-10-08/bm07-in-cell/actual-export.xlsx): **87,954 bytes**, SHA256 `19fa2ffc73ee9fa6cab36d4a4824b2a159ee99a37a5eb74f1c0ed3e766165896`. Một sheet BM-GS-07, bốn native local-image cells, không có drawing hoặc appendix. Chuỗi F-cell `vm` → valueMetadata → XLRICHVALUE → `_localImage` → richValueRel → media nội bộ đúng từng ID ảnh.

| Ô | Ảnh | Kích thước phái sinh | Chiều cao dòng | Bytes gốc → phái sinh |
|---|---|---|---|---|
| F8 | Ngang | 1280×720 px | 114 pt, text quyết định | 14,151 → 32,488 |
| F9 | Dọc | 720×1280 px | 236.67 pt | 14,085 → 35,155 |
| F10 | Vuông | 1000×1000 px | 135.75 pt | 10,756 → 10,756 |
| F11 | Cao | 355×1280 px | 240 pt | 10,079 → 22,066 |

Các PNG synthetic vốn nén tốt; resize ba ảnh làm tăng bytes. Đây là hành vi optimizer giữ PNG đã có, không phải thay đổi mới của sửa in-cell. Không tuyên bố mọi ảnh hoặc workbook sẽ nhỏ hơn gốc; tối ưu dung lượng/chất lượng trên ảnh client thật chưa nghiệm thu. Gốc/checksum được giữ; cạnh dài phái sinh vẫn tối đa1280 px.

[Ảnh màn nguồn](acceptance-evidence/bm-gs-native-2026-10-08/bm07-in-cell/actual-source-ui.jpg), [ảnh Google Sheets thực tế](acceptance-evidence/bm-gs-native-2026-10-08/bm07-in-cell/actual-google-render.jpg), [kết quả inspection](acceptance-evidence/bm-gs-native-2026-10-08/bm07-in-cell/actual-export-inspection.json), [source/build/test manifest](acceptance-evidence/bm-gs-native-2026-10-08/bm07-in-cell/renderer-verification.json), [tested working-tree fingerprint](acceptance-evidence/bm-gs-native-2026-10-08/bm07-in-cell/tested-source-state.json).

## Google conversion: đã quan sát và chưa biết

Actual UI export đã import thành native Sheet; metadata xác nhận một tab BM-GS-07. API bounded F7:F11 và authenticated Edge tại100% cùng cho thấy bốn ô ảnh là errorVALUE, không có ảnh. [Bản Google Sheets đã thử](https://docs.google.com/spreadsheets/d/1kSTOESFZnGkVipNqWYNrTalKVardmakPUZDTLCg8lcg/edit).

Hai prototype dùng metadata có/không Text và control từ bundled XlsxWriter3.2.9 `embed_image` cũng thất bại qua cùng đường native import. Control độc lập giảm khả năng lỗi chỉ do writer mới; đây là bằng chứng về đường chuyển đổi đã thử, chưa phải chứng nhận XLSX hợp lệ trong Excel hoặc kết luận hỗ trợ Google toàn cục. [Control độc lập](https://docs.google.com/spreadsheets/d/116VxgC7LI4ZYN6tofPID59mQ5P7rlJbHxTgbdGzWXqI/edit); [screenshot control](acceptance-evidence/bm-gs-native-2026-10-08/bm07-in-cell/reference-google-import.jpg).

Theo [Microsoft Local Image Type](https://learn.microsoft.com/en-us/openspecs/office_standards/ms-xlsx/aac1e2a6-7072-4e66-b1de-f20b8bf63318), local image có ID và CalcOrigin; Text optional hợp lệ. Raw XLSX cell dùng sentinel `#VALUE!` kèm rich metadata; raw sentinel không tự chứng minh lỗi. Google sau chuyển đổi không hiển thị ảnh mới là render failure đã quan sát. [XlsxWriter embed_image](https://xlsxwriter.readthedocs.io/worksheet.html#worksheet-embed-image) ghi rõ đây là PlaceInCell của Excel3652023+; không dùng screenshot mô phỏng hoặc ảnh được chèn lại thủ công vào Google làm proof cho file hệ thống.

## Việc còn lại

1. Mở chính actual-export.xlsx bằng Excel có hỗ trợ PlaceInCell để kiểm ảnh, vị trí/căn/padding, tỷ lệ/crop, các dòng ảnh cao và text dài, vùng in và sort/filter nếu cần. Máy hiện tại chưa có Excel; không có kết quả này.
2. Nếu native Google Sheets là đích xuất bắt buộc, cần một review package riêng cho integration/format tương thích, thay vì dùng floating image hoặc URL ảnh công khai ngoài hợp đồng đã duyệt.
3. Giữ các gap BM04/form-photo và tối ưu ảnh thật trong plan hiện tại. Không commit/push/deploy, không migrate DB thường/production và không đóng plan trong lượt này.
