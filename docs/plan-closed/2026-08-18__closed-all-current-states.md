# [Closed] All Current States

Closed: 2026-08-18
Reason: User requested closing all current state entries.

## [Closed] Hồ sơ cá nhân và bảo mật tài khoản

### Will be done

- Thêm `/profile` cho mọi user đã xác thực, không tạo permission mới; hiển thị thông tin tài khoản, metadata nhân sự chỉ đọc và hai dialog chỉnh sửa thông tin/đổi mật khẩu.
- Mở rộng `/auth/me` với số điện thoại, phòng ban, chức vụ; thêm `PUT /auth/profile` và `PUT /auth/password` chỉ thao tác actor hiện tại.
- Đồng bộ họ tên/email/điện thoại từ self-service profile sang hồ sơ staff đang liên kết trong cùng transaction; không thay đổi hành vi quản trị tại `/staff` và không tự tạo staff khi thiếu liên kết.
- Yêu cầu mật khẩu hiện tại khi đổi email hoặc mật khẩu; lưu mật khẩu mới bằng bcrypt, thu hồi toàn bộ session sau thay đổi nhạy cảm và không ghi secret vào audit.
- Bổ sung backend/frontend regression coverage, cập nhật central/backend/frontend/admin-governance maps và chạy format/test/build; browser/UAT không chạy nếu người dùng không yêu cầu.

### Done

- Người dùng đã duyệt review package, impact matrix và Mockup as Text bằng câu `OK triển khai`.
- Đã xác nhận `/auth/me` hiện chỉ đọc `users`; phone/department/position thuộc `staff`; chưa có API hoặc UI Profile/đổi mật khẩu.
- Đã xác nhận email JWT hiện được dùng làm actor name nên đổi email với token cũ sẽ làm lệch audit; scope đã khóa thành thu hồi session và đăng nhập lại.
- Đã tải central/backend/frontend mapping, admin-governance, backend/frontend workflow và impact-analysis guardrails; owner và contract không cần migration/seed đã được xác định.
- Đã thêm `GET /auth/me` mở rộng cùng `PUT /auth/profile` và `PUT /auth/password`; self-service mutation dùng actor từ JWT, đồng bộ linked staff, ghi audit trong transaction và thu hồi mọi session khi email/mật khẩu đổi.
- Đã tái sử dụng kiểm tra mật khẩu demo/bcrypt, lưu mật khẩu mới bằng bcrypt, chuẩn hóa email/điện thoại và fail-closed khi phone chưa có linked staff hoặc email đã được dùng.
- Đã thêm `/profile`, entry header/menu mobile, trang tóm tắt responsive, shared dialog chỉnh sửa thông tin/đổi mật khẩu, inline validation, warning/error/success toast và AuthContext refresh sau thay đổi không nhạy cảm.
- Targeted backend Profile chạy với PostgreSQL local qua `CONSFLOW_TEST_DATABASE_URL`: 3/3 passed, gồm chuẩn hóa input, cập nhật profile/đổi mật khẩu/thu hồi session/audit và fail-closed khi chưa liên kết staff.
- Frontend full test: 154/154 passed; `npm run build` passed; bundle budget passed với entry gzip 122.1 KiB, Profile chunk gzip 4.5 KiB và 117 chunks; `git diff --check` passed.
- Backend full `go test ./...` chưa xanh do 7 lỗi ngoài phạm vi Profile trên working tree dùng chung: 4 lỗi unique code ở accounting, 2 lỗi workflow Rev8/migration shape và 1 lỗi pending trigger ở migration runner. Browser/UAT không chạy theo repo gate nên UI vẫn `NOT UX ACCEPTED`.

## [Closed] Tái cấu trúc Thống kê và Báo cáo nghiệp vụ

### Will be done

- Đổi workspace tổng hợp hiện tại từ `Báo cáo` sang `Thống kê`, thêm route `/statistics`, giữ redirect tương thích cho các URL thống kê cũ và không thay đổi Dashboard/Tiến trình/Tài chính công trình.
- Tách quyền `statistics.view`/`statistics.export` khỏi quyền Báo cáo nghiệp vụ; giữ compatibility có kiểm soát cho `report.view` hiện tại để không mở rộng quyền ngoài ý muốn.
- Bổ sung xuất dữ liệu PDF, Excel `.xlsx` thật và CSV từ cùng actor/project/filter/query scope; ghi audit cho mỗi lần xuất.
- Tạo module `Báo cáo` tại `/reports` trên nguồn `operation_daily_reports` hiện hữu, thêm delivery/revision/review event để hỗ trợ lập, gửi, yêu cầu bổ sung, gửi lại và xác nhận đã xem mà không tạo ledger thi công song song.
- Giữ Rev8 `construction.item_reporting`, evidence, checks/actions, stock, quality và workflow historical làm nguồn chuẩn; dữ liệu lịch sử không được backfill người nhận/trạng thái giả.
- Bổ sung backend/frontend/migration regression coverage, cập nhật central/backend/frontend/reporting/construction/workflow maps và chạy migration check, backend test, frontend test/build; browser/UAT chỉ được công nhận sau khi chạy trên build hiện tại.

### Done

- Người dùng đã duyệt review package, permission/export matrix, Mockup as Text và scope Rev8 bằng câu `OK triển khai theo review package này`.
- Đã tải central/backend/frontend mapping cùng reporting, construction, workflow, planning, database, backend/frontend workflow và impact-analysis guardrails.
- Đã xác nhận current `/reports` là aggregate workspace gồm project/procurement/warehouse/staff và CSV; repo có nguồn báo cáo thi công/kế hoạch rời rạc nhưng chưa có unified authored-report module.
- Đã xác nhận Rev8 `construction.item_reporting` thuộc Operations ledger và `operation_daily_reports` chưa có delivery/reviewer lifecycle; workflow PDF infrastructure có revision/hash còn repo chưa có Excel `.xlsx` implementation.
- Đã xác nhận working tree đang chứa các thay đổi Rev8/Dashboard/action-column có sẵn, bao gồm reporting owner files và `AppShell.tsx`; lượt triển khai phải bảo toàn toàn bộ thay đổi đó.
- Đã đổi sản phẩm aggregate sang `Thống kê` tại `/statistics`, tách menu/quyền `statistics.view|export`, giữ redirect URL aggregate cũ và không đưa kế toán chuyên nghiệp vào route/catalog Thống kê.
- Đã đồng bộ search/sort/project scope giữa màn hình và xuất dữ liệu; PDF dùng font Unicode nhúng, Excel là gói Open XML `.xlsx` thật và CSV UTF-8. Quyền chỉ xem không hiển thị action xuất; quyền project-scoped không nhìn thấy tồn kho trung tâm hoặc material-only ngoài phạm vi.
- Đã tạo module `Báo cáo` tại `/reports` theo đúng luồng người lập → người nhận: danh sách cá nhân/chờ xem/toàn phạm vi, gửi, yêu cầu bổ sung bắt buộc lý do, gửi lại, xác nhận đã xem, lịch sử event và revision snapshot SHA-256 bất biến. Sổ đăng ký hỗ trợ PDF/Excel/CSV; từng revision hỗ trợ PDF.
- Đã giữ Operations/Workflow Rev8 làm nguồn nội dung duy nhất: CTA lập báo cáo chỉ xuất hiện khi đồng thời có `work_report.create`, `operation.report`, `operation.view`; nguồn bị khóa khi đã gửi/đã xác nhận và chỉ mở sửa khi chưa gửi hoặc đang `needs_changes`. Nhật ký trước V106 được giữ ở trạng thái read-only `recorded`, không backfill người nhận/trạng thái gửi giả.
- Đã thêm V106, cập nhật seed parity và áp dụng trên PostgreSQL local: `migrations applied=1 skipped=105`. V106 thêm quyền tách biệt, `report_delivery_eligible`, `work_reports`, revision/event bất biến và index/constraint liên quan.
- PostgreSQL integration test đã chạy thật và passed cho toàn bộ vòng đời submit → khóa nguồn → needs changes → resubmit revision 2 → acknowledge → PDF/hash/immutability, cùng regression project-scoped Statistics không rò tồn kho trung tâm.
- `make migrate-check`: 106/106 migration hợp lệ. Full backend `GOCACHE=/private/tmp/consflow-reporting-gocache go test ./...`: passed. Frontend `npm test -- --run`: 150/150 passed; `npm run build`: passed trên source UI hiện tại. `git diff --check`: passed.
- Đã rebuild/recreate backend container local từ source hiện tại; startup ghi nhận `applied=0 skipped=106`, `/api/health` và `/api/ready` trả 200, còn hai route mới `/api/work-reports` và `/api/statistics/projects` trả 401 khi không có token (route đã đăng ký và được bảo vệ).
- Đã cập nhật central/backend/frontend maps cùng reporting, construction, workflow và admin-governance domain maps cho route, owner, RBAC, lifecycle, source lock và export semantics mới.
- Browser/UAT chưa chạy vì lượt xác nhận này không yêu cầu browser; trạng thái UI/UX là `NOT UX ACCEPTED`. Trạng thái code là `IMPLEMENTED` và `TECHNICALLY VERIFIED` trên PostgreSQL local/test/build nêu trên.

## [Closed] Rev8 - tinh gọn Step theo gói công việc

### Will be done

- Xuất bản revision workflow chính thức mới, bất biến, giữ 15 node nhưng giảm từ 50 xuống 21 Step; không sửa Rev7 hoặc runtime lịch sử.
- Gộp các Step cùng một đầu ra nghiệp vụ thành work package, giữ một vai trò chịu trách nhiệm chính và đọc trạng thái con từ module nguồn.
- Chuyển hồ sơ chi/thu, hóa đơn và thu tiền khỏi critical path kỹ thuật; giữ readiness tài chính fail-closed tại giai đoạn đóng dự án.
- Giảm node-level approval trùng với BOQ, hợp đồng, procurement, acceptance và handover source lifecycle.
- Hiển thị composite source readiness dưới dạng checklist trong Work Item mà không tạo nguồn dữ liệu hoặc quyền mới.
- Thêm Golden Rev8, migration/DB/backend/frontend regression coverage và cập nhật đầy đủ central/detail/domain maps.

### Done

- Người dùng đã duyệt review package, ma trận tác động và Mockup as Text bằng câu `OK triển khai theo review package Rev8 này`.
- Đã xác nhận baseline Rev7 trên PostgreSQL local: 15 node, 50 Step, 27 form-backed, 22 source/action-only, 1 photo-only và 13 node có approval policy.
- Đã xác nhận target Rev8: 21 Step toàn template; nhánh Có thiết kế/Không thiết kế/Chủ đầu tư có BOQ lần lượt đúng 20/18/15 work item.
- Đã tải central/backend/frontend mapping cùng workflow, planning, construction, commercial, warehouse, acceptance/handover và finance domain guards.
- Đã áp dụng V105 trên PostgreSQL local: Rev8 `...027` published/engine 4 có 15 node, 21 Step, 15 form-backed occurrence, 6 source/action-only Step, 2 node approval policy và 0 Step có nhiều hơn một vai trò chính; Rev7 vẫn giữ 15/50/13.
- Đã tạo bốn form work-package bất biến cho Survey, Design, kế hoạch thi công và nghiệm thu thầu phụ; Rev7/form/runtime lịch sử không bị sửa.
- Đã bổ sung composite `business_source.checks`, canonical execution target và UI checklist `Điều kiện nguồn · x/y` trong Work Item.
- Đã tách bàn giao vật lý và khóa tài chính chỉ cho active official-family Rev8+: regression chứng minh Rev7 vẫn bị finance chặn, còn Rev8 bàn giao được trước nhưng `closure.finance` vẫn fail-closed với chi phí/đề nghị thu chi chưa kết thúc.
- Đã thêm và chạy idempotent Golden `GOLDEN-REV8-LEAN-001`: 15 runtime node, 21 runtime Step, 15 form instance; cập nhật Makefile, validator và central/backend/frontend/workflow/planning/construction/commercial/quality/finance maps.
- Backend `make migrate-check`: 105/105 manifest hợp lệ; `go test ./...`: passed. Frontend `npm test`: 140/140 passed; `npm run build`: passed trên source state hiện tại.
- Full Golden validator trên database local không chạy hết do baseline thiếu ba App role account (`design`, `sales`, `legal`), trước khi tới assertion Rev8. Kiểm chứng database tạm sạch xác nhận 105 migration áp dụng được nhưng chuỗi seed tổng hiện có dừng khi phát lại V103 (temp-table/autocommit và assertion field occurrence của Rev7); đây là giới hạn có sẵn ngoài thay đổi Rev8, database tạm đã được xóa.
- Browser/UAT không chạy theo quy định repo; trạng thái UI/UX là `NOT UX ACCEPTED`.

## [Closed] Đưa cột action lên đầu tất cả bảng frontend

### Will be done

- Di chuyển toàn bộ 39 cấu hình `DataTable` có key `action`/`actions` lên vị trí cột đầu tiên.
- Di chuyển toàn bộ 18 biến thể bảng/grid thủ công có cột thao tác chuyên biệt lên vị trí đầu tiên.
- Bỏ tiêu đề hiển thị của mọi cột action; vẫn giữ tên hỗ trợ trình đọc màn hình khi cấu trúc hiện tại cho phép.
- Giữ nguyên callback, quyền, điều kiện hiển thị, responsive card và nghiệp vụ của từng action.
- Cập nhật frontend UI utility map cho quy ước cột action dùng chung.
- Chạy kiểm tra nguồn có thể lặp lại, test frontend liên quan và `npm run build`; không chạy browser theo quy định repo.

### Done

- Đã kiểm kê baseline: 57 định nghĩa/biến thể, gồm 43 bề mặt route/embedded và 14 bề mặt legacy/dormant.
- Đã xác nhận phạm vi với người dùng bằng câu `OK triển khai tất cả bảng đang có cột action`.
- Đã bổ sung quy ước dùng chung trong `DataTable`: key `action`/`actions` hoặc `action: true` luôn được render đầu tiên, header chỉ còn nhãn `sr-only`.
- Đã di chuyển cột thao tác của các bảng/grid thủ công trong BOQ, hợp đồng, nghiệm thu, operations, planning, công trình, kho, handover, workflow/evidence và các line editor lên đầu.
- Audit sâu đã bao phủ thêm các cột mang tên nghiệp vụ như `Việc tiếp theo`, `Mở`, mũi tên điều hướng và các action track không có header mà lượt đếm theo key ban đầu không nhận diện.
- Đã thêm regression coverage trong `tableActionColumns.test.ts`, `manualActionColumns.test.ts` và cập nhật test responsive của Project Explorer.
- Targeted tests: 15/15 passed; full frontend suite: 140/140 passed.
- Frontend production build: passed trên app source hiện tại sau toàn bộ source edits; sau lần build cuối chỉ còn cập nhật trạng thái plan nếu có.
- Browser/UAT không chạy theo quy định repo; trạng thái UX là `NOT UX ACCEPTED`.

## [Closed] Thiết kế lại dialog Gán vai trò

### Will be done

- Giữ nguyên `permission.assign`, API `/users/:id/roles`, payload, cơ chế thay thế role theo scope và bộ lọc `external_only` hiện tại.
- Việt hóa dialog, làm rõ phạm vi toàn hệ thống/công trình và chỉ hiển thị selector công trình khi cần.
- Thay lưới role bằng bảng semantic cục bộ trong `StaffPage.tsx`, có tìm kiếm, bộ lọc Tất cả/Đã chọn, responsive hai cột trên mobile, trạng thái tải/lỗi/thử lại/empty và validation inline.
- Không sửa `DataTable`, `Dialog`, router, API client, backend, migration hoặc seed đang thuộc các scope khác trong worktree.
- Thêm regression test tập trung, cập nhật central/frontend Staff-RBAC mapping và chạy frontend test/build trên source state cuối.

### Done

- Người dùng đã duyệt review package, Mockup as Text dạng bảng và scope cô lập bằng câu `OK triển khai theo scope đã re-plan`.
- Đã xác nhận `StaffPage.tsx` và `Dialog.tsx` không có diff trước triển khai; `DataTable.tsx`, mapping và file current-state đang có thay đổi từ các plan khác nên chỉ áp dụng patch hẹp vào các vị trí đã duyệt.
- Đã triển khai bảng vai trò semantic cục bộ trong `StaffPage.tsx`: tìm kiếm, lọc Tất cả/Đã chọn, chọn theo từng dòng, scope toàn hệ thống/công trình, responsive mobile, loading, lỗi/thử lại, empty state và validation inline; không sửa shared component, backend hoặc hợp đồng API/RBAC.
- Đã cập nhật central/frontend Staff-RBAC mapping và thêm regression test `staffRoleDialogPresentation.test.ts`; domain map không cần cập nhật vì invariant, API và quyền không thay đổi.
- Targeted frontend tests: 5/5 passed; full frontend suite: 143/143 passed.
- Frontend production build: passed trên app source hiện tại sau toàn bộ source edits; sau lần build cuối chỉ cập nhật trạng thái plan trong tài liệu này.
- Browser/UAT không chạy theo quy định repo; trạng thái UI/UX là `NOT UX ACCEPTED`.

## [Closed] Dashboard tổng hợp công trình và dòng tiền theo kỳ

### Will be done

- Đổi nhãn điều hướng `Điều hành` thành `Dashboard` nhưng giữ nguyên route `/dashboard` và quyền mở trang `report.view`.
- Thay hàng đợi phê duyệt/hành động trên Dashboard bằng snapshot công trình hiện tại, dòng tiền payment-ledger theo tháng/quý/năm, bảng công trình phân trang và liên kết báo cáo.
- Mở rộng `/reports/dashboard` bằng hợp đồng kỳ có kiểu rõ ràng và thêm `/reports/dashboard/projects` với search/status/sort/cursor phía server.
- Giữ snapshot công trình độc lập với kỳ; chỉ dòng tiền và các trường tiền theo công trình thay đổi theo kỳ.
- Chỉ trả/hiển thị dữ liệu tiền khi actor có `report.finance.view` hoặc `accounting.view`; `report.view` không được dùng để mở rộng quyền tài chính.
- Dùng cùng payment-ledger semantics với `/reports/cashflow`, không dùng `project_cash_requests.processed` hoặc công thức `cashflow_month` cũ làm bằng chứng tiền đã thu/chi.
- Tái sử dụng `DataTable` action-first hiện có mà không sửa shared component hoặc `frontend/src/lib/types/domain.ts` đang có thay đổi Rev8.
- Bổ sung backend/frontend regression coverage, merge mapping Dashboard vào các map hiện có và chạy test/build trên working tree cuối.

### Done

- Người dùng đã duyệt review package, impact matrix, Mockup as Text và scope bảo toàn dirty worktree bằng câu `OK triển khai theo plan đã rescope`.
- Đã khóa baseline trên `main` tại HEAD `6f41cc6`: Dashboard/report owner files chưa có diff, không có staged changes; các diff Rev8/V105/action-column và hai plan đang hoạt động được xác định là thay đổi có sẵn cần giữ nguyên.
- Đã tải central/backend/frontend mapping cùng reporting, accounting-finance, impact-analysis và git guardrails cho lượt triển khai.
- Đã thay hợp đồng `/reports/dashboard` bằng response có kiểu cho kỳ tháng/quý/năm, snapshot công trình hiện tại và capability tài chính; thêm `/reports/dashboard/projects` có search/status/sort/limit/cursor phía server.
- Đã tái sử dụng canonical approved-payment cashflow cho summary, diễn biến và breakdown theo công trình; aggregate và từng dòng công trình đều fail-closed theo `report.finance.view|accounting.view`, không dùng `report.view`, `project_cash_requests.processed` hoặc form answer để suy ra dòng tiền.
- Đã đổi nhãn AppShell thành `Dashboard` và thay màn hình Điều hành cũ bằng toàn cảnh công trình, dòng tiền theo kỳ, danh mục công trình responsive/action-first và liên kết báo cáo; không còn gọi `/approvals/pending`, không mount hành động duyệt và không tải broad `/projects`.
- Đã giữ type Dashboard trong module riêng, không sửa `frontend/src/lib/types/domain.ts` hoặc shared `DataTable.tsx` đang thuộc các plan khác; không có migration/schema change trong scope Dashboard.
- Đã cập nhật central/backend/frontend map và reporting/accounting-finance domain map cho contract, route, ownership, nguồn số liệu và ranh giới quyền mới.
- Full backend `GOCACHE=/private/tmp/consflow-dashboard-gocache make test`: passed trên working tree hiện tại. Unit coverage kỳ/cursor passed; hai DB-backed Dashboard tests được `SKIP` vì không có `CONSFLOW_TEST_DATABASE_URL`, nên repository SQL/RBAC chưa được xác nhận trên PostgreSQL trong lượt này.
- Full frontend `npm test`: 149/149 passed. Production `npm run build`: passed, 1.822 module transformed; Dashboard chunk 7,78 KiB gzip. `npm run check:bundle-budget`: passed với 114 chunk và entry 124,64 KiB gzip dưới ngân sách 250 KiB.
- `git diff --check`: passed; audit path xác nhận các owner file Dashboard ban đầu sạch và các thay đổi Rev8/V105/action-column/Staff plan có sẵn vẫn được giữ nguyên.
- Browser/UAT không chạy theo quy định repo; trạng thái UI/UX là `NOT UX ACCEPTED`. Trạng thái tổng thể là `IMPLEMENTED`; chưa `TECHNICALLY VERIFIED` đầy đủ ở tầng PostgreSQL do test DB bị thiếu và chưa `UX ACCEPTED`.
