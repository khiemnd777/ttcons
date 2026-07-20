# Current States

## My Work Action-First Redesign

Status: complete

Mục đích chính:
- Biến `/my-work` thành hộp công việc hành động: người dùng nhìn thấy ngay việc thực sự cần xử lý, lý do ưu tiên, hạn, ngữ cảnh và một CTA mở workspace chi tiết; trao đổi cần phản hồi giữ realtime nhưng là luồng phụ tách biệt.

Will be done:
- Tái cấu trúc UI thành hai chế độ `Công việc` và `Trao đổi cần phản hồi`; mặc định action-first, Việt hóa nhãn, compact empty/error/loading/offline states và responsive desktop/tablet/mobile.
- Bổ sung chọn work item thật sự, preview `Việc tiếp theo`, filter/search/sort, query-state trên URL, xóa filter, KPI tương tác, một nút làm mới và tái sử dụng `WorkItemDetailDialog` cho xử lý chi tiết.
- Mở rộng `GET /me/work-items` thành contract phân trang/cursor có server filters, source-backed summary/facets và capability theo actor; giữ cancelled/project-cancelled ngoài queue.
- Bổ sung capability cho work-item detail để UI action khớp `CanActOnWorkItem`/`CanDecideWorkItem`; backend gates vẫn authoritative.
- Bổ sung invalidation realtime `work_queue_changed` cho direct `WorkItemAction`; giữ evidence/form/runtime theo refresh contract hiện hữu và activity read/resolve tách biệt workflow transition.
- Không thêm bulk approve/complete, snooze, saved views, migration, permission hay thay đổi template/runtime/form/evidence/approval gate.
- Cập nhật central/frontend/backend/workflow/admin mappings theo contract mới.
- Chạy focused source/tests, backend tests, frontend build và `git diff --check`; browser verification bỏ qua vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- Người dùng đã duyệt toàn bộ review package/Mockup as Text và yêu cầu delegate subagents bằng `Approve for all with delegate subagents`.
- Đã tải mandatory central/frontend/backend mappings, frontend/backend workflows, workflow-orchestration, impact-analysis và admin-governance guardrails.
- Đã xác nhận owner frontend là `MyWorkPage.tsx`, shared work-item presentation/detail và `domain.ts`; owner backend là route/handler/service/repository/types của `/me/work-items`, work-item detail/action và activity SSE.
- Đã xác nhận current queue filter/count chạy client-side trên tối đa 200 rows, context luôn lấy dòng đầu, attention tối đa 120 rows/chỉ render 8 và UI action hiện suy từ status trong khi backend tách `CanActOnWorkItem`/`CanDecideWorkItem`.
- Đã khóa ownership: backend và frontend do hai subagent riêng triển khai; root giữ plan, mapping, integration và validation cuối.
- Đã tái cấu trúc `/my-work` thành hai chế độ `Công việc` và `Trao đổi cần phản hồi`, bổ sung URL-backed KPI/queue/search/filter/sort, cursor load-more có dedupe, selected-item preview, shared `WorkItemDetailDialog`, source-local loading/error/empty states và responsive layout.
- Đã đổi `/me/work-items` sang page object với server SQL filtering/count/summary/facets, opaque cursor, cancelled exclusion, role-aware Department, requester-separated Pending Approval và actor-scoped `capabilities`/`available_actions`; không thêm migration, permission, seed hoặc route mới.
- Đã làm detail action fail-closed, giữ backend mutation gates authoritative, sửa parity watcher/decision role/project scope/empty approval roles và dùng timezone `Asia/Ho_Chi_Minh` cho `done_today`.
- Đã bổ sung invalidation-only `work_queue_changed` cho successful direct `WorkItemAction`; project activity stream không forward work-item event và communication vẫn dùng `attention_changed`.
- Đã cập nhật central/frontend/backend/workflow/admin mappings và thu hẹp realtime claim đúng phạm vi direct work-item transition.
- Frontend `npm run lint` và `npm run build` pass; backend `go test ./...` và focused My Work tests pass; `git diff --check` pass. DB-backed My Work pagination/RBAC test đã được thêm nhưng skip khi không có `CONSFLOW_TEST_DATABASE_URL`; browser verification bỏ qua theo repo instruction vì người dùng không yêu cầu.
- Post-implementation reviewer xác nhận không còn blocking finding sau khi sửa decision parity, KPI queue alignment, capability fail-closed, reset queue và business timezone.

## Reports Analytics Workspace Redesign

Status: complete

Mục đích chính:
- Biến `/reports` từ một trang cuộn dài tải đồng thời mọi nguồn thành workspace phân tích permission-aware, mỗi lần hiển thị một báo cáo với phạm vi, cảnh báo, bảng nguồn và CSV nhất quán.

Will be done:
- Sửa query aggregate công trình, kho và kế toán để loại bỏ nhân bản/thiếu số; scope báo cáo nhân sự theo các công trình người dùng được phép xem.
- Thêm `project_id` filter và CSV export parity cho năm báo cáo vận hành; giữ P&L/VAT theo invoice ledger và Cashflow theo payment ledger.
- Tổ chức lại frontend thành danh mục báo cáo tài chính/vận hành/nguồn lực, chỉ gọi query của báo cáo đang mở và ẩn mục không có quyền.
- Tái sử dụng `CashflowPanel`, chuẩn hóa nhãn tiếng Việt, loading/error/empty/permission, deep-link `/reports/:reportKey`, bộ lọc áp dụng rõ ràng và điều hướng nguồn nghiệp vụ.
- Cập nhật route, types, report tests và central/frontend/backend/reporting/source-domain mappings trong cùng turn.
- Chạy focused/full backend tests khi khả dụng, frontend test/build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- Người dùng đã duyệt review package và Mockup as Text bằng `Approve`.
- Đã tải mandatory central/frontend/backend/root-cause/reporting mappings cùng frontend, backend, diagnosis, impact-analysis và source-domain guardrails.
- Đã xác nhận root cause query: join issue×approval trước COUNT, join stock movement×site balance trước SUM, và `SUM(DISTINCT amount)` trong accounting; staff report bỏ qua project scope.
- Đã xác nhận route `/reports` dùng any-permission nhưng trang hiện gọi mọi endpoint; finance ProjectSelector gọi `/projects` ngoài contract báo cáo.
- Worktree đang chứa thay đổi người dùng ở AppShell, Dashboard, Projects, domain types, mappings và docs; implementation sẽ chỉnh hẹp và bảo toàn các thay đổi đó.
- Backend đã thêm `/reports/options`, optional `project_id` và CSV parity cho năm báo cáo vận hành; handler và service cùng kiểm tra permission/project scope.
- Aggregate đã được tách đúng nguồn: project dùng subquery issue/approval độc lập; kho dùng CTE movement/site balance; accounting dùng CTE AP/AR/budget với `SUM` thường; staff chỉ join active membership của accessible project.
- Frontend đã chuyển thành workspace `/reports/:reportKey` với catalog theo quyền, desktop rail/mobile selector, lazy-load một panel, project options scoped và điều hướng nguồn nghiệp vụ theo permission.
- P&L, VAT, Cashflow và operational reports dùng shared apply/reset/CSV controls; applied filters được phản ánh trên URL, nhãn chính được Việt hóa và DataTable giữ loading/error/retry/empty/search/paging.
- Warning có dòng nguồn trong P&L/VAT có thể bấm để lọc ledger theo exact backend warning flag; warning aggregate hoặc dòng bị loại vẫn giữ trạng thái thông tin trung thực.
- Đã tái sử dụng `CashflowPanel` trong report workspace, mở quyền route/nav cho `accounting.view`, đồng thời giữ module Staff là nơi xem toàn công ty bằng `staff.view`.
- Đã bổ sung regression tests cho project cross-join, warehouse cross-join, accounting equal-amount totals, staff scope, report options, operational CSV và frontend report catalog permissions.
- Đã cập nhật central frontend/backend/root-cause mappings cùng reporting, project, materials, procurement, accounting và admin domain maps cho contract mới.
- Validation đạt: `GOCACHE=/tmp/ttcons-go-cache go test ./...`, `npm run test:reports`, `npm run lint`, production `npm run build` và `git diff --check`; Vite chỉ còn cảnh báo chunk-size hiện hữu.
- Browser verification được bỏ qua đúng guardrail vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

## AppShell Horizontal Brand Lockup

Status: complete

Mục đích chính:
- Biến khu vực logo thành điểm nhận diện rõ ràng và lối quay về trang mặc định, đồng thời giữ AppShell gọn và không làm lệch hàng header 64px.

Will be done:
- Đổi logo desktop 48px căn giữa thành lockup ngang gồm mark 40px và wordmark hai dòng `Thủ Thiêm` / `WORKFLOW`, căn trái trong sidebar 288px.
- Biến toàn bộ lockup desktop và mark mobile thành liên kết về `/`, tiếp tục dùng `HomeRedirect` permission-aware hiện hữu.
- Giữ nguyên SVG, favicon, Login 112px, header 64px, sidebar/navigation, route, API, backend, database và permission.
- Cập nhật central/frontend mappings cho branding/AppShell contract.
- Chạy source-level contract, frontend lint/build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- Người dùng đã duyệt review package, Mockup as Text và mockup PNG bằng `Approve`.
- Đã tải mandatory central/frontend mappings cùng frontend workflow/UI guardrails; owner chính là `BrandLogo.tsx` và `AppShell.tsx`.
- Đã xác nhận `/` dùng `HomeRedirect` để đưa người có `project.view` tới `/projects`, người còn lại tới `/my-work`; không cần thay đổi router.
- Đã xác nhận worktree đang có thay đổi navigation đã duyệt trong `AppShell.tsx`; implementation sẽ bảo toàn các thay đổi đó.
- Đã mở rộng `BrandLogo` bằng hai biến thể `mark` và `lockup`; lockup giữ mark SVG 40px, thêm wordmark `Thủ Thiêm` / `WORKFLOW` bằng text HTML để tên vẫn đọc được nếu asset ảnh lỗi.
- Đã thay logo desktop 48px căn giữa bằng lockup ngang căn trái trong sidebar 288px, giữ nguyên hàng header 64px và không thay đổi navigation.
- Đã biến toàn bộ lockup desktop và mark mobile thành liên kết keyboard-focusable về `/`; `HomeRedirect` hiện hữu tiếp tục quyết định landing route theo permission.
- Đã giữ nguyên SVG, favicon, Login 112px, router, API, backend, database và permission.
- Đã cập nhật central/frontend mappings cho variant, kích thước, responsive treatment và hành vi điều hướng branding; không cần backend/domain map vì không đổi route contract hoặc nghiệp vụ.
- Validation đạt: source-level contract, `npm run lint`, production `npm run build` và `git diff --check`; Vite chỉ báo lại cảnh báo chunk-size hiện có.
- Browser verification được bỏ qua đúng guardrail vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

## Project Explorer Light Dashboard

Status: complete

Mục đích chính:
- Khi mở rộng một công trình, người dùng vừa chọn nhanh phân hệ nghiệp vụ ở cột trái, vừa nắm tình trạng công trình qua dashboard nhẹ ở cột phải mà không rời danh sách.

Will be done:
- Chia vùng con của project đang mở rộng thành hai cột desktop 58/42; mobile xếp folder trước và dashboard sau.
- Giữ nguyên cấu hình folder, permission và group landing route dùng chung; dashboard hiển thị `Tiến độ`, `Vướng mắc`, `Chờ phê duyệt`, `Hạn dự kiến`.
- Chỉ dùng dữ liệu source-backed hiện có từ `/projects`: `progress_percent`, `open_issues`, `pending_approvals`, `planned_start_date`, `planned_end_date`; không thêm request, API, backend hoặc migration.
- Bổ sung typed project-list fields và cập nhật central/frontend/project-planning mappings.
- Chạy frontend production build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- Người dùng đã duyệt review package và Mockup as Text bằng `OK triển khai`; wording `VẤN ĐỀ MỞ` đã được đổi thành `VƯỚNG MẮC` trước khi triển khai.
- Đã tải mandatory central/frontend/project-planning mappings cùng frontend UI guardrails; owner chính là `ProjectsPage.tsx` và `domain.ts`.
- Đã xác nhận `/projects` đã trả các metric cần dùng và áp dụng project visibility scope; implementation không cần thay đổi backend.
- Đã chia vùng mở rộng thành hai cột desktop 58/42, giữ folder/permission/landing route ở cột trái và đặt `Tổng quan nhanh` ở cột phải; dưới `lg` hai vùng tự xếp dọc với folder trước.
- Dashboard nhẹ hiển thị `Tiến độ`, `Vướng mắc`, `Chờ phê duyệt`, `Hạn dự kiến`; số 0 dùng trạng thái trung tính, ngày thiếu hiển thị `Chưa thiết lập`, tiến độ được giới hạn 0-100%.
- Đã bổ sung typed list fields `pm_name`, planned dates, `open_issues`, `pending_approvals`; không thêm request, API, backend, database, route, permission hoặc migration.
- Đã cập nhật central/frontend/project-planning mappings cho layout, wording, nguồn metric và responsive contract; reporting map không cần đổi vì không thay metric/report semantics.
- Validation đạt: `npm run build`, `npm run test:project-navigation`, source-level wording/field check và `git diff --check`; Vite chỉ báo lại cảnh báo chunk-size hiện có.
- Browser verification được bỏ qua đúng guardrail vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

## Dashboard Sidebar Entry And Công việc Group

Status: complete

Mục đích chính:
- Đưa Dashboard thành lối vào đầu tiên của nhóm tác nghiệp chính và đổi tên nhóm từ `Làm việc` thành `Công việc`.

Will be done:
- Thêm item `Dashboard` đầu nhóm, route `/dashboard`, icon `LayoutDashboard`, permission `report.view`.
- Đổi tiêu đề nhóm `Làm việc` thành `Công việc`; giữ nguyên thứ tự Công trình, Việc của tôi và Báo cáo sau Dashboard.
- Giữ nguyên `HomeRedirect`, route, API, backend, database, permission definition và Dashboard content.
- Cập nhật central/frontend mappings cho navigation contract.
- Chạy source-level contract, frontend lint/build và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- Người dùng đã duyệt review package và Mockup as Text bằng `OK triển khai`.
- Đã tải mandatory central/frontend mappings cùng frontend UI guardrails.
- Đã xác nhận `/dashboard` tồn tại và dùng `report.view`; owner thay đổi là `frontend/src/layouts/AppShell.tsx`.
- Đã xác nhận worktree đang chứa thay đổi Dashboard đã duyệt từ plan trước; implementation sẽ chỉ chèn thay đổi điều hướng và không ghi đè các thay đổi hiện hữu.
- Đã đổi nhóm `Làm việc` thành `Công việc` và thêm `Dashboard` ở vị trí đầu tiên với icon `LayoutDashboard`, route `/dashboard`, permission `report.view`.
- Đã giữ nguyên thứ tự Công trình, Việc của tôi, Báo cáo sau Dashboard; active state và permission filtering tiếp tục dùng `NavLink`/`hasAnyPermission` hiện hữu.
- Đã giữ nguyên `HomeRedirect`, router, DashboardPage, API, backend, database và permission definitions.
- Đã cập nhật central/frontend mappings cho group label, Dashboard nav entry, thứ tự và quyền; reporting domain map không cần đổi vì metric/report semantics không thay đổi.
- Validation đạt: source-level navigation contract, `npm run lint`, production `npm run build`, router no-diff và `git diff --check`; Vite chỉ báo lại cảnh báo chunk-size hiện có.
- Browser verification được bỏ qua đúng guardrail vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

## Dashboard Tổng quan điều hành

Status: complete

Mục đích chính:
- Biến Dashboard từ lưới KPI đồng hạng thành màn hình điều hành ưu tiên việc cần xử lý, công trình cần chú ý và tình hình tài chính trong phạm vi người dùng được phép xem.

Will be done:
- Đổi tiêu đề thành `Tổng quan điều hành`, Việt hóa toàn bộ nhãn Dashboard và ba nhãn điều hướng `Pipeline builder`, `Workflow Forms`, `Audit`.
- Tổ chức lại `/dashboard` theo thứ bậc: Cần xử lý -> Việc chờ phê duyệt/Công trình cần theo dõi -> Tình trạng công trình/Tài chính tổng hợp.
- Giữ nguyên route, API và công thức backend; dùng đúng dữ liệu `/reports/dashboard`, `/approvals/pending`, `/projects` hiện hữu.
- Chỉ gọi `/projects` khi người dùng có `project.view`; hiển thị loading/error/empty/permission riêng thay vì biến lỗi thành số 0 hoặc danh sách rỗng.
- Lọc danh sách cần theo dõi từ dữ liệu đã được backend scope, theo `blocked`, `open_issues` hoặc `pending_approvals`; không thêm chuyển trạng thái hoặc hành động duyệt tại Dashboard.
- Cập nhật central/frontend/reporting mappings; chạy frontend lint/build và `git diff --check`.
- Không chạy browser verification vì người dùng chưa yêu cầu điều khiển browser trong turn triển khai.

Done:
- Người dùng đã duyệt review package và Mockup as Text bằng `Approve`; phạm vi được hiểu là duyệt cả frontend-only và Việt hóa ba nhãn điều hướng toàn cục.
- Đã tải mandatory central/frontend/backend/reporting mappings cùng frontend UI và reporting guardrails.
- Đã xác nhận owner chính là `frontend/src/modules/dashboard/DashboardPage.tsx` và `frontend/src/layouts/AppShell.tsx`; không sửa `StatCard` vì Warehouse và Procurement đang dùng chung.
- Đã xác nhận API Dashboard dùng payment-ledger cho dòng tiền, áp dụng `ProjectAccessFilter`, và metric `active_projects` bao gồm planning/in-progress/acceptance/handover nên nhãn mới là `Đang triển khai`.
- Đã xác nhận `/dashboard` yêu cầu `report.view` nhưng `/projects` yêu cầu `project.view`; implementation sẽ điều kiện hóa query và hiển thị permission state.
- Đã tái cấu trúc Dashboard đúng thứ bậc đã duyệt: `Cần xử lý`, `Việc chờ phê duyệt`, `Công trình cần theo dõi`, `Tình trạng công trình`, `Tài chính tổng hợp`; loại bỏ lưới KPI và khối công nợ lặp.
- Đã thêm `Làm mới` cho toàn bộ query đang được phép chạy; mỗi nguồn có loading skeleton, lỗi với `Thử lại`, empty state và permission state riêng, không còn biến lỗi API thành số 0 hoặc danh sách rỗng.
- Đã điều kiện hóa `/projects` bằng `project.view`; attention list chỉ dùng các dòng non-cancelled đã được backend scope và sắp xếp blocked -> pending approval -> open issue -> tiến độ thấp.
- Đã giữ phê duyệt chỉ đọc trên Dashboard: work item có thể mở nguồn, các loại khác chỉ mở công trình khi có quyền; không thêm mutation hoặc bypass audit/permission.
- Đã thay thanh trạng thái bằng tỷ trọng thật trên tổng trạng thái trả về, hiển thị tối đa một chữ số thập phân; nhãn `Đang triển khai` phản ánh đúng tập trạng thái backend.
- Đã hợp nhất tài chính thành phải thu còn lại, phải trả còn lại và dòng tiền ròng tháng hiện tại; không thay đổi metric, API, route, backend, database hoặc permission.
- Đã Việt hóa AppShell thành `Trình thiết kế quy trình`, `Biểu mẫu quy trình`, `Nhật ký hệ thống`; cấu trúc nhóm, route và permission giữ nguyên.
- Đã cập nhật central/frontend/reporting mappings cho Dashboard và nhãn điều hướng mới; backend map không cần đổi vì không có route, handler, service, repository hoặc contract thay đổi.
- Validation đạt: source-level contract, `npm run lint`, production `npm run build` và `git diff --check`; Vite chỉ báo lại cảnh báo chunk-size hiện có.
- Browser verification được bỏ qua đúng guardrail vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

## AppShell Header Alignment And 35px Logo Restore

Status: complete

Mục đích chính:
- Căn khối logo sidebar và application header thành một hàng desktop thống nhất, lấy header bên phải làm chuẩn, đồng thời khôi phục tỷ lệ chữ `Workflow` 35px trong logo.

Will be done:
- Đặt header logo trái và application header phải cùng chiều cao 64px; dùng logo desktop 48px căn giữa.
- Đồng bộ top inset sidebar từ 16px lên 20px để `LÀM VIỆC` bắt đầu cùng cao độ với vùng `Công trình` dùng main inset 20px.
- Khôi phục `Workflow` trong SVG từ 29px về 35px; giữ logo mobile 40px và Login 112px.
- Không đổi route, API, auth, database, permission, nội dung navigation hoặc internal `consflow` identifiers.
- Cập nhật central/frontend/root-cause mappings; chạy source-level contract, SVG XML, frontend lint/build và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- Screenshot người dùng cung cấp đã xác nhận divider dưới logo trái thấp hơn divider header phải và sidebar content bắt đầu thấp hơn vùng `Công trình`.
- Code evidence xác nhận header trái khoảng 137px (`96px + py-5 + border`), trong khi header phải khoảng 64px; sidebar dùng `py-4` còn main dùng `py-5`.
- Root cause đã xác nhận là hai contract sizing/inset độc lập trong `AppShell.tsx`, không phải typography hoặc border nội bộ của logo.
- Pre-fix source contract đã fail vì chưa có shared 64px row, logo desktop vẫn 96px và SVG `Workflow` đang 29px.
- Người dùng đã duyệt Mockup as Text, mockup PNG và xác nhận triển khai bằng `OK triển khai`; yêu cầu bổ sung khôi phục logo 35px đã được đưa vào cùng scope.
- Đã tải mandatory central/frontend/root-cause mappings cùng frontend/diagnosis guardrails; owner chính là `AppShell.tsx` và `brand-logo.svg`.
- Worktree đang có thay đổi hiện hữu ở các owner/mapping files; implementation chỉ chỉnh đúng các dòng mục tiêu và giữ nguyên thay đổi không liên quan.
- Đã đặt brand cell trái và application header phải cùng `h-16` (64px), dùng logo desktop 48px căn giữa và giữ nguyên header phải làm chuẩn trực quan.
- Đã đổi sidebar navigation từ `py-4` sang `py-5`, đồng bộ 20px top inset với main để `LÀM VIỆC` và vùng `Công trình` bắt đầu cùng cao độ.
- Đã khôi phục dòng `Workflow` trong SVG từ 29px về 35px; logo mobile 40px và Login 112px giữ nguyên.
- Đã cập nhật central/frontend mappings cho branding/AppShell contract và root-cause map cho đường chẩn đoán divider/inset lệch; không cần backend/domain map vì route, API, auth, database, permission và runtime không đổi.
- Validation đạt: source-level contract kiểm đủ 64px/48px/20px, mobile 40px, Login 112px và SVG 35px; SVG XML, `npm run lint`, `npm run build`, asset trong `dist` và `git diff --check` đều pass. Vite chỉ báo lại cảnh báo chunk-size hiện có.
- Không thêm browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai; mockup PNG đã được duyệt trước khi sửa code.

## Project Explorer Thi công & Chất lượng Description

Status: complete

Mục đích chính:
- Đồng bộ mô tả nhóm `Thi công & Chất lượng` trong cây công trình với các không gian người dùng hiện thấy: Nhật ký, Kiểm tra, Cần xử lý và Nghiệm thu.

Will be done:
- Đổi description tĩnh từ `Thi công, ITP/RFI/NCR, nghiệm thu, rework` thành `Nhật ký, kiểm tra, cần xử lý, nghiệm thu`.
- Giữ nguyên nhãn nhóm, nhóm con, route, permission, API, database và toàn bộ chuyển trạng thái Thi công/Chất lượng.
- Cập nhật central/frontend mappings cho copy điều hướng dùng chung.
- Chạy source-level contract, frontend production build và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- User đã duyệt review package và Mockup as Text bằng `OK triển khai`.
- Đã tải mandatory central/frontend/construction/quality mappings cùng frontend và domain guardrails.
- Đã xác nhận owner chính là `frontend/src/modules/projects/projectWorkspaceNavigation.ts`; `ProjectsPage.tsx` chỉ render `folder.description` từ cấu hình dùng chung.
- Đã xác nhận UI Thi công hiện chỉ có `Nhật ký`, `Kiểm tra`, `Cần xử lý`; Nghiệm thu là nhóm con còn lại, còn ITP/RFI/NCR/rework là lịch sử tương thích kỹ thuật.
- Worktree đang có thay đổi hiện hữu ở nhiều module và mapping files; implementation chỉ chỉnh đúng các dòng mục tiêu và giữ nguyên thay đổi không liên quan.
- Đã đổi description dùng chung thành `Nhật ký, kiểm tra, cần xử lý, nghiệm thu`; nhãn nhóm, nhóm con, route và permission giữ nguyên.
- Đã cập nhật central/frontend mappings cho copy điều hướng mới; construction/quality domain maps không cần sửa vì các invariant đã ghi đúng luồng `Nhật ký`/`Kiểm tra`/`Cần xử lý`, Acceptance và lịch sử tương thích kỹ thuật.
- Source-level contract đạt: description mới tồn tại đúng owner file và chuỗi cũ không còn trong `frontend/src`.
- Frontend production build đạt; Vite chỉ báo lại cảnh báo chunk-size hiện có.
- Browser verification được bỏ qua đúng quy định vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

## Thủ Thiêm Workflow Logo Safe Right Padding

Status: complete

Mục đích chính:
- Thu nhỏ riêng dòng `Workflow` trong logo navy để tạo khoảng trống an toàn bên phải, đồng thời giữ nguyên kích thước ô và hai dòng tên chính.

Will be done:
- Giảm cỡ chữ SVG của `Workflow` từ 35px xuống 29px; giữ nguyên vị trí, màu, độ đậm, ô navy và hai dòng `Thủ`/`Thiêm`.
- Giữ nguyên kích thước hiển thị 96px/40px trong AppShell, 112px trên Login và cùng asset cho favicon.
- Không đổi route, API, auth, database, permission hoặc internal `consflow` identifiers.
- Cập nhật central/frontend mappings; chạy source-level contract, SVG XML, frontend lint/build và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- Screenshot người dùng cung cấp đã xác nhận dòng `Workflow` 35px nằm quá sát giới hạn bên phải trong Microsoft Edge.
- Root cause đã được xác nhận tại `frontend/public/brand-logo.svg`: cỡ chữ cố định 35px không chừa đủ khoảng an toàn với font fallback thực tế.
- Pre-fix contract đã fail tại 35px; mockup PNG 35px/29px đã được người dùng xem và duyệt bằng `OK triển khai thu nhỏ Workflow`.
- Đã tải mandatory central/frontend/root-cause mappings cùng frontend/diagnosis guardrails và xác nhận owner chính là SVG branding dùng chung.
- Worktree đang có thay đổi hiện hữu ở nhiều module; implementation chỉ chỉnh các dòng mục tiêu và giữ nguyên thay đổi không liên quan.
- Đã giảm riêng dòng `Workflow` từ 35px xuống 29px; giữ nguyên tọa độ, màu, độ đậm, hai dòng `Thủ`/`Thiêm` và toàn bộ kích thước hiển thị của component.
- Đã cập nhật central/frontend mappings với contract 29px và safe right padding; không cần backend/domain map vì route, API, auth, database, permission và runtime không đổi.
- Validation đạt: source-level 29px contract, SVG XML, `npm run lint`, `npm run build`, asset trong `dist` và `git diff --check` đều pass; Vite chỉ báo lại cảnh báo chunk-size hiện có.
- Browser verification được bỏ qua đúng quy định vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

## Thủ Thiêm Workflow Square Navy Logo

Status: complete

Mục đích chính:
- Thay placeholder `CF` bằng logo chữ `Thủ Thiêm Workflow` dạng ô vuông nền navy đã được duyệt, giúp nhận diện thương hiệu nhất quán tại sidebar, trang đăng nhập và favicon.

Will be done:
- Tạo SVG 1:1 nền `#172033`, chữ `Thủ`/`Thiêm` màu trắng và `Workflow` màu `#60A5FA`, bo góc và giữ dấu tiếng Việt bằng text vector code-native.
- Thêm component `BrandLogo` dùng chung; hiển thị 96px trong sidebar và 112px trên trang đăng nhập.
- Xóa placeholder `CF`, wordmark/subtitle lặp trong sidebar và đổi login heading còn `Đăng nhập`; đăng ký SVG làm favicon.
- Không đổi route, API, auth, database, permission, demo account hoặc internal `consflow` identifiers.
- Cập nhật central/frontend mappings; chạy kiểm tra SVG, frontend production build và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- User đã chọn phương án 2 ô nền navy và duyệt triển khai bằng `OK triển khai phương án 2`.
- Đã tải mandatory central/frontend mappings, frontend workflow/UI guardrails và xác nhận owner files.
- Đã xác nhận current UI dùng placeholder `CF` 40/44px tại AppShell/Login, sidebar lặp product name/subtitle và browser chưa đăng ký favicon.
- Worktree đang có thay đổi hiện hữu ở nhiều module, gồm `AppShell.tsx` và mapping files; implementation sẽ chỉnh mục tiêu và giữ nguyên các thay đổi không liên quan.
- Đã tạo `brand-logo.svg` 1:1 nền navy, chữ trắng/xanh đúng wording và component `BrandLogo` dùng chung có alt text tiếng Việt.
- Đã thay placeholder/sidebar wordmark bằng logo 96px, thêm logo compact 40px cho header hẹp, dùng logo 112px tại Login và rút heading còn `Đăng nhập`.
- Đã đăng ký cùng SVG làm favicon, rút browser title còn `Thủ Thiêm Workflow` và xóa `Construction Workflow OS` khỏi frontend presentation.
- Đã cập nhật central/frontend mappings cho asset, component, kích thước và owner contract mới; không cần backend/domain map vì route, API, auth, database, permission và runtime không đổi.
- Validation đạt: SVG XML hợp lệ, production build copy đúng favicon/asset, `npm run lint`, `npm run build` và `git diff --check` đều pass; Vite chỉ báo lại cảnh báo chunk-size hiện có.
- Browser verification được bỏ qua đúng quy định vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

## Pipeline Template PDF Single Diagram

Status: complete

Mục đích chính:
- Mở đầu PDF bằng một sơ đồ workflow đầy đủ, tránh lặp lại hai phiên bản tổng quan/chi tiết của cùng topology.

Will be done:
- Bỏ trang `SƠ ĐỒ TỔNG QUAN`; đưa renderer chi tiết lên trang đầu và đổi tiêu đề thành `1. SƠ ĐỒ`.
- Giữ card hai dòng `Tên node` + `Loại | N steps.` và toàn bộ nhãn connector tối đa hai dòng.
- Đổi bảng Node thành section 2, Checklist/Step thành section 3; giữ no-node/no-edge/dangling/cycle fallback.
- Xóa code/test chỉ phục vụ overview và cập nhật central/backend/frontend/workflow mappings cho contract một diagram.
- Không đổi endpoint, frontend, database, permission, audit, runtime, dependency hoặc migration.
- Chạy focused/full backend tests, module verification, Pipeline Builder contract check, `git diff --check` và render Poppler kiểm tra toàn bộ PDF.
- Không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- User đã duyệt review package và Mockup as Text bằng `OK triển khai`.
- Đã tải mandatory central/backend/workflow mappings cùng backend/PDF guardrails; matched feature là `Pipeline Template PDF Export`, lớp artifact generation.
- Đã xác nhận owner chính là `pipeline_template_pdf.go`, `pipeline_template_pdf_diagram.go` và tests; API/frontend download contract không đổi.
- Đã bỏ hoàn toàn page/style/renderer tổng quan; trang đầu dùng renderer topology đầy đủ với tiêu đề `1. SƠ ĐỒ`.
- Đã giữ card hai dòng `Tên node` + `Loại | N steps.` và toàn bộ nhãn connector tối đa hai dòng; empty/no-edge/dangling/cycle fallback không đổi.
- Đã đổi bảng Node thành section 2 và Checklist/Step thành section 3; fixture 13 node giảm từ 6 còn 5 trang.
- Đã thay overview-label regression bằng kiểm tra một diagram hiển thị mọi connector label và cập nhật central/backend/frontend/workflow mappings.
- Không đổi route, handler/service contract, frontend, database, permission, audit, runtime, dependency hoặc migration.
- Validation đạt: focused/full backend tests, `go mod verify`, Pipeline Builder production build/regression check, `git diff --check` và Poppler visual QA toàn bộ 5 trang A4 ngang.
- Browser verification được bỏ qua đúng guardrail vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

## Pipeline Template PDF Overview + Detail

Status: complete

Mục đích chính:
- Tách sơ đồ tổng quan dễ đọc khỏi sơ đồ chi tiết đầy đủ để PDF trình bày rõ tuyến chính, nhánh quyết định và toàn bộ nhãn connector mà không nhồi nội dung vào một trang.

Will be done:
- Trang 1 dùng topology document-first với card một dòng, chỉ hiển thị nhãn tại node phân nhánh và lời dẫn sang trang chi tiết.
- Trang 2 dùng topology document-first với compact card hai dòng `Tên node` và `Loại | N steps.`, toàn bộ nhãn connector tối đa hai dòng.
- Bỏ phụ thuộc tọa độ/connector-side của canvas trong PDF; source/target/label vẫn lấy đúng revision template hiện tại.
- Đổi bảng Node thành section 3 và Checklist/Step thành section 4; giữ empty/no-edge/dangling/cycle fallback rõ ràng.
- Không đổi endpoint, frontend, database, permission, audit hoặc runtime; không thêm dependency hay migration.
- Bổ sung topology/overview/detail/label regression tests, cập nhật central/backend/frontend/workflow mappings, chạy backend checks và render Poppler kiểm tra toàn bộ PDF.
- Không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- User đã chọn phương án `Overview + Detail` và duyệt review package/Mockup as Text bằng `OK triển khai`.
- Đã tải mandatory central/backend/workflow mappings cùng backend/PDF guardrails; owner chính là `pipeline_template_pdf_diagram.go`, route/handler/service và frontend download contract giữ nguyên.
- Đã xác nhận root cause hiện hữu: PDF dùng card canvas `236x136`, auto-layout theo lưới `order_index` không đọc topology và label fallback có thể rút xuống 10mm.
- Đã tách PDF thành trang 1 `SƠ ĐỒ TỔNG QUAN`, trang 2 `SƠ ĐỒ CHI TIẾT`, rồi section 3 Node và section 4 Checklist/Step; trường hợp không có node không tạo trang chi tiết rỗng.
- Đã thay renderer bằng topology phân tầng 5 cột, lane riêng cho nhánh và gutter cho connector chuyển hàng; cả hai diagram không phụ thuộc tọa độ/connector-side canvas.
- Overview dùng card một dòng và chỉ hiện nhãn ở node quyết định; Detail dùng card hai dòng `Tên node` + `Loại | N steps.` và hiển thị đủ nhãn connector tối đa hai dòng.
- Đã bổ sung fallback no-edge/cycle theo thứ tự node ổn định, bỏ dangling edge kèm cảnh báo, và regression tests cho topology, nhánh/merge, band wrap, cycle, compact card cùng label placement.
- Đã cập nhật central/backend/frontend/workflow mappings; không cần cập nhật API, frontend, database, permission, audit, runtime, dependency hoặc migration.
- Validation đạt: focused backend test, full backend test, module verification, Pipeline Builder contract check, `git diff --check` và render Poppler A4 ngang 6 trang; đã kiểm tra trực quan toàn bộ trang, trong đó hai trang diagram có 13 node/13 connector và nhãn nhánh bám đúng cạnh.
- Browser verification được bỏ qua đúng guardrail vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

## Pipeline Template PDF Diagram

Status: complete

Mục đích chính:
- Thêm sơ đồ vector của revision Workflow đang chọn vào trang đầu PDF để người đọc nắm luồng Node/Connector trước khi xem bảng cấu hình chi tiết.

Will be done:
- Mở rộng PDF document model với node ID/vị trí và connector source/target/label/diagram metadata từ template detail hiện hữu.
- Vẽ diagram vector A4 ngang: dùng vị trí đã lưu khi đủ rõ, tự sắp xếp khi thiếu/quá giãn, giữ connector-side thủ công, route trực giao và nhãn connector.
- Đặt `1. SƠ ĐỒ WORKFLOW` ở trang đầu; đổi bảng Node thành section 2 và Checklist/Step thành section 3.
- Hiển thị trạng thái không có node, không có connector và cảnh báo connector thiếu source/target; không làm hỏng toàn bộ PDF draft.
- Không đổi endpoint, frontend button, database, permission, audit hoặc runtime; không thêm dependency.
- Bổ sung unit/regression tests, cập nhật central/backend/frontend/workflow mappings, chạy format/test/build và render Poppler kiểm tra mẫu phân nhánh nhiều trang.
- Không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- User đã duyệt review package và Mockup as Text bằng `OK triển khai`.
- Đã tải mandatory central/backend/frontend/workflow mappings cùng impact/PDF guardrails và xác nhận owner chính là backend PDF renderer.
- Đã xác nhận template detail đã trả `edges`, node có tọa độ đã lưu và connector-side thủ công nằm trong `condition_config.diagram`; không cần API hoặc schema mới.
- Đã ghi nhận routing tương tác đầy đủ hiện chạy ở frontend; PDF sẽ dùng renderer vector riêng, không xuất grid/zoom/selection/editor chrome và không thay đổi canvas/runtime.
- Đã mở rộng document mapper với node ID/tọa độ, edge source/target/label/priority và connector-side metadata; giữ nguyên API, permission và frontend download flow.
- Đã thêm renderer vector trang đầu với node card, connector trực giao, mũi tên, nhãn cạnh, saved-layout/fitted-auto-layout và trạng thái rỗng/cạnh treo; bảng Node/Checklist chuyển thành section 2/3.
- Đã bổ sung regression test cho saved layout, manual connector sides, auto-layout fallback, dangling edge và alias metadata; focused backend test đang đạt.
- Đã cập nhật central/backend/frontend/workflow mappings cho diagram export contract.
- Đã bổ sung kiểm tra saved-layout không chồng node và bộ đặt nhãn connector tránh node/nhãn khác; sơ đồ giữ nhãn rõ ràng kể cả khoảng nối ngắn.
- Validation đạt: focused `go test ./internal/core`, full `go test ./...`, `go mod verify`, `npm run check:pipeline-builder` và `git diff --check`; Vite chỉ báo lại cảnh báo chunk-size hiện có.
- Đã render mẫu cuối bằng Poppler: PDF A4 ngang 5 trang, trang 1 có 13 node/13 connector phân nhánh, Unicode/metadata/footer đúng và toàn bộ nhãn connector hiển thị không bị node che. Browser verification được bỏ qua đúng guardrail vì người dùng không yêu cầu.

## Workflow Forms UI Redesign

Status: complete

Mục đích chính:
- Giúp quản trị viên tìm đúng biểu mẫu, nhận biết rõ template/version đang dùng và chỉnh draft rồi phát hành an toàn mà không nhầm với phiên bản lịch sử.

Will be done:
- Tổ chức lại `/workflow-forms` thành thư viện biểu mẫu gọn bên trái và workspace template/version có thứ bậc rõ bên phải; màn hình hẹp dùng bộ chọn template phía trên.
- Việt hóa nhãn nghiệp vụ, ưu tiên tên field trước khóa kỹ thuật và giữ nguyên số liệu field/pipeline step/instance từ API hiện hữu.
- Thu gọn draft field thành danh sách mở rộng, đưa JSON vào cấu hình nâng cao và giữ nguyên validation/payload hiện có.
- Thêm trạng thái draft chưa lưu, cảnh báo trước khi chuyển template/version hoặc rời trang và dialog xác nhận publish/archive.
- Giữ published version bất biến; không đổi route, API, database, permission hoặc workflow runtime.
- Cập nhật central/frontend/workflow mappings, chạy frontend build, lint và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- User đã duyệt review package và Mockup as Text bằng `OK triển khai`.
- Đã tải mandatory central/frontend/workflow mappings cùng frontend UI và workflow orchestration guardrails.
- Đã xác nhận thư viện/detail API hiện hữu cung cấp đủ template status/type, version status, field count, step usage count và instance count; không cần backend contract mới.
- Đã xác nhận owner chính là `frontend/src/modules/workflow-forms/WorkflowFormsPage.tsx`; các thay đổi khác trong worktree thuộc công việc hiện hữu và sẽ được giữ nguyên.
- Đã tái cấu trúc trang thành thư viện/workspace có thứ bậc rõ: desktop dùng thư viện sticky 320px với danh sách gọn, màn hình hẹp dùng bộ chọn template đã lọc; loading, lỗi, không có kết quả và chưa có version đều có trạng thái riêng.
- Đã Việt hóa nhãn thao tác/loại form/loại field, ưu tiên tên nghiệp vụ trước khóa kỹ thuật và hiển thị đúng field/step/instance metrics từ managed-template contract hiện hữu.
- Đã thay bảng published thành desktop table/mobile cards chỉ đọc; draft dùng field accordion, cấu hình JSON nâng cao thu gọn, inline validation và CTA theo trạng thái.
- Đã thêm dirty-state fingerprint, badge `Chưa lưu`, `beforeunload`, React Router blocker và cảnh báo trước khi chuyển template/version/tạo template hoặc rời route; editor khóa trong lúc mutation đang chạy.
- Đã thêm shared confirmation dialogs cho publish, archive template và archive version; publish nhắc rõ phiên bản trở thành bất biến. Đường metadata chỉ cho khôi phục template đã lưu trữ, không cho bypass archive confirmation.
- Đã giữ nguyên API, route, database, permission và runtime; không có backend/domain type/migration change.
- Đã cập nhật central/frontend/workflow mappings cho UI contract và published/draft invariant.
- Validation đạt: `npm run lint`, production `npm run build` và `git diff --check`. Vite chỉ báo lại cảnh báo chunk-size hiện có.
- Browser verification được bỏ qua đúng quy định vì người dùng không yêu cầu điều khiển browser trong turn này.

## Pipeline Template PDF Export

Status: complete

Mục đích chính:
- Cho phép người có `pipeline.view` tải tài liệu PDF mô tả chính xác revision Pipeline Builder đang chọn mà không thay đổi template hoặc runtime lịch sử.

Will be done:
- Thêm `GET /pipeline-templates/:id/pdf` chỉ đọc, dùng dữ liệu template-detail hiện hữu và bảo vệ bằng `pipeline.view`.
- Sinh PDF A4 ngang với font Unicode tiếng Việt nhúng trong backend; phần node gồm tên, mô tả, loại, phân quyền, role duyệt, yêu cầu ý kiến và cấm người gửi tự duyệt; phần step gồm tên, loại và role phụ trách theo từng node.
- Thêm nút `Xuất PDF` sau `Kiểm tra`, hỗ trợ loading/disabled, tải file tự động và toast thành công/lỗi; giữ toolbar responsive và published revision chỉ đọc.
- Không thêm migration, không lưu artifact và không ghi audit; export là snapshot tải trực tiếp tương tự báo cáo.
- Bổ sung backend/frontend regression checks, cập nhật central/backend/frontend/workflow mappings, chạy format/test/build và render PDF sang PNG để kiểm tra bố cục.
- Không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- User đã duyệt review package và Mockup as Text bằng `OK triển khai`.
- Đã tải mandatory central/backend/frontend/workflow mappings cùng impact/PDF guardrails và xác nhận owner files.
- Đã xác nhận worktree đang có thay đổi chưa commit ở các owner files; implementation sẽ chèn thay đổi có mục tiêu và không ghi đè công việc hiện hữu.
- Đã thêm protected `GET /pipeline-templates/:id/pdf`, `GetTemplatePDF`, service/repository lookup và generator A4 ngang nhúng Noto Sans theo SIL OFL; phiên bản `codeberg.org/go-pdf/fpdf v0.11.0` giữ tương thích module Go 1.23.
- Đã xuất đầy đủ metadata revision/status, bảng node với permission/approval policy và bảng checklist/step theo node; empty state, row wrapping, repeated header, node continuation và footer nhiều trang đều được xử lý.
- Đã thêm nút `Xuất PDF` sau `Kiểm tra`, tải bằng `downloadBlob`, đặt tên `workflow-<slug>-rev-<n>.pdf`, khóa khi template/loading đang chờ và hiển thị toast thành công/lỗi.
- Đã bổ sung unit test mapping/generation và frontend source regression check; `go test ./...`, `go mod verify`, `npm run check:pipeline-builder` và `git diff --check` đều pass.
- Đã render kiểm tra mẫu Unicode cuối với Poppler: A4 ngang, metadata đúng múi giờ, tiếng Việt/empty state đúng; stress test 13 node ngắt 4 trang đúng. Browser verification được bỏ qua đúng theo guardrail.
- Đã cập nhật central/backend/frontend/workflow mappings; không có migration vì export chỉ đọc, không lưu artifact và không ghi audit.

## Workflow Navigation Group

Status: complete

Mục đích chính:
- Tách công cụ cấu hình workflow khỏi nhóm tác nghiệp `Làm việc` và đưa Pipeline Builder cùng Workflow Forms vào một nhóm điều hướng dễ tìm.

Will be done:
- Xóa mục `Workflow` cũ khỏi `Làm việc`; thêm nhóm `Workflow` giữa `Làm việc` và `Quản trị`.
- Thêm `Pipeline builder` trỏ tới `/pipeline-builder` và `Workflow Forms` trỏ tới `/workflow-forms`, cùng giữ quyền `pipeline.view`.
- Giữ Pipeline Builder ở chế độ toàn màn hình và giữ responsive sidebar hiện tại; không đổi route, API, database hoặc workflow runtime.
- Cập nhật central/frontend mappings, chạy frontend build và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- User đã duyệt review package và Mockup as Text bằng `OK triển khai`.
- Đã tải mandatory central/frontend/workflow mappings và frontend UI guardrails.
- Đã xác nhận `/workflow-forms` đã có route với `pipeline.view`; thiếu sót chỉ nằm ở cấu hình sidebar `navGroups`.
- Đã xóa mục `Workflow` khỏi `Làm việc`, thêm nhóm `Workflow` giữa `Làm việc` và `Quản trị`, đồng thời thêm `Pipeline builder` và `Workflow Forms` với icon và active state hiện hữu.
- Đã giữ nguyên `pipeline.view`, cơ chế ẩn group rỗng, Pipeline Builder toàn màn hình và sidebar responsive hiện tại; không đổi route, API, database, runtime hoặc frontend domain contract.
- Đã cập nhật central/frontend mappings cho navigation mới. Workflow domain map không cần cập nhật vì không có invariant template/runtime/form nào thay đổi.
- Frontend production build đạt; Vite chỉ báo lại cảnh báo chunk-size hiện có.
- Browser verification được bỏ qua đúng quy định vì người dùng không yêu cầu điều khiển browser trong turn này.

## Published Connector Inspector Read-Only View

Status: complete

Mục đích chính:
- Cho phép mở xem đầy đủ cấu hình connector trên revision Pipeline Builder đã phát hành mà không mở đường ghi vào cấu trúc workflow.

Will be done:
- Render Connector Inspector khi connector được chọn ở draft hoặc published; published và người chỉ có quyền xem dùng mode read-only.
- Ẩn nút Lưu, khóa label/condition/config/priority và giữ source/target/edge metadata cùng trạng thái loading/error có thể đọc được.
- Giữ thay đổi port cùng node ở canvas tách biệt với cấu hình connector; không đổi backend, API, database, definition hash hoặc runtime copy.
- Bổ sung focused regression check, cập nhật central/frontend/workflow mappings và chạy Pipeline Builder check, lint, `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- User đã duyệt review package và Mockup as Text bằng `OK triển khai`.
- Đã tải mandatory central/frontend/root-cause/workflow mappings cùng frontend/diagnosis guardrails.
- Đã xác nhận connector selection và edge form hoạt động; root cause là điều kiện `!isReadOnly && selectedConnector` loại Inspector khỏi published, trong khi `EdgeInspectorPanel` chưa có read-only mode.
- Trước triển khai, source-level pass/fail signal thất bại đúng triệu chứng: `published connector selection is gated out by !isReadOnly`.
- Đã render Connector Inspector cho mọi connector được chọn; draft có `pipeline.edit` giữ mode chỉnh sửa, còn published/người chỉ xem nhận mode read-only với badge phù hợp.
- Đã ẩn Save, khóa label/condition/config/priority, thêm callback defense guards và giữ raw condition/form-field keys có thể đọc khi catalog phụ trợ thiếu hoặc lỗi.
- Đã bổ sung source-level regression check và cập nhật central/frontend/root-cause/workflow mappings; không cần backend, API contract, migration hoặc shared domain type change.
- Feedback loop đã chuyển sang PASS: `published connector selection can render the read-only inspector`.
- Validation đạt: `npm run check:pipeline-builder`, `npm run lint` và `git diff --check`. Vite chỉ báo lại cảnh báo chunk-size hiện có.
- Browser verification được bỏ qua đúng quy định vì người dùng không yêu cầu điều khiển browser trong turn này.

## Published Pipeline Builder Layout Editing

Status: complete

Mục đích chính:
- Cho phép người có `pipeline.edit` chỉnh bố cục của revision Pipeline Builder đã phát hành mà không thay đổi cấu trúc, điều kiện chuyển bước hoặc runtime lịch sử.

Will be done:
- Thêm API layout nguyên tử cho vị trí node và cạnh bám source/target của connector; endpoint chỉ nhận metadata trình bày, giữ nguyên node liên kết và ghi audit.
- Tách UI thành ba mode `full`/`layout`/`view`: published cho kéo node, tự sắp xếp vị trí và đổi port trên đúng node hiện tại; mọi structural edit vẫn khóa.
- Giữ `order_index`, `from_node_id`, `to_node_id`, điều kiện, label, step/form và `definition_hash` đã phát hành không đổi; metadata bố cục không tham gia definition hash mới.
- Bổ sung focused backend/frontend regression checks; cập nhật central/backend/frontend/workflow mappings; chạy backend tests, Pipeline Builder check và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- User đã duyệt review package và Mockup as Text bằng `OK triển khai`.
- Đã tải mandatory central/backend/frontend/workflow mappings cùng backend/frontend/impact guardrails.
- Đã xác nhận runtime không snapshot vị trí node và edge condition engine bỏ qua `condition_config.diagram`; thay đổi được giới hạn an toàn ở lớp template presentation.
- Đã xác nhận code hiện tại khóa published bằng một `isReadOnly` chung ở frontend và `ensureDraftTemplate` chung ở backend, khiến node kéo tạm thời nhưng không được lưu và endpoint connector bị khóa hoàn toàn.
- Đã thêm `PUT /pipeline-templates/:id/layout` với `pipeline.edit`, validation, transaction và audit; payload chỉ nhận tọa độ node cùng source/target side, còn endpoint node được lấy từ connector hiện hữu.
- Đã loại tọa độ node và `condition_config.diagram` khỏi definition hash mới nhưng vẫn giữ các business condition keys trong hash.
- Đã tách Pipeline Builder thành `full`/`layout`/`view`; published có quyền edit layout cho kéo node, Auto Layout nguyên tử và reconnect port trên đúng endpoint node, trong khi toàn bộ structural controls vẫn khóa.
- Đã bổ sung backend unit tests và source-level Pipeline Builder regression checks cho layout contract.
- Đã cập nhật central/backend/frontend/workflow mappings; không cần migration vì tái sử dụng `position_x`, `position_y` và `condition_config.diagram` hiện hữu.
- Validation đạt: `go test ./...`, `npm run check:pipeline-builder`, `npm run lint` và `git diff --check`. Vite chỉ báo lại cảnh báo chunk-size hiện có.
- Browser verification được bỏ qua đúng quy định vì người dùng không yêu cầu điều khiển browser trong turn này.

## Pipeline Builder Node Type Vietnamese Labels

Status: complete

Mục đích chính:
- Hiển thị nhãn nghiệp vụ tiếng Việt cho toàn bộ node type chính thức trong Pipeline Builder, đặc biệt `contract`, `construction_planning` và `incoming_acceptance`, trong khi vẫn giữ nguyên enum kỹ thuật ở API/database.

Will be done:
- Dùng một danh sách node type dùng chung cho Node Inspector và regression test; bổ sung ba nhãn đã duyệt cùng màu chip tương ứng.
- Không đổi API, database, migration, template/runtime state hoặc kích thước/bố cục node card.
- Bổ sung focused presentation test; cập nhật central/frontend/root-cause/workflow mappings; chạy frontend test, lint, build, pipeline-builder check và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- User đã duyệt review package với mapping `contract` → `Hợp đồng`, `construction_planning` → `Kế hoạch thi công`, `incoming_acceptance` → `Nghiệm thu đầu vào`.
- Đã tải central/frontend/root-cause/workflow mappings và frontend/diagnose/workflow guardrails.
- Đã tái hiện bằng feedback loop: `enumLabel('node_type', value)` trả nguyên ba enum trên, trong khi `construction` và `acceptance` đã trả nhãn tiếng Việt.
- Root cause đã xác nhận: `NodeCard` dùng đúng shared `EnumChip`, nhưng `status.ts`, tone map và danh sách Node Inspector đều thiếu ba node type chính thức.
- Đã thêm `nodeTypeValues` làm danh sách dùng chung, bổ sung ba nhãn tiếng Việt và tone chip; Node Inspector dùng trực tiếp cùng danh sách nên thẻ node và trường chọn không còn lệch nhau.
- Đã giữ nguyên API/database enum và raw fallback cho node type tương lai; không đổi layout, template/runtime state hoặc backend contract.
- Đã thêm focused test cho ba mapping, shared Inspector options và fallback; tích hợp test vào `check:pipeline-builder`.
- Đã cập nhật central/frontend/root-cause/workflow mappings cho presentation contract và đường chẩn đoán raw enum. Backend mapping không cần cập nhật vì không có route, handler, service, repository hoặc API contract nào thay đổi.
- Validation đạt: focused node-type tests `3/3`, `npm run lint`, production build và Pipeline Builder diagram regression check. Vite chỉ báo lại cảnh báo chunk-size hiện có; `git diff --check` sạch.
- Browser verification được bỏ qua vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

## Official Workflow “Đầu Vào” Terminology

Status: complete

Mục đích chính:
- Dùng `đầu vào` làm tên gọi chung cho tổ đội, thầu phụ và nhà cung cấp vật tư trong các nhãn tổng hợp của workflow `40000000-0000-0000-0000-000000000021`, đồng thời giữ tên đối tượng cụ thể tại các nghiệp vụ cần phân biệt.

Will be done:
- Thêm migration additive V69; không sửa V68 đã áp dụng và không thay đổi checksum migration lịch sử.
- Rút gọn ba checklist tổng hợp, tên hai form template, mô tả/điều kiện thoát của node `Đầu Vào` và nhãn cạnh chuyển bước theo wording đã duyệt.
- Đồng bộ các snapshot hiển thị của revision 2 hiện có mà không đổi role, trạng thái, form schema/answer, approval, audit hoặc PDF.
- Đồng bộ seed, regression tests và central/backend/workflow mappings; chạy migration check, backend tests và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu điều khiển browser trong turn triển khai.

Done:
- User xác nhận `tổ đội, thầu phụ và nhà cung cấp vật tư` được gọi chung là `đầu vào` và đã duyệt review package triển khai.
- Đã tải mandatory central/backend/workflow mappings cùng backend/custom migration guardrails; xác nhận Pipeline Builder lấy wording từ template/backend data, không cần đổi frontend component hoặc API contract.
- Đã xác nhận các nhãn nghiệp vụ cần giữ đối tượng cụ thể: đàm phán thầu phụ, nghiệm thu thầu phụ/tổ đội, nghiệm thu vật tư, hồ sơ/thanh toán/hóa đơn thầu phụ.
- Đã thêm additive V69, giữ nguyên checksum V68: chuẩn hóa ba checklist tổng hợp, hai form-template title, mô tả/exit text của node `Đầu Vào`, edge label và các snapshot hiển thị revision 2 hiện có; không đổi role, assignment, trạng thái, form schema/answer, approval, audit hoặc PDF.
- Đã đồng bộ resettable seed cho revision-1 source steps/node/edge; V66 clone và V69 tiếp tục đảm bảo revision 2 nhận cùng wording trên fresh database.
- Đã bổ sung regression assertions cho wording tổng hợp, wording đối tượng cụ thể cần giữ, node/edge và form template; cập nhật central/backend/workflow mappings. Không cần cập nhật frontend/admin mapping cho V69 vì không có UI/API/RBAC behavior nào thay đổi.
- Validation đạt: migration checker nhận đủ 69 migration, clean PostgreSQL chạy V1–V69, focused DB regression test pass, seed chạy thành công và trả đúng `3` step + `1` node + `1` edge, full backend test suite pass, `gofmt` và `git diff --check` sạch.
- Đã áp dụng V69 vào database local đang phục vụ UI (`1` applied, `68` skipped) và đọc lại đúng ba step, node/exit, edge cùng hai form-template title. Database và Docker images kiểm thử tạm đã được xóa.
- Browser verification được bỏ qua vì người dùng không yêu cầu điều khiển browser; thay vào đó đã xác minh trực tiếp dữ liệu mà Pipeline Builder đọc.

## Official Workflow Revision 2 Wording And Responsibility Roles

Status: complete

Mục đích chính:
- Làm tên node/checklist của workflow `40000000-0000-0000-0000-000000000021` ngắn, hành động-trước; tách vai trò/phụ trách khỏi title mà vẫn giữ đúng quyền thao tác work item.

Will be done:
- Đổi node `Đầu Vào: Tổ Đội / Thầu Phụ / Cung cấp vật tư` thành `Đầu Vào` và chuẩn hóa đủ 53 step đã duyệt, chuyển chi tiết note cần thiết xuống description.
- Thêm các role `Thiết kế`, `Sale`, `Pháp lý`; ánh xạ wording `Đội kỹ thuật` sang role hiện có `Đội thi công`; cấp đúng các permission tối thiểu cần cho các step được gán.
- Dùng migration additive V68 để cập nhật template revision 2 và các snapshot hiển thị liên quan mà không đổi trạng thái, form answer, approval, audit hay PDF.
- Đồng bộ seed, regression tests và central/backend/workflow/admin mappings; chạy migration check, backend tests, seed validation phù hợp và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu trong turn triển khai.

Done:
- User approved the wording review package, then explicitly approved creating `Thiết kế`, `Sale`, `Pháp lý` and mapping `Đội kỹ thuật` to `Đội thi công`.
- Loaded mandatory central/backend/workflow/admin mappings plus backend and custom database migration guardrails.
- Confirmed V66 owns revision 2 with 13 nodes and 53 steps; step roles are copied into runtime steps, work items, assignments and form instances, so role updates must preserve actionability.
- Added additive V68 with all 53 approved action-first titles, node `Đầu Vào`, structured template responsibility roles and display-snapshot wording updates for existing revision-2 runs, work items and form instances. Existing runtime role/assignment snapshots, statuses, answers, approvals, audit and PDF artifacts remain unchanged.
- Added system roles `Thiết kế`, `Sale`, `Pháp lý` with self-contained minimum permission keys/grants; mapped the reviewed technical responsibility to `Đội thi công` and added its pipeline/planning permissions without auto-assigning users or project members.
- Synchronized the resettable base seed for all 52 revision-1 source steps, the input node, contract permissions and role grants so a later V66/V68 migration path and a fresh local seed present the same wording model.
- Added DB regression coverage for revision 2: published 13-node/53-step shape, zero note/role punctuation in titles, representative exact title/role mappings, new roles and their minimum permission grants.
- Updated central/backend/workflow/admin mappings for V68 ownership, action-first wording, responsibility-role handling and the runtime-role preservation invariant. No frontend mapping update was required because no UI route, component, client contract or presentation helper changed.
- Validation passed: migration checker (`68` migrations), clean PostgreSQL V1–V68 execution, focused DB regression test, full backend test suite, disposable seed execution (`52` source steps, `0` unclean titles, `3` new roles, `Đội thi công` has `planning.edit`) and `git diff --check`. All disposable databases/image were removed; the main local database was not migrated or reseeded.
- Browser verification was skipped because the user did not explicitly request it and this change has no frontend code.

## Handover Linked Checklist Source Viewing

Status: complete

Mục đích chính:
- Cho phép người dùng mở và kiểm tra nguồn tài liệu/minh chứng đã liên kết của từng checklist Bàn giao, kể cả khi hồ sơ đã bàn giao và bị khóa.

Will be done:
- Bổ sung metadata nguồn chính xác vào từng checklist item trong Handover detail mà không đổi route, schema, trạng thái, readiness hoặc audit.
- Thay nhãn tĩnh `Đã liên kết` bằng `Xem nguồn`; mở dialog đọc-only có metadata nguồn và hành động mở tệp/URL khi khả dụng.
- Giữ quyền tải minh chứng qua `/evidence/:id/file`; chỉ cho cập nhật liên kết khi hồ sơ chưa khóa và người dùng có quyền xác minh.
- Cảnh báo rõ khi URL demo/nguồn không có tệp mở được; không tạo PDF giả hoặc thay seed trong scope này.
- Bổ sung regression tests, cập nhật central/backend/frontend/root-cause/handover mappings, chạy backend/frontend validation và `git diff --check`.

Done:
- User approved all three review points with `OK triển khai`.
- Loaded mandatory central/backend/frontend/root-cause/handover mappings plus backend/frontend/diagnose/impact guardrails.
- Reproduced the bug with a stable source-level signal: linked rows render static `Đã liên kết`; automatic rows alone receive `Xem nguồn`.
- Confirmed `HO-GOLD-001` has four linked active documents with source IDs/names and placeholder `example.com` URLs; backend data linkage is intact.
- Confirmed the existing protected evidence file endpoint remains the authorization owner for local evidence files.
- Enriched each Handover checklist item with its exact linked evidence metadata and document metadata/file URL; the detail contract no longer depends on the capped source-option lists.
- Replaced the locked/static linked-source action with `Xem nguồn` on desktop and mobile, including handed-over hồ sơ. The dialog shows source type/status, verification notes/time and opens only active safe document URLs or protected local evidence files.
- Added explicit warnings for placeholder `example.com`, inactive documents and sources without an openable file. Mutable hồ sơ with `handover.verify` can continue from source inspection to the shared `Cập nhật liên kết` flow; locked hồ sơ remain read-only.
- Added frontend regression coverage for linked document/evidence presentation and backend integration assertions for exact evidence/document metadata. Updated central, backend, frontend, root-cause and quality/handover mappings.
- Validation passed: handover source tests 7/7, project navigation tests 2/2, frontend lint/build, backend test suite and `git diff --check`. Vite reported only the existing large-chunk advisory.
- Browser verification was skipped because the user did not explicitly request it.

## Handover Checklist Item Vietnamese Labels

Status: complete

Mục đích chính:
- Hiển thị loại checklist Bàn giao bằng nhãn nghiệp vụ tiếng Việt nhất quán thay cho các mã kỹ thuật như `operation_manuals`, đồng thời vẫn giữ mã gốc trong tooltip để hỗ trợ kỹ thuật.

Will be done:
- Tạo helper ánh xạ đủ 8 `item_code` hiện hành sang nhãn tiếng Việt đã được duyệt, với fallback `Nội dung khác` cho mã chưa biết.
- Áp dụng helper tại bảng desktop, thẻ mobile và dialog nguồn; không thay đổi API, database, readiness hoặc trạng thái checklist.
- Bổ sung regression test cho đủ 8 mapping và fallback; cập nhật central/frontend/handover mappings; chạy focused tests, frontend build và `git diff --check`.

Done:
- User approved the exact label mapping and tooltip-only technical code behavior with `OK triển khai`.
- Loaded mandatory central/frontend mappings plus frontend workflow UI expectations and quality/acceptance/handover guardrails.
- Confirmed the backend currently creates exactly eight handover checklist item codes and the frontend directly renders the raw code in desktop, mobile and source-dialog surfaces.
- Added one shared presentation mapping for all eight current checklist codes, with `Nội dung khác` as the safe label for an unknown future code and the original code retained only in the `Mã kỹ thuật` tooltip.
- Applied the Vietnamese business label consistently to the desktop checklist table, responsive mobile cards and automatic-source detail dialog without changing the primary checklist title, API/database values, readiness checks or verification state.
- Added regression coverage for all eight exact mappings, fallback behavior and technical tooltip; updated central/frontend/handover mappings for the presentation invariant. Backend mapping did not require an update because no route, contract, repository or readiness behavior changed.
- Validation passed: `npm run test:handover-source` (5/5), `npm run test:project-navigation` (2/2), `npm run lint`, `npm run build` and `git diff --check`. Vite reported only the existing large-chunk advisory.
- Browser verification was skipped because the user did not explicitly request it.

## Handover Automatic Checklist Source Transparency

Status: complete

Mục đích chính:
- Cho phép người dùng mở và hiểu nguồn của từng checklist Bàn giao được hệ thống xác minh tự động: module nguồn, tiêu chí, kết quả snapshot và thời điểm kiểm tra; thao tác xem không được thay đổi readiness hay dữ liệu nguồn.

Will be done:
- Thay nhãn tĩnh `Tự động` bằng hành động `Xem nguồn` cho nghiệm thu cuối, kho/vật tư dư, công nợ phải trả và công nợ phải thu.
- Thêm dialog đọc-only dùng dữ liệu checklist/readiness hiện có, phân biệt snapshot lịch sử với dữ liệu hiện tại và chỉ hiện CTA mở module khi người dùng có quyền.
- Deep-link nghiệm thu đúng bản ghi, Công nợ đúng Phải trả/Phải thu và Kho đúng tab `Tại công trình & dư`; giữ nguyên xác minh thủ công, readiness, ledger và trạng thái bàn giao.
- Tách helper ánh xạ nguồn có regression test; cập nhật central/frontend/handover/accounting mappings; chạy frontend tests/build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved implementation with `OK triển khai` after reviewing the source dialog mockup and aggregate-snapshot limitation.
- Loaded mandatory central/backend/frontend maps plus frontend workflow, impact-analysis, quality/handover, accounting/finance and materials/warehouse guardrails.
- Confirmed the backend detail already returns checklist source IDs and readiness aggregate counts; no API, schema or migration change is required for the approved scope.
- Replaced the passive automatic label with a responsive `Xem nguồn` action for the exact final acceptance, site surplus, payable and receivable readiness checks; evidence/document-linked items remain user-linked manual verification sources.
- Added a read-only, permission-aware source dialog showing the backend criterion, stored result/count/amount/check time and a clear warning that the source module may have changed since the last readiness snapshot.
- Added presentation-only deep links that select the exact Acceptance record, the Warehouse site/surplus tab and the permitted Debt payables/receivables kind with the `all` queue; no source or financial mutation is triggered by navigation.
- Added a pure source-presentation helper and four regression tests covering classification, linked documents/evidence, all source targets and snapshot metrics/result states.
- Updated central/frontend/handover/accounting mappings for source transparency and query-driven navigation. Backend and warehouse mappings did not require changes because API/readiness queries and Warehouse tab behavior were unchanged.
- Validation passed: `npm run test:handover-source`, `npm run test:project-navigation`, `npm run build` and `git diff --check`. Vite reported only the existing large-chunk advisory.
- Browser verification was skipped because the user did not explicitly request it.

## Handover List-First Workspace And Accounting Debt Label

Status: complete

Mục đích chính:
- Cho phép người dùng tìm và chọn hồ sơ bàn giao trước khi đọc chi tiết dài; dùng nhãn điều hướng Công nợ ngắn gọn, đúng ngữ cảnh trong chi tiết phiếu Kế toán.

Will be done:
- Đưa bộ lọc lịch sử và `Danh sách hồ sơ` lên trên workspace chi tiết Bàn giao.
- Giữ hành vi `Xem chi tiết` chọn đúng hồ sơ rồi cuộn/focus workspace inline bên dưới danh sách.
- Đổi nhãn `Mở công nợ nguồn` thành `Mở công nợ`, giữ nguyên route và quan hệ phân bổ.
- Cập nhật central/frontend/accounting/handover mappings; chạy frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the combined UI implementation with `Duyệt, triển khai`.
- Loaded mandatory central/frontend maps plus frontend workflow, accounting-finance and quality-handover guardrails.
- Confirmed the Handover detail currently renders before its history/list and the existing `openHandoverDetail` already selects then scrolls/focuses the inline workspace.
- Confirmed `Mở công nợ nguồn` has one frontend owner in `AccountingDetailDialog.tsx`; its target remains the project Debt route.
- Moved the Handover status/history filters and `Danh sách hồ sơ` above the selected inline detail, with explicit spacing before the selector/detail block.
- Preserved the shared `openHandoverDetail` path so history `Xem chi tiết` and the multi-record selector still select and scroll/focus the workspace now located below the list.
- Changed the linked Accounting payment action from `Mở công nợ nguồn` to `Mở công nợ` without changing its project Debt route or any allocation/reconciliation behavior.
- Updated central/frontend/accounting/handover mappings for the list-first handover hierarchy and concise Debt navigation label.
- Validation passed: `cd frontend && npm run build` and `git diff --check`. Vite reported only the existing large-chunk advisory.
- Browser verification was skipped because the user did not explicitly request it.

## Handover Detail Navigation And Vietnamese Terms

Status: complete

Mục đích chính:
- Khi người dùng chọn `Xem chi tiết`, luôn đưa đúng hồ sơ bàn giao vào tầm nhìn; toàn bộ bề mặt liên quan dùng thuật ngữ nghiệp vụ tiếng Việt rõ ràng thay cho `Gói` và `AP/AR`.

Will be done:
- Sửa hành vi `Xem chi tiết` trong lịch sử bàn giao để chọn đúng hồ sơ rồi cuộn/focus vùng chi tiết, kể cả khi hồ sơ đó đang được chọn.
- Đổi thuật ngữ người dùng từ `Gói` sang `Hồ sơ bàn giao` trên trang, dialog tạo, workspace chi tiết, work-item surface và thông báo/audit phát sinh mới.
- Đổi `AP`, `AR`, `AP/AR` trên các bề mặt người dùng liên quan thành `công nợ phải trả`, `công nợ phải thu`, `công nợ phải trả và phải thu`; giữ nguyên API, key kỹ thuật và nghiệp vụ đối soát.
- Cập nhật Golden seed và bản ghi local `HO-GOLD-001` có mục tiêu, không viết lại ghi chú do người dùng tạo.
- Cập nhật central/frontend/accounting/handover mappings; chạy frontend build, focused backend test phù hợp và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the combined implementation plan with `OK duyệt`.
- Loaded mandatory central/backend/frontend/root-cause maps plus frontend/backend workflow, accounting-finance and quality-handover guardrails.
- Confirmed the detail API already succeeds; the row action only reassigned the already-selected ID and had no scroll/focus effect. Confirmed the visible `AP/AR` note for `HO-GOLD-001` comes from `backend/seed/golden_project_finance.sql` while additional user guidance is owned by Accounting and `WorkItemPrimarySurface`.
- Added one `openHandoverDetail` path for the history row and multi-record selector. It always selects the requested record and scrolls/focuses the inline detail section, including when the same record is already active; loading, error and loaded content share that focus target.
- Replaced user-facing `Gói` terminology with `Hồ sơ bàn giao` across list metrics/history, create dialog, inline detail, work-item guidance, generated checklist/blocker text and new audit descriptions without changing routes, statuses, API fields or package lifecycle functions.
- Replaced user-facing AP/AR/đầu-vào/đầu-ra wording with `công nợ phải trả` and `công nợ phải thu` across Accounting, Debt reconciliation, shared finance/handover work-item surfaces, backend automation/blockers/audit and the Golden finance seed. Internal AP/AR keys and reconciliation behavior remain unchanged.
- Updated the running local `HO-GOLD-001` note plus its two finance checklist items and Golden closure audit through targeted SQL; verified the three displayed values directly and did not reseed or rewrite other/user-authored notes.
- Updated central/backend/frontend/root-cause/accounting/handover mappings for the navigation invariant, Vietnamese terminology convention and generated message ownership.
- Validation passed: `cd frontend && npm run build`, `cd backend && make test`, targeted local SQL verification and `git diff --check`. Vite reported only the existing large-chunk advisory.
- `cd backend && make seed-validate` was attempted but the current local database has zero of the two Golden projects in the validator's expected completed/100% official-template state. This fixture drift predates and is independent of the targeted text updates; no destructive reseed/reset was performed.
- Browser verification was skipped because the user did not explicitly request it.

## Project Explorer Navigation

Status: complete

Mục đích chính:
- Mỗi nhóm trong cây Công trình mở đúng workspace nghiệp vụ đầu tiên mà người dùng có quyền xem, thay vì tất cả cùng quay về Tổng quan.

Will be done:
- Dùng chung một cấu hình nhóm/mục con/quyền/route giữa `ProjectsPage` và `ProjectWorkspaceLayout`.
- Route mặc định theo nhóm phải khớp cây nghiệp vụ hiện hữu và tự fallback theo quyền để không dẫn người dùng vào trang 403.
- Thêm regression test cho Admin và vai trò có quyền hạn chế; cập nhật central/frontend/project-planning mappings.
- Chạy focused navigation test, frontend production build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved implementation with `OK triển khai`.
- Loaded mandatory central/frontend/root-cause/project-planning mappings and confirmed the bug is frontend-only: all six explorer folders used an empty path even though the nested project routes already exist.
- Built the failing source-level signal showing five non-overview groups resolve to the project index.
- Added `projectWorkspaceNavigation.ts` as the single group/child/permission/route source for both the expanded project explorer and project workspace sidebar.
- Changed group landing behavior to the first child visible under the current permission set, with a group-level fallback; Admin routes now open Overview, Pipeline, Contracts, Operations, Handover and Members respectively.
- Added dependency-free Node regression coverage for the Admin mapping and a restricted role that must fall back to BOQ, Warehouse, Acceptance and Debt.
- Updated central/frontend/root-cause/project-planning mappings for the shared owner, landing invariant and triage search path.
- Validation passed: focused navigation tests, frontend production build and `git diff --check`. Vite reported only the existing large-chunk advisory.
- Browser verification was skipped because the user did not explicitly request it.

## Create And Detail Redesign For Debt, Accounting And Handover

Status: complete

Mục đích chính:
- Giúp người dùng tạo đúng chứng từ ngay lần đầu và khi mở chi tiết luôn thấy trạng thái hiện tại, số liệu quan trọng, việc tiếp theo và nguồn nghiệp vụ liên quan; giữ Công nợ sở hữu AP/AR allocation, Kế toán sở hữu budget/generic phiếu và Bàn giao chỉ điều phối readiness.

Will be done:
- Thiết kế lại create Công nợ thành dialog có switch Phải trả/Phải thu, form theo nhóm và live summary; detail có header hành động theo trạng thái, tab có số đếm và tổng quan cân đối nghĩa vụ.
- Thiết kế lại create Kế toán thành dialog có switch Ngân sách/Phiếu thu chi khác; detail riêng cho ngân sách và phiếu, chỉ dùng generic `outbound|inbound|other` và dẫn AP/AR về Công nợ.
- Thiết kế lại create Bàn giao với preview nghiệm thu cuối, đại diện hai bên và summary sau tạo; detail giữ inline workspace, tách Việc cần làm/Checklist/Thông tin hồ sơ và một primary CTA theo trạng thái.
- Giữ backend/API/schema hiện hữu; chỉ thêm UI cho transition đã tồn tại và không tạo nguồn dữ liệu mới.
- Cập nhật central/frontend/accounting/handover mappings; chạy frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `Approve`.
- Loaded mandatory central/backend/frontend/root-cause maps plus frontend workflow, accounting-finance, quality-handover and impact-analysis guardrails.
- Confirmed typed backend contracts already cover all approved create/detail actions; no migration or API contract change is required.
- Extended the shared `SimpleForm` contract with optional grouped sections, a responsive sticky live-summary aside and linked field derivation while preserving all existing flat-form consumers and inline validation/toasts.
- Replaced Công nợ create/detail with dedicated dialogs: permission-aware Phải trả/Phải thu switching, locked project context, obligation summaries, issue-aware default tabs, tab counters and one approve/reconcile/pay-or-receive next action that continues to use canonical allocation panels.
- Replaced Kế toán create/detail with dedicated budget/generic-phiếu dialogs: server-owned budget values remain read-only/derived, generic creation remains limited to `outbound|inbound|other`, and AP/AR-linked work is directed back to Công nợ.
- Reworked Bàn giao create/detail with final-acceptance source preview and representative prefill, plus a responsive three-tab inline workspace for `Việc cần làm`, `Checklist` and `Thông tin hồ sơ` with one state-derived primary action.
- Updated central/frontend/accounting/handover mappings. Backend and root-cause mappings did not change because no route, API contract, service behavior, schema, diagnostic path or failure mode changed.
- Validation passed: frontend TypeScript lint, frontend production build and `git diff --check`. Vite reported only the existing large-chunk advisory.
- Browser verification was skipped because the user did not explicitly request it.

## P0 + P1 Bàn giao, Kế toán và Công nợ

Status: complete

Mục đích chính:
- Biến ba màn hình tài chính cuối dự án thành các workspace hành động rõ ràng: Công nợ xử lý AP/AR và đối soát, Kế toán xử lý ngân sách/phiếu/dòng tiền, Bàn giao điều phối readiness và dẫn người dùng về đúng nguồn đang chặn.

Will be done:
- P0: khóa project scope của receivables bằng regression test; server sở hữu trạng thái/số dư ngân sách mới; generic payment không thay thế allocation AP/AR; đồng bộ permission affordance và đủ bucket 61–90 ngày.
- P1: Công nợ hiển thị một tab AP hoặc AR toàn chiều rộng, hàng đợi ưu tiên và workspace chi tiết theo tab nghiệp vụ.
- P1: Kế toán tách Tổng quan/Ngân sách/Phiếu thu chi/Dòng tiền, dùng CTA theo việc cần xử lý và giữ report/export cùng query semantics.
- P1: Bàn giao hiển thị package đang chọn trực tiếp, ưu tiên next action/blocker và liên kết về module nguồn; không thêm ràng buộc một package active ở P1.
- Cập nhật central/backend/frontend/root-cause/accounting/handover/reporting mappings; không thêm migration hoặc thay đổi P2.
- Chạy Go format/tests, frontend production build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved implementation with `Duyệt, triển khai P0 + P1 theo review package này`.
- Loaded mandatory central/backend/frontend/root-cause maps plus backend/frontend workflow, accounting-finance, handover-quality, warehouse, reporting and impact guardrails.
- Confirmed existing dirty Acceptance/V67/branding changes are outside this plan; overlapping shared Go/map/state files will be edited only in targeted sections and preserved.
- Fixed `ListReceivables` so validated `project_id` is combined with actor project visibility exactly like Payables. Strengthened the DB regression by making the actor a member of both fixture projects, so the old missing filter now fails deterministically.
- Added typed `BudgetCreateRequest` and `PaymentCreateRequest`. New budgets ignore client-derived state, start draft with zero approved/used/available balance, and approval derives approved/remaining values from planned/used. Generic `/payments` now accepts only `outbound|inbound|other`; AP/AR types must use dedicated allocation endpoints.
- Rebuilt Công nợ as one full-width Phải trả/Phải thu workspace with action queues, project-aware columns, complete 61–90 aging, shared create dialogs, permission-aware nested actions and five detail tabs for overview, invoices, schedule/advance, payments and reconciliation.
- Rebuilt Kế toán into Tổng quan/Ngân sách/Phiếu thu chi/Dòng tiền tabs. The overview prioritizes pending approvals, budget forms expose only client-owned inputs, generic phiếu forms explain AP/AR ownership, and Cashflow surfaces pending amount/unclassified/non-cash warnings while preserving report/export query parity.
- Rebuilt Bàn giao as an inline selected-package readiness workspace with state-derived next action, grouped checklist, source-module links for each blocker and secondary package history. P1 adds no one-active-package constraint and no source mutation.
- Updated central/backend/frontend/root-cause plus accounting, handover and reporting mappings. Warehouse behavior was read as a downstream invariant but its mapping did not change because blocker links are navigation-only.
- Validation passed: `cd backend && make test`, `cd backend && make migrate-check` (67 migrations), `cd frontend && npm run build`, and `git diff --check`. Vite reported only the existing large-chunk advisory.
- The focused DB-backed finance tests compile and are present, but the test harness skipped execution because `CONSFLOW_TEST_DATABASE_URL` is not configured; the Docker daemon was unavailable from the sandbox, so no application database was touched.
- Browser verification was skipped because the user did not explicitly request it.

## Planning To Construction Handoff

Status: complete

Mục đích chính:
- Bàn giao một lần các hạng mục giám sát đã được chuẩn bị trong Planning sang hàng đợi `Kiểm tra` để Giám sát viên chỉ thực hiện kiểm tra, xử lý/khắc phục và báo cáo tại hiện trường.

Will be done:
- Thêm contract `construction_handoffs`/`construction_handoff_items` với snapshot nguồn, actor/time audit, trạng thái và khóa idempotency; mỗi hạng mục được kích hoạt đúng một work item và một canonical construction check.
- Tạo API đọc preview, lưu/activate handoff atomic và hỗ trợ runtime hiện hữu mà không viết lại lịch sử; nguồn kế hoạch và check được liên kết hai chiều.
- Tách planning/pre-construction với execution trong official workflow revision mới; không mutate published template hoặc runtime copy lịch sử.
- Đồng bộ check/action lifecycle với generated work item và chặn rời node Thi công khi check/action từ bàn giao chưa hoàn tất.
- Thêm Planning handoff panel/dialog, trạng thái readiness và một CTA bàn giao; Thi công hiển thị nguồn `Từ kế hoạch` nhưng giữ manual check cho phát sinh ngoài kế hoạch.
- Cập nhật Golden fixtures/tests và central/backend/frontend/root-cause/workflow/planning/construction mappings; chạy backend format/tests/migration/seed validation, frontend build và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the cross-domain review package and Mockup as Text with `OK triển khai theo review package này`.
- Loaded mandatory central/backend/frontend/root-cause maps, workflow/planning/construction domain guardrails, impact analysis, backend/frontend workflow and custom database migration instructions.
- Confirmed PostgreSQL through `database/sql`, custom checksum migration runner and a high-risk schema/transaction/API/UI change owned by `repository_domain.go`, service/handler/routes, workflow runtime, Planning and Operations.
- Confirmed existing dirty files belong to the previous approved dialog-width change and will be preserved.
- Added V66 with `construction_handoffs`/`construction_handoff_items`, snapshot hash/revision/actor audit, generated record links, `construction.handoff.manage`, the dedicated `client_construction_handoff_plan` form and published official workflow revision 2.
- Revision 2 preserves the immutable revision-1 runtime and separates node 9 `Kế hoạch thi công` from node 10 `Thi công`; future project runs use revision 2 while active/historical runs keep their selected template snapshot.
- Added typed read/activate routes and focused `construction_handoff.go` service/repository ownership. Preview prefers the dedicated submitted form and supports current revision-1 QA/assignment form imports; activation atomically/idempotently creates one handoff item, target-node work item and canonical check per source row. The client sends its previewed form revision and backend locks/rechecks it to reject stale-tab activation. Same-form-revision replay returns the existing handoff; a newer form revision replaces only unstarted rows and is rejected after work/evidence/action begins.
- Added `CONSTRUCTION_HANDOFF_REQUIRED` to the engine-v2 planning completion gate. Generated work stays pending until the normal construction node advance; a generated check cannot run early.
- Synchronized generated work lifecycle: check pass completes it, check fail blocks it and creates the canonical action, verification pass completes it, and verification fail keeps it blocked. Planning-generated checks are read-only snapshots; manual checks remain available for unplanned conditions.
- Added `ConstructionHandoffPanel` to Planning with readiness metrics, one bulk CTA, report-width inline editor, participant assignment, inline validation, dismissible toasts and read-only revision view. Operations lists/detail show `Từ kế hoạch · rev n` and preserve the shared perform/remediation dialogs.
- Added DB regression coverage for the planning completion blocker, atomic two-item generation, idempotent replay, guarded form-revision supersession and pass/fail/resolution/verification synchronization.
- Updated central/backend/frontend/root-cause plus planning/workflow/construction domain mappings. The completed Golden historical fixture was intentionally not rewritten to revision 2; disposable DB coverage validates the new revision while preserving the fixture's historical runtime contract.
- Validation passed: frontend production build; backend package tests; all 66 migrations on a fresh disposable PostgreSQL database; revision-2 schema check (`13` nodes, `53` steps, ordered `Kế hoạch thi công -> Thi công -> Nghiệm thu Đầu Vào`); focused DB regression; `git diff --check`.
- Browser verification was skipped because the user did not explicitly request it.

## Information-Dense Dialog Widths

Status: complete

Mục đích chính:
- Cho các dialog chứa nhiều biểu mẫu, bảng, tiêu chí, minh chứng hoặc panel liên quan tận dụng tốt chiều rộng desktop trong khi dialog xác nhận đơn giản vẫn gọn và mobile vẫn responsive.

Will be done:
- Giữ `sm`/`md`; mở rộng semantic sizes dùng chung thành `lg` 800px, `xl` 1120px, `report` 1320px và `workspace` 1480px, luôn giới hạn theo viewport.
- Chuyển form Tạo/Sửa Kiểm tra từ `lg` sang `xl`; giữ Báo cáo ở `report` và các workspace giàu thông tin ở tier hiện hữu để tự nhận chiều rộng mới.
- Không đổi API, validation, trạng thái, permission, footer CTA hoặc responsive single-column behavior.
- Cập nhật central/frontend mappings; chạy frontend build, rà soát Dialog usages và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `Approve`.
- Reloaded the mandatory central/frontend mappings, frontend workflow skill and shared Dialog source before implementation.
- Confirmed the root cause is the legacy fixed maxima (672/896/1080/1280px) remaining after information-heavy dialogs adopted multi-column layouts.
- Expanded the shared responsive tiers to `lg` 800px, `xl` 1120px, `report` 1320px and `workspace` 1480px while preserving the existing viewport cap and compact `sm`/`md` tiers.
- Promoted the information-heavy Tạo/Sửa Kiểm tra form from `lg` to `xl`; report create/edit/view and existing rich workspaces automatically receive their approved wider tiers.
- Updated the central and frontend UI maps with the semantic size contract; no backend, API, permission, validation or business-domain mapping changed.
- Validation passed: all explicit Dialog usages were inventoried, the four widths were emitted in the production CSS, frontend production build and `git diff --check` passed.
- Browser verification was skipped because the user did not explicitly request it.

## One-Flow Daily Report Field Actions

Status: complete

Mục đích chính:
- Cho phép ghi báo cáo và chuyển phát sinh thành `Kiểm tra` hoặc `Cần xử lý` trong một dialog, một ngữ cảnh và một primary CTA; không đóng báo cáo hoặc đổi tab để tạo liên kết.

Will be done:
- Đổi form tạo báo cáo sang chế độ nhanh: nhóm công việc dạng tab gọn, blocker có lựa chọn xử lý, minh chứng ở luồng chính và nguồn lực/ghi chú mặc định thu gọn.
- Thêm shared compact composer cho kiểm tra/việc xử lý, dùng trực tiếp trong form tạo và chi tiết báo cáo; tự kế thừa công việc, tiêu đề và minh chứng nguồn.
- Lưu báo cáo rồi tạo các liên kết đã chuẩn bị bằng API V65 hiện hữu; nếu một liên kết lỗi, giữ báo cáo và chuyển đúng draft lỗi sang chi tiết để thử lại mà không tạo trùng.
- Tạo kiểm tra/việc từ báo cáo hiện hữu ngay trong dialog báo cáo, refresh timeline liên quan tại chỗ và không đổi tab.
- Giữ nguyên permission, evidence ownership, fail-to-action, action events, technical compatibility và handover blocker.
- Cập nhật central/frontend/root-cause/construction/quality mappings; chạy frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `Approve`.
- Reloaded mandatory central/frontend/root-cause mappings plus frontend workflow, impact analysis and construction/quality domain guardrails.
- Confirmed implementation remains frontend-only over the existing V65 create routes and canonical evidence/status transitions.
- Added a shared compact composer for `Kiểm tra` and `Việc xử lý`; it reuses report work-item/evidence context, supports direct criteria, and keeps optional fields collapsed.
- Reworked report creation into one flow: quick context defaults, tabbed work groups, blocker-level `Chỉ ghi nhận`/`Tạo kiểm tra`/`Giao xử lý`, primary evidence capture, collapsed supplementary data, and one summary CTA.
- Orchestrated report plus linked records from one submit while keeping successful children idempotent; only failed drafts and failed evidence are retained for inline retry.
- Embedded creation in report detail so new checks/actions refresh the related timeline in place without closing the report or changing tabs.
- Reused the canonical construction create commands and preserved permission, evidence ownership, check-to-action transition, action events, technical compatibility, and handover blockers.
- Updated central/frontend/root-cause/construction/quality mappings for the new composer, one-flow behavior and partial-retry path.
- Validation passed with the frontend production build and `git diff --check`; browser verification was skipped because the user did not explicitly request it.

## Daily Report To Construction Resolution Traceability

Status: complete

Mục đích chính:
- Tạo chuỗi truy vết hiện trường có quan hệ nguồn rõ ràng `Nhật ký/Báo cáo -> Kiểm tra -> Cần xử lý -> Khắc phục/Xác nhận`, không suy luận theo ngày hoặc công việc.

Will be done:
- Thêm migration V65 với nguồn báo cáo tùy chọn cho kiểm tra và việc xử lý trực tiếp; việc sinh từ kiểm tra tiếp tục kế thừa nguồn qua `source_check_id` để tránh lưu trùng.
- Mở rộng typed contract, service/repository validation và construction workspace response; nguồn phải cùng công trình, và phải cùng work item khi báo cáo đã khóa work item.
- Bổ sung chi tiết báo cáo với trạng thái liên quan, CTA tạo kiểm tra/việc xử lý, form prefill nguồn/công việc/minh chứng và liên kết ngược trong View kiểm tra/việc xử lý.
- Giữ nguyên fail-to-action atomic transition, technical issue/rework/replanning bridge, permission hiện hữu, lịch sử action events và evidence ownership.
- Cập nhật Golden fixture, integration tests, central/backend/frontend/construction/quality mappings; chạy backend format/tests/migration checks, frontend build và `git diff --check`.
- Không backfill production bằng ngày/work item và không chạy browser verification vì người dùng không yêu cầu trong lượt này.

Done:
- User approved the review package and Mockup as Text with `OK triển khai`.
- Reloaded mandatory central/backend/frontend/root-cause mappings plus frontend/backend workflow, impact analysis and construction/quality domain guardrails.
- Confirmed current storage only links `construction_action_items.source_check_id`; reports, checks and direct actions otherwise share contextual project/work-item fields without a causal report relation.
- Added migration V65 with optional daily-report source foreign keys for checks and direct actions, same-project/work-item validation, indexes, and inherited report traceability for actions generated from failed checks.
- Extended the construction workspace contracts to expose report date, weather and direct-versus-check linkage while preserving the existing issue, rework, replanning and action-event lifecycle.
- Added report-list counters and a linked check/action/remediation timeline in report detail, with permission-aware create actions and navigation through the existing shared check/action dialogs.
- Updated create/view flows so checks and actions can start from a daily report with locked work-item context and reusable report evidence; check/action detail links back to its source report.
- Updated Golden fixtures and validators with a complete report -> check -> failed action -> resolved remediation chain; the construction fixture reload is idempotent and leaves no open quality/handover blockers.
- Updated the central, backend, frontend, root-cause, construction-operations and quality/handover maps for the new relation and UI ownership.
- Validation passed: backend formatting/full tests, focused database integration test, 65-migration check, Golden fixture and validator checks, authenticated construction-workspace API smoke check, frontend production build and `git diff --check`.
- Browser verification was skipped because the user did not explicitly request it in this turn.

## Direct Construction Check Criteria And Evidence

Status: complete

Mục đích chính:
- Cho phép người dùng tạo một kiểm tra hoàn chỉnh ngay trong một dialog bằng cách nhập tiêu chí trực tiếp và đính kèm ảnh hiện trạng, không bắt buộc tạo mẫu trước.

Will be done:
- Thêm migration V64 với liên kết nhiều evidence theo vai trò `context`/`result`, backfill evidence kết quả hiện hữu và giữ `construction_checks.evidence_id` làm primary compatibility field.
- Mở rộng create/update check contract với tiêu chí trực tiếp và context evidence; complete check nhận nhiều result evidence, giữ validation cùng project/work item, status và actor/audit.
- Đổi form Tạo/Sửa sang tiêu chí inline thêm/xóa, `Nạp từ mẫu` là lựa chọn phụ, ảnh hiện trạng tùy chọn; View tách gallery ảnh hiện trạng và minh chứng kết quả.
- Giữ RFI-backed check read-only, quyền V63, fail-to-action và handover blocker không đổi.
- Cập nhật seed/validator/tests và central/backend/frontend/construction/quality mappings; chạy backend format/tests/migration/seed validation, frontend build, API smoke và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu trong lượt triển khai này.

Done:
- User approved the review package and Mockup as Text with `OK triển khai theo review package này`.
- Reloaded mandatory central/backend/frontend/construction/quality mappings plus impact-analysis and migration instructions.
- Confirmed the existing worktree contains the approved V62/V63 construction workspace and will be preserved.
- Added V64 `construction_check_evidence` with ordered `context`/`result` roles and legacy primary-result backfill; create/edit accepts direct criteria plus optional context evidence, while completion accepts up to 10 result evidence items.
- Updated the check dialogs so criteria are entered inline, a template only preloads editable rows, create/edit can capture current-state images, and View renders separate current-state and result galleries.
- Preserved pending RFI edit locks, same-project/work-item evidence validation, fail-to-action behavior and the legacy `construction_checks.evidence_id` primary result field.
- Extended the Golden construction fixture with one pending current-state image and three result evidence associations; added validator checks and focused normalization tests.
- Updated central/backend/frontend/root-cause plus construction/quality domain mappings for V64 contracts, ownership and troubleshooting paths.
- Ran backend format/tests, 64-migration check, fixture check, frontend production build and `git diff --check`: pass; Vite reported only the existing large-chunk advisory.
- Applied V64 through the custom migration runner (`applied=1 skipped=63`), reloaded only the Golden construction fixture, rebuilt/recreated backend and confirmed the final startup reports `applied=0 skipped=64`.
- Golden validation returned `GOLDEN_SAMPLE_VALIDATION_PASSED`; authenticated API smoke returned 4 checks, 1 context link, 3 result links and 2 direct criteria on the pending sample, and the typed edit API preserved its criteria/context evidence.
- Browser verification was skipped because the user did not explicitly request it in this turn.

## Construction Check And Action UI Redesign

Status: complete

Mục đích chính:
- Giúp người dùng tại công trường tìm nhanh kiểm tra hoặc việc cần xử lý, xem đầy đủ ngữ cảnh/minh chứng/lịch sử, thực hiện đúng hành động tiếp theo và sửa metadata an toàn mà không viết lại kết quả nghiệp vụ.

Will be done:
- Thiết kế lại List `Kiểm tra` và `Cần xử lý` thành bảng tác nghiệp dense trên desktop, card responsive trên mobile, có search, status queue, bộ lọc, row actions và đầy đủ loading/error/empty/no-result states.
- Tách shared dialog cho Create/Edit/View; giữ transition `Ghi kết quả`, `Gửi khắc phục`, `Xác nhận` theo trạng thái và permission hiện hữu.
- Thêm typed update API cho kiểm tra thủ công còn `pending` và action canonical còn `open`; khóa các trường nguồn/minh chứng/ảnh hưởng/trạng thái và các bản ghi technical backfill.
- Đồng bộ action edit với compatibility issue/rework trong một transaction, ghi actor/audit, giữ nguyên handover blocker và evidence/work-item gates.
- Thêm permission `construction.check.edit`, `construction.action.edit` bằng migration V63 và cấp cho cùng nhóm role đang có quyền request/create tương ứng.
- Cập nhật focused tests, Golden validator, central/backend/frontend/construction/quality mappings; chạy backend format/tests, migration/seed validation, frontend build và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu trong lượt triển khai này.

Done:
- User approved the full review package and Mockup as Text with `OK triển khai`.
- Reloaded the mandatory central/backend/frontend/construction/quality/workflow maps plus impact-analysis, migration and git workflow instructions.
- Confirmed the current worktree already contains the approved V62 simplified construction workspace changes; unrelated/current user changes will be preserved.
- Replaced the compact check/action card grids with operational status queues, search and domain filters, dense desktop tables, mobile cards, overdue/urgent emphasis, row actions and contextual empty/no-result states.
- Added shared check dialogs for template creation, create/edit/view and evidence-gated pass/fail completion; check detail now shows criteria, context, protected evidence and its linked action.
- Added shared action dialogs for create/edit/view, resolution submission and verification; action detail now shows immutable source context, before/after protected evidence, impact state and the complete event timeline.
- Added typed `PUT /construction-checks/:id` and `PUT /construction-actions/:id` routes with project-scoped RBAC, request validation and audit. Pending RFI-backed checks and technical NCR/legacy issue backfills remain read-only.
- Kept action source/work item/evidence/impact/status immutable; safe metadata changes synchronize linked issue/rework title, description, priority/severity, assignee and due date in one transaction.
- Added migration V63 with `construction.check.edit` and `construction.action.edit`, restored both permissions in Golden seed resets and granted them to Admin, PM, QA/QC, Supervisor and Construction Team.
- Added focused editability regression coverage and extended Golden validation for permission/grant counts; updated central/backend/frontend/construction/quality mappings for the new files, routes, helpers and invariants.
- Ran `cd backend && make fmt`, `make test`, `make migrate-check`, `make seed-fixtures-check`, `make seed-validate`, `cd frontend && npm run build` and `git diff --check`: pass; Vite reported only the existing large-chunk advisory.
- Built/recreated only the backend container; the custom migration runner reported `applied=1 skipped=62`, and Golden validation returned `GOLDEN_SAMPLE_VALIDATION_PASSED`.
- Authenticated API smoke confirmed the workspace remains 1 template/4 checks/1 pending/1 completed action/0 active actions, pending manual check edit returns HTTP 200, completed action edit is rejected with `VALIDATION_ERROR`, and the Construction Team receives both new edit permissions.
- Browser verification was skipped per repository rule because the user did not explicitly request it.

## Simplified Construction Check And Action Workspace

Status: complete

Mục đích chính:
- Giúp người dùng tại công trường ghi nhận kết quả kiểm tra và xử lý một việc chưa đạt bằng ít thao tác nhất, không phải hiểu hay vận hành trực tiếp ITP, RFI, NCR, rework hoặc replanning.

Will be done:
- Thay bề mặt chính của `Thi công` bằng ba tab `Nhật ký`, `Kiểm tra`, `Cần xử lý`; bỏ dashboard kỹ thuật và các form ITP/RFI/NCR/rework/replanning khỏi luồng người dùng hiện trường.
- Thêm contract dữ liệu đơn giản cho mẫu kiểm tra, lần kiểm tra, việc cần xử lý và lịch sử xử lý; giữ liên kết kỹ thuật với dữ liệu cũ để bảo toàn audit và báo cáo.
- Cho phép hoàn tất kiểm tra bằng `Đạt/Không đạt + ảnh + ghi chú`; kết quả không đạt tự tạo một việc cần xử lý và chỉ hỏi người phụ trách, hạn xử lý, ảnh hưởng.
- Cho phép việc cần xử lý đi qua ba trạng thái `Cần xử lý -> Chờ xác nhận -> Đã xong`, gồm ảnh khắc phục, xác nhận đạt/không đạt và lịch sử thử lại.
- Đồng bộ cờ ảnh hưởng với issue/rework/replanning kỹ thuật phía sau và bổ sung action đang mở vào điều kiện sẵn sàng bàn giao.
- Thêm sample data cho project `30000000-0000-0000-0000-000000000020` với kiểm tra đạt, kiểm tra không đạt đã khắc phục, lịch sử hoàn tất và một kiểm tra đang chờ; không tạo blocker mở trên công trình đã bàn giao.
- Cập nhật central/backend/frontend/construction/quality mappings; chạy migration check, backend tests, seed validation, frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the full simplified review package and requested implementation plus sample data for project `30000000-0000-0000-0000-000000000020`.
- Loaded the mandatory central/backend/frontend/construction/quality mappings plus repository architecture, backend, frontend, impact-analysis and database workflow instructions.
- Confirmed the existing operations page directly exposes technical Quality and Rework workspaces, while legacy project quality/rework routes redirect into those tabs.
- Confirmed legacy quality, issue, rework and replanning records can remain as technical compatibility links behind one new field-facing model without changing historical rows.
- Confirmed the target Golden Real project is already handed over, so sample data must contain no open simplified action blocker.
- Added migration V62 with canonical simplified check templates, checks, action items/events, `construction.*` permissions, technical-history backfill and open-action handover blocking.
- Added protected workspace/template/check/action APIs with typed validation, same-project/work-item evidence ownership, active-project assignee validation, audit events and atomic compatibility issue/rework/optional replanning synchronization.
- Replaced the operations quality/issues embeds and technical status cards with `Nhật ký`, `Kiểm tra`, `Cần xử lý`; all field create/result/resolution/verification interactions now use compact shared dialogs and camera/file evidence.
- Kept legacy global quality/rework consumers and technical API routes available, while project compatibility routes now redirect to `tab=checks|actions`.
- Added `golden_project_construction.sql` to the seed sequence and loaded project `30000000-0000-0000-0000-000000000020` with one active template, four checks, one resolved action and three action events; the only pending sample is a check, not an action blocker.
- Updated seed validation for the simplified dataset, `construction.*` role grants, open-action handover invariant, Golden Live untouched state and both terminal rework status spellings used by current code.
- Applied V62 through the custom migration runner in Docker, loaded only the dedicated construction seed, rebuilt/recreated only backend and confirmed the runner reports 62 migrations.
- Ran the Golden validator: `GOLDEN_SAMPLE_VALIDATION_PASSED`; authenticated API smoke returned 1 template, 4 checks, 1 pending check, 1 action, 0 open actions and 7 participants.
- Updated central/backend/frontend/construction/quality mappings for V62 ownership, field terminology, compatibility links and handover impact.
- Ran `cd backend && make test`, `make migrate-check`, `cd frontend && npm run build`, focused legacy-label searches and `git diff --check`: pass; Vite reported only the existing large-chunk advisory.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Contract Workspace Vietnamese Redesign

Status: complete

Mục đích chính:
- Giúp người quản lý nhanh chóng chọn đúng hợp đồng, nhận ra tình trạng và việc cần xử lý, rồi đi sâu vào phiên bản, mốc thanh toán, phụ lục hoặc khoản giữ lại bằng giao diện hoàn toàn tiếng Việt.

Will be done:
- Giữ bố cục danh sách–chi tiết trên màn hình lớn; bổ sung tìm kiếm, lọc chiều công nợ/trạng thái và luồng danh sách–chi tiết rõ ràng trên màn hình nhỏ.
- Đưa tình trạng, giá trị và một việc cần xử lý lên đầu chi tiết; chuyển bốn bảng dài thành các thẻ nội dung có số lượng.
- Việt hóa toàn bộ nhãn hệ thống của trang, loại hợp đồng, mốc thanh toán, công nợ, trạng thái, biểu mẫu và phản hồi; giữ nguyên mã, tên riêng và dữ liệu người dùng nhập.
- Dùng bảng dày trên màn hình rất rộng và thẻ dữ liệu trên màn hình hẹp để loại bỏ phụ thuộc vào bảng rộng 900px.
- Giữ nguyên API, quyền và các cổng nghiệp vụ phiên bản/mốc/phụ lục/giữ lại; cập nhật central/frontend/commercial/accounting mappings.
- Chạy `cd frontend && npm run build`, tìm chuỗi giao diện chưa Việt hóa và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `OK triển khai`.
- Loaded the mandatory central/frontend/commercial/accounting mappings and inspected the overlapping dirty worktree before editing.
- Confirmed the existing contract list/detail APIs, permission gates and financial transitions already support a frontend-only redesign.
- Rebuilt the project contract workspace with a searchable/filterable master list, selected-contract summary, local Vietnamese contract/type/direction labels and mobile list-to-detail navigation.
- Added one permission-aware next-action band ordered through version review/approval/signing, contract activation, milestone eligibility/obligation, variation lifecycle and retention release without changing server transitions.
- Replaced four stacked 900px-minimum tables with count-bearing tabs, very-wide desktop tables and compact responsive record cards for versions, milestones, variations and retentions.
- Removed raw obligation ID fragments from the presentation, localized gross/net/hash/AP-AR concepts, formatted dates with the Vietnamese locale and translated the remaining shared `review`/`incorporated` status labels.
- Kept shared dialogs, inline form validation, dismissible mutation feedback, all current permission checks and the existing contract/AP/AR endpoints; no backend, migration, seed or response-contract change was required.
- Updated central/frontend/commercial/accounting mappings with the new action-first contract workspace ownership and preserved financial/commercial invariants.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran focused user-facing English-label searches and `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Contract Detail Project Permission Allowlist Fix

Status: complete

Mục đích chính:
- Khôi phục workspace chi tiết Hợp đồng cho người có quyền bằng cách cho phép helper phân quyền theo bản ghi phân giải `contracts.project_id`, không thay đổi UI, API contract hoặc nghiệp vụ AP/AR.

Will be done:
- Bổ sung `contracts` vào allowlist tên bảng an toàn mà `RecordProjectID` sử dụng.
- Thêm regression test khóa cả trường hợp hợp lệ và tên bảng có nội dung injection.
- Cập nhật backend function map và root-cause triage map cho lỗi `500 Bảng không hợp lệ`.
- Chạy format, focused backend tests, authenticated API smoke test và kiểm tra quyền; không chạy browser verification.

Done:
- User approved the diagnosed backend-only fix with `OK triển khai`.
- Reproduced `GET /api/contracts/65000000-0000-0000-0000-000000000201` returning HTTP 500 `INTERNAL_ERROR: Bảng không hợp lệ`, while the project contract list returned HTTP 200 and every contract-detail SQL query passed directly.
- Confirmed `GetContract` calls `requireRecordProjectPermission(..., "contracts", ...)`, `RecordProjectID` rejects names absent from `safeTable`, and the allowlist omitted `contracts`.
- Added `contracts` to the safe table allowlist and added a source-level regression test that preserves rejection of an unsafe table name.
- Updated the backend function map with the `contracts`/`RecordProjectID` dependency and added a root-cause recipe for HTTP 500 `Bảng không hợp lệ`; central/frontend/domain maps did not require changes because routes, response shapes, UI and commercial/finance invariants are unchanged.
- Ran the focused `TestSafeTableAllowsContractsForProjectRecordPermission`: pass.
- Ran `GOCACHE="$PWD/.gocache" go test ./...` under `backend`: pass.
- Built the backend image and recreated only the backend container; no migration or seed changed.
- Smoke-tested the authenticated Admin contract detail: HTTP 200 with 2 versions, 3 milestones, 1 variation and 2 retentions; the project contract list remains HTTP 200 with 3 records.
- Smoke-tested Warehouse without `contract.view`: HTTP 403 `FORBIDDEN`, confirming project RBAC remains enforced.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Members Role Label SQL Correction

Status: complete

Mục đích chính:
- Khôi phục workspace Thành viên cho người có quyền quản lý bằng cách sửa lỗi PostgreSQL `DISTINCT + ORDER BY` mà không đổi UI, permission hoặc contract API.

Will be done:
- Bọc tập nhãn distinct trong subquery và sắp xếp case-insensitive ở outer query.
- Thêm DB-backed regression coverage gọi `ProjectMemberOptions` để khóa lỗi SQLSTATE 42P10.
- Cập nhật backend mapping; chạy truy vấn pass/fail gốc, backend tests, build/recreate backend Docker và `git diff --check`.
- Không thay frontend, migration, seed hoặc chạy browser verification.

Done:
- User approved the diagnosed fix with `OK triển khai sửa SQL thành viên`.
- Reproduced the exact SQLSTATE 42P10 against PostgreSQL Docker and confirmed the subquery-wrapped candidate returns 13 distinct role labels.
- Wrapped the distinct role-label projection in an inner query and applied `ORDER BY lower(label)` only in the outer query; response shape and ordering semantics remain unchanged.
- Extended `TestProjectMemberLifecycleAndProjectManagerProtection` to call `ProjectMemberOptions`, require non-empty unique role labels and confirm the project operational role remains present.
- Updated the backend function map with the distinct/outer-ordering ownership rule; no central/frontend/domain map change was required because routes, response contract, UI, permissions and business invariants are unchanged.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass.
- Ran the DB-backed `TestProjectMemberLifecycleAndProjectManagerProtection` inside the Consflow Docker network: pass.
- Built the production backend image and recreated only the backend container; V60 remained applied and no migration changed.
- Smoke-tested the authenticated Admin member API: success, 12 items, 12 active, 13 role labels and `can_manage=true`.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Workflow Execution Workspace Redesign

Status: complete

Mục đích chính:
- Giúp người điều phối nhận ra ngay giai đoạn hiện tại, tiến độ thực thi, việc ưu tiên tiếp theo và mở đúng đầu việc; giữ điều phối node là hành động phụ có kiểm soát.

Will be done:
- Dùng chung một workflow execution workspace cho route `/projects/:id/pipeline` và tab `Quy trình` trong tổng quan công trình.
- Đưa current node, tiến độ bước loại trừ nhánh bỏ qua, count cần xử lý/chờ duyệt/bị chặn và một CTA `Mở việc tiếp theo` lên đầu.
- Dùng stage rail 280px trên desktop, selector trên mobile, bảng công việc desktop và card mobile với bộ lọc trạng thái/tìm kiếm.
- Chuyển request approval, advance và block vào khu vực `Thao tác giai đoạn`; chuyển `Tạo lại quy trình` vào menu phụ với dialog xác nhận.
- Giữ nguyên API, permission và runtime gates; không thay backend, migration, status key hoặc work-item detail behavior.
- Cập nhật central/frontend/workflow mappings; chạy `cd frontend && npm run build` và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `OK triển khai`.
- Loaded the mandatory central/frontend/workflow mappings and inspected the overlapping dirty worktree plus the active Planning plan before editing.
- Confirmed the route `/projects/:id/pipeline` still renders the legacy `PipelineRuntimeSection`, while `ProjectDetailPage` already contains the approved action-first workflow presentation over `/projects/:id/operations`.
- Exported one `ProjectWorkflowWorkspace` and reused it in both the project Workflow tab and `/projects/:id/pipeline`, preserving the existing work-item detail navigation and backend runtime gates.
- Replaced the 12 equal-weight node cards on `/pipeline` with current-node/next-work summary, executable step progress, attention counts, 280px desktop stage rail, responsive selector, filtered desktop work table and mobile work cards.
- Moved request approval, advance and block into `Thao tác giai đoạn`; added required-reason block dialog and moved recreate into `Quản lý quy trình` with a confirmation dialog.
- Added project-operations loading/error states and invalidated the shared operations query after runtime actions so summary, stages and work rows refresh together.
- Updated central/frontend/workflow mappings for the shared workspace ownership, `/operations` presentation source and secondary node-control contract.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Planning Action-first Workspace

Status: complete

Mục đích chính:
- Giúp người dùng mở trang `Kế hoạch` và nhận ra ngay giai đoạn hiện tại, việc nào phải xử lý tiếp theo, cùng hàng đợi công việc đúng trạng thái runtime mà không phải quét toàn bộ biểu mẫu tương lai.

Will be done:
- Chuyển `Kế hoạch` từ inventory 51 biểu mẫu sang hàng đợi 52 đầu việc, mặc định ưu tiên blocker -> chờ duyệt/rà soát -> việc đang mở theo thứ tự workflow.
- Tách rõ `Cần xử lý`, `Chờ duyệt`, `Sắp tới`, `Hoàn tất` và `Tất cả`; giữ trạng thái hồ sơ, mức bắt buộc và minh chứng như metadata riêng.
- Đưa một việc tiếp theo, trạng thái node hiện tại, tiến độ bước thực hiện và các count cần chú ý lên đầu màn hình.
- Giữ node transition, phê duyệt, chặn node và audit đầy đủ trong route `Quy trình`; `Kế hoạch` chỉ liên kết sang đó.
- Trong `Kế hoạch`, chỉ hiển thị CTA khởi động khi chưa có run. Việc tạo lại run đang hoạt động thuộc route `Quy trình`, nằm trong menu quản lý và bắt buộc qua dialog xác nhận theo review package mới nhất đã được duyệt.
- Giữ nguyên API, route, permission và work-item detail gates; cập nhật central/frontend/project-planning/workflow mappings.
- Chạy `cd frontend && npm run build` và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `OK triển khai`.
- Loaded the mandatory central/frontend/project-planning/workflow mappings, confirmed `PlanningPage.tsx` ownership and inspected the overlapping dirty worktree before editing.
- Confirmed the current `needsSubmit` classification ignores future `pending` work-item state, while the existing pipeline-run response already supplies all 52 ordered steps/work items and 51 form instances needed for a frontend-only redesign.
- Rebuilt `Kế hoạch` as an action-first queue over all ordered runtime steps, with default actionable, waiting approval/review, future pending, completed/history and all-work views.
- Added one next-work CTA ordered blocker -> waiting approval/review -> other open work, plus current-stage progress, actionable/waiting/blocked counts and submitted-form coverage.
- Kept workflow-form status, requirement and evidence as supporting metadata; added dense desktop rows, responsive mobile cards, search/type/status/requirement filters and loading/error/empty/filter-empty states.
- Kept planning rows navigation-only and routed node transition/approval/block operations to `Quy trình`; the active Planning header no longer presents `Khởi động / tạo lại quy trình` as its primary action.
- Preserved work-item return navigation, permission-gated detail behavior, API routes and frontend domain contracts; no backend, migration or seed change was required.
- Integrated the concurrent `/projects/:id/pipeline` execution-workspace redesign without restoring the removed legacy runtime section.
- Updated central/frontend/project-planning/workflow mappings with the queue classification and navigation-only invariants.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran focused `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Members Governance Workspace

Status: complete

Mục đích chính:
- Giúp thành viên công trình xem đúng danh bạ đang hoạt động; giúp người có quyền quản lý thêm, chỉnh vai trò/quyền và ngừng tham gia mà vẫn giữ lịch sử truy cập.

Will be done:
- Tách danh sách thành viên khỏi payload Operations thành API typed, permission-scoped và có capability quản lý rõ ràng.
- Cho thành viên đang hoạt động xem danh bạ active; chỉ người có `project.member.manage` được xem lịch sử inactive và thực hiện thay đổi.
- Thêm permission `project.member.manage`, unique membership theo công trình/nhân sự và luồng thêm/cập nhật/ngừng tham gia dạng transaction đồng bộ `project_members` với `user_roles` project scope.
- Chặn ngừng tham gia PM hiện tại, dùng soft-deactivate, ghi audit cho mọi thay đổi và không hiển thị `assigned_nodes` chưa có runtime semantics.
- Xây `ProjectMembersPanel` với tìm kiếm, bộ lọc, bảng/card responsive, dialog dùng chung, loading/empty/error/filter-empty và toast dismissible.
- Đồng bộ tab/sidebar theo visibility/capability backend; cập nhật central/backend/frontend/project-planning/admin-governance mappings.
- Chạy backend format/tests, migration check, frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `OK triển khai theo review package này`.
- Loaded the mandatory central/backend/frontend/project-planning/admin-governance mappings and inspected the dirty worktree before editing.
- Confirmed current members are embedded in `/projects/:id/operations`, only Admin/Director receive them, the navigation still exposes the empty tab to project members, inactive rows are not filtered, and project-scoped `user_roles` are managed separately from `project_members`.
- Added typed protected member routes for list/add/update/soft-deactivate with `project.view` directory access and server-computed `project.member.manage` capabilities.
- Added migration V60 with member notes/updated timestamps, one-active-membership integrity, non-destructive duplicate normalization and the new permission granted to Admin/Director/PM.
- Added transactional membership persistence that synchronizes project-scoped `user_roles`, preserves global roles, requires effective `project.view`, excludes Admin/Director from assignable project roles, audits every change and blocks removal of the current PM.
- Removed members from the Operations payload/hidden-section contract and kept the legacy project detail member list active-only.
- Replaced the flat four-column table with `ProjectMembersPanel`: summary, search/department/role/status filters, dense desktop table, mobile cards, detail/add/edit/deactivate dialogs, inline validation, loading/error/empty/filter-empty states and dismissible feedback.
- Kept operational `role_ids`, project roles and global roles visually/contractually separate; left unused `assigned_nodes` out of the UI.
- Added frontend domain types plus focused normalization and DB-backed lifecycle/PM-protection tests; updated central/backend/frontend/project-planning/admin-governance mappings.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass.
- Ran `make migrate-check`: pass with 60 migrations.
- Built the backend Docker image and applied V60 through `/app/consflow-migrate`: one migration applied, 59 skipped.
- Ran `make seed-validate`: `GOLDEN_SAMPLE_VALIDATION_PASSED` for GOLDEN-LIVE, GOLDEN-NO-DESIGN and GOLDEN-REAL.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Audit Investigation Workspace

Status: complete

Mục đích chính:
- Giúp người quản trị nhanh chóng xác định ai đã làm gì, vào lúc nào, trên đối tượng nào và dữ liệu chi tiết của thay đổi là gì.

Will be done:
- Chuyển trang Audit theo công trình và toàn hệ thống thành workspace điều tra với tìm kiếm, bộ lọc server-side, phân trang, chế độ danh sách/timeline và dialog chi tiết chỉ đọc.
- Thu gọn tab Audit trong Tổng quan công trình thành preview năm sự kiện gần nhất, không lặp Timeline và bảng song song.
- Giữ nguyên `audit.view`, `requirePermission`, `requireProjectPermission`, cách ghi `audit_logs` và phạm vi nhìn thấy theo công trình.
- Thêm contract Audit có kiểu dữ liệu rõ ràng và response phân trang; trang toàn hệ thống hiển thị thông tin công trình nhưng vẫn áp `ProjectAccessFilter`.
- Cập nhật central/backend/frontend/admin-governance mappings; chạy backend format/tests, frontend build và `git diff --check`.
- Không chạy browser verification vì người dùng không yêu cầu trong lượt này.

Done:
- User approved the review package and Mockup as Text with `OK triển khai theo review package này`.
- Loaded the mandatory central/backend/frontend/admin-governance mappings and completed the cross-contract impact matrix.
- Confirmed the only frontend consumers of `/audit-logs` and `/projects/:id/audit-logs` are `AuditPage` and `ProjectAuditPanel`.
- Confirmed no migration, seed, permission, audit-write, report or export change is required for the approved scope.
- Added typed `AuditLogFilter` and paginated Audit response with server-side project/event/target/actor/date/search filters, stable newest-first ordering and permission-scoped filter options.
- Preserved global `ProjectAccessFilter`, project `requireProjectPermission`, route `audit.view` guards and all existing `AddAudit` write behavior.
- Rebuilt `/audit` and `/projects/:id/audit` as a dense investigation workspace with global project context, list/timeline toggle, responsive cards, read-only metadata detail, refresh, empty/error states and server pagination.
- Replaced the project Audit tab's duplicated timeline/table layout with a five-event preview and removed the misleading preview-length badge.
- Updated Planning's audit timeline to consume `AuditLogPage.items` after the contract impact scan found it as a third endpoint consumer.
- Updated central/backend/frontend/admin-governance/project-planning mappings for the new contract and UI ownership.
- Added unit coverage for Audit filter normalization and a DB-backed query test that runs when `CONSFLOW_TEST_DATABASE_URL` is available.
- Verified the generated count/list/filter-option SQL with read-only queries inside the Consflow PostgreSQL container.
- Ran `cd backend && make test`, `cd frontend && npm run build`, and `git diff --check` successfully; Vite reported only the existing large-chunk advisory.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Overview Vietnamese Redesign

Status: complete

Mục đích chính:
- Giúp người điều hành trong khoảng 10 giây nhận ra công trình đang ở đâu, việc nào phải xử lý tiếp theo và rủi ro nào cần can thiệp.

Will be done:
- Đưa một công việc cần làm tiếp theo và giai đoạn hiện tại lên đầu tab `Tổng quan`.
- Tách cảnh báo cần chú ý khỏi thông tin tham khảo; rút danh sách ưu tiên còn năm việc và bổ sung bố cục thẻ trên màn hình nhỏ.
- Việt hóa toàn bộ nhãn hệ thống trong tab, hành động và cây nghiệp vụ của không gian công trình; giữ nguyên mã, tên riêng và dữ liệu do người dùng nhập.
- Giữ nguyên API `/projects/:id/operations`, quyền, trạng thái và thứ tự ưu tiên công việc.
- Cập nhật frontend/project/workflow mapping; chạy frontend build, kiểm tra chuỗi tiếng Anh và `git diff --check`.

Done:
- User approved the review package and Mockup as Text with `OK triển khai`.
- Confirmed the redesign is presentation-only and the existing project operations response already supplies project summary, health counts, workflow timeline and complete scoped work items.
- Loaded the mandatory central/frontend/project-planning/workflow mappings and inspected the overlapping dirty worktree before editing.
- Rebuilt `Tổng quan` around one prioritized next action, current runtime stage, a distinct attention strip, five-item priority preview and a separate project-information panel.
- Added compact mobile work cards while preserving the dense desktop table and existing work-item detail navigation.
- Localized project tab, action, empty/error, workflow and project-tree system labels into Vietnamese; preserved route keys, API status keys, project codes, names and user-entered work-item text.
- Updated central/frontend/project-planning/workflow mappings for the new overview hierarchy and localization contract.
- Ran `cd frontend && npm run build` successfully; Vite reported only its existing large-chunk advisory.
- Ran focused `git diff --check` successfully and confirmed the remaining English search hits are internal identifiers, route/status keys or type names rather than user-facing labels.
- Browser verification was skipped per repository rule because it was not explicitly requested in this turn.

## Project Activity Emotion Button Contrast

Status: complete

Mục đích chính:
- Làm nút thêm emotion luôn được nhận biết là hành động tương tác trên cả bubble xanh và nền sáng mà không cạnh tranh với reaction đã chọn.

Will be done:
- Thêm nền trắng, viền, shadow nhẹ và icon xanh đậm cho nút `SmilePlus`.
- Thêm state hover, focus-visible, open và pending/disabled rõ ràng; giữ nguyên picker, emoji và API.
- Cập nhật frontend mapping; chạy frontend build và `git diff --check`.

Done:
- User approved the focused review package and Mockup as Text with `OK triển khai nút emotion`.
- Confirmed the root cause is the transparent `text-slate-500` icon button rendered on the own-message `bg-blue-600` bubble; reaction state and API remain healthy.
- Replaced the transparent trigger with a bordered `bg-white/95` surface, blue icon and light shadow so it remains visible on both blue and light bubbles.
- Added distinct hover, focus-visible and open styling, increased the icon to 16px and exposed `aria-expanded`.
- Disabled the trigger while a reaction mutation is pending; the existing picker, emoji set, optimistic mutation and reaction pills remain unchanged.
- Updated the frontend UI map for the shared interactive reaction trigger. Central/domain maps did not need updates because route, API contract, persistence, permission and activity invariants are unchanged.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Evidence Workspace Redesign

Status: complete

Mục đích chính:
- Biến tab `Hồ sơ` thành trung tâm tra cứu và rà soát minh chứng công trình dạng danh sách dày, có dữ liệu đầy đủ, phân trang/lọc nguồn thật và hành động đúng quyền.

Will be done:
- Thêm API project evidence riêng với tìm kiếm, filter loại/trạng thái/đầu việc/người gửi/thời gian, cursor pagination, summary theo đúng project scope và `can_review` theo từng bản ghi.
- Giữ hồ sơ của work item `cancelled` ngoài danh sách vận hành nhưng cho phép xem lịch sử `rejected` trong tab Hồ sơ.
- Thay gallery PDF có preview lớn bằng project evidence workspace: KPI, filter dày, danh sách desktop, card compact mobile, loading/empty/error/filter-empty và dialog chi tiết dùng shared primitives.
- Giữ upload trong work-item detail; tab Hồ sơ chỉ tra cứu, mở và rà soát, không gộp `documents` hoặc workflow PDF trong lượt này.
- Cập nhật central/backend/frontend/project-planning/workflow mappings; chạy backend format/tests, frontend build và `git diff --check`.

Done:
- User approved the review package and Mockup as Text with `OK triển khai theo review package này`.
- Confirmed current `/projects/:id/operations` exposes only 50 latest non-rejected evidence rows, while the project UI filters locally and presents a rejected filter that cannot return data.
- Confirmed PostgreSQL direct-SQL repository ownership, existing `evidence_records` indexes/status model and backend `CanDecideWorkItem` enforcement; no migration is planned unless focused query validation proves an index is required.
- Loaded mandatory central/backend/frontend/project-planning/workflow mappings and preserved the existing dirty worktree without reverting overlapping Activity/Workflow changes.
- Added protected `GET /projects/:id/evidence` with validated search/type/status/work-item/sender/visibility/date filters, stable waiting-first cursor pagination, filtered totals, source-backed summary and scoped filter options.
- Included rejected evidence as review history while keeping evidence from cancelled work items out of the project workspace; internal project users can see internal/project/client-visible evidence in their work scope, while client profiles remain restricted to client-visible evidence.
- Added per-row `can_review` derived from the existing `CanDecideWorkItem` gate; the accept/reject endpoints remain the authoritative backend enforcement.
- Added `ProjectEvidencePanel.tsx` with KPI filters, dense search/filter toolbar, compact desktop/mobile evidence rows, loading/error/empty/filter-empty states and cursor `Tải thêm`; upload remains in work-item detail.
- Extended shared `EvidenceGallery` with externally controlled filters, a project list context, stage/source metadata and permission-aware review controls while preserving the existing work-item evidence list/gallery.
- Changed the project Hồ sơ tab badge to use the full permission-scoped evidence summary instead of the capped `latest_evidence` slice.
- Updated upload/review invalidation so project evidence queries refresh after work-item evidence changes.
- Added unit coverage for evidence filter/cursor normalization and PostgreSQL integration coverage for waiting-first order, cursor continuation, summary parity, rejected history, cancelled-work-item exclusion and review capability.
- Updated central/backend/frontend/project-planning/workflow mappings for the new route, contract, repository functions, UI owner and evidence-history invariants.
- No migration or seed change was required because existing `evidence_records` status/visibility fields and project/work-item indexes support the new read contract.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass.
- Ran focused PostgreSQL integration test `TestProjectEvidencePageScopeHistoryAndReviewCapability` against an isolated temporary PostgreSQL 17 database: pass; the temporary server was stopped afterward.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Activity Chat Edit UX Correction

Status: complete

Mục đích chính:
- Làm cho chế độ chỉnh sửa tin nhắn chỉ phát request khi nội dung thực sự thay đổi, phản hồi validation gọn, không che workspace và giữ các điều khiển chat rõ ràng, nhất quán.

Will be done:
- Thêm dirty-state đã trim; khóa `Lưu chỉnh sửa` khi chưa thay đổi hoặc đang pending và không phát request/toast cho no-op.
- Phân loại validation edit thành warning; deduplicate toast giống nhau toàn hệ thống và giới hạn ba toast khác nhau.
- Đóng menu tin nhắn sau action, Escape hoặc click ngoài; giữ một menu duy nhất tại một thời điểm trong bubble.
- Tăng tương phản metadata của bubble người dùng và thu gọn banner edit về thời gian/phạm vi thay vì lặp tên công trình dài.
- Thêm `decisions` nguồn thật vào project activity summary và hiển thị count nhất quán trên tab `Quyết định`.
- Cập nhật central/backend/frontend/project-planning mappings; chạy backend format/tests, focused integration test, frontend build và `git diff --check`.

Done:
- User approved all eight diagnosed corrections and the Mockup as Text with `OK triển khai gói sửa UX edit`.
- Confirmed the winning frontend causes: no dirty comparison, generic error mapping, random-ID toast stacking, uncontrolled native `details`, fixed slate metadata on blue bubble, repeated long target context and a conditional that hides the decision badge.
- Confirmed backend correctly rejects unchanged content and current project activity summary does not yet expose a dedicated decision count.
- Added trimmed dirty-state comparison in the chat composer; unchanged edits show `Chưa có thay đổi`, keep the save button disabled and never call the API or create a toast.
- Kept pending edit submissions disabled and mapped backend `VALIDATION_ERROR` responses to a warning toast while preserving unexpected failures as error toasts.
- Updated the global toast provider to ignore identical active variant/title/description notifications and keep at most three distinct visible toasts.
- Replaced the native message-action `details` with a controlled popover that closes on edit/withdraw action, Escape and outside click.
- Added own-blue-bubble contrast tokens for visibility, target, compact time, edited state and work-item link metadata.
- Compacted the edit banner to message time plus scope/visibility; removed the repeated long project target label.
- Added source-backed `decisions` to `ProjectActivitySummary`, frontend domain/default state and the `Quyết định` badge; removed the conditional that hid that badge.
- Extended the Pha 2B integration test to compare the returned decision summary against the project decision row count.
- Updated central/backend/frontend/project-planning maps for dirty edit behavior, controlled menu, toast deduplication and decision summary ownership.
- No migration or seed change was required because this correction changes response derivation and frontend interaction only.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `make restart`: pass; backend skipped all 59 already-applied migrations and listened on port 8080.
- Ran Docker-network integration tests `TestActivityReplyAndReactionScope` and `TestActivityMessageEditAndWithdrawScope`: pass, including decision summary parity.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Activity Chat Edit And Withdraw Phase 2B

Status: complete

Mục đích chính:
- Cho phép tác giả sửa hoặc thu hồi comment trong thời hạn ngắn mà vẫn giữ nguyên chuỗi trả lời, lịch sử phiên bản, audit và ranh giới giữa trao đổi với activity nghiệp vụ.

Will be done:
- Chỉ cho tác giả chỉnh sửa hoặc thu hồi activity loại `comment` trong 15 phút; kiểm tra quyền và thời hạn ở backend.
- Chỉnh sửa chỉ thay đổi nội dung; scope, visibility, work item, mentions, attachments và liên kết reply giữ nguyên.
- Thu hồi dùng tombstone, không xóa vật lý; giữ replies, ẩn reactions và chặn reply/reaction mới.
- Thêm migration V59 cho metadata edit/withdraw và `activity_message_revisions` append-only ledger.
- Loại nội dung đã thu hồi khỏi unread, mentions, attention và search nhưng vẫn giữ activity trong timeline/count lịch sử.
- Thêm menu `Chỉnh sửa`/`Thu hồi`, composer edit mode, shared confirmation dialog và trạng thái tombstone trong Project Activity chat.
- Giữ typed operational activities bất biến; không thêm manager override hoặc workflow transition.
- Cập nhật central/backend/frontend/project-planning/workflow/admin mappings; chạy backend format/tests, migration check, frontend build và `git diff --check`.

Done:
- User approved the Phase 2B review package with `OK triển khai Pha 2B`.
- Re-confirmed backend and UI currently support reply/reaction but have no edit, withdraw or message revision contract.
- Re-confirmed `Dialog` is the shared modal primitive and the project chat composer is the correct edit surface.
- Added migration `V59__activity_message_edit_withdraw.sql` with edit/withdraw actor timestamps, monotonic revision number and append-only `activity_message_revisions` ledger.
- Added protected `PUT /activities/:id/message` and `POST /activities/:id/withdraw`; both reuse activity visibility, require exact author ownership, allow only `comment` and enforce the 15-minute server window.
- Kept scope, visibility, target, work item, mentions, attachments and reply parent immutable during edit; typed operational activities remain immutable.
- Made withdrawal transactional: preserve a tombstone and replies, clear current comment, deactivate reactions, close unread states and append revision plus audit without changing workflow state.
- Blocked replies and reactions to withdrawn messages; conversation responses expose withdrawn parent previews, server-computed capabilities and no active reactions for tombstones.
- Excluded withdrawn messages from My Work attention and project unread/mention counts while retaining them in project history, totals, pagination and reply counts.
- Added the own-message `…` menu, sticky composer edit mode, shared withdrawal confirmation dialog, edited label and responsive tombstone rendering; work-item history and reply quotes render tombstones read-only.
- Added focused integration coverage for author/type/time-window gates, revision/audit persistence, tombstone behavior, attention cleanup, reply preservation and reply/reaction rejection.
- Updated central/backend/frontend/project-planning/workflow-orchestration/admin-governance maps for V59, routes, tables, UI ownership and communication-only invariants.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `make migrate-check`: pass with 59 migrations.
- Ran `make restart`: pass; backend auto-applied V59 and started on port 8080.
- Verified PostgreSQL Compose reports 59 applied migrations, the new columns and `activity_message_revisions` table.
- Ran Docker-network integration tests `TestActivityReplyAndReactionScope` and `TestActivityMessageEditAndWithdrawScope`: pass.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Activity Chat Reply And Reactions Phase 2A

Status: complete

Mục đích chính:
- Cho phép thành viên phản hồi trực tiếp một trao đổi và thể hiện phản ứng nhanh mà không làm mất tính truy vết, visibility, quyền theo công trình hoặc ranh giới giữa trao đổi và trạng thái nghiệp vụ.

Will be done:
- Thêm reply dạng quote trong luồng chat phẳng; reply luôn là `comment` và kế thừa project/target/work item/visibility của tin gốc.
- Tự động tạo mention/unread cho tác giả tin gốc khi người khác reply.
- Thêm reaction idempotent `👍`, `👀`, `🙏`, `🎉`; reaction không thay đổi read, resolve, work-item status hoặc activity summary.
- Thêm migration `V58` cho `parent_activity_id` và `activity_reactions` active-state ledger; không sửa migration đã áp dụng.
- Đồng bộ API response, Project Activity chat, Work Item activity timeline và My Work attention context.
- Giữ edit/delete ngoài Pha 2A; giữ activity audit cho reply và không tạo audit/system event cho reaction.
- Cập nhật central/backend/frontend/project-planning/workflow/admin mappings; chạy backend format/tests, migration check, frontend build và `git diff --check`.

Done:
- User approved the Phase 2A review package, inheritance rules and reaction set with `Đồng ý`.
- Confirmed current persistence has no parent reference or reaction state and the existing activity table/query contract is shared by project chat, work-item detail and My Work attention.
- Added migration `V58__activity_replies_reactions.sql` with nullable self-reference `parent_activity_id`, parent lookup index and active-state `activity_reactions` ledger for `👍`, `👀`, `🙏`, `🎉`.
- Added protected `POST /activities/:id/reactions`; reaction upserts reuse `ActivityVisibleToUser`, reject system events and publish existing activity SSE invalidation without audit/system-event noise.
- Added project reply normalization: backend forces `comment`, inherits the parent project/target/work item/visibility, rejects hidden/system/cross-project parents and auto-mentions the parent actor.
- Extended activity query contracts with `reply_to`, `reply_count` and reaction summaries across project activity, work-item detail and My Work attention.
- Added chat reply mode with parent quote, inherited/locked context, reply counts, optimistic reaction toggles and shared read-only reply/reaction metadata in operational activity surfaces.
- Added focused integration coverage for inheritance, cross-project rejection, auto-mention/unread, reaction idempotency/whitelist, system-event rejection and response conversation fields.
- Updated central/backend/frontend/project-planning/workflow-orchestration/admin-governance maps for V58, routes, tables, UI ownership and communication-only invariants.
- Ran `make restart`: pass; backend auto-applied one migration and skipped 57, then listened successfully on port 8080.
- Verified PostgreSQL Compose reports 58 applied migrations and smoke-tested project activity, My Work attention and work-item detail API contracts: all returned success.
- Ran the focused PostgreSQL Compose integration test `TestActivityReplyAndReactionScope`: pass.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass.
- Ran `make migrate-check`: pass with 58 migrations.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Activity Chat-like Discussion Phase 1

Status: complete

Mục đích chính:
- Giúp thành viên công trình trao đổi theo nhịp hội thoại tự nhiên trong tab `Thảo luận`, đồng thời giữ `Tất cả` và `Nhật ký hệ thống` ở dạng activity feed phục vụ vận hành và truy vết.

Will be done:
- Chỉ chuyển tab `Thảo luận` sang luồng chat theo thứ tự cũ đến mới; tin của người hiện tại nằm bên phải, người khác nằm bên trái.
- Hiển thị câu hỏi, quyết định, điểm chặn, nhắc việc và tệp dưới dạng bubble có nhận diện loại; giữ hành động đã đọc/đã xử lý và liên kết đầu việc.
- Đặt composer chat cố định cuối luồng, tái sử dụng validation/payload hiện có và không tự thu gọn sau khi gửi.
- Tải tin cũ ở đầu luồng, giữ vị trí cuộn, tự cuộn khi người dùng đang ở cuối và báo có tin mới khi họ đang đọc nội dung cũ.
- Giữ nguyên API, RBAC, visibility, SSE, flat-message model và activity feed hiện có; chưa làm reply, reaction, edit hoặc delete trong Pha 1.
- Cập nhật frontend/central/project-planning mappings; chạy `cd frontend && npm run build` và `git diff --check`; không chạy browser vì người dùng không yêu cầu.

Done:
- User approved the Phase 1 chat-like implementation with `OK triển khai chat-like Pha 1.`
- Confirmed the existing activity contract already provides actor, typed activity, visibility, mentions, attachments, unread/resolution metadata and realtime invalidation required by this frontend-only phase.
- Changed only the `Thảo luận` tab to a chronological chat surface with current-user messages aligned right, other participants aligned left, grouped authors and typed bubbles for operational messages.
- Added top loading for older messages with scroll anchoring, initial/latest auto-scroll and a `Có tin nhắn mới` affordance when the reader is away from the bottom.
- Added a persistent compact chat composer that shares the existing payload, validation, visibility, target, mention, attachment, owner and due-date logic; it remains open after successful send.
- Kept `Tất cả`, `Quyết định` and `Nhật ký hệ thống` on the existing descending activity-card feed and retained read/resolve/work-item actions in chat bubbles.
- Kept backend routes, database schema, RBAC, SSE and flat-message persistence unchanged; reply, reaction, edit and delete remain deferred.
- Updated central routing, frontend UI and project-planning domain maps with the chat/feed split and Phase 1 boundary.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Activity Sample Visibility Correction

Status: complete

Mục đích chính:
- Khi mở `Trao đổi`, người dùng nhìn thấy toàn bộ activity hiện có trước; thảo luận và nhật ký hệ thống là các bộ lọc chủ động, không làm sample project trông như thiếu dữ liệu.

Will be done:
- Đổi filter mặc định từ `Thảo luận` sang `Tất cả`.
- Đổi badge tab `Trao đổi` từ attention count sang tổng activity nhìn thấy được; attention count tiếp tục nằm trong panel `Cần chú ý`.
- Khi filter `Thảo luận` rỗng nhưng có system activity, hiển thị CTA mở `Nhật ký hệ thống`.
- Giữ nguyên backend rule ẩn activity thuộc work item `cancelled` trong lượt sửa này.
- Cập nhật frontend/project activity mappings và chạy `cd frontend && npm run build`, `git diff --check`; browser verification bỏ qua vì người dùng không yêu cầu.

Done:
- Reproduced against live `GOLDEN-REAL-001`: database has 52 activity rows, 0 discussion and 52 system rows; API exposes 48 system rows after excluding four cancelled-work-item rows.
- User approved the focused diagnosis with `OK triển khai theo chuẩn đoán này`.
- Changed project activity default from `Thảo luận` to `Tất cả` so seeded system history is visible immediately.
- Changed the project-tab badge to the source-backed total visible activity count; attention remains in `Cần chú ý`.
- Added a `Mở Nhật ký hệ thống` CTA when the discussion filter is empty but system history exists.
- Updated frontend, central and project-planning maps with the corrected default/count semantics.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Activity Operational Redesign

Status: complete

Mục đích chính:
- Biến tab `Trao đổi` thành trung tâm phối hợp công việc của công trình: ưu tiên nội dung cần phản hồi, gửi đúng người/đúng đối tượng và tách thảo luận khỏi nhật ký hệ thống.

Will be done:
- Thu gọn composer mặc định; chỉ hiện trường phụ theo loại ghi chú/câu hỏi/quyết định/điểm chặn/nhắc việc/tệp.
- Thay nhập UUID bằng picker thành viên, vai trò, đầu việc và đối tượng nghiệp vụ được backend trả theo project scope/RBAC.
- Mở rộng project activity API với tìm kiếm, filter, cursor pagination và summary count nguồn thật.
- Thêm hành động đã trả lời/đã xử lý/hoàn tất nhắc việc cùng mark-all-read, lưu trạng thái trong `metadata_json` và giữ activity history append-only.
- Tách thảo luận khỏi nhật ký hệ thống; thêm panel `Cần chú ý`, trạng thái loading/empty/error/filter-empty và responsive mobile.
- Đổi badge tab từ số bản ghi latest bị giới hạn sang tổng activity nhìn thấy được; số cần chú ý nằm trong panel riêng.
- Giữ nguyên visibility `internal/project/client`, project/work-item permission gates, SSE invalidation và auditability.
- Không triển khai thread reply, reaction hoặc external notification trong pha này; không cần migration schema.
- Cập nhật central/backend/frontend/project-planning/workflow-orchestration mappings trong cùng lượt.
- Chạy backend format/tests, frontend build và `git diff --check`; browser verification bỏ qua vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `OK triển khai theo review package này`.
- Confirmed current activity persistence already supports typed messages, visibility, mentions, unread states, attachments, arbitrary metadata and realtime SSE.
- Confirmed existing project activity query only supports scope filters with a hard limit and the tab badge currently reflects the latest-activity slice rather than a total.
- Confirmed schema migration is unnecessary while threaded replies remain deferred.
- Added cursor-paginated project activity feed with server search/filter for kind, attention queue, scope, activity type, actor and visibility.
- Added source-backed project activity summary, permission-scoped member/role/target/attachment options and a total-visible-activity tab badge; attention counts remain inside the attention panel.
- Added mark-all-read and authorized question/blocker/reminder resolution; resolution metadata remains on the original activity while a system event and audit record preserve append-only history.
- Rebuilt the project activity UI with a collapsed type-aware composer, no UUID inputs, discussion/decision/system tabs, desktop attention panel, mobile attention chips, filters, grouped activity cards, loading/filter-empty/error states and cursor loading.
- Reused project activity options in work-item detail so the shared composer retains mention and owner pickers without technical IDs.
- Updated central/backend/frontend/project-planning/workflow-orchestration maps for the new routes, contracts, UI surface and resolution invariant.
- Ran `GOCACHE="$PWD/.gocache" go test ./...` under `backend`: pass; DB-backed tests remain environment-gated by `CONSFLOW_TEST_DATABASE_URL`.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Work Tab Operational Queue Redesign

Status: complete

Mục đích chính:
- Giúp PM/Admin nhìn thấy ngay công việc cần xử lý, công việc đang chờ hoặc bị chặn, người/vai trò phụ trách và mở đúng đầu việc trong phạm vi một công trình.

Will be done:
- Thay `Department Surfaces` bằng hàng đợi công việc theo mức độ hành động, mặc định `Cần xử lý`.
- Tách rõ `Chờ duyệt`, `Bị chặn`, `Chưa đến lượt`, `Hoàn tất`, `Đã bỏ qua` và vẫn giữ `Tất cả` để truy vết.
- Bổ sung tìm kiếm, lọc giai đoạn, người/vai trò phụ trách, trạng thái và sắp xếp ưu tiên/workflow/hạn.
- Dùng bảng dày trên desktop và card gọn trên mobile; giữ một CTA `Mở` và điều hướng project-scoped hiện hữu.
- Dùng toàn bộ `work_items` từ `/projects/:id/operations`, không dùng danh sách con tối đa 10 item của `department_surfaces` làm nguồn lọc.
- Giữ nguyên backend contract, RBAC, transition, approval, evidence, workflow form và lịch sử cancelled read-only.
- Cập nhật central/frontend/workflow mappings trong cùng lượt thay đổi.
- Chạy `cd frontend && npm run build` và `git diff --check`; browser verification bỏ qua vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `OK triển khai`.
- Confirmed the existing operations response already contains the status, node, assignee, role, due date, form and evidence fields required by the redesign.
- Confirmed `department_surfaces.work_items` is capped at 10 records per department and cannot be used as the complete filtered list.
- Confirmed no backend API or shared frontend type contract change is required.
- Added `ProjectWorkTab.tsx` and replaced the system-facing `Department Surfaces` layout with an operational project work queue.
- Added actionable/waiting/blocked/future/completed/skipped/all quick filters, accent-insensitive search, stage/owner-role/status filters and priority/workflow/due sorting.
- Added a 20-row dense desktop table, responsive mobile cards and project-scoped `Mở` navigation without adding transition controls to the list.
- Presenter now prioritizes direct assignee, then required roles, then department fallback; `Chưa phân công` appears only when those sources are absent.
- Updated central, frontend UI and workflow orchestration maps with the new Work-tab ownership and complete-list invariant.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Project Workflow Execution Workspace Redesign

Status: complete

Mục đích chính:
- Giúp người điều hành biết ngay workflow đang ở giai đoạn nào, việc nào cần xử lý tiếp theo và mở đúng đầu việc với ít thao tác nhất.

Will be done:
- Thay layout Workflow tab hai cột gần cân bằng bằng summary điều hành, stage rail 280px và vùng công việc chiếm phần lớn không gian.
- Hiển thị tiến độ toàn workflow, giai đoạn hiện tại, số việc cần xử lý/chờ duyệt/bị chặn và CTA duy nhất `Mở việc tiếp theo` theo thứ tự blocker -> chờ duyệt -> việc sẵn sàng.
- Đổi danh sách work item trong node sang bảng/row dày dữ liệu ở desktop và card gọn ở mobile.
- Mặc định filter `Cần xử lý`; giữ `Tất cả` để truy vết lịch sử completed/skipped.
- Bổ sung stage selector responsive, loading/empty/filter-empty/skipped state và giữ cancelled branch ở chế độ chỉ đọc.
- Không đổi backend API, work-item transition, approval, workflow form, evidence hay orchestration invariant.
- Cập nhật frontend UI map và workflow orchestration domain map trong cùng lượt thay đổi.
- Chạy `cd frontend && npm run build` và `git diff --check`; browser verification tiếp tục bỏ qua vì người dùng không yêu cầu.

Done:
- User approved the review package and generated visual mockup with `OK, triển khai.`
- Confirmed owner surface is `frontend/src/modules/projects/ProjectDetailPage.tsx` under `/projects/:id`.
- Confirmed the existing `/projects/:id/operations` response already provides timeline, work-item status, owner, due date, form and evidence fields required by the redesign.
- Confirmed no backend or shared frontend type contract change is required.
- Replaced the balanced timeline/work-card layout with a workflow execution summary, 280px desktop stage rail and responsive stage selector.
- Scoped summary counts and next-work selection to the current runtime node so future pending work does not inflate the actionable count.
- Added next-work priority blocker -> waiting approval/review -> other current-node open work, with one primary CTA inside the Workflow tab.
- Replaced node work cards with a dense desktop work table and compact mobile cards; retained owner, due date, form requirement, evidence count, status and blocker reason.
- Changed active-node default filter to `Cần xử lý`; skipped/cancelled nodes automatically open `Đã bỏ qua` read-only history; `Tất cả` remains available.
- Updated central, frontend UI and workflow orchestration maps for the new execution workspace behavior.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Pipeline Builder Back Navigation Target

Status: complete

Mục đích chính:
- Make the Pipeline Builder `[← Quay lại]` control return to the `Công trình` list as a workflow navigation action, not as a browser-history back action.

Will be done:
- Change Pipeline Builder back handling from `navigate(-1)` history behavior to a fixed `/projects` navigation target.
- Keep the visible button label `Quay lại`.
- Change the button tooltip to `Về trang Công trình`.
- Update the frontend UI map to record that Pipeline Builder sidebar back navigation targets `/projects`.
- Keep backend APIs, template publish/clone/edit behavior, canvas/diagram interactions, workflow gates, and runtime copies unchanged.
- Run `cd frontend && npm run build` and `git diff --check`; browser verification remains skipped unless explicitly requested.

Done:
- User approved the review package with `OK triển khai`.
- Re-confirmed current code uses `window.history.state?.idx` and `navigate(-1)`, which behaves like browser back.
- Re-confirmed `/projects` is the guarded project list route and current expected target.
- Changed Pipeline Builder back handling to fixed `/projects` navigation.
- Changed the button tooltip to `Về trang Công trình`.
- Updated `frontend-ui-utility-map.md` to record that Pipeline Builder sidebar back navigation targets `/projects`.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Official Workflow Text Correction — Trao đổi

Status: complete

Mục đích chính:
- Sửa nhãn `Trảo đổi` thành `Trao đổi` ở mọi bề mặt dữ liệu workflow đang hiển thị cho người dùng, không thay đổi trạng thái hay lịch sử nghiệp vụ.

Will be done:
- Thêm migration V57 sửa ba Step template và các snapshot pipeline-run/work-item/workflow-form instance tương ứng.
- Đồng bộ ba nhãn seed để lần seed sau không ghi đè lại bản sửa.
- Bổ sung Golden validator chống tái phát, cập nhật mapping và chạy migration/check truy vấn cục bộ; không chạy `make seed` vì lệnh reset dữ liệu local.

Done:
- User approved the review package with `Approve`.
- Confirmed the local database has 3 affected template steps, 9 run steps, 9 work items and 6 workflow-form instances.
- Confirmed V40 is already applied but `seed.sql` runs afterward and upserts the three incorrect labels.
- Added V57 to correct the three labels in template, pipeline-run, work-item and workflow-form snapshots without changing statuses, answers, approvals, audit records or PDF artifacts.
- Corrected the three seed labels and added Golden validator coverage that rejects any remaining `Trảo đổi` label in those four data surfaces.
- Applied V57 through the backend Docker custom migration runner: one migration applied and 56 skipped.
- Verified locally: template/run/work-item/form-instance typo counts are all zero; all three configured Step labels now start with `Trao đổi`.
- Validation passed: `make migrate-check`, `make seed-validate` (`GOLDEN_SAMPLE_VALIDATION_PASSED`) and `git diff --check`. `make seed` was intentionally skipped because it resets local data.

## Published Pipeline Node Inspector

Status: complete

Mục đích chính:
- Cho phép người dùng xem đầy đủ Cấu hình Node của revision workflow đã phát hành mà không mở bất kỳ đường chỉnh sửa nào.

Will be done:
- Render Node Inspector khi một node được chọn ở template `published`, gắn nhãn chỉ xem và giữ nút đóng.
- Khóa các trường cấu hình/approval policy và ẩn toàn bộ thao tác lưu, thêm, sửa, xóa, kéo sắp xếp step hoặc mở dialog chỉnh sửa.
- Bổ sung regression check cho hợp đồng UI này, cập nhật mapping Pipeline Builder và chạy frontend validation không dùng browser.
- Mở từng Checklist / Step ở revision đã phát hành bằng dialog chỉ xem, hiển thị đúng cấu hình Step và Workflow Form hiện hữu.

Done:
- User approved the review package with `Approve`.
- Confirmed the node-detail toggle sets `detailsNodeId`, but `PipelineBuilderPage` only renders Node Inspector when `!isReadOnly`.
- Confirmed backend `ensureDraftTemplate` continues to reject every node/step/edge write for a published revision; no backend contract, migration or runtime pipeline behavior is in scope.
- Published revisions now render the selected Node Inspector with a `Đã phát hành · Chỉ xem` label and an explicit immutability notice.
- Disabled every node/approval control; removed save, step add/edit/delete and step-drag affordances; blocked step edit-dialog display and guarded mutation callbacks.
- Updated central and frontend Pipeline Builder mappings and added a source-level regression check for the read-only Node Inspector contract.
- Validation passed: `cd frontend && npm run check:pipeline-builder` and `git diff --check`. Browser verification was skipped because it was not explicitly requested.
- User approved the Step detail review package with `Approve`.
- Each Step card in a published revision now opens by click, Enter or Space and exposes an explicit `Xem` affordance.
- Reused the Step dialog in `view` mode: it shows the exact stored name, type, roles, form version and required-field summary, disables all controls and offers only `Đóng`.
- Updated the Pipeline Builder regression check and the central/frontend maps for the read-only Step-detail contract.
- Validation passed: `cd frontend && npm run check:pipeline-builder` and `git diff --check`. Browser verification was skipped because it was not explicitly requested.

## Workflow Overall Progress Excludes Skipped Branches

Status: complete

Mục đích chính:
- Tiến độ chung phản ánh nhánh workflow thực tế đã thực hiện, đồng thời công khai số step bị bỏ qua thay vì làm project hoàn tất chỉ đạt 92%.

Will be done:
- Loại runtime step `skipped`/`cancelled` khỏi mẫu số tiến độ thực hiện.
- Hiển thị riêng số step thực hiện và số step đã bỏ qua.
- Giữ tổng coverage template trong dữ liệu workflow, không giả các step bỏ qua thành đã hoàn tất.
- Cập nhật frontend mapping và chạy frontend build, backend regression, agent validation, diff check.

Done:
- Screenshot and source confirmed the old calculation used all 52 runtime steps as denominator, while only 48 executed steps entered the completed numerator.
- User approved `100% · 48/48 step thực hiện · 4 đã bỏ qua` with `OK triển khai`.
- Overall sidebar progress now filters skipped/cancelled steps out of the execution denominator before calculating completion.
- Sidebar displays executable completion and separately discloses skipped count plus total workflow coverage.
- Updated frontend mapping with the execution-progress denominator invariant.
- Validation passed: frontend production build, backend core regression tests, agent artifact validation and `git diff --check`. Browser verification was skipped because it was not explicitly requested.

## Workflow Sidebar Submitted Step Progress

Status: complete

Mục đích chính:
- Đồng bộ bộ đếm và thanh tiến độ của cây workflow với trạng thái hiển thị từng work item, để node có năm step đã thỏa gate hiển thị `5/5` thay vì `0/5`.

Will be done:
- Coi runtime step `submitted` và `completed` là đã hoàn tất khi tính sidebar progress.
- Ưu tiên `completed_steps` và `progress_percent` từ backend timeline khi payload có summary.
- Không tính `skipped` hoặc `cancelled` vào tiến độ thực hiện.
- Đồng bộ icon fallback của step `submitted` với trạng thái hoàn tất.
- Cập nhật frontend mapping và chạy frontend build, backend regression, agent validation và diff check.

Done:
- User confirmed the screenshot expectation: five completed work items in node `Họp báo cáo khảo sát` must display `5/5`.
- Root cause confirmed in `ProjectWorkspaceLayout`: row status prefers `work_item_status=completed`, while node counter reads runtime `step.status=submitted` through a helper that only accepted literal `completed`.
- User approved the review package with `OK triển khai`.
- Updated sidebar completion semantics so `submitted` and `completed` both count as satisfied steps; skipped/cancelled remain outside completed progress.
- Node counters now prefer backend `completed_steps` and `progress_percent` with bounded fallback calculation from runtime steps.
- Updated fallback step icon semantics and frontend mapping for the shared workflow-sidebar contract.
- Validation passed: frontend production build, backend core regression tests, agent artifact validation and `git diff --check`. Browser verification was skipped because it was not explicitly requested.

## Project Workflow Work Item History Visibility

Status: complete

Mục đích chính:
- Giữ toàn bộ work item theo đúng node workflow ngay trên Project Detail để người dùng xem được việc đang xử lý, đã hoàn tất và đã bỏ qua mà không phải chuyển sang UI khác.

Will be done:
- Project Operations trả cả work item terminal cho màn hình lịch sử, nhưng các API danh sách tác nghiệp và action gate vẫn không đưa work item đã hủy vào hàng đợi xử lý.
- Cho phép mở work item đã bỏ qua ở chế độ read-only để xem activity và metadata lịch sử.
- Timeline đếm runtime step `submitted` là đã hoàn thành và tách số bước `skipped`.
- Panel workflow mặc định hiển thị tất cả, có tìm kiếm và bộ lọc Cần xử lý/Chờ duyệt/Bị chặn/Hoàn tất/Đã bỏ qua.
- Badge Công việc dùng tổng số thay vì chỉ số đang mở; empty state phân biệt chưa phát sinh, không khớp bộ lọc và node đã bỏ qua.
- Bổ sung test, cập nhật mapping, chạy backend test, frontend build và diff check.

Done:
- User approved the review package and Mockup as Text with `OK triển khai theo mockup này`.
- Confirmed frontend `ProjectDetailPage` filters out `completed` and `cancelled` before rendering the workflow panel and tab count.
- Confirmed Project Operations currently uses the operational list query that excludes cancelled records.
- Confirmed Golden Sample runtime nodes are completed while executed runtime steps use terminal status `submitted`, causing the existing timeline aggregation to report `0/N`.
- Added a Project Operations history query that includes completed/cancelled work items without changing the operational `/projects/:id/work-items` queue contract.
- Kept cancelled work items viewable under the original project/RBAC scope while `CanActOnWorkItem` continues to reject them.
- Updated workflow timeline aggregation so `submitted` and `completed` steps count as complete, skipped/cancelled steps are reported separately and skipped nodes remain at 0% execution progress.
- Rebuilt the Project Detail workflow panel with default `Tất cả`, total count, search, status filters, explicit skipped-branch context, terminal compact cards and status-specific empty states.
- Changed the Project Detail `Công việc` tab count from open-only to all visible project work items and excluded skipped nodes from automatic current-node selection.
- Added a read-only skipped-work-item detail surface that exposes history without primary business actions or activity composer.
- Added focused timeline regression coverage and updated central, backend, frontend, workflow orchestration and project-planning maps.
- Validation passed: backend core/full tests, frontend production build, agent artifact validation and `git diff --check`. Browser verification was skipped because it was not explicitly requested.

## Golden Live Operational Project

Status: complete

Mục đích chính:
- Cung cấp một project Golden chưa hoàn tất để người dùng tự vận hành từng node, step, form, approval và nghiệp vụ từ khảo sát đến bàn giao trên đúng engine-v2 contract.

Will be done:
- Sửa approval policy của official template trong seed về đúng schema V46 (`roles`, `quorum`, `requester_cannot_approve`, `comment_required`, `signature_required`) và mode `any`.
- Bổ sung demo users/staff cho Mua hàng, QA/QC, Đội thi công, Thầu phụ và Chủ đầu tư.
- Tạo `GOLDEN-LIVE-001` ở trạng thái planning 0%, node Khảo sát in-progress, sáu work item ready, các node/step còn lại pending và 51 workflow form draft.
- Không seed answer, evidence, approval hoặc domain ledger cho Live project để người dùng tự tạo qua UI/API.
- Thêm runbook theo node/vai trò/gate và validator đảm bảo project thực sự chưa được thao tác.
- Seed database sạch và local, chạy fixture/database validator, backend tests, migration check và frontend build.

Done:
- User approved the review package with `OK triển khai Golden Live`.
- Confirmed `StartPipeline` creates the first node `in_progress`, first-node work items `ready`, later work items `pending`, all runtime steps `pending`, and exact draft form instances.
- Confirmed current Golden seed approval policy uses incompatible keys and would fail normalization/request-approval for a real engine-v2 run.
- Confirmed the current demo accounts do not cover procurement, QA/QC, construction team, subcontractor or client roles.
- Added five demo users/staff/role assignments: procurement, QA/QC, construction team, subcontractor and client; all use the documented local password and are members of Golden Live.
- Corrected official seeded approval policies to the exact V46 contract with mode `any`, role array, quorum, requester separation, comment and signature flags.
- Restored the 15 contract/quality/handover permissions and role-permission matrix that base seed reset previously removed after migrations.
- Added `backend/seed/golden_live_project.sql`, mirroring `StartPipeline` state: one in-progress survey node, eleven pending nodes, 52 pending run steps, six ready work items, 46 pending work items and 51 untouched draft form instances.
- Golden Live intentionally has zero answers, evidence, approvals, documents, BOQ, quotations, procurement, contracts, stock, quality, acceptance, finance, invoice or handover records after seed.
- Added `backend/seed/GOLDEN_LIVE_RUNBOOK.md` with project scenario, role accounts, node-by-node operating sequence, expected blockers and reset instructions.
- Extended `backend/Makefile` to seed Golden Live before validation and expanded `validate_golden_project.sql` with runtime, untouched-data, policy-contract, RBAC and role-account assertions.
- Updated module-memory, backend, workflow, project-planning and admin-governance maps with Golden Live ownership and invariants.
- Rebuilt a separate V1-V56 PostgreSQL database and ran base + completed Golden + Golden Live + validator successfully.
- Applied `make seed` successfully to the running local database; Golden Live is ready for use at project id `30000000-0000-0000-0000-000000000022`.
- Final validation passed: Golden fixtures, Golden database validator, backend tests, 56 migration checks and frontend production build. Browser verification was skipped because it was not explicitly requested.

## Golden Sample 100% Official Workflow

Status: complete

Mục đích chính:
- Cung cấp một bộ dữ liệu chuẩn để nhìn xuyên suốt toàn bộ Consflow như một công trình thật, dùng đúng template chính thức `40000000-0000-0000-0000-000000000020` và không bỏ sót node, step, form hay ledger nghiệp vụ.

Will be done:
- Tạo flagship `GOLDEN-REAL-001` chạy nhánh Có thiết kế đến hoàn thành/bàn giao/đóng tài chính.
- Tạo companion `GOLDEN-NO-DESIGN-001` chạy nhánh Không thiết kế để bao phủ nhánh loại trừ mà không làm sai workflow thực tế.
- Bao phủ 12 node, 52 step, 51 workflow form, evidence, approvals, signatures, PDF và activity/audit.
- Seed đầy đủ BOQ, báo giá/so sánh, procurement, hợp đồng/version/milestone/variation/retention, kho/vật tư, nhật ký thi công, ITP/RFI/NCR/recurring, nghiệm thu/rework, AP/AR, hóa đơn/thanh toán và handover checklist.
- Thêm validator định lượng cho coverage, quan hệ nguồn, cân bằng kho/tài chính và trạng thái bàn giao cuối.
- Chạy seed trên PostgreSQL đã migrate V56, validator, backend tests, frontend build và diff checks; browser verification chỉ chạy khi được yêu cầu riêng.

Done:
- User approved the review package with `OK triển khai Golden Sample`.
- Confirmed the existing `CF-FULL-001` is tied to demo template `...0010` and lacks V47-V56 commercial/contract/quality coverage.
- Confirmed the official template has 12 nodes, 52 steps, 51 required forms and one evidence-only step, including mutually exclusive design/no-design branches.
- Loaded module-memory, backend/database workflow, project planning, workflow orchestration, commercial/procurement, construction, materials, quality/handover, finance and impact-analysis guidance.
- Chosen a flagship plus companion structure so 100% branch coverage remains truthful to a real project execution.
- Added `backend/seed/golden_project.sql` to rebuild the official V56 form/surface bindings after a local reset, then generate two complete runtime snapshots with 24 total nodes, 104 steps/work items, 92 submitted form instances, 94 accepted evidence records, signatures, approvals, activity and audit.
- Added `backend/seed/golden_project_domains.sql` with six final-locked BOQ lines, three suppliers/eighteen quote items, six selections, two received procurement requests, two incoming acceptances, explicit receive/issue/return stock ledgers, six closed site balances and three construction reports.
- Added `backend/seed/golden_project_closure.sql` with ITP Rev.01/Rev.02, pass/fail/reinspection RFIs, closed NCR, completed rework, three recurring QA occurrences, final pass acceptance and subcontractor/client reconciliation.
- Added `backend/seed/golden_project_finance.sql` with three active contracts, four signed versions, incorporated variation, six paid milestones, three released retentions, four AP, five AR, nine terminal payments/receipts, schedules, settled advances, invoice allocations, material closure and a handed-over 8/8 package.
- Added `backend/seed/validate_golden_project.sql`; `make seed-validate` reports `GOLDEN_SAMPLE_VALIDATION_PASSED` and exact project counts.
- Added eight rendered Golden business PDFs, copied invoice fixtures, a checksum manifest and `make seed-fixtures-check`; visual QA found no clipping, overlap or unreadable Vietnamese text.
- Fixed base seed drift exposed by a clean V56 database: reset now cascades to later domain tables; pipeline templates include required lineage/engine/hash/publish fields; legacy handover rows include `handover_code` and current statuses.
- Updated `backend/Makefile` so `make seed` applies base + Golden layers, validates the suite and copies upload fixtures; applied it successfully to the running local database.
- Rebuilt a separate clean PostgreSQL database through all V1-V56 SQL migrations, then ran every seed layer and validator successfully.
- Updated central/backend/domain maps with Golden Sample ownership, IDs, coverage and validation commands.
- Ran backend tests, migration check for 56 migrations, frontend production build, PDF fixture validation and Golden database validation successfully. Browser verification was skipped because it was not explicitly requested.

## Product-grade Workflow Engine And Business Forms

Status: complete

Mục đích chính:
- Biến pipeline thành hợp đồng thực thi nghiệp vụ: template đã phát hành bất biến, project chỉ chạy đúng phiên bản đã phát hành, và mọi bước nghiệp vụ phải đạt đúng gate trước khi quy trình đi tiếp.

Will be done:
- S0: lập baseline tương thích cho template `40000000-0000-0000-0000-000000000020`, các run hiện hữu và dữ liệu form lịch sử; không sửa submitted/signed/PDF history.
- S1: thêm template lineage, revision, engine version, definition hash, published actor/time; khóa mọi đường ghi vào template đã phát hành.
- S1: mở rộng publish validation cho graph, form version, condition config và permission; project chỉ được start bằng template published.
- S2-S5: runtime snapshot, gate engine, state transition/branch, hard automation, approval và capability-driven UI.
- B5: hoàn tất UAT, regression hardening và sign-off toàn hệ thống theo các vertical slice đã duyệt.
- Mỗi slice cập nhật migration, backend/frontend contract, mapping, automated tests và UAT evidence tương ứng.

Done:
- User approved the complete review package with `OK triển khai theo review package này.`
- Loaded the mandatory module memory, backend/frontend maps, workflow orchestration, project planning, commercial/procurement, construction operations, quality/acceptance/handover, accounting/finance, administration/governance, reporting and impact-analysis domain guidance.
- Confirmed from current code that published pipeline templates remain writable through template/node/step/edge endpoints and `StartPipeline` does not reject a selected draft template.
- Confirmed the current publish gate only validates required workflow-form mappings.
- Began S0-S1 with an additive migration and shared immutable-template/publish-validation design; existing run/form history remains out of mutation scope.
- Added V43 template lifecycle schema: family/revision/source lineage, engine version, definition hash, publish actor/time, plus run engine/hash snapshots.
- Made template creation and revision cloning transactional; clone now creates the next draft revision in the same family.
- Guarded every template/node/step/edge structural write so published revisions are immutable; deleting a published revision archives it.
- Expanded publish validation to graph root/sink/reachability/cycle, required published forms, answer-condition field/value validity, and permission-key existence.
- Updated Pipeline Builder with published read-only mode, `Tạo revision`, local graph validation, and authoring controls for `workflow_form_answer_equals.field_key/value`.
- Filtered Project template choices to published revisions and made backend `StartPipeline` reject draft/archived templates.
- Added V44 runtime completion gates: run-step snapshots of exact required form version, engine-v2 active-node enforcement, exact form submission, complete work items/steps, node approval, and blocked automation checks before advance.
- Removed synthetic legacy checklist completion from engine v2 while preserving compatibility for engine-v1 runs.
- Added direct non-selected branch skipping/cancellation and excluded skipped nodes from runtime progress.
- Added V45 stable work-item surface binding with template/run-step snapshots of `surface_kind` and `domain_action_key`.
- Bound official steps 38/39/40/46 explicitly: subcontractor checklist reconciliation, incoming material acceptance, quantity dossier workflow form, and owner/client reconciliation respectively; this removes the prior title-based misrouting.
- Updated work-item primary-surface resolution to prefer the backend snapshot and retain text heuristics only for legacy/non-pipeline records.
- Added V46 hard automation and structured approval-policy schema, including request policy snapshots, current approval stage, and actor role labels on approval actions.
- Replaced engine-v2 display-name dispatch with stable terminal `domain_action_key` mappings for BOQ/procurement, input/output finance scaffolds, and handover readiness; engine v1 retains legacy behavior.
- Required engine-v2 hard automation to succeed before work-item/legacy-step completion. Blocked/failed outcomes are stored with the same idempotency key and returned as actionable validation blockers instead of being ignored after completion.
- Added work-item automation-event payload and requirement-panel status so users see the exact hard action and blocker; retrying completion safely retries the same idempotent action.
- Implemented structured approval evaluation for `any`, `all`, `quorum`, and ordered role stages with requester separation, required comments, duplicate-approval prevention, row locking, stage progression, and audit.
- Added Pipeline Builder approval-policy controls for mode, ordered roles, quorum, requester separation, and required comments.
- Completed B1 supplier quotation and procurement vertical slice with V47: supplier quotation headers/items, qualification state, immutable submitted quote items, auditable quote selection history, BOQ selected-quote/waiver source, procurement quotation snapshots, supplier/PO/delivery metadata, and supporting indexes/constraints.
- Added project supplier-quotation list/create/detail/update/submit/qualify APIs, BOQ-line comparison API, and qualified-quote selection API. Selection requires approved BOQ + norm lock, validates quote/item compliance and validity, locks final price atomically, writes the immutable BOQ price event, and records audit metadata.
- Manual BOQ final-price lock now requires an explicit quotation-waiver reason. BOQ submit/approve now use transactional state transitions with audit and no longer approve draft rows directly.
- Procurement generation now requires a selected quote or waiver source, groups BOQ lines into one request per supplier, and snapshots quote item, supplier, BOQ price revision/event, quoted price, and variance while retaining the legacy top-level response shape.
- Procurement create/update now validates project, title, budget state, partner catalog, and delivery date and writes audit. Structured approval requests can no longer be silently forced approved by the procurement action endpoint.
- Procurement ordering now requires supplier, PO/order reference, and expected delivery date. Receive derives `partially_received`/`received` only from submitted incoming-acceptance quantities; cumulative accepted quantity cannot exceed requested quantity, and no implicit stock movement is created.
- Added the BOQ `Báo giá nhà cung cấp & so sánh thương mại` workspace with quote editor, supplier catalog, quote-item grid, technical/commercial assessment, comparison table, decision reason, loading/empty/error states, and responsive horizontal tables.
- Reworked the Procurement page around generated requests and real lifecycle actions, with supplier/order metadata, source-price detail, incoming-acceptance progress, status-specific actions, and shared dialogs instead of one-off drawers.
- Added DB-backed integration coverage for quote submission/qualification/selection, BOQ final lock, procurement supplier/quote snapshot, and the legacy manual-waiver generation path.
- Applied V47 successfully through the running backend migration runner and verified schema migration version `47` in PostgreSQL.
- Ran B1 DB-backed integration tests inside the backend Docker network: both targeted scenarios pass.
- Ran `GOCACHE=$PWD/.gocache go test ./...` in `backend`: pass.
- Ran `make migrate-check`: 47 migrations checked successfully.
- Ran `npm run build` in `frontend`: pass; only the existing large chunk warning remains.
- Browser verification for B1 was skipped because it was not explicitly requested.
- Completed B2 contract lifecycle with V48-V50: project-scoped contracts, signed immutable versions, payment milestones, variation/appendix workflow, retention ledger/release history, dedicated contract RBAC, cascade-safe history triggers, and official-workflow surface binding.
- Added contract lifecycle API and service/repository transitions: create/update draft contract, create version, submit/review/approve/sign, activate signed current version, create/qualify milestone, materialize AP/AR, create/submit/approve/reject/incorporate variation, and release retention into traceable AP/AR.
- Kept `projects.contract_amount` as a compatibility snapshot updated only from the active signed main-client contract; signed `contract_versions` and `contracts.current_amount` are now the commercial source of truth.
- Version definition hashes cover amount, scope, commercial/payment terms, dates and document reference. Signed versions cannot be updated; incorporated/rejected variations cannot be updated. Project cascade cleanup remains possible through V49 without exposing a direct history-delete API.
- Milestone obligation creation requires an active contract and eligible milestone. Acceptance-required milestones require an approved pass/conditional-pass acceptance. Obligation creation is idempotent and snapshots contract/milestone source into payable/receivable records.
- Milestone retention creates a held ledger entry. Retention release is warranty-gated, cannot exceed outstanding retention, appends a numbered release row, opens a traceable AP/AR record, and updates held/partial/released state with audit.
- Approved variation incorporation creates the next draft version with revised amount, schedule and scope. It does not silently mutate the active signed version; the new revision must pass review/approve/sign/activate again.
- Added `frontend/src/modules/contracts/ContractsPage.tsx` with project-scoped contract list, capability-gated lifecycle actions, version hash/status table, milestone/AP-AR actions, variation revision controls, retention release dialog, loading/empty/error states and responsive tables.
- Added `/projects/:id/contracts` to router and project workspace navigation under `Thương mại & Vật tư`; added contract domain types and Vietnamese status presentation.
- Added `surface_kind='contract'` to workflow surface contracts. Official template steps 24 and 29-31/33 open the Contract Workspace while preserving the step business form/PDF as a supporting tab; step 32 remains the structured approval surface. Only pending runtime steps are remapped.
- Applied V48, V49 and V50 successfully through the running backend migration runner; PostgreSQL confirms five official template steps on the contract surface and seven contract permission keys.
- Added and ran DB-backed integration coverage for contract create → version review/approve/sign → activation → milestone → receivable/retention → variation/revision → re-sign/re-activate → retention release. The scenario passes inside the Docker network and cleanup is cascade-safe.
- Ran backend core tests, `make migrate-check` for 50 migrations, and frontend production build successfully; only the existing large chunk warning remains.
- Browser verification for B2 was skipped because it was not explicitly requested.
- Completed B3 QA/QC and construction execution with V51-V53: revisioned ITP plans/items, RFI inspection lifecycle, NCR root-cause/corrective/reinspection lifecycle, recurring construction occurrences, dedicated quality RBAC, official-workflow quality surfaces, and an idempotent construction daily-report source bridge.
- Added the project quality workspace API and transactional transitions for ITP create/revise/submit/approve/activate, RFI create/submit/schedule/inspect, NCR create/root-cause/corrective/reinspection/close, recurring plan generation and occurrence complete/skip.
- Rejected RFI inspection now creates a linked NCR and draft rework atomically. NCR closure requires project-owned corrective evidence, an accepted/conditional reinspection RFI, and completed linked rework; actor and audit history are preserved.
- Active/approved/superseded ITP definitions are immutable and hash-backed. Only one active revision exists per project/family; recurring occurrence generation is idempotent by plan/date and respects evidence gates.
- Added `frontend/src/modules/quality/QualityPage.tsx` with capability-gated ITP, RFI, NCR and recurring-work sections, including structured editors, lifecycle actions, loading/empty/error states, project-owned evidence selection and responsive tables.
- Added `/projects/:id/quality`, project workspace navigation, quality frontend contracts/status labels, and the `quality` work-item primary surface. Official workflow steps 36-37 now open the quality workspace while completed historical runtime steps remain unchanged.
- Submitted `builtin_construction_daily_report` forms now sync idempotently into `operation_daily_reports` with workflow-form/work-item/evidence linkage, structured answer snapshot, progress, work, crew, material and issue summaries; the existing next-plan work-item generation remains intact.
- Applied V51, V52 and V53 with the custom migration runner. `make migrate-check` validates all 53 migrations.
- Added DB-backed lifecycle coverage for ITP -> failed RFI -> NCR/rework -> corrective evidence -> accepted reinspection -> close and recurring occurrence completion. Added daily-report bridge assertions covering the complete structured mapping.
- Ran targeted Docker/PostgreSQL integration tests for B3 and both planning/construction daily-report next-plan flows: all pass.
- Ran backend core tests and the frontend production build successfully during B3 implementation; the final full-suite validation remains part of the B3 completion gate below.
- Browser verification for B3 was skipped because it was not explicitly requested.
- Completed B4 financial creation and handover package hardening with V54-V56: typed/server-owned AP/AR creation, draft-only generic payment creation, actor audit, handover package lifecycle, evidence/document checklist, readiness snapshot, new verification permission, official finance/handover workflow surfaces, and historical checklist backfill.
- Manual AP/AR APIs no longer accept client-controlled paid/received/remaining/status fields. Payables derive remaining from gross minus advance/deduction; receivables derive remaining from gross; all new records start draft with zero paid/received and audited creator.
- Generic payment/receipt creation validates project/code/type/method/positive amount, ignores client approval/status fields, and always starts draft. Dedicated payable/receivable payment endpoints continue to own allocation and reconciliation updates.
- Added exact final-acceptance source locking, handover code, workflow-form source, checklist items, accepted-evidence/active-document verification, submit/check/ready/blocked/handed-over lifecycle, checked/completed actor timestamps, and immutable completion snapshot semantics.
- Readiness now checks the linked final acceptance, open issues, NCRs, rework, material surplus, AP/AR remaining balances, pending payments and all required checklist items. Complete always reruns readiness immediately before writing `handed_over`.
- Rebuilt `HandoverPage.tsx` as a dense project workspace with package summary, exact acceptance selection, checklist evidence/document verification, blocker display, capability-gated submit/check/complete actions and required completion comment. Removed self-attested readiness checkboxes.
- Removed client-editable derived balance/status controls from AP/AR and generic payment create forms. Server ownership is explained inline.
- Added finance and handover primary work-item surfaces. Official steps 42, 44-45 and 48-51 open the project debt/finance workspace; step 52 opens the handover package while preserving each workflow form/PDF in the support tab. Only pending runtime steps are remapped.
- Applied V54-V56 with the custom migration runner; V54 backfills legacy `checking` to `submitted`, and V56 gives every historical non-deleted handover the same eight required checklist items.
- DB-backed regression chain passes for input/output invoice PDF, payable/receivable approval, payment schedule, advance, partial payment/receipt allocation, reconciliation ledger/aging, derived-state hardening, blocked handover and acceptance -> verified package -> readiness -> completed handover.
- Frontend production build passes after the B4 workspace/forms/surface changes; only the existing large chunk warning remains.
- Browser verification for B4 was skipped because it was not explicitly requested.
- Completed B5 full impact audit and DB-backed UAT hardening across template/runtime, forms/signatures/PDF, activity/RBAC, BOQ/procurement, contract, stock/materials, QA/NCR/rework, invoice/payment/reconciliation and handover.
- The first full DB suite exposed two production SQL defects that focused tests had not reached: runtime queries ordered `pipeline_run_steps` by a non-existent `created_at`, and site-material balance SQL reused an untyped quantity placeholder across numeric/integer expressions. Both root causes were fixed in repository code.
- Explicit UUID/text casts were added to activity mention insertion SQL after the full suite exposed PostgreSQL parameter inference ambiguity on role mentions.
- UAT fixtures were migrated to the current engine-v2 contract: template lineage is mandatory, node completion fixtures satisfy exact forms/steps/approvals, and official hard automation is selected by `domain_action_key` instead of legacy display-name dispatch.
- Updated checklist UAT to the currently published structured planning-checklist schema and configured test-local upload storage for signature artifact verification.
- Corrected the material ledger regression expectation: a partial return with positive remaining surplus stays `partial`, not `handled`.
- Re-ran every previously failing regression group: all pass.
- Re-ran the entire compiled core test suite with `CONSFLOW_TEST_DATABASE_URL` against PostgreSQL in Docker: `PASS`.
- Final backend `go test ./...`, frontend production build, `make migrate-check` for 56 migrations, agent artifact validation and `git diff --check` all pass. Browser verification was not run because it was not explicitly requested.
- Applied V43, V44, and V45 successfully through the running backend container migration runner.
- Applied V46 successfully through the running backend container migration runner.
- Ran `make migrate-check`: 46 migrations checked successfully.
- Ran `GOCACHE=$PWD/.gocache go test ./...` in `backend`: pass.
- Ran `npm run build` in `frontend`: pass; only the existing large chunk warning remains.
- Browser verification was skipped because it was not explicitly requested.

## Pipeline Builder Back Navigation

Status: complete

Mục đích chính:
- Add a clear `[← Quay lại]` control to the full-screen Pipeline Builder workspace so users can return to the previous page without using the browser chrome.

Will be done:
- Reuse the existing `TemplateSidebar` back-button surface instead of adding a new UI primitive.
- Change the button label from `Back` to `Quay lại`.
- Route the button through browser/app history with a safe `/` fallback when no internal history entry exists.
- Keep backend APIs, template publish/clone/edit behavior, canvas/diagram interactions, workflow gates, and runtime copies unchanged.
- Update the frontend UI mapping because the Pipeline Builder surface behavior changes.
- Run `cd frontend && npm run build`; browser verification remains skipped unless explicitly requested.

Done:
- User approved the review package with `OK`.
- Re-read Consflow module memory, frontend workflow, workflow orchestration domain, central module map, frontend UI map, domain map, `AGENTS.md`, router, app shell, and Pipeline Builder owner files.
- Enabled the existing Pipeline Builder sidebar back-button surface.
- Changed the back button label from `Back` to `Quay lại` and added a Vietnamese title.
- Updated Pipeline Builder back behavior to use browser/app history with `/` as a fallback when no internal history entry exists.
- Updated `frontend-ui-utility-map.md` to record Pipeline Builder sidebar back navigation.
- Ran `cd frontend && npm run build`: pass; Vite reported the existing large chunk warning.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Mandatory Agent Mapping Bootstrap

Status: complete

Mục đích chính:
- Make Consflow agents and subagents load the routing/mapping memory as a mandatory first step for repository work, and keep that memory updated whenever code changes affect features, functions, utilities, helpers, UI, routes, contracts, tables, statuses, or domain invariants.

Will be done:
- Keep scope limited to agent artifacts, mapping references, and documentation; do not change backend or frontend runtime behavior.
- Add a mandatory mapping bootstrap rule to `AGENTS.md`, `consflow-module-memory`, subagent profiles, and agent docs.
- Add a mapping maintenance gate: any map-affecting code change must update the relevant mapping in the same work turn or state why no mapping update is required.
- Split detailed mapping into targeted reference files so agents load the central map first, then detail maps by keyword/domain instead of loading all inventory for every task.
- Add backend function/route/repository/service mapping guidance.
- Add frontend UI/component/helper/API mapping guidance.
- Add root-cause triage keywords and search recipes for faster diagnosis.
- Validate agent artifacts with `.agents/scripts/validate.sh` and targeted `rg` checks.

Done:
- User approved the review package with `Approve`.
- Re-read `AGENTS.md`, `consflow-module-memory`, `consflow-repo-architect`, `consflow-impact-analysis`, central module keyword map, subagent profiles, and agent docs.
- Confirmed current behavior: mapping is implicit/instruction-driven, not engine-level preloaded; runtime sync is not automatic.
- Added mandatory mapping bootstrap and mapping maintenance gate to `AGENTS.md`.
- Updated `consflow-module-memory` so agents must load the central map, then targeted backend/frontend/root-cause/domain maps before scans, diagnosis, review, planning, or edits.
- Added `backend-function-map.md` for backend route, handler, service, repository, helper, type, migration, and search ownership.
- Added `frontend-ui-utility-map.md` for frontend routes, UI surfaces, shared components, helpers, API usage, domain types, status labels, and pipeline diagram helpers.
- Added `root-cause-triage-map.md` for symptom-to-owner routing and focused `rg` recipes.
- Updated subagent profiles so explorer, backend worker, frontend worker, reviewer, and DB reviewer must perform mapping bootstrap and enforce mapping maintenance.
- Updated agent docs and skill README to describe the mandatory mapping policy.
- Ran `bash .agents/scripts/validate.sh`: pass.
- Ran targeted `rg` checks confirming mandatory bootstrap and mapping maintenance gate are present in subagent profiles and mapping docs.
- Ran `git diff --check`: pass.

## Project Operations Lightweight Workflow Sidebar Payload

Status: complete

Mục đích chính:
- Make project-scoped work item detail pages load the workflow sidebar quickly by requesting only the project summary, workflow timeline, run nodes/steps, and minimal work item data needed for sidebar navigation.

Will be done:
- Keep the default `/projects/:id/operations` dashboard contract unchanged.
- Add a lightweight `view=workflow-sidebar` variant for `/projects/:id/operations`.
- Preserve project operations visibility, project/work item scoping, runtime workflow gates, form/evidence/signature/PDF semantics, and audit/activity behavior.
- Batch pipeline run step loading so runtime nodes no longer trigger one step query per node.
- Update `ProjectWorkspaceLayout` to use the lightweight view only on project-scoped work item detail routes.
- Update frontend types for the lighter operations response shape.
- Add or adjust focused tests when a stable backend seam exists.
- Run focused backend validation and `cd frontend && npm run build` when feasible. Browser verification remains skipped unless explicitly requested.

Done:
- User approved the review package with `OK`.
- Re-read Consflow backend workflow, diagnose, workflow orchestration, module memory, AGENTS, route, handler, service, repository, frontend layout, router, and shared type code.
- Confirmed root cause: project-scoped work item detail pages fetch the full project operations dashboard response even though the detail content fetches `/work-items/:id` separately.
- Added `ProjectOperationsFilter` with a validated `view=workflow-sidebar` backend variant.
- Kept default `/projects/:id/operations` full dashboard response unchanged.
- Added lightweight project operations repository response with project summary, visibility, health counts, workflow timeline, pipeline run, summary, and sidebar work items only.
- Added minimal sidebar work-item query that preserves project/work item scoping and workflow form/evidence summary fields.
- Batched `GetPipelineRun` step loading into one query grouped by node, preserving `pipeline_run.nodes[].steps`.
- Updated `ProjectWorkspaceLayout` so project-scoped work item detail routes request `/projects/:id/operations?view=workflow-sidebar`; non-detail project workspace routes still request the full endpoint.
- Added `ProjectOperationsSidebarResponse` frontend type.
- Extended the DB-backed start-pipeline integration test to assert the lightweight operations response shape when `CONSFLOW_TEST_DATABASE_URL` is available.
- Ran `GOCACHE=$PWD/.gocache go test ./internal/core` in `backend`: pass.
- Ran `GOCACHE=$PWD/.gocache go test ./...` in `backend`: pass.
- Ran `npm run build` in `frontend`: pass; Vite reported only the existing large chunk warning.

## Work Item Detail Workflow Sidebar Collapse Scroll

Status: complete

Mục đích chính:
- Keep the work item detail workflow sidebar focused on the currently selected Checklist / Step by expanding only the active node by default and auto-scrolling to the active step.

Will be done:
- Add expand/collapse controls for workflow nodes in the project-scoped work item detail sidebar.
- Default to only the node containing the selected work item step being expanded.
- Collapse other nodes by default when the active work item changes.
- Auto-scroll the sidebar to the active step after expansion.
- Preserve project-scoped work item links, active state, fallback step data, backend APIs, workflow gates, form/evidence/signature/PDF behavior, and action semantics.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `OK triển khai.`
- Re-read Consflow frontend workflow, workflow orchestration, module memory, AGENTS, module keyword map, domain map, UI guardrails, router, shared domain types, and current `ProjectWorkspaceLayout` sidebar code.
- Added workflow node expand/collapse state in `frontend/src/modules/projects/ProjectWorkspaceLayout.tsx`.
- Defaulted the sidebar to expand only the node containing the active work item step, with other nodes collapsed on active work item changes.
- Added node header toggle buttons with expanded/collapsed chevrons and `aria-expanded`.
- Added active step ref and auto-scroll to center the selected step after the active node expands.
- Preserved project-scoped work item links, active state, fallback step data, backend APIs, workflow gates, forms, evidence, signatures, PDFs, and action semantics.
- Ran `cd frontend && npm run build` successfully; Vite reported only the existing large chunk warning.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Detail Workflow Sidebar Step Fallback

Status: complete

Mục đích chính:
- Ensure the work item detail left workflow sidebar shows real Checklist / Step rows, not only workflow nodes, when `pipeline_run.nodes[].steps` is empty in `/projects/:id/operations`.

Will be done:
- Keep using existing `/projects/:id/operations`; do not add backend endpoints or migrations.
- Build sidebar steps from `pipeline_run.nodes[].steps` when available.
- Fallback/merge steps from `operations.work_items` grouped by `run_node_id` and `run_step_id`.
- Preserve project-scoped work item detail links, active step state, workflow gates, forms, evidence, signatures, PDFs, and action semantics.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User confirmed implementation with `OK triển khai`.
- Re-read Consflow frontend workflow, workflow orchestration, module memory, AGENTS, domain map, module keyword map, and UI guardrails.
- Confirmed root cause: sidebar only read `pipeline_run.nodes[].steps`, while available `operations.work_items` already contains `run_node_id`, `run_step_id`, `run_node_name`, and `run_step_name`.
- Updated `frontend/src/modules/projects/ProjectWorkspaceLayout.tsx` to merge `pipeline_run.nodes[].steps` with fallback steps built from `operations.work_items`.
- Added stable step grouping by `run_node_id`, dedupe/merge by `run_step_id`, workflow-order sorting, and fallback node construction when needed.
- Preserved project-scoped work item links, active state, status/evidence/form chips, backend APIs, and runtime workflow semantics.
- Ran `cd frontend && npm run build` successfully; Vite reported only the existing large chunk warning.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Detail Workflow Sidebar Timeline

Status: complete

Mục đích chính:
- Replace the left `Cây nghiệp vụ` menu only on project-scoped work item detail pages with a workflow > Checklist / Step timeline so users can see progress and jump between runtime work items from the same project workspace.

Will be done:
- Keep project workspace navigation unchanged on non-work-item project pages.
- In `/projects/:id/work-items/:workItemId`, render a workflow timeline sidebar using existing `/projects/:id/operations` data.
- Show overall step progress and per-node progress.
- Show node > Checklist / Step rows with status, active state, evidence/form hints, and disabled state for steps without a `work_item_id`.
- Let clicking an enabled step open the corresponding project-scoped work item detail route on the right.
- Preserve backend APIs, workflow gates, form/evidence/signature/PDF behavior, query invalidation, and runtime workflow semantics.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `Approve`.
- Re-read Consflow frontend workflow, workflow orchestration, module memory, and UI guardrail skills.
- Confirmed `/projects/:id/operations` already provides `pipeline_run` and `workflow_timeline`; no backend or migration change is needed for this scope.
- Updated `frontend/src/modules/projects/ProjectWorkspaceLayout.tsx` so project-scoped work item detail pages render a `Workflow` sidebar instead of `Cây nghiệp vụ`.
- Kept the existing `Cây nghiệp vụ` sidebar unchanged for non-work-item project workspace pages.
- Added overall step progress, per-node progress, node > Checklist / Step rows, active state, disabled state for steps without `work_item_id`, and project-scoped links to work item detail pages.
- Preserved backend APIs, route contracts, workflow gates, form/evidence/signature/PDF behavior, and query invalidation semantics.
- Ran `cd frontend && npm run build` successfully; Vite reported only the existing large chunk warning.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Official Workflow Business Forms Product Grade

Status: complete

Mục đích chính:
- Turn official client workflow template `40000000-0000-0000-0000-000000000020` into a production-grade operational workflow where every Node > Step opens the correct business form, checklist, evidence gate, or domain panel.
- Remove user-facing generic wording from production work-item execution UI. Users must see business language, not implementation language such as `Generic submission`, `Workflow Form`, or `Trường dữ liệu`.

Evidence gathered:
- `backend/migrations/V38__client_workflow_template.sql` defines the official client workflow template with 12 nodes and 52 steps.
- In V38, 51/52 steps map to `builtin_generic_submission`; step 2 is evidence-only with no form; step 11 maps to `client_design_branch_decision`.
- `backend/migrations/V40__vietnamese_official_workflow_text.sql` only cleans Vietnamese labels/descriptions and does not replace the generic form mapping.
- Existing business UI/form surfaces in the repo include planning daily report, meeting minutes, planning checklist, evidence, BOQ unit price lifecycle, subcontractor checklist, owner/client checklist, construction daily report, reconciliation panels, material snapshot, surplus handling, incoming/outgoing acceptance, input/output invoices, payment schedules, debt reconciliation, cashflow, tax, and profit reporting.

Will be done:
- Save this active plan as the shared source-of-truth before implementation.
- Produce a review package before any code, migration, or UI change.
- Produce a Mockup as Text before any UI change.
- Classify all 52 steps as one of: keep evidence-only, remap to existing business form, remap to existing domain panel, create a new `client_*` workflow form, or remove generic fallback.
- Design field-level specs for every new `client_*` workflow form: field key, label, type, required flag, validation, help text, signature/PDF needs, and completion gate.
- Add only new migrations for form templates/versions/fields and step remapping. Do not edit already-applied migrations.
- Preserve workflow template/runtime separation. Publishing, remapping, and cloning must not silently mutate submitted or historical project runs.
- Define runtime migration handling for existing draft instances separately from submitted/signed/PDF instances.
- Update work-item UI labels so business users see the current step's business form title, expected input, and next action, not internal generic template names.
- Verify backend/frontend contracts around form detail, form submit, signature, PDF, evidence, required gates, activity/audit, and RBAC.
- Build a UAT script that starts a project from template `40000000-0000-0000-0000-000000000020` and checks every node/step mapping.

Done:
- User confirmed the plan-save direction with `OK`.
- User approved implementation with `OK triển khai với điều phối subagents phù hợp, đúng skills và hiệu quả`.
- Re-read Consflow repo architecture and workflow orchestration guardrails.
- Re-read V38 official workflow template and V40 Vietnamese text cleanup migration.
- Re-read the user-provided business UI/form catalog attachment.
- Confirmed the current plan was not yet present in `docs/current-states.md` before this entry.
- Delegated frontend work-item form-copy cleanup to a frontend subagent; build passed in that subagent's scope.
- Delegated migration/schema safety review to a DB reviewer subagent; reviewer confirmed the safe path is a new V41 migration, published versions, and template-step remap only.
- Added `backend/migrations/V41__official_workflow_business_forms.sql` to seed official `client_*` business forms and remap template `40000000-0000-0000-0000-000000000020` away from `builtin_generic_submission` for future runs.
- Preserved step 2 as evidence-only and preserved step 11 branch routing through `needs_design` on `client_design_branch_decision` version 2.
- Kept runtime instances, submitted answers, signatures, and PDF exports untouched.
- Updated work-item execution UI copy so user-facing execution labels say `Hồ sơ nghiệp vụ`, `Nội dung cần xử lý`, and `Thông tin nghiệp vụ` instead of generic form-engine wording.
- Updated work-item primary panel routing so document-upload steps with a workflow form open the business hồ sơ, incoming acceptance steps open incoming acceptance, procurement material steps with forms open hồ sơ nghiệp vụ, and subcontractor quantity dossier steps open reconciliation.
- Ran `make migrate-check`: 42 migrations checked successfully.
- Applied V41 successfully to the running Docker PostgreSQL stack through `/app/consflow-migrate`: `applied=1 skipped=40`.
- Verified template `40000000-0000-0000-0000-000000000020` in PostgreSQL: 52 total steps, 51 steps with forms, 0 steps on `builtin_generic_submission`, step 2 remains evidence-only, step 11 remains `client_design_branch_decision` v2 with `needs_design`.
- Added `backend/migrations/V42__remap_official_workflow_draft_forms.sql` to remap existing blank draft runtime instances from official workflow runs away from `builtin_generic_submission`.
- Applied V42 successfully to the running Docker PostgreSQL stack through `/app/consflow-migrate`: `applied=1 skipped=41`.
- Verified existing official workflow runtime instances in PostgreSQL: 0 `workflow_form_instances` remain on `builtin_generic_submission`.
- Ran `GOCACHE=$PWD/.gocache go test ./...` in `backend`: pass.
- Ran `npm run build` in `frontend`: pass; Vite reported only the existing large chunk warning.
- Browser/Playwright verification was skipped per repo rule because it was not explicitly requested in this turn.

Source-of-truth mapping:

| # | Node | Step / checklist | Current mapping | Product-grade target | Implementation status |
|---|---|---|---|---|---|
| 1 | Khảo sát | Khảo sát yêu cầu của chủ đầu tư | `builtin_generic_submission` | New `client_survey_requirement_intake` form for owner requirements, scope, stakeholders, constraints, deadline, and notes | Implemented in V41 |
| 2 | Khảo sát | Khảo sát / Chụp / Tải ảnh hiện trạng | No form; photo evidence required | Evidence-only step with guided photo categories, location/area, before-condition notes, and required evidence gate | Evidence-only preserved |
| 3 | Khảo sát | Thiết kế nắm phong cách | `builtin_generic_submission` | New `client_design_style_brief` form for style direction, references, materials, color tone, exclusions, and owner preference | Implemented in V41 |
| 4 | Khảo sát | Ghi nhận kích thước chính | `builtin_generic_submission` | New `client_site_measurement_record` form for measured areas, dimensions, unit, tolerance, measurement owner, and attachment evidence | Implemented in V41 |
| 5 | Khảo sát | Xác định rủi ro mặt bằng | `builtin_generic_submission` | New `client_site_risk_assessment` checklist for structural, MEP, access, safety, schedule, and cost risks | Implemented in V41 |
| 6 | Khảo sát | Biên bản khảo sát | `builtin_generic_submission` | New `client_site_survey_minutes` form with survey conclusion, participants, issues, required attachments, signature, and PDF | Implemented in V41 |
| 7 | Họp báo cáo khảo sát | QS / Bốc khối lượng không thiết kế | `builtin_generic_submission` | BOQ/takeoff business surface plus a takeoff summary form if embedded work-item submission is needed | Implemented in V41 |
| 8 | Họp báo cáo khảo sát | Thiết kế / Lên bản bố trí mặt bằng | `builtin_generic_submission` | New `client_design_layout_submission` form for layout revision, file links, assumptions, owner feedback, and readiness | Implemented in V41 |
| 9 | Họp báo cáo khảo sát | Thiết kế / Lên bản vẽ 3D | `builtin_generic_submission` | New `client_design_3d_submission` form for 3D package, revision, render files, owner comments, and approval readiness | Implemented in V41 |
| 10 | Họp báo cáo khảo sát | Thiết kế / Lên bản vẽ thi công | `builtin_generic_submission` | New `client_construction_drawing_submission` form for drawing set, revision, issued-for status, and attachment evidence | Implemented in V41 |
| 11 | Họp báo cáo khảo sát | Biên bản cuộc họp | `client_design_branch_decision` | Business meeting minutes with participants, decisions, tasks, and routing field `needs_design`; branch decision remains but cannot be the whole meeting form | Implemented in V41 |
| 12 | Thiết kế | Trao đổi / Lên bản vẽ bố trí mặt bằng | `builtin_generic_submission` | New `client_design_review_layout` form for design iteration, comments, blockers, attachment, and next revision | Implemented in V41 |
| 13 | Thiết kế | Trao đổi / Lên bản vẽ 3D | `builtin_generic_submission` | New `client_design_review_3d` form for 3D iteration, comments, acceptance criteria, attachment, and next revision | Implemented in V41 |
| 14 | Thiết kế | Trao đổi / Lên bản vẽ thi công | `builtin_generic_submission` | New `client_design_review_construction_drawing` form with document upload, drawing status, revision, and review notes | Implemented in V41 |
| 15 | Thiết kế | Chốt với Sale / Chủ đầu tư | `builtin_generic_submission` | Owner/client confirmation checklist or new `client_design_owner_approval` form with approval status and signature/PDF | Implemented in V41 |
| 16 | Thiết kế | Đưa bản vẽ qua bên QS | `builtin_generic_submission` | New `client_design_to_qs_handoff` form with handoff package, drawing revision, BOQ assumptions, and receiver confirmation | Implemented in V41 |
| 17 | BOQ (Không thiết kế) | QS lấy giá từ thầu phụ / tổ đội / vật tư | `builtin_generic_submission` | BOQ unit price lifecycle plus supplier price collection fields | Implemented in V41 |
| 18 | BOQ (Không thiết kế) | QS / PM / GĐ kiểm tra đơn giá đầu vào | `builtin_generic_submission` | BOQ price check/approval form with margin, variance, and reviewer decision | Implemented in V41 |
| 19 | BOQ (Không thiết kế) | GĐ rà soát định mức quản lý | `builtin_generic_submission` | Director management-rate review form with approval, conditions, and audit note | Implemented in V41 |
| 20 | BOQ (Không thiết kế) | QS / PM / GĐ gửi BOQ qua chủ đầu tư | `builtin_generic_submission` | BOQ submission package with document upload, sent date, recipient, version, and evidence | Implemented in V41 |
| 21 | BOQ (Có thiết kế) | QS / Sale / PM gửi BOQ qua chủ đầu tư | `builtin_generic_submission` | BOQ-after-design submission package with drawing revision linkage and owner delivery evidence | Implemented in V41 |
| 22 | Đầu Vào | QS / Bốc khối lượng | `builtin_generic_submission` | Procurement input takeoff form or BOQ-derived procurement quantity review | Implemented in V41 |
| 23 | Đầu Vào | Mua hàng / QS mua hàng với tổ đội / thầu phụ / NCC vật tư | `builtin_generic_submission` | Procurement sourcing form for vendor, scope, quotes, lead time, comparison, and selected option | Implemented in V41 |
| 24 | Đầu Vào | Mua hàng / QS ký hợp đồng / hồ sơ tạm ứng | `builtin_generic_submission` | Input contract/advance package using contract document upload and input payment schedule/advance surface | Implemented in V41 |
| 25 | Đầu Vào | PM / GĐ rà soát & kiểm tra đầu vào | `builtin_generic_submission` | Procurement approval checklist with scope, price, contract, advance, risk, and approve/reject decision | Implemented in V41 |
| 26 | Đàm phán giá | Sale / QS / PM / GĐ đàm phán thầu phụ | `builtin_generic_submission` | Negotiation log form with parties, original price, negotiated price, concessions, and decision | Implemented in V41 |
| 27 | Đàm phán giá | QS / PM chốt lại định mức | `builtin_generic_submission` | Final rate/margin confirmation form linked to BOQ unit price lifecycle | Implemented in V41 |
| 28 | Đàm phán giá | GĐ rà soát & chốt báo cáo đơn giá | `builtin_generic_submission` | Director final price approval form with signed decision and audit note | Implemented in V41 |
| 29 | Chốt Hợp đồng với chủ đầu tư | QS lên báo giá & hợp đồng | `builtin_generic_submission` | Owner quote/contract package form with quote version, contract draft, payment terms, and files | Implemented in V41 |
| 30 | Chốt Hợp đồng với chủ đầu tư | Kế toán / Pháp lý kiểm tra hợp đồng | `builtin_generic_submission` | Legal/finance contract review checklist with risk flags, tax/payment terms, and required corrections | Implemented in V41 |
| 31 | Chốt Hợp đồng với chủ đầu tư | Gửi file mềm cho chủ đầu tư | `builtin_generic_submission` | Owner soft-file submission form with document list, delivery channel, recipient, and sent evidence | Implemented in V41 |
| 32 | Chốt Hợp đồng với chủ đầu tư | PM / GĐ rà soát và ký duyệt | `builtin_generic_submission` | Internal contract approval/signature form with PDF/signature gate | Implemented in V41 |
| 33 | Chốt Hợp đồng với chủ đầu tư | In và gửi bản cứng cho chủ đầu tư | `builtin_generic_submission` | Hardcopy dispatch form with print set, courier/hand delivery, receiver, date, and evidence | Implemented in V41 |
| 34 | QA & QC / Thi công | QS / PM / Thiết kế / QA họp & phân công | `builtin_generic_submission` | Meeting minutes plus construction assignment matrix | Implemented in V41 |
| 35 | QA & QC / Thi công | Chốt biên bản cuộc họp | `builtin_generic_submission` | Meeting minutes v2 with PDF/signature, action items, owners, and due dates | Implemented in V41 |
| 36 | QA & QC / Thi công | QA đảm bảo chất lượng / kế hoạch thi công | `builtin_generic_submission` | QA/construction plan checklist with inspection criteria and schedule | Implemented in V41 |
| 37 | QA & QC / Thi công | QA báo cáo liên tục trong quá trình thi công | `builtin_generic_submission` | Existing construction daily report form | Implemented in V41 |
| 38 | Nghiệm thu Đầu Vào | Nghiệm thu thầu phụ / tổ đội | `builtin_generic_submission` | Incoming acceptance panel plus subcontractor checklist/reconciliation | Implemented in V41 |
| 39 | Nghiệm thu Đầu Vào | Nghiệm thu vật tư đầu vào vs đầu ra | `builtin_generic_submission` | Incoming acceptance panel plus material before/after snapshot | Implemented in V41 |
| 40 | Nghiệm thu Đầu Vào | QS hồ sơ kiểm tra khối lượng thầu phụ | `builtin_generic_submission` | Subcontractor reconciliation panel with quantity and pricing check | Implemented in V41 |
| 41 | Nghiệm thu Đầu Vào | GĐ rà soát hồ sơ kiểm tra khối lượng thầu phụ | `builtin_generic_submission` | Director approval checklist/signature for subcontractor quantity dossier | Implemented in V41 |
| 42 | Nghiệm thu Đầu Vào | QS hồ sơ thanh toán cho thầu phụ | `builtin_generic_submission` | Input invoice/payment schedule/payable dossier surface | Implemented in V41 |
| 43 | Nghiệm thu Đầu Vào | GĐ ký nghiệm thu hồ sơ thanh toán | `builtin_generic_submission` | Signed payment acceptance approval with PDF/signature gate | Implemented in V41 |
| 44 | Nghiệm thu Đầu Vào | Kế toán kiểm tra hồ sơ thanh toán | `builtin_generic_submission` | Payable reconciliation/input invoice review panel | Implemented in V41 |
| 45 | Nghiệm thu Đầu Vào | Thanh toán cho thầu phụ | `builtin_generic_submission` | Input payment plan/payment execution surface | Implemented in V41 |
| 46 | Nghiệm thu Đầu Ra | QA / QS / PM / GĐ nghiệm thu & ký duyệt công trình | `builtin_generic_submission` | Outgoing acceptance panel plus owner/client checklist and signature | Implemented in V41 |
| 47 | Nghiệm thu Đầu Ra | GĐ hồ sơ nghiệm thu & ký duyệt | `builtin_generic_submission` | Director outgoing acceptance approval with PDF/signature gate | Implemented in V41 |
| 48 | Nghiệm thu Đầu Ra | Hồ sơ thanh toán & quyết toán & ký duyệt | `builtin_generic_submission` | Output billing/receivable settlement package with document upload and approval | Implemented in V41 |
| 49 | Nghiệm thu Đầu Ra | Xuất hóa đơn | `builtin_generic_submission` | Output invoice PDF panel | Implemented in V41 |
| 50 | Nghiệm thu Đầu Ra | Thanh toán nhiều kỳ | `builtin_generic_submission` | Output billing schedule/payment collection panel | Implemented in V41 |
| 51 | Kết thúc: Bàn giao + Đóng tài chính | Kế toán thầu phụ xuất hóa đơn về công ty | `builtin_generic_submission` | Input invoice PDF/AP closure surface | Implemented in V41 |
| 52 | Kết thúc: Bàn giao + Đóng tài chính | Kế toán xuất hóa đơn cho chủ đầu tư | `builtin_generic_submission` | Output invoice PDF/AR closure plus cashflow/tax/profit closure checks | Implemented in V41 |

Product-grade definition of done:
- 52/52 steps are explicitly classified and implemented with no accidental generic fallback in the official client template.
- 0 user-facing occurrences of `Generic submission`, `Trường dữ liệu`, or `Workflow Form` remain in the work-item execution experience for official client workflow runs.
- Every required form has domain-specific labels, help text, validation, and completion summary.
- Evidence-only steps explain exactly what evidence the user must upload.
- Domain-panel steps open the relevant business panel with clear empty/loading/error states.
- Submitted/signed/PDF historical form instances remain auditable and are not silently rewritten.
- Draft runtime instances have a documented migration/remap path before any data rewrite is implemented.
- Backend tests, frontend build, and migration validation pass. Browser verification remains skipped unless explicitly requested by the user in that turn.

Implementation review package:
- Completed before code changes and approved by the user before implementation.

## Work Item Remove Workflow Position Rail

Status: complete

Will be done:
- Remove the left `Vị trí workflow` rail from the work item detail body.
- Keep `Node`, `Bước`, `Minh chứng`, and `Biểu mẫu` in the existing `Quy trình đang chạy` header stats.
- Change the work item detail body from three columns to two columns: main content and right `Việc cần làm`.
- Do not add a breadcrumb replacement.
- Preserve workflow actions, required gates, evidence, activity, forms, backend APIs, query invalidation, and runtime workflow semantics.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `OK`.
- Re-read Consflow frontend workflow and workflow orchestration skills and target work item detail code.
- Removed the `WorkItemProgressRail` render from the work item detail body.
- Removed the unused `WorkItemProgressRail` component implementation.
- Changed the work item detail body grid from three columns to two columns: main content plus right `Việc cần làm`.
- Kept `Node`, `Bước`, `Minh chứng`, and `Biểu mẫu` in the existing `Quy trình đang chạy` header stats.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Workflow Position Breadcrumb

Status: reverted

Before change snapshot:
- User screenshot: `Microsoft Edge Appshot 2026-07-06T10-39-01.914Z.png`.
- Work item detail body is a three-column grid: left `Vị trí workflow`, center current step/support content, right `Việc cần làm`.
- Left `Vị trí workflow` rail is rendered by `WorkItemProgressRail` in `frontend/src/modules/work-items/WorkItemDetailDialog.tsx`.
- `WorkItemProgressRail` displays four vertical items: `Công trình`, `Node quy trình`, `Bước`, `Đầu việc`.
- `WorkItemWorkspaceHeader` currently shows `Quy trình đang chạy` with title, status/project/priority chips, context stats for `Node`, `Bước`, `Minh chứng`, `Biểu mẫu`, and next action.

Attempted change:
- Remove the left `Vị trí workflow` column from the work item detail body.
- Move the workflow position information into a compact breadcrumb inside the `Quy trình đang chạy` header.
- Change the work item detail body from three columns to two columns: main content and right `Việc cần làm`.
- Preserve workflow actions, required gates, evidence, activity, forms, backend APIs, query invalidation, and runtime workflow semantics.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Attempt done:
- User approved implementation and explicitly requested preserving the before-change reference for comparison.
- Re-read Consflow frontend workflow and workflow orchestration skills, AGENTS.md, current state notes, and target work item detail code.
- Removed the `WorkItemProgressRail` left column from the work item detail body.
- Changed the work item detail body grid from three columns to two columns: main content plus right `Việc cần làm`.
- Added compact `Vị trí workflow` breadcrumb inside `Quy trình đang chạy` with `Công trình > Node > Bước > Đầu việc`.
- Kept `Minh chứng` and `Biểu mẫu` as header stats after moving `Node` and `Bước` into the breadcrumb.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

Revert done:
- User requested `Revert lại đoạn vừa rồi đi`.
- Restored the three-column work item detail body with left `Vị trí workflow`, center content, and right `Việc cần làm`.
- Restored `WorkItemProgressRail` and removed the compact breadcrumb from `Quy trình đang chạy`.
- Restored header stats for `Node`, `Bước`, `Minh chứng`, and `Biểu mẫu`.
- Kept the earlier sticky offset fix for the restored left rail: `xl:top-[5.5rem]`.
- Ran `cd frontend && npm run build` successfully after the revert.

## Work Item Sticky Side Panel Offset

Status: complete

Will be done:
- Retouch only sticky positioning for the side panels inside `frontend/src/modules/work-items/WorkItemDetailDialog.tsx`.
- Move `Vị trí workflow` and `Việc cần làm` sticky panels below the sticky project top bar so their headers remain visible while scrolling.
- Keep the existing three-column work item layout, workflow actions, evidence/forms/activity behavior, backend APIs, query invalidation, and workflow gates unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `OK`.
- Re-read Consflow frontend workflow and diagnose skills, current state notes, and target `WorkItemDetailDialog` sticky panel code.
- Updated both work item side panels from `xl:top-0` to `xl:top-[5.5rem]` so their headers sit below the sticky project top bar.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Evidence Search Row Layout

Status: complete

Will be done:
- Retouch only the search/filter layout inside `frontend/src/modules/work-items/EvidenceGallery.tsx`.
- Move the `Tìm minh chứng, đầu việc, mô tả` search input to its own full-width row above the evidence filters.
- Keep the evidence filter segmented buttons below the search input.
- Keep evidence data, upload, review, search/filter behavior, backend APIs, query invalidation, and work item workflow gates unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `làm đi`.
- Re-read Consflow frontend workflow skill and target `EvidenceGallery` code.
- Moved the evidence search input to its own full-width row above the filter group.
- Kept the evidence filter segmented buttons below search without the previous desktop two-column split.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Evidence Filter Affordance

Status: complete

Will be done:
- Retouch only the evidence filter row inside `frontend/src/modules/work-items/EvidenceGallery.tsx`.
- Make the `Tất cả`, `Ảnh`, `Tài liệu`, status filters read as interactive filter controls instead of passive chips or labels.
- Rename the `submitted` filter label from `Đã gửi` to `Đang chờ` to match the current `draft/submitted/pending` filter logic.
- Keep evidence data, upload, review, search, backend APIs, query invalidation, and work item workflow gates unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `approve`.
- Re-read Consflow frontend workflow skill, UI guardrails, current plan notes, and target `EvidenceGallery` code.
- Added a visible `Lọc minh chứng` filter group around the evidence filter controls.
- Changed evidence filter buttons to clearer segmented filter controls with active state, active icon, `aria-pressed`, and count badges.
- Renamed the `submitted` filter label from `Đã gửi` to `Đang chờ` while keeping the existing `draft/submitted/pending` filter behavior unchanged.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Detail Project Workspace Route

Status: complete

Will be done:
- Add a project-scoped work item detail route under `/projects/:projectId/work-items/:workItemId` so the detail page stays inside the project workspace shell.
- Keep `/work-items/:id` as a fallback route for callers that do not have project context.
- Update work item detail navigation to prefer the project-scoped route when `project_id` is available.
- Remove standalone work item focus mode from `AppShell`; project workspace focus mode remains responsible for project-scoped detail pages.
- Keep backend routes, API contracts, workflow gates, form/evidence/signature/PDF behavior, and query invalidation unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the correction with `Approve`.
- Re-read Consflow frontend/workflow orchestration skills, route shell, current work item detail page, navigation helper, and current work item entry points.
- Added the nested project workspace route `/projects/:id/work-items/:workItemId`.
- Updated `WorkItemDetailPage` to resolve either nested `workItemId` or fallback top-level `id` and to return to the project overview when no `returnTo` is supplied.
- Updated work item navigation links from project detail, my work, planning runtime, planning daily reports, meeting minutes, and planning checklists to pass project context.
- Removed the standalone work item focus mode from `AppShell`; project-scoped work item detail now relies on project focus mode.
- Kept `/work-items/:id` as a fallback route for callers without project context.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Detail Full Page

Status: complete

Will be done:
- Add a full-page work item detail route at `/work-items/:id`.
- Reuse the existing work item detail workspace so dialog and page behavior stay consistent without duplicated domain logic.
- Add return navigation so callers can open the page and return to the original screen.
- Convert current work item open actions from modal state to route navigation where feasible.
- Keep backend routes, API contracts, mutations, query invalidation, and workflow domain gates unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User selected implementation direction 2: open work item detail as a page and back to the caller.
- Re-read Consflow frontend/workflow orchestration skills, router, dialog shell, current work item dialog, and current callsites.
- Added a reusable work item detail workspace extracted from the dialog body.
- Added `/work-items/:id` page route shell with a `Quay lại` action.
- Added a small helper for work item detail links with safe `returnTo` paths.
- Converted project detail, planning runtime, my work, daily reports, meeting minutes, and planning checklists work-item open actions to navigate to the full-page detail route.
- Added work item focus mode in `AppShell` so `/work-items/:id` uses full page width without the global shell/sidebar.
- Preserved the existing dialog wrapper as a reuse path, but current work item entry points now open the page route.
- Kept backend APIs, workflow actions, form/evidence/signature/PDF mutations, and query invalidation behavior inside the shared detail workspace unchanged.
- Ran `cd frontend && npm run build` successfully after the route/page changes and again after enabling work item focus mode.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Main And Support Single Column Retouch

Status: complete

Will be done:
- Refactor only the `Nội dung chính` and `Thông tin hỗ trợ` areas in the work item detail workspace.
- Keep the overall work item dialog layout unchanged: workflow rail, central content, and right action panel stay as-is.
- Make `WorkflowFormPanel` stack form summary, fields, signature/PDF, and history in one column.
- Make workflow form fields full width inside `Nội dung chính`.
- Change `Thông tin hỗ trợ` selectors from card-like buttons into a proper tab strip.
- Keep backend routes, API contracts, mutations, query invalidation, and domain panels unchanged.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User clarified the correct scope: `Nội dung chính` and `Thông tin hỗ trợ`.
- User approved implementation with `làm đi`.
- Re-read Consflow frontend/workflow orchestration skills, current state notes, and target work item UI code.
- Refactored `WorkflowFormPanel` so the workflow form summary, fields, signature/PDF, and events stack in one column.
- Removed the two-column field grid inside `Nội dung chính` so workflow form controls use full width.
- Refactored `WorkItemSupportingTabs` so the support header stacks in one column.
- Replaced card-like support tab buttons with a compact tab strip using tab semantics.
- Kept the overall work item dialog layout, backend APIs, mutations, and query invalidation unchanged.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Work Item Support Workspace Retouch

Status: complete

Will be done:
- Refactor only the `THÔNG TIN HỖ TRỢ` area inside the work item detail workspace.
- Keep backend routes, API response contracts, mutations, and query invalidation behavior unchanged.
- Retouch the support shell so evidence, activity, and workflow form/PDF share a clearer workspace hierarchy.
- Make evidence list/gallery-first with compact upload controls.
- Make activity timeline-first with a compact composer entry point.
- Make workflow form/PDF summary-first with validation, form fields, signature, and PDF artifacts separated clearly.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved the review package with `Approve`.
- Re-read Consflow frontend workflow skill, AGENTS.md, router, UI guardrails, current plan notes, and target work item components.
- Refactored `WorkItemSupportingTabs` into a clearer support workspace shell with status-aware tabs and a next support action.
- Changed work-item activity support to timeline-first with a compact composer toggle.
- Changed work-item evidence support to summary/list-first and moved upload into a compact expandable panel.
- Added a list variant to `EvidenceGallery` while preserving the existing card default for project files.
- Reworked `WorkflowFormPanel` into an action center, processing summary, form fields section, artifacts column, and form event panel.
- Removed the old unused workflow form summary helper after replacing it with the new summary layout.
- Ran `cd frontend && npm run build` successfully.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Workflow Detail Workspace UI

Status: complete

Will be done:
- Redesign `WorkItemDetailDialog` into a clearer workflow detail workspace with a left workflow-position rail, central current-step content, and right action panel.
- Keep existing work item actions, evidence, activity, workflow form, and domain-specific panels without changing backend contracts.
- Move competing footer actions into a single action hierarchy with one primary CTA and secondary actions.
- Add or reuse shared sizing/layout support needed by the dialog.
- Run `cd frontend && npm run build`; browser verification is skipped unless explicitly requested.

Done:
- User approved implementation with `Làm đi` after reviewing text and image mockups.
- Read Consflow frontend and workflow orchestration skills, UI guardrails, router, current work item detail implementation, and existing dirty diffs.
- Added a `workspace` dialog size for wide operational detail layouts.
- Reworked `WorkItemDetailDialog` into a 3-zone layout: workflow position rail, current-step panel, and right-side action panel.
- Preserved existing APIs and domain panels; no backend contract or route change was introduced.
- Ran `cd frontend && npm run build` successfully.
- Skipped browser verification per repository rule because it was not explicitly requested in this turn.
- Tuned `Vị trí workflow` so it no longer stretches to the body height; it is sticky on wide viewports with internal timeline scrolling.

## Full Workflow Demo Seed

Status: complete

Will be done:
- Extend `backend/seed/seed.sql` with a full demo project that covers all current work item types and workflow form templates.
- Seed a runtime pipeline with nodes, steps, work items, workflow form instances, answers, evidence, activities, approvals, and domain records.
- Seed operational data for material snapshots, surplus handling, incoming/outgoing acceptance, subcontractor/client reconciliation, and finance/debt panels.
- Validate seed syntax and run the nearest feasible backend checks without browser verification.

Done:
- Review package approved by the user with `OK triển khai`.
- Confirmed this is a seed/demo-data change only; no schema, API, or UI contract change is intended.
- Added full workflow/form coverage seed data in `backend/seed/seed.sql`.
- Added seed upload fixtures for workflow form PDF, input invoice PDF, and output invoice PDF.
- Updated `backend/Makefile` so `make seed` copies seed upload fixtures into the configured upload directory/container volume.
- Validated `backend/seed/seed.sql` against the running Postgres container inside a rollback transaction.
- Applied the seed to the running local Docker Postgres with `cd backend && make seed`.
- Ran `cd backend && make test` successfully.

## Project-First Folder UI

Status: complete

Will be done:
- Make `/projects` the primary post-login entry for users with `project.view`.
- Reduce the main sidebar to high-level entry points instead of exposing every project domain module as a global navigation item.
- Convert `frontend/src/modules/projects/ProjectsPage.tsx` from a dense table into a folder/tree explorer for construction projects.
- Convert the project detail overview in `frontend/src/modules/projects/ProjectDetailPage.tsx` to a compact project operations summary that sits inside the project workspace layout.
- Add project focus mode for `/projects/:id/*` so the project business tree becomes the main menu and the global app sidebar/header is hidden.
- Keep existing project-scoped routes and backend APIs unchanged.
- Run `cd frontend && npm run build` after frontend edits.

Done:
- User approved the visual mockup direction: `/projects` as project explorer and project detail as a folder-style business tree.
- Updated `HomeRedirect` so users with `project.view` land on `/projects`.
- Reduced the main sidebar to project-first high-level navigation and removed the project domain modules from the primary global menu.
- Reworked `ProjectsPage` into a searchable/filterable folder tree explorer with expandable construction project rows.
- Reworked `ProjectDetailPage` overview into a folder-based operations workspace with a persistent business tree and compact work table.
- Added `ProjectWorkspaceLayout` focus mode for `/projects/:id/*` with project top bar, fixed business tree menu, and scoped content outlet.
- Nested all project-scoped routes under `ProjectWorkspaceLayout` while preserving their existing URLs and permission guards.
- Updated `AppShell` so project focus mode hides the global sidebar/header, matching the focused `pipeline-builder` behavior.
- Removed the old embedded business tree from `ProjectDetailPage`; the project workspace layout is now the single owner of that menu.
- Added query-tab support for project members/files links from the workspace tree.
- Replaced user-facing `PM` labels with `Phụ trách` and removed internal `Folder` / `Theo quyền` text from project folder rows.
- Kept backend APIs, routes, migrations, and project-scoped module routes unchanged.
- Ran `cd frontend && npm run build` successfully after both project-first and focus-mode changes.

## Consflow Impact Analysis Skill

Status: complete

Will be done:
- Add `consflow-impact-analysis` as a repo-scoped skill for feature overlap and logic collision analysis.
- Add UI metadata for the new skill.
- Wire the new skill into the Consflow skill README, module routing, and repo architecture workflow.
- Validate the skill artifact with targeted file inspection and `rg`.

Done:
- Review package approved by the user.
- Added `.agents/skills/consflow-impact-analysis/SKILL.md`.
- Added `.agents/skills/consflow-impact-analysis/agents/openai.yaml`.
- Wired `consflow-impact-analysis` into `.agents/skills/README.md`.
- Added impact-analysis pairing guidance to `consflow-module-memory`.
- Added cross-feature collision routing to `consflow-repo-architect`.
- Validated `SKILL.md` frontmatter and `openai.yaml` with Ruby YAML checks because the Python validator is missing `PyYAML` in this environment.

## Procurement And Warehouse Operational Workspace Redesign

Status: complete

Mục đích chính:
- Làm rõ chuỗi BOQ -> yêu cầu mua -> đặt hàng -> nghiệm thu đầu vào -> phiếu nhập kho -> xuất công trình -> xử lý vật tư dư, để người dùng luôn thấy đúng trạng thái và một hành động tiếp theo có thể thực hiện.

Will be done:
- Thiết kế lại Mua sắm thành workspace ưu tiên việc cần xử lý, có bộ lọc nghiệp vụ, tiến độ nghiệm thu/nhập kho, CTA chính theo trạng thái và action visibility khớp RBAC.
- Thiết kế lại Kho thành bốn bề mặt: tồn theo kho, phiếu kho, tồn công trình/vật tư dư và danh mục vật tư.
- Bổ sung tồn theo từng kho, ledger/phân trang phiếu kho, chứng từ nhiều dòng và liên kết rõ với nghiệm thu đầu vào mà không tự động tạo stock movement.
- Hoàn thiện điều chuyển hai đầu với kho nguồn/kho đích trong một transaction; giữ kiểm tra không âm tồn và site balance/surplus hiện hữu.
- Tách ý nghĩa audit đối soát nghiệm thu mua sắm khỏi event nhập kho thật; giữ lịch sử cũ không bị viết lại.
- Cập nhật report query, frontend/backend contracts, integration tests và central/detail/domain mappings liên quan.
- Chạy backend format/tests, frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the full review package and Mockup as Text with `Duyệt, triển khai theo review package này`.
- Loaded the mandatory central/backend/frontend mappings, commercial procurement, materials warehouse, quality acceptance, reporting, impact-analysis and database workflows.
- Confirmed the PostgreSQL/custom SQL migration boundary, existing V60 maximum, current procurement acceptance-derived receive invariant and overlapping dirty worktree that must be preserved.
- Added migration V61 with posted multi-line stock documents/items, movement document linkage and target warehouse linkage for two-ended transfers; applied it through the custom migration runner in Docker (`applied=1 skipped=60`).
- Added source-backed per-warehouse positions, movement ledger, stock document list/detail/create and project warehouse-receipt-option endpoints with project visibility and warehouse permission gates.
- Refactored stock posting through one transaction helper; transfer locks source/target in stable order, subtracts the source and adds the target, while issue/return continue updating site balances and surplus status.
- Linked explicit receive movements to incoming-acceptance items, capped receipt quantity by remaining accepted quantity and preserved the invariant that submitting acceptance alone creates no stock movement.
- Split procurement acceptance reconciliation audit events from real warehouse-receipt events and extended procurement list/detail data with accepted-vs-stocked quantities plus latest acceptance/work-item linkage.
- Rebuilt Mua sắm as a permission-aware action-first queue with truthful acceptance/stock progress, dense desktop table, responsive cards, operational filters, one primary next action and enhanced detail dialog.
- Rebuilt Kho as four tabs for per-warehouse stock, posted documents, project site/surplus and material catalog; added a shared responsive multi-line document dialog for receive/issue/transfer/return and acceptance-source prefill.
- Updated frontend contracts/status mappings plus central/backend/frontend/commercial/materials/quality/reporting mappings for the new routes, tables, audit semantics and UI ownership.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass after updating the procurement event regression expectation.
- Ran the DB-backed `TestStockDocumentTransferMovesBothWarehouseBalancesAtomically` in the Consflow Docker network: pass.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Preserved Golden Sample compatibility by recognizing historical explicit receipts linked to their source BOQ items; runtime smoke now reports both received procurement requests with accepted quantity equal to stocked quantity and no duplicate receipt options.
- Rebuilt and recreated the local backend/admin containers, confirmed V61 remains applied, and smoke-tested health plus project-filtered procurement, per-warehouse positions, stock documents and receipt options successfully.
- Ran `git diff --check`: pass; user-facing `Pending / Partial / Handled` labels are no longer present in the redesigned pages.
- Browser verification was skipped per repository rule because it was not explicitly requested.

## Construction And Acceptance Focused Workspace

Status: complete

Mục đích chính:
- Gom bốn bề mặt Thi công, Quản lý chất lượng, Nghiệm thu và Khắc phục sai lỗi thành hai không gian dễ hiểu: theo dõi thực thi hằng ngày tại `Thi công` và lập/kết luận biên bản tại `Nghiệm thu`.

Will be done:
- Rút gọn cây nghiệp vụ `Thi công & Chất lượng` còn `Thi công` và `Nghiệm thu`; giữ route cũ `/quality` và `/rework` bằng redirect tương thích.
- Chuyển `Thi công` thành status cards và ba tab `Nhật ký & tiến độ`, `Checklist chất lượng`, `Vấn đề & khắc phục` với timeline báo cáo gồm đã làm, đang làm, sẽ làm và vấn đề.
- Tái sử dụng các lifecycle ITP/RFI/NCR, issue, rework và replanning hiện hữu trong đúng tab, không gộp bảng dữ liệu hoặc thay đổi transition backend.
- Chuyển `Nghiệm thu` thành master-detail, checklist rõ ràng, một CTA tiếp theo theo trạng thái và một dialog kết luận Đạt/Đạt có điều kiện/Không đạt.
- Tách editor checklist nghiệm thu dùng chung cho project workspace và work-item outgoing acceptance.
- Cập nhật central/frontend/construction/quality/workflow mappings; chạy frontend build, backend test phù hợp và `git diff --check`.

Done:
- User approved the complete review package with `Approve`.
- Confirmed Release 1 can preserve the current API tables and lifecycle: failed RFI still creates NCR plus linked rework atomically; failed acceptance still creates an issue and exposes the explicit idempotent create-rework action.
- Confirmed the implementation overlaps a dirty worktree; existing user changes will be preserved and target-file diffs reviewed before patching.
- Reduced the project business tree to `Thi công` and `Nghiệm thu`; `/projects/:id/quality` and `/projects/:id/rework` now preserve old bookmarks through redirects to the matching construction tab.
- Rebuilt `OperationsPage` around four status cards and the approved three tabs. The daily timeline reads workflow-form structured answers and legacy summary rows, keeps report blockers separate from official issues, and captures quick reports through distinct Đã/Đang/Sẽ làm/Vấn đề fields without a backend migration.
- Embedded `QualityPage` as field checks plus advanced ITP configuration and as the NCR remediation section; embedded `ReworkPage` retains separate rework/replanning queries and RBAC-gated create/approve actions.
- Rebuilt `AcceptancePage` as a responsive master-detail workspace with status cards, progress steps, explicit checklist/evidence table, one state-driven CTA, and one radio-based conclusion dialog with required comment.
- Added shared `AcceptanceEditor.tsx` for checklist rows, evidence/required-action validation, draft payload mapping and submit gates; both the project acceptance workspace and `OutgoingAcceptancePanel` now use it.
- Kept the existing backend/domain lifecycle unchanged: acceptance fail creates its issue first, `Mở yêu cầu khắc phục` calls the existing idempotent rework action, and RFI/NCR transitions remain owned by quality APIs.
- Updated central/frontend/construction/quality/workflow mappings for the consolidated presentation routes, shared editor ownership, structured/legacy timeline mapping, permissions and compatibility redirects. No backend-function map or migration update was required because API contracts, handlers, repository logic and schema were unchanged.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran `cd backend && go test ./...`: pass.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because the user did not request it.

## BOQ / Dự Toán Workspace Redesign

Status: complete

Mục đích chính:
- Giúp QS/PM/Giám đốc đưa từng dòng dự toán qua đúng chuỗi đơn giá dự kiến -> định mức -> phê duyệt -> so sánh báo giá -> giá chốt -> yêu cầu mua sắm, luôn thấy rõ việc cần làm và nguồn gốc quyết định giá.

Will be done:
- Tổ chức lại `/projects/:id/boq` thành ba tab `Dòng dự toán`, `Báo giá nhà cung cấp`, `So sánh & chốt giá`, với một CTA chính theo trạng thái và UI tiếng Việt nhất quán.
- Chuyển tạo/xem/sửa dòng BOQ sang shared dialog, bỏ status khỏi form, thêm chi tiết dòng gồm tổng quan, đơn giá/lịch sử, báo giá và liên kết mua sắm.
- Làm chi tiết báo giá, đánh giá, so sánh và preview sinh yêu cầu mua responsive, permission-aware, có loading/empty/error/readonly states.
- Siết backend lifecycle: tạo luôn draft, chỉ sửa draft/revised, submit nhận draft/revised, approve yêu cầu submitted + norm lock, revise là action có lý do/audit.
- Bổ sung API/response additive cho chi tiết BOQ và liên kết procurement đang hiệu lực; không sinh lại dòng đã chuyển mua sắm, không tạo stock movement hoặc AP/AR side effect.
- Cập nhật central/backend/frontend/commercial mappings và shared plan state; không dự kiến migration.
- Chạy backend format/tests, focused BOQ tests, Golden seed validation khi khả dụng, frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the complete review package with `OK triển khai theo review package này` and explicitly requested subagent coordination.
- Loaded the mandatory central/backend/frontend mappings, repo architect, backend/frontend workflow, commercial/procurement, impact-analysis and git workflow guidance.
- Confirmed the worktree was clean on `main` before implementation; no user edits overlap the target files.
- Confirmed current code exposes client-owned BOQ status, submit excludes revised rows, UI ignores action permissions, quote statuses leak English, and Golden BOQ rows already linked to two procurement requests are still presented as ready to generate.
- Rebuilt the BOQ page as a responsive Vietnamese workspace with three tabs: `Dòng dự toán`, `Báo giá nhà cung cấp`, and `So sánh & chốt giá`; create/edit/view/revise/result interactions use shared dialogs, detail has overview/price/quote/procurement tabs, and catalog/action controls follow permissions.
- Added inline required/invalid highlighting plus warning-toast summaries for supplier quotation editing and quote selection; loading, error, empty, read-only and truncated-price-history states are explicit.
- Added record-scoped `GET /boq-items/:id` and `POST /boq-items/:id/revise`; create is server-owned draft, update/expected price are limited to draft/revised, submit accepts draft/revised, norm lock requires submitted, approval requires every submitted row norm-locked, and final lock requires approved.
- Kept approved-to-revised safe without a migration: revise requires an audited reason, clears the approver, blocks final-locked or active-procurement rows, retains norm lock, and allows descriptive metadata only while unit, quantity, material and expected price remain immutable.
- Serialized submit/approve/revise/generate with one project lifecycle advisory lock. Generation is project-scope and union-permission guarded, excludes BOQ lines with active procurement, preserves quotation/final-price snapshots, and has regression assertions for no stock movement, payable or receivable side effects.
- Added BOQ list/detail procurement linkage (`request id/code/status`, active count and warning state) plus bounded price history count/truncation. Frontend readiness excludes linked lines, invalidates BOQ/procurement caches after writes and exposes procurement navigation only with `procurement.view`.
- Updated the central keyword map, backend function map, frontend UI/utility map and commercial/procurement domain map. No root-cause-triage map update was required because search ownership/diagnostic routing did not change.
- No migration was added or modified. Residual non-blocking scale risk: active-procurement linkage currently lacks a dedicated `procurement_request_items.boq_item_id` index, so a future performance-only migration should be considered for very large projects.
- Ran `env GOCACHE="$PWD/.gocache" go test ./...` in `backend`: pass. DB-backed integration cases compile but skip without `CONSFLOW_TEST_DATABASE_URL`; Golden seed/database validation was therefore unavailable.
- Ran `npm run build` and `npm run lint` in `frontend`: pass; Vite reported only the existing large-chunk advisory.
- Ran `git diff --check`: pass. Browser verification was skipped per repository rule because the user did not request it.

## Thủ Thiêm Workflow App Display Name

Status: complete

Mục đích chính:
- Đổi tên sản phẩm hiển thị từ `Consflow` sang `Thủ Thiêm Workflow` trên app shell, trang đăng nhập và tiêu đề trình duyệt trước khi chốt thiết kế logo.

Will be done:
- Đổi tên hiển thị ở sidebar, heading đăng nhập và browser title.
- Giữ nguyên mark `CF` như placeholder tạm thời và giữ subtitle `Construction Workflow OS`.
- Giữ nguyên package/database/Docker identifiers, backend app name và email demo `@consflow.local`.
- Cập nhật central/frontend mappings cho owner branding dùng chung.
- Chạy frontend build và tìm kiếm chuỗi mục tiêu; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User explicitly asked to skip logo design and implement the app display-name change first.
- Loaded the mandatory central/frontend mappings, frontend workflow guidance, router, app shell, login page and browser metadata.
- Confirmed the user-facing name is owned by `AppShell.tsx`, `LoginPage.tsx` and `frontend/index.html`; no API, database or domain lifecycle change is required.
- Changed the sidebar product name to `Thủ Thiêm Workflow`, the login heading to `Đăng nhập Thủ Thiêm Workflow`, and the browser title to `Thủ Thiêm Workflow - Construction Workflow OS`.
- Preserved the existing `CF` mark as a temporary placeholder, kept the `Construction Workflow OS` subtitle, and left internal `consflow` package/database/Docker identifiers plus `@consflow.local` demo accounts unchanged.
- Updated the central and frontend UI mappings with the branding owner files and the intentional display-name-only scope. No backend, database, route, contract, status, permission or domain mapping update was required.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Confirmed the built `dist/index.html` carries the new browser title and the target user-facing frontend files contain no remaining capitalized `Consflow` display label.
- Browser verification was skipped per repository rule because the user did not request it.

## Construction Daily Report Create/Detail Contract Alignment

Status: complete

Mục đích chính:
- Đảm bảo báo cáo được tạo tại `Thi công` lưu đúng cấu trúc mà form chi tiết trình bày, đồng thời minh chứng luôn có trạng thái rõ ràng và giữ nguyên permission/work-item evidence semantics.

Will be done:
- Thay form tạo generic bằng dialog chuyên biệt cho ngày/thời tiết/tiến độ, Đã làm, Đang làm, Sẽ làm, blocker, ghi chú, nhân công, vật tư, đầu việc nguồn và minh chứng.
- Cho phép chọn minh chứng hiện hữu hoặc tải tệp mới sau khi chọn đầu việc nguồn; tái sử dụng work-item evidence APIs và không tạo project-level upload mới.
- Mở rộng typed create contract của `/projects/:id/daily-reports` để lưu `work_item_id`, `structured_json`, `photos` và `primary_evidence_id`, với kiểm tra project/work-item/evidence/status.
- Giữ workflow-form bridge idempotent hiện hữu, legacy report fallback, audit và schema V53; không thêm migration.
- Luôn hiển thị trạng thái `Minh chứng · 0` trong chi tiết và đồng bộ CTA frontend với quyền `operation.report` của backend.
- Cập nhật backend/frontend/central/construction/workflow mappings và focused regression tests.
- Chạy backend format/tests, frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved all three review points with `Approve`.
- Confirmed root cause: the generic create form and repository whitelist persist only legacy summary fields, while the detail reads the richer V53/workflow-backed structure and hides evidence when no IDs exist.
- Confirmed evidence upload remains work-item scoped (`evidence_records.work_item_id NOT NULL`, `CanActOnWorkItem`), so the approved create flow must select a source work item before uploading or linking evidence.
- Completed cross-feature impact check: no workflow-form lifecycle, approval, stock, quality, acceptance, report export or migration behavior needs to change.
- Added typed `ConstructionDailyReportCreateRequest` handling and expanded `dailyReportColumns()` to persist the existing V53 `work_item_id`, `structured_json` and `primary_evidence_id` columns together with legacy summaries and evidence IDs.
- Added backend validation for report date/progress/work groups/resources, same-project and permission-visible source work item, same-work-item non-rejected evidence, client visibility and a maximum of 50 linked evidence records; audit metadata now records source and evidence count.
- Replaced the generic create form with `ConstructionReportFormDialog.tsx`; its hierarchy matches the detail dialog and supports structured Đã/Đang/Sẽ làm, blockers, neutral notes, crew, materials, source work item, existing evidence selection and immediate work-item evidence upload.
- Updated the shared report view model so manual structured reports are not labeled as workflow reports, and neutral notes are excluded from the red timeline problem card when canonical blockers exist.
- The detail dialog now always shows `Minh chứng · N`; historical reports without linked IDs display the explicit `Minh chứng · 0` empty state instead of hiding the section. The create CTA now requires `operation.report`, matching backend enforcement.
- Added focused backend regression coverage for structured/source/evidence persistence and mismatched work-item evidence rejection. No migration was added because V53 already owns all required columns; workflow-form bridge behavior remains unchanged and legacy summary rows remain readable.
- Updated central/backend/frontend/root-cause/construction/workflow mapping references for the typed manual-report contract, component ownership, RBAC, evidence invariants and workflow boundary.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Browser verification was skipped per repository rule because the user did not request it.

## Construction Report Evidence Capture And Manual Edit

Status: complete

Mục đích chính:
- Cho phép người dùng đính kèm minh chứng bằng tệp hoặc camera ngay trong tạo mới, xem chi tiết và chỉnh sửa báo cáo thi công thủ công, với một luồng nhất quán và truy vết về đầu việc nguồn.

Will be done:
- Luôn hiển thị `Chọn tệp` và `Mở camera`; tệp/ảnh được giữ trong hàng đợi cục bộ trước khi báo cáo có đầu việc nguồn.
- Dùng camera thật qua `navigator.mediaDevices.getUserMedia`, có đổi camera, chụp lại, dùng ảnh và fallback sang file input có `capture` khi trình duyệt không cấp camera.
- Thêm cập nhật báo cáo thủ công và upload minh chứng theo báo cáo; khi có tệp chờ tải, đầu việc nguồn là bắt buộc.
- Cho phép `Sửa báo cáo` và `Thêm minh chứng` từ form chi tiết của báo cáo thủ công; báo cáo do workflow-form sinh ra tiếp tục chỉ đọc và phải chỉnh tại đầu việc nguồn.
- Giữ work-item evidence permission, giới hạn 50 minh chứng, audit và schema V53; không thêm migration, không thay lifecycle workflow/quality/acceptance/stock/finance.
- Cập nhật central/backend/frontend/root-cause/construction/workflow mappings; chạy backend format/tests, frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `Approve`.
- Confirmed create currently hides all upload controls until a source work item is selected, the camera icon is decorative, detail is read-only, and no daily-report update/report-evidence endpoint exists.
- Confirmed existing evidence persistence remains work-item scoped and report rows already have `work_item_id`, `photos`, `structured_json` and `primary_evidence_id`; no database migration is required.
- Added permission-scoped `PUT /daily-reports/:id` and `POST /daily-reports/:id/evidence/upload`; create/update share one normalizer, preserve project/reporter identity and legacy resource summaries, enforce same-project source/evidence plus the 50-record cap, and emit update/evidence-link audit events.
- Kept report uploads on the existing work-item evidence storage/permission path. The report-scoped upload attaches the resulting evidence ID to `photos`, assigns the source/primary evidence when absent, and rejects direct update/upload for workflow-form-backed rows.
- Added `ConstructionEvidenceCapture.tsx` with always-visible multi-file selection, local pre-save queue, image previews, editable names/captions, real `getUserMedia` camera, facing-mode switch, capture/retry/use flow and a mobile/system-camera `capture="environment"` fallback.
- Reused the capture component in create, edit and the evidence-only quick-add dialog. Manual report detail now exposes `Sửa báo cáo` and `Thêm minh chứng`; workflow-backed detail remains read-only and points to its source work item.
- Added a report edit model that preserves structured rows and legacy summaries, and kept explicit partial-upload feedback so a saved report is not duplicated when a later file upload fails.
- Added focused DB-backed regression coverage for manual update, report/work-item evidence linking, audit events, source mismatch and workflow read-only gates. The focused test passed against the running Docker PostgreSQL instance via its LAN-bound port.
- Updated central/backend/frontend/root-cause/construction/workflow mappings. No migration or other domain mapping was required because schema V53 and workflow/quality/acceptance/stock/finance lifecycles were unchanged.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass (DB-backed tests skip without `CONSFLOW_TEST_DATABASE_URL`).
- Ran the focused DB-backed daily-report regression test with `CONSFLOW_TEST_DATABASE_URL`: pass.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because the user did not request it.

## Construction Daily Report Detail Dialog

Status: complete

Mục đích chính:
- Giúp người dùng hiểu trong 5–10 giây tình hình một ngày thi công: tiến độ, đã/đang/sẽ làm, blocker, ghi chú hiện trường, nguồn lực, minh chứng và nguồn báo cáo.

Will be done:
- Thay definition-list chung bằng dialog chi tiết báo cáo chuyên biệt, rộng trên desktop và một cột trên mobile.
- Dùng chung một view-model cho timeline và dialog; tách blocker có cấu trúc khỏi ghi chú `issues_summary` tự do để không tô đỏ câu như `Không có sự cố an toàn`.
- Hiển thị nhân công, vật tư, minh chứng và liên kết đầu việc nguồn khi dữ liệu hiện hữu; giữ dialog chỉ đọc.
- Tái sử dụng `/projects/:id/evidence`, `/evidence/:id/file` và project operations cache; không thêm migration hoặc thay đổi lifecycle/API ghi báo cáo.
- Cập nhật frontend/construction mappings, chạy frontend build, backend test phù hợp và `git diff --check`; không chạy browser verification nếu người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `Approve`.
- Confirmed the current report detail is the generic `RecordDetailDialog` definition list, while the report row already carries structured answers, source IDs and evidence IDs.
- Confirmed project evidence metadata/file routes and project operations work-item metadata already exist, so the approved detail can remain a frontend/read-model change.
- Added `ConstructionReportDetailDialog.tsx` as a responsive 1080px read-only workspace with date/weather/source context, progress bar, four work cards, resources, neutral field notes, linked evidence and source work-item navigation.
- Added `constructionReportViewModel.ts` and reused it from the timeline and detail dialog so structured and legacy daily reports share one parser; structured blocked items/blockers are separated from free-text `issues_summary` in the detail.
- Reused the permission-scoped project evidence read, protected evidence gallery/file behavior, project operations cache and existing work-item return navigation; no API contract, backend handler, lifecycle, migration or database mapping changed.
- Added the shared `Dialog` `report` width and updated central/frontend/construction mapping files for the new component ownership, read-model invariant and evidence/source behavior. No backend-function map update was required because backend behavior and contracts are unchanged.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran `cd backend && go test ./...`: pass.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because the user did not request it.

## Construction Daily Report Related Work Wording

Status: complete

Mục đích chính:
- Thay thuật ngữ kỹ thuật `Đầu việc nguồn` bằng wording người dùng dễ hiểu `Công việc liên quan` trong toàn bộ luồng Báo cáo thi công, đồng thời giữ nguyên liên kết `work_item_id`, evidence, permission và API.

Will be done:
- Đổi wording trong form tạo/sửa, dialog thêm minh chứng, chi tiết báo cáo và cảnh báo tải tệp của module `operations`.
- Đồng bộ thông báo validation backend của riêng Báo cáo thi công để lỗi trả về không lộ lại wording cũ.
- Giữ nguyên module Nghiệm thu, payload, route, trạng thái, persistence và workflow-form gates.
- Cập nhật central/frontend/construction mappings với thuật ngữ UI mới.
- Chạy tìm kiếm chuỗi, frontend build, backend tests và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `Duyệt, làm đi`.
- Loaded the mandatory central/frontend/construction mappings and confirmed the change is wording-only over the existing source-work-item/evidence contract.
- Confirmed the target files overlap a dirty worktree; patched only the approved user-facing strings and preserved unrelated changes.
- Replaced `Đầu việc nguồn` and adjacent technical wording with `Công việc liên quan`, `Công việc`, `Chọn công việc` and `Không chọn công việc` across create/edit, evidence capture, evidence-only add and report detail states.
- Updated the daily-report backend validation messages to use the same terminology while preserving `work_item_id`, permission checks, evidence ownership, the 50-record cap and workflow-form read-only gates.
- Left the separate Acceptance wording unchanged as approved; no route, request/response type, migration, seed or domain lifecycle changed.
- Updated central, frontend and construction-domain mappings with the user-facing terminology and unchanged internal contract.
- Confirmed the Operations module has no remaining old source-work-item wording; the one remaining `Đầu việc liên kết` backend string belongs to a separate out-of-scope flow.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran `cd backend && make test`: pass.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because the user did not request it.

## Acceptance Centered Detail And Checklist Inspection

Status: complete

Mục đích chính:
- Đặt nội dung biên bản nghiệm thu ở giữa vùng làm việc, giải thích rõ trạng thái hiện tại và cho phép xem đầy đủ từng tiêu chí checklist mà không làm thay đổi lifecycle nghiệm thu.

Will be done:
- Thay master-detail có cột danh sách cố định 360px bằng bố cục một cột căn giữa; danh sách biên bản trở thành bộ chọn gọn phía trên nội dung chi tiết.
- Bỏ ba ô tiến trình dễ bị hiểu nhầm là thao tác; thay bằng mô tả trạng thái và hành động tiếp theo theo `draft`, `submitted`, `completed` và kết quả nghiệm thu.
- Thêm thao tác `Xem` mở shared dialog chứa đầy đủ chi tiết checklist item và cho phép mở minh chứng liên kết khi có.
- Giải thích rõ checklist được lập trong chính biên bản; công việc, biểu mẫu và PDF chỉ là liên kết tham chiếu, không tự động sinh tiêu chí.
- Giữ nguyên API, schema, payload, RBAC, audit, acceptance/rework/handover lifecycle và shared outgoing-acceptance editor contract.
- Cập nhật central/frontend/quality mappings; chạy frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved all four review points with `OK triển khai`.
- Loaded the mandatory central/backend/frontend/root-cause mappings, quality/acceptance domain guidance, frontend workflow, impact analysis and git workflow.
- Confirmed the worktree is clean on `main`; no existing user changes overlap the target files.
- Confirmed the layout shift is caused by the nested 360px acceptance list, the progress boxes are presentation-only, and checklist rows have no detail interaction despite the detail API already returning all item fields.
- Confirmed runtime checklist rows are authored through `AcceptanceEditor` and persisted in `acceptance_checklist_items`; work item/form/PDF links do not auto-generate criteria. The Golden Project rows are explicit seed fixtures.
- Replaced the nested 360px master-detail grid with a centered 1180px single-column workspace. Searchable acceptance records now sit in a compact bounded selector above the selected detail, so the record content no longer starts after a second left rail.
- Replaced the non-interactive `Soạn biên bản` / `Đã gửi` / `Kết luận` stepper with state-specific guidance for draft, submitted, pass, conditional-pass and fail while preserving the existing single next-action CTA and permission checks.
- Added responsive checklist inspection: desktop rows expose `Xem`, mobile uses stacked cards, and a shared `lg` dialog displays result, severity, required action, due date, photo/evidence requirements, note, supplemental metadata and the acceptance source.
- Added protected linked-evidence opening through the existing `downloadBlob` client and `/evidence/:id/file`; photo URL fallback remains available when the checklist stores only a photo URL.
- Added source guidance in both the read-only checklist and shared editor so project and work-item acceptance flows explain that checklist criteria are authored in the acceptance while work item/form/PDF links remain references.
- Updated central/frontend/quality mappings for the centered workspace, status guidance, checklist detail dialog, evidence opening and checklist-source invariant. No backend-function, root-cause or migration mapping change was required because routes, contracts, persistence and lifecycle behavior are unchanged.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because the user did not request it.

## Acceptance Selector And Detail Width Alignment

Status: complete

Mục đích chính:
- Canh mép trái/phải của nội dung biên bản và checklist trùng với bộ chọn/search list phía trên trên mọi breakpoint.

Will be done:
- Bỏ responsive outer padding khỏi wrapper nội dung chính; giữ nguyên padding nội bộ của từng card.
- Áp dụng cùng hành vi cho project route và global route đang dùng chung `AcceptancePage`.
- Giữ nguyên API, schema, payload, RBAC, audit và acceptance/rework/handover lifecycle.
- Cập nhật frontend mapping; chạy frontend build và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved all three review points with `OK triển khai`.
- Confirmed the selector and detail already share the same 1180px parent; the detail-only `p-3 sm:p-4 lg:p-5` wrapper is the exact source of the unequal outer width.
- Confirmed both `/projects/:id/acceptance` and `/acceptance` render the same `AcceptancePage`, so one scoped presentation change covers both routes.
- Removed the detail wrapper's responsive outer padding while retaining every card's own internal spacing and borders.
- Updated the frontend ownership map to record that the selector and selected-detail sections share one outer width.
- No central, backend, root-cause, quality-domain or migration mapping update was required because route ownership, domain invariants, contracts, persistence and lifecycle behavior are unchanged.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because the user did not request it.

## Acceptance Editor And Multi-Evidence Criteria

Status: complete

Mục đích chính:
- Giúp người lập biên bản nghiệm thu nhập thông tin theo đúng thứ tự công việc, tập trung vào từng tiêu chí và đính kèm minh chứng ngay tại tiêu chí bằng camera, tệp mới hoặc hồ sơ công trình đã có.

Will be done:
- Đổi wording riêng của Nghiệm thu từ `Đầu việc nguồn` thành `Công việc liên quan` trong UI và validation backend, giữ nguyên field `work_item_id`.
- Thay checklist dạng bảng ngang bằng danh sách tiêu chí và vùng chỉnh sửa tiêu chí đang chọn; giữ một CTA chính `Gửi nghiệm thu` cùng `Lưu nháp` phụ.
- Cho phép mỗi tiêu chí có tối đa 10 minh chứng; giữ `evidence_id` làm minh chứng chính tương thích và thêm association table bằng migration V67.
- Cho phép chụp ảnh, chọn PNG/JPG/WebP/PDF hoặc chọn minh chứng hiện hữu qua `/projects/:id/evidence` với search/filter/cursor pagination.
- Tệp chờ được giữ cục bộ; phải chọn `Công việc liên quan` trước khi upload qua route work-item evidence hiện hữu. Minh chứng hiện hữu có thể chọn khi chưa chọn công việc, nhưng phải cùng công việc nếu biên bản đã chọn một công việc.
- Giữ nguyên lifecycle draft/submitted/completed, signature/PDF gates, RBAC, audit, rework và handover.
- Cập nhật central/backend/frontend/root-cause/quality/construction mappings; chạy backend format/tests, frontend build, migration directory check và `git diff --check`; không chạy browser verification vì người dùng không yêu cầu.

Done:
- User approved the review package and Mockup as Text with `OK triển khai`.
- Loaded the mandatory central/backend/frontend/root-cause mappings plus frontend, backend, database, quality/acceptance, construction, impact-analysis and git workflows.
- Confirmed the current contract stores only one `acceptance_checklist_items.evidence_id`, validates evidence against the project/optional work item and renders the editor as a 1120px horizontal grid.
- Confirmed `/projects/:id/evidence` already supports permission-scoped server search/filter/cursor pagination and `/work-items/:id/evidence/upload` already owns camera/file persistence and permission checks.
- Confirmed migration V66 is the current maximum; the new association schema will be additive V67 and will backfill every legacy `evidence_id`.
- Confirmed unrelated dirty changes in app branding files are outside this plan and will be preserved untouched.
- Replaced the horizontal 1120px checklist grid with compact record metadata, a responsive criterion list and a focused selected-criterion editor. Conditional/fail remediation fields appear in context, while signature/PDF/form/notes remain available under supporting details.
- Renamed Acceptance user-facing source wording to `Công việc liên quan` and `Không chọn công việc` in the shared project/work-item editor, read-only detail and acceptance-specific backend validation while preserving `work_item_id`.
- Moved the existing PNG/JPG/WebP/PDF validation, local queue and real/system camera implementation into shared `modules/_shared/EvidenceCapture.tsx`; Operations keeps a compatibility re-export and Acceptance reuses the same implementation without changing upload storage.
- Added `AcceptanceEvidencePicker.tsx` backed by permission-scoped `/projects/:id/evidence` search/type/status filters and cursor pagination. Rejected evidence is excluded; selecting a related work item scopes the picker and frontend/backend validation to that work item.
- Added partial-upload-safe orchestration: queued files require a related work item, upload sequentially through `/work-items/:id/evidence/upload`, update local draft progress after each success and never recreate evidence that already uploaded when a later file fails.
- Added V67 `acceptance_checklist_item_evidence` with ordered associations, legacy backfill and cascade only from checklist item to association. Service/repository contracts now accept/return `evidence_ids` and `evidence_records`, enforce 10 unique records per criterion, validate same project/optional same work item/non-rejected status and mirror the first association into legacy `acceptance_checklist_items.evidence_id`.
- Extended the read-only checklist table/cards and item-detail dialog to show counts and open every protected linked evidence record while retaining legacy `photo_url` fallback.
- Added focused unit coverage for ordered de-duplication/10-record limit and extended the DB regression to persist/read two evidence records while preserving the primary compatibility ID through submit, fail/rework and handover behavior.
- Updated central/backend/frontend/root-cause/quality/construction mappings for V67, editor/picker/shared-capture ownership, wording, API reuse and evidence invariants.
- Ran `go run ./cmd/migrate -check`: pass with 67 migrations.
- Ran `GOCACHE="$PWD/.gocache" go test ./...`: pass.
- Applied all V1-V67 migrations to a fresh disposable PostgreSQL database: 67 applied, 0 skipped.
- Ran focused DB-backed `TestOutgoingAcceptanceSubmitLocksAndFeedsReworkAndHandover`: pass; the disposable database/container was removed afterward.
- Applying V67 to the existing local Compose database was skipped after the custom runner proved the pre-existing volume lacks its configured `consflow` role; no role/database mutation was attempted.
- Ran `cd frontend && npm run build`: pass; Vite reported only the existing large-chunk advisory.
- Ran `git diff --check`: pass.
- Browser verification was skipped per repository rule because the user did not request it.
