# BM01 — Nghiệm thu xuất từ phiếu khảo sát, 13-10-2026

**Kết quả:** TECHNICALLY VERIFIED trong phạm vi phiếu khảo sát v3 đã lưu nháp; chờ anh review hình để UX ACCEPTED. Phiếu chưa Chốt hồ sơ, chưa chứng minh hoàn tất khảo sát hoặc toàn luồng Rev10. Không sửa ứng dụng, field, schema, nguồn hoặc lịch sử trong lượt này.

**Phạm vi chưa chạy:** mã nguồn thứ hai `client_site_measurement_record`, hồ sơ đã chốt, nhiều dòng/công suất lớn, vai trò khác Admin, mobile/thiết bị thật, Excel desktop và in. Không có blocker mapping hoặc lỗi ô được phát hiện trong nguồn một dòng đã kiểm tra. BM04 mobile đã được người dùng chốt trước lượt BM01; không mở lại phạm vi đó.

## Nguồn và mẫu gốc

- Công trình: `DEV-BM-REUSE-20261009`, ID `d2398f6e-83db-4171-a729-fc2e2e1557d2`.
- Đầu việc: `BM01 — Khảo sát synthetic`, ID `8faf4204-8f57-4e86-888f-01b0edae0620`, trạng thái `in_progress`.
- Phiếu: `b3dd9bb4-20d7-450e-bf0e-6e85f29ee32b`, code `client_survey_work_package`, exact pinned version `bb618ed1-6cd8-4f03-823e-c06d916e582e` v3, `draft`, revision0. Một dòng `dimension_items` có đủ kích thước, MEP, hiện trạng, đánh giá và ghi chú đã lưu từ 09-10-2026. Không nhập thêm giá trị trong UAT.
- Đã đọc trực tiếp TOC và BM-GS-01 của `/Users/khiemnguyen/Works/ttcons/documents/Bo_Bieu_Mau_Giam_Sat_Thu_Thiem.xlsx`. TOC xác định khảo sát hiện trạng trước thi công; BM01 có 7 cột A:G, không có ô ảnh. Câu mẫu G11 nhắc đã chụp ảnh là ghi chú, không phải vị trí nhúng ảnh. Nguồn hiện tại có 0 minh chứng nên chỉ một sheet dữ liệu; không tự thêm sheet ảnh.
- Native source kiểm tra liên kết Đầu việc, quyền xem, công trình active, trạng thái không cancelled/deleted và exact pinned version; không bắt buộc submitted. Vì vậy nguồn nháp đã lưu hợp lệ cho nút xuất, nhưng không đại diện nghiệm thu Chốt hồ sơ. Phiếu Rev10 v1 khác đang trống không được dùng hoặc sửa để vượt gate.

## Môi trường và source state

Development `http://127.0.0.1:5173`, đăng nhập Admin bằng phiên hiện có, backend8081 `/api/ready` HTTP200. PostgreSQL `consflow`,158 migration đã áp dụng. HEAD `75dc44707785d399ed2408959bf784928a544e96` cùng working tree đang có các thay đổi trước đó; không stage/reset/commit/push.

978 hash source/test/migration khớp trước/sau và retained build của mobile BM04: Medium `npm run build` PASS, Vite4.42s, completion observed `2026-10-13T10:56:48+0700`. Không có application edit hoặc rebuild/restart trong UAT BM01. Build được báo là retained build của cùng source state; không gọi đây là build mới. Các suite ứng dụng không chạy lại vì không đổi code.

## File thật và Google Sheets

1. Mở đúng `/work-items/8faf4204-8f57-4e86-888f-01b0edae0620`; quan sát phiếu v3 nháp, ngày đo và dòng nguồn. Không click Lưu nháp/Chốt hồ sơ hoặc sửa controls.
2. Click chính nút **Xuất báo cáo**. Browser báo `Đã tải biểu mẫu Excel`, download `/Users/khiemnguyen/Downloads/BM-GS-01 (3).xlsx`. Bản giữ `docs/acceptance-evidence/bm01-native-development-2026-10-13/BM01.xlsx` giống từng byte;11,041bytes, SHA256 `9820ef239f57f44d7063e83a857ccdaa12f2af951c218f24891e0d62e6fa45ba`.
3. Import nguyên bản giữ sang native Google Sheets bằng connector; `converted:true`, MIME `application/vnd.google-apps.spreadsheet`, `shared:false`, chỉ owner. ID `1MUtJY8Jj2Ea1L7PZtE6Qs5rpv2NG6DfU8JVhfQ1XbN8`; sheet `2026-10-09_01`, gid249616049. Không sửa cell/style/sharing. Metadata trước rồi đọc vùng bounded `'2026-10-09_01'!A1:G20`, gồm `userEnteredValue/effectiveValue/formattedValue` và định dạng.
4. 140 ô đối chiếu,35 ô có dữ liệu,0 formula,0 `effectiveValue.errorValue`,0 `#VALUE!`. Tất cả values khớp XLSX, gồm phần ký tên ở17–18. Ảnh normal Sheets100% hiển thị đủ tiêu đề, metadata,7headers,dòng nguồn và phần ký; text trong hàng10 vừa ô, không che nội dung bên cạnh. Chữ ký chỉ là placeholder nguyên mẫu, không được gọi là người đã ký.

| Nội dung mẫu | Ô xuất | Đối chiếu nguồn |
|---|---|---|
| Công trình / mã / địa điểm | B5,F5,B6 | Đúng project đã lưu |
| Ngày khảo sát | F6 | `measurement_date=2026-10-09`, không dùng ngày xuất |
| Đơn vị / đại diện khách hàng | B7,F7 | `survey_unit`, `client_representative` đã lưu |
| Vị trí / kích thước / MEP | B10,C10,D10 | `area`, `dimension` + `unit`, `mep` |
| Hiện trạng / đánh giá / ghi chú | E10,F10,G10 | `existing_condition`, `assessment`, `note` |
| 7headers / tên mẫu / phần ký | A9:G9,A1:A3,17–18 | Đúng BM01 gốc; các dòng ví dụ11–14 đã xóa nội dung |

## Bảo toàn và kiểm tra

`PYTHONDONTWRITEBYTECODE=1` + bundled Python chạy `docs/acceptance-evidence/bm01-native-development-2026-10-13/verify.py`:22/22 assertions PASS. Đây là audit evidence, không thay thế regression suite ứng dụng.

39 bảng nghiệp vụ/workflow2,515rows và snapshot construction/audit được hợp nhất tránh đếm lặp:2,891rows cũ không thay đổi hoặc mất. Chỉ thêm1audit `native_source_exported` đúng hành động xuất.149 file storage giữ nguyên hash;978 source hashes giữ nguyên; database/migrationV158 giữ nguyên. Native source form/pin/status/revision/answers được snapshot đầy đủ và nằm trong baseline không đổi.

Hai truy vấn read-only metadata từng dùng sai tên cột `current_revision_no` và `sort_order` đã lỗi rồi được sửa sau khi đọc schema `current_revision`/`order_index`. Không có ghi DB hoặc bug sản phẩm do các truy vấn này. Một lệnh đọc glob compose không tồn tại bị zsh từ chối; không có thao tác Docker mutation.

Screenshot evidence:

- `browser-source-desktop.jpg`: nguồn v3 nháp, code công trình và nút Xuất báo cáo tại1280×720.
- `browser-source-dimensions.jpg`: ngày09/10/2026 và kích thước4x3m; bảng có scroll ngang đúng source UI. Screenshot không trình bày toàn bộ cột MEP/assessment; AX và source JSON xác nhận các values đó.
- `google-main-100pct.jpg`: toàn bộ populated sheet ở Google Sheets100%, bao gồm cả phần ký17–18.

GPT-6.1 Sol High audit độc lập chạy import-only `verify()`22/22 PASS và xem3screenshots. Reviewer kiểm thêm parity TOC/BM01 trực tiếp với file client, same-project WI, exact v3 pin và audit đúng form/project/revision0/11041bytes. Không có phát hiện P0–P2 mới trong saved-draft export scope. Kết quả ghi `independent-review.md`; High không tự rerun trình duyệt/Google, không gọi nguồn submitted/fullRev10 hoặc UX ACCEPTED.

Không cần cập nhật mapping vì không đổi hành vi/owner/API/UI/schema. Kết quả này nằm trong browser→actual-download→native-Google scope được duyệt; không đóng plan hoặc triển khai Production.

**Việc tiếp theo:** anh review ảnh nút xuất và file BM01. Sau khi anh chốt, cập nhật trạng thái UX ACCEPTED cho đúng phạm vi đã kiểm tra. Các mẫu05/06/07 vẫn chờ nghiệm thu riêng.
