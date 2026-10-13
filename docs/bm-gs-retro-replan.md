# Retro và replan: xuất biểu mẫu client từ nghiệp vụ hiện có

Ngày: 2026-10-08. Trạng thái: **DESIGN REJECTED đối với luồng cũ; APPROVED đối với kế hoạch thay thế**, theo phản hồi “Approve kế hoạch thay thế.” Mapping/capture native mới đã triển khai và kiểm chứng kỹ thuật; R5 chưa được nghiệm thu browser/Google Sheets. Theo dõi bằng chứng hiện hành và phần còn lại tại [current-states.md](current-states.md); nội dung retro dưới đây ghi lại bối cảnh trước triển khai.

Mục đích chính: Giám sát làm việc trong tính năng hiện có, ghi nhận hồ sơ một lần và xuất chính hồ sơ đó theo mẫu client. Điểm xuất phải kế thừa hồ sơ, bộ lọc và phiên bản đang xem. Ảnh đã liên kết được đưa vào file tự động.

Yêu cầu đính chính của người dùng là căn cứ thay thế thiết kế chọn biểu mẫu/chọn nguồn/chọn ảnh tại Project Detail. Tài liệu này giữ review package và kế hoạch đã duyệt. Implementation contract được theo dõi tại [Native export contract](bm-gs-native-export-contract.md); kết quả thực tế được ghi riêng trong `current-states.md`.

## Phạm vi ảnh hưởng

| Miền | Owner hiện có | Xung đột của thiết kế cũ | Kiểm tra cần có | Kết quả rà hiện tại |
|---|---|---|---|---|
| Báo cáo/thi công | Operations, Work Reports, workflow daily form | Chooser dựng lại báo cáo; editor bổ sung tách khỏi nội dung nguồn | Current report, list filter, immutable revision, cùng dữ liệu và ảnh | Sai luồng được xác minh; native owners đã có |
| Nghiệm thu thầu phụ/chất lượng | SubcontractorReconciliationPanel, checklist package; ConstructionCheck/Acceptance | Gộp các loại kiểm tra/nghiệm thu vào một nguồn `acceptance` | Actor, loại nghiệm thu, quantity semantics, kết quả/ảnh đúng hồ sơ | Có nguồn thầu phụ bị bỏ sót; mapping BM03 cần chốt |
| Rủi ro/phát sinh/giao việc | Survey risk form, daily blockers/next plan, construction handoff, variation owner | Sidecar trùng dữ liệu; trạng thái thương mại và kỹ thuật khác nhau | Quan hệ exact, lifecycle, owner và tiêu chí phạm vi | Không có căn cứ cho universal mapping cũ |
| Evidence | Uploader/viewer/linking của nguồn | Upload và chọn ảnh lặp tại export | Source membership, protected revision bytes, metadata chụp | Tự resolve tại nguồn là hướng thay thế |
| Bàn giao | HandoverWorkspace/checklist/readiness | Editor kết quả/tồn tại riêng; verified bị đồng nhất Đạt | Physical checklist, nguồn đối chiếu, điều kiện/kết luận đúng nghĩa | Có nguồn canonical; chưa đủ căn cứ cho mọi cột client |
| DB/contracts/quyền | Native routes/services/repos, custom migrations | JSON chung tám bảng và quyền `work_report.export` cho mọi mẫu | Từng field owner, trạng thái/audit/snapshot, checksum/deployment state | V152 chỉ UAT; cần review lại trước rollout |

## 1. Retro: sai ở đâu và vì sao

| Sai lệch | Bằng chứng | Hệ quả | Cách sửa trong kế hoạch mới |
|---|---|---|---|
| Biến bộ mẫu đầu ra thành một luồng thao tác riêng | `ProjectDetailPage.tsx:194,257` mở `BMGSExportDialog`; dialog yêu cầu chọn mẫu, nguồn, ảnh | Người dùng phải dựng lại ngữ cảnh đã có trong màn hình nghiệp vụ | Đặt hành động xuất tại hồ sơ hoặc danh sách của từng tính năng |
| Hiểu “Giám sát chủ động xuất mẫu phù hợp theo giai đoạn” thành chọn mã mẫu từ danh mục | `types_bm_gs_exports.go` và `BMGSOptions` tổ chức hệ thống quanh `template_code` | Bộ mẫu điều khiển UX thay cho công việc thực tế | Nghiệp vụ/hồ sơ xác định mẫu; mã mẫu là thông tin đầu ra |
| Hiểu hỗ trợ ảnh nhúng thành thêm upload/chọn ảnh trong export | `BMGSExportDialog.tsx:103,138,139`; request có `evidence_ids` | Thêm thao tác và có thể xuất thiếu ảnh nếu không chọn lại | Upload/link tại nguồn đang dùng; export đọc chính các liên kết đã lưu |
| Dùng nguồn “gần giống” để lấp đủ tám mã mẫu | BM03 gắn `acceptance`; BM05 nhận survey/action/variation; BM06 nhận Work Item | Có nguy cơ xuất đẹp nhưng sai ý nghĩa nghiệm thu/rủi ro/giao việc | Kiểm tra mục đích từng nguồn; nguồn chưa tương đương phải ghi là khoảng thiếu nghiệp vụ |
| Tách dữ liệu bổ sung thành editor chuyên cho BM-GS | `BMGSSourceEditorDialog`, `bmgsSourceFields`, JSON V152 trên tám bảng | Người dùng nhập hồ sơ chính rồi mở thêm form để đủ dữ liệu export | Bổ sung đúng trường còn thiếu vào editor/form của nguồn; đọc chung một dữ liệu |
| Kiểm thử theo contract đã hiểu sai | Tests/QA kiểm tra chooser, preflight và file theo contract cũ | PASS kỹ thuật không chứng minh yêu cầu được đáp ứng | Nghiệm thu phải bắt đầu từ tính năng gốc và chứng minh không chọn lại hồ sơ/ảnh |
| Câu hỏi “gộp hay tách” đặt trên thao tác chọn nguồn của thiết kế sai | Service tạo một main sheet cho mỗi source ref | Đẩy chi tiết nội bộ thành quyết định khó hiểu cho người dùng | Đơn vị hồ sơ và phạm vi danh sách quyết định bố cục workbook |

Trách nhiệm thuộc về thiết kế và điều phối của agent. Việc người dùng duyệt kế hoạch trước không thay thế nghĩa vụ đối chiếu kế hoạch với câu hỏi gốc. Không quy nguyên nhân cho phản hồi của client.

**Rút lại kết luận rộng:** “IMPLEMENTED / TECHNICALLY VERIFIED for the approved code scope” và “no remaining evidenced finding” trong ghi chú cũ không được dùng để kết luận đã đáp ứng nghiệp vụ. Bằng chứng đó chỉ áp dụng cho các kiểm tra kỹ thuật của thiết kế bị bác bỏ. Luồng sản phẩm hiện tại không đạt mục đích người dùng đã đính chính.

## 2. Hiện trạng đã xác minh

- Mốc trước thay đổi: `75dc44707785d399ed2408959bf784928a544e96`, branch `main`. Đối chiếu bằng `git show HEAD:<path>` và working tree; không reset hoặc rollback.
- Trước lần cập nhật tài liệu retro này, Git có 74 mục status: backend 29, frontend 34, mapping 9, docs 2. `assets/` được Git hiển thị theo thư mục, nên đây không phải số lượng chính xác tất cả file vật lý.
- `/reports` đã có xuất sổ đăng ký với scope/bộ lọc và xuất PDF một lần gửi cụ thể. Sổ đăng ký là danh sách báo cáo; nội dung báo cáo ngày và PDF từng phiên bản là hai đầu ra khác nhau.
- Giới hạn có sẵn được xác minh từ mã gốc: `ExportWorkReports` gọi `WorkReports`; nhánh `scope == supervision` trong repository vẫn áp dụng `LIMIT/OFFSET`. Vì vậy, không thể lấy nguyên query danh sách phân trang làm query xuất toàn bộ phạm vi lọc. Đây là rủi ro cần xử lý và kiểm thử trong R2, chưa phải lỗi đã sửa trong lượt retro.
- Operations là nguồn nội dung báo cáo thi công, kiểm tra và cần xử lý. Work Reports là nguồn gửi/nhận và lịch sử bất biến, không phải editor thứ hai cho nội dung thi công.
- Mã gốc chưa có các nút Excel chuẩn client tại Operations, Acceptance, Handover hay EvidencePanel. Kế hoạch mới sẽ bổ sung hành động trực tiếp vào các tính năng đó khi mapping nghiệp vụ phù hợp; không gọi đây là khôi phục nút có sẵn.
- TOC đã được đọc lại trực tiếp từ workbook gốc, `A8:C15`. BM03 là nghiệm thu nội bộ với thầu phụ/tổ đội; BM06 là giao việc hiện trường cho đại diện thầu phụ; BM07 là nhật ký ảnh. Tám mẫu là tám đầu ra theo mục đích, không phải tám chức năng nhập liệu mới.
- UAT đã chạy một phần trên dữ liệu tổng hợp cô lập; Google Sheets đã nhận các workbook thử. Những file này phản ánh thiết kế cũ và không phải nghiệm thu thiết kế mới.
- V152 và runtime mới chỉ được chạy trong DB UAT/test riêng. Chưa có migration vào DB làm việc chính hoặc production, chưa commit/push/deploy.

Nguồn code chính: `WorkReportsPage`, `WorkReportReviewDialog`, `OperationsPage`, `ConstructionReportDetailDialog/FormDialog`, `WorkflowFormPanel`, `SubcontractorReconciliationPanel`, `ConstructionCheckDialogs`, `ConstructionActionDialogs`, `EvidencePanel`, `AcceptanceEditor`, `HandoverWorkspace`; backend `service_work_reports.go`, `repository_work_reports.go`, `repository_construction_quality.go`, `repository_workflow_forms.go`, `repository_reconciliations.go`, `construction_handoff.go`, `types_construction.go`; các định nghĩa form trong V18/V20/V41/V105 và owner bàn giao/thương mại.

## 3. Trả lời lại câu hỏi gốc

**Logic hiện có cung cấp nhiều nguồn phù hợp để tích hợp export, nhưng chưa đủ để xuất chính xác toàn bộ tám mẫu.** Có hai loại khoảng thiếu: trường dữ liệu chưa được ghi nhận trong hồ sơ hiện có; và nghiệp vụ có tên gần giống nhưng chưa tương đương mẫu client. Cần giải quyết tại nguồn trước khi tuyên bố hỗ trợ đầy đủ.

### Mapping theo nghiệp vụ, không theo bộ chọn template

| Mẫu client | Nơi người dùng đang làm việc | Nguồn và ảnh kế thừa | Khoảng thiếu/phạm vi cần xác minh | Hướng xuất đề xuất |
|---|---|---|---|---|
| BM-GS-01 Khảo sát hiện trạng | Phiếu khảo sát trong Đầu việc/WorkflowFormPanel | Phiếu và phiên bản đang mở; đã có ngày/người đo, `dimension_items.area/dimension/unit/note`, rủi ro và minh chứng | Điểm giao MEP, kết cấu/hoàn thiện, đánh giá xử lý theo vị trí; đơn vị khảo sát/đại diện. Không nhập lại area/dimension/note đã có | Xuất một phiếu ngay trong phiếu khảo sát; bổ sung đúng trường thiếu vào form khảo sát |
| BM-GS-02 Báo cáo ngày | Operations → Nhật ký; `/reports` → báo cáo đã gửi | Daily report đang xem hoặc các báo cáo trong bộ lọc hiện tại; historical dùng snapshot và ảnh của đúng lần gửi | 12 nội dung giám sát chưa có đánh giá có cấu trúc; đại diện 24/7; vật tư nhập trường/quy cách/xuất xứ khác với vật tư sử dụng | Chi tiết xuất một báo cáo; danh sách xuất nhiều báo cáo trong một workbook; một sheet cho một báo cáo. Ngày là metadata/tên sheet, không gộp hai báo cáo khác nhau cùng ngày |
| BM-GS-03 Nghiệm thu nội bộ | Ưu tiên đối chiếu checklist/gói nghiệm thu thầu phụ và SubcontractorReconciliationPanel | `builtin_subcontractor_checklist`/package đã có ngày kiểm tra, thầu phụ/đại diện, khu vực, BOQ, claimed/confirmed quantity, kết quả, minh chứng | Tiêu chuẩn/bản vẽ/yêu cầu kỹ thuật; BOQ không mặc nhiên là KL thiết kế, claimed/confirmed không mặc nhiên là KL đo thực tế. ConstructionCheck là kiểm tra định tính; Acceptance `partial/final` chưa chứng minh nội bộ | Đề xuất đặt tại hồ sơ nghiệm thu/checklist thầu phụ sau khi chốt quantity semantics và quy trình; không mặc định mọi acceptance |
| BM-GS-04 Lỗi và tồn tại | Operations → Cần xử lý | Danh sách/action hiện tại; ảnh trước/khắc phục và lịch sử xác minh gắn với lỗi | Mã lỗi, vị trí và đơn vị/đại diện 24/7; mức độ lỗi không mặc nhiên bằng ưu tiên xử lý | Xuất danh sách đang lọc thành punch list; chi tiết chỉ xuất lỗi đang xem |
| BM-GS-05 Vấn đề/rủi ro/phát sinh | Rủi ro khảo sát; vướng mắc của báo cáo ngày; Cần xử lý; đề xuất phát sinh đúng ngữ cảnh | Survey đã có risk/severity/mitigation; daily blockers có content/impact/severity/owner/target date; NCR và variation giữ dữ liệu/lifecycle riêng | Chưa có sổ thống nhất đã xác minh; phải xác định trường thiếu theo từng nguồn, không nhập lại mitigation/cause/owner đã có. Ngày phát hiện không mặc nhiên là created_at | Xuất từ nguồn đã tương đương; chỉ aggregate khi có owner/phạm vi/quan hệ và quy tắc trạng thái rõ ràng |
| BM-GS-06 Công việc hiện trường | Kế hoạch tiếp theo và các giao việc đã sinh từ báo cáo ngày/bàn giao thi công | `next_plan_items` đã có title/owner/due/priority/description và sinh Work Item; `construction_handoff_items` có location/schedule/assigned user và liên kết công việc | Đầu việc workflow không đồng nghĩa chỉ dẫn cho thầu phụ; thiếu mã giao việc, người nhận/đại diện 24/7, ngày giao và progress có căn cứ | Xuất tại phạm vi giao việc hiện trường đã có; chốt tiêu chí trên generated links, không mọi Work Item |
| BM-GS-07 Sổ hình ảnh | Minh chứng của báo cáo/Đầu việc hoặc danh sách ảnh hiện trường phù hợp đã có | Toàn bộ ảnh đã liên kết; filename/caption/description/checksum đã có | Thời điểm chụp khác thời điểm tải lên; vị trí/giai đoạn còn thiếu. Nội dung kỹ thuật có thể dùng caption/description đúng nghĩa; mẫu có cả “Phát hiện lỗi” | Xuất nhật ký ảnh tại nguồn ảnh, có ảnh nhúng; bổ sung metadata thiếu vào luồng ghi nhận ảnh chuẩn |
| BM-GS-08 Checklist bàn giao | Hồ sơ bàn giao → Checklist | Hồ sơ đang chọn, checklist/readiness, notes/verifier và minh chứng/hồ sơ đã liên kết | Checklist hiện có chưa mặc nhiên bao phủ toàn bộ bàn giao vật lý client; yêu cầu/tồn tại/hạn/giai đoạn còn thiếu. `verified/waived` không tự thành Đạt vật lý; đủ điều kiện khác đã bàn giao | Xuất trực tiếp hồ sơ đang xem; đối chiếu từng kết quả/kết luận theo nghĩa nguồn, bổ sung tại Handover owner |

“Ảnh thuộc công trình” không đủ để tự gắn vào một báo cáo. Phải có quan hệ thuộc hồ sơ, checklist, lỗi hoặc phiên bản cụ thể. Nhật ký ảnh ở mức công trình cũng phải giữ bộ lọc/phạm vi của tính năng nguồn và loại trừ minh chứng không thuộc nghiệp vụ ảnh hiện trường.

## 4. Luồng mục tiêu và Mockup as Text

### Báo cáo ngày

Mục đích chính: xem/ghi nhận công việc trong ngày và xuất báo cáo ngày đang xem.

```text
+-----------------------------------------------------------+
| Báo cáo ngày · 08-10-2026                              [×] |
| Công trình A · Hạng mục thi công B                         |
|                                                           |
| Công việc trong ngày                                      |
| Nhân lực · Vật tư · Ghi nhận giám sát                      |
| Minh chứng · 6 ảnh đã liên kết                            |
| [ảnh 1] [ảnh 2] [ảnh 3] ...                               |
|                                                           |
| [Chỉnh sửa]  [Gửi báo cáo]            [Xuất Excel BM-GS-02]|
+-----------------------------------------------------------+
```

Những CTA ghi/sửa/gửi trên mockup chỉ xuất hiện nếu trạng thái và quyền hiện có cho phép. Nút xuất chỉ đọc dữ liệu đã lưu. Form đang có thay đổi chưa lưu thì thông báo lưu hồ sơ; không tự lưu hoặc xuất dữ liệu chưa ghi nhận.

Mục đích chính của danh sách: xem các báo cáo trong phạm vi ngày/công trình đang lọc và xuất cùng phạm vi đó.

```text
+-----------------------------------------------------------+
| Nhật ký thi công                    [Xuất báo cáo Excel]  |
| Từ ngày [01-10-2026]  Đến ngày [08-10-2026] [Áp dụng]      |
| Phạm vi đang áp dụng: Công trình A · 01–08/10              |
| Ngày        Hạng mục           Người lập       Trạng thái |
| 08-10       ...                ...             ...        |
| 07-10       ...                ...             ...        |
+-----------------------------------------------------------+
```

Nếu màn hình nguồn chưa có bộ lọc ngày, chỉ bổ sung bộ lọc phục vụ nhật ký khi thật sự cần và theo convention hiện có. Export không mở thêm bộ chọn nguồn. Phân trang hiển thị không được khiến file chỉ có trang hiện tại. `/reports` giữ nhãn “Xuất sổ đăng ký” cho đầu ra danh sách cũ và nhãn riêng “Xuất báo cáo Excel” cho nội dung BM02, tránh đổi nghĩa nút hiện có.

### Lỗi và ảnh

Mục đích chính: theo dõi các lỗi đang lọc và xuất chính punch list đó.

```text
+-----------------------------------------------------------+
| Cần xử lý                         [Xuất Excel BM-GS-04]   |
| [Cần xử lý] [Chờ xác nhận] [Đã xong] [Tất cả]               |
| Tìm kiếm [...] Người xử lý [...] Ưu tiên [...] Ảnh hưởng   |
| Mã lỗi    Vị trí        Người phụ trách    Hạn    Trạng thái|
| ERR-01    ...           ...                ...    ...      |
| ERR-02    ...           ...                ...    ...      |
+-----------------------------------------------------------+
```

Giữ queue và bộ lọc trực tiếp hiện có; mockup chỉ bổ sung hành động export. Một main sheet chứa các dòng của danh sách đang xuất là đề xuất bố cục theo nghiệp vụ, không phải yêu cầu bố cục do TOC đã xác nhận. Chi tiết một lỗi chỉ xuất một dòng. Số hồ sơ độc lập quyết định tách sheet, không dùng một quy tắc “mỗi source một sheet” cho mọi mẫu.

Mục đích chính: xem/lưu minh chứng cho hồ sơ đang làm và xuất nhật ký ảnh của phạm vi đó.

```text
+-----------------------------------------------------------+
| Minh chứng · 6 ảnh                 [Xuất nhật ký ảnh]     |
| [ảnh 1] [ảnh 2] [ảnh 3] [ảnh 4] [ảnh 5] [ảnh 6]           |
| Ảnh mới: uploader/camera chuẩn hiện có                    |
| Thông tin ảnh: vị trí · giờ chụp · nội dung · giai đoạn     |
+-----------------------------------------------------------+
```

Nút xuất lấy sáu ảnh đã lưu thuộc phạm vi này, tạo BM07 và ảnh nhúng. Khi xuất BM02, sáu ảnh của báo cáo được đưa vào phụ lục của BM02. Xuất BM02 không tự tạo thêm BM07; BM07 là hành động nhật ký ảnh ở nguồn ảnh.

### Trạng thái thiếu dữ liệu và màn hình nhỏ

Mục đích chính: giúp người ghi hồ sơ nhận biết và sửa đúng trường còn thiếu trong hồ sơ đang làm.

```text
+------------------------------------+
| Báo cáo ngày · 08-10-2026       [×] |
| ... Nội dung và ảnh hiện có ...     |
|                                    |
| Thiếu thông tin tại hồ sơ:          |
| Đại diện tổ đội · Quy cách vật tư   |
| [Chỉnh sửa báo cáo]                 |
|                                    |
| [Xuất Excel BM-GS-02]               |
+------------------------------------+
```

Đây là trạng thái minh họa. Trường nào bắt buộc, trường nào cảnh báo và trường nào được để trống phải được chốt trong mapping cột. Không mặc định mọi ô nhập mẫu đều là gate mới. Nút sửa mở editor vốn có của hồ sơ; hồ sơ đã gửi/khóa dùng lịch sử và quy trình bổ sung hiện có. Màn hình nhỏ xếp hành động dọc, giữ nội dung và cùng phạm vi xuất.

### Reuse các tương tác chuẩn

| Tương tác | Owner chuẩn | Entry point mục tiêu |
|---|---|---|
| Ghi/sửa báo cáo ngày | ConstructionReportFormDialog; workflow form tương ứng khi form-backed | Operations, chi tiết báo cáo; không editor BM-GS phụ |
| Ảnh mới/camera | EvidenceCapture → EvidenceUploader và API upload/link đúng nguồn | Form báo cáo, EvidencePanel, checklist nghiệm thu/bàn giao |
| Xem ảnh/tài liệu | EvidenceRecordViewerDialog/EvidenceReferenceViewer | Báo cáo, lịch sử, lỗi, checklist; giữ exact revision |
| Thông báo/validation | ToastProvider có nút đóng, FieldBlock/owner form hiện có | Lưu nguồn, thiếu dữ liệu, lỗi xuất |
| Hồ sơ nghiệm thu | AcceptanceEditor hoặc owner kiểm tra phù hợp đã xác minh | Không sao chép kết quả sang một form export riêng |
| Nghiệm thu thầu phụ/khối lượng | SubcontractorReconciliationPanel và workflow checklist/package hiện có | Review owner BM03 trước khi bổ sung tiêu chuẩn/yêu cầu kỹ thuật |
| Bàn giao | HandoverWorkspace/checklist editor hiện có | Hồ sơ đang chọn, không auto-chọn bản khác |

## 5. Thiết kế kỹ thuật đề xuất

### Contracts

Mỗi owner công bố read contract xuất hồ sơ của chính owner đó. Backend nhận ID/phiên bản từ chi tiết hoặc cùng bộ lọc đã áp dụng từ danh sách. Backend xác định mẫu đã mapping; không nhận danh sách ảnh tùy chọn hay mã mẫu tùy ý để thay đổi nghiệp vụ.

Ví dụ đề xuất, chưa phải API đã triển khai:

- Chi tiết daily report: `(daily_report_id)` → current saved report → BM02.
- Chi tiết Work Report: `(report_id, revision_no)` → immutable revision → BM02.
- Danh sách nhật ký: `(project_id, applied_date/status/search filters, source scope)` → matching reports → BM02 workbook.
- Action register: `(project_id, applied_action_filters)` → scoped punch list → BM04.
- Nhật ký ảnh tại nguồn: `(source_kind, source_id, revision if historical)` hoặc filter của danh sách ảnh hiện có → associated eligible photos → BM07.
- Handover: `(handover_id)` → exact checklist/readiness/linked artifacts → BM08.

Một endpoint nội bộ chung hay nhiều endpoint mỏng là quyết định implementation theo owner, không phải lý do tạo UI/module BM-GS riêng. Giữ formatter chung là hợp lý; quyền đọc và query dữ liệu vẫn thuộc module nguồn. Không bắt các owner khác chỉ dùng `work_report.export` nếu chưa có chính sách quyền đã xác minh; thiết kế quyền phải phù hợp hành động export của từng owner và quyền xem chính hồ sơ.

### Dữ liệu và lịch sử

1. Lập mapping từng cột: ý nghĩa; nguồn exact; cách chuyển đổi; đơn vị; thời điểm nghiệp vụ; ảnh liên quan; bắt buộc/cảnh báo/để trống; owner ghi/sửa; trạng thái cho phép.
2. Tái sử dụng dữ liệu hiện có trước. Trường thiếu nhập tại editor native với type/validation và audit của nguồn. Không giữ một phiên bản cùng thông tin chỉ phục vụ export.
3. Với workflow forms, thêm trường bằng revision định nghĩa mới khi cần. Không sửa definition/answers/history đã phát hành hoặc các runtime đang pin chỉ để đủ mẫu.
4. Formal Work Report tiếp tục snapshot dữ liệu nguồn và ảnh của đúng lần gửi. Export historical không fallback về current source, ảnh mới hay metadata mới. Nếu thiếu trường, hiển thị thiếu theo chính sách đã chốt.
5. `material_usage` không phải hàng nhập công trường. Dữ liệu giao nhận chỉ tự lấy khi có quan hệ chứng từ/đợt giao với báo cáo; thiếu quan hệ phải ghi nhận tại nguồn phù hợp, không gom mọi receipt trong công trình chỉ vì cùng ngày.
6. Priority không mặc nhiên là mức độ lỗi; acknowledgement không phải nghiệm thu; contract approval không phải xử lý xong rủi ro; `handed_over` khác đủ điều kiện bàn giao. Khối lượng thiết kế/thực tế/nhập/sử dụng giữ nghĩa riêng.

### Ảnh nhúng

Hệ thống giải quyết liên kết ảnh từ source contract trước khi mở storage. Dedupe theo đúng định danh file/nguồn/phiên bản; cùng evidence ID ở hai revision không cho phép tráo metadata hoặc lấy bản mới thay lịch sử. Ảnh phải có caption và liên hệ đến dòng hồ sơ.

Chỉ nén derivative khi export, không ghi đè ảnh gốc. Có thể tái sử dụng chuẩn hóa EXIF, resize không phóng to và giới hạn byte/pixel của prototype sau khi review. Các giới hạn phải được đo theo capacity hồ sơ hiện có; không ép người dùng chọn bỏ ảnh để vừa giới hạn 20 ảnh cũ. Không âm thầm truncate. Chọn batching/đóng gói/job chỉ khi measurement cho thấy synchronous không đáp ứng; không thêm queue mặc định.

### Bố cục workbook

Giữ cột và ý nghĩa mẫu client, bỏ dữ liệu mẫu, dùng dòng động và công thức phù hợp. BM02 nhiều báo cáo chung workbook theo quyết định đã có. Các mẫu sổ/danh sách đề xuất một bảng nhiều dòng theo phạm vi nguồn; hồ sơ độc lập có thể tách theo đơn vị hồ sơ. Điều này được review cùng luồng nguồn, không yêu cầu người dùng quyết định từ một bộ chọn source kỹ thuật.

## 6. Xử lý phần triển khai cũ

Chưa rollback file hoặc migration trong lượt retro. Inventory dưới đây là quyết định đề xuất cho lần sửa được duyệt, không phải công việc đã thực hiện.

| Nhóm | File/thành phần | Xử lý đề xuất |
|---|---|---|
| Bỏ UX sai | ProjectDetail toolbar/entrypoint; BMGSExportDialog; chooser/recommendation/upload state và tests chỉ bảo vệ luồng đó | Loại khỏi luồng người dùng; không chữa bằng auto-prefill hoặc ẩn select trong dialog cũ |
| Thay orchestration | `*_bm_gs_exports.go`, `types_bm_gs_exports.go`, options/query chọn nguồn/ảnh | Chuyển sang contracts tại nguồn, list filter và exact revision; hệ thống resolve ảnh tự động |
| Thay cách nhập khoảng thiếu | BMGSSourceEditorDialog/bmgsSourceFields và buttons gắn ở các trang | Di chuyển trường cần thiết vào owner native, loại trường trùng/không có nghĩa; không giữ một form bổ sung riêng |
| Rà persistence | V152, bm_gs source JSON/version/services, crew IDs và source hash integrations | Review từng trường, quan hệ, version và snapshot. Không mang nguyên migration tám bảng sang kế hoạch mới |
| Reuse có điều kiện | `bm_gs_workbook.go`, `bm_gs_images.go`, sanitized template và tests độc lập | Giữ phần đúng về layout/nén; kiểm tra toàn bộ column mapping/metadata/grouping theo contract mới |
| Reuse có điều kiện | Shared binary API helper, exact historical viewer path, uploader file-kind hint, navigation helpers | Giữ phần cần cho UX/native flow và có regression riêng; loại caller/nhánh chỉ phục vụ chooser |
| Viết lại verification | Selection/preflight/mock harness tests; docs/mappings cũ | Thay assertion về hành trình nguồn/ảnh/filter/revision. Không xóa regression bảo mật/lịch sử chỉ vì bỏ chooser |

Migration: xác minh nơi đã áp dụng/checksum trước khi thay kế hoạch persistence. DB UAT có V152 đã áp dụng; không chỉnh applied migration/checksum tại DB đó. Chưa có bằng chứng V152 đã áp dụng ở môi trường làm việc chính/production. Không tự renumber, rollback hay xóa dữ liệu; kế hoạch additive hoặc tái tạo riêng môi trường disposable phải có phạm vi cụ thể.

## 7. Kế hoạch thay thế và gates

| Bước | Việc sẽ làm sau duyệt | Tiêu chí thoát | Owner |
|---|---|---|---|
| R0 | Ghi nhận bác bỏ thiết kế; inventory prototype; supersede contract/mapping; lập retro/replan | Agent sau không tiếp tục chooser như yêu cầu đã chốt | Root + High reviewer, documentation only |
| R1 | Chốt matrix từng cột và native source; giải quyết BM03/BM05/BM06; review mockup và rights/history/capacity | Mỗi mẫu có owner phù hợp hoặc gap nghiệp vụ explicit; không mặc định 8/8 fit | Root + domain exploration Medium + audit High |
| R2 | Loại chooser/entrypoint sai có kiểm soát; đưa BM02 và ảnh tự động vào báo cáo ngày/lịch sử | Mở một báo cáo đã có → bấm Xuất Excel → đúng nội dung và toàn bộ ảnh đã liên kết; list scope nhất quán | Backend/FE Medium; High audit |
| R3 | Lấp trường thiếu ở native editors/form revisions, theo source owner; bổ sung BM01/BM04/BM07/BM08 | Ghi nhận một lần; editor, detail, export và snapshot cùng dữ liệu | Bounded backend/FE Medium, sở hữu file không chồng nhau |
| R4 | Tích hợp BM03/BM05/BM06 theo mapping đã chốt, không tạo nghiệp vụ giả để đủ mã mẫu | Internal acceptance/risk/site instruction semantics được kiểm chứng | Domain owners Medium + High review |
| R5 | Chạy regression tích hợp, browser từ nguồn native và Google Sheets; TechNote Level 3 về thiết kế đúng | Tất cả tiêu chí đã duyệt có bằng chứng hiện hành; giới hạn còn lại rõ | Root QA + độc lập High |

R0 là công việc của lượt này. R1 không phải nghiên cứu lại từ đầu: TOC, code gốc và các gap trong tài liệu này là đầu vào. Các decision còn thiếu được giải quyết bằng evidence/domain trước; chỉ hỏi người dùng khi là quyết định nghiệp vụ không thể suy ra.

R2 là lát cắt đầu tiên để kiểm chứng hướng UX đã đúng trước khi mở rộng tám mẫu. Không tiếp tục theo phân công cũ “API BMGS + UI BMGS + source editor” vì phân công đó củng cố ranh giới sai. Phân công theo vertical slice/nghiệp vụ; engine render/image là component hỗ trợ.

## 8. Tiêu chí nghiệm thu thiết kế mới

1. Người dùng bắt đầu tại báo cáo/phiếu/checklist/danh sách đã có. Không vào module BM-GS riêng hoặc chọn lại loại mẫu để dựng hồ sơ.
2. Bấm export ở chi tiết xuất đúng ID và phiên bản đang xem; bấm ở danh sách xuất đúng filter/scope đã áp dụng, xuyên phân trang. Không export toàn dự án hoặc cả tám mẫu ngoài ngữ cảnh.
   Regression phải có số báo cáo vượt page size, gồm phạm vi Giám sát; file không bị giới hạn bởi `LIMIT/OFFSET` của trang đang xem.
3. Ảnh đã lưu thuộc hồ sơ tự có trong file. Chạy regression với nhiều ảnh, cùng ảnh ở nhiều quan hệ và lịch sử; không chọn lại ảnh, không upload trong export, không silent omission.
4. Trường thiếu được bổ sung trong editor native bằng quyền/trạng thái hiện có; không duplicate edit hoặc tự biến ô client thành gate mới.
5. Không có suy diễn Đạt/khối lượng/tiến độ/trạng thái/giờ chụp. Snapshot cũ và checksum ảnh gốc giữ nguyên.
6. BM03/BM05/BM06 chỉ được báo hỗ trợ khi đã xác minh nghĩa nguồn. Acceptance cuối không mặc nhiên là nghiệm thu nội bộ; mọi Work Item không mặc nhiên là chỉ dẫn thi công.
7. Export kiểm tra actor/project/source/file scope ở backend; bị thu hồi quyền, wrong project/record/revision phải bị từ chối. Query/count/paging cùng phạm vi; update field không bypass workflow transitions.
8. UI giữ đúng owner Dialog/EvidenceUploader/viewer/toast; desktop/mobile, loading/error/retry, stale edit, lịch sử/khóa và chưa lưu có bằng chứng từ browser.
9. Workbook được mở trong Google Sheets thực tế theo lựa chọn của người dùng; kiểm tra nội dung, công thức, cột, dòng dài, ảnh nhúng, caption và layout. Excel desktop/in vật lý vẫn là phần chưa xác minh nếu chưa chạy.
10. Tests/build/audit phải gắn với revision hoặc working-tree fingerprint hiện hành. Không dùng thành công của prototype để ký nghiệm thu hướng mới. TechNote phân biệt Verified, Proposed và Unknown.

## 9. Tình trạng bàn giao lượt retro

- **Verified:** hiểu sai đã được xác nhận qua đính chính người dùng, TOC và source/diff; inventory và native export báo cáo đã được rà.
- **Proposed:** native export actions, field ownership, cách đóng gói danh sách và lộ trình R1–R5.
- **Unknown:** mapping nghiệp vụ hoàn chỉnh cho BM03/BM05/BM06; chính sách bắt buộc từng cột; capacity thực tế của dự án client; nghiệm thu hướng mới, Excel desktop và bản in.
- Lượt này chỉ cập nhật tài liệu/replan và nhãn superseded của mapping. Không sửa application code/migration, không chạy thêm browser/test/build, không commit/push/deploy, không xóa workbook thử.
- Review độc lập GPT-6.1 Sol High đã nêu hai điểm cần sửa trong tài liệu: đơn vị sheet BM02 và bộ lọc mockup Cần xử lý. Hai điểm đã được sửa; reviewer kiểm tra lại các hunk đó cùng ghi chú pagination và không còn phát hiện cần sửa trong phạm vi kiểm tra cuối. `git diff --check` đạt trên tracked diff; đây là kiểm tra tài liệu, không phải nghiệm thu ứng dụng.
- Kế hoạch cũ được giữ trong active notes với trạng thái bác bỏ để truy vết. Chưa đóng/archive plan vì người dùng chưa yêu cầu đóng plan.
