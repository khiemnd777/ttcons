# BM04 — Sửa cột thao tác Development, 13-10-2026

**Cập nhật nghiệm thu:** user đã chốt riêng cột thao tác desktop. Phạm vi desktop là UX ACCEPTED. Nút nổi `Tiện ích` vẫn che một phần `Gửi khắc phục` trên mobile390×844; user yêu cầu lập kế hoạch/mockup cho phần này, chưa duyệt triển khai. Toàn bộ mobile/BM04 chưa được chốt.

**Đã kiểm chứng:** lỗi cột thao tác desktop được sửa. Các nút nằm trong cột và cách tên việc24px tại1024/1280/1440px. Development đang hiển thị source mới; nhãn, dữ liệu, lịch sử và file xuất được bảo toàn.

## Phạm vi và bản chạy

- User duyệt sửa đúng cột thao tác theo mockup, không thêm field, giữ nhãn/dữ liệu/lịch sử; cập nhật Development và nghiệm thu trình duyệt, chụp ảnh. Không mở rộng sang mobile utility, API, migration, lifecycle, Production, Git delivery hay đóng plan.
- HEAD `75dc44707785d399ed2408959bf784928a544e96` cùng working tree đang có được giữ nguyên. So với baseline trước sửa, chỉ `frontend/src/modules/operations/ConstructionWorkspaceLists.tsx` thay đổi trong976 file source/test/migration đã ghi hash. Manifest hiện tại vẫn trùng source đã build.
- `admin` chạy Vite tại `http://127.0.0.1:5173`, bind thư mục frontend hiện tại vào `/app`. Cập nhật qua HMR và browser Reload; không cần restart toàn stack. Backend tại8081, database consflow/V158 không thay đổi. Không chạy migration.
- Build Medium: `cd frontend && npm run build` PASS, gồm tsc và Vite4.54s; dist index10:26:52ICT13 October. Đây là kết quả thực thi worker đã báo; không giả lập root build log. `git diff --check` PASS. High review read-only delta hai dòng, không có P0–P2 mới trong phạm vi sửa.

## Thay đổi và kiểm tra thực tế

Mục đích chính của danh sách là xem và xử lý việc phát sinh; thao tác phải nằm riêng trong cột. Cột cũ ép chiều rộng nhóm flex trong khi CTA giữ chiều rộng, gây lấn12,34375px vào tên việc ở1280×720. Hai dòng JSX được sửa để header/cell đầu co theo nội dung và nhóm nút giữ chiều rộng thực tế: `w-px`, `whitespace-nowrap`, `w-max`. Không hardcode kích thước pixel.

Tái sử dụng `RowActions`, `PrimaryButton`, `NativeExcelExportButton` và scrollport `consflow-table-viewport`. Handler, điều kiện quyền, nhãn, bảng Kiểm tra và card mobile giữ nguyên.

| Viewport | Kết quả quan sát |
|---|---|
| 1280×720 | Cột244,48px; nhóm220,48px; khoảng cách tên24px; chồng lấn0px |
| 1024×768 | Các nút vẫn nằm trong cột; bảng cuộn ngang thực tế137,5px; toàn trang không tràn ngang |
| 1440×900 | Các nút nằm trong cột; khoảng cách24px; chồng lấn0px |
| 390×844 | Bảng desktop ẩn, card hiện có hiển thị; trang không tràn ngang. Nút nổi Tiện ích che một phần CTA ở cuối trang |

- Actor Admin, một source thử đã lưu: project `DEV-BM04-REUSE-20261009`, action `ae4aba28-043c-48fa-8125-6b6da4a313d4`, trạng thái open. Không tạo source hoặc đổi trạng thái.
- Đã mở xem chi tiết, tải hai nút Xuất báo cáo, mở Chỉnh sửa rồi Hủy, mở Gửi khắc phục rồi Hủy. Không lưu, gửi xác nhận hoặc upload. Bộ lọc Đã xong0 cho trạng thái rỗng và nút export bị vô hiệu hóa. Reload giữ source hiện có.
- File mới tải thật từ browser: `danh-sach-can-xu-ly-BM-GS-04 (4).xlsx` và `BM-GS-04 (4).xlsx`. Các bản lưu trong thư mục evidence giữ nguyên byte tải xuống. Toàn bộ ô/công thức và media ảnh bằng hai file đã kiểm Google trước sửa; vị trí, tổng1/0/1 và trạng thái không đổi. Không import Google lại trong lượt sửa CSS này.
- Verifier evidence17/17 PASS:2.887 unique bản ghi cũ không đổi/không mất; chỉ thêm hai audit native_source_exported.149 file storage giữ nguyên; V158 không đổi;976 source hiện tại trùng manifest build.
- Audit cuối độc lập GPT-6.1 Sol High import-only `verify()` đạt17/17 và xem9 screenshots. Xác nhận desktop không có P0–P2 mới, export/history/build parity đúng; P2 mobile hiện có vẫn còn. Reviewer không chạy browser riêng hoặc import Google lại; không nâng trạng thái thành UX ACCEPTED.
- Một truy vấn audit dùng nhầm tên cột đã thất bại read-only; sửa theo V1 schema event_type/target_type/target_id. Một assertion của verifier ban đầu chọn nhầm nhánh mobile Kiểm tra; sửa phạm vi về ConstructionActionList rồi chạy lại. Đây là lỗi công cụ thu bằng chứng, không phải lỗi ứng dụng.

## Bằng chứng và việc tiếp theo

Ảnh [desktop1280](acceptance-evidence/bm04-list-detail-development-2026-10-13/action-column-fix/desktop-1280.jpg), [desktop1024 sau cuộn](acceptance-evidence/bm04-list-detail-development-2026-10-13/action-column-fix/desktop-1024-scrolled.jpg), [desktop1440](acceptance-evidence/bm04-list-detail-development-2026-10-13/action-column-fix/desktop-1440.jpg), [chi tiết/nút xuất](acceptance-evidence/bm04-list-detail-development-2026-10-13/action-column-fix/detail-export.jpg), [empty state](acceptance-evidence/bm04-list-detail-development-2026-10-13/action-column-fix/empty-state.jpg), [mobile và lỗi nút nổi](acceptance-evidence/bm04-list-detail-development-2026-10-13/action-column-fix/mobile-card.jpg).

[Measurements](acceptance-evidence/bm04-list-detail-development-2026-10-13/action-column-fix/browser-measurements.json), [verifier](acceptance-evidence/bm04-list-detail-development-2026-10-13/action-column-fix/verify.py), [kết quả](acceptance-evidence/bm04-list-detail-development-2026-10-13/action-column-fix/verification.json), [file danh sách](acceptance-evidence/bm04-list-detail-development-2026-10-13/action-column-fix/BM04-list.xlsx), [file chi tiết](acceptance-evidence/bm04-list-detail-development-2026-10-13/action-column-fix/BM04-detail.xlsx).

Mapping central/frontend/triage đã cập nhật owner/layout và vấn đề mobile riêng. Backend/API/schema/domain invariant không đổi nên không cần sửa các map tương ứng. Không thêm test chỉ kiểm classname; phép đo browser là tín hiệu regression cho lỗi này. Không chạy full backend suite, role/lifecycle/multirow/capacity, Excel desktop hoặc in giấy vì không thuộc phạm vi sửa. Viewport thử đã reset.

**Bước tiếp theo:** review kế hoạch/mockup mobile trong current-states và phản hồi của lượt lập kế hoạch. Phương án đề xuất đưa cùng nút Tiện ích có nhãn vào hàng riêng của sticky project header; chưa sửa ứng dụng. Không suy ra nghiệm thu mobile/toàn BM04 từ việc chốt cột desktop.
