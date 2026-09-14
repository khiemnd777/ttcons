# [Closed] My Work compact row action buttons

Closed: 2026-09-14

Status: IMPLEMENTED; TECHNICALLY VERIFIED; NOT UX ACCEPTED.

## Mục đích chính

Giữ danh sách `/my-work` gọn và dễ quét bằng cách ngăn CTA chữ bị kéo cao theo toàn bộ chiều cao dòng.

## Delivered

- Giữ nguyên container và vùng chọn dòng, nhưng đặt CTA chữ cao 36px và căn giữa từ breakpoint ngang.
- Áp dụng nhất quán cho thiết lập công trình, chuyển Giai đoạn sẵn sàng và hồ sơ kế toán trong hàng đợi hợp nhất.
- Giữ nguyên nhãn, màu, icon, trạng thái loading/disabled, click handler, queue semantics và nút icon-only mở Đầu việc.
- Bổ sung regression contract và cập nhật central/frontend/root-cause mappings.

## Verification

- `cd frontend && node --experimental-strip-types --test tests/myWorkListPresentation.test.ts` passed (18/18).
- `cd frontend && npm run build` passed with Vite 6.4.3 and 1,947 modules transformed.
- `git diff --check` passed before closure.

## Scope limitation

- Browser/UAT không chạy theo quy định dự án vì người dùng chưa yêu cầu trong lượt triển khai; trạng thái UX là `NOT UX ACCEPTED`.
- Không có thay đổi backend, API, database hoặc migration.
