# Hiệu năng bộ lọc Giám sát — 2026-10-02

**UAT-DM-01: IMPLEMENTED / TECHNICALLY VERIFIED trên local; browser retest PASS trong phạm vi bộ lọc và regression đã chạy.** Tính năng tổng thể vẫn NOT UX ACCEPTED theo các giới hạn nghiệm thu. User đã duyệt “fix hiệu năng” sau review package trong `deputy-manager-uat-2026-10-02.md`.

**Cập nhật23:36 ICT:** Browser retest sau sửa đã chạy theo yêu cầu “UAT check”: bộ lọc sẵn sàng140ms ở lần mở tab, reload234–333ms; các regression scope/read/history/mobile được kiểm tra đạt. Checkpoint hiện tại và quan sát HTTP429 realtime tự phục hồi được ghi ở `deputy-manager-uat-after-performance.md`. Các mục NOT_RUN bên dưới là trạng thái lịch sử tại thời điểm sửa, không phải trạng thái retest mới.

## Kết quả trên cùng DB/API local

| Phép đo | Trước sửa | Sau sửa |
| --- | --- | --- |
| `GET /api/me/supervision-options` | 14.168,7 ms | 42,85 / 15,72 / 16,17 / 12,36 / 10,33 ms |
| Trung vị 5 lần sau sửa | — | 15,72 ms |
| Payload data có thứ tự | 840 công trình, 4 Giám sát | Giống hoàn toàn trước sửa trong cả 5 lần |
| SQL Giám sát EXPLAIN ANALYZE | UAT trước đó: 14.346,551 ms, 1.789.592 subplan loops | 2,075 ms, 261 shared buffers hit/read |

API được đo bằng tài khoản UAT Phó Quản Lý qua login/session thật, trước và sau thay backend binary. Main DB không bị reset hoặc seed. Số Giám sát khác lượt UAT trước vì tài khoản Giám sát UAT đã inactive; công trình C vẫn còn. SQL cũ là checkpoint UAT trước, không phải phép đo API mới. Năm request tuần tự trên local không phải load test hoặc bằng chứng production.

## Nguyên nhân và thay đổi

Truy vấn cũ kiểm tra `supervisorUserSQL(u.id,p.id)` trên từng user qua từng công trình. Query plan xác nhận chi phí ở điều kiện phân loại lồng nhau. SQL chậm loại trừ giả thuyết render UI là nguyên nhân chính; endpoint thành công và V148 đã áp dụng không phù hợp với giả thuyết lỗi migration/permission.

`supervision.go::supervisionSupervisorOptionsSQL` lấy tập user bằng UNION từ Supervisor global (khi tồn tại công trình thật), Supervisor theo công trình thật và membership active mang alias/UUID role chuẩn trên công trình thật. Sau đó join user active/non-deleted/non-sample, giữ predicate Deputy hiện tại và sort tên/id. UNION loại trùng nhiều nguồn quyền. Semantics staff chưa deleted được giữ đúng canonical classifier hiện có.

`supervisorUserSQL` dùng phân loại báo cáo giữ nguyên. Không thay API/UI/permission/snapshot/migration; không thêm cache hoặc index.

## Kiểm chứng kỹ thuật khi sửa, trước browser retest

- Source: working tree chưa commit trên `main`, HEAD `465eb76`. SHA-256 `supervision.go`: `3d12367ab4afacbcb7969402d55f5d56b913541fd4bbd68d01f8fc0a2fe68ed9`. Giữ thay đổi feature có sẵn. Sau build chỉ sửa test/mapping/docs, không sửa mã ứng dụng đã build.
- Docker build thành công, thay riêng backend local lúc **23:11:33 ICT**. Image `sha256:bf32f629354515536af9cf584e4e908c82d49c5246cc8bd3ded9f1d84dce4711`. Admin, main PostgreSQL/Redis vẫn chạy; health 200, main DB V148. Không có migration mới/full seed/production mutation.
- `GOCACHE="$PWD/.gocache" CONSFLOW_TEST_DATABASE_URL=<isolated PostgreSQL> go test ./internal/core -run 'TestSupervision|TestDeputyCompanyScope|TestDeputyReportReceipts|TestNormalizeSupervisionReportFilter' -count=2 -v`: **PASS**, hai lượt liên tiếp, core 4,242s trên DB mới sau cập nhật test cuối. DB test riêng PostgreSQL 16.13, migrations V1–V148; không chạy test mutation trên main DB.
- Test mới có 18 trường hợp role/member/account/project; đối chiếu toàn bộ kết quả có thứ tự với canonical classifier, kiểm tra duplicate grants, không có công trình thật và actor thường/thu hồi quyền.
- Chạy thêm `TestSupervisionOptionsMatchCanonicalClassification -count=1` trên cùng DB test có sẵn một sample Supervisor chuẩn: **PASS**, core 0,715s. Test tái sử dụng identity mẫu mà không sửa/xóa nó, tránh xung đột unique role khi DB test đã có seed.
- Fixture ban đầu vi phạm sample-role constraint; đã sửa cách tạo fixture hợp lệ. Chạy lại phát hiện cleanup role cascade gây FK explorer actor version; cleanup mới xóa grants khi user còn tồn tại. Hai lượt cuối đạt, zero user fixture `options-%` còn lại. Không giảm assertion hoặc bỏ gate.
- `GOCACHE="$PWD/.gocache" go test ./... -count=1`: **PASS** trên source cuối. Không có DB env ở lệnh này; integration cần DB được skip. Nhóm DB liên quan đã chạy riêng như trên.
- `GOCACHE="$PWD/.gocache" go vet ./...`: **PASS**. `git diff --check`: **PASS**.
- Browser sau sửa: **NOT_RUN** vì lượt này không yêu cầu browser. Không chạy lại frontend build do frontend không đổi, API data được so sánh chính xác. Chưa nghiệm thu client/thiết bị thật/production.

## Bàn giao

- Cập nhật central/backend/triage maps và Admin Governance/Construction Operations domain maps. Không có frontend interaction/report contract thay đổi nên không cần đổi frontend/report mapping.
- Deputy UAT đưa lại về inactive, mọi session revoked, token cũ trả 401; token tạm gỡ. Không đổi Cường/Nguyệt hoặc quyền tài khoản thật. Container test riêng được dọn sau kiểm tra.
- Logs/JSON/query plan: `/tmp/consflow-deputy-perf/`. Chưa commit/push/deploy tại thời điểm checkpoint hiệu năng. Plan được đóng theo yêu cầu của người dùng ngày 2026-10-02; xem [bản lưu plan](plan-closed/2026-10-02__closed-deputy-manager-supervision.md). Các giới hạn nghiệm thu được giữ nguyên trong bản lưu.
