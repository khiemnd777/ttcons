# [Closed] Loại bỏ phạm vi `Giao trực tiếp` khỏi Bàn làm việc

Status: IMPLEMENTED AND TECHNICALLY VERIFIED; UX NOT ACCEPTED. Closed by user request on 2026-08-26.

## Mục tiêu

Đơn giản hóa cấu trúc phạm vi của Bàn làm việc: người dùng chỉ cần `Tôi phụ trách` cho toàn bộ việc mình chịu trách nhiệm, không phải phân biệt thêm một phạm vi `Giao trực tiếp` có dữ liệu giao thoa và dễ gây hiểu nhầm.

## Will be done

- Bỏ `Giao trực tiếp` khỏi danh sách phạm vi hiển thị trên desktop và màn hình hẹp.
- Giữ toàn bộ Work Item giao đích danh trong `Tôi phụ trách` theo predicate `responsible` hiện hành.
- Chuẩn hóa URL cũ `queue=mine` về phạm vi mặc định `Tôi phụ trách` và xóa query tương thích khỏi URL.
- Giữ backend `queue=mine` và count `mine` để tương thích API; không xóa dữ liệu seed/sample, không đổi schema hoặc runtime assignment.
- Cập nhật regression test và mapping; chạy toàn bộ frontend tests/build. Browser/UAT chỉ chạy khi được yêu cầu riêng.

## Done

- Review package và Mockup as Text đã được duyệt bằng `OK triển khai`.
- Regression fail-before xác nhận contract cũ còn render `Giao trực tiếp`; pass-after xác nhận rail chỉ còn bốn phạm vi hiển thị và `WorkScopeKey` vẫn giữ `mine` cho tương thích backend.
- Đã bỏ lựa chọn `Giao trực tiếp` khỏi `MyWorkPage`; `responsible` tiếp tục bao gồm giao đích danh, vai trò phù hợp và pool phòng ban chưa có người nhận theo đúng backend hiện hành.
- URL cũ `queue=mine` được thay thế về URL mặc định và chọn `Tôi phụ trách`; không có backend, database, seed/sample, notification, RBAC hoặc workflow mutation nào bị thay đổi.
- Working tree hiện tại đã pass 285/285 frontend tests, production build và bundle budget. Browser/UAT không chạy theo giới hạn xác minh của repo nên UI vẫn ở trạng thái `NOT UX ACCEPTED`.
