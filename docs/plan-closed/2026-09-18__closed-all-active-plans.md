# [Closed] Active plans archive — 2026-09-18

Closed by user request on 2026-09-18. The archived statuses and limitations remain authoritative: closing these plans does not claim deployment, browser/UAT, or production-database migration acceptance.

## [Closed] Đề nghị theo đợt và đối soát MISA — bước 2–3 (đã duyệt)

- Duyệt: người dùng xác nhận «Duyệt 1–3» cho review package và hai Mockup as Text. MISA là nguồn chính thức; nhập CSV mẫu Consflow, không API MISA.
- IMPLEMENTED; TECHNICALLY VERIFIED trong môi trường kiểm thử tách biệt; NOT UX ACCEPTED. V145 bổ sung căn cứ xác nhận, đề nghị có phiên bản/cap theo đợt, thu hồi tạm ứng, giải phóng giữ lại, CSV preview/confirm, lịch sử thay thế và phân bổ nhiều-nhiều có lock/cap/retry UUID.
- Tái sử dụng dialog chuẩn tại Công trình, chi tiết bàn giao và Bàn làm việc; quyền V145, seed parity, observer allowlist, mapping và tài liệu import đã được cập nhật.
- Kiểm chứng trên working tree từ `9672dad`: backend focused 23 PASS; frontend focused 85 PASS; `npm run build` đạt. DB PostgreSQL 17.5 tách biệt áp dụng V145, không chạm DB công trình.
- Chưa chạy seed reset, full-suite toàn repo, browser/UAT, native MISA file/API, migration DB công trình hoặc deploy. API MISA/native export parser không thuộc scope đã duyệt.
- Bất biến: không tạo AP/AR/payment/journal, không ghi stock, không thay đổi close gate hoặc lịch sử runtime. Tiếp nhận không phải duyệt chi; số dư tại ngày và giao dịch tách riêng.

## [Closed] Hồ sơ kế toán — bước 1: truy vết nguồn (đã duyệt 2026-09-17)

- IMPLEMENTED; TECHNICALLY VERIFIED trong môi trường kiểm thử tách biệt; NOT UX ACCEPTED.
- API `GET /projects/:id/accounting-handoff-detail?source_type=&source_id=` kiểm tra quyền handoff và từng nguồn, liên kết chính xác acceptance/WorkItem/evidence/PDF cùng phiếu kho, và tách snapshot bàn giao cuối khỏi nguồn hiện tại.
- Dialog chuẩn dùng chung tại AccountingHandoffPage, MyWork inspector/mobile và AccountingWorkLane desktop/mobile; cache chi tiết được invalidation chung sau hành động/conflict. Mapping domain và inventory viewer/test đã được cập nhật.
- Kiểm chứng trên working tree từ `9672dad`: backend focused 15 PASS; frontend focused 78 PASS; `npm run build` đạt. PostgreSQL 17.5 tách biệt áp dụng V1–V144; không chạm DB công trình.
- Chưa chạy browser/UAT, deploy, hoặc kiểm chứng DB công trình thật. Không có migration, thay đổi close gate, số dư/định giá kho, hay đồng bộ MISA trong bước 1; không tự ghép nguồn theo tên hoặc material ID.
