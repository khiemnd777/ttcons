# [Closed] Hồ sơ/Minh chứng — hành động xem và trình đọc Office

Closed: 2026-08-19
Reason: User requested closing the implemented evidence plan before commit and push.

## [Closed] Bỏ hành động `Chi tiết` trùng lặp

- Trạng thái khi đóng: `IMPLEMENTED` + `TECHNICALLY VERIFIED` trên working tree ngày 2026-08-19; `NOT UX ACCEPTED` vì chưa chạy browser/UAT theo yêu cầu repo.
- Mục đích: Mỗi bản ghi Hồ sơ/Minh chứng chỉ còn một nút xem/mở rõ ràng; tên hoặc thumbnail vẫn mở dialog chi tiết đầy đủ.

### Will be done

- Bổ sung regression test khóa hành vi không còn nút `Chi tiết` độc lập trong `EvidenceGallery`.
- Bỏ nút `Chi tiết` khỏi cả list/table và card responsive của `EvidenceGallery`.
- Giữ nguyên `Xem ảnh`, `Xem PDF`, `Mở tệp`, dialog metadata, review actions và RBAC.
- Cập nhật module/frontend/domain maps theo mapping maintenance gate.
- Chạy test tập trung và `cd frontend && npm run build`; browser/UAT không chạy vì người dùng không yêu cầu.

### Done

- Review package, impact matrix và Mockup as Text đã được người dùng xác nhận bằng `OK triển khai`.
- Đã thêm regression test xác nhận list/card không còn nút `Chi tiết` độc lập, trong khi file cell/title/thumbnail vẫn mở dialog và action vẫn giữ `Xem ảnh`/`Xem PDF`/`Mở tệp`.
- Đã bỏ hai nút `Chi tiết` trùng lặp khỏi shared `EvidenceGallery`, áp dụng cho Hồ sơ công trình, Minh chứng đầu việc và các gallery Minh chứng Thi công.
- Đã cập nhật central map, frontend UI map và domain maps của Project Planning, Workflow Orchestration, Construction Operations.
- Focused test: `node --experimental-strip-types --test tests/projectEvidenceColumns.test.ts tests/constructionOperationsPresentation.test.ts` — 11/11 pass.
- Full frontend test: `npm test` — 199/199 pass.
- Frontend build: `npm run build` — TypeScript và Vite production build pass.

## [Closed] Trình đọc Word và Excel

- Trạng thái khi đóng: `IMPLEMENTED` + `TECHNICALLY VERIFIED` trên working tree ngày 2026-08-19; `NOT UX ACCEPTED` vì chưa chạy browser/UAT theo yêu cầu repo.
- Mục đích: Người dùng đọc trực tiếp tệp Word `.docx` và Excel `.xlsx` được lưu trong Consflow mà không rời dialog minh chứng, đồng thời vẫn giữ đầy đủ ngữ cảnh hồ sơ và thao tác duyệt.

### Will be done

- Mở `.docx` nội bộ trong shared evidence dialog với trạng thái tải, đang phân tích, lỗi/thử lại, zoom và hành động `Mở tệp gốc`.
- Mở `.xlsx` nội bộ trong shared evidence dialog với tab sheet, lưới chỉ đọc có cuộn/virtualization, giới hạn preview và hành động `Mở tệp gốc`; không thực thi công thức hoặc hyperlink.
- Giữ `.doc`, `.xls`, tài liệu ngoài hệ thống và định dạng không hỗ trợ theo luồng file-first `Mở tệp`, kèm giải thích rõ cho Office đời cũ.
- Tải byte qua endpoint protected hiện có, chỉ parse khi người dùng mở; không đổi API, schema, migration, RBAC, trạng thái review hoặc completion gate.
- Bổ sung regression test cho phân loại/open-label/lifecycle và cập nhật central/frontend/workflow/planning/construction maps.
- Chạy test tập trung, full frontend test, production build và bundle-budget; browser/UAT không chạy vì người dùng chưa yêu cầu.

### Done

- Review package, impact matrix và Mockup as Text đã được người dùng xác nhận bằng `OK triển khai`.
- Đã xác nhận backend protected download hiện hữu và frontend shared dialog/blob client là owner phù hợp; không cần thay đổi backend hoặc dữ liệu.
- Đã bổ sung shared DOCX reader tải byte qua endpoint protected, lazy-load `docx-preview`, vô hiệu altChunk/comment/change, làm sạch node/handler thực thi, lọc protocol liên kết và hỗ trợ zoom/`Vừa chiều rộng`.
- Đã bổ sung shared XLSX reader tạo worker khi mở, parse SheetJS trong worker, đọc sheet theo yêu cầu và render lưới chỉ đọc có row virtualization; preview giới hạn 50 sheet, 5.000 dòng, 100 cột và 2.000 ký tự mỗi ô, không trích xuất/thực thi công thức, HTML, style, VBA hoặc hyperlink.
- Đã bổ sung preflight ZIP dùng central directory trước cả hai parser: từ chối archive lỗi/split/encrypted/ZIP64, hơn 4.096 entry, một entry giải nén trên 50 MB hoặc tổng giải nén trên 100 MB; giới hạn file nén của mỗi reader là 10 MB.
- Đã nối `Xem Word`/`Xem Excel` vào Hồ sơ công trình, Minh chứng đầu việc và hồ sơ đã lưu của Nhật ký thi công; metadata, review controls, protected auth và hành động `Mở tệp gốc` giữ nguyên. DOC/XLS đời cũ, tài liệu external và định dạng khác vẫn file-first với giải thích rõ.
- Đã thêm `docx-preview@0.4.0` và SheetJS `xlsx@0.20.3`; parser Word, worker Excel và route evidence tiếp tục tách chunk/lazy-load. Không có thay đổi backend, API, schema, migration, RBAC hoặc workflow state.
- Đã cập nhật central map, frontend UI map và domain maps của Project Planning, Workflow Orchestration, Construction Operations; mapping update là bắt buộc vì shared evidence presentation thay đổi.
- Focused Office/PDF/evidence/construction test: 21/21 pass trước full suite.
- Full frontend test: `npm test` — 205/205 pass.
- Frontend typecheck: `npm run lint` — pass.
- Production build và bundle gate: `npm run build:budget` — pass; Vite 6.4.3 build 1.848 module trong 3,23 giây, 127 chunk; entry 123,1 KiB gzip, `docx-preview` 50,1 KiB gzip, spreadsheet worker 82,5 KiB gzip, đều dưới ngân sách 250 KiB.
- Đã parse thử tệp XLSX thật được theo dõi trong repo bằng SheetJS 0.20.3: nhận 2 sheet (`Đề nghị mua`, `Chi tiết kỹ thuật`), sheet đầu có phạm vi A1:K36 và 36 dòng.
- `npm audit` hiện báo 4 advisory (1 moderate, 3 high) trong chuỗi phụ thuộc hiện hữu `nanoid`/`postcss`/`react-router`/`react-router-dom`; không có `docx-preview` hoặc `xlsx` trong báo cáo và không tự động sửa ngoài phạm vi.
