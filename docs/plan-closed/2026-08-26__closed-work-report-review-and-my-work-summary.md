# [Closed] Work Report review and My Work summary — 2026-08-26

Closed by user request on 2026-08-26.

## [Closed] Chia tab con cho Chờ tôi duyệt

Trạng thái: `IMPLEMENTED`; `TECHNICALLY VERIFIED` bằng full frontend test, TypeScript/Vite production build, bundle budget và local Vite source delivery trên working tree hiện tại; `NOT UX ACCEPTED` vì chưa chạy browser/UAT theo giới hạn phiên làm việc. Không đổi API, count, quyền hoặc database.

### Will be done

- Chạy browser/UAT khi người dùng yêu cầu để xác nhận trực quan desktop/mobile, back/forward URL state và deep link node trên build hiện tại.

### Done

- Đã xác nhận owner `frontend/src/modules/my-work/MyWorkPage.tsx`, ba nguồn dữ liệu độc lập và invariant top-level count hiện hữu.
- Đã duyệt Mockup as Text và review package; không cần backend, type contract hay migration.
- Đã chia `Chờ tôi duyệt` thành ba tab responsive `Phê duyệt công việc`, `Phê duyệt giai đoạn quy trình`, `Báo cáo chờ xác nhận`; mỗi tab giữ count/loading/error riêng và chỉ active lane được render.
- Đã giữ bộ lọc công trình dùng chung, thêm presentation state `approval_type=work|node|report`, ưu tiên `approval=<request_id>` cho node và chọn ổn định lane đầu tiên có dữ liệu khi URL không chỉ định.
- Đã giới hạn desktop `Việc tiếp theo` cho Work Item; node/report dùng toàn bộ chiều rộng. Quyền Work Report, API, mutations, top-level count và SSE invalidation không đổi.
- Đã cập nhật central/frontend/workflow/reporting maps và focused source-contract regression.
- Đã chạy `npm test -- myWorkListPresentation.test.ts` (test script chạy full suite) đạt 275/275, `npm run build` đạt, `npm run check:bundle-budget` đạt 137 chunks với entry 134.2 KiB gzip, và `git diff --check` đạt.
- Local Docker stack vẫn running/healthy; Vite tại `127.0.0.1:5173` đã trả transformed `MyWorkPage.tsx` có `approval_type` và ba tab panel mới. Không cần restart backend vì backend không thay đổi.

## [Closed] Sửa badge Bàn làm việc cộng đúng hai luồng hành động

Trạng thái: `IMPLEMENTED`; `TECHNICALLY VERIFIED` cho regression thuần, full backend test, full frontend test, production build và local Docker stack; DB-backed regression bị skip do chưa có `CONSFLOW_TEST_DATABASE_URL`; `NOT UX ACCEPTED` vì chưa chạy browser/UAT theo giới hạn phiên làm việc.

### Will be done

- Chạy DB-backed regression trên test database riêng khi có `CONSFLOW_TEST_DATABASE_URL`.
- Chạy browser/UAT trên build hiện tại khi người dùng yêu cầu; xác nhận sidebar desktop/mobile sau khi tạo và xử lý từng nguồn chờ duyệt.

### Done

- Đã xác nhận root cause: shell dùng `navigation-summary.total`, trong khi backend total cũ chủ động loại toàn bộ decision lane.
- Đã duyệt Mockup as Text và loại `Trao đổi cần phản hồi` khỏi công thức badge.
- Đã mở rộng `GET /me/work-items/navigation-summary` thành projection toàn cục, actor-scoped cho đúng hai luồng đã duyệt.
- `work_total` giữ Work Item cần làm trong phạm vi `responsible` và current-Node workflow action sẵn sàng; `approval_total` cộng Work Item actor có thể quyết định, pipeline-node approval actor có thể quyết định và Work Report gửi đúng actor có quyền review/manage.
- Đã thêm breakdown contract, giữ invariant `total = work_total + approval_total` và xác nhận không có attention trong response.
- Đã đồng bộ type frontend và invalidation để `work_report_changed` làm mới badge `Bàn làm việc`.
- Đã thêm regression DB cho trường hợp `0 Công việc + 1 báo cáo chờ duyệt = Bàn làm việc 1` cùng regression thuần cho công thức tổng.
- Đã cập nhật central/backend/frontend/root-cause/workflow/reporting maps; không cần migration.
- Đã chạy `gofmt`, `git diff --check`, backend regression thuần và `GOCACHE=/tmp/consflow-go-cache go test ./...` đạt.
- Đã chạy `npm test` đạt 274/274 và `npm run build` đạt trên working tree hiện tại; Vite còn cảnh báo chunk lớn không chặn. DB-backed Work Report test skip rõ vì thiếu `CONSFLOW_TEST_DATABASE_URL`.
- Đã chạy `make restart` theo pipeline `down -> build -> run`; backend/admin được build lại từ working tree hiện tại, PostgreSQL/Redis healthy và `GET /api/health` trả `200 OK`. Không có migration mới.

## [Closed] Thiết kế lại duyệt báo cáo thi công và minh chứng theo revision

Trạng thái: `IMPLEMENTED`; `TECHNICALLY VERIFIED` cho build, test thuần và contract frontend; DB-backed integration bị skip do chưa có `CONSFLOW_TEST_DATABASE_URL`; `NOT UX ACCEPTED` vì chưa chạy browser/UAT theo giới hạn phiên làm việc.

### Will be done

- Chạy lại hai bài integration Work Report trên database test khi có `CONSFLOW_TEST_DATABASE_URL`.
- Chạy browser/UAT trên build hiện tại khi người dùng yêu cầu xác nhận trực quan; kiểm tra desktop, mobile, ảnh/PDF/Word/Excel và hai quyết định review.

### Done

- Đã xác nhận review package và Mockup as Text theo 5 điểm.
- Đã kiểm tra owner `WorkReportsPage`, Work Report service/repository, snapshot `photos`/`primary_evidence_id`, gallery/viewer dùng chung và chênh lệch quyền Work Report/Work Item.
- Đã xác nhận không cần migration cơ sở dữ liệu.
- Đã thêm evidence metadata theo từng immutable revision và endpoint tải tệp có kiểm tra quyền Work Report, phạm vi author/recipient/manager, exact revision và snapshot membership trước khi mở storage.
- Đã tách `WorkReportReviewDialog.tsx`: summary có tiến độ, ba tab, nội dung hai cột, gallery read-only nhấn mạnh ảnh đầu, review rail sticky, trạng thái lịch sử read-only và CTA `Xác nhận đã xem` phân cấp chính.
- Đã mở rộng `EvidenceGallery`/`EvidenceRecordViewerDialog` bằng file-path override tùy chọn; các caller hiện hữu giữ nguyên endpoint Work Item.
- Đã thêm type/contract và hồi quy cho revision evidence; không thay đổi state machine, audit, notification, PDF hay nguồn sửa tại Thi công.
- Đã cập nhật central/backend/frontend/reporting/construction maps.
- Đã chạy `gofmt`; `GOCACHE=/tmp/consflow-go-cache go test ./...` đạt; helper snapshot evidence đạt. Hai bài integration được test runner skip rõ do thiếu database test URL.
- Đã chạy `npm test` đạt 274/274, `npm run build` đạt và `npm run check:bundle-budget` đạt trên working tree hiện tại; Vite chỉ còn cảnh báo chunk lớn đã có tính chất không chặn.
