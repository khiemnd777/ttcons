# [Closed] Đưa điều phối khởi tạo Công trình vào Bàn làm việc

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED. Closed by user request on 2026-08-26.

## Objective

Sau khi tạo một Công trình thật, hệ thống phải xác định người chủ trì thiết lập, chỉ rõ vai trò/người sẽ nhận bước đầu, phát thông báo đúng người và đưa hành động authoritative vào `Bàn làm việc` cho đến khi Pipeline được khởi chạy.

## Will be done

- Lưu `projects.created_by` và chuyển tạo Công trình đơn/import sang transaction có audit + notification outbox idempotent.
- Thêm projection actor-scoped cho việc thiết lập Công trình và readiness project-scoped; không tạo Work Item giả trước runtime.
- Thêm mutation chỉ định PM có validation membership/RBAC, audit, chuyển trách nhiệm và resolve notification cũ trong cùng transaction.
- Khi Pipeline chưa chạy, creator/PM thấy action cần thiết lập; người đủ vai trò bước đầu thấy mục `Sắp tới · chỉ xem`.
- Khi Pipeline khởi chạy hoặc chuyển Node, Work Item thật vừa `ready` phát notification đến actor hiện có thể xử lý.
- Tích hợp count, deep link, loading/error/empty/responsive states vào `Bàn làm việc`; tạo đơn mở thẳng setup, import phát một notification tổng hợp.
- Bổ sung migration/tests và cập nhật central/backend/frontend/project/workflow/admin mapping.

## Done

- Review package v2 và Mockup as Text đã được duyệt bằng `OK triển khai`.
- Đã xác nhận PostgreSQL + custom SQL migration runner, route/service/repository owners và notification outbox hiện có; trước thay đổi, các candidate assignment/PM chưa được nối vào transaction nghiệp vụ.
- V130 đã thêm `projects.created_by`, index startup và bật các event/template `project.setup_*`, `project.setup_batch_required`, `work_item.assigned`; migration đã áp dụng local và `migrate-check` kiểm tra đủ 130 checksum.
- Tạo Công trình đơn hiện audit + emit startup trong cùng transaction; import lưu creator và phát một notification tổng hợp. Project creator được đưa vào project scope trong thời gian chịu trách nhiệm.
- Đã thêm `/me/project-startup-actions`, `/projects/:id/startup-readiness`, mutation PM audit-atomic và gate Pipeline cho Công trình thật mới. Legacy/null-creator và sample không bị gate mới.
- Pipeline start/advance emit notification cho Work Item vừa `ready`; audience dùng assignment vận hành thực tế và không broadcast theo global Admin override.
- `Bàn làm việc` đã có row/inspector/dialog setup, deep link `?setup=`, checklist PM/template/role-permission coverage, CTA quản lý thành viên và bắt đầu Pipeline; tạo đơn chuyển thẳng về setup.
- Trạng thái source hiện tại đã đạt `backend make test`, 279 frontend tests, frontend production build và bundle gzip budget. Hai DB integration tests mới đã đạt trên PostgreSQL local, bao gồm create/block, PM handover/readiness/start, real Work Item và exact-recipient notification; `migrate-check` đạt đủ 130 checksum.
- Central/backend/frontend/project/workflow/admin mapping đã được cập nhật theo hành vi V130.

## Skipped / remaining acceptance

- Browser/UAT không được yêu cầu trong lượt triển khai và vẫn là `NOT UX ACCEPTED`.
- Docker backend local chưa được rebuild/restart sau thay đổi; source và migration đã được kiểm chứng nhưng runtime `localhost` có thể vẫn chạy binary cũ cho đến lần restart tiếp theo.
