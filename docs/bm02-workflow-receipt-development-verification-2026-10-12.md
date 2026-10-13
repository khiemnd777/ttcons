# BM02 — Phiếu quy trình đã chốt, vật tư nhập trường và lịch sử gửi

**Trạng thái tổng: BLOCKED.** Phiếu nhận vật tư hợp lệ đã ghi nhận 12 m² nhưng BM02 không tự lấy được dòng vật tư. Phần nội dung, ảnh và lịch sử hai lần gửi đã được kiểm chứng trên Development. Chờ anh review hình; chưa chốt nghiệm thu phạm vi này.

## Kết quả và giới hạn

| Tiêu chí | Quan sát thực tế | Trạng thái |
| --- | --- | --- |
| Tự tận dụng vật tư nhập trường | Phiếu nhận cùng công trình/ngày, đã gửi, giao 12 m²/đạt 12 m². BM02 D12 vẫn báo chưa ghi nhận; E25:G27 trống | **FAIL** |
| Phiếu quy trình đã chốt | Cùng phiếu, cùng định nghĩa v5 có 11 field; chốt Lần 1, sửa chính thức thành phiên bản 2; không thêm field | TECHNICALLY VERIFIED |
| Nội dung và ảnh lần gửi cũ | Sau sửa phiếu và gửi Lần 2, Lần 1 giữ 2 m², 3 người, 8 giờ, ảnh đỏ; Lần 2 có 9 m², 6 người, 7 giờ, ảnh xanh | TECHNICALLY VERIFIED |
| Nút xuất thật | Tải 5 XLSX qua nút trên phiếu và chi tiết lần gửi; các bản lưu bằng nguyên byte file Downloads | TECHNICALLY VERIFIED |
| Google Sheets | Nhập nguyên 4 file; so khớp 92/92/96/96 ô có dữ liệu/công thức. Không có effectiveValue.errorValue trong vùng kiểm tra; tổng nhân lực tính lại 3/6/6/3; ảnh đúng bản | TECHNICALLY VERIFIED |
| Tiến độ và đánh giá chưa ghi | Không biến tiến độ thiếu thành 0%; 12 trạng thái vẫn Chưa đánh giá, không suy luận Đạt từ dữ kiện | TECHNICALLY VERIFIED |
| Nhãn thời tiết | Phiếu gốc hiển thị Nắng nóng/Mưa; file B6 và các hộp thoại báo cáo dùng hot/rain | Lỗi trình bày đã quan sát, chưa sửa |
| UX cuối cùng | Anh chưa review các hình/file của lượt này | PENDING |

Không kiểm tra Production, toàn bộ quy trình chính thức upstream, các BM khác, Excel desktop, in giấy, điện thoại, ảnh camera thật hoặc tải tối đa. Fixture dùng hai giai đoạn hợp lệ tối thiểu, không đại diện toàn bộ Rev10. Google CellData không chứng minh pixels ảnh; ảnh được kiểm tra bằng màn hình Google thật và đối chiếu media trong XLSX riêng.

## Lỗi vật tư: nguyên nhân đã kiểm chứng

Phiếu nhận thuộc Đầu việc `58969f9e-8a27-44b1-b2d9-7759442c61a5`. Phiếu báo cáo thuộc Đầu việc `40ccc567-6b74-4925-b4b4-72e1cae418f1`. Hai hồ sơ cùng công trình `b705236b-9cc5-42dd-ac2d-abf77e699cc4`, ngày nghiệp vụ 12-10-2026.

`native_daily_source_reuse.go:73` yêu cầu cả chủ sở hữu phiếu nhận và dòng vật tư phải bằng Đầu việc báo cáo. `incomingAcceptanceWorkItemPolicy` chỉ cho phiếu nhận thuộc đúng Đầu việc nhận vật tư; `syncConstructionDailyReportTx` giữ đúng Đầu việc của phiếu báo cáo. Luồng chuẩn vì vậy tạo hai chủ sở hữu riêng, không thỏa điều kiện mapping.

Đã tái hiện bằng Create/Submit phiếu nhận, Complete/Advance bước nhận vật tư, Submit phiếu báo cáo và Send/Amend/Resend báo cáo qua API/UI chính thức. Không chèn SQL, không ghép chủ sở hữu, không bỏ guard, không nhập lại vật tư vào phiếu báo cáo. Bằng chứng tối thiểu: `receipt-after.json`, `form1-submitted.json`, `delivery-after.json`, D12/E25:G27 của cả 5 file và hình 05/12/17/20.

Nhận định trước đây rằng chỉ thiếu fixture nghiệm thu không đầy đủ. Có fixture hợp lệ rồi nhưng mapping vẫn loại dữ liệu. Các test source-reuse cũ chèn trực tiếp dòng nhận vật tư vào DB với cùng Đầu việc nên không phát hiện lỗi kết nối luồng này.

## Nguồn, runtime và cách kiểm tra

- HEAD `75dc44707785d399ed2408959bf784928a544e96`, giữ working tree đang có. Không đổi ứng dụng/schema/migration trong lượt nghiệm thu. Manifest 962 file khớp trước/sau; SHA256 của bản manifest nền `1529289ef6e24d4cfce6fefdf681a0e83725870c1118a62c72d15d717f9ba219`.
- Development UI `http://127.0.0.1:5173`, API `http://127.0.0.1:8081/api`; health=ok, ready.database=ok. Container backend/admin tạo 02:41:09 UTC ngày 12-10-2026. Backend image `sha256:30537ea8444f792226d69074c8b2207a9d9805324a7f82e225e781df5e132949`; admin image `sha256:a5b9de8273c63be8d8a351451f1cdc5eb9f4942cf3c7cb19bdf06a6d5722ee28`.
- Dùng bản build cuối đã ghi trong [nghiệm thu Nhật ký](journal-progress-development-verification-2026-10-12.md), source không đổi. Không chạy lại build/test ứng dụng vì lượt này chỉ tạo hồ sơ thử và đọc bằng chứng. Không coi kiểm chứng file là test bản sửa lỗi vật tư.
- DB `consflow`, 157 migration; 5→6 công trình và 20→21 phiếu do fixture mới. Các ID/hash dòng có trước trong 8 bảng đã chụp baseline giữ nguyên: projects, work_items, workflow_form_instances, workflow_form_events, operation_daily_reports, work_report_revisions, incoming_acceptance_records, incoming_acceptance_items. Baseline hai bảng nhận vật tư trống; không suy diễn đã thử bảo toàn phiếu nhận cũ.
- Công trình thử `DEV-BM02-WORKFLOW-20261012`. Tạo template thử với hai chủ sở hữu nhận vật tư/báo cáo riêng và cạnh nhận→báo cáo, dùng định nghĩa phiếu đã xuất bản v5. Lần publish đầu bị từ chối vì thiếu cạnh; thêm cạnh chuẩn rồi publish, không bỏ validation. Người nhận thử đã tồn tại và được thêm PM trong công trình thử mới; không cấp quyền global.
- Phiếu `a6edf273-25fb-4a49-aaa5-fdd525c4c4a2`; báo cáo `2bed89af-0988-4c6d-a4d5-23ee7fbc7f86`. Lần 1 snapshot SHA256 `f9b3b922f8a5f4b08640d78298f2631dd13f3561def3f36bd1fd11cb9341563c`; Lần 2 `070dd290323eefbd0b89fcaccca6efb33f855200aaf4ba5be23c9bd006e6b49a`.
- Giữ toàn bộ record lần gửi 1 và các event phiếu có trước sửa; bản cũ tải lại giữ nguyên XML sheet/style/media. Chỉ thứ tự khai báo MIME trong `[Content_Types].xml` khác, tập khai báo bằng nhau. Không tuyên bố hai ZIP có cùng SHA256 toàn file.
- Root đọc ảnh gốc qua endpoint bảo vệ từng lần gửi với tài khoản người nhận; byte khớp ảnh đã upload. Chỉ một ảnh đúng revision trong mỗi XLSX dù Đầu việc còn giữ cả hai ảnh.
- Script `verify_acceptance.py` chạy bằng bundled Python, exit0, **90 kiểm tra bằng chứng đạt**, trạng thái tổng vẫn BLOCKED vì một tiêu chí nghiệp vụ FAIL. Script đọc API/DB/XLSX/JSON, làm mới login bình thường và ghi evidence; không sửa file xuất, Sheet hoặc hồ sơ ứng dụng. Hai lỗi cấu hình checker đã sửa: tên container thật là `admin`; đáp án tùy chọn trống có thể được normalization bỏ key. Không giảm tiêu chí nghiệp vụ để có kết quả đạt.
- Google đọc metadata trước, vùng `'2026-10-12_01'!A1:H75`, `'Hình ảnh'!A1:H5`. Kiểm tra cả ô trống để phát hiện dữ liệu thừa. Trình duyệt nhìn cả sheet dữ liệu và sheet ảnh ở 100%; giữ nguyên nội dung, kích thước và format nhập từ file. 26 ảnh chụp đã xem trực tiếp.
- Reviewer độc lập GPT-6.1 Sol High kiểm tra source và tính lại bằng chứng lưu, xác nhận lỗi vật tư và giới hạn kết luận. Reviewer không lặp HTTP/browser; root đã chạy những thao tác đó.

## File và hình để review

Các file dưới đây là bản tải thật, không chỉnh lại bằng openpyxl hoặc Google Sheets.

| File | Nút/source | Byte | Google Sheets |
| --- | --- | ---: | --- |
| [Phiếu đã chốt Lần 1](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/BM02-phieu-chot-lan-1.xlsx) | Xuất báo cáo ngày trên Đầu việc | 28328 | [Mở phiếu Lần 1](https://docs.google.com/spreadsheets/d/1s898qdoJ7pF2DZ0ujRoDtxZUR5s-AEm3KkZ5e3L3Z6k/edit) |
| [Lần gửi 1 trước sửa](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/BM02-lan-gui-1-truoc.xlsx) | Xuất báo cáo tại chi tiết Lần 1 | 28529 | Đối chiếu ZIP với bản tải lại bên dưới |
| [Phiếu đã sửa Lần 2](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/BM02-phieu-chot-lan-2.xlsx) | Xuất báo cáo ngày trên cùng Đầu việc | 28055 | [Mở phiếu Lần 2](https://docs.google.com/spreadsheets/d/1dnubHM-M_vTG5AaJddf8EBzHXb87q24Z9yaYnvkJd4U/edit) |
| [Lần gửi 2](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/BM02-lan-gui-2.xlsx) | Xuất báo cáo tại chi tiết Lần 2 | 28257 | [Mở Lần gửi 2](https://docs.google.com/spreadsheets/d/1jOslmTKlC5N2JyQSOgGhtzOONELJDPLhSG-rqBEKeeA/edit) |
| [Lần gửi 1 tải lại sau sửa](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/BM02-lan-gui-1-sau.xlsx) | Chọn Các lần gửi→Lần 1→Xem nội dung→Xuất báo cáo | 28530 | [Mở Lần gửi 1 tải lại](https://docs.google.com/spreadsheets/d/1VR1VYxhu2ergtHsHNcewOaLX6pQ9EN2OAVLRtlEfjxU/edit) |

Ảnh chính:

| Màn hình | Ảnh |
| --- | --- |
| Phiếu đã chốt và ba nút xuất trên Đầu việc | [01](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/01-form-submitted.jpg) |
| Phiếu nhận hợp lệ 12 m² | [05](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/05-valid-receipt-12m2.jpg) |
| Người nhận yêu cầu bổ sung | [06](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/06-recipient-request-changes.jpg) |
| Cùng phiếu đã sửa phiên bản 2 | [07](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/07-form2-submitted.jpg) |
| Hai lần gửi và nút xem bản cũ | [10](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/10-two-revisions.jpg) |
| Nút xuất bản cũ sau sửa | [11](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/11-delivery1-after.jpg) |
| Google: nội dung và dòng vật tư thiếu của Lần 2 | [17](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/17-google-delivery2-main.jpg) |
| Google: tổng nhân lực và nội dung Lần 2 | [26](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/26-google-delivery2-footer.jpg) |
| Google: ảnh xanh Lần 2 | [19](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/19-google-delivery2-photo.jpg) |
| Google: Lần 1 giữ nội dung cũ | [20](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/20-google-delivery1-after-main.jpg), [21](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/21-google-delivery1-after-lower.jpg) |
| Google: Lần 1 giữ ảnh đỏ | [22](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/22-google-delivery1-after-photo.jpg) |

[Kết quả chi tiết và hash file](acceptance-evidence/bm02-workflow-receipt-development-2026-10-12/acceptance-checks.json). Tất cả 26 ảnh nằm cùng thư mục; phần còn lại gồm phiếu nhận hoàn tất, gửi lần đầu/gửi lại, chi tiết lần 2 và các phần chính/cuối/ảnh của hai file phiếu.

## Review package — sửa riêng mapping vật tư, chưa triển khai

**Mục tiêu:** BM02 tự tận dụng phiếu vật tư đã có, không yêu cầu nhập lại, không thêm field hoặc thao tác chọn nguồn.

Hiện tại query so chủ sở hữu Đầu việc; kỳ vọng đề xuất là lấy phiếu nhận **cùng công trình và ngày nghiệp vụ**. Đây là lựa chọn phạm vi nghiệp vụ cần anh duyệt: các báo cáo của nhiều Đầu việc trong cùng công trình/ngày sẽ cùng nhìn thấy những phiếu nhận mà người lập được phép xem. Không được diễn giải các phiếu đó thành vật tư riêng của một Đầu việc báo cáo.

1. Đổi nguồn đọc thành phiếu nhận đã gửi trong công trình/ngày báo cáo. Áp quyền xem nghiệm thu của công trình và quyền xem Đầu việc chủ sở hữu theo contract GetIncomingAcceptance trước khi lấy, đếm hoặc giới hạn dòng. Giữ receipt/item/owner IDs, số lượng đã giao, đơn vị và ghi chú để truy vết. Dòng nhập thủ công không rỗng vẫn có ưu tiên. Không biến nhà cung cấp thành xuất xứ hoặc số lượng đạt thành số lượng đã giao.
2. Đóng băng dữ kiện khi chốt/sửa phiếu. Với báo cáo từ quy trình, lần gửi phải dùng chính dữ kiện đã đóng băng của phiên bản phiếu tương ứng, không truy vấn lại phiếu nhận phát sinh sau khi chốt. Báo cáo nhập trực tiếp vẫn đóng băng tại thời điểm gửi. Giữ các snapshot trước sửa nguyên trạng; không backfill âm thầm.
3. Bổ sung test qua canonical Create/Submit phiếu nhận và Submit/Amend/Send báo cáo với hai Đầu việc riêng; kiểm tra cùng/ngược công trình/ngày, quyền bị từ chối, nhiều phiếu/dòng, nhập thủ công ưu tiên, giới hạn 200 dòng được phép xem và phiếu nhận đến sau chốt. Cập nhật Development rồi tải lại nút xuất và kiểm tra Google Sheets/hình trước khi đề nghị chốt nghiệm thu.

Files dự kiến: `backend/internal/core/native_daily_source_reuse.go`, `repository_incoming_acceptance.go`, `native_workflow_source_reuse.go`, `native_work_report_evidence.go`; tests tương ứng `native_daily_source_reuse_test.go`, `native_workflow_source_reuse_test.go` và native BM export integration tests. `bm02_projection.go` đã đọc material_arrival_items, không cần tạo field. Cập nhật central/backend/root-cause/domain mapping cùng lượt triển khai theo gate; UI không đổi nên không có mockup mới. Lượt nghiệm thu hiện tại không thay hành vi, chưa sửa mapping.

Rủi ro cần kiểm chứng: phạm vi project/day rộng hơn work-item; tránh lọt nguồn không có quyền và tránh tính limit theo nguồn bị ẩn. Tách việc nhận trường khỏi sử dụng vật tư, stock movement và quyết định chất lượng. Tính bất biến form/delivery phải được test với nguồn đến sau chốt. Nhãn thời tiết là lỗi trình bày riêng, nằm ngoài đề xuất sửa vật tư này.

**Bước tiếp theo:** chờ anh duyệt 1–3 ở trên. [AGENTS.md:9](../AGENTS.md) yêu cầu “first present a review package and wait for explicit user confirmation before implementation.” Yêu cầu hiện tại duyệt nghiệm thu, chưa duyệt thay đổi phạm vi đọc vật tư từ Đầu việc sang công trình/ngày.
