# Tiện ích mobile công trình — Development,13-10-2026

**Giới hạn:** đang chờ anh review/chốt mobile. Nghiệm thu dùng viewport trình duyệt; điện thoại thật, safe-area/keyboard, capacity/all-role và Google import mới chưa chạy. Một quan sát riêng tại MyWork390: documentWidth663px; chưa điều tra nguyên nhân, không gọi là lỗi có trước hoặc lỗi do bản sửa. Không tuyên bố toàn bộ UI standard/mobile đã đạt.

**Đã kiểm chứng:** phương án A đưa cùng nút Tiện ích vào hàng riêng của sticky header công trình, giữ nguyên chữ và số đếm động. Nút không còn che Gửi khắc phục tại320/390/430/639px; actual CTA mở đúng dialog, Hủy an toàn. Cột desktop đã chốt giữ nguyên. Bản nháp trao đổi chưa gửi giữ nguyên qua chuyển trang. Hai file báo cáo và dữ liệu/lịch sử được bảo toàn trong các baseline đã kiểm.

## Source và môi trường

- User duyệt A/mockup/mục1–3, không field mới, giữ nhãn/dữ liệu/lịch sử, Development/browser/screenshots. Không backend/migration/Production/Git delivery/đóng plan.
- HEAD75dc44707785d399ed2408959bf784928a544e96 cùng working tree đang có được giữ nguyên. Ba app owners thay đổi: AppShell.tsx, ProjectWorkspaceLayout.tsx, UtilityDock.tsx; thêm UtilityDockLauncherHost.tsx. Một existing source-contract test và một new controlled lifecycle suite.978 current source/test/migration hashes khớp manifest build; ConstructionWorkspaceLists đã nghiệm thu không thay đổi.
- Development admin chạy Vite5173, bind frontend hiện tại vào/app; cập nhật HMR và browser Reload thực tế. Backend8081/readiness200, database consflow/V158 giữ nguyên. Không full restart hoặc migration; không đổi Makefile/Compose.
- Medium build `cd frontend && npm run build` PASS, Vite4.42s; completion được kiểm tra10:56:48ICT13 October. Log thật frontend-build.log lưu trong evidence. Root không giả lập build log. High independent source review không có P0–P2 mới trong approved placement scope.

## Thay đổi và tái sử dụng

Mục đích chính là giữ các thao tác hiện trường không bị nút phụ che. Nguyên nhân cũ: project layout thiếu header host, UtilityDock rơi về launcher fixed44px tại76px + safe-area trên đáy màn hình. Bằng chứng trước sửa390×844 giao với CTA101,83×27,5px được giữ ở báo cáo desktop trước.

Shared UtilityDockLauncherProvider/context và callback ref đăng ký host khi header mount; cleanup kiểm đúng registration để host cũ không xóa host mới. AppShell giữ cùng provider type và keyed UtilityDock qua hai nhánh. Project header thêm một hàng dưới sm với presentation label; standard header vẫn icon-only, desktop/canvas giữ cách hiển thị hiện có. Không thêm padding ngoài focus wrapper hoặc nhân bản launcher/handler.

Tái sử dụng createPortal, Sparkles, CountBadge, mobile handlers và UtilityWindowHeader/UtilityWorkPanel/ProjectConversation. Nút CTA và xuất tiếp tục dùng owners ConstructionActionDialogs/NativeExcelExportButton. Không API/data shape, permission, field, source-state hoặc history rewrite.

## Kiểm tra đã chạy

```sh
cd frontend
node --experimental-strip-types --test tests/utilityDockLauncherLifecycle.test.ts tests/utilityDock.test.ts tests/appShellNavigation.test.ts tests/projectWorkspaceCompactHeader.test.ts tests/workItemFocusLayout.test.ts tests/projectExplorerPagination.test.ts tests/preparationWorkspaceResponsive.test.ts
npm run build
```

Medium31/31 PASS; High independently31/31 PASS279,876ms. Các test lifecycle dùng controlled hooks/ref/render-tree simulation; không gọi đó là real DOM/StrictMode integration. Root browser bổ sung actual source/control/draft proof sau build.

| Scenario | Quan sát |
|---|---|
| 320×568,390×844,430×932,639×844 cuối trang | Một launcher có nhãn trong header, cao44px; CTA hiển thị/hit test đúng; giao0px; documentWidth bằng viewport |
| 390 cuộn giữa | scrollY164,5; header giữ vị trí; không giao CTA |
| 320 Tiện ích mở | Khung hiện có vừa màn hình, Thu nhỏ hoạt động; khung mở chủ động phủ nội dung |
| 640 breakpoint | Mobile launcher ẩn, utility desktop hiện có giữ nguyên; CTA không bị che |
| 1024/1280 desktop | Cột thao tác vẫn244,48px, title gap24px/overlap0 |
| Project→standard→Back | Một launcher; standard icon40px, project trở lại full label; work panel vẫn mở |
| Draft trao đổi thử | Nội dung synthetic chưa gửi giữ nguyên qua chuyển panel/SPA route; đã clear rồi đóng tab thử |
| Empty Đã xong0 | Header Tiện ích giữ nguyên; export disabled |

Actual Gửi khắc phục đã mở đúng phiếu nguồn, sau đó Hủy; không upload/lưu/Gửi xác nhận/chuyển trạng thái. Không bấm Gửi trao đổi. Nguyên bản draft0messages được giữ; ảnh trước/sau và JSON xác nhận local draft bằng nhau, đã clear.

Hai Downloads thật: danh-sach-can-xu-ly-BM-GS-04(5).xlsx và BM-GS-04(5).xlsx. Bản lưu giữ nguyên byte tải; toàn bộ ô/công thức/media bằng file ở lượt desktop trước đã kiểm Google. Không dùng CSS change để tuyên bố Google đã được nghiệm thu lại.

Root verifier24/24PASS;2.889 unique bản ghi baseline không đổi/không mất; chỉ hai audit native_source_exported mới.149file storage/V158 giữ nguyên. Đây là baseline39business/workflow tables cộng construction/audit, không phải tuyên bố kiểm mọi bảng hệ thống. High import-only24/24 và xem8representative screenshots xác nhận source/history/file/draft/geometry scope; reviewer không chạy browser hoặc ghi dữ liệu.

Một locator Cần xử lý trùng tên giữa workspace/status tabs đã được giới hạn đúng tablist. Một thao tác generic backdrop-close mở linked readonly check; root Reload để quay về nguồn. Không coi các thao tác đó là kiểm chứng dialog-close hoặc tự sửa owner ngoài phạm vi.

## Bằng chứng và bước tiếp theo

Evidence: docs/acceptance-evidence/bm04-mobile-utility-development-2026-10-13/.

- [Mobile390 cuối trang](acceptance-evidence/bm04-mobile-utility-development-2026-10-13/mobile-390-end.jpg), [320 màn hình thấp](acceptance-evidence/bm04-mobile-utility-development-2026-10-13/mobile-320-end.jpg), [header390](acceptance-evidence/bm04-mobile-utility-development-2026-10-13/mobile-390-top.jpg).
- [Draft trước](acceptance-evidence/bm04-mobile-utility-development-2026-10-13/mobile-390-draft-before-navigation.jpg), [draft sau](acceptance-evidence/bm04-mobile-utility-development-2026-10-13/mobile-390-draft-after-navigation.jpg), [draft JSON](acceptance-evidence/bm04-mobile-utility-development-2026-10-13/browser-draft-preservation.json).
- [Measurements](acceptance-evidence/bm04-mobile-utility-development-2026-10-13/browser-measurements.json), [verifier](acceptance-evidence/bm04-mobile-utility-development-2026-10-13/verify.py), [kết quả](acceptance-evidence/bm04-mobile-utility-development-2026-10-13/verification.json), [build log](acceptance-evidence/bm04-mobile-utility-development-2026-10-13/frontend-build.log).
- [File danh sách](acceptance-evidence/bm04-mobile-utility-development-2026-10-13/BM04-list.xlsx), [file chi tiết](acceptance-evidence/bm04-mobile-utility-development-2026-10-13/BM04-detail.xlsx).

Central/frontend/triage/ProjectPlanning mappings cập nhật actual header-host/lifecycle owner. Backend/migration/business invariant không thay đổi nên không sửa các map đó. Viewport reset; không chạy full backend suite hoặc gửi dữ liệu thử. Standard Project-list screenshot ghi lúc lazy content loading, chỉ dùng chứng minh icon header; không chấm layout toàn danh sách. MyWork663px là quan sát ngoài scope chưa điều tra; không che giấu hoặc suy ra nguyên nhân từ source hash.

**Bước tiếp theo:** anh review ảnh390/320 để chốt riêng mobile Tiện ích. Không tự nâng trạng thái thành UX ACCEPTED; desktop acceptance giữ nguyên. MyWork overflow cần review/diagnosis riêng nếu được yêu cầu.
