# Cam kết thuật ngữ workflow Consflow

Trạng thái: ĐÃ ĐƯỢC NGƯỜI DÙNG DUYỆT NGÀY 2026-09-14.

## Cam kết chung

Người dùng và agent thống nhất dùng một hệ thuật ngữ duy nhất khi trao đổi, lập kế hoạch, viết tài liệu và thiết kế nhãn hiển thị cho workflow/pipeline Consflow:

| Khái niệm kỹ thuật hiện có | Thuật ngữ tiếng Việt đã thống nhất |
|---|---|
| Node | Giai đoạn |
| Step | Gói công việc |
| Work Item | Đầu việc |
| Tài chính công trình, khi là tên giai đoạn kết thúc workflow | Bàn giao công trình và hồ sơ kế toán |

Agent phải ưu tiên các thuật ngữ đã thống nhất trong mọi review package, Mockup as Text, kế hoạch, tài liệu mới và nội dung hiển thị được đề xuất sau ngày duyệt.

## Ranh giới bắt buộc

- Đây là cam kết về ngôn ngữ nghiệp vụ và nhãn người dùng; không phải yêu cầu đổi tên hàng loạt trong mã nguồn.
- Giữ nguyên các khóa kỹ thuật, API, schema, kiểu dữ liệu, tên hàm và mã nghiệp vụ ổn định như `node`, `step`, `work_item`, `pipeline_*`, `run_node_id`, `run_step_id`, `work_items`, `handover_finance`, `financial_closure`, `project_finance.*` và các event key liên quan.
- Không sửa migration đã áp dụng và không viết lại tên đã chụp trong Pipeline Run, Work Item, Workflow Form, audit hoặc PDF lịch sử.
- Việc đổi tên giai đoạn kết thúc không tự động đổi tên workspace/module độc lập `Tài chính công trình`; module đó chỉ đổi khi có một quyết định phạm vi riêng chứng minh tên mới đúng với toàn bộ chức năng của trang.
- Mọi thay đổi UI hoặc hành vi sản phẩm để triển khai cam kết này vẫn phải qua review package, Mockup as Text và xác nhận triển khai rõ ràng theo `AGENTS.md`.

## Nguồn ghi nhớ

Bản cam kết này được neo tại `AGENTS.md`, bản đồ module trung tâm và bản đồ miền Workflow Orchestration. Kế hoạch triển khai đang hoạt động được theo dõi trong `docs/current-states.md`.
