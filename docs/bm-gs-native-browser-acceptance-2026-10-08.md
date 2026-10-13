# Nghiệm thu trình duyệt — export native BM-GS

Mục đích chính: xuất báo cáo từ hồ sơ đang xem hoặc danh sách đang lọc trong tính năng nghiệp vụ hiện có, với tên nút đã duyệt và ảnh được kế thừa đúng nguồn.

**Kết luận: chưa đạt nghiệm thu toàn bộ.** Nghiệm thu trình duyệt đã chạy đủ 14 điểm xuất. Lỗi tổng BM-GS-04 được sửa sau khi duyệt ngày 2026-10-09; [hai file tải từ nút thật đã qua kiểm tra Google tổng1/đã khắc phục0/đang xử lý1 khi mã lỗi trống](bm-gs-04-verification-2026-10-09.md). Tiêu chí ảnh của bốn file BM01/BM05 đã được rút lại ngày 2026-10-09: hai mẫu client không có cột hoặc vùng đặt ảnh. Tại thời điểm lượt nghiệm thu trình duyệt ban đầu, native Google Sheets, Microsoft Excel và in thực tế chưa chạy. BM07 sau đó đã qua nghiệm thu Google riêng như liên kết dưới đây. BM03/BM08 còn khoảng thiếu và các mẫu khác chưa nghiệm thu Google đầy đủ; không thay kết luận toàn bộ.

**Trạng thái BM07 hiện tại:** sau khi bản rich-value bị Google chuyển thành `#VALUE!`, phương án `oneCellAnchor` đã được duyệt, triển khai và nghiệm thu chính file tải từ nút `Xuất báo cáo`. Bốn ảnh native trong ô Google giữ tỷ lệ và không có lỗi. [Kết quả và giới hạn BM07 tương thích Google](bm-gs-07-google-compatible-verification-2026-10-08.md) là trạng thái mới nhất. Các ảnh preview appendix và kết quả rich-value bên dưới chỉ là bằng chứng lịch sử đã bị thay thế.

Không sửa application source trong lượt nghiệm thu này. Kết quả không phải xác nhận production hoặc nghiệm thu công trình thật.

## Phát hiện và khoảng chưa xác minh

### P2 — BM-GS-04 đếm thiếu lỗi khi mã lỗi trống — đã sửa ngày 2026-10-09

Các dòng sau ghi nhận phát hiện ban đầu. Bản sửa và Google tính lại được lưu trong [BM04 verification](bm-gs-04-verification-2026-10-09.md); không coi công thức cũ dưới đây là trạng thái runtime mới.

- Hai file thực tế từ danh sách và chi tiết đều có `A8=1`, mô tả ở `D8`, trạng thái `I8=Mới`, nhưng `B8` (mã lỗi) trống đúng nguồn.
- Công thức `A13` đếm `COUNTA(B8:B10)`. Theo ngữ nghĩa công thức, tổng lỗi bằng 0, trong khi đang xử lý bằng 1. Đây là kiểm tra công thức và dữ liệu file; chưa có spreadsheet engine tính lại/hiển thị kết quả.
- Nguyên nhân đã xác minh: [renderer:296](/Users/khiemnguyen/Works/ttcons/ttcons/backend/internal/core/bm_gs_workbook.go:296) đếm mã lỗi optional. STT cột A được renderer cấp cho mỗi dòng tại [renderer:204](/Users/khiemnguyen/Works/ttcons/ttcons/backend/internal/core/bm_gs_workbook.go:204). [Projection:246](/Users/khiemnguyen/Works/ttcons/ttcons/backend/internal/core/bm_gs_export_projection.go:246) giữ mã lỗi chưa ghi ở trạng thái trống, phù hợp dữ liệu nguồn.
- Workbook gốc chứa hướng dẫn dạng text về việc đếm mã. Renderer chuyển hướng dẫn đó thành công thức thật mà chưa thích ứng với mã optional của hồ sơ native. Không sửa mã nguồn hoặc bổ sung mã lỗi giả để che lỗi đếm.
- GPT-6.1 Sol High xác nhận phát hiện. Không thấy lỗi đếm cùng dạng trong BM03/05/06/07/08 ở phạm vi công thức đã đọc. Đây không phải tuyên bố mọi công thức của mọi mẫu đã được tính lại.

### Đính chính — BM01/BM05 không yêu cầu ảnh trong biểu mẫu client

- `01-measurement-form`, `01-survey-form`, `05-survey-context`, `05-risk-form` đều có 0 ảnh nhúng trong file đã tải. Cùng Đầu việc trên UI có một ảnh trong Sổ minh chứng.
- [Nguồn phiếu:65](/Users/khiemnguyen/Works/ttcons/ttcons/backend/internal/core/service_native_bm_gs_owners.go:65) chỉ lấy tham chiếu evidence được schema đã pin cho phép. Ảnh từ EvidencePanel được lưu với quan hệ đến Work Item; không có form ID trong quan hệ đó. Key JSON tùy ý ngoài schema không được xem là liên kết chuẩn.
- Đối chiếu lại workbook client ngày 2026-10-09: TOC A8:C8/A12:C12, BM01 A9:G9 và BM05 A7:K7. Cả hai mẫu không có cột/vùng đặt ảnh, không có ảnh nhúng. BM01 G11 chỉ là ghi chú text `Đã chụp ảnh hiện trạng làm bằng chứng`.
- Phát biểu trước đây rằng đây là khoảng thiếu ảnh chặn nghiệm thu là sai: agent đã biến khả năng appendix ảnh bổ sung thành yêu cầu bắt buộc của client. Rút lại phát biểu và tiêu chí đó; bốn file không có ảnh không phải lỗi tuân thủ hai mẫu.
- Đề xuất mở thêm quan hệ ảnh của phiếu cũng được rút lại. Giữ nguyên các quan sát nguồn và file ở trên, không thay code/quan hệ dữ liệu để giải quyết một tiêu chí không có trong mẫu. BM07 là phạm vi ảnh trong ô đã được yêu cầu và nghiệm thu riêng.

## Môi trường và source đã kiểm tra

- Ngày nghiệm thu: 08-10-2026, macOS arm64, trình duyệt Codex IAB. Desktop 1440×1000; mobile 390×844; một phần Operations ban đầu chạy ở viewport mặc định 570px. Đây là kiểm tra responsive bằng viewport, không phải thiết bị di động thật hay kiểm tra đa trình duyệt.
- HEAD `75dc44707785d399ed2408959bf784928a544e96`, branch `main`, working tree chưa commit. 98 file backend/frontend khớp các manifest đã chạy tests/build; không có thay đổi application source sau các kiểm tra đó.
- Combined fingerprint hiện tại: `e1d955fd14749b31b5b06171d1d0b763a065b23e8287a9dae5df902e7c45c0f7`. Backend: `4e47c6b50c3c8d871454afecd058b506e5c730e7c9d470207208d87f312d892d`. Frontend: `aceb30e7d9589baaf2461ae7d904f0afa694dab9626ac5b1d935599dcbabe6ff`. [Manifest source](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm-gs-native-2026-10-08/tested-source-state.json) xác minh lúc 13:30:52 UTC.
- API `http://127.0.0.1:18082/api`, UI `http://127.0.0.1:15174`. Fresh temporary Go server build exit0 trước kiểm tra; Vite 6.4.3 khởi động từ source hiện tại. Launcher dùng routes, JWT, RBAC và storage thật; không khởi tạo các background worker/observability của server thông thường.
- PostgreSQL 16 task DB `bmgs_native_browser_20261008`, clone từ DB UAT tổng hợp cũ V152. Custom migration runner đã áp dụng V153–V156; DB có 156 migration. DB UAT cũ và DB thông thường/production không bị thay đổi. Redis task DB2/3, protected storage được copy sang thư mục task mới.
- Sáu phiếu mới được lưu qua `SaveWorkflowFormDraft`; báo cáo ngày mới qua `CreateDailyReport`. SQL chỉ thiết lập Work Item/Run Step tổng hợp trong ngữ cảnh thi công. Phiếu vẫn là draft; đây không phải kiểm chứng toàn bộ luồng publish → progression → submit → nghiệm thu thật.
- Hash snapshot của lần gửi cũ, hai dòng báo cáo cũ và pin/answers của ba phiếu cũ khớp DB UAT gốc. Không viết lại lịch sử để tạo dữ liệu nghiệm thu. [Manifest fixture](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm-gs-native-2026-10-08/fixture-ids.json) ghi nguồn tổng hợp và giới hạn này.
- FE575/575 và TypeScript/Vite build đã pass trên cùng source manifest trong lượt đổi nhãn; không chạy lại trong lượt browser. Fresh runtime build/migration và nghiệm thu browser/download là bằng chứng bổ sung, không thay thế spreadsheet rendering hay production UAT.

## Ma trận 14 điểm xuất đã duyệt

`Đã tải` xác nhận thao tác trên UI và file XLSX thực tế, không đồng nghĩa toàn bộ mẫu đã được nghiệm thu.

| # | Mẫu và tính năng | Nhãn / vị trí quan sát | File thực tế | Kết quả / giới hạn |
|---|---|---|---|---|
| 1 | BM01 — phiếu khảo sát/đo hiện trạng | Xuất báo cáo; đầu phiếu | 01-measurement-form, 01-survey-form | Đã tải; dữ liệu đo/MEP/hiện trạng đúng nguồn; mẫu client không yêu cầu ảnh |
| 2 | BM02 — Thi công → Nhật ký | Xuất báo cáo; trong nhóm lọc ngày | 02-list-all, 02-list-filter07 | Đã tải; file đầu có 07/08, lọc 07 chỉ có 07; danh sách trống khóa nút |
| 3 | BM02 — chi tiết báo cáo ngày | Xuất báo cáo ngày; trước Công việc trong ngày | 02-detail-08, 02-detail-09 | Đã tải; đúng ngày và tiến độ; ngày09 giữ 0% |
| 4 | BM02 — Báo cáo → danh sách | Xuất báo cáo; header trang | 02-workreports-list | Đã tải đúng phạm vi đang xem; Xuất sổ đăng ký giữ riêng |
| 5 | BM02 — chi tiết lần gửi | Xuất báo cáo; footer dialog | 02-workreport-revision1 | Đã tải ngày07, lần1, 20%, đúng source ID của snapshot |
| 6 | BM02 — phiếu báo cáo thi công | Xuất báo cáo ngày; đầu phiếu | 02-daily-workflow; bản mobile390 | Đã tải từ answers đã lưu; nội dung/print/formulas/anchors của hai bản khớp |
| 7 | BM03 — checklist/gói nghiệm thu thầu phụ | Xuất báo cáo; đầu phiếu | 03-subcontractor-checklist, 03-subcontractor-package | Đã tải; thiết kế/đo thực tế riêng; done không tự thành Đạt |
| 8 | BM04 — Cần xử lý → danh sách | Xuất báo cáo; cạnh nhóm queue trên danh sách | 04-list | Đã tải; lỗi công thức tổng còn tồn tại |
| 9 | BM04 — chi tiết việc xử lý | Xuất báo cáo; trước Mô tả hiện trạng | 04-detail | Đã tải; cùng lỗi công thức tổng |
| 10 | BM05 — phiếu đánh giá rủi ro | Xuất báo cáo; đầu phiếu | 05-risk-form | Đã tải; facts rủi ro đúng nguồn; mẫu client không yêu cầu ảnh |
| 11 | BM05 — khảo sát/phiếu thi công/chi tiết ngày | Xuất báo cáo rủi ro; đầu phiếu hoặc Công việc trong ngày | 05-survey-context, 05-daily-workflow, 05-detail-09 | Đã tải; đúng dòng risk/blocker; nút ẩn khi không có blocker; mẫu client không yêu cầu ảnh |
| 12 | BM06 — phiếu thi công/chi tiết ngày | Xuất báo cáo hiện trường; đầu phiếu hoặc Công việc trong ngày | 06-daily-workflow, 06-detail-08, 06-detail-09 | Đã tải; đúng kế hoạch/người nhận; số0 được giữ |
| 13 | BM07 — Đầu việc → Minh chứng | Xuất báo cáo; dưới thống kê ảnh/tài liệu trong Sổ minh chứng | 07-evidence-register | Đã tải; ảnh nhúng từ Work Item và capture metadata đúng nguồn |
| 14 | BM08 — Bàn giao → hồ sơ đang xem | Xuất báo cáo; trên chi tiết hồ sơ | 08-handover | Đã tải HO-UAT-01; xác thực nguồn không tự suy ra kết quả vật lý/kết luận |

Tất cả 14 điểm được kiểm tra ở desktop và giao diện mobile. Sáu loại phiếu có bằng chứng tọa độ nhãn/nút ở 390px trong [responsive evidence](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm-gs-native-2026-10-08/responsive-evidence.json). Mobile BM02 đã bấm tải trực tiếp. Không tuyên bố đã bấm lại mọi export ở mọi viewport.

## Các trạng thái bổ sung đã quan sát

1. Nhật ký: nhập/apply ngày07 → một báo cáo; apply ngày31/10 → không có báo cáo và export disabled. Điều khiển ngày được nhập bằng phím native để phát sinh change; thao tác fill của công cụ đơn thuần chưa cập nhật React state và không được dùng làm bằng chứng lỗi app.
2. Cần xử lý: queue Chờ xác nhận0 → danh sách trống và export disabled.
3. Chi tiết ngày08: không blocker → không có nút rủi ro. Ngày09 có blocker/kế hoạch → đủ ba nút, ba file đúng nguồn; progress0 không bị biến thành thiếu hoặc dùng mặc định khác.
4. Phiếu đo: sửa một trường nhưng chưa lưu → nút đầu phiếu disabled, có hướng dẫn `Lưu thay đổi trước khi xuất Excel.` Reload bỏ thay đổi thử, khôi phục đúng dữ liệu đã lưu và bật lại export. Không bấm lưu thay đổi thử.
5. Tài khoản Viewer: Nhật ký không có nguồn được trả về, export disabled. Cần xử lý hiển thị một action nhưng export bị từ chối với `Không có báo cáo trong phạm vi lọc`; không download. Toast lỗi có nút đóng, đóng được và nút xuất được bật lại. Đây là quan sát scope cụ thể, không phải toàn bộ ma trận role/permission/revocation.
6. Một batch navigation mobile chuyển về đăng nhập trước phiếu cuối. Đăng nhập lại và kiểm tra phiếu cuối thành công. Chưa xác định nguyên nhân hoặc kiểm chứng độ bền phiên. Không quy kết là lỗi nghiệp vụ export.

Không ép latency/offline để chứng minh mọi loading state; không thay ảnh gốc, không upload ảnh mới từ giao diện, không chạy production migration hoặc thao tác nghiệm thu chất lượng công trình.

## Đối chiếu workbook và ảnh

- 23 file download, đủ tám mã BM-GS; tổng 333259 bytes. Có duplicate download để đối chiếu mobile/desktop, không phải 23 nguồn độc lập.
- 246 kiểm tra đã định nghĩa về cells/source/revision/filter/media/provenance đạt. Lỗi BM04 được ghi riêng ngoài tập pass; không coi 246 pass là workbook acceptance toàn diện.
- 20 picture anchors và 19 media parts cộng qua các file. Hai ảnh theo hai source trong workbook nhiều ngày dùng một media deduplicated; caption giữ source identity riêng.
- Ảnh giả lập gốc PNG1400×900,6069 bytes; bản xuất PNG1280×822,5244 bytes. Không crop/upscale theo kích thước đã quan sát. Giảm khoảng13,6% byte trên ảnh giả lập này; không suy rộng thành mức nén mọi ảnh khách hàng.
- Bốn file phiếu BM01/BM05 không có media; đây không phải lỗi vì mẫu client không yêu cầu ảnh. 19 file còn lại có media. Giữ nguyên original/checksum; không cần chọn ảnh hoặc upload lại tại export.
- BM03 giữ hai cặp lượng thiết kế/đo riêng: 24/23,5 và 10/9,5. Chỉ trạng thái thực hiện không phải kết luận chất lượng. BM08 giữ thông tin vật lý/kết luận chưa ghi ở trạng thái trống.
- Chưa chạy Google Sheets để tính lại công thức, xem ảnh, kiểm tra tràn trang/row heights hoặc in. ZIP/openpyxl và source audit không chứng minh các tiêu chí đó.

[Danh sách download](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm-gs-native-2026-10-08/browser-downloads.json), [kết quả từng check](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm-gs-native-2026-10-08/expectation-checks.json), [cells/hash/media từng file](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm-gs-native-2026-10-08/workbook-inspection.json), [report workbook](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm-gs-native-2026-10-08/final-workbook-report.json).

## Ảnh giao diện thực tế

Phiếu thi công desktop, dùng dữ liệu đã lưu:

![Ba nút xuất trên phiếu thi công desktop](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm-gs-native-2026-10-08/daily-form-desktop.jpg)

Ba nút trên mobile390px:

![Ba nút xuất trên phiếu thi công mobile](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm-gs-native-2026-10-08/daily-three-exports-mobile390.jpg)

Nút xuất đúng lần gửi ở footer mobile:

![Xuất lần gửi trong thanh hành động](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm-gs-native-2026-10-08/report-revision-mobile390.jpg)

Sổ minh chứng:

![Xuất báo cáo trong Sổ minh chứng](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm-gs-native-2026-10-08/evidence-register-mobile390.jpg)

## Review package — BM04 đã duyệt và triển khai ngày 2026-10-09

Đây là review package gốc. User đã duyệt; sửa, regression, High review và nghiệm thu hai download/Google đã chạy theo [kết quả BM04](bm-gs-04-verification-2026-10-09.md). Các từ “dự kiến/đề xuất/chưa chạy” bên dưới mô tả thời điểm trước phê duyệt.

Mục đích chính: tổng hợp đúng số dòng lỗi được xuất, kể cả khi hồ sơ chưa có mã lỗi. Giữ ý nghĩa nguồn, vị trí nút và quy trình một lần bấm xuất.

**Hiện tại / mong đợi:** có1dòng, mã lỗi trống → tổng0; mong đợi tổng1. Workbook thực tế và source là tín hiệu lặp lại nhỏ nhất. Nguyên nhân đã xác minh như phần P2; không cần sửa DB hoặc nhập mã lỗi.

```text
CẦN XỬ LÝ — BM-GS-04
STT | Mã lỗi | Mô tả                           | Trạng thái
 1  |        | Xử lý vị trí góc nền UAT         | Mới

Hiện tại: Tổng lỗi: 0 | Đã khắc phục: 0 | Đang xử lý: 1
Đề xuất:  Tổng lỗi: 1 | Đã khắc phục: 0 | Đang xử lý: 1
```

**Phạm vi sửa dự kiến:** `backend/internal/core/bm_gs_workbook.go`, `bm_gs_workbook_test.go`, regression native export liên quan; cập nhật mapping renderer và current plan cùng lượt nếu được duyệt. Không đổi permission, route, source code fields, migration, lịch sử hoặc layout.

**Cách sửa đề xuất:** tổng dùng `COUNT(A8:A<end>)` trên STT số do renderer tạo; end theo bảng đã mở rộng. Giữ COUNTIF/COUNTIFS trạng thái và mọi mã lỗi trống đúng nguồn.

**Kiểm chứng đề xuất:** regression reopened workbook cho toàn bộ mã trống, mã hỗn hợp và hơn3dòng; native list/detail; dữ liệu/nút/ảnh/print structure giữ nguyên; xuất lại từ browser và tính lại tổng bằng Google Sheets. Reviewer High đọc diff cuối. Chưa có diff application hay tests cho sửa lỗi này.

**Rủi ro còn lại:** engine recalculation chưa chạy; không nên sửa assertion range chỉ để suite xanh. Tiêu chí ảnh của bốn phiếu BM01/BM05 đã được rút lại sau đối chiếu mẫu; không mở rộng vào sửa công thức. [AGENTS.md:99](/Users/khiemnguyen/Works/ttcons/ttcons/AGENTS.md:99) yêu cầu dừng trước implementation khi thay đổi bugfix/code/UI; review package này là đề xuất để xác nhận, không phải thay đổi đã được duyệt.

## Bàn giao

**Bổ sung — xem BM07 có ảnh:** theo yêu cầu tiếp theo của anh, file BM07 tải thực tế từ native UI đã được import nguyên nội dung thành [Google Sheets BM-GS-07 có ảnh](https://docs.google.com/spreadsheets/d/1ALyrLCIBYXDwSuHbGXvQahfYDlSpbB3b9UzlEeIhJRk/edit?gid=365185761). Connector xác nhận native conversion và hai sheet `2026-10-08_01`/`Hình ảnh`. Edge đăng nhập đã hiển thị trang sổ và ảnh nhúng/caption ở 100%; [ảnh trang sổ](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm-gs-native-2026-10-08/bm07-sheets-report.jpg), [ảnh nhúng hiển thị](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm-gs-native-2026-10-08/bm07-sheets-image.jpg). Đây là ảnh giả lập UAT, không phải ảnh công trình thật. Không sửa workbook gốc, nội dung Sheet hay application source; không đổi chia sẻ. Native Sheets chuyển sang PARTIAL cho phần preview BM07, không xác nhận toàn bộ tám mẫu, công thức hoặc in.

TechNote Level3 đã được kiểm tra bằng mock DOM và preview IAB thật: Trước/Tiếp/Xem lại, giới hạn đầu/cuối và trạng thái lưu. Preview không có JS error được ghi nhận. Bước đầu/cuối ở 390px có document width và scroll width cùng 358px, không tràn ngang ở hai bước đã đo. [Ảnh preview mobile](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm-gs-native-2026-10-08/technote-mobile390.jpg), [kết luận mobile](/Users/khiemnguyen/Works/ttcons/ttcons/docs/acceptance-evidence/bm-gs-native-2026-10-08/technote-findings-mobile390.jpg). Đây là kiểm chứng phần giải thích, không thay nghiệm thu sản phẩm. Viewport thử nghiệm đã được reset. `git diff --check` đạt sau cập nhật tài liệu.

Shared NativeExcelExportButton/API client/toast vẫn là owner; EvidencePanel và EvidenceRecordViewerDialog chuẩn được giữ. Browser runtime/evidence task được giữ để xem lại. Không commit/push/deploy; không đóng plan. Đính chính ngày 2026-10-09 cập nhật routing/domain notes để không tiếp tục coi ảnh BM01/BM05 là bắt buộc; không đổi application behavior/ownership/contracts. TechNote Level3 phân biệt bằng chứng trình duyệt, lỗi BM04, kết quả BM07 và các phạm vi Sheets/print chưa chạy.
