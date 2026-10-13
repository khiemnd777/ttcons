# BM02 — Nhãn thời tiết tiếng Việt trên Development, 2026-10-12

**Cập nhật sau báo cáo:** anh đã xác nhận `Chốt nghiệm thu nhãn thời tiết.` Phần weather được UX ACCEPTED trong phạm vi đã kiểm. Anh đồng thời duyệt sửa riêng tiến độ PDF; kết quả mới tại [PDF progress verification](bm02-pdf-progress-development-verification-2026-10-12.md). Các trạng thái/bằng chứng chụp lúc nghiệm thu dưới đây được giữ để truy vết, không dùng lỗi PDF cũ để mô tả bản sửa mới.

**Mục đích chính:** hiển thị thời tiết đã ghi bằng nhãn tiếng Việt tại đúng hồ sơ và lần gửi đang xem; tận dụng dữ liệu hiện có, giữ nguyên dữ liệu lịch sử.

**Giới hạn cần biết trước:** còn lỗi có sẵn ở tiến độ PDF: báo cáo quy trình chưa ghi tiến độ vẫn in `0%`. Lỗi này được ghi riêng bên dưới và chưa sửa. Ba vị trí banner báo cáo liên kết chưa có fixture trình duyệt; đã kiểm tra bằng render test của component thực. Không tuyên bố toàn bộ PDF hoặc cả tám biểu mẫu đã nghiệm thu.

**Trạng thái:** sửa nhãn thời tiết IMPLEMENTED / TECHNICALLY VERIFIED trên Development. Anh review ảnh và file để chốt UX; hiện NOT UX ACCEPTED. Nghiệm thu mapping vật tư BM02 trước đó vẫn được giữ riêng.

## 1. Phạm vi và nguyên nhân đã kiểm chứng

Anh duyệt phương án A, triển khai1–3, không thêm field, giữ lịch sử, cập nhật Development và kiểm tra trình duyệt/Google Sheets. Phạm vi gồm nhãn trên các màn hình báo cáo thi công đang có, BM02 XLSX và Work Report PDF. Không tạo thao tác chọn mẫu/ảnh, không đổi bố cục hoặc thêm dữ liệu.

Nguồn và snapshot lưu mã `hot`/`rain` hợp lệ. Một số điểm render trước đây in mã trực tiếp. Các lựa chọn tiếng Việt đã tồn tại trong V20 và phiên bản phiếu được chốt. Sửa ở lớp trình bày; nguồn lưu mã cũ.

| Mã đang có | Nhãn hiển thị |
| --- | --- |
| normal | Bình thường |
| hot | Nắng nóng |
| rain | Mưa |
| wind | Gió mạnh |
| restricted_access | Hạn chế mặt bằng |
| other | Khác |

BM02 ưu tiên nhãn chính xác trong lựa chọn của phiên bản phiếu được chốt nếu có. Nhãn đó được giữ nguyên, kể cả khi chữ của nhãn giống một mã khác. Khi snapshot không chứa lựa chọn, dùng hợp đồng sáu mã đã công bố trong V20, không tra danh mục mới nhất. Thiếu thời tiết hiển thị `Chưa ghi thời tiết`; văn bản tự nhập hoặc mã không thuộc sáu mã được giữ nguyên. Giá trị edit/serialization không đổi.

## 2. Thành phần thay đổi và tái sử dụng

- Backend: [helper trình bày](../backend/internal/core/construction_weather_presentation.go), [projection BM02](../backend/internal/core/bm_gs_export_projection.go), [PDF](../backend/internal/core/work_report_pdf.go), [regression](../backend/internal/core/construction_weather_presentation_test.go).
- Frontend: [helper trình bày](../frontend/src/modules/operations/constructionWeatherPresentation.ts), OperationsPage, ConstructionReportDetailDialog, ConstructionCheckDialogs, ConstructionActionDialogs, WorkReportReviewDialog và [regression component](../frontend/tests/constructionWeatherPresentation.test.ts).
- Bảy file hiện có thay đổi và bốn file mới so với bản đã nghiệm thu vật tư. Không đổi field, API, schema, migration, EditModel hay query nguồn.
- Nhật ký/detail/review dùng các component đang có. Phiếu dùng WorkflowFormPanel với lựa chọn đã chốt. Ảnh tiếp tục dùng composition EvidenceRecordViewerDialog và nguồn snapshot được bảo vệ; không tạo viewer mới. Các nút xuất và vị trí giữ nguyên.
- Mapping central/backend/frontend/triage và domain Construction/Reporting/Workflow đã cập nhật với owner, search và regression của hành vi này.

## 3. Kiểm tra kỹ thuật, source và Development

| Kiểm tra | Kết quả thực tế |
| --- | --- |
| Backend focused weather/PDF/BMGS/native form | PASS5.063s; dedicated DB `consflow_bm02_receipt_test_20261012` |
| `go vet ./internal/core` qua runner DB/cache hiện có | Exit0 |
| Scoped gofmt | Không có file lệch format |
| Frontend focused, bốn test files | 23tests PASS261.430291ms; SSR useLayoutEffect warnings không phải nghiệm thu browser |
| `cd frontend && npm run build` | TypeScript + Vite PASS4.38s,1956modules; kết quả worker trên source đã freeze |
| Restart Development | Normal `make restart ENV_FILE=...`: down→build→run, exit0 |
| API `/api/health`, `/api/ready` | HTTP200; database ok |
| Root verifier bằng chứng | 76checks PASS;31 hash ảnh |
| Independent GPT-6.1 Sol High | Không có P0–P2 chặn phạm vi weather; lỗi tiến độ PDF ghi riêng |

Lệnh backend của root:

```text
python3 /private/tmp/consflow-bm02-receipt-test-runner.py test ./internal/core -run 'TestConstructionWeatherPresentation|TestWorkReportPDF|TestBMGS|TestNativeDailyFormExport' -count=1
python3 /private/tmp/consflow-bm02-receipt-test-runner.py vet ./internal/core
```

Logs: [backend](acceptance-evidence/bm02-weather-development-2026-10-12/backend-focused.log), [vet](acceptance-evidence/bm02-weather-development-2026-10-12/backend-vet.log), [frontend focused](acceptance-evidence/bm02-weather-development-2026-10-12/frontend-focused.log), [restart](acceptance-evidence/bm02-weather-development-2026-10-12/development-restart.log), [worker validation](acceptance-evidence/bm02-weather-development-2026-10-12/worker-validation.md), [audit](acceptance-evidence/bm02-weather-development-2026-10-12/independent-audit.md).

Source: HEAD `75dc44707785d399ed2408959bf784928a544e96` cộng working tree đang có, được giữ nguyên ngoài phần duyệt. Manifest967 file, SHA256 `661b896336d710b19183e2c6d9763b6d6848fbe1c0beae0c07cc8dfcfedfe317`, chụp08:19:36UTC/15:19 giờ Việt Nam; toàn bộ967 hash khớp sau nghiệm thu. Đây là thay đổi trên working tree, chưa commit/push.

Development UI `http://127.0.0.1:5173`, API8081. Normal restart giữ task-local env hiện có; Buildx cần quyền ghi activity file ngoài workspace và đã được tự động duyệt. Không sửa cấu hình repository; giữ tiến trình Java8080 khác. Containers tạo08:21:49UTC, chạy08:21:56UTC, health/ready ghi08:22:18UTC:

- backend image `sha256:9619e6673a916665c45c3f6f561f03d30131bb2692a3a73b4acdf9388eab58c6`.
- admin image `sha256:16eceb74d0405c21f737b865965707c1cf60d13701bf440f176baa6e39deba09`.
- DB `consflow`:157 migrations,6projects,21forms. Không có migration mới.

Không chạy lại full suite cho sửa trình bày này. Không dùng kết quả full suite của bản vật tư để khẳng định full suite bản weather.

## 4. Dữ liệu lịch sử giữ nguyên

Tận dụng fixture đã có `DEV-BM02-WORKFLOW-20261012`: phiếu thi công đã chốt, receipt hợp lệ và người nhận hợp lệ. Không tạo/sửa/gửi lại hồ sơ trong lượt này. Mở draft chưa lưu rồi Hủy; chỉ có đăng nhập/renew phiên bình thường.

Đối chiếu trước/sau toàn bộ JSON phiếu, báo cáo và receipt bằng nhau;13 bảng nghiệp vụ có cùng số dòng và MD5 từng dòng: projects, work_items, workflow_form_instances, workflow_form_events, operation_daily_reports, work_reports, work_report_revisions, work_report_events, incoming_acceptance_records, incoming_acceptance_items, evidence_records, workflow_form_versions, workflow_form_fields. Form vẫn11fields. Raw weather các lần gửi vẫn `hot`, `rain`, `rain`; không backfill nhãn hoặc sửa hash snapshot.

| Hồ sơ | Dữ liệu giữ nguyên | Thời tiết xuất mới |
| --- | --- | --- |
| Lần gửi1 | 2m² công việc;3người×8giờ;ảnh đỏ; chưa có vật tư nhập trong snapshot | Nắng nóng |
| Lần gửi2 | 9m²;6người×7giờ;ảnh xanh; chưa có vật tư nhập trong snapshot | Mưa |
| Phiếu hiện tại / lần gửi3 | 9m²;6người×7giờ;ảnh xanh; receipt12.000m² | Mưa |

Không lấy dữ liệu hiện tại để lấp vật tư/ảnh vào lần gửi cũ. Lần3 dùng lại đúng ảnh xanh của lần2, tên/nhãn ảnh vẫn thể hiện nguồn đó.

## 5. Tải từ nút thực tế

Tám file dưới đây là bản sao byte-for-byte của đường dẫn Downloads do sự kiện browser download trả về. Không sửa workbook sau tải hoặc sau import.

| Vị trí / nút đã bấm | File thực tế |
| --- | --- |
| Thi công → Nhật ký → Xuất báo cáo | [BM02 nhật ký](acceptance-evidence/bm02-weather-development-2026-10-12/BM02-nhat-ky-tieng-Viet.xlsx) |
| Chi tiết báo cáo ngày → Xuất báo cáo ngày | [BM02 báo cáo hiện tại](acceptance-evidence/bm02-weather-development-2026-10-12/BM02-bao-cao-hien-tai-tieng-Viet.xlsx) |
| Đầu việc → phiếu đã chốt → Xuất báo cáo ngày | [BM02 phiếu đã chốt](acceptance-evidence/bm02-weather-development-2026-10-12/BM02-phieu-da-chot-tieng-Viet.xlsx) |
| Chi tiết lần gửi3 → Xuất báo cáo | [BM02 lần3](acceptance-evidence/bm02-weather-development-2026-10-12/BM02-lan-gui-3-tieng-Viet.xlsx) |
| Chi tiết lần gửi1 → Xuất báo cáo | [BM02 lần1](acceptance-evidence/bm02-weather-development-2026-10-12/BM02-lan-gui-1-tieng-Viet.xlsx) |
| Chi tiết lần gửi2 → Xuất báo cáo | [BM02 lần2](acceptance-evidence/bm02-weather-development-2026-10-12/BM02-lan-gui-2-tieng-Viet.xlsx) |
| Lần3 → PDF bản này | [PDF lần3](acceptance-evidence/bm02-weather-development-2026-10-12/BM02-lan-gui-3-tieng-Viet.pdf) |
| Lần1 → PDF bản này | [PDF lần1](acceptance-evidence/bm02-weather-development-2026-10-12/BM02-lan-gui-1-tieng-Viet.pdf) |

Bốn XLSX tương ứng với baseline trước weather (phiếu, lần1/2/3): chỉ giá trị ô `2026-10-12_01!B6` đổi nhãn. Shared-string entry tương ứng đổi theo. Mọi giá trị/style khác, geometry, formulas, drawing references và media được giữ; mọi archive part còn lại byte-equal. MIME declarations tương đương dù thứ tự serialize có thể đổi. Không khẳng định cả ZIP trước/sau byte-equal.

Sáu XLSX đều có hai sheet: một sheet dữ liệu và `Hình ảnh`, không có stored cell errors. BM02 gốc không có vùng ảnh riêng, sheet ảnh trong cùng workbook đã được anh chấp thuận trước đó. Không đổi bố trí này. G26 giữ chuỗi NUMERIC nguồn `12.000` ở current/form/r3, đối chiếu Decimal bằng12; không tuyên bố đó là cell số dùng tính toán.

## 6. Google Sheets và ảnh nghiệm thu

Bốn file gốc được import thành bốn workbook native riêng, Private/only me; không ghi đè workbook trước. Kiểm tra toàn bộ bounded values/formulas kể cả ô trống tại data A1:H75 và photo A1:H5. Không thay giá trị/công thức/style sau import.

| Workbook native | Ô có dữ liệu/công thức | Tổng nhân sự | Lỗi effectiveValue | B6 |
| --- | ---: | ---: | ---: | --- |
| [Phiếu đã chốt](https://docs.google.com/spreadsheets/d/1br9QZFpTK7v4AOmNTY8THTKqlqwLICQIWYvKeFg7Xro/edit) | 98 | 6 | 0 | Mưa |
| [Lần gửi3](https://docs.google.com/spreadsheets/d/1h7BIvfC5ien2x9e4qcRPJAwMEqYT9I9HiLN7dpvCY1Y/edit) | 102 | 6 | 0 | Mưa |
| [Lần gửi1](https://docs.google.com/spreadsheets/d/17bq9kvEavPMIVyiKXQsYI_r8IrsmaNpBbrSEeoKUTLk/edit) | 96 | 3 | 0 | Nắng nóng |
| [Lần gửi2](https://docs.google.com/spreadsheets/d/1QGyy5VfLG0xV-twrdldA-7D4Xk9dOXUA5heyfgjszxo/edit) | 96 | 6 | 0 | Mưa |

Không có `#VALUE!` hoặc lỗi cell khác trong các vùng kiểm tra. Google tính lại tổng nhân sự đúng; ảnh đỏ/xanh đúng nguồn hiển thị trên sheet ảnh. 31 ảnh chụp/render lưu nguyên trạng; ảnh ở vị trí scroll chưa thấy weather không được dùng làm bằng chứng nhãn.

Ảnh chính để anh review:

- Browser: [Nhật ký Mưa](acceptance-evidence/bm02-weather-development-2026-10-12/01-journal-mua.jpg), [phiếu và nút xuất](acceptance-evidence/bm02-weather-development-2026-10-12/03-form-export.jpg), [lựa chọn đã chốt](acceptance-evidence/bm02-weather-development-2026-10-12/04-pinned-form-weather.jpg), [lần1 Nắng nóng desktop](acceptance-evidence/bm02-weather-development-2026-10-12/06b-old1-weather-visible.jpg), [mobile390px](acceptance-evidence/bm02-weather-development-2026-10-12/08b-mobile-weather-visible.jpg), [bản hiện tại Mưa](acceptance-evidence/bm02-weather-development-2026-10-12/09-current3-header.jpg).
- Google lần3: [thời tiết/checklist](acceptance-evidence/bm02-weather-development-2026-10-12/10-google-delivery3-main.jpg), [vật tư12.000](acceptance-evidence/bm02-weather-development-2026-10-12/11-google-delivery3-material.jpg), [phần dưới](acceptance-evidence/bm02-weather-development-2026-10-12/12-google-delivery3-lower.jpg), [ảnh xanh](acceptance-evidence/bm02-weather-development-2026-10-12/13-google-delivery3-photo.jpg).
- Google phiếu: [phần trên](acceptance-evidence/bm02-weather-development-2026-10-12/14-google-form-main.jpg), [vật tư](acceptance-evidence/bm02-weather-development-2026-10-12/15-google-form-material.jpg), [phần dưới](acceptance-evidence/bm02-weather-development-2026-10-12/16-google-form-lower.jpg), [ảnh](acceptance-evidence/bm02-weather-development-2026-10-12/17-google-form-photo.jpg).
- Google lần1: [Nắng nóng](acceptance-evidence/bm02-weather-development-2026-10-12/18-google-old1-main.jpg), [vật tư cũ](acceptance-evidence/bm02-weather-development-2026-10-12/19-google-old1-material.jpg), [phần dưới](acceptance-evidence/bm02-weather-development-2026-10-12/20-google-old1-lower.jpg), [ảnh đỏ gốc](acceptance-evidence/bm02-weather-development-2026-10-12/21-google-old1-photo.jpg).
- Google lần2: [Mưa](acceptance-evidence/bm02-weather-development-2026-10-12/22-google-old2-main.jpg), [vật tư cũ](acceptance-evidence/bm02-weather-development-2026-10-12/23-google-old2-material.jpg), [phần dưới](acceptance-evidence/bm02-weather-development-2026-10-12/24-google-old2-lower.jpg), [ảnh xanh gốc](acceptance-evidence/bm02-weather-development-2026-10-12/25-google-old2-photo.jpg).
- PDF gốc render bằng Poppler125dpi: [lần1](acceptance-evidence/bm02-weather-development-2026-10-12/30-pdf1-nang-nong.png), [lần3](acceptance-evidence/bm02-weather-development-2026-10-12/31-pdf3-mua.png). Nhãn weather đúng; lỗi0% còn tồn tại.

## 7. Phần chưa kiểm chứng và lỗi riêng

Fixture runtime chứng minh `hot/rain`, desktop/mobile và các nút thật ở bảng trên. Bốn mã còn lại, dữ liệu thiếu/custom và pinned override chỉ có source/regression. Ba banner liên kết trong checklist/action có actual-component SSR coverage; browser NOT_RUN vì fixture không có liên kết này. Không tạo thêm hồ sơ chỉ để lấp giới hạn đó. Không nghiệm thu Excel desktop/in/physical camera/high volume/toàn bộ client workflow/Production/các BM khác.

**P2 có sẵn — PDF0%:** [work_report_pdf.go:228](../backend/internal/core/work_report_pdf.go#L228) đọc trực tiếp header `progress_percent`. Snapshot quy trình có header mặc định0 nhưng không có tiến độ được ghi trong structured answers. UI và BM02 chọn đúng dữ liệu đã ghi; PDF coi header0 là giá trị thật. Diff weather chỉ đổi dòng weather, không đổi thuật toán progress. Cả hai PDF thực tế chứng minh0%; dữ liệu chưa bị sửa.

**Đề xuất tiếp theo, chưa triển khai:** PDF dùng cùng nguồn tiến độ đã ghi như detail/BM02. Thiếu giá trị thì in `Chưa ghi tiến độ`, vẫn giữ0% khi thực sự đã ghi0; không thêm field hoặc ghi lại snapshot. Mockup giữ bố cục:

```text
TIẾN ĐỘ GHI NHẬN          THỜI TIẾT
Chưa ghi tiến độ           Nắng nóng
```

Lỗi tiến độ cần review package/duyệt riêng trước sửa; build hoặc weather pass không thay thế gate này.

**Việc tiếp theo:** anh review ảnh và file weather để chốt nghiệm thu phạm vi nhãn. Sau đó quyết định sửa tiến độ PDF theo dữ liệu hiện có; không có công việc triển khai weather còn bỏ dở trong phạm vi đã duyệt.

Bằng chứng máy: [checks](acceptance-evidence/bm02-weather-development-2026-10-12/acceptance-checks.json), [verifier](acceptance-evidence/bm02-weather-development-2026-10-12/verify_weather.py), [source](acceptance-evidence/bm02-weather-development-2026-10-12/source-state.json), [runtime](acceptance-evidence/bm02-weather-development-2026-10-12/runtime-after-restart.json), [download origins](acceptance-evidence/bm02-weather-development-2026-10-12/browser-downloads.json). Audit độc lập chỉ đọc source/evidence đã lưu, không phải một lần browser nghiệm thu thứ hai.

Verifier phụ thuộc helper/session tạm tại `/private/tmp/consflow-bm02-receipt-mapping-uat.py`, runtime Python có openpyxl/pypdf và tập evidence đã lưu. Script có renew phiên đăng nhập bình thường và ghi JSON evidence; không ghi dữ liệu nghiệp vụ hoặc sửa workbook. Đây không phải một script standalone có thể chạy ở máy khác mà không chuẩn bị phụ thuộc.
