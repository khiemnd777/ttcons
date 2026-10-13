# BM02 — Mapping dữ liệu hiện có và nghiệm thu development, 12-10-2026

**Kết quả:** mapping BM02 đã triển khai, không thêm field. Năm file tải từ các nút hiện có đã được nhập nguyên trạng vào Google Sheets và kiểm tra dữ liệu/hiển thị. Dữ liệu checklist cùng ngày khớp giữa các điểm xuất; ghi chú nằm trong bảng 12 tiêu chí của mẫu gốc.

**Giới hạn:** đây là nghiệm thu kỹ thuật trên dữ liệu thử development. Anh chưa duyệt UX. Nút xuất lần gửi lịch sử chưa chạy trên trình duyệt vì không có hồ sơ đã gửi/người nhận hợp lệ. Microsoft Excel, in giấy, mobile, ảnh công trường thực và dữ liệu ở giới hạn tối đa chưa kiểm tra. Không coi lượt này là nghiệm thu toàn bộ tám BM hoặc luồng thi công hoàn chỉnh.

**Lỗi hiển thị có sẵn còn lại:** thẻ Nhật ký ngày09 vẫn ghi0% mặc dù nguồn workflow chưa ghi tiến độ. `OperationsPage.tsx:372` dùng `Number(row.progress_percent ?? 0)` trực tiếp; `ConstructionReportDetailDialog.tsx:244` đã dùng `report.hasProgress`, giống ngữ nghĩa file BM02 mới. Đã quan sát cả hai trên browser. Lỗi thẻ Nhật ký không làm đổi file xuất, nhưng ngăn kết luận đồng nhất toàn bộ UI tiến độ. Không sửa frontend trong lượt mapping này. Đề xuất tận dụng `constructionReportViewModel(row)` đang có để thẻ dùng cùng `hasProgress`/`progressPercent`, không thêm field; UI vẫn cần review gate riêng.

**Cập nhật sau lượt mapping, 12-10-2026:** anh đã duyệt mockup riêng cho thẻ Nhật ký. Lỗi trên đã được sửa và kiểm chứng trên project/global journal, desktop/mobile và file tải thật; không thêm field. Xem [Nghiệm thu sửa thẻ Nhật ký](journal-progress-development-verification-2026-10-12.md). Kết quả và source snapshot của lượt mapping trước vẫn được giữ nguyên để truy vết.

## Phạm vi đã duyệt và nguyên nhân

Mapper cũ chỉ đưa `supervision_assessments` vào checklist. Nhân lực, khối lượng, rủi ro và kế hoạch đã có trong nguồn nhưng chỉ xuất ở phần chi tiết phía dưới. Bản sửa đưa các sự kiện này vào ghi chú tương ứng, giữ ghi chú/đánh giá nhập tay, dùng `Chưa đánh giá` khi thiếu đánh giá. Không tự suy ra Đạt từ ảnh, task hoặc khối lượng.

Owner: `bm02_projection.go`, `bm_gs_export_projection.go`, `bm_gs_workbook.go`, `service_native_bm_gs_exports.go` và hai file regression tests. Dùng nguồn đã lưu/current/exact snapshot và kết quả membership ảnh được bảo vệ; không thêm truy vấn liên module, field, migration hoặc giao diện chọn mẫu/ảnh. Source sections giữ đầy đủ dữ liệu khi ghi chú được rút gọn. Vật tư sử dụng không biến thành vật tư nhập trường; workflow default0 không biến thành tiến độ đã ghi.

Matched domains: Construction Operations và Reporting; central/backend/frontend/triage/domain maps đã cập nhật. First searches: `CriteriaRows|supervision_assessments|nativeDailyPhotos|nativeWorkflowDailyExportRow`. Giữ các nút/component xuất hiện có, gồm `NativeExcelExportButton`; không sửa frontend hoặc trải nghiệm upload/preview.

## Nguồn và môi trường đã kiểm tra

- HEAD `75dc44707785d399ed2408959bf784928a544e96`, working tree có nhiều thay đổi trước đó được giữ nguyên. Bản sửa BM02 tác động sáu file backend source/test. Manifest 961 file: SHA256 `b6a13d0eb6b737f0da50243a868fa4515ae7c45cb14a574a5b83a4feb9e40a85`. Hash nguồn không đổi từ trước build tới sau nghiệm thu.
- `make restart` giữ đúng down→build→run. Lần đầu build thành công nhưng startup thất bại vì Java của pinky-lamb-cms chiếm host8080. Lần chạy lại thành công với env riêng `/private/tmp/consflow-bm02-development-20261012.env`: UI5173/API8081; `.env` repository không đổi. Không dừng ứng dụng Java.
- Backend image `sha256:177987f90d4cd080c3d77923d4aaa86e1ddfe1fb19bb832e52ec84faa61a1f7c`, container tạo `2026-10-12T02:20:30.0889753Z`. Frontend image `sha256:103659a5a3f40f2ec738c613a148996ec279ad5f34a1a2c2392dbccd59758e18`.
- UI `http://127.0.0.1:5173/` và API `/api/health`, `/api/ready` tại8081 trả HTTP200. Database `consflow`, 157 migrations/maxV157, năm projects và 20 form instances trước/sau. Không có migration mới hoặc sửa hồ sơ để làm đầy file. Backup trước rebuild ở `/private/tmp/consflow-before-bm02-mapping-20261012.dump`; không chạy restore rehearsal vì không thực hiện thao tác phá hủy dữ liệu.
- Công trình thử `DEV-BM-REUSE-20261009`, ID `d2398f6e-83db-4171-a729-fc2e2e1557d2`. Phiếu `35416810-63ab-4175-8340-5be6e13bb51b` vẫn submitted, revision2. Nguồn ngày09 `bd131f5c-ccf0-4c3f-b862-7d2dfc551eb4` có một đội, một công việc, một blocker và một kế hoạch. Nguồn ngày08 `ded0bb8b-39dd-4757-9cef-3c2f87692b1f` có một đội, một công việc, không blocker/kế hoạch. Cả hai dùng ảnh hợp lệ đã liên kết từ trước.

## Nút xuất, file và Google Sheets

| Điểm xuất thực tế | Nút | File XLSX nguyên trạng | Kết quả Google |
|---|---|---|---|
| Đầu việc → phiếu báo cáo thi công | Xuất báo cáo ngày | [bm02-form.xlsx](acceptance-evidence/bm02-mapping-development-2026-10-12/exports/bm02-form.xlsx) | [Phiếu](https://docs.google.com/spreadsheets/d/1rgH_x4gpuu_MKPgjoDevWapX9Y-KVYMQKxH6olqSsts/edit) — một ngày + ảnh |
| Thi công → Nhật ký, hai ngày | Xuất báo cáo | [bm02-journal-two-days.xlsx](acceptance-evidence/bm02-mapping-development-2026-10-12/exports/bm02-journal-two-days.xlsx) | [Hai ngày](https://docs.google.com/spreadsheets/d/1eUc1kdhLiuPQJMLrKIr96noCiZ4wJaFJ-n4brfRe6gY/edit) — hai sheet ngày + ảnh |
| Thi công → mở ngày09 | Xuất báo cáo ngày | [bm02-detail.xlsx](acceptance-evidence/bm02-mapping-development-2026-10-12/exports/bm02-detail.xlsx) | [Chi tiết](https://docs.google.com/spreadsheets/d/11ztXrRu1HpeEotYqrQLq3vErJi1CGkvusSmKSo_y0tY/edit) — một ngày + ảnh |
| Thi công → Nhật ký, lọc riêng ngày08 | Xuất báo cáo | [bm02-journal-filter08.xlsx](acceptance-evidence/bm02-mapping-development-2026-10-12/exports/bm02-journal-filter08.xlsx) | [Bộ lọc](https://docs.google.com/spreadsheets/d/1Wed1RIsLogUpTFmODIYesjZOyr-R0zciLKFESEqMtks/edit) — chỉ ngày08 + ảnh |
| Báo cáo → Nháp tại nguồn | Xuất báo cáo | [bm02-reports-list.xlsx](acceptance-evidence/bm02-mapping-development-2026-10-12/exports/bm02-reports-list.xlsx) | [Danh sách](https://docs.google.com/spreadsheets/d/1Yt2xXvzAG4rK33hxHttuTflIERLixAXq9YXm2mjvsR8/edit) — hai ngày + ảnh |

Các file gốc nằm trong Downloads, có bản sao nguyên byte ở thư mục bằng chứng. SHA256/kích thước/path gốc lưu trong `xlsx-inspection.json`. Công cụ import nhận chính path tải từ trình duyệt; không sửa XLSX sau tải, không sửa ô hoặc kích thước Google để tạo kết quả đẹp hơn. Chỉ chuyển sheet/chọn ô ngoài vùng nội dung để kiểm tra. Google file riêng tư theo tài khoản đang kết nối; không thay quyền chia sẻ.

## Kết quả mapping và hiển thị

| Ô/ngữ nghĩa | Ngày09 | Ngày08 |
|---|---|---|
| D10 — tiến độ | Chưa có ghi nhận; không dùng default0 | 0% thực đã lưu |
| D11 — nhân lực | 3 người, 8 giờ, đại diện/liên hệ | 2 người, 4 giờ |
| D12 — vật tư nhập trường | Chưa có ghi nhận; không dùng hai viên gạch đã sử dụng | Chưa có ghi nhận |
| D14 — khối lượng | Công việc, 2m2, khu vực và ghi chú nguồn | Công việc đã ghi; không bịa số lượng |
| D19 — rủi ro | Blocker, mã, nguyên nhân, ảnh hưởng, mức độ, phụ trách/hạn/trạng thái | Chưa có ghi nhận |
| D20 — ảnh | Một ảnh đúng membership, tên/chú thích/vị trí/giai đoạn đã có | Một ảnh của riêng ngày08 |
| D21 — chỉ dẫn | Kế hoạch, người nhận/liên hệ/hạn, tiến độ thực0%, Đang làm | Chưa có ghi nhận |
| C10:C21 | 12 trạng thái Chưa đánh giá, nền trung tính | 12 trạng thái Chưa đánh giá, nền trung tính |

Ghi chú/đánh giá nhập tay được kiểm chứng bằng regression; hai nguồn development không có đánh giá nhập tay nên không dùng fixture này để tuyên bố nghiệm thu nhập đánh giá trên UI. Chất lượng/MEP/an toàn/nghiệm thu/phát sinh chưa có ghi nhận phù hợp trong nguồn; file ghi rõ thiếu dữ liệu, không tự kết luận.

Bảy sheet ngày và năm sheet ảnh đã xem ở100%. Ghi chú xuống dòng vừa ô, không che ô bên cạnh; ảnh appendix hiển thị đầy đủ theo từng nguồn. Đọc `A1:H75` trên mỗi sheet Google: không có `effectiveValue.errorValue` hoặc `#VALUE!`; toàn bộ 12 cặp trạng thái/ghi chú khớp XLSX. Công thức tổng nhân lực tính3 và2 tương ứng. Nội dung checklist cùng ngày khớp ở mọi điểm xuất. Không thay đổi bố trí ảnh BM07 trong lượt này.

## Kiểm tra và review độc lập

- Root: `GOCACHE=/private/tmp/consflow-bm02-diagnosis-go-cache GOMODCACHE=/private/tmp/consflow-bm02-diagnosis-modcache go test ./internal/core -run 'TestBMGS|TestBM02|TestNative.*(Progress|Projection)' -count=1` PASS2.241s; `go vet ./internal/core` exit0; Docker backend build exit0; `git diff --check` exit0. Không chạy frontend production build vì không sửa nguồn frontend; development frontend đã chạy và được kiểm tra bằng browser.
- GPT-6.1 Sol Medium triển khai. GPT-6.1 Sol High review độc lập phát hiện hai P2: mất đơn vị vật tư từ hàng thứ ba khi rút gọn, và nền xanh gây hiểu nhầm ở trạng thái chưa đánh giá. Đã giữ toàn bộ arrival facts ở source section và chuyển fill unknown sang `F2F2F2`. Reopened-XLSX tests giữ đơn vị dòng thứ ba và kiểm tra style cả12 ô.
- Re-review High: không còn P0–P2 trong delta; focused PASS1.024s; reproduction trước đây thất bại nay PASS0.744s. Manual note/assessment, zero/missing, mixed units, duplicate titles, frozen photo facts và no-source-mutation có regression.
- Audit bằng chứng cuối cùng của High xác nhận961 source hashes, năm XLSX/hash/path gốc, bảy sheet×12 tiêu chí và21 screenshots nhất quán. Đã xem ảnh07/16, không tìm thấy mâu thuẫn trong kết luận mapping giới hạn này. Trường hợp ba arrivals và manual Pass/Fail chỉ có pure-test proof; chưa chạy trên UI development. Số lượng projects/forms không chứng minh toàn bộ database bất biến.

## Bằng chứng và bước tiếp theo

- [Nút xuất trên phiếu](acceptance-evidence/bm02-mapping-development-2026-10-12/screenshots/01-form-export-button.jpg).
- [Checklist và ghi chú đã map](acceptance-evidence/bm02-mapping-development-2026-10-12/screenshots/07-google-form-risk-photo-plan.jpg).
- [Ảnh trong file của phiếu](acceptance-evidence/bm02-mapping-development-2026-10-12/screenshots/08-google-form-linked-photo.jpg).
- Toàn bộ21 screenshots, metadata/readback Google, original/copy hash, source manifest và kiểm tra đối chiếu nằm trong `docs/acceptance-evidence/bm02-mapping-development-2026-10-12/`.

**Bước tiếp theo:** anh review ảnh và file trên để nghiệm thu UX. Phần xuất lần gửi lịch sử cần một hồ sơ đã gửi hợp lệ để nghiệm thu riêng; không gửi báo cáo giả hoặc bỏ guard người nhận. Không commit/push, production, reset, shutdown hoặc đóng plan trong lượt này.
