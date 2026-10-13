# Tiến độ PDF khớp chi tiết và BM02 — Development, 2026-10-12

**Mục đích chính:** PDF của đúng lần gửi phải phân biệt chưa ghi tiến độ với đã ghi `0%`, dùng dữ liệu hiện có và giữ nguyên lịch sử.

**Giới hạn:** `0%`, dữ liệu invalid và wrapper được kiểm bằng PDF tạo từ regression; chưa có fixture trình duyệt tương ứng. Lượt này không chạy Google Sheets, Excel desktop/in hoặc mobile. Chưa có nghiệm thu UX của anh cho phần sửa PDF.

**Trạng thái:** IMPLEMENTED / TECHNICALLY VERIFIED trên Development. Review PDF/ảnh riêng được anh xác nhận NOT_REQUIRED trong quyết định `3 điểm chưa nghiệm thu trên không cần thiết` ngày13-10-2026. Không chuyển quyết định này thành UX signoff hoặc kiểm thử mới. Không có blocker đã phát hiện trong phạm vi duyệt; nhãn thời tiết đã chốt riêng.

## 1. Lỗi và cách sửa đã kiểm chứng

PDF trước sửa đọc header `progress_percent` trực tiếp. Báo cáo đồng bộ từ phiếu quy trình có header mặc định `0`, dù người lập chưa ghi tiến độ. Chi tiết và BM02 đọc tiến độ đã ghi trong structured snapshot, nên hai phần đó báo thiếu đúng còn PDF in `0%` sai.

Đã tái sử dụng selector `nativeBMGSDailyRecordedProgress` tách từ cách chọn nguồn của BM02. PDF và BM02 dùng chung selector và parser hiện có:

| Nguồn / dữ liệu | Kết quả PDF |
| --- | --- |
| Phiếu quy trình: chưa ghi, blank/null/invalid | `Chưa ghi tiến độ`; không có thanh tiến độ |
| Phiếu quy trình: thực sự ghi0 | `0%`; giữ thanh nền rỗng |
| Phiếu quy trình: giá trị hợp lệ0–100 | Hiện đúng phần trăm và thanh tương ứng |
| Báo cáo thủ công | Giữ header đã ghi làm nguồn; không lấy structured override |

Legacy `{value: ...}` được unwrap cục bộ tối đa8 lớp, cùng giới hạn với chi tiết. Không đổi generic answer unwrap hoặc weather. Không fallback sang header mặc định khi workflow thiếu dữ liệu, không tra nguồn live hoặc ghi lại snapshot. Bố cục PDF giữ nguyên; chỉ cách hiển thị tiến độ được sửa theo mockup đã duyệt.

## 2. Files và ranh giới

- [bm_gs_export_projection.go](../backend/internal/core/bm_gs_export_projection.go): selector chung và BM02 gọi selector đó.
- [work_report_pdf.go](../backend/internal/core/work_report_pdf.go): summary dùng selector, nhãn thiếu và điều kiện vẽ thanh.
- [work_report_pdf_progress_test.go](../backend/internal/core/work_report_pdf_progress_test.go):24 scenario PDF thực, số rectangle và full revision bytes/hash nonmutation.

Không thêm field, đổi API/query/schema/migration hoặc sửa frontend. Existing WorkReportReviewDialog → protected download → `PDF bản này` vẫn gọi `/work-reports/:id/pdf?revision=<selected>`; service tiếp tục kiểm quyền và đọc đúng immutable WorkReportRevision. Nút BM02 `Xuất báo cáo` trong dialog vẫn dùng exportExcel/protected export đang có. Dialog, toast và protected evidence composition giữ nguyên; không tạo viewer hoặc thao tác mới.

Đã cập nhật central/backend/frontend/triage và domain Construction/Reporting/Workflow mappings. Không đổi lifecycle, actor, quyền, weather, vật tư hoặc nguồn ảnh.

## 3. Tests, build và source

Regression RED chạy trước sửa, tái hiện thiếu label đúng trong PDF workflow; raw tool-result excerpt được ghi rõ tại [worker-red.log](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/worker-red.log). Đây là excerpt, không phải log terminal đầy đủ được tạo lại sau sửa.

| Kiểm tra | Kết quả thực tế |
| --- | --- |
| Worker PDF + reopened BM02 authority + weather | PASS1.867s; [log](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/worker-green.log) |
| Root broader focused PDF/BMGS/native form/weather | PASS5.417s; [log](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/backend-focused.log) |
| `go vet ./internal/core` qua dedicated runner | Exit0; [log](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/backend-vet.log) |
| Scoped gofmt và `git diff --check` | Không có lỗi |
| Independent GPT-6.1 Sol High source review | Không có P0–P2 trong delta đã freeze |
| Normal Development restart/Go build | Exit0, down→build→run; [log](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/development-restart.log) |
| Root evidence verifier | 44 checks PASS;12 image hashes |

Lệnh root:

```text
python3 /private/tmp/consflow-bm02-receipt-test-runner.py test ./internal/core -run 'TestWorkReportPDF|TestBMGS|TestNativeDailyFormExport|TestConstructionWeatherPresentation' -count=1
python3 /private/tmp/consflow-bm02-receipt-test-runner.py vet ./internal/core
make restart ENV_FILE=/private/tmp/consflow-bm02-development-20261012.env
```

Tests dùng dedicated `consflow_bm02_receipt_test_20261012`, không ghi fixtures vào Development. Generated PDF tests cover absent/omitted/blank/null/header conflict/zero/decimal/numeric strings/invalid range/legacy8-versus9 wrappers/manual authority; test kiểm actual text và absence/zero/positive bar counts. Reopened BM02 regression hiện có tiếp tục chạy. Không chạy lại full suite hoặc frontend TypeScript build riêng cho thay đổi backend này; frontend source không đổi.

Source HEAD `75dc44707785d399ed2408959bf784928a544e96` cộng working tree đã giữ; so baseline weather967: hai file hiện có đổi, một regression mới. Manifest968, SHA256 `fb82303bd2a8a0ea4fc46f3424c082c18030f6a90d66a03bc54ea7b3095bd011`, freeze10:36:45UTC/17:36 giờ Việt Nam. Toàn bộ968 hash khớp sau nghiệm thu. Chưa commit/push.

Development UI5173/API8081; giữ task-local env và tiến trình Java8080 khác. Buildx filesystem escalation được tự động duyệt; không sửa Docker/Makefile/env repository hoặc xóa volumes. Backend/admin containers started10:37:18/19UTC, health/ready HTTP200 tại10:48UTC:

- backend `sha256:a669db43e6ede2571e57c3b7d818a89c2167b556ce4b2c15f0306048add2ebe0`.
- admin `sha256:2fc711645f527a8fea04477bca8578d9887aa652af799e577d34ee78d9964ed3`.
- DB `consflow` giữ157 migrations/6projects/21forms.

## 4. Nghiệm thu từ các nút thực tế

Tái sử dụng hai báo cáo synthetic đã có và người nhận hợp lệ. Đăng nhập Admin bình thường sau khi phiên cũ hết hạn. Chỉ xem/tải; không tạo, sửa, gửi lại hoặc xác nhận đã xem hồ sơ.

| Bản được chọn tại `PDF bản này` | Kết quả | File tải thực tế |
| --- | --- | --- |
| Workflow lần1 | Chưa ghi tiến độ; Nắng nóng; không có thanh | [PDF lần1](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/PDF-quy-trinh-lan-1.pdf) |
| Workflow lần3 | Chưa ghi tiến độ; Mưa; không có thanh | [PDF lần3](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/PDF-quy-trinh-lan-3.pdf) |
| Manual lần1 |25%; đúng nội dung lần1 | [PDF25%](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/PDF-thu-cong-lan-1-25.pdf) |
| Manual lần2 |60%; đúng nội dung lần2 | [PDF60%](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/PDF-thu-cong-lan-2-60.pdf) |

Cả bốn bản sao byte-equal với actual Downloads; không sửa PDF sau tải. Mỗi PDF một trang, có đúng số lần gửi và SHA-256 của snapshot tương ứng. Hai PDF workflow đối chiếu với bản weather trước sửa: sau chuẩn hóa whitespace, toàn bộ text chỉ đổi lần xuất hiện `0%` thành `Chưa ghi tiến độ`; nội dung khác giữ nguyên. Không tuyên bố file PDF trước/sau byte-equal hoặc stored snapshot đã đổi.

Đối chiếu thêm hai actual `Xuất báo cáo` BM02: [workflow lần3](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/BM02-quy-trinh-lan-3-doi-chieu.xlsx) và [manual lần2](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/BM02-thu-cong-lan-2-doi-chieu.xlsx). Tất cả resolved cell values/styles/number formats, sheet names/merges và media bytes giữ đúng accepted baseline. Workflow D10 vẫn `Chưa có ghi nhận tiến độ trong nguồn báo cáo.`, G26 chuỗi nguồn `12.000`; manual D10 vẫn60%. Không sửa workbook sau tải; không tuyên bố Google recalculation mới hoặc whole-ZIP byte identity với baseline.

## 5. Lịch sử và dữ liệu giữ nguyên

Before/after toàn bộ payload workflow form/report/receipt/manual-report bằng nhau. Counts và MD5 từng dòng của13 bảng nghiệp vụ bằng nhau: projects, work_items, workflow_form_instances, workflow_form_events, operation_daily_reports, work_reports, work_report_revisions, work_report_events, incoming_acceptance_records, incoming_acceptance_items, evidence_records, workflow_form_versions, workflow_form_fields. Migration count157 không đổi. Không sửa nguồn/phiếu/báo cáo/receipt/snapshot, thêm field/form/revision hoặc stock posting. Đường export vẫn gọi audit hiện có tại service_work_reports.go:580; phiên xác thực renew bình thường. Audit/session không thuộc13 bảng đối chiếu, nên không tuyên bố toàn database không có write.

Điều được sửa là cách render PDF tải mới từ snapshot cũ. Nội dung và SHA-256 snapshot giữ nguyên. Những file evidence tải ở lượt trước vẫn được giữ nguyên.

## 6. Ảnh để review

Browser desktop1280×720:

- [Workflow hiện tại thiếu tiến độ](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/01-workflow-current-detail.jpg).
- [Các nút PDF workflow](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/02-workflow-pdf-buttons.jpg).
- [Workflow lần1 giữ nội dung cũ](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/03-workflow-old1-detail.jpg).
- [Manual hiện tại60%](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/04-manual-current-60-detail.jpg).
- [Các nút PDF manual](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/05-manual-pdf-buttons.jpg).
- [Manual lần1 giữ25%](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/06-manual-old1-25-detail.jpg).

Render trực tiếp PDF Downloads bằng Poppler125dpi, đã xem toàn trang để kiểm chữ/bố cục/thanh/metadata/footer:

- [Workflow lần1](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/10-render-workflow1.png).
- [Workflow lần3](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/10-render-workflow3.png).
- [Manual25%](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/10-render-manual1.png).
- [Manual60%](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/10-render-manual2.png).
- [Generated test PDF actual0%](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/10-render-test-zero.png), [generated42.5%](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/10-render-test-decimal.png). Hai ảnh này là pure-test evidence, không phải file tải từ browser/record Development.

Không thấy chữ bị cắt/tràn hoặc thanh sai trong các trang đã xem. Việc đóng dialog ngay sau một lần xuất XLSX chưa đổi trạng thái UI trong observation đó; dùng URL report đã được UI mở trước để tiếp tục đối chiếu. Không sửa frontend và không kết luận nguyên nhân cho sự kiện đóng này; đây không phải nghiệm thu toàn bộ dialog lifecycle.

## 7. Bằng chứng và việc tiếp theo

[Checks](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/acceptance-checks.json), [source](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/source-state.json), [runtime](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/runtime-after-restart.json), [PDF download origins](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/browser-downloads.json), [XLSX download origins](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/side-downloads.json), [verifier](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/verify_progress.py), [independent audit](acceptance-evidence/bm02-pdf-progress-development-2026-10-12/independent-audit.md).

Verifier dùng Python runtime có pypdf/openpyxl và helper/session tạm `/private/tmp/consflow-bm02-receipt-mapping-uat.py`; có renew phiên bình thường và ghi JSON evidence. Không sửa dữ liệu nghiệp vụ/PDF/XLSX. Không phải script standalone chạy ở máy khác mà không chuẩn bị phụ thuộc. Independent audit đọc source/evidence/render đã lưu, không chạy browser thứ hai.

**Việc tiếp theo:** không còn yêu cầu review PDF bổ sung trong đợt nghiệm thu này theo quyết định của anh. Không còn việc triển khai trong phạm vi duyệt; các giới hạn kiểm thử trong báo cáo giữ nguyên. Không tự đóng plan hoặc thực hiện Git delivery.
