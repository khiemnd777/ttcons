# [Closed] Icon ngữ nghĩa cho menu trái Project Workspace

Status: IMPLEMENTED; TECHNICALLY VERIFIED; NOT UX ACCEPTED

Mục đích chính:
- Giúp người dùng nhận biết nhanh từng khu vực nghiệp vụ trong cây menu Project bằng icon riêng thay cho các biểu tượng folder giống nhau.

Done:
- Review package, bộ icon và Mockup as Text đã được người dùng duyệt.
- Đã bổ sung icon Lucide riêng cho Project, toàn bộ nhóm và mục con trong Project Workspace.
- Renderer tiếp tục dùng trạng thái active/hover/badge/RBAC hiện hữu; Project Explorer và Workflow sidebar giữ folder icon theo phạm vi đã duyệt.
- Đã bổ sung contract test cho toàn bộ icon map và entrypoint render, đồng thời cập nhật mapping trung tâm, frontend và Project Planning.
- `npm run test:project-navigation` đạt 6/6; frontend production build đạt.
- Bundle budget đạt với entry 432.91 kB raw/114.8 KiB gzip và chunk điều hướng Project 7.58 kB raw/2.4 KiB gzip; `git diff --check` đạt.

Scope limitation:
- Browser/UAT không chạy; trạng thái UX là `NOT UX ACCEPTED`.
- Full frontend suite còn đúng một lỗi source-contract đã tồn tại ngoài phạm vi tại `projectEvidenceColumns.test.ts`; `EvidenceGallery.tsx` và test đó không bị thay đổi trong plan này.
