# Production And DDoS Operations Runbook

Runbook này vận hành Consflow sau managed CDN/DDoS protection và WAF. Các giới hạn Nginx trong repository bảo vệ origin ở lớp HTTP; chúng không hấp thụ được tấn công volumetric L3/L4. Không đưa origin trực tiếp ra Internet.

## Kiến trúc bắt buộc

```text
Internet
  -> Managed CDN / DDoS protection / WAF / TLS
  -> Firewall chỉ cho phép CIDR của proxy tới origin
  -> admin:8080 (Nginx static frontend + /api reverse proxy)
  -> backend:8080 (internal Docker network)
  -> PostgreSQL:5432 và Redis:6379 (internal Docker network)
```

Production Compose chỉ publish service `admin`. `backend`, `postgres` và `redis` chỉ dùng network `app_internal`; không tạo host port cho ba service này. TLS kết thúc tại managed edge hoặc load balancer do đơn vị vận hành quản lý.

Local `docker-compose.yml` cũng bind `admin`, backend, PostgreSQL và Redis vào `127.0.0.1`. Nếu cần kiểm thử từ thiết bị trong LAN, tạo Compose override hẹp cho đúng interface/IP và review firewall trước khi chạy; không đổi bind mặc định thành mọi interface.

## Chuẩn bị cấu hình

1. Sao chép `.env.prod.example` thành `.env.prod` ngoài source control.
2. Thay mọi giá trị `CHANGE_ME`. Tạo mật khẩu/JWT bằng nguồn ngẫu nhiên bảo mật; JWT secret tối thiểu 32 byte.
3. URL-encode mật khẩu khi đưa vào `DATABASE_URL` và `REDIS_URL`; password trong URL phải tương ứng với password của service.
4. Đặt `FRONTEND_ORIGIN` thành HTTPS origin người dùng thật.
5. Đặt `TRUSTED_PROXY_CIDR` đúng subnet của proxy/load balancer kết nối trực tiếp tới Nginx và `REAL_IP_HEADER` đúng header do proxy đó ghi đè. Không tin header client khi origin còn nhận kết nối trực tiếp.
6. Giữ `ADMIN_BIND_ADDRESS=127.0.0.1` nếu proxy chạy cùng host. Chỉ dùng private interface hoặc `0.0.0.0` sau khi firewall đã allowlist proxy CIDR và từ chối mọi nguồn khác.
7. Chọn `APP_INTERNAL_SUBNET` không trùng với host/VPC. Backend chỉ tin forwarded IP từ subnet Docker này; Nginx ghi đè `X-Forwarded-For` bằng client IP đã chuẩn hóa trước khi chuyển tiếp.

File mẫu không chứa production secret. Tạo file thật với permission chỉ owner đọc/ghi:

```bash
cp .env.prod.example .env.prod
chmod 600 .env.prod
```

Không commit `.env.prod`, không đưa nội dung file hoặc output Compose đã render vào ticket, artifact hay log. Ưu tiên secret manager của nền tảng để cấp file tạm với permission tương đương thay vì lưu secret dài hạn trên host.

## Kiểm tra và triển khai

```bash
docker compose --env-file .env.prod -f docker-compose.prod.yml config --quiet
docker compose --env-file .env.prod -f docker-compose.prod.yml build
docker compose --env-file .env.prod -f docker-compose.prod.yml up -d
docker compose --env-file .env.prod -f docker-compose.prod.yml ps
```

Không chạy hoặc lưu `docker compose ... config` dạng render trong CI/log dùng chung: output đó chứa plaintext environment của PostgreSQL, Redis, JWT và backend. `config --quiet` chỉ kiểm tra tính hợp lệ mà không in cấu hình đã nội suy.

Kiểm tra từ origin host:

```bash
curl -fsS http://127.0.0.1:8080/gateway-health
curl -fsS http://127.0.0.1:8080/api/health
curl -fsS http://127.0.0.1:8080/api/ready
```

`/api/health` là liveness. `/api/ready` là readiness có kiểm tra dependency và được dùng bởi production Compose. Không đưa một backend chưa ready vào luồng phục vụ.

PostgreSQL là hard startup/readiness dependency. Redis chỉ dùng `service_started`: nếu Redis chưa healthy hoặc lỗi sau restart, backend vẫn khởi động ở degraded mode với limiter local bounded fallback và SSE replica-local. Cảnh báo vận hành phải được phát ngay; khôi phục Redis sớm để lấy lại distributed quota và shared SSE fanout.

Sau deploy, xác minh từ một máy ngoài allowlist rằng origin port, `8080`, `5432` và `6379` đều không truy cập được. Người dùng chỉ truy cập hostname HTTPS của managed edge.

## Chính sách tại origin

| Nhóm route | Giới hạn khởi đầu theo client IP | Kết nối đồng thời | Body tối đa |
|---|---:|---:|---:|
| Health/readiness | 5 request/giây | 5 | 1 KiB |
| Login | 5 request/phút, burst 5 | 5 | 64 KiB |
| Refresh/logout | 30 request/phút, burst 10 | 5 | 16 KiB |
| API thông thường | 20 request/giây, burst 40 | 20 | 2 MiB |
| Upload evidence/invoice | 6 request/phút, burst 2 | 2 | 11 MiB |
| PDF/report export và protected file download | 12 request/phút, burst 3 | 4 | 2 MiB |
| Mở SSE | 6 request/phút, burst 2 | 4/client, 300 toàn gateway | 1 KiB |

Request vượt ngân sách trả `429` và `Retry-After`. Khi upstream không kết nối được hoặc timeout, gateway trả `503` và `Retry-After`. SSE tắt proxy buffering, có read timeout 75 giây và dựa vào heartbeat backend để giữ kết nối.

Protected evidence, input/output invoice và workflow-form PDF downloads dùng cùng heavy request/connection budget với PDF/report export; file vẫn đi qua route authorization hiện hữu, không được Nginx cache. `SSE_MAX_CONNECTION_LIFETIME` là cận trên của thời hạn kết nối (mặc định 5 phút); mỗi connection chọn ngẫu nhiên trong 80-100% cận này, tức khoảng 4-5 phút ở cấu hình mặc định, để reconnect/revalidate định kỳ mà không tạo một đợt reconnect đồng loạt. Client tiếp tục tôn trọng backoff và `Retry-After`.

Effective SSE cap là `min(Nginx global SSE cap, SSE_MAX_CONNECTIONS)`, mặc định `min(300, 300) = 300`. Mỗi proxied SSE giữ xấp xỉ hai Nginx connections: một client-to-gateway và một gateway-to-backend. Với `worker_connections=4096`, 300 SSE dùng khoảng 600 connection slots, giữ phần lớn headroom cho API/static traffic và overhead ngay cả khi chỉ có một worker. Main Nginx config đặt `worker_rlimit_nofile=8192`; Compose đặt soft/hard `nofile=8192`. Khi chỉnh cap, phải bảo đảm `2 × active SSE + ordinary client connections + ordinary upstream connections + overhead` thấp hơn tổng worker connection budget và không vượt file-descriptor limit của từng worker.

Nginx ghi toàn bộ `5xx` và lấy mẫu khoảng 1% response `2xx`/`3xx`/`4xx`; Docker xoay log theo `LOG_MAX_SIZE` và `LOG_MAX_FILES`. Managed edge phải giữ access/security log đầy đủ theo chính sách lưu trữ của tổ chức.

Các ngưỡng trên là baseline. Điều chỉnh bằng staging load test và số liệu thật; không nới quota để xử lý một đợt tấn công đang diễn ra.

Lần deploy đầu giữ `RATE_LIMIT_MODE=observe`, kiểm tra metric/log staging, sau đó chuyển thành `enforce`. Semaphore upload/export/report và giới hạn SSE của backend vẫn hoạt động theo các giá trị trong `.env.prod`.

Backend có hai lớp bucket riêng: `RATE_LIMIT_IP_*` chạy trước JWT/session DB; sau khi JWT claims hợp lệ, `RATE_LIMIT_ACTOR_*` chạy trước kiểm tra `AuthSessionActive` authoritative trong PostgreSQL. Mỗi policy có `LIMIT`, `BURST` và `WINDOW`; file `.env.prod.example` liệt kê đầy đủ baseline cho `AUTH`, `GENERAL`, `UPLOAD`, `EXPORT` và `SSE`. `RATE_LIMIT_REDIS_MAX_IN_FLIGHT` giới hạn số lượt limiter đồng thời được phép chờ Redis để tránh Redis outage giữ quá nhiều request/goroutine. Khi Redis bận, lỗi hoặc đã hết admission budget, backend dùng local bounded fallback theo `RATE_LIMIT_LOCAL_MAX_KEYS`; theo dõi header `X-RateLimit-Source: local-fallback` và xử lý Redis thay vì để fallback kéo dài.

Production Compose cố định `APP_ENV=production`. Backend từ chối khởi động khi secret/origin/trusted proxy không an toàn, Redis URL sai, limiter bị tắt, hoặc biến timeout/pool/quota/concurrency/SSE được khai báo sai định dạng hay không dương; không dựa vào giá trị mặc định im lặng sau lỗi gõ cấu hình.

## Chính sách bắt buộc tại managed edge

- Bật always-on volumetric DDoS protection, bot filtering và WAF managed rules.
- Chỉ cho phép phương thức HTTP cần thiết; giới hạn URL/header/body trước khi request tới origin.
- Rate limit theo client IP cho login, refresh, upload, export và SSE; bổ sung account/user key khi edge có nhận diện đáng tin cậy.
- Cache immutable frontend assets; không cache auth, API riêng tư, file được bảo vệ hoặc SSE.
- Ghi đè real-IP header, không nối tiếp giá trị do client tự gửi.
- Dùng TLS hiện hành, redirect HTTP sang HTTPS và bật health probe tới gateway.
- Firewall origin chỉ allowlist egress CIDR của provider/proxy. Compose không tự cấu hình cloud firewall, DNS, TLS certificate hoặc vendor WAF.

## Theo dõi và cảnh báo

Theo dõi tối thiểu:

- edge request rate, blocked request, challenge, bandwidth và nguồn địa lý;
- gateway `429`, `503`, active connections, upstream latency và container CPU/RAM/PID;
- backend request rate, p95/p99, active SSE và goroutine;
- PostgreSQL open/in-use/wait, query timeout, disk và replication/backup status;
- Redis latency, rejected connection, memory và eviction;
- upload volume capacity, disk latency và OOM/restart count.

Cảnh báo khi `5xx` hoặc DB pool wait tăng, container gần memory limit, readiness fail liên tục, Redis không healthy, upload volume gần đầy, hoặc `429` lệch mạnh khỏi baseline.

## Xử lý sự cố DDoS

1. Xác định L3/L4 hay HTTP flood từ edge dashboard và origin metrics.
2. Với L3/L4, chuyển ngay cho provider; không cố hấp thụ tại VPS/origin.
3. Với HTTP flood, bật/chặt rule edge theo route, ASN, quốc gia, bot score hoặc client identity đã xác minh.
4. Giữ firewall origin đóng với Internet; kiểm tra attacker không bypass CDN bằng origin IP cũ hoặc DNS history.
5. Không scale PostgreSQL connection pool theo request flood. Giảm tải ở edge/gateway trước.
6. Nếu dependency bão hòa, để readiness loại backend và trả `503`; bảo toàn DB, Redis và upload volume.
7. Lưu timeline, rule thay đổi, metric, log mẫu và quyết định rollback để hậu kiểm.

## Rollback

Trước deploy, ghi image/commit hiện tại và xác nhận backup theo `docs/ops/backup-restore.md`. Nếu release mới gây lỗi:

1. Lưu Compose status và log liên quan.
2. Deploy lại commit/image đã biết tốt bằng cùng `.env.prod`.
3. Không xóa PostgreSQL, Redis hoặc upload volumes.
4. Chỉ rollback database theo `docs/ops/deployment-rollback.md` khi migration thật sự yêu cầu.

## Giới hạn còn lại

- Limiter Nginx là theo client IP đã chuẩn hóa; quota theo user/account và distributed limiter phải do backend/edge cung cấp.
- `TRUSTED_PROXY_CIDR` nhận một subnet của proxy trực tiếp. Nếu provider có nhiều dải, đặt một load balancer đáng tin cậy trong subnet xác định trước origin hoặc mở rộng cấu hình sau khi review danh sách CIDR chính thức.
- Production Compose chạy một backend. Không scale nhiều replica cho tới khi SSE fanout dùng shared broker và migration startup đã được xác nhận an toàn cho multi-instance.
- Hiện backend nhận `DATABASE_URL`, `REDIS_URL` và `JWT_SECRET` qua environment; người có quyền đọc `.env.prod`, Docker socket hoặc container inspect có thể xem plaintext runtime metadata. Bước nâng cấp tiếp theo là thêm hỗ trợ `*_FILE` vào backend, dùng Compose/Docker secrets cho PostgreSQL, Redis và JWT, rồi bỏ các secret value khỏi container environment. Việc này cần thay đổi contract cấu hình backend và phải được review riêng.
- Resource limit và HTTP load shedding giảm blast radius nhưng không thay thế capacity planning, backup, observability hoặc diễn tập sự cố.
