# BM02 — Nghiệm thu xuất theo lần gửi, Development 12-10-2026

**Kết quả:** TECHNICALLY VERIFIED cho báo cáo thử thủ công có hai lần gửi. Lần 1 vẫn giữ đúng nội dung, số liệu và ảnh sau khi nguồn báo cáo đã sửa và gửi Lần 2. Đang chờ anh review ảnh/file để chốt UX; chưa đánh dấu UX ACCEPTED.

**Giới hạn:** Chỉ nghiệm thu luồng báo cáo thủ công với người lập Admin và người nhận thử có vai trò PM trong đúng công trình. Chưa chạy lại nguồn phiếu workflow, mobile, ảnh camera vật lý, Microsoft Excel desktop hoặc in giấy. Không suy rộng kết quả này thành nghiệm thu toàn bộ tám biểu mẫu. Không sửa ứng dụng, thêm field, migration, ghi SQL trực tiếp, reset dữ liệu hoặc triển khai production.

**Bố trí ảnh đã được duyệt, 12-10-2026:** sau khi đối chiếu BM02 gốc không có ô/vùng ảnh riêng, hàng20 chỉ ghi chú và tham chiếu BM-GS-07, anh xác nhận `Vậy thì hướng làm của bạn OK rồi.` Giữ nguyên sheet báo cáo BM02 và ảnh nhúng ở sheet `Hình ảnh` trong cùng workbook. Đề xuất chuyển ảnh vào cuối sheet báo cáo được rút lại; không sửa code. Xác nhận này chốt riêng vị trí ảnh, không suy rộng thành nghiệm thu các phạm vi chưa kiểm chứng.

## Môi trường và phạm vi

- Development UI `http://127.0.0.1:5173`, API `http://127.0.0.1:8081/api`; UI và hai route thật `/api/health`, `/api/ready` trả 200. Hai probe đầu ở `/health`, `/ready` không phải route đã đăng ký, trả 404 và được ghi riêng trong evidence.
- Working tree trên `main`, HEAD `75dc44707785d399ed2408959bf784928a544e96`, giữ các thay đổi chưa commit từ trước. Manifest 962 file ứng dụng/config/test bằng manifest của bản đã build ở lượt sửa thẻ Nhật ký; không có source drift trong lượt nghiệm thu này.
- Admin và backend containers tạo lúc `2026-10-12T02:41:09Z`, chạy cùng image đã nghiệm thu thẻ Nhật ký. Không rebuild trong lượt này vì source không đổi. Image IDs, thời điểm chạy và health hiện tại có trong [environment.json](acceptance-evidence/bm02-delivery-revision-development-2026-10-12/environment.json).
- PostgreSQL `consflow`: giữ 157 migration, 5 công trình, 20 phiếu workflow. Snapshot runtime và danh sách Đầu việc của công trình thử không đổi; luồng gửi báo cáo không hoàn thành Đầu việc hoặc chuyển Giai đoạn.
- Công trình thử hiện có `DEV-BM04-REUSE-20261009`, `5020b7b5-d383-4504-967d-5e705c4ee2dd`; Đầu việc hợp lệ `2c969a28-945f-4abe-bf9a-8d44c722699d`, construction stage, report_eligible=true, in_progress.
- Nguồn báo cáo thử mới `257da527-beb9-4b02-8a90-dba13ac3b282`; hồ sơ lần gửi `7a22a2d6-86a1-41b3-b08f-af5aaed9ceec`.
- Người lập `Consflow Admin`; người nhận riêng `[Synthetic] BM02 người nhận — 2026-10-12`, user `bc554766-1b43-425e-ab4a-295dd86060bd`. Tạo qua API nhân sự chuẩn, thêm vai trò PM chỉ tại công trình thử; không cấp role toàn hệ thống hoặc đổi quyền/password của tài khoản đang có. Tài khoản thử đã đăng nhập, được danh sách người nhận chấp nhận, đọc đúng báo cáo và yêu cầu bổ sung bằng UI. SELECT chỉ đọc xác nhận đúng một role PM với project_id của công trình thử, một membership active và user/staff active; [chứng cứ quyền theo công trình](acceptance-evidence/bm02-delivery-revision-development-2026-10-12/recipient-scoped-assignment.json).
- Đã tạo đúng một báo cáo, một người nhận thử, một membership và hai PNG thử qua API chuẩn. Các record thử được giữ lại để review. Mật khẩu/token nằm trong file tạm riêng 0600, không nằm trong evidence/repository.

## Luồng đã chạy

1. Tạo báo cáo ngày 12-10-2026 và upload PNG đỏ Lần 1 vào đúng Đầu việc bằng API chuẩn. Không dùng lại fixture có Giai đoạn sai hoặc PNG hỏng ở lượt nghiệm thu trước.
2. Người lập mở báo cáo tại Thi công và bấm `Gửi cho [Synthetic] BM02 người nhận — 2026-10-12`. Revision 1 được lưu với nội dung LẦN 1, 25%, 3 người / 8 giờ và một ảnh đỏ.
3. Tại Báo cáo → chi tiết lần gửi, bấm `Xuất báo cáo`, tải nguyên file Lần 1 trước khi thay nguồn.
4. Đăng nhập đúng người nhận thử trên trình duyệt, mở báo cáo chờ xem, nhập yêu cầu đổi nội dung/ảnh và bấm `Yêu cầu bổ sung`. Event Lần 1 ghi đúng người nhận; không dùng override của Admin để phản hồi.
5. Người lập sửa chính báo cáo này bằng PUT chuẩn với đầy đủ payload đang có. Đổi sang nội dung LẦN 2, 60%, 5 người / 7 giờ; bỏ liên kết ảnh đỏ khỏi báo cáo hiện tại và dùng PNG xanh mới. Ảnh gốc Lần 1 và snapshot không bị sửa.
6. Người lập trở lại Thi công, kiểm tra nội dung/ảnh mới, bấm `Gửi lại`. Cùng report ID có Revision 2.
7. Báo cáo → chi tiết lần gửi: tải Lần 2; mở `Các lần gửi` → `Lần 1` → `Xem nội dung` rồi bấm `Xuất báo cáo` lần nữa. UI Lần 1 giữ nội dung/25%/3 người/ảnh đỏ và chỉ có `Về bản hiện tại`; không mở thao tác duyệt bản cũ.
8. Mở ảnh cũ bằng trình xem minh chứng chuẩn: ảnh đỏ hiển thị qua endpoint kiểm tra quyền và đúng lần gửi. Lịch sử hiển thị Admin gửi L1 → người nhận thử yêu cầu bổ sung L1 → Admin gửi lại L2.
9. Import trực tiếp hai file gốc trong Downloads thành Google Sheets riêng tư. Không sửa hoặc tái tạo workbook trước/sau import; kiểm tra cả hai sheet được tạo và chụp Google-rendered views ở 100%.

## Đối chiếu file và lịch sử

| Tiêu chí | Kết quả thực tế |
| --- | --- |
| Snapshot Lần 1 trước/sau | Toàn bộ revision record, snapshot JSON, SHA-256 và evidence membership bằng nhau |
| Nội dung / tiến độ / nhân lực cũ | Nội dung LẦN 1, 25%, 3 người / 8 giờ giữ nguyên; Lần 2 có dữ liệu mới |
| Bố cục / styles / drawing / cell XML | Bằng nhau cho hai file Lần 1; không chỉ so vài ô mẫu |
| Ảnh nhúng Lần 1 | Cùng bytes và pixel SHA-256; chỉ ảnh đỏ. Ảnh xanh chỉ thuộc Lần 2 |
| File ảnh gốc bảo vệ | Người nhận đọc được đúng endpoint từng revision; bytes bằng PNG gốc đã upload |
| File tải / evidence copy | Mỗi bản evidence bằng nguyên bytes của file tải tương ứng trong Downloads |
| Google Sheets | 100 ô không rỗng/công thức mỗi workbook khớp XLSX; 0 effectiveValue.errorValue trên vùng chứa toàn bộ dữ liệu, không có #VALUE! |
| Công thức tổng nhân lực | Google tính đúng 3 ở Lần 1 và 5 ở Lần 2 |
| Render file | Checklist, nội dung chi tiết và ảnh đúng phiên bản; ảnh đầy đủ, không chồng nội dung; không sửa kích thước sau import |
| Runtime / Đầu việc | Giữ nguyên; gửi/nhận báo cáo không thay trạng thái thi công |

SHA-256 snapshot Lần 1 `b9006547ae54b475a46423ced5db9cfa7a857bd909babf99de08778c3d649349`; Lần 2 `5ebcaf9d6f9ea7c6903938b70babdbab1ffa1dbb4dd359f546d795894637de59`.

Hai file XLSX Lần 1 không có cùng hash toàn file: thứ tự khai báo MIME trong `[Content_Types].xml` thay đổi khi thư viện xuất file. Các khai báo có cùng giá trị; mọi part khác, bao gồm sheet/style/drawing/media, bằng nguyên bytes. Đây là khác biệt đóng gói; không phải thay nội dung báo cáo. Verifier ban đầu so quá chặt thứ tự XML và ô chuỗi rỗng so với ô trống Google; đã đối chiếu khác biệt rồi sửa riêng comparator đọc evidence. Không sửa ứng dụng hoặc file kết quả để làm kiểm tra pass.

## File cho review

- [Lần 1 tải trước khi gửi lại](acceptance-evidence/bm02-delivery-revision-development-2026-10-12/BM02-lan-1-truoc.xlsx).
- [Lần 1 tải lại sau Lần 2](acceptance-evidence/bm02-delivery-revision-development-2026-10-12/BM02-lan-1-sau.xlsx).
- [Lần 2](acceptance-evidence/bm02-delivery-revision-development-2026-10-12/BM02-lan-2.xlsx).
- [Google Sheets — Lần 1 giữ nguyên](https://docs.google.com/spreadsheets/d/1clVd-l6HwuEySFni8BTjSTLROpGSgb5XEEvwUkSNSE0/edit).
- [Google Sheets — Lần 2](https://docs.google.com/spreadsheets/d/1s_MFTLD3p8bIMrS-gWO_prnoRvPyEAHsqUPHB9ZBrOQ/edit).
- [Ảnh UI bản cũ](acceptance-evidence/bm02-delivery-revision-development-2026-10-12/08-delivery1-after.jpg), [trình xem ảnh cũ](acceptance-evidence/bm02-delivery-revision-development-2026-10-12/18-revision1-protected-viewer.jpg), [lịch sử đủ ba sự kiện đúng actor](acceptance-evidence/bm02-delivery-revision-development-2026-10-12/20-delivery-history-resubmit.jpg).
- [Nội dung chi tiết Lần 1](acceptance-evidence/bm02-delivery-revision-development-2026-10-12/16-google1-details.jpg), [ảnh nhúng Lần 1](acceptance-evidence/bm02-delivery-revision-development-2026-10-12/12-google1-photo.jpg), [nội dung chi tiết Lần 2](acceptance-evidence/bm02-delivery-revision-development-2026-10-12/17-google2-details.jpg), [ảnh nhúng Lần 2](acceptance-evidence/bm02-delivery-revision-development-2026-10-12/15-google2-photo.jpg).

## Kiểm tra và bước tiếp theo

- Bundled Python chạy [verify_acceptance.py](acceptance-evidence/bm02-delivery-revision-development-2026-10-12/verify_acceptance.py): PASS42 kiểm tra cuối cùng, exit0. Kết quả máy đọc: [acceptance-checks.json](acceptance-evidence/bm02-delivery-revision-development-2026-10-12/acceptance-checks.json). Script chỉ đọc dữ liệu ứng dụng/file xuất/Google readback, nhưng ghi JSON evidence; cần helper tạm và phiên API hợp lệ, không phải verifier độc lập chỉ dựa trên repository. Phiên test hết hạn ở lần chạy lại đã được đăng nhập lại bình thường; không bypass auth.
- Không có work_report_revision nào trước fixture này: kiểm tra giữ nguyên các lần gửi tồn tại trước fixture là NOT_APPLICABLE. Kiểm tra Lần 1 của chính fixture có baseline riêng và đã đối chiếu toàn bộ record trước/sau. Parity về số công trình/phiếu/migration không chứng minh mọi dòng trong database không đổi; chỉ runtime/danh sách Đầu việc cụ thể và revision1 có chứng cứ bằng nhau.
- `git diff --check`: exit0. Không chạy lại build/unit tests vì không đổi source; kết quả build cũ chỉ áp dụng cho manifest đã kiểm tra không đổi.
- Mapping không cần cập nhật: lượt này chỉ tạo fixture và ghi chứng cứ nghiệm thu, không đổi feature, API, helper, UI hoặc invariant.
- [Rà soát độc lập GPT-6.1 Sol High](acceptance-evidence/bm02-delivery-revision-development-2026-10-12/independent-audit.md): tự tính lại file, snapshot, actor/event, Google values/formulas, ảnh, runtime và source hashes; không phát hiện lỗi chặn trong phạm vi fixture. Reviewer kiểm tra lại chứng cứ project-only role, ảnh lịch sử đủ ba bước và kết quả42checks; các khoảng thiếu chứng cứ đã khép. Trạng thái vẫn TECHNICALLY VERIFIED / NOT UX ACCEPTED cho đến khi anh review.
- **Bước tiếp theo:** anh review ảnh và file của hai lần gửi để chốt riêng BM02 tại chi tiết lần gửi. Các giới hạn ở đầu báo cáo vẫn tách biệt.
