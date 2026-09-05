# [Closed] Topic hồ sơ trong Trao đổi — 05-09-2026

- Đóng theo yêu cầu user: `Đóng all plan, commit all changes, push.` ngày 05-09-2026. Đây là bản lưu lịch sử; không dùng làm durable agent memory.
- Đóng plan không xác nhận UX/UAT. Giữ nguyên phạm vi kiểm chứng và các check chưa chạy bên dưới; không còn công việc được theo dõi trong `docs/current-states.md`.
- Kiến thức bền vững về topic đã được cập nhật trong central/frontend/triage và domain maps. Lượt đóng plan chỉ lưu trữ trạng thái, không đổi behavior nên không cần bổ sung mapping.

## Topic hồ sơ trong Trao đổi — 05-09-2026

- Mục đích chính: nhận biết hồ sơ đang được trao đổi và mở đúng chi tiết trong dialog, giữ vị trí chat và draft.
- Đã duyệt: user `OK triển khai` sau mockup thẻ loại hồ sơ + tên + biểu tượng mở; dùng chung trang Trao đổi/chat thu nhỏ/composer; bỏ nhãn `Thành viên công trình` và ngữ cảnh công trình lặp dưới tin chung.
- Source ban đầu: `ad0f7b7`, working tree sạch. Chỉ kiểm chứng không dùng browser theo AGENTS.md.
- Done: đọc mapping central/frontend/backend/triage và domain; xác nhận `ChatMessage` render target bằng span, Dock chưa render target; scope_type/scope_id đã được lưu ở backend.
- Done: `ActivityTopic` provider/card dùng chung cho chat đầy đủ, card feed, Dock và preview composer; bỏ metadata project lặp, giữ phạm vi lịch sử internal/client và nút Mở đầu việc. Dialog portal phía trên Dock, quản lý focus/Escape, giữ cây conversation/composer mounted.
- Done: `activityTopicDomain` đối chiếu 10 loại source với API có sẵn; exact ID/project/status validation; nguồn thiếu/unknown không dựng link giả. `ActivityTopicDetails` chỉ đọc khi mở, query actor/project/source-bound, không giữ cache khi đóng, loading/error/retry/denied/missing. BOQ dùng lại 4 tab `BOQDetail`, Procurement dùng lại `ProcurementDetail`; nguồn khác hiển thị trường nghiệp vụ/checklist qua `RecordDetailFields`.
- Done: cập nhật central/frontend/triage và domain maps Project Planning, Commercial Procurement, Materials Warehouse, Quality Acceptance Handover, Accounting Finance.
- Chưa kiểm chứng tại thời điểm đóng: thao tác browser/UAT gồm click topic, đóng/Escape trả focus, giữ draft/vị trí chat, responsive và trường hợp lỗi từ API thật. Chưa chạy live API/DB integration hoặc Docker rebuild; không có backend/schema/migration thay đổi.
- Phạm vi kỹ thuật: movement ID dùng `/stock/movements?project_id=`, không gọi stock-document bằng movement ID; Rework/AP/AR dùng list có scope project; `/payments` legacy không có server project filter nên chọn exact ID/project từ authorized list. Quyền xem activity không được dùng thay quyền xem hồ sơ. Không có mutation source/workflow/mention/read cursor trong topic viewer.
- Trạng thái: IMPLEMENTED; TECHNICALLY VERIFIED cho TypeScript/build, bundle budget, resolver và server-render tests; NOT UX ACCEPTED. Không có lỗi check còn tồn tại trong phạm vi đã chạy.

### Evidence — source cuối

- Working tree trên `ad0f7b7`, chưa commit; frontend source/config SHA-256 `09ca6282793b0487d2a214ccf56affb86447e0cf125787acf8b46630b6f5e966` (tất cả file `frontend/src` + package/package-lock/tsconfig/vite config). Không sửa source frontend sau build này.
- Environment: local macOS, Node `v23.11.0`, Vite `6.4.3`; không dùng browser/headless. DB và migration state không được kiểm tra hoặc thay đổi trong lượt này.
- `node --experimental-strip-types --test tests/activityTopic.test.ts tests/activityTopicRendering.test.ts tests/activityChatInput.test.ts tests/utilityDock.test.ts`: 25 passed, 0 failed, 0 skipped. Rendering tests dùng React server renderer với auth/details boundary stubs; không chứng minh tương tác DOM, quyền live hoặc UX.
- `cd frontend && npm run build`: exit 0; 05-09-2026 15:36:52–15:37:11 ICT (08:36:52–08:37:11 UTC), tổng 19.37s, Vite 5.47s.
- `cd frontend && npm run check:bundle-budget`: exit 0; 120 chunks, entry 299.80 kB raw / 79.5 KiB gzip; giới hạn từng chunk 500 kB raw / 250 KiB gzip.
- `git diff --check`: không có lỗi whitespace. Build log: `/tmp/consflow-topic-build.log`; bundle log: `/tmp/consflow-topic-bundle-budget.log` (artifact tạm thời).
