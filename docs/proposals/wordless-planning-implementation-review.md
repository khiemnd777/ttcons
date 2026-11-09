# Wordless cho Kế hoạch và chi tiết Đầu việc — bản triển khai chờ duyệt

Ngày: 09-11-2026. Trạng thái: APPROVED trong chat (“Duyệt bảng triển khai này.”); implementation local được theo dõi tại docs/current-states.md. Chưa UX ACCEPTED, chưa phát hành.

## Mục đích chính

Giúp người dùng tập trung vào hồ sơ và công việc bằng cách giảm chữ lặp, đưa diễn giải tùy chọn vào trợ giúp. Giữ cấu trúc nghiệp vụ, bố cục hai cột của chi tiết Đầu việc và bản giao diện hiện tại để quay lại khi cần.

## Phạm vi

- `/projects/:id/preparation`: Hồ sơ chuẩn bị, revision thiết kế và hàng đợi.
- `/projects/:id/planning`: Kế hoạch công việc, bàn giao giám sát, tổng quan Giai đoạn, hàng đợi và cột hỗ trợ.
- `/projects/:id/pipeline`: Quy trình runtime, danh sách Giai đoạn và Đầu việc.
- Chi tiết Đầu việc được mở trong luồng trên, gồm hồ sơ, điều kiện bắt buộc, cột hành động, ký/PDF và thông tin hỗ trợ. Không tự mở rộng sang tất cả entrypoint ngoài phạm vi.
- Thiết lập, Pipeline Builder, trình thiết kế Form, PDF xuất và dữ liệu lịch sử không thuộc đợt này.

## Quy ước URL bật/tắt

1. Cập nhật theo yêu cầu trực tiếp mới nhất của Khiêm ngày 09-11-2026: Wordless là mặc định trong các phạm vi đã triển khai. Không có parameter hoặc có đúng một `wordless=1`: dùng Wordless.
2. Có đúng một `wordless=0`: quay lại giao diện cũ. Giá trị không hợp lệ, rỗng hoặc parameter lặp tiếp tục dùng bản cũ để tránh diễn giải khác nhau.
3. Muốn quay lại bản cũ phải đặt `wordless=0`; bỏ parameter sẽ bật lại Wordless. Không tự redirect chỉ vì thiếu/không hợp lệ.
4. Không lưu lựa chọn vào database, localStorage, cookie hay tài khoản; URL là nguồn lựa chọn duy nhất.
5. Trong phạm vi: duy trì chế độ khi đổi subitem, chọn Giai đoạn, mở/đóng Đầu việc, chuyển trước/sau và quay về nguồn. Đọc chế độ từ URL đang hoạt động; không để `returnTo` cũ bật lại Wordless sau khi người dùng đã tắt.
6. Ghép query bằng URLSearchParams và helper điều hướng hiện có. Bảo toàn giá trị `task`, `stage`, filters và hash phù hợp với route đích. Giữ quy tắc lọc `returnTo` nội bộ và loại context cũ để không tạo returnTo lồng đệ quy. Không sao chép mù toàn bộ query sang route khác.
7. Mode ở outer URL và return path do ứng dụng tạo phải nhất quán. Không sửa execution_target của backend và không đổi API payload vì parameter này.
8. Ngoài phạm vi: không áp presentation Wordless chỉ vì URL có parameter. Chi tiết Đầu việc phải kiểm tra entrypoint thuộc phạm vi, không chỉ kiểm tra flag toàn cục.
9. Không thêm toggle/button/DDL mới trong sản phẩm. Người kiểm tra dùng URL để bật/tắt. Đổi route/query qua ứng dụng phải giữ guard nội dung chưa lưu hiện có; sửa thanh địa chỉ hoặc reload có thể làm mất draft bộ nhớ, không cam kết giữ draft qua reload.

Ví dụ:

```text
/projects/<id>/planning                     → Wordless (mặc định)
/projects/<id>/planning?wordless=1          → Wordless
/projects/<id>/planning?task=<id>&wordless=1 → đúng context Đầu việc + Wordless
/projects/<id>/planning?wordless=0          → hiện tại
```

## Ranh giới giữ nguyên component/style

- Không copy CSS hoặc controls HTML của prototype vào ứng dụng.
- Giữ style nút, icon, kích thước, màu, thứ tự CTA, disabled/loading, handler và điều kiện quyền hiện có.
- Giữ DDL, options, mặc định, vị trí và cơ chế lọc. Không giấu hai DDL Planning dưới nút Bộ lọc như prototype.
- Giữ WorkspaceTabs, bảng/cột/mobile cards, Dialog, accordion, field renderer, đầy đủ fields/DDL trong structured list, validation và standard evidence compositions.
- Không thêm bước xác nhận chốt hồ sơ từ prototype; giữ luồng thực tế.
- Giữ trạng thái, dữ liệu, số lượng, lỗi, cảnh báo, điều kiện chặn và hệ quả cần quyết định hiển thị trực tiếp.
- Không tự đổi theme, typography scale, spacing toàn cục, breakpoint hoặc ẩn cột như CSS prototype.

## Thay đổi chữ được đề xuất

| Bề mặt | Thay đổi đề xuất | Giữ nguyên |
| --- | --- | --- |
| Hồ sơ chuẩn bị | Bỏ nhãn phụ lặp; giải thích revision vào trợ giúp | Revision cards, lịch sử, tạo/tiếp tục/bàn giao theo quyền, cảnh báo chưa bàn giao lại |
| Kế hoạch công việc | Bỏ tagline; chuyển mô tả bàn giao/Giai đoạn vào trợ giúp; rút câu lặp việc tiếp theo | Thứ tự các khối, metrics, 3 DDL, bảng/mobile cards, cột hỗ trợ và CTA |
| Quy trình | Bỏ tiêu đề Quy trình lặp trong tổng quan; mô tả Đầu việc vào trợ giúp | Navigation Giai đoạn, tiến độ, phê duyệt, trạng thái và thao tác |
| Chi tiết Đầu việc | Một tên chính tại header dialog; giảm tên/tagline/headline lặp; ví dụ/hướng dẫn tùy chọn vào trợ giúp | Hai cột, điều kiện bắt buộc, form đầy đủ, trình tự ký/PDF, action rail, evidence/discussion/history |

Nếu workspace không nằm trong Dialog, vẫn phải có tên chính. Không bỏ metadata duy nhất hoặc toolbar trước/sau. Với shared components, chỉ áp nội dung thu gọn trong mode/phạm vi được duyệt.

## Trợ giúp và kiến trúc hai chế độ

- Tái sử dụng ContextHelp làm owner trợ giúp; đề xuất thêm mode tooltip opt-in với hover/focus, click/chạm, Escape/click ngoài, vị trí trong viewport. Giữ style trigger chuẩn và mặc định của consumer khác.
- Giữ nhánh trình bày hiện tại; Wordless là biến thể presentation tường minh. Dùng chung fetching, state, form answers, handlers, quyền và transitions; không clone toàn bộ trang hoặc form engine.
- Không dùng CSS toàn cục để giấu text. Không sửa schema/form definition đã phát hành hoặc snapshot để thay presentation.
- Quay lại bằng URL là rollback presentation; không hoàn tác nghiệp vụ đã lưu trong bất kỳ chế độ nào. Nếu lỗi nguồn dùng chung, vẫn có thể cần Git rollback; không hứa cách ly mọi lỗi bằng query flag.

## Owner và trình tự triển khai sau duyệt

Mapping đã đối chiếu: consflow-module-memory, frontend-ui-utility-map; domain Project Planning và Workflow Orchestration. Tìm kiếm trọng tâm: description, ContextHelp, WorkItemWorkspaceHeader, WorkItemActionPanel, WorkflowFormPanel, URLSearchParams, returnTo.

1. Helper nhận diện mode và truyền mode qua điều hướng có kiểm soát. Owner dự kiến: helper presentation nhỏ; workItemNavigation, workItemWorkflowNavigation, ProjectTaskContext và các callsite cần thiết.
2. Trợ giúp opt-in và consumer PreparationPage/DesignReleasePanel.
3. PlanningPage/ConstructionHandoffPanel và ProjectDetailPage::ProjectWorkflowWorkspace.
4. WorkItemDetailDialog, WorkItemPrimarySurface, WorkflowFormPanel: truyền presentation context tường minh, không ảnh hưởng entrypoint ngoài scope.
5. Cập nhật mapping liên quan, current-states và kiểm tra tích hợp. Không thay backend, migration hoặc deploy.

## Tiêu chí kiểm chứng

- URL: absent/1/0/empty/invalid/duplicate; back/forward; deep link task; stage; filters; hash; đóng dialog và returnTo. Parameter không thay quyền hoặc tự bắt đầu công việc.
- Mặc định: mode off hiển thị và xử lý như bản hiện tại; component/style/DDL/CTA giữ nguyên cả hai mode.
- Mode on: đúng đoạn chữ được chuyển; trợ giúp đọc được bằng chuột/bàn phím/chạm; Escape đóng đúng lớp; không che/cắt nội dung hoặc đẩy layout ngoài dự kiến.
- Nghiệp vụ: draft/submit/read-only/submitted, thiếu field/evidence, blocked/review, ký/PDF và lịch sử; không mất fields, dữ liệu hoặc cảnh báo. Đổi query trong app giữ dirty guard.
- Standard components: EvidencePanel và EvidenceRecordViewerDialog giữ nguyên; kiểm tra entrypoint thực tế, không chỉ assertion import. Giữ schema-driven fields và controls hiện có.
- Chạy focused frontend tests theo lệnh thực tế repo, frontend build và diff check sau implementation. Browser kiểm chứng cần yêu cầu rõ trong lượt thực hiện; hiện NOT_RUN, NOT UX ACCEPTED.

## Cần duyệt

1. Phạm vi bốn bề mặt và danh mục chỉ thay chữ/trợ giúp, giữ component/style/DDL như trên.
2. Quy tắc URL đã cập nhật: mặc định Wordless, `wordless=0` về bản cũ; truyền mode qua navigation/returnTo, không để returnTo cũ ghi đè lựa chọn outer URL.
3. Mở rộng ContextHelp opt-in; giữ nhánh hiện tại, không thay logic nghiệp vụ hoặc tự thêm controls/confirmations.

Khiêm đã duyệt các điểm trên trong chat ngày 09-11-2026. Phê duyệt triển khai local không tự cấp quyền browser, commit/push hoặc deploy.
