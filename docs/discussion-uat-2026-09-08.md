# Kiểm thử trải nghiệm dự án hướng Thảo luận — 08-09-2026

**Mục đích chính:** kiểm tra thành viên có thể cập nhật hồ sơ và xử lý công việc ngay trong Trao đổi, đồng thời giữ được ngữ cảnh và nội dung đang nhập.

## Kết luận và giới hạn

**NOT UX ACCEPTED.** Các luồng chính đã được chạy trên Edge với dữ liệu UAT riêng, nhưng còn lệch dialog mobile, kết nối trực tiếp bị HTTP 429 khi có nhiều tab và trạng thái đầu việc thiết kế vẫn `ready` sau phát hành. Chưa sửa mã ứng dụng trong lượt kiểm thử này.

- Upload/phát hành thiết kế và chốt checklist được chạy bằng Admin. Tài khoản PM mẫu đã xem thẻ revision, trả lời và bàn giao cho QS. Tài khoản Mua hàng mẫu đã mở đúng đầu việc, sửa và gửi yêu cầu mua.
- Chưa kiểm thử upload bằng tài khoản chỉ có quyền Thiết kế: không có thông tin đăng nhập test phù hợp đã được xác minh. Không thay mật khẩu hoặc cấp thêm quyền toàn hệ thống.
- Nút xem PDF đã mở một tab `blob:`. Công cụ trình duyệt chặn đọc URL này; không xác nhận đã xem trực quan nội dung PDF. Không dùng cách khác để vượt chặn.
- Mobile là viewport Edge 390 × 844, không phải điện thoại thật. Chưa kiểm thử bàn phím ảo, camera, landscape, mất mạng giữa chừng, upload nhiều tệp thành công một phần hoặc đóng hẳn trình duyệt.
- Chưa chạy R02 và kiểm thử nhìn lại R01 sau R02 trên trình duyệt. Việc lịch sử mua sắm giữ dữ liệu trước đặt hàng đã được kiểm tra trực tiếp.
- Đây là fixture có ba bước có thể thao tác trong một node tổng hợp. Không chứng minh toàn bộ quy trình chính thức từ khởi tạo đến bàn giao.

## Môi trường và nguồn đã thử

- Thời gian: 08-09-2026, khoảng 20:38–21:07 +07.
- Branch `codex/discussion-centered-project`, HEAD nền `46a8cec`, working tree chưa commit chứa cả hai đợt đã duyệt.
- SHA-256 backend/frontend: `25b22243020842d08dd8ff7db69f8645dfa5be18319981dc7fcc6035cb9e3a34`, đối chiếu lại trong lượt này và khớp bản đã build.
- Local Docker: frontend `http://localhost:5173`, backend `http://localhost:8080`, PostgreSQL 16, custom migrations V139. Không tạo migration mới.
- Không rebuild/re-run unit tests vì không đổi mã ứng dụng. Build gần nhất của đúng nguồn này: `npm run build:budget`, 20:21:56 +07; các kiểm tra kỹ thuật trước đó nằm trong `docs/current-states.md`.
- Công trình mới: `UAT-DISC-260908`, ID `1ecb5a43-97b0-4995-bebc-88696f073ca5`. Dữ liệu, nhà cung cấp, PO, PDF và bình luận đều được đánh dấu UAT. Fixture được tạo trong DB local hiện hành, không phải DB integration-test riêng.
- Setup tạm: `/private/tmp/consflow-discussion-uat-setup.sql`; PDF giả lập: `/private/tmp/consflow-uat-design-r01.pdf`. Hai lần setup đầu bị ràng buộc schema từ chối và rollback toàn transaction trước lần tạo thành công. Quyền bước mua trong fixture sau đó được chỉnh từ `operation.report` sang `procurement.create` cho đúng tài khoản Mua hàng; đây là điều chỉnh fixture.
- Đã khôi phục phiên Admin, reset viewport, giữ tab công trình UAT để xem kết quả. Tab công trình ban đầu vẫn ở URL ban đầu.

## Kết quả quan sát

| Kịch bản | Kết quả và phạm vi bằng chứng |
| --- | --- |
| Việc của tôi | Admin thấy ba đầu việc; Mua hàng thấy đúng một đầu việc mua sắm sau khi chuẩn hóa quyền fixture. Có bộ chọn khi công trình có hai yêu cầu mua. |
| Giữ nháp trao đổi | Nhập nội dung ở composer, mở/đóng thiết kế: nháp vẫn còn nguyên. |
| Thiết kế | Admin chọn PDF, nhập UAT-R01 và phát hành ngay trong dialog. Feed có thẻ thêm tệp ở trạng thái nháp và thẻ phát hành riêng. |
| Phản hồi theo revision | Admin và PM gửi hai phản hồi thử dưới thẻ phát hành UAT-R01. PM mở được snapshot với đúng mã revision/lý do/tên tệp. |
| Quyền PM | Không thấy nút tạo revision. Có nút bàn giao; thao tác tạo một handoff UAT-R01, hiển thị Nguyễn Minh PM và thời điểm bàn giao. |
| Checklist | Đóng khi chưa lưu mở cảnh báo; Tiếp tục nhập giữ nguyên nội dung. Chốt thành công, tạo snapshot lần 1; đầu việc chuyển `ready` → `in_progress`, không tự hoàn tất. |
| Mua sắm | Admin lưu thông tin, gửi duyệt, duyệt và ghi nhận đặt hàng MS-01. Có ba snapshot theo ba lần chuyển trạng thái. |
| Thiếu trường đặt hàng | Thiếu PO/ngày giao bị chặn, lỗi nằm tại trường và toast có nút đóng. Nhập ngày bằng CUA `setValue` thành công; Playwright fill ô date không hoạt động trong phiên công cụ này, không kết luận đó là lỗi ứng dụng. |
| Xung đột hai tab | Tab A giữ nháp; tab B lưu bản mới. Tab A báo có người khác cập nhật và khóa lưu. Xem bản mới mở cảnh báo; chọn Tiếp tục nhập vẫn giữ nguyên nháp A. Cuối phép thử, bỏ nháp thử qua dialog xác nhận. |
| Lưu thành công, gửi thất bại | Với MS-02 cố ý thiếu nguồn giá trong fixture, sửa tiêu đề rồi Gửi duyệt: header lưu thành công, submit bị backend từ chối. UI ghi rõ “Đã lưu thông tin; chưa thực hiện được bước tiếp theo” và giữ tiêu đề đã lưu. Sau phép thử đã khôi phục nguồn giá fixture. |
| Mua hàng gửi yêu cầu | Đăng nhập tài khoản Mua hàng mẫu, sửa và gửi MS-02 thành công. Tài khoản này hiện có cả `procurement.approve` trong DB, nên nút Duyệt yêu cầu xuất hiện đúng theo quyền; không suy diễn có phân tách người lập/người duyệt. |
| Snapshot mua sắm | Sau khi MS-01 đã ordered với PO và ngày giao, snapshot lúc gửi duyệt vẫn là Chờ duyệt, Chưa lập PO, Chưa chốt ngày giao, 2 m2 × 500 ₫. Mở yêu cầu hiện tại từ snapshot hiển thị PO UAT-PO-0001, ngày 15-09-2026 và Đang giao/nghiệm thu. |
| Mobile | Việc của tôi thu gọn/mở rộng được; dialog có một vùng nội dung cuộn, bảng mua sắm đổi sang thẻ và không tràn ngang. Toàn bộ dialog bị đẩy xuống 12 px; chưa đạt. |

## Phát hiện cần xử lý trước nghiệm thu

### 1. Dialog mobile bị lệch 12 px — tái hiện và nguyên nhân đã xác định

- Mục đích chính: người dùng điện thoại phải nhìn thấy và bấm được toàn bộ nút ở chân cửa sổ.
- Tái hiện: mở Trao đổi → Việc của tôi → Xử lý yêu cầu mua, đặt viewport 390 × 844.
- DOM đo được: dialog `top=12`, `height=844`, `bottom=856`; document width và scrollWidth đều 390. Parent fixed overlay có `margin-top:12px`, parent phía ngoài có class `space-y-3`.
- Nguyên nhân: `ActivityThread.tsx:366` dùng `space-y-3` và mount `DiscussionWorkDialog` trực tiếp tại dòng 649. Quy tắc khoảng cách sibling tác động lên wrapper fixed của `Dialog`. `dialog.css` đặt chiều cao bằng viewport nhưng chưa loại bỏ margin từ bên ngoài.
- Owner: `frontend/src/modules/work-items/ActivityThread.tsx`, `frontend/src/components/Dialog.tsx`, `frontend/src/components/dialog.css`. First searches: `space-y-3|DiscussionWorkDialog|dialog-mobile-fullscreen`.
- Hướng sửa đề xuất: tách overlay khỏi quy tắc khoảng cách của nội dung bằng cấu trúc/portal dùng chung phù hợp; kiểm tra các dialog lồng nhau và focus. Chưa chọn giải pháp chỉ bằng tăng/giảm chiều cao hoặc che overflow.
- Kiểm chứng cần chạy sau khi duyệt sửa: 360/390/440 px, dialog thiết kế/checklist/mua sắm, overlay lồng nhau, chân nút nằm trong viewport, Tab/Escape và build frontend.

### 2. Cập nhật trực tiếp bị từ chối khi mở nhiều tab — còn rủi ro cần chẩn đoán hẹp

- Giao diện nhiều lần hiện Đang nối lại/Mất kết nối. Log backend có HTTP 429 ở cả `/me/activity-attention/stream` và `/projects/.../activity/stream`.
- DB cấu hình hiện tại: `SSE_MAX_CONNECTIONS_PER_ACTOR=2`, per IP 20, toàn hệ thống 500. Mỗi trang Trao đổi mở một actor stream và một project stream.
- Mã `ActivityStreamCapacity.Acquire` chặn khi đạt một trong các giới hạn. Cấu hình hai luồng/tài khoản không đủ cho nhiều tab cùng tài khoản. Chưa đọc response body của từng 429 nên chưa loại trừ toàn bộ ảnh hưởng rate limit và thời điểm giải phóng kết nối cũ.
- Quan sát ban đầu: feed đã có hai thẻ thiết kế nhưng tab tổng Trao đổi vẫn 0; số tổng cập nhật sau đó. Chưa nghiệm thu khả năng đồng bộ tức thời giữa các tab.
- Owner: `backend/internal/core/activity_stream.go`, `backend/internal/shared/config/config.go`, `frontend/src/modules/notifications/ActorRealtimeProvider.tsx`, `frontend/src/modules/work-items/ActivityThread.tsx`, `frontend/src/modules/projects/ProjectDetailPage.tsx`. First searches: `Acquire|SSEMaxConnectionsPerActor|streamSSE|activity_summary|project-operations`.
- Kiểm tra tiếp cần tách: admission cap, rate limit, giải phóng stream khi chuyển tab/đăng xuất và đồng bộ số đếm khi reconnect. Không tự tăng giới hạn hoặc tắt bảo vệ trong lượt này.

### 3. Đầu việc thiết kế vẫn Sẵn sàng sau phát hành — cần chốt hành vi mong muốn

- UI và DB cùng xác nhận hồ sơ UAT-R01 đã `released`, PM đã bàn giao, nhưng Work Item liên kết vẫn `ready`.
- `CreateDesignRelease`, `AddDesignReleaseFile`, `ReleaseDesignRelease` trong `repository_design_releases.go` lưu gói/tệp/audit/snapshot; không cập nhật trạng thái Work Item. Cần phân biệt trạng thái hồ sơ với trạng thái đầu việc.
- Đề xuất để người dùng theo dõi dễ hiểu: lần cập nhật thiết kế thành công trong ngữ cảnh công việc có thể chuyển đầu việc sang Đang làm theo cơ chế chung, giữ bước chốt hoàn tất riêng. Đây là đề xuất thay đổi hành vi, chưa được thực hiện hoặc coi là yêu cầu đã nghiệm thu.
- Rủi ro: phải kiểm tra permission, node hiện hành, lịch và liên kết nguồn trong transaction; không dùng client gọi start riêng hoặc tự hoàn tất bước.

## Đối chiếu dữ liệu cuối phép thử

- MS-01: `ordered`, edit_version 6, PO `UAT-PO-0001`, ngày giao `2026-09-15`, tổng 1.000 ₫.
- MS-02: `waiting_approval`, edit_version 4, tổng 1.000 ₫; được giữ lại làm hồ sơ thử chờ duyệt.
- UAT-R01: `released`, một handoff do PM thực hiện. Checklist có snapshot lần chốt 1.
- Công trình UAT: 8 `work_update`, 2 bình luận thử, 2 `started`, 1 `system_event`; không có stock_movements, payables hoặc receivables.
- Công trình người dùng `74c409ae-eb91-434a-baa8-7b3fa4422b9a`: vẫn 20 Work Item pending, 1 ready, 0 work_update. Không gửi bình luận hoặc thay đổi nghiệp vụ tại công trình này.
- `git diff --check` không báo lỗi. Không commit/push/merge, không đổi mã runtime hoặc migration.

## Theo dõi tiếp

Chuẩn bị review sửa các phát hiện đã xác định trước khi mở rộng BOQ/hợp đồng. Các lỗi/hành vi mới cần review package và Mockup as Text theo AGENTS.md trước khi sửa. Chưa có mapping update vì lượt này chỉ kiểm thử, tạo dữ liệu giả lập và ghi báo cáo; mapping liên quan phải được cập nhật cùng bản sửa đã duyệt.
