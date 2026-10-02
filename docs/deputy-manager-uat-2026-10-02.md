# UAT local Phó Quản Lý — 2026-10-02

**Kết quả: các kịch bản nghiệp vụ được chạy đạt; còn lỗi hiệu năng bộ lọc. NOT UX ACCEPTED.** Đây là kiểm tra của agent trên local, chưa phải nghiệm thu của client.

**Cập nhật sau UAT, 23:15 ICT:** User duyệt “fix hiệu năng”; UAT-DM-01 đã sửa và kiểm chứng kỹ thuật local: API từ 14,17 giây xuống trung vị 15,72 ms, data giữ nguyên. Xem `deputy-manager-performance.md`. Chưa chạy lại browser sau sửa; giới hạn nghiệm thu vẫn áp dụng. Kết quả UAT ban đầu được giữ để truy vết.

## Vấn đề cần xử lý trước nghiệm thu

**UAT-DM-01 — tải bộ lọc chậm.** `GET /api/me/supervision-options` mất 14,496 giây, trả 839 công trình và 5 Giám sát trên DB local hiện có. Trong trình duyệt, màn hình hiển thị “Đang tải bộ lọc...” kéo dài. API danh sách công việc với bộ lọc mất 0,129 giây, trả 25 dòng và `has_more=true`; hai chi phí này được đo riêng.

Đã xác nhận nguyên nhân bằng SQL hiện tại tại `backend/internal/core/supervision.go::SupervisionOptions`: truy vấn Giám sát dùng `EXISTS` theo từng công trình, lồng điều kiện vai trò/phân công. `EXPLAIN (ANALYZE, BUFFERS, TIMING OFF)` trên truy vấn này mất 14.346,551 ms; nested semi join xét 2.138 user với 839 công trình, subplan chạy 1.789.592 lần, 37.090.322 shared buffer hits. Đây là chi phí truy vấn đã đo, chưa sửa trong lượt UAT.

### Review package cho bước sửa tiếp theo — PROPOSED, chưa duyệt triển khai

- Mục tiêu: Phó Quản Lý mở bộ lọc Giám sát nhanh, vẫn giữ nguyên phạm vi công trình thật và cách nhận diện Giám sát chuẩn.
- Domain đã đối chiếu: Administration/Governance, Construction Operations và Reporting; maps central/backend/frontend/triage đã được nạp. Điểm tìm đầu tiên: `SupervisionOptions`, `supervisorUserSQL`, `deputyManagerUserSQL`, `/me/supervision-options`.
- Owner dự kiến: `backend/internal/core/supervision.go`, `supervision_test.go`, các mapping liên quan và ghi chú plan. Consumer hiện tại là `frontend/src/modules/my-work/SupervisionWorkspace.tsx`; đề xuất này chưa thay đổi UI.
- Giải pháp: tạo tập user Giám sát từ role assignment/membership hợp lệ, dùng phép hợp tập và join công trình theo phạm vi ở nguồn dữ liệu; tránh nhân toàn bộ user với toàn bộ công trình. Giữ kiểm tra actor, active/deleted/sample, vai trò chuẩn, role theo công trình và membership alias hiện có. Không dùng cache để che chi phí truy vấn.
- Validation: so sánh tập kết quả trước/sau; test Giám sát global/scoped/member, inactive/deleted/sample, actor không được phép và thu hồi quyền; đo lại API cùng query plan trên DB local đại diện. Mục tiêu đề xuất là dưới 1 giây cho endpoint trên tập dữ liệu này, cần đo mới được khẳng định đạt.
- Rủi ro: bỏ sót Giám sát có vai trò theo công trình hoặc mở rộng dữ liệu mẫu nếu viết lại predicate sai. Không thay danh mục quyền, migration hay snapshot báo cáo để tối ưu.

Quan sát UX phụ: công trình chưa có pipeline vẫn hiện CTA “Hoàn tất thiết lập” khi Phó Quản Lý mở trao đổi. Với fixture đầy đủ `created_by`/mẫu quy trình, wizard mở chỉ xem; không có nút sửa hay chạy pipeline. Chưa phát hiện vượt quyền. Lỗi wizard được thấy trước đó do fixture thiếu `created_by`, không phải lỗi phân quyền; không tính nó là bug sản phẩm. Đổi nhãn/ẩn CTA chỉ là đề xuất UX, chưa được duyệt.

## Môi trường và nguồn kiểm tra

- Thời gian local: khoảng 22:35–23:03 ICT ngày 2026-10-02. Source: working tree chưa commit trên `main`, HEAD `465eb76`, feature Phó Quản Lý đã triển khai trước lượt UAT. Không sửa mã ứng dụng trong lượt kiểm tra này.
- Stack Docker local do người dùng build: frontend Vite dev tại `127.0.0.1:5173`, bind `frontend` hiện tại; backend binary tại `127.0.0.1:8080`; PostgreSQL 16 và Redis 7. Container khởi tạo 22:35:40 ICT. Đây không phải static frontend dist hay production.
- Backend image: `sha256:4bef5cc08b55c9e31e676a4b3dfa9a0c64e18152e84c5fd1a91ede905c7ebe1e`. Admin image: `sha256:a443bbeab2996a44131e06f248f1dfb2c60fbade5783c5ff843db5b72ca82833`.
- `docker compose ps`: toàn bộ service chạy; PostgreSQL/Redis healthy. `/api/health`: HTTP 200. DB main local có V146–V148 áp dụng lúc 22:35:47–48 ICT. Không reset DB và không chạy full seed.
- Trước fixture: 2.147 users, 884 projects (42 sample). Dữ liệu UAT mới có tag `uat-dm-1002-6ff17d`: ba tài khoản Phó Quản Lý/PM/Giám sát; ba công trình thật A/B/C và một sample; 29 công việc, hai báo cáo. Phó Quản Lý có **0 project membership**.

## Kịch bản đã chạy và kết quả

| Kịch bản | Bằng chứng/kết quả local |
| --- | --- |
| Đăng nhập Phó Quản Lý | UI hiển thị đúng role và hai tab Công việc/Báo cáo Giám sát. |
| Theo dõi nhiều công trình | A/B xuất hiện không cần membership; phân trang từ 25 lên 28 dòng; lọc công trình + Giám sát còn đúng một dòng. |
| Công trình mới | C tạo sau khi phiên đăng nhập đã tồn tại; công việc C xuất hiện qua API và UI tìm kiếm, không thêm membership. |
| Quan sát công việc | Shared WorkItem detail có thông báo chỉ xem; không có hành động bắt đầu/hoàn tất/duyệt. API bắt đầu công việc `ready` trả 403. |
| Trao đổi | Gửi ghi chú trên công trình B thành công qua UI; composer chỉ có Ghi chú, không có attachment/đổi trạng thái/phân công. API quyết định activity bị chặn 403. |
| Đồng gửi báo cáo | Supervisor gửi A/B bằng API thật; DB có notification gửi tới PM và Phó Quản Lý; UI notification mở đúng báo cáo B. |
| PM xem trước | B đã có receipt PM nhưng Phó Quản Lý vẫn thấy “Tôi chưa xem” và CTA xác nhận. Sau xác nhận, có hai receipt riêng. |
| Phó Quản Lý xem trước | A lần 2 được Phó Quản Lý xác nhận trên mobile; PM vẫn thấy mục Chờ tôi duyệt và xác nhận riêng thành công qua UI. |
| Yêu cầu bổ sung | Nội dung trống bị inline/toast validation; gửi lý do thật chuyển A sang `needs_changes`; không có CTA xác nhận, API xác nhận trả 400. |
| Gửi lại/lịch sử | Supervisor submit lại qua API tạo lần 2. Dialog lần 1 đang mở nhận cập nhật nhưng giữ snapshot cũ, cảnh báo lịch sử và không cho xác nhận; trở về bản hiện tại thấy nội dung mới. |
| Snapshot minh chứng | Lần 1 giữ 40%, nội dung cũ, 0 ảnh; lần 2 có 55%, nội dung bổ sung và 1 ảnh. Shared protected image viewer mở đúng ảnh test. Lấy ảnh lần 2 bằng revision 1 trả 403. |
| Thao tác lặp/stale | Duplicate acknowledge B trả 200, không tạo receipt trùng. Acknowledge revision 1 sau khi có revision 2 trả 400 `stale_revision`. DB cuối cùng có 4 receipt, mỗi actor/report/revision đúng 1 dòng. |
| Bộ lọc đã xem | Đã xem hiển thị hai báo cáo; Chưa xem hiển thị empty state đúng, không lỗi. |
| Giới hạn quyền | PM/Giám sát gọi company queue: 403; Phó Quản Lý đọc sample/finance và review/export báo cáo không thuộc phạm vi: 403. |
| Thu hồi role | Cùng token đang dùng bị 403 queue/detail sau thu hồi; reload UI mất tab company. Role chỉ được phục hồi trên tài khoản fixture để chạy phần còn lại. |
| Responsive | Dialog báo cáo tại 360/390/440px không làm page/dialog tràn ngang; bảng danh sách mobile dùng cuộn ngang bên trong wrapper. Desktop 1440px đã kiểm tra. |
| Thiết lập công trình | Fixture B hợp lệ trả capabilities sửa/chạy pipeline=false; wizard UI chỉ có Đóng, không có nút mutation. |

Shared components kiểm tra qua hành động thực tế: WorkItemDetailDialog, WorkReportReviewDialog, ActivityComposer, EvidenceRecordViewerDialog và viewer ảnh được tái sử dụng; không thêm trải nghiệm preview riêng. Console warning/error thu thập cuối lượt không có bản ghi.

## Giới hạn kiểm chứng

- Source báo cáo và project được tạo bằng SQL fixture. Nội dung source A lần 2 được sửa bằng SQL fixture; upload minh chứng và submit/resubmit dùng API thật. Chưa kiểm tra toàn bộ thao tác tạo/sửa báo cáo qua UI Giám sát.
- PDF API revision 1/2 trả 200 và bytes có `%PDF`, kích thước 30.240/30.859 bytes. Chưa kiểm tra layout PDF bằng mắt; chưa xác nhận file tải thành công qua trình duyệt do sự kiện download hết thời gian chờ. Không kết luận đây là bug tải file.
- Responsive dùng viewport trình duyệt, chưa kiểm tra điện thoại thật, camera, bàn phím mềm hoặc Safari/iOS.
- Chưa chạy UAT bằng tài khoản Cường/Nguyệt, chưa chuyển role hoặc phân công thật, chưa có xác nhận nghiệm thu của client.
- Bộ kiểm thử/build trước UAT đã có checkpoint, hiện được lưu trong [plan đã đóng](plan-closed/2026-10-02__closed-deputy-manager-supervision.md); không chạy lại trong lượt UAT vì mã ứng dụng không thay đổi. Docker runtime local và các tình huống trên được kiểm tra riêng; không đại diện cho deployment production.

## Kết thúc và artifacts

- Đã logout, reset viewport, vô hiệu hóa đúng ba user UAT và revoke toàn bộ sáu session của họ. DB xác nhận ba user inactive, zero unrevoked session; token cũ của cả ba role trả 401. Lịch sử, receipt, project và báo cáo có tag UAT được giữ để đối chiếu, không xóa audit.
- Temporary token/password files được gỡ; giữ manifest không có password. Không sửa tài khoản Cường/Nguyệt, không commit/push/deploy.
- Ảnh và query plan: `/Users/khiemnguyen/.codex/visualizations/2026/10/02/01a0fc1e-bd41-7350-97cc-6a24777bf4bc/consflow-deputy-uat/`. Các ảnh tiêu biểu: `pm-independent-receipts.jpg`, `work-readonly-desktop.jpg`, `new-project-auto-scope.jpg`, `report-protected-image.jpg`, `report-mobile-390.jpg`.
- Chỉ cập nhật tài liệu kiểm chứng/plan trong lượt UAT; không có thay đổi behavior/API/permission nên không cần thay mapping.
