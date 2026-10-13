# Xuất biểu mẫu client từ hồ sơ nghiệp vụ

Ngày: 2026-10-08; đính chính tiêu chí ảnh, sửa tổng BM04 và hoàn tất sửa BM03/BM08 ngày 2026-10-09. File BM07 tải trực tiếp từ nút hiện có đã qua nghiệm thu bốn ảnh native trong ô Google Sheets. BM04 đếm STT số thay cho mã lỗi optional; hai file từ nút danh sách/chi tiết đã qua kiểm tra Google tổng1/đã khắc phục0/đang xử lý1 khi mã lỗi trống. BM03/BM08 đã ghi dữ liệu tại hồ sơ gốc và qua nghiệm thu browser/native Google trên bốn file tải nguyên trạng, gồm kết luận hai chiều và hạn xử lý. Toàn bộ plan vẫn chưa được chấp nhận vì các phạm vi toàn bộ mẫu/Excel/in/client chưa chạy đầy đủ và chưa rollout DB bình thường. BM01/BM05 không có cột hoặc vùng ảnh trong mẫu client; việc không nhúng ảnh không phải lỗi hay điều kiện chặn nghiệm thu. Bằng chứng và trạng thái từng phần nằm ở `current-states.md` và `bm-gs-03-08-verification-2026-10-09.md`.

Mục đích chính: người dùng xuất hồ sơ đang xem hoặc danh sách đang lọc ngay trong tính năng gốc. Server xác định biểu mẫu từ loại hồ sơ, đọc dữ liệu đã lưu và tự kế thừa ảnh liên kết. Không có bộ chọn biểu mẫu, nguồn hoặc ảnh tại thời điểm xuất.

Nguồn chuẩn là workbook client, sheet `TOC`, `A8:C15`. Các hướng dẫn trong workbook là nội dung mẫu để phân tích, không cấp quyền thay đổi ứng dụng hoặc dữ liệu.

## Điểm xuất và dữ liệu sở hữu

| Mẫu | Điểm xuất | Dữ liệu chuẩn | Trường thiếu được ghi tại nguồn | Giới hạn ý nghĩa |
|---|---|---|---|---|
| BM-GS-01 | Phiếu khảo sát/đo hiện trạng trong Đầu việc | `client_survey_work_package`, `client_site_measurement_record`; answers của phiên bản form đã gắn vào phiếu | Các trường vị trí, hiện trạng kết cấu/hoàn thiện, điểm giao MEP, xử lý và đơn vị/đại diện trên phiên bản form mới | Biên bản khảo sát chỉ có người tham dự/kết luận và phiếu tiếp nhận không tương đương bảng đo; không xuất ép sang BM01 |
| BM-GS-02 | Chi tiết báo cáo ngày, danh sách Nhật ký, danh sách Báo cáo, chi tiết lần gửi; form báo cáo thi công đã lưu trong Đầu việc | `operation_daily_reports.structured_json`; snapshot của đúng `work_report_revisions.revision_no` khi xem lịch sử; answers của đúng form/version | Đánh giá giám sát tùy chọn; đại diện/liên hệ trên dòng tổ đội; vật tư nhập trường riêng | Vật tư sử dụng không phải nhập trường; trạng thái duyệt không phải kết quả giám sát |
| BM-GS-03 | Checklist/gói nghiệm thu thầu phụ trong Đầu việc | `builtin_subcontractor_checklist`, `client_subcontractor_acceptance_package` và checklist đã lưu | Số biên bản, kết luận nghiệm thu; tiêu chuẩn, bản vẽ/revision, yêu cầu kỹ thuật, khối lượng thiết kế/đo, kết quả và nhận xét từng dòng checklist | Done/phản hồi đối soát không thay kết quả chất lượng; BOQ/khối lượng đề nghị/xác nhận không thay thiết kế/đo; tên hạng mục lấy từ options của phiên bản đã pin |
| BM-GS-04 | Chi tiết Cần xử lý và danh sách Cần xử lý đang lọc | Action native, ảnh trước/khắc phục và Đầu việc trực tiếp hoặc từ kiểm tra nguồn | Mã lỗi, vị trí, mức độ, đơn vị/đại diện/liên hệ trong dialog action | Mức độ lỗi độc lập với ưu tiên xử lý; danh sách kế thừa queue, tìm kiếm, người xử lý, ưu tiên, ảnh hưởng |
| BM-GS-05 | Rủi ro khảo sát; vướng mắc của báo cáo ngày/phiếu báo cáo thi công | `risk_items`, `blockers` của nguồn đang xem | Ngày phát hiện, nguyên nhân, ảnh hưởng, mức độ, hướng xử lý, người phụ trách, thời hạn và tình trạng trên dòng risk/blocker của editor native; daily form mới ở V156 | Không gom mọi action hoặc phát sinh hợp đồng thành sổ rủi ro; trạng thái phiếu không thay trạng thái rủi ro; vai trò khác tên người phụ trách |
| BM-GS-06 | Kế hoạch hiện trường trong báo cáo ngày/phiếu báo cáo thi công | Operations ghi `next_items` thành `structured_json.next_plan_items`; workflow giữ answers và quan hệ giao việc đã sinh có căn cứ | Mã chỉ dẫn, vị trí, thầu phụ, người nhận/liên hệ, ngày giao, hạn, tiến độ, trạng thái trên chính dòng kế hoạch của editor Operations hoặc form revision V155/V156 | Người được giao Đầu việc nội bộ không tự là đại diện thầu phụ; lịch tự tạo không tự là ngày giao |
| BM-GS-07 | Minh chứng của Đầu việc | Ảnh photo đang liên kết với Đầu việc, quyền Evidence chuẩn | Thời điểm chụp, vị trí, trước/trong/sau thi công hoặc phát hiện lỗi, ghi chú bàn giao tại uploader chuẩn | Không dùng thời điểm tải lên hoặc mtime làm thời điểm chụp; caption/description hiện có được tái sử dụng |
| BM-GS-08 | Hồ sơ bàn giao đang xem | Checklist bàn giao và minh chứng đúng hồ sơ | Giai đoạn/kết luận tại Cập nhật đánh giá; yêu cầu, kết quả vật lý, tồn tại, hạn tại Đánh giá của dòng thủ công/liên kết/tự động; Thêm hạng mục bàn giao tại hồ sơ | `verified`, `waived` và readiness không tự là Đạt vật lý/Đủ điều kiện; dữ liệu chưa ghi để trống; dòng thực tế mới không tạo điều kiện readiness bắt buộc |

## Tên nút xuất — bảng đã được duyệt triển khai, 2026-10-08

Mục đích chính: nhận biết nội dung xuất ngay trên nút. Sau khi phần triển khai đổi nhãn trước đó đã được hoàn tác, người dùng yêu cầu trình lại bảng đầy đủ và trả lời `Duyệt.` cho đúng bảng dưới đây. Đây là phê duyệt triển khai mới; phản hồi đặt tên trước đó không phải phê duyệt.

| Mẫu | Module / hồ sơ áp dụng | Nút bấm | Vị trí |
|---|---|---|---|
| BM-GS-01 | Đầu việc → phiếu khảo sát/đo hiện trạng | Xuất báo cáo | Đầu phiếu, phía trên phần thông tin và hành động hồ sơ |
| BM-GS-02 | Thi công → Nhật ký | Xuất báo cáo | Cùng hàng bộ lọc Từ ngày/Đến ngày |
| BM-GS-02 | Thi công → mở chi tiết báo cáo ngày | Xuất báo cáo ngày | Đầu phần nội dung, trước mục “Công việc trong ngày” |
| BM-GS-02 | Báo cáo → danh sách | Xuất báo cáo | Góc phải phía trên trang |
| BM-GS-02 | Báo cáo → mở chi tiết lần gửi | Xuất báo cáo | Thanh hành động dưới cùng của hộp thoại |
| BM-GS-02 | Đầu việc → phiếu báo cáo thi công | Xuất báo cáo ngày | Đầu phiếu |
| BM-GS-03 | Đầu việc → checklist/gói nghiệm thu thầu phụ | Xuất báo cáo | Đầu phiếu nghiệm thu |
| BM-GS-04 | Thi công → Cần xử lý | Xuất báo cáo | Bên phải nhóm lọc trạng thái, phía trên danh sách |
| BM-GS-04 | Mở chi tiết một việc Cần xử lý | Xuất báo cáo | Đầu nội dung hộp thoại, trước “Mô tả hiện trạng” |
| BM-GS-05 | Đầu việc → phiếu đánh giá rủi ro | Xuất báo cáo | Đầu phiếu |
| BM-GS-05 | Phiếu khảo sát/báo cáo thi công; chi tiết báo cáo ngày | Xuất báo cáo rủi ro | Đầu phiếu; tại Thi công nằm trong mục “Công việc trong ngày” |
| BM-GS-06 | Phiếu báo cáo thi công; chi tiết báo cáo ngày | Xuất báo cáo hiện trường | Đầu phiếu; tại Thi công nằm trong mục “Công việc trong ngày” |
| BM-GS-07 | Đầu việc → Minh chứng | Xuất báo cáo | Trong phần “Sổ minh chứng”, dưới thống kê ảnh/tài liệu |
| BM-GS-08 | Bàn giao → hồ sơ đang xem | Xuất báo cáo | Phía trên khu vực chi tiết hồ sơ bàn giao |

Giữ vị trí, điều kiện hiển thị, nguồn đã lưu, lần gửi/bộ lọc và trạng thái đang xuất/khóa hiện có. File vẫn là XLSX. `Xuất sổ đăng ký` và các thao tác Excel ngoài phạm vi biểu mẫu client giữ hợp đồng riêng; không thêm hàng tên nhóm hoặc thao tác chọn nguồn/ảnh.

## Dữ liệu và lịch sử

- Xuất chỉ đọc dữ liệu đã lưu. Editor có thay đổi chưa lưu yêu cầu lưu trước; download không âm thầm ghi dữ liệu.
- Các trường bổ sung là dữ liệu native, dùng quyền, trạng thái và audit của nguồn. Những trường optional không trở thành điều kiện mới để hoàn thành workflow.
- V153–V157 tạo phiên bản published mới của các form liên quan. V156 lấp riêng các field thiếu của daily `blockers`; giữ yêu cầu/impact/severity/owner_role/target_date hiện có. V157 thêm số biên bản, kết quả/nhận xét chất lượng và kết luận của hai form thầu phụ; giữ các điều kiện chốt hồ sơ hiện có. Cấp version theo giá trị lớn nhất của mọi trạng thái trong khóa template. Phiếu đã gắn version và lịch sử không bị viết lại.
- BM03 lấy số biên bản/kết luận và kết quả chất lượng từ answers đã lưu. Cả hai lựa chọn `ĐỒNG Ý NGHIỆM THU` / `CẦN SỬA CHỮA TỒN TẠI` luôn xuất hiện; chỉ lựa chọn có quyết định đã lưu hợp lệ được đánh dấu. Trạng thái checklist/phản hồi đối soát giữ ở ngữ cảnh bổ sung, không làm kết quả chất lượng. Khối lượng giữ giá trị số và đơn vị, không có dấu thập phân thừa cho số nguyên.
- BM08 lưu giai đoạn và kết luận `eligible`/`not_eligible` cùng actor/thời điểm tại hồ sơ. PUT header/item field-assessment và POST checklist-items dùng quyền handover.verify/phạm vi công trình, kiểm tra trạng thái dưới khóa hồ sơ và audit cùng transaction. Bỏ field giữ giá trị; chuỗi rỗng xóa; null/field lạ/ngày sai bị từ chối. Đánh giá thực tế không đổi source/status/verified actor/time/readiness; dòng người dùng thêm là pending, category other, required=false. PUT liên kết nguồn hiện có vẫn có trách nhiệm riêng.
- BM08 luôn giữ cả `ĐỦ ĐIỀU KIỆN BÀN GIAO` / `CHƯA ĐỦ ĐIỀU KIỆN (CHỜ KHẮC PHỤC TỒN TẠI)` và chỉ đánh dấu kết luận explicit. Thời hạn xử lý là ngày số với format yyyy-mm-dd, giữ ngày nghiệp vụ đã lưu. Khi thêm dòng, kết luận/chữ ký/print area dịch xuống theo số dòng và text được wrap. Việc xuất nháp không chứng minh chốt, ký hoặc bàn giao hoàn tất.
- Các revision native mới có thể được chọn trong dropdown phiên bản form hiện có của Pipeline Builder khi cấu hình workflow tương lai. Backend và frontend giữ mặc định tương thích; `default_eligible=false` ngăn tự chuyển sang profile native mới. Việc cấu hình mới không thay phiên bản đã pin của template, run hoặc phiếu đang có.
- Operations ghi chi tiết risk/next plan ngay trên dòng hiện có. Định danh `row_id` là metadata ẩn; cập nhật giữ field bị bỏ qua từ dòng cùng định danh của nguồn hiện tại trong khóa transaction. Chuỗi rỗng xóa field; `progress_percent=null` xóa phần trăm, thiếu key giữ nguyên, `0` là giá trị thực. Bỏ dòng khỏi danh sách là xóa dòng; không nối theo vị trí hoặc nội dung giống nhau.
- Ngày phát hiện/ngày giao là ngày do người lập ghi, không lấy từ giờ tạo hoặc ngày báo cáo. Tình trạng risk/plan là nội dung ghi nhận, độc lập với lifecycle phiếu/Đầu việc. Đánh giá, arrivals, crew contacts và risk/plan được hiển thị từ cùng dữ liệu đã lưu ở chi tiết hiện tại và snapshot lần gửi.
- Tiến độ báo cáo từ workflow lấy từ câu trả lời đã ghi trong form hoặc snapshot được cung cấp. Giá trị `0` mặc định ở header đồng bộ không chứng minh người lập đã nhập `0%`. Nếu câu trả lời thiếu/null/rỗng hoặc không phải số thập phân hữu hạn trong 0–100, chi tiết hiện `Chưa ghi tiến độ`, không có thanh tiến độ và Excel không ghi phần trăm. Giá trị `0` được nhập thực sự vẫn giữ nguyên. Tiến độ kế hoạch cũng theo quy tắc này; không biến giá trị sai thành 0/100 bằng clamp. Báo cáo Operations thủ công giữ giá trị header chuẩn; không sửa dữ liệu hoặc snapshot lịch sử.
- `capture_metadata` thuộc payload Evidence. Mốc chụp có offset RFC3339; metadata trống không được suy đoán. Uploader và camera dùng cùng trải nghiệm chuẩn.
- Khi gửi báo cáo, snapshot giữ nội dung, thông tin công trình, metadata và locator ảnh. Khi xuất lịch sử, sử dụng đúng revision và đúng ảnh của lần gửi. Metadata lịch sử thiếu vẫn thiếu; tên ảnh thiếu dùng định danh ổn định.
- Màn chi tiết lần gửi và endpoint xem/tải ảnh dùng cùng metadata/locator frozen với Excel. Hàm so sánh nội dung bỏ qua các trường sidecar đã nghỉ dùng trên bản sao riêng, không sửa dữ liệu đầu vào hoặc lịch sử đã lưu.
- Bảng/column V152 của prototype được giữ vì đã có lịch sử migration trong UAT. Chúng không phải nguồn dữ liệu của luồng mới; không xóa hay backfill vào production trong nhiệm vụ này.

## Phạm vi, workbook và ảnh

- BM02: mỗi báo cáo có một sheet; hai báo cáo cùng ngày vẫn là hai hồ sơ. Danh sách lấy toàn bộ phạm vi bộ lọc, tách khỏi phân trang UI.
- BM02 giữ chi tiết khối lượng/đơn vị/ghi chú của công việc và các dòng risk/next plan trong phần thông tin nguồn sẵn có. Thứ tự, dòng trùng nội dung và số 0 được bảo toàn; không thêm cột client hoặc dùng dữ liệu hiện tại để lấp snapshot cũ.
- BM01/03/08 giữ đơn vị hồ sơ. BM04–07 là bảng/sổ với nhiều dòng trong phạm vi nguồn, không tạo thao tác chọn/gộp cho người dùng.
- BM01/BM05: workbook client không có cột hoặc vùng đặt ảnh (BM01 A9:G9; BM05 A7:K7), cũng không có ảnh nhúng. Ghi chú BM01 G11 về việc đã chụp ảnh là text, không quy định nhúng ảnh vào mẫu. Khả năng appendix ảnh bổ sung của bộ xuất không trở thành yêu cầu bắt buộc của hai mẫu. Không mở thêm quan hệ ảnh của phiếu để đáp ứng tiêu chí do agent tự đặt; đề xuất đó đã được rút lại.
- Ảnh được tự lấy từ liên kết chuẩn đã lưu. Chỉ ảnh thuộc đúng hồ sơ, công trình và Đầu việc được đọc. Ảnh thiếu, mất quyền, checksum sai hoặc vượt giới hạn khiến export báo lỗi, không tạo file thiếu ảnh mà báo thành công.
- Bản xuất tối ưu cạnh dài tối đa 1280 px; JPEG chất lượng 80. PNG/ảnh có transparency giữ định dạng phù hợp. GIF dùng khung đầu và ghi rõ trong bản xuất. File gốc và checksum gốc được giữ nguyên.
- BM07 nhúng ảnh theo từng ô F8/F9… của dòng tương ứng, giữ header client hiện có. Không tạo sheet `Hình ảnh` cho BM07. XLSX dùng drawing `oneCellAnchor`, ô ảnh có giá trị trống; Google import đã kiểm chứng chuyển thành ảnh native trong ô. Tên file, ID nguồn/ảnh/revision và ghi chú ảnh phái sinh nằm trong alternative metadata của XLSX; việc Google giữ metadata này chưa xác minh. Khóa nội bộ nối từng dòng với ảnh chính xác, không nối bằng filename hoặc thứ tự của tập ảnh.
- Cột F dùng độ rộng 24 ký tự. Chiều cao dòng theo tỷ lệ ảnh và phần text lân cận; chiều cao do ảnh yêu cầu tối thiểu 54 pt và tối đa 240 pt, với khoảng dự phòng 6 pt. Ảnh rất cao được chứa trong ô theo tỷ lệ thay vì kéo giãn ảnh hoặc làm dòng quá cao. Đây là hợp đồng bố cục của file; chất lượng hiển thị/in phải được kiểm tra trong ứng dụng bảng tính.
- Phương án tương thích Google Sheets được người dùng duyệt ngày 2026-10-08 thay cấu trúc Microsoft rich-value bằng `oneCellAnchor`. Bộ xuất tạo file hoàn chỉnh ngay từ nút hiện có; không chạy script sau tải hoặc chèn lại ảnh trên Google. Excelize tạo media/relationships hợp lệ và adapter XML có kiểu dữ liệu chuẩn hóa drawing mới theo cấu trúc đã đối chứng. Không dùng URL ảnh công khai. File tải trực tiếp từ nút đã qua nghiệm thu bốn ảnh native trong ô Google, không có `#VALUE!`; bằng chứng ở `bm-gs-07-google-compatible-verification-2026-10-08.md`. XLSX vẫn là drawing, chưa đáp ứng Microsoft Place in Cell. Các mẫu khác giữ appendix; không suy rộng kết quả sang mọi Google version, Microsoft Excel hoặc in thực tế.
- Giới hạn đồng bộ: 50 hồ sơ độc lập, 250 phiên bản ảnh, 12 MiB/ảnh gốc, 256 MiB tổng ảnh gốc, 100 MiB ảnh sau tối ưu, 150 MiB workbook. Bảng/sổ có ngân sách 5000 dòng; lỗi giới hạn yêu cầu thu hẹp bộ lọc. Không truncate kết quả.
- Các route GET `.../export.xlsx` dùng guard export nặng và thời hạn xử lý. Quyền backend được kiểm tra theo nguồn; quyền download không cho phép sửa, nghiệm thu hoặc bàn giao.
- Form báo cáo thi công dùng cùng ID form đã lưu cho BM02/BM05/BM06, không lấy báo cáo live khác để thay nội dung. Chi tiết Operations có hành động vướng mắc/kế hoạch theo nguồn đang xem. Lịch sử lần gửi chỉ xuất BM02 theo revision; không gắn nút BM05/BM06 vào dữ liệu daily live thay thế lịch sử.

## Thành phần và kiểm chứng

- Download: `NativeExcelExportButton`, `bmgsExport.ts`, API client, ToastProvider.
- Ghi nhận ảnh: `EvidenceUploader`, `EvidenceCapture`, `EvidencePanel`; xem ảnh tiếp tục dùng `EvidenceRecordViewerDialog`/reference adapter chuẩn.
- Backend: `service_native_bm_gs_exports.go`, `service_native_bm_gs_owners.go`, `repository_native_bm_gs_exports.go`, các native handler và `native_work_report_evidence.go`; renderer/images là thành phần thuần dữ liệu. Service/handler/query/editor/projection cho chooser và sidecar cũ đã bị loại; V152 và snapshots lịch sử vẫn giữ nguyên.
- Adapter BM07: `bm_gs_cell_images.go`; regression resolve worksheet/drawing/relationship/media đến đúng bytes ảnh từng dòng, tỷ lệ/chiều cao động, 51 ảnh, nhiều sheet/đổi tên, tên file trùng, nguồn/revision khác nhau, thiếu/xung đột định danh, hidden/nonblank cells và ZIP/relationship/media collisions. Kiểm tra raw XML/bytes, reader Excelize và actual spreadsheet render là các bằng chứng riêng. Không thêm dependency hoặc đổi route/quyền/persistence.
- Regression cần chứng minh scope/filter/revision/ảnh, dữ liệu thiếu và số 0, ý nghĩa khối lượng/kết quả, validation trước upload, retry không upload lặp, khóa dữ liệu và migration immutable. Build hoặc tests không thay nghiệm thu giao diện.
- Nghiệm thu browser/Google Sheets dùng dữ liệu tổng hợp cô lập và các export từ tính năng gốc. Các workbook từ prototype đã bị bác bỏ không được dùng làm bằng chứng của luồng mới. Microsoft Excel, in thực tế và client acceptance vẫn là phạm vi riêng.
