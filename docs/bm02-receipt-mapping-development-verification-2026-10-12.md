# BM02 — Sửa mapping vật tư và nghiệm thu Development, 12-10-2026

**Trạng thái: IMPLEMENTED / TECHNICALLY VERIFIED / UX ACCEPTED cho mapping vật tư BM02 trong kịch bản Development dùng dữ liệu thử.** Anh đã xác nhận ngày12-10-2026: “Chốt nghiệm thu mapping vật tư BM02.” Phiếu phiên bản 3 và lần gửi 3 tự lấy đúng vật tư nhận trường 12 m²; lần gửi 1 và 2 giữ nguyên nội dung và ảnh. Không thêm field, UI, schema hoặc migration. Các giới hạn bên dưới vẫn áp dụng; bằng chứng máy đọc giữ trạng thái tại lúc chụp trước khi anh chốt.

Giới hạn: kiểm tra này không chốt nghiệm thu tất cả biểu mẫu hoặc Production. Nhãn thời tiết vẫn hiện `hot/rain`, là lỗi hiển thị riêng đã ghi nhận trước đó. Mobile, Excel desktop/in ấn, camera thực tế, giới hạn dung lượng qua trình duyệt và toàn bộ quy trình client chưa chạy trong lượt này.

## Phạm vi đã duyệt và nguyên nhân

Yêu cầu: “Duyệt 1–3. Sửa mapping vật tư BM02, không thêm field. Cập nhật development, nghiệm thu lại file tải từ nút xuất trên trình duyệt và Google Sheets, chụp hình để tôi review.”

Trước sửa, phiếu nhận trường và phiếu báo cáo có hai Đầu việc sở hữu hợp lệ khác nhau. Query cũ yêu cầu cùng Đầu việc, nên bỏ sót phiếu nhận 12 m² dù đúng công trình/ngày và đã gửi. Lỗi đã tái hiện bằng luồng nghiệp vụ thật ở Development và regression test gọi các Service chuẩn. Đây là lỗi mapping; không cần nhập lại vật tư hoặc tạo Đầu việc lai.

Sau sửa:

1. Khi chốt/sửa phiếu, BM02 lấy phiếu nhận đã gửi theo công trình và ngày báo cáo, chỉ lấy nguồn mà actor có quyền xem. Kiểm tra quyền ở nguồn trước giới hạn 200 dòng; vượt giới hạn báo lỗi rõ ràng.
2. Giữ nguyên người sở hữu, mã phiếu/dòng nguồn, vật tư, đơn vị, ghi chú và khối lượng giao. Không lấy khối lượng nghiệm thu thay cho khối lượng nhập. Danh sách nhập trường do người dùng điền sẵn vẫn được ưu tiên.
3. Báo cáo từ quy trình khi gửi/gửi lại dùng dữ liệu đã đóng băng của đúng phiên bản phiếu đã chốt. Phiếu nhận phát sinh sau lúc chốt cần một lần sửa phiếu mới để được đưa vào. Lần gửi cũ không được bổ sung hồi tố. Báo cáo thủ công vẫn ghi nhận tại lúc gửi; đọc BM06 hiện tại vẫn dùng trạng thái công việc hiện tại.

## Thành phần thay đổi

| Thành phần | Vai trò |
| --- | --- |
| `backend/internal/core/native_daily_source_reuse.go` | Query phiếu nhận theo công trình/ngày; quyền theo từng nguồn, giới hạn và dữ liệu nguồn |
| `backend/internal/core/native_workflow_source_reuse.go` | Khôi phục đúng dữ liệu phiếu đã chốt cho bản gửi và điều kiện gửi lại |
| `backend/internal/core/native_daily_source_reuse_test.go` | Regression quyền, ngày/công trình/trạng thái, ghi nhận thủ công và giới hạn |
| `backend/internal/core/native_receipt_canonical_test.go` | Luồng nhận trường → chốt/sửa phiếu → gửi/gửi lại; giữ lịch sử và nguồn ảnh |

Entrypoint trong `native_work_report_evidence.go` dùng enrichment đã sửa, không cần chỉnh thêm. `repository_incoming_acceptance.go` giữ nguyên quy tắc Đầu việc sở hữu của phiếu nhận; không gọi helper tái sử dụng mới và không bị sửa. Không thay đổi frontend. Các map trung tâm/backend/triage và Construction/Workflow/Materials/Reporting đã cập nhật trong cùng lượt.

Triển khai: GPT-6.1 Sol Medium. Review độc lập: GPT-6.1 Sol High, read-only. Reviewer phát hiện và yêu cầu sửa hai lỗi trong bản nháp: nguồn không có Đầu việc bị loại và điều kiện gửi lại khác dữ liệu thực tế lúc gửi. Cả hai đã sửa và có regression tái hiện.

## Môi trường và kiểm tra

HEAD `75dc44707785d399ed2408959bf784928a544e96`; kiểm tra bản working tree chưa commit, giữ nguyên thay đổi có sẵn. Manifest 963 file nguồn/cấu hình/test: SHA256 `c15991c8ece89b0dc195541373ec34a2a2f67f7094fa53b2331982c626cea543`. So với baseline nghiệm thu trước: sửa ba file có sẵn trong baseline và thêm một test. Tất cả hash vẫn khớp sau khi nghiệm thu.

Development: UI `http://127.0.0.1:5173`, API cổng 8081. Giữ tiến trình Java đang dùng cổng 8080; cấu hình ghi trong task env riêng, không sửa env repository.

| Kiểm tra thực tế | Kết quả |
| --- | --- |
| 4 focused regression tests | PASS, 10.365 giây |
| Backend `test ./... -count=1` | PASS; core 257.783 giây, migrations 81.429 giây |
| Backend `vet ./...` | Exit 0 |
| Backend `mod verify` | All modules verified |
| `gofmt -l` trên bốn file thay đổi | Không có output |
| `git diff --check` | Exit 0 |
| `make restart ENV_FILE=/private/tmp/consflow-bm02-development-20261012.env` | PASS, đúng down → build → run |
| Health và ready sau restart | HTTP 200; database ready |
| Verifier dữ liệu/file/source/Google | PASS 68 kiểm tra; user visual review PENDING |

Backend tests dùng DB riêng `consflow_bm02_receipt_test_20261012`, mỗi test dùng schema biệt lập. Runner tạm cung cấp URL và cache riêng mà không hiển thị credentials. Không chạy fixture-test ghi dữ liệu vào DB Development `consflow`.

Lần restart đầu dừng sau down do sandbox chặn file activity của Docker Buildx. Chạy lại chính pipeline trên với escalation chỉ cho lệnh này đã thành công. Backend/admin khởi động `2026-10-12T07:12:53Z`; health/ready kiểm tra lúc `07:15:19Z` (UTC).

- Backend image: `sha256:87e3844a1dab7ae5d927d853a197527d08a62235ba37ea96187243f50b073301`.
- Admin image: `sha256:e78b842b8543863178f1a01ff224b6ee7dfd9d7c65a3adc0da060fd6d6b61be9`.
- DB Development: 157 migrations; 6 công trình và 21 workflow form instances trước/sau giữ nguyên.

Không có source frontend mới để cần build riêng; pipeline chuẩn đã build container admin. Không chạy migration mới, Production, reset DB, commit/push hoặc đóng plan.

## Kịch bản Development và lịch sử

Tận dụng lại công trình thử `DEV-BM02-WORKFLOW-20261012`, phiếu quy trình v5 có 11 field, phiếu nhận đã gửi và người nhận thử hợp lệ. Không tạo công trình/phiếu nhận/form mới trong lượt sửa này.

| Bản | Khối lượng thi công | Nhân lực | Ảnh | Vật tư nhập trường |
| --- | --- | --- | --- | --- |
| Lần gửi 1 đã lưu | 2 m² | 3 người / 8 giờ | Đỏ, photo1 | Giữ nguyên thiếu nguồn lúc gửi |
| Lần gửi 2 đã lưu | 9 m² | 6 người / 7 giờ | Xanh, photo2 | Giữ nguyên thiếu nguồn lúc gửi |
| Phiếu phiên bản 3 / lần gửi 3 | 9 m² | 6 người / 7 giờ | Cùng ảnh xanh photo2 | Tự lấy đúng 12 m² từ phiếu nhận |

Người nhận yêu cầu bổ sung ở lần gửi 2 qua API chuẩn. Tác giả sửa cùng phiếu, dùng `expected_revision=2`; chỉ thay một ghi chú có sẵn để tạo phiên bản 3, không nhập danh sách vật tư. Phiếu mới tự ghi nhận vật tư nguồn. Tác giả bấm nút gửi lại trên trình duyệt để tạo lần gửi 3.

Ảnh xanh được tái sử dụng nguyên bản, nên chữ bên trong ảnh vẫn ghi “Lần 2”. Caption bản gửi ghi đúng lần gửi 3. Không phải ảnh mới hoặc ảnh bị đổi nguồn.

Ở hai file mới: `D12` ghi tên vật tư, số lượng và đơn vị; `E26` ghi tên vật tư; `G26` giữ đúng chuỗi NUMERIC nguồn `12.000`, tương đương 12 m² theo đối chiếu Decimal; không chứng nhận ô này là số để tính toán Excel. `F26` để trống khi không có quy cách/xuất xứ; không tự dùng nhà cung cấp làm xuất xứ. Tổng nhân lực `C28` tính đúng 6. Không có dữ liệu đánh giá thì trạng thái vẫn “Chưa đánh giá”.

Toàn bộ record lần gửi cũ, bốn event phiếu cũ và record phiếu nhận không thay đổi. Đối chiếu MD5 các row cũ trong tám bảng giữ nguyên ngoài ba record fixture được phép cập nhật: Đầu việc báo cáo, form instance và daily report. Đây là đối chiếu có phạm vi; không phải chứng nhận toàn bộ DB byte-equal.

XLSX cũ chỉ khác thứ tự serialization MIME/shared strings và các chỉ số tham chiếu chuỗi. Sau khi resolve chuỗi, toàn bộ giá trị/kiểu ô/formula/style/geometry/drawing reference khớp; các phần khác và media khớp bytes. Không tuyên bố hai ZIP có SHA256 giống nhau.

## File tải thật và Google Sheets

Cả năm file được tải từ nút trên trình duyệt sau cập nhật Development. Bản lưu bằng chứng khớp bytes với file Download gốc. Không sửa workbook sau tải, không sửa ô/kích thước/ảnh trong Google Sheets.

| File tải | Nút / nguồn | Google kiểm tra |
| --- | --- | --- |
| [Lần gửi 2 trước sửa phiếu](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/BM02-lan-gui-2-truoc-sua-phieu.xlsx) | Chi tiết lần gửi 2 → Xuất báo cáo | Đối chiếu file lịch sử trước/sau |
| [Phiếu phiên bản 3](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/BM02-phieu-phien-ban-3.xlsx) | Phiếu quy trình → Xuất báo cáo ngày | [Google Sheets phiếu 3](https://docs.google.com/spreadsheets/d/1rMYbBNEvp8DDf2DV1LR2aJlPgW-v6UDVVbJW45gDTKs/edit) |
| [Lần gửi 3](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/BM02-lan-gui-3.xlsx) | Chi tiết lần gửi 3 → Xuất báo cáo | [Google Sheets lần gửi 3](https://docs.google.com/spreadsheets/d/1RwfD8LklMgwjCUkhOGt6UKXLUb03BY4RdTbY2QnTtkk/edit) |
| [Lần gửi 1 giữ nguyên](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/BM02-lan-gui-1-giu-nguyen.xlsx) | Các lần gửi → Lần 1 → Xem nội dung → Xuất báo cáo | [Google Sheets lần gửi 1](https://docs.google.com/spreadsheets/d/1StfFCHtXHX57clmzrQVIFuk1-yRLkobqlXCTzK5bR8E/edit) |
| [Lần gửi 2 giữ nguyên](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/BM02-lan-gui-2-giu-nguyen.xlsx) | Các lần gửi → Lần 2 → Xem nội dung → Xuất báo cáo | [Google Sheets lần gửi 2](https://docs.google.com/spreadsheets/d/1dd_G_qeaDszNOPGwGNdGa3g9NeQcENCAN6cZ3R7OIbI/edit) |

Bốn import native riêng tư: dữ liệu `2026-10-12_01!A1:H75` và `Hình ảnh!A1:H5` khớp tất cả ô/formula đọc trong phạm vi, gồm 98/102/96/96 ô có dữ liệu. Không có `effectiveValue.errorValue`, gồm `#VALUE!`; tổng nhân lực được Sheets tính lại đúng 6/6/3/6. Quan sát trực tiếp cả sheet dữ liệu và sheet ảnh ở 100%; nội dung vật tư và ảnh hiển thị trong các ảnh chụp dưới đây. BM02 giữ sheet “Hình ảnh” trong cùng workbook theo cách anh đã duyệt; không thêm sheet mới trong lượt sửa mapping.

## Ảnh chụp để review

23 ảnh thực tế, không chỉnh sửa; hash nằm trong `acceptance-checks.json`.

| Kịch bản | Ảnh |
| --- | --- |
| Nút phiếu và gửi lại | [02 — Xuất báo cáo ngày](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/02-form3-export.jpg), [03 — Gửi lại thành công](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/03-resubmitted-delivery3.jpg) |
| Chi tiết lần gửi / lịch sử | [01 — Lần 2 trước sửa](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/01-old-delivery2-before.jpg), [04 — Xuất lần 3](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/04-delivery3-export.jpg), [05 — Ba lần gửi](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/05-three-revisions.jpg), [06 — Xuất lại lần 2](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/06-old-delivery2-after.jpg), [07 — Xuất lại lần 1](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/07-old-delivery1-after.jpg) |
| Google phiếu 3 | [08 — Đầu mẫu](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/08-google-form3-main.jpg), [09 — Vật tư](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/09-google-form3-material.jpg), [10 — Cuối mẫu](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/10-google-form3-lower.jpg), [11 — Ảnh](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/11-google-form3-photo.jpg) |
| Google lần gửi 3 | [12 — Đầu mẫu](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/12-google-delivery3-main.jpg), [13 — Vật tư 12 m²](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/13-google-delivery3-material.jpg), [14 — Cuối mẫu](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/14-google-delivery3-lower.jpg), [15 — Ảnh xanh](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/15-google-delivery3-photo.jpg) |
| Google lần gửi 1 cũ | [16 — Đầu mẫu](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/16-google-old1-main.jpg), [17 — Vật tư giữ nguyên](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/17-google-old1-material.jpg), [18 — Cuối mẫu](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/18-google-old1-lower.jpg), [19 — Ảnh đỏ](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/19-google-old1-photo.jpg) |
| Google lần gửi 2 cũ | [20 — Đầu mẫu](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/20-google-old2-main.jpg), [21 — Vật tư giữ nguyên](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/21-google-old2-material.jpg), [22 — Cuối mẫu](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/22-google-old2-lower.jpg), [23 — Ảnh xanh](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/23-google-old2-photo.jpg) |

## Ranh giới chứng cứ và bước tiếp theo

Việc phiếu nhận phát sinh sau lúc chốt, nguồn không có Đầu việc, từ chối quyền, 200/201 dòng và khác biệt dữ liệu đọc BM06/gửi lại được kiểm tra bằng regression canonical trên DB test riêng. Không tạo thêm fixture cạnh tranh/giới hạn qua trình duyệt Development để tuyên bố đã chạy UAT các trường hợp đó.

Verifier chỉ đọc record/file/Google JSON và ghi bằng chứng, có đăng nhập lại thông thường cho hai tài khoản thử. Không chỉnh XLSX/Google, bypass guard hoặc sửa dữ liệu lịch sử. Kiểm tra độc lập dựa trên source và bằng chứng đã lưu; không phải phiên browser nghiệm thu thứ hai do reviewer tự chạy.

**Bước tiếp theo:** mapping vật tư đã được anh chốt. Lập kế hoạch riêng để hiển thị nhãn thời tiết tiếng Việt bằng dữ liệu hiện có, không thêm field. Lỗi nhãn thời tiết và các kiểm tra chưa chạy giữ trạng thái riêng; chưa triển khai thay đổi nhãn khi chưa duyệt kế hoạch mới.

Bằng chứng máy đọc: [acceptance-checks.json](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/acceptance-checks.json), [browser-downloads.json](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/browser-downloads.json), [runtime-after-restart.json](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/runtime-after-restart.json), [source-state.json](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/source-state.json). [Audit độc lập](acceptance-evidence/bm02-receipt-mapping-development-2026-10-12/independent-audit.md) không có finding chặn trong phạm vi đã duyệt; sửa câu mô tả incoming repository theo audit, không thay đổi ứng dụng.
