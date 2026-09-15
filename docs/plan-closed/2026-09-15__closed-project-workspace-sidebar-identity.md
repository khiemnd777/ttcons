# [Closed] Bỏ nhận diện trùng lặp khỏi cây nghiệp vụ

Status: IMPLEMENTED; TECHNICALLY VERIFIED; NOT UX ACCEPTED

Mục đích chính:
- Dọn phần nhận diện công trình lặp lại trong Cây nghiệp vụ, để sidebar bắt đầu ngay từ các nhóm điều hướng và header sticky là nguồn ngữ cảnh duy nhất.

Done:
- Review package và Mockup as Text đã được người dùng duyệt ngày 15-09-2026.
- Đã bỏ thẻ chứa tên/mã công trình cùng `Building2` và props chỉ phục vụ thẻ đó khỏi `ProjectNavigationSidebar` dùng chung desktop/mobile.
- Header sticky tiếp tục hiển thị code/tên, chủ đầu tư, người phụ trách và trạng thái công trình.
- Đã cập nhật regression coverage để sidebar không còn nhận `projectName`, `projectCode` hoặc render `Building2`.
- Đã đồng bộ module keyword map, frontend UI utility map và Project Planning domain map.
- Trên local working tree: `npm run test:project-navigation` đạt 6/6; `npm run build` đạt; `git diff --check` đạt.

Scope limitation:
- Không chạy browser/UAT trong lượt triển khai; trạng thái UX là `NOT UX ACCEPTED`.
- Không thay đổi route, quyền, badge, API, workflow, dữ liệu hay migration.
