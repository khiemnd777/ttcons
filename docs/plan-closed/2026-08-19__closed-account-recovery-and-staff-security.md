# [Closed] Account Recovery And Staff Account Security

Closed: 2026-08-19
Reason: User requested closing all active plans.

## [Closed - Implemented, UAT pending] Hiển thị email khôi phục đã che trong Staff

### Will be done

- Browser/UAT chỉ chạy khi người dùng yêu cầu nên trạng thái tại thời điểm đóng là `NOT UX ACCEPTED`.

### Done

- Người dùng đã duyệt review package và Mockup as Text bằng câu `OK triển khai`.
- Đã tái hiện: database local xác nhận recovery email của Admin đã được lưu và verified; `GET /staff` hiện chỉ trả `has_recovery_email`; `StaffPage` luôn khởi tạo ô thay thế bằng chuỗi rỗng. Root cause là contract boolean-only trước đó, không phải lỗi persistence hoặc cache.
- Đã tải central/backend/frontend/root-cause mappings cùng Admin Governance, backend/frontend workflow, impact-analysis và diagnose guardrails; owner là `ListStaff`, `staff_account_security.go`, shared `maskEmail`, `StaffPage.tsx` và focused tests.
- `ListStaff` hiện lấy email verified vào trường repository-only, xóa trường đó trước response, luôn trả `has_recovery_email` và chỉ thêm `recovery_email_masked` khi actor có `staff.account_security.manage`; response không chứa email đầy đủ/private.
- Shared mask đã chuẩn hóa `abcdef@gmail.com` -> `abc***@g**.com`, gồm fallback an toàn cho local/domain ngắn; Profile và audit tiếp tục dùng cùng helper.
- Dialog đổi label thành `Email khôi phục mới`, giữ input rỗng, hiển thị `Hiện tại: <mask> · Đã xác minh` trong helper và chờ refetch Staff trước khi đóng sau non-self save. Self password reset vẫn bỏ qua refetch vì session đã bị revoke rồi logout.
- Regression backend xác nhận persistence, permission denial, boolean cho actor chỉ có `staff.view`, mask cho actor có permission, và không rò trường full/private; frontend source contract khóa label/helper/blank input/awaited refetch.
- Đã cập nhật central keyword map, backend function map, frontend UI/utility map, root-cause triage map và Admin Governance domain map cho contract mới; không cần migration/schema.
- Working tree tại thời điểm triển khai: `gofmt` và `git diff --check` pass; focused backend/frontend tests pass; `go test ./...` pass; frontend 168/168 test, production build và 250 KiB bundle budget pass.
- `make restart` đã rebuild/start đủ năm service. Health/readiness pass, migration log xác nhận `applied=0 skipped=109`, recovery preview vẫn ready. Runtime API bằng fixture Admin tạm xác nhận account hiện có trả mask `tes***@g**.com`, `has_recovery_email=true`, không có full/private field; fixture và session đã được dọn sạch.

## [Closed - Implemented, UAT pending] Admin quản trị bảo mật tài khoản nhân sự

### Will be done

- Browser/UAT chỉ chạy khi người dùng yêu cầu; trạng thái tại thời điểm đóng là `NOT UX ACCEPTED`.

### Done

- Người dùng đã duyệt review package, permission/RBAC, session/token semantics và Mockup as Text bằng câu `OK triển khai`.
- Đã tải central/backend/frontend mapping, admin-governance, backend/frontend workflow và impact-analysis guardrails; owner là `/staff`, `StaffPage.tsx`, handler/service/repository, `users`, `staff`, `auth_sessions`, `auth_email_tokens`, `audit_logs`.
- Đã xác nhận worktree đang chứa các thay đổi account-recovery trước đó và bảo toàn toàn bộ thay đổi không liên quan.
- Đã thêm V109 và seed parity cho `staff.account_security.manage`; migration mặc định chỉ cấp permission này cho role `admin`.
- `GET /staff` bổ sung trạng thái recovery đã che theo quyền. `PUT /staff/:id` vẫn bắt buộc `staff.update`; request có password/recovery khác rỗng còn bắt buộc permission mới và tài khoản liên kết.
- Đã thêm transaction staff-account security: khóa staff/user, bcrypt password, xác minh trực tiếp recovery email không gửi mail, revoke session khi đổi password, invalidate recovery token khi đổi một trong hai field, giữ nguyên trạng thái user/staff và ghi audit không chứa password/email đầy đủ. Admin tự đổi password cho chính mình nhận `reauth_required=true`.
- Đã chuyển chỉnh sửa nhân sự sang shared `Dialog` hai phần, thêm native `email`/`password` cho `SimpleForm`, bỏ icon con mắt, giữ một icon bút và icon gán vai trò độc lập. Nhân sự chưa liên kết tài khoản nhận warning thay cho field bảo mật.
- Đã thêm regression backend cho permission denial, bcrypt, verified recovery, inactive-state preservation, session/token revoke, audit redaction, unlinked staff và self logout; thêm migration grant test và frontend source-contract test.
- Đã cập nhật central keyword map, backend function map, frontend UI/utility map và admin-governance domain map cho V109/API/UI mới.
- Working tree tại thời điểm triển khai: `gofmt` và `git diff --check` pass; backend `make test` pass; focused PostgreSQL integration và V109 migration test chạy thật/pass; frontend 168/168 test, production build và 250 KiB bundle budget pass; migration manifest xác nhận 109 file.
- `make restart` đã rebuild/start đủ năm service. Runtime log xác nhận `database_migrations applied=0 skipped=109`, health/readiness pass, Admin login có permission mới và `/staff` trả `has_recovery_email`. Runtime API fixture xác nhận password bcrypt, recovery verified, inactive state preserved, session revoked, token invalidated và audit secret-safe; fixture đã được dọn sạch.

## [Closed - Implemented, UAT and production mail setup pending] Email khôi phục và quên mật khẩu

### Known limitations at closure

- Browser/UAT chưa chạy theo repo rule; trạng thái tại thời điểm đóng là `NOT UX ACCEPTED`.
- Cần hoàn tất Resend relay (`smtp.resend.com`, verified from-address và API key làm SMTP password) để chuyển từ preview/discarded delivery sang gửi email thật.
- Cần cấu hình credential Resend thật và xác nhận delivery trên môi trường đích trước khi công bố account recovery operational ở production.
- Bộ backend toàn repo trên database tạm sạch còn 2 lỗi baseline ngoài phạm vi recovery: `TestRev8EveryWorkItemHasOneExecutionTarget` kỳ vọng 3 BOQ nhưng migration hiện tạo 1, và `TestRunBackfillsDuplicateOutgoingAcceptanceCodes` của V26 gặp pending trigger event. Database local dùng chung còn thêm lỗi do fixture/mã kế toán trùng; cần review package riêng nếu yêu cầu sửa các baseline này.

### Done

- Người dùng đã duyệt follow-up UI preview bằng câu `OK triển khai`: local `development/test` không bị khóa Profile hoặc luồng Quên mật khẩu chỉ vì thiếu Resend; preview/no-op delivery được phép báo thành công để test UI, còn staging/production vẫn fail closed nếu thiếu sender thật.
- Đã thêm `mailer.PreviewSender` và route wiring theo môi trường: `development|test` tự động dùng preview khi mail thiếu/sai, `staging|production` vẫn không tạo sender. Preview nhận rồi bỏ message, không ghi token/email/API key vào log; service tiếp tục dùng cùng validation, persistence, masking và token lifecycle như sender thật.
- Đã cập nhật focused regression cho development/test preview, staging/production fail-closed và `/auth/me` availability; đồng thời cập nhật central/backend/frontend/root-cause mappings cùng admin-governance invariant.
- Working tree sau thay đổi: `gofmt` và `git diff --check` pass; `go test ./...` pass; frontend 165/165 test, production build và 250 KiB bundle budget pass.
- `make restart` đã rebuild/start đủ năm service. Runtime log xác nhận `status=ready provider=preview delivery=discarded`, migrations `applied=0 skipped=108`, health/readiness pass. API login trả `recovery_email_available=true`, `recovery_email_verified=false`; forgot-password trả `200`, `accepted=true`.
- Người dùng đã duyệt review package, impact matrix, chính sách token/verification/session, SMTP mặc định và Mockup as Text bằng câu `OK triển khai theo review package này`.
- Người dùng đã duyệt follow-up graceful degradation bằng câu `OK triển khai`: `AUTH_RECOVERY_ENABLED=true` với mail chưa sẵn sàng không làm dừng backend ở bất kỳ môi trường nào; hành vi recovery fail-closed ban đầu sau đó được follow-up preview mode thay thế cho development/test.
- Tại review ban đầu đã xác nhận code chưa có recovery email, forgot/reset route, reset-token persistence hoặc mail transport; các phần đó đã được triển khai trong plan này.
- Đã tải central/backend/frontend mapping, admin-governance, backend/frontend workflow và impact-analysis guardrails; mapping maintenance đã áp dụng.
- Đã thêm V107 cho users-only verified recovery email và hash-only one-time token; V108 thêm retention indexes. Cả hai đã được custom migration runner áp dụng trên PostgreSQL local và `migrate-check` xác nhận manifest 108 migration.
- Đã thêm generic SMTP sender/config, Compose/Kubernetes secret wiring và exact Nginx limits cho public recovery routes. Mail validation được tách khỏi startup-critical validation: cấu hình sai/thiếu dùng preview trên development/test nhưng để recovery unavailable trên staging/production; sender thật vẫn yêu cầu đầy đủ host/port/from/TLS/timeouts và paired credentials.
- Đã thêm protected set/remove, public verify/forgot/reset, current-password race pinning, active/non-external rechecks, account-enumeration-safe bounded async delivery, 60 giây cooldown, bcrypt reset, session/token revoke, masked audit và opportunistic 30-day token cleanup.
- Đã mở rộng Profile/Login và thêm Forgot/Reset/Verify pages dùng shared auth card, URL fragment token được xóa khỏi history, masked recovery states, inline validation và toast/state feedback.
- Targeted PostgreSQL integration `TestRecoveryEmailAndPasswordResetLifecycle` và profile regression pass; frontend 165 source/unit tests pass; frontend build + 250 KiB bundle budget pass; dev/prod Compose config và Kubernetes deterministic render validation pass.
- Full backend unit suite không gắn DB pass. Full DB suite đã được thử lại trên database tạm sạch với đủ V1-V108: toàn bộ scope recovery pass, còn đúng 2 baseline nêu trên; database tạm đã được xóa sau kiểm tra.
- Đã cập nhật central keyword map, backend function map, frontend UI map và admin-governance domain map. Không thêm recovery email vào seed `.local`, report hoặc export.
- Đã thêm `AUTH_RECOVERY_ENABLED=true` vào `.env` local theo xác nhận của người dùng; trạng thái graceful-degradation ban đầu sau đó được thay bằng development/test preview mode đã xác minh ở trên.
