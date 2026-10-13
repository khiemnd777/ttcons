# BM04 — Nghiệm thu danh sách và chi tiết Cần xử lý, 13-10-2026

**Cập nhật sau duyệt sửa:** P2 cột thao tác desktop bên dưới đã được sửa và kiểm lại ở1024/1280/1440px. Xem [báo cáo sửa cột thao tác](bm04-action-column-development-verification-2026-10-13.md). Còn lỗi riêng nút nổi Tiện ích che một phần CTA trên mobile; chưa UX ACCEPTED. Phần dưới giữ nguyên bằng chứng của lượt nghiệm thu trước khi sửa: tại1280×720, Gửi khắc phục lấn12,34375px vào tên việc; hai file Excel đã kiểm vị trí, tổng lỗi và trạng thái đúng.

## Phạm vi và môi trường

- User duyệt BM04 từ danh sách/chi tiết trên trình duyệt và Google Sheets; actual `Xuất báo cáo`, kiểm vị trí/tổng/status, không field mới và giữ lịch sử. Không có approval sửa app/UI/field, chuyển trạng thái, migration, Production, Git delivery hoặc đóng plan.
- Main Development UI5173/API8081, database consflow/V158, HEAD `75dc44707785d399ed2408959bf784928a544e96` cộng dirty tree đã giữ.976 source/test/migration hashes khớp build checkbox đã nghiệm thu; không rebuild/application-suite khi app source không đổi. Chạy verifier UAT cho file/readback/history, không coi đó là full application coverage.
- Tái dùng project `5020b7b5-d383-4504-967d-5e705c4ee2dd`, code DEV-BM04-REUSE-20261009, một action `ae4aba28-043c-48fa-8125-6b6da4a313d4` và check `6ee90828-61b2-4402-af9b-a22e8f41658f` đã có. Actor Admin, fixture synthetic; không tạo/chỉnh source hoặc biến fixture này thành full Rev10/all-role/all-status acceptance.
- Maps central/backend/frontend/triage + Construction Operations/Reporting và spreadsheet/native-Google instructions đã đọc. Owners `service_native_bm_gs_owners.go::nativeConstructionActionSource`, native action list filter, `bm_gs_export_projection.go`, `bm_gs_workbook.go`, `ConstructionWorkspaceLists.tsx`, `ConstructionActionDialogs.tsx`, shared `NativeExcelExportButton`/source/evidence dialogs. First searches nativeConstructionActionSource/export_location/COUNT(A8/nativeActionListExcelPath/actionStatusLabel. Không mapping change vì application behavior/contract không đổi.

## Kết quả hai nút xuất

| Kiểm tra | Danh sách | Chi tiết |
|---|---|---|
| Vị trí C8 | [Synthetic] Khu B — ô kiểm tra04 | Cùng vị trí kiểm tra nguồn |
| Mã lỗi B8 | Trống theo nguồn | Trống theo nguồn |
| Tổng A13 trên Google | Tổng lỗi1; đã khắc phục0; đang xử lý1 | Cùng kết quả1/0/1 |
| Trạng thái I8 | Mới | Mới |
| Dữ liệu/formulas/media | Khớp bản chi tiết | Khớp bản danh sách |
| Google errors |0|0|

- Action status làopen và UI trình bày `Cần xử lý`; BM04 dùng nhãn hiện có `Mới`. Đây là cùng trạng thái; không đổi source status hoặc suy diễn việc đã bắt đầu sửa chữa. Tổng `Đang xử lý` của BM đếm các dòng trạng thái không phảiĐã khắc phục, nên bao gồmMới. Chưa có dòng waiting_confirmation/done trong fixture này; không tuyên bố đã nghiệm thu các dòng đó.
- Action.locationNULL, không có native override/explicit-clear audit; linked check cùngproject cònactive có vị trí `[Synthetic] Khu B — ô kiểm tra 04`. Actual source dialog cho thấy đúng vị trí; export read model kế thừa vị trí đó, không ghi lại action/check. Đọc lại dữ liệu và hash sau lượt này giữ nguyên.
- Đã đọc lại workbook client TOC và BM04 A7:J7. Giữ10 cột gốc. FormulaA13 dùngCOUNT(A8:A10), nên lỗi không có mã vẫnđếm1. Google effectiveValue đã tính thật1/0/1; không chỉ kiểm chuỗi formula. Mẫu thêm phần ghi chú nguồn ở15/16 và giữ đủ ba chỗ ký ở17/18.
- XLSX XML dimension cache vẫnA1:J16 sau footer relocation, nhưng cells/footer thực tế có tới18. Verifier dùng read_only=False để kiểm đủA1:J18; Google/footer đã kiểm rõ và hiển thịđủ. Không sửa file để cập nhật dimension cache. Đây là giới hạn metadata cần phân biệt với missing footer; Excel desktop/print vẫnNOT_RUN.
- Native nguồn có một ảnh liên kết8057abdc-e188-4605-b996-77f339d961a2; hai file cùng derivativePNG SHA8dadf6... và cả hai Google sheetHình ảnh hiển thị đủ gradient/aspect/chú thích. Ảnh nhậpfile không có captured_at; không invent giờ chụp. Không chọn/upload ảnh lại khi xuất.

## File và Google nguyên bản

- Actual list download `/Users/khiemnguyen/Downloads/danh-sach-can-xu-ly-BM-GS-04 (3).xlsx` → copy nguyên byte `BM04-list.xlsx`,14596bytes, SHA256 `4f9cbf89e65dd3de7e927a41eb23e771f6eec341cec4ac296214a83accd74f0b`.
- Actual detail download `/Users/khiemnguyen/Downloads/BM-GS-04 (3).xlsx` → copy nguyên byte `BM04-detail.xlsx`,14597bytes, SHA256 `34cf76a538c9181fcfbf6a0e096821a4bee07d03b79a95b0255ce397a3ad7b23`.
- Không tuyên bố haiZIP byteequal với nhau; values/formulas/photo bằng nhau và từngcopy bằng đúng fileDownloads riêng.
- Native Google danh sách: https://docs.google.com/spreadsheets/d/1yJ1LsVr2D3VAcI0rvHzmvTckfYgVNL9TmVMd_YzGQig/edit . Main2026-10-09_01 gid1532828539, Hình ảnh gid1096122567.
- Native Google chi tiết: https://docs.google.com/spreadsheets/d/1sOMXCGU2Yao9PawpNl23UOpvaPDUZ3YnY_LtAp5xyNU/edit . Main2026-10-09_01 gid1888690589, Hình ảnh gid1397808877.
- Import nguyên byte qua Drive native conversion, không cell/style/dimension/table edit hoặc sharing change. Metadata trước boundedreads: mỗifileA1:J16 + footerA17:J18 + Hình ảnhA1:H25; tổng760 ô,64 populated,0mismatch và0effectiveError/#VALUE. Kiểm tất cả hai populatedtabs/file bằng CUA ở Sheets100%. Header tên công trình dài xuống nhiều dòng; thời hạn giữ nguyênUTC timestamp từ nguồn. Không tuyên bố đã cải thiện các định dạng này hoặc chứng nhận native Excel/print.
- Actual browser Reload vẫnmột openaction. Đã xong0 cho empty state vànút exportdisabled; Tất cả1 giữđúng1record; trảvềCần xử lý1. Không clickGửi khắc phục/Xác nhận/Chỉnh sửa/Tạo việc. Source-check viewer closure không được chấmđạt; dùngReload để quay lại danh sách, không sửa ứng dụng.

## Lịch sử và review

- Root `verify.py`:24 assertionsPASS. Before/after39table source baseline cộng construction/audit tables, khử trùngtable/id, đối chiếu2885 unique oldrows:0changed/0lost. Chỉthêm2audit native_source_exported đúngproject/source/ảnh/twofilebytes tại09:45:48/09:45:56ICT. Không có lifecycle/source/template/history rewrite.
-976 source hashes và158migrations giữ nguyên.149 protected storage files hiện tại bằng snapshot storage-after của lượtBM08 trước đó; không claim đã capture một storage-before mới cho lượt này. Google và Downloads không viết protectedstorage.
- Independent GPT-6.1 Sol High audit chạyverify() import-only không ghi file/domain và kiểm24/24; Downloads/copy,2885unique rows/2audit/976hashes/V158/760cells/64populated/errors0 xác nhận. Reviewer xem8screenshots, không chạy browser; xác nhậnP2overlap, không báo full visualpass.
- Evidence root: `docs/acceptance-evidence/bm04-list-detail-development-2026-10-13/`. Main browser-list/detail/source-location/empty-done, Google-list/detail và haiimage sheets; native imports/metadata/cells/footer, before/after hashes, export-audit, storage-preservation, layoutmeasurement vàverification JSON được giữ.
- Known limits: một dòngopenkhôngmã; nhiều dòng/mixedstatuses/sort/capacity/ordinaryrole/nativeExcel/print/clientphysicalrecords chưa chạy. Không dùng kết quả BM04 để chốt cácBMkhác. Một câuSQLinventory dùng nhầm tablealias khôngtồn tại đã sửa thànhconstruction_action_items bằng sourceevidence; không có domainwrite. Một tọađộtabảnh vượt viewport trảlỗi khôngclick; đã dùngAXindex đúngtab. Không coi orchestrationerrors đó là applicationdefects.

## Đề xuất sửa riêng P2 — chưa triển khai

**Mục đích chính:** giám sát đọc tên việc, trạng thái và thực hiện thao tác ngay trong danh sách Cần xử lý.

**Hiện trạng/cause đã đo:** ở1280×720, firstcell208,13px nhưng flex group chỉ184,13px; CTAshrink-0 kết thúc ởx594,48 trongkhi title bắtđầu x582,13. `ConstructionWorkspaceLists.tsx:112` không ràng buộc cột thao tác theo toàn bộnhómnút, nên nhómnút tràn sang tên việc12,34px. Ảnh và `browser-list-layout-measurement.json` giữ bằng chứng. Đây làP2hiển thị, không làm đổi values/formulas trong haiXLSX.

**Mockup as Text — đề xuất giữ các nút/nhãn hiện có, tách rõ cột:**

```text
CẦN XỬ LÝ                                      [Xuất báo cáo]
[Cần xử lý 1] [Chờ xác nhận 0] [Đã xong 0] [Tất cả 1]
[Tìm việc...] [Tất cả người xử lý] [Mọi ưu tiên] [Mọi ảnh hưởng]

                        | VIỆC / NGUỒN           | ... | TRẠNG THÁI
[👁] [✎] [Gửi khắc phục] | [Synthetic] Sửa bề mặt | ... | Cần xử lý
                        | ô04 / Từ kiểm tra...   |     |
```

- Files dự kiến: chỉlayoutbảngActions trongConstructionWorkspaceLists.tsx vàmappingUI/triage tươngứng sauduyệt. Tái sử dụng RowActions/PrimaryButton/NativeExcelExportButton vàtableviewport hiệncó; khônginput/schema/source/API/lifecycle mới.
- Đề xuất: cột thao tác giữ đủbề rộng thực của nhómnút; màn hình thiếuchỗ dùng cuộnngang trongtableviewport sẵn có. Giữhành vi/hànhđộng/quyền/nhãn. Không sửaCSS global/alltables hoặc đổi mobilecards khôngliênquan.
- Validation nếu được duyệt: frontendbuild; browser1024/1280/1440 đo nooverlap và giữ truycập nút; mobile390 khôngregression; giữsource/history rồi tải lạihaiBM04 vàđối chiếuGoogle. Không dùng classname-only test để tuyên bốUIđạt.
- Rủi ro: cầncuộnngang ởviewports thiếu chỗ. Bộlọc/empty/loading/error/exporttoast giữhànhvi. Phươngán này chưađược anhduyệt; không code/migration/Developmentrebuild tronglượtnghiệmthu.
- **Trạng thái đề xuất này:** đã được user duyệt và triển khai trong báo cáo sửa cột thao tác liên kết ở đầu tài liệu. Bằng chứng trước sửa và Google readback ở đây được giữ nguyên. User review bản sửa vẫn pending; vấn đề mobile riêng chưa được sửa.
