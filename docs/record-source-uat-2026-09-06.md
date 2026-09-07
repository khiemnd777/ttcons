# Kiểm tra UI và liên kết hồ sơ — 06-09-2026

**Mục đích chính:** từ một bản xem chi tiết, mở đúng hồ sơ gốc hoặc hồ sơ liên quan để đối chiếu, đồng thời giữ ngữ cảnh đang làm việc.

Hai plan liên quan đã được đóng theo yêu cầu người dùng ngày 06-09-2026; xem [hồ sơ đóng plan](plan-closed/2026-09-06__closed-detail-redesign-and-source-links.md). Báo cáo này lưu kết quả tại thời điểm UAT; trạng thái Git và các kiểm tra còn thiếu bên dưới là ảnh chụp lịch sử, không phải plan tiếp tục triển khai.

## Trạng thái và giới hạn

**IMPLEMENTED; TECHNICALLY VERIFIED trong phạm vi dưới đây; toàn bộ package NOT UX ACCEPTED.** Đây là kiểm tra trực tiếp bởi agent trên bản local, chưa phải nghiệm thu của người dùng hoặc production.

- Đã mở nội dung hợp lệ của 18/19 loại. Phiếu kho chỉ kiểm tra được trạng thái không khả dụng: cả 40 phiếu hiện có thuộc công trình đã hủy. Chưa tạo hoặc khôi phục dữ liệu để ép kịch bản thành công.
- In-app browser không hiển thị tab con khi công cụ click liên kết `target="_blank"`. Đã kiểm tra `href`, `target`, `rel` thực tế và mở chính URL đó trong tab in-app khác. Kết quả này chứng minh đích đến, chưa chứng minh toàn bộ thao tác click mở tab tự nhiên.
- Đã thấy tab và dialog nguồn giữ trạng thái khi mở URL đích riêng. Chưa nhập và kiểm chứng một bản nháp trao đổi chưa gửi.
- Mobile thực tế 390×844: Yêu cầu mua và Biên bản họp. Các loại còn lại mới có kiểm tra desktop; không suy rộng kết quả responsive cho cả 19 loại. Chưa kiểm tra browser với modal lồng nhau sau thay đổi focus.
- Khi mở nhiều tab UAT rồi tải lại, `/auth/me` trả HTTP 429 và ứng dụng chuyển về đăng nhập. Code `frontend/src/lib/auth/AuthContext.tsx` xác nhận bootstrap catch xóa token với mọi lỗi. Đây là lỗi auth còn mở, chưa sửa trong nhóm detail/source này. Đã đóng các tab kiểm tra thừa và tiếp tục tuần tự trên một tab; không thay đổi rate limiter hoặc quyền.

## Môi trường và cách kiểm tra

- Chỉ dùng **Codex in-app browser**. Không điều khiển Edge của người dùng.
- Runtime Docker local: frontend `localhost:5173`, API `localhost:8080/api`; dùng dữ liệu đã lưu và tài khoản local có quyền phù hợp. Không gửi trao đổi, duyệt hồ sơ, ghi sổ, nhập/xuất kho hoặc cập nhật dữ liệu nghiệp vụ. Đăng nhập/đăng xuất có tác động thông thường lên phiên xác thực.
- Kiểm tra bằng accessibility tree, DOM hiển thị, link thật, ảnh chụp tại các điểm kiểm tra, vị trí/focus và thao tác bàn phím. Không coi việc chỉ mở trang là đã kiểm tra toàn bộ chức năng của module.
- Domain khớp: Commercial/Procurement, Quality/Handover, Materials/Warehouse, Project Planning, Admin Governance và Accounting. Bootstrap đã đọc central/frontend/backend/domain maps. Tìm kiếm đầu tiên: `recordPath|relatedRecordTargets|useRecordDeepLink`, `ActivityTopicDetails`, `GetSupplierQuotationDetail|safeTable`, `workspaceRef|openHandoverDetail`, `Dialog|manageFocus`.

## Ma trận 19 loại

“Mở được” dưới đây chỉ có nghĩa là bản local hiển thị đúng hồ sơ được yêu cầu và nội dung đã được kiểm tra; không phải nghiệm thu toàn bộ module.

| Loại | Hồ sơ hoặc tình huống đã mở | Kết quả quan sát trực tiếp |
|---|---|---|
| BOQ | Sơn nội thất [MẪU], `78964174-3b07-4c2a-83e8-7e39959f417c` | Preview và hồ sơ gốc đúng; 16 thùng, 12.800.000 ₫; tab Mua sắm liên kết MAU-KT-MS-03. Đóng nguồn xóa tham số mở, không tự mở lại. |
| Yêu cầu mua | MAU-KT-MS-03, `bba3f2da-7582-4406-9340-c2bbd8f298ab` | Đúng 16 thùng; tách yêu cầu/nghiệm thu/nhập/chờ nhập; có liên kết BOQ và đối tác. Desktop và mobile 390px đọc được. |
| Công việc | Biên bản khảo sát, `b5992616-dddb-4735-8d31-d2c70ea00df0` | Đúng Work Item tại công trình Golden; hồ sơ hoàn tất chỉ đọc. |
| Nghiệm thu | Nghiệm thu trần khu A, `71000000-0000-0000-0000-000000000001` | Workspace chọn đúng hồ sơ OA-6A4E7D13 và hai tiêu chí. |
| Bàn giao | HO-GOLD-001, `73000000-0000-0000-0000-000000000201` | Chọn đúng hồ sơ thay vì dòng đầu danh sách; có nguồn nghiệm thu cuối. Sau sửa, vùng chi tiết nhận focus ở y=80, dưới header cố định. |
| Biến động kho | SM-OUT-001, `8ae92001-093c-424e-a6a5-f9d7fe136903` | Bản xem chỉ đọc đúng Dây điện Cadivi; sau sửa hiển thị **800 m**, không còn chuỗi `800.000 m` gây nhầm. |
| Khắc phục | Sửa màu sơn pantry, `7997f0a3-872f-41f1-8934-7cfa7a8503db` | Đúng hồ sơ trên `/rework`, có liên kết nghiệm thu và vấn đề nguồn. |
| Báo giá NCC | QT-GOLD-DULUX, `64000000-0000-0000-0000-000000000202` | Sau sửa mở được 6 dòng; tổng **496.916.189 ₫**, liên kết từng BOQ và Dulux; close button nhận focus. |
| Phiếu kho | `46f84357-8b86-4aa8-a63d-3b8996b38ef3` | Hiện không tìm thấy/không có quyền; không mở hồ sơ thay thế. **BLOCKED đối với kịch bản nội dung hợp lệ** vì công trình đã hủy. |
| Đối tác | Vật tư An Phát [MẪU], `96126f15-346b-448e-b25b-6628a2e0afae` | Canonical dialog mở đúng đối tác kể cả không nằm ở trang đầu danh sách hiện tại. |
| Điều chỉnh kế hoạch | `91211a52-eff5-45b4-acfc-aab94265028a` | Đúng bản nháp, 0 ngân sách/0 ngày; không tạo liên kết khi bản ghi không có ID nguồn. |
| Nhật ký kế hoạch | Daily report planning structured v2 | Đúng 18-06-2026, 35%, một việc đã làm/một việc tiếp theo; footer mở Work Item `43000000-0000-0000-0000-000000000005`. |
| Biên bản họp | Họp chốt thiết kế demo | Đúng 19-06-2026, hai người dự/một quyết định/một hành động; nguồn Work Item `43000000-0000-0000-0000-000000000007`. Desktop và mobile; Tab/Shift+Tab/Escape/khôi phục focus đã kiểm tra. |
| Checklist kế hoạch | Checklist planning structured v2 | Đúng 16-06-2026, hai mục hoàn tất/một minh chứng; nguồn Work Item `43000000-0000-0000-0000-000000000003`. |
| Vai trò | PM, `20000000-0000-0000-0000-000000000003` | Admin mở đúng vai trò với 77 quyền; đóng xóa tham số nguồn và không tự mở lại. Không sửa phân quyền. |
| Nhật ký hệ thống | `d7c9e241-1f89-4280-9644-7fdd4ac72761` | Mở đúng sự kiện phát hành mẫu; dữ liệu kỹ thuật thu gọn. Một sự kiện tạo yêu cầu mua khác có link đúng `60000000-0000-0000-0000-000000000001`. |
| Phải trả | PAY-Q1-001, `ef3cd5dd-1212-4524-ab02-2d2fc140b6c0` | Admin mở đúng 120 triệu, đã trả 60 triệu, còn 60 triệu; liên kết đối tác. PM/Accounting local thiếu quyền legacy bị từ chối. |
| Phải thu | REC-Q1-002, `e10a6f67-277c-4fd7-b026-21f942331fc1` | Admin mở đúng 300 triệu, đã thu 120 triệu, còn 180 triệu; liên kết đối tác. |
| Thanh toán legacy | RCV-REC-GOLD-RET-02, `0cbc597d-ab1e-463e-b36d-4b66cabff682` | Bản xem chỉ đọc đúng 58,5 triệu, chuyển khoản, đã duyệt và nguồn phải thu. |

Kiểm tra âm bổ sung: dùng ID BOQ của công trình mẫu trong URL công trình Q1 và dùng `record=invalid-record-id` trên Roles đều hiện thông báo không khả dụng, không mở nhầm bản ghi. Dialog lỗi nhận focus; Escape đóng và giữ các query params không thuộc nguồn.

## Năm lỗi đã sửa từ UAT

| Quan sát trước sửa | Nguyên nhân có bằng chứng | Sửa và kiểm tra lại |
|---|---|---|
| Tab đi vào navigation nền khi mở nguồn mua sắm | Canonical detail không bật quản lý focus của Dialog | Bật `manageFocus` tại các detail liên quan; chỉ modal trên cùng xử lý bàn phím/focus ban đầu. Biên bản họp mobile: Shift+Tab từ Đóng về link cuối, Tab quay lại Đóng; Escape trả focus về Xem chi tiết. Báo giá và dialog lỗi cũng nhận focus đúng. |
| Báo giá list mở được nhưng detail HTTP 500 | `safeTable` thiếu `supplier_quotations` trong đường kiểm tra project permission | Thêm đúng một bảng vào allowlist. Test đã thất bại trước sửa và qua sau sửa; browser mở được QT-GOLD-DULUX. Không bỏ kiểm tra quyền. |
| Báo giá detail hiển thị tổng 0 khi các dòng có tiền | `GetSupplierQuotationDetail` không trả aggregate như list | Bổ sung item count, tổng trước thuế và tổng tiền từ chính các dòng báo giá. Integration test 10 × 102 = 1.020; browser xác nhận tổng Golden là 496.916.189 ₫. |
| Khối lượng 800 hiển thị `800.000 m` | PostgreSQL trả numeric string, summary nội suy nguyên chuỗi | Chuẩn hóa số bằng định dạng vi-VN, tối đa ba số lẻ. SSR kiểm tra 800.000 → 800 và 1234.500 → 1.234,5; browser xác nhận 800 m. |
| Chọn đúng bàn giao nhưng người dùng vẫn nhìn danh sách | Cuộn khi chọn ID xảy ra trước lúc detail tải xong | Cuộn/focus một lần sau đúng detail đã tải; chừa header 80px. Browser đo y=80 và activeElement đúng vùng chi tiết. |

Nhận xét thị giác của agent: tên hồ sơ, trạng thái và giá trị chính có thứ bậc rõ hơn bản trước; liên kết đặt cạnh đối tượng cần đối chiếu giúp giảm tìm kiếm thủ công. Các dialog bảng nhiều dòng cần cuộn nội dung; mobile Procurement chuyển thành bố cục đọc theo nhóm, Meeting Minutes giữ CTA nguồn trong luồng đọc. Nhận xét này dựa trên các màn đã xem, không phải điểm UX đã được người dùng nghiệm thu.

## Kiểm chứng kỹ thuật sau sửa

- Branch `codex/ui-detail-redesign`, HEAD an toàn `f5e2ffb42ec6a873e6c6c92441e2a581baa22c4c`; thay đổi source links/UAT đang **uncommitted**, chưa push.
- SHA-256 (đường dẫn repository-relative đã sắp xếp + NUL + nội dung + NUL): `frontend/src` 305 files `2fca3d23dbd306bcc6916e88a78fcbf40b1d15a91deff7f576bafa4c32490ef1`; `frontend/tests` 93 files `d8d9c956e4473d05d5f4a097e1d6710f3291056f637f38bff027e3eb18366f2f`; `backend/internal/core` 247 Go files `f8e54af44b4d8286c112f101aaf09e9f9b609a756ebc7cb9d8ec1a1e76e05f9c`.
- macOS, Node 23.11.0/npm 11.4.2: `cd frontend && npm test` — **393 pass, 0 fail, 0 skip**, 2.266s; kết thúc 13:39:02 ICT.
- `cd frontend && npm run build:budget` — TypeScript, Vite và mọi chunk budget đạt; Vite 6.68s; entry 385.25 kB raw/102.7 KiB gzip. Kết thúc 13:39:25 ICT. Không sửa source sau build cuối.
- Backend chạy trong DB **PostgreSQL 16 tạm, tách biệt main local**, tên `consflow_source_uat`; custom runner áp dụng 136 migrations đến V136. `CONSFLOW_TEST_DATABASE_URL=… GOCACHE="$PWD/.gocache" go test ./internal/core -run 'TestSafeTable|TestSupplierQuotationSelectionLocksBOQAndSnapshotsProcurement|TestNormalizeAuditLogFilter|TestAuditLogPageFiltersAndPagination' -count=1 -v` — **6 pass**, 2.817s; kết thúc 13:38:07 ICT. Không coi DB test này là browser/UAT.
- `make restart` down → build → run kết thúc 13:39:19 ICT; browser retest dùng stack này. Health API `status: ok`; năm service chạy, PostgreSQL và hai Redis healthy tại 13:45 ICT. Main database vẫn 136 migrations, phiên bản số lớn nhất V136. Không thêm/sửa migration hoặc seed.
- `git diff --check` sạch; `gofmt -l` cả tám Go files thay đổi không trả file chưa format. Container PostgreSQL tạm `consflow-source-uat-test` đã dừng và tự xóa sau kiểm tra. Logs: `/tmp/consflow-record-links-uat-tests-final.log`, `/tmp/consflow-record-links-uat-build-final.log`, `/tmp/consflow-record-links-uat-runtime-final.log`, `/tmp/consflow-source-uat-backend.log`.
- Chưa chạy full backend regression suite. Các giới hạn browser/UAT đầu báo cáo giữ trạng thái toàn package ở **NOT UX ACCEPTED**.

## Phần còn lại

1. Kiểm tra phiếu kho trên một hồ sơ hợp lệ thuộc công trình còn truy cập được.
2. Xác nhận click mở tab tự nhiên cùng bản nháp trao đổi chưa gửi; kiểm tra modal lồng nhau và mở rộng mobile cho các loại còn lại.
3. Sửa riêng auth bootstrap để phân biệt lỗi tạm thời 429/network/5xx với phiên không hợp lệ, giữ fail-closed và rate limiter; sau đó kiểm tra luồng nhiều tab.
4. Người dùng xem và nghiệm thu UX trên bản local đã sửa.
