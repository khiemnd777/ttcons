# Runbook kiểm thử tải cho DDoS hardening

## Mục tiêu và giới hạn

Runbook này xác minh hệ thống suy giảm có kiểm soát khi tăng tải HTTP hoặc số kết nối SSE: ưu tiên trả `429 RATE_LIMITED` hoặc `503 SERVER_BUSY/SERVICE_UNAVAILABLE` có `Retry-After`, giữ các lỗi còn lại ở mức thấp, không làm cạn pool PostgreSQL và phục hồi sau khi hạ tải.

Các script **không phải công cụ tấn công DDoS** và không được tự động chạy trong CI/CD. Chỉ chạy trên localhost hoặc staging đã được chủ hệ thống phê duyệt. Không chạy trên production, không nhắm vào hostname của khách hàng và không dùng credential thật của người dùng.

Artifacts:

- `tests/load/ddos.js`: health, authenticated general API, Dashboard, report và upload PDF nhỏ.
- `tests/load/sse.js`: áp lực kết nối SSE qua module `k6/x/sse`.
- `tests/load/sse-contract.js`: companion HTTP probe xác minh body của SSE `429` rejection.
- `tests/load/lib/config.js`: profile, threshold và safety guard dùng chung.
- `tests/load/lib/backpressure.js`: phân loại success, controlled `429/503` và lỗi hợp đồng.
- `tests/load/lib/sse-lifetime.js`: tính duration và fail-fast lifetime guard cho SSE soak.

## Safety guard bắt buộc

Script dừng ngay ở init context, trước khi tạo VU, nếu vi phạm một trong các điều kiện sau:

- thiếu `TARGET_BASE_URL`;
- URL chứa credential, path, query, fragment hoặc protocol ngoài HTTP(S);
- hostname có marker `prod`, `production`, `live` hoặc `www`;
- non-loopback target không dùng HTTPS;
- non-loopback target không có `TARGET_ENV=staging` và `CONFIRM_NON_PRODUCTION=YES_I_CONFIRM_NON_PRODUCTION`;
- hostname public không thể hiện rõ staging/dev/qa/test/sandbox/perf/load;
- workload protected thiếu `AUTH_TOKEN`;
- tải vượt 50 VU mà thiếu `CONFIRM_HIGH_LOAD=YES_I_UNDERSTAND_HIGH_LOAD`;
- upload vượt hard cap 10 VU hoặc tệp tổng hợp vượt 16 KiB;
- SSE healthy vượt hard cap 2 VU;
- SSE capacity thiếu `CONFIRM_SSE_CAPACITY=YES_I_EXPECT_CONTROLLED_REJECTIONS` hoặc vượt `SSE_CAPACITY_VU_CAP`;
- SSE soak thiếu khai báo lifetime staging hoặc lifetime sớm nhất không dài hơn duration cộng safety margin.

Không có biến override để cho phép production-like hostname.

## Công cụ

HTTP script dùng k6 chuẩn. SSE cần `k6/x/sse`; extension `v0.1.12` được kiểm thử với k6 `v1.2.2` theo [tài liệu package xk6-sse](https://pkg.go.dev/github.com/phymbert/xk6-sse). Dùng binary đã được đội vận hành phê duyệt và pin phiên bản. Ví dụ build trong môi trường công cụ tách biệt:

```sh
xk6 build --output ./tmp/k6-sse --with github.com/phymbert/xk6-sse@v0.1.12
```

Không cài hoặc build dependency trong lúc chạy test trên load generator production. Kiểm tra binary trước cửa sổ thử tải:

```sh
k6 version
./tmp/k6-sse version
```

## Chuẩn bị staging

1. Chốt cửa sổ thử tải, người thực hiện, người quan sát backend/DB/edge và tiêu chí dừng.
2. Xác nhận DNS/IP thuộc staging, origin không trỏ sang production và dataset có thể reset.
3. Tạo tài khoản thử tải riêng, quyền tối thiểu cho endpoint được kiểm tra, token ngắn hạn. Không ghi token vào file hoặc commit. Khi thử SSE, không dùng cùng token cho browser/worker khác vì backend giới hạn mặc định 2 stream trên mỗi actor.
4. Với upload, tạo một work item fixture có thể ghi minh chứng và một `UPLOAD_RUN_ID` duy nhất để truy vết/xóa dữ liệu tổng hợp sau test.
5. Mở dashboard quan sát edge, backend, PostgreSQL, Redis và container trước khi chạy baseline.
6. Ghi baseline ít nhất 10 phút khi không có tải k6 để phân biệt nhiễu nền.

Thứ tự chạy bắt buộc: health baseline → general baseline → Dashboard/report baseline → upload baseline → SSE baseline → step → spike → soak. Không chuyển bước nếu bước trước vượt tiêu chí dừng.

## Cấu hình môi trường

| Biến | Bắt buộc | Mặc định | Ý nghĩa |
|---|---:|---|---|
| `TARGET_BASE_URL` | Có | Không có | Chỉ origin, ví dụ `http://127.0.0.1:8080` hoặc `https://api.staging.example.test` |
| `TARGET_ENV` | Staging | `local` cho loopback | `local` hoặc `staging` |
| `CONFIRM_NON_PRODUCTION` | Staging | Không có | Phải bằng `YES_I_CONFIRM_NON_PRODUCTION` |
| `PROFILE` | Không | `baseline` | `baseline`, `step`, `spike`, `soak` |
| `WORKLOAD` | HTTP | `health` | `health`, `general`, `dashboard`, `report`, `upload` |
| `AUTH_TOKEN` | Protected workloads | Không có | Bearer token của tài khoản staging chuyên dụng |
| `HEALTH_PATH` | Không | `/api/health` | Endpoint liveness |
| `GENERAL_PATH` | Không | `/api/auth/me` | API authenticated chung |
| `DASHBOARD_PATH` | Không | `/api/reports/dashboard` | Dashboard aggregate |
| `REPORT_PATH` | Không | `/api/reports/projects?page_size=25` | Report read endpoint; có thể đổi sang report khác |
| `REQUEST_TIMEOUT` | Không | `15s` | Timeout cho HTTP request |
| `THINK_TIME_SECONDS` | Không | `0.25` | Nghỉ giữa các iteration |
| `VUS`, `DURATION` | Không | Theo profile/workload | Override constant-VU profile |
| `START_VUS` | Không | Theo profile | VU ban đầu của step/spike |
| `STAGE_N_VUS`, `STAGE_N_DURATION` | Không | Theo profile | Override từng stage 1–4 |
| `P95_MS` | Không | Theo profile/workload | Ngưỡng latency p95 |
| `MAX_ERROR_RATE` | Không | 1–3% | Tỷ lệ network error, status bất ngờ, `5xx` không kiểm soát hoặc `429/503` sai hợp đồng |
| `MAX_429_RATE` | Không | 1–80% | Tỷ lệ rate limited tối đa theo profile |
| `MAX_BUSY_503_RATE` | Không | 1–80% | Tỷ lệ `503` có hợp đồng backpressure hợp lệ tối đa theo profile |
| `UPLOAD_WORK_ITEM_ID` | Upload | Không có | UUID fixture, không dùng work item nghiệp vụ thật |
| `UPLOAD_RUN_ID` | Upload | Không có | Nhãn truy vết 3–64 ký tự, không phải secret |
| `UPLOAD_BYTES` | Không | `512` | PDF tổng hợp 128 B–16 KiB |
| `UPLOAD_VISIBILITY` | Không | `internal` | `internal` hoặc `project` |
| `SSE_MODE` | Không | `healthy` | `healthy` xác minh stream hợp lệ; `capacity` chủ động xác minh từ chối vượt cap |
| `CONFIRM_SSE_CAPACITY` | Capacity | Không có | Phải bằng `YES_I_EXPECT_CONTROLLED_REJECTIONS` |
| `SSE_CAPACITY_VU_CAP` | Không | `50` | Hard cap VU riêng cho capacity, từ 3–500 |
| `SSE_KIND` | Không | `attention` | `attention` hoặc `project` |
| `PROJECT_ID` | Project SSE | Không có | UUID công trình fixture |
| `SSE_MAX_ERROR_RATE` | Không | `0.02` | Tỷ lệ lỗi mở/duy trì SSE tối đa |
| `SSE_MIN_429_RATE` | Không | Healthy `0`, capacity `0.05` | Capacity phải quan sát ít nhất một tỷ lệ từ chối có kiểm soát |
| `SSE_MAX_429_RATE` | Không | Healthy `0.01`, capacity `1` | Trần tỷ lệ từ chối có kiểm soát; có thể hạ cho một capacity test cụ thể |
| `SSE_REJECTION_BACKOFF_SECONDS` | Không | `30` | Nghỉ 1–300 giây sau mỗi lần mở stream bị từ chối/lỗi; luôn tôn trọng `Retry-After` lớn hơn |
| `SSE_SERVER_MAX_LIFETIME_SECONDS` | Soak | Không có | Giá trị thực đã cấu hình cho `SSE_MAX_CONNECTION_LIFETIME` trên staging, tính bằng giây |
| `SSE_LIFETIME_MIN_RATIO` | Không | `0.8` | Hệ số lifetime sớm nhất khi server áp dụng jitter 80–100% |
| `SSE_TURNOVER_MARGIN_SECONDS` | Không | `120` | Khoảng an toàn phải còn lại sau toàn bộ scenario |
| `CONFIRM_SSE_CONTRACT_PROBE` | Contract probe | Không có | Phải bằng `YES_ACTOR_CAP_IS_ALREADY_OCCUPIED` |

Profile mặc định:

| Profile | HTTP read | Upload | SSE healthy | SSE capacity |
|---|---|---|---|---|
| Baseline | 2 VU / 2 phút | 1 VU / 1 phút | 2 VU / 2 phút | 5 VU / 2 phút |
| Step | 5 → 15 → 30 VU | 1 → 3 → 5 VU | 1 → 2 VU | 2 → 5 → 10 VU |
| Spike | 5 → 50 VU | 1 → 5 VU | 1 → 2 VU | 2 → 20 VU |
| Soak | 10 VU / 30 phút | 2 VU / 15 phút | 2 VU / 30 phút | 5 VU / 30 phút |

Các mức này là điểm khởi đầu staging, không phải công suất cam kết. Tăng tải chỉ sau khi xác nhận SLO, tài nguyên và giới hạn nhà cung cấp.

## Lệnh preflight không tạo tải

Các lệnh `inspect` chỉ parse và dựng cấu hình; chúng không chạy scenario:

```sh
TARGET_BASE_URL=http://127.0.0.1:8080 PROFILE=baseline WORKLOAD=health k6 inspect tests/load/ddos.js
```

```sh
TARGET_BASE_URL=http://127.0.0.1:8080 PROFILE=baseline SSE_MODE=healthy AUTH_TOKEN="$AUTH_TOKEN" ./tmp/k6-sse inspect tests/load/sse.js
```

```sh
TARGET_BASE_URL=http://127.0.0.1:8080 AUTH_TOKEN="$AUTH_TOKEN" CONFIRM_SSE_CONTRACT_PROBE=YES_ACTOR_CAP_IS_ALREADY_OCCUPIED k6 inspect tests/load/sse-contract.js
```

Kiểm tra negative guard trước cửa sổ test: bỏ `TARGET_BASE_URL`, dùng URL có `/api`, hoặc dùng hostname chứa `prod`; mọi trường hợp phải dừng ở init và không tạo request.

## Chạy HTTP

Baseline localhost:

```sh
TARGET_BASE_URL=http://127.0.0.1:8080 PROFILE=baseline WORKLOAD=health k6 run tests/load/ddos.js
```

General API trên staging:

```sh
TARGET_BASE_URL=https://api.staging.example.test TARGET_ENV=staging CONFIRM_NON_PRODUCTION=YES_I_CONFIRM_NON_PRODUCTION AUTH_TOKEN="$AUTH_TOKEN" PROFILE=step WORKLOAD=general k6 run tests/load/ddos.js
```

Dashboard hoặc report:

```sh
TARGET_BASE_URL=https://api.staging.example.test TARGET_ENV=staging CONFIRM_NON_PRODUCTION=YES_I_CONFIRM_NON_PRODUCTION AUTH_TOKEN="$AUTH_TOKEN" PROFILE=baseline WORKLOAD=dashboard k6 run tests/load/ddos.js
```

```sh
TARGET_BASE_URL=https://api.staging.example.test TARGET_ENV=staging CONFIRM_NON_PRODUCTION=YES_I_CONFIRM_NON_PRODUCTION AUTH_TOKEN="$AUTH_TOKEN" PROFILE=baseline WORKLOAD=report REPORT_PATH='/api/reports/accounting?page_size=25' k6 run tests/load/ddos.js
```

Upload nhỏ, có tạo dữ liệu staging:

```sh
TARGET_BASE_URL=https://api.staging.example.test TARGET_ENV=staging CONFIRM_NON_PRODUCTION=YES_I_CONFIRM_NON_PRODUCTION AUTH_TOKEN="$AUTH_TOKEN" PROFILE=baseline WORKLOAD=upload UPLOAD_WORK_ITEM_ID=00000000-0000-4000-8000-000000000001 UPLOAD_RUN_ID=ddos-baseline-001 k6 run tests/load/ddos.js
```

Sau upload, lọc evidence có tên bắt đầu bằng `K6 load test <UPLOAD_RUN_ID>`, đối chiếu số row/file rồi xóa bằng quy trình cleanup staging đã phê duyệt. Không dùng lệnh xóa hàng loạt trực tiếp từ runbook.

## Chạy SSE healthy và capacity

`k6/x/sse` giữ event loop của VU trong lúc stream mở; mỗi VU tương ứng một kết nối đồng thời. `gracefulStop` và `gracefulRampDown` được đặt về 0 để k6 đóng kết nối khi duration/stage kết thúc.

Hai mode có mục đích khác nhau:

- `healthy`: tối đa 2 VU, khớp cap backend mặc định `SSE_MAX_CONNECTIONS_PER_ACTOR=2`; yêu cầu tỷ lệ `429` không quá 1%.
- `capacity`: phải xác nhận rõ, dùng trên 2 VU với một token để chứng minh backend actor cap 2 và edge per-client cap 4 từ chối có kiểm soát. Mode này yêu cầu tỷ lệ `429` tối thiểu mặc định 5%; threshold `connections_opened>0` riêng biệt chứng minh vẫn có stream hợp lệ.

Sau một lần bị từ chối hoặc lỗi mở stream, VU nghỉ mặc định 30 giây và lâu hơn nếu `Retry-After` yêu cầu. Điều này ngăn các VU bị từ chối tạo vòng retry chặt.

Do stream chủ động sống đến hết stage, k6 có thể báo `interrupted iterations` khi hạ VU hoặc kết thúc test; đây là hành vi dự kiến. Đánh giá SSE bằng connection/event/error/429 metrics và active connection phía server, không dùng số iteration hoàn tất như throughput HTTP.

Healthy attention stream:

```sh
TARGET_BASE_URL=https://api.staging.example.test TARGET_ENV=staging CONFIRM_NON_PRODUCTION=YES_I_CONFIRM_NON_PRODUCTION AUTH_TOKEN="$AUTH_TOKEN" PROFILE=baseline SSE_MODE=healthy SSE_KIND=attention ./tmp/k6-sse run tests/load/sse.js
```

Healthy project stream:

```sh
TARGET_BASE_URL=https://api.staging.example.test TARGET_ENV=staging CONFIRM_NON_PRODUCTION=YES_I_CONFIRM_NON_PRODUCTION AUTH_TOKEN="$AUTH_TOKEN" PROFILE=baseline SSE_MODE=healthy SSE_KIND=project PROJECT_ID=00000000-0000-4000-8000-000000000002 ./tmp/k6-sse run tests/load/sse.js
```

Capacity/rejection mode:

```sh
TARGET_BASE_URL=https://api.staging.example.test TARGET_ENV=staging CONFIRM_NON_PRODUCTION=YES_I_CONFIRM_NON_PRODUCTION AUTH_TOKEN="$AUTH_TOKEN" PROFILE=step SSE_MODE=capacity CONFIRM_SSE_CAPACITY=YES_I_EXPECT_CONTROLLED_REJECTIONS SSE_KIND=attention ./tmp/k6-sse run tests/load/sse.js
```

SSE phải nhận ít nhất event `connected`; heartbeat hiện tại giúp chứng minh kết nối còn sống. Trong `xk6-sse v0.1.12`, HTTP response chỉ expose `url`, `status`, `headers` và `error`, không có response body. Vì vậy `sse.js` chỉ xác minh status `429` và `Retry-After`; file này **không tuyên bố tự xác minh JSON envelope**.

### Companion probe cho SSE rejection envelope

Chạy probe một lần trong khi đúng 2 healthy stream đang mở bằng cùng `AUTH_TOKEN`. Terminal thứ nhất giữ actor cap backend:

```sh
TARGET_BASE_URL=https://api.staging.example.test TARGET_ENV=staging CONFIRM_NON_PRODUCTION=YES_I_CONFIRM_NON_PRODUCTION AUTH_TOKEN="$AUTH_TOKEN" PROFILE=baseline SSE_MODE=healthy SSE_KIND=attention ./tmp/k6-sse run tests/load/sse.js
```

Ngay sau khi hai stream nhận `connected`, terminal thứ hai gửi đúng một HTTP request tới cùng SSE endpoint:

```sh
TARGET_BASE_URL=https://api.staging.example.test TARGET_ENV=staging CONFIRM_NON_PRODUCTION=YES_I_CONFIRM_NON_PRODUCTION AUTH_TOKEN="$AUTH_TOKEN" CONFIRM_SSE_CONTRACT_PROBE=YES_ACTOR_CAP_IS_ALREADY_OCCUPIED SSE_KIND=attention k6 run tests/load/sse-contract.js
```

Probe chỉ pass khi request thứ ba nhận `429`, có `Retry-After`, `success=false` và `error.code=RATE_LIMITED`. Nếu actor cap chưa thực sự được chiếm, request có thể mở stream rồi timeout và probe phải fail. Với project stream, thêm cùng `SSE_KIND=project PROJECT_ID=<fixture-uuid>` ở cả hai terminal. Không phát sinh mutation chỉ để tạo event trong cùng lượt thử tải.

### Ngăn planned turnover làm sai kết quả soak

Backend kết thúc chủ động SSE sau `SSE_MAX_CONNECTION_LIFETIME` mặc định 5 phút; effective lifetime có thể jitter trong khoảng 80–100%. Vì `soak` mặc định kéo dài 30 phút, giữ giá trị 5 phút sẽ tạo EOF/close dự kiến mà `xk6-sse` không cung cấp close reason đủ tin cậy để phân biệt với network failure.

Trước SSE soak, chỉ trên staging:

1. Đặt `SSE_MAX_CONNECTION_LIFETIME=45m` hoặc lớn hơn.
2. Restart backend staging và xác minh effective config.
3. Chạy k6 với `SSE_SERVER_MAX_LIFETIME_SECONDS=2700` đúng bằng giá trị staging.
4. Sau test, khôi phục giá trị trước đó, thông thường `5m`, rồi restart staging và xác minh active stream trở lại chính sách chuẩn.

Ví dụ healthy soak:

```sh
TARGET_BASE_URL=https://api.staging.example.test TARGET_ENV=staging CONFIRM_NON_PRODUCTION=YES_I_CONFIRM_NON_PRODUCTION AUTH_TOKEN="$AUTH_TOKEN" PROFILE=soak SSE_MODE=healthy SSE_KIND=attention SSE_SERVER_MAX_LIFETIME_SECONDS=2700 ./tmp/k6-sse run tests/load/sse.js
```

Init guard tính `configured lifetime × SSE_LIFETIME_MIN_RATIO` và yêu cầu kết quả lớn hơn tổng duration cộng `SSE_TURNOVER_MARGIN_SECONDS`. Với 45 phút, jitter 80% cho lifetime sớm nhất 36 phút, đủ cho soak 30 phút cộng margin 2 phút. Không khai báo số chỉ để vượt guard: giá trị phải khớp staging thực tế. Nếu stream vẫn EOF sớm trong cửa sổ đã bảo vệ, coi đó là lỗi kết nối/config drift; planned-turnover behavior 5 phút được kiểm tra riêng sau khi restore, không trộn vào load-soak threshold.

Guard này bắt buộc cho profile `soak` và cũng được áp dụng cho profile khác khi `SSE_SERVER_MAX_LIFETIME_SECONDS` được khai báo. Nếu override một profile ngắn thành duration gần hoặc dài hơn lifetime staging, phải thực hiện cùng quy trình raise/declare/restore thay vì coi EOF dự kiến là lỗi tải.

## Threshold và cách đọc kết quả

HTTP script ghi các metric:

- `consflow_request_latency`: p95 của request hoàn chỉnh;
- `consflow_unexpected_error`: network error, status ngoài success/controlled-backpressure, hoặc response `429/503` sai hợp đồng;
- `consflow_rate_limited`: tỷ lệ `429`;
- `consflow_controlled_busy_503`: tỷ lệ `503` có `Retry-After` và envelope code hợp lệ;
- `http_req_failed`: status `429/503` được đánh dấu expected ở lớp k6; custom metrics/checks vẫn fail nếu header/envelope sai;
- `checks`: chỉ pass khi status thành công hoặc controlled backpressure hợp lệ.

Để kiểm tra đúng response đã bị giới hạn, HTTP script giữ response body dạng text cho từng iteration dù global `discardResponseBodies` đang bật. Body chỉ được parse khi status là `429/503` và không được lưu sau iteration. Hợp đồng hợp lệ là:

- `429`: có `Retry-After`, `success=false`, `error.code=RATE_LIMITED`;
- backend semaphore `503` trên Dashboard/report/upload: có `Retry-After`, `success=false`, `error.code=SERVER_BUSY`;
- edge `503`: có `Retry-After`, `success=false`, `error.code=SERVICE_UNAVAILABLE`;
- mọi `5xx` khác, hoặc `429/503` thiếu header/envelope đúng, được tính là unexpected error.

Đọc `Retry-After` theo response thực tế, không tự suy ra từ profile. Backend `RATE_LIMITED`/`SERVER_BUSY` hiện trả số giây và lặp lại giá trị trong `error.details.retry_after_seconds`; `SERVER_BUSY` còn có `request_class`. Edge `SERVICE_UNAVAILABLE` có thể chỉ có header. HTTP checks yêu cầu header tồn tại trên đúng response và envelope code tương ứng. Với SSE, giá trị số lớn hơn backoff cấu hình sẽ được tôn trọng, tối đa 300 giây; giá trị không phải số vẫn dùng backoff an toàn mặc định và phải được điều tra trong kết quả test.

Ngưỡng mặc định:

| Profile | Unexpected error | 429 tối đa | Controlled 503 tối đa | Ý nghĩa |
|---|---:|---:|---:|---|
| Baseline | ≤ 1% | ≤ 1% | ≤ 1% | Không nên backpressure traffic hợp lệ nhỏ |
| Step | ≤ 2% | ≤ 25% | ≤ 25% | 429/controlled 503 được phép xuất hiện trước lỗi không kiểm soát |
| Spike | ≤ 3% | ≤ 80% | ≤ 80% | Ưu tiên load shedding; không chấp nhận crash/OOM |
| Soak | ≤ 1% | ≤ 10% | ≤ 10% | Không có suy giảm tích lũy |

SSE pressure script ghi connection opened, event received, open error và `429` có `Retry-After` riêng. Healthy yêu cầu tối đa 1% `429`; capacity yêu cầu tối thiểu 5% và cho phép đặt trần riêng qua `SSE_MAX_429_RATE`. Cả hai mode đều phải mở được ít nhất một stream và nhận event. Tỷ lệ attempt bị từ chối có thể tiến gần 100% trong capacity soak vì stream hợp lệ sống lâu còn VU vượt cap tiếp tục retry có backoff; dùng `connections_opened`, active-stream server metric và rejection rate cùng nhau, không diễn giải rate đơn lẻ. `sse-contract.js` phải pass riêng trước khi gọi các rejection này là controlled backpressure đầy đủ.

Không đánh giá pass chỉ từ k6. Đồng thời quan sát:

- edge/LB: request rate, active connections, connection rejects, WAF/rate-limit decisions, origin bandwidth;
- backend: RPS, p95/p99, `429`, controlled/uncontrolled `503`, các `5xx` khác, active requests, goroutine, active SSE, limiter hit/fallback, CPU/RAM/OOM/restart;
- PostgreSQL: open/in-use/idle, pool wait count và wait duration, query duration, statement timeout, lock wait;
- Redis: latency, errors, reconnects, limiter fallback và memory;
- storage: upload bytes, inode/disk growth và cleanup count.

Kỳ vọng DB: số connection không vượt pool cap; wait count có thể tăng ngắn hạn nhưng wait duration không tăng liên tục sau khi hạ tải; không có lock/statement timeout hàng loạt; auth-session query không lấn át workload. Nếu metrics DB chưa được expose, test chỉ mang tính thăm dò và không được dùng để công bố năng lực chống DDoS.

## Tiêu chí dừng và phục hồi

Dừng test ngay nếu có một trong các dấu hiệu:

- target được phát hiện không phải staging;
- backend/DB/Redis restart hoặc OOM;
- lỗi `5xx` ngoài controlled `503` trên 5% trong 30 giây;
- DB pool ở trạng thái full và wait duration tăng liên tục 60 giây;
- CPU trên 90% hoặc memory trên 85% trong 2 phút mà không có xu hướng phục hồi;
- p99 vượt timeout client, lỗi storage tăng hoặc dung lượng upload vượt ngân sách;
- on-call yêu cầu dừng.

Sau khi dừng, tải phải trở về gần baseline trong 2 phút, active SSE phải giảm theo số VU, DB pool wait phải ngừng tăng và không còn restart mới. Lưu summary k6, biểu đồ hệ thống, thay đổi cấu hình, timestamp bắt đầu/kết thúc và `UPLOAD_RUN_ID`. Chỉ tăng profile sau khi owner backend/DB xác nhận trạng thái ổn định.

## Điều kiện nghiệm thu

- Baseline đạt threshold p95/unexpected-error/429/controlled-503; mọi response backpressure có `Retry-After` và envelope đúng, đồng thời không làm thay đổi tài nguyên nền đáng kể.
- Step/spike tạo `429` hoặc controlled `503` trước khi xuất hiện lỗi không kiểm soát/cạn DB; response phải có `Retry-After` và error envelope đúng.
- Upload bị giới hạn concurrency/quota, tệp nhỏ được truy vết và cleanup đầy đủ.
- SSE healthy mở tối đa 2 stream/actor với tỷ lệ `429` thấp; capacity quan sát được controlled `429`, backoff đúng và vẫn giữ ít nhất một stream hợp lệ nhận `connected`/heartbeat.
- SSE companion probe xác nhận rejection có `Retry-After` và `RATE_LIMITED` envelope; pressure script chỉ chịu trách nhiệm status/header/connection metrics.
- Soak chạy với server lifetime guard đã đối chiếu staging, không có planned turnover trong cửa sổ đo và không có xu hướng tăng liên tục của memory, goroutine, connection, DB wait, Redis error hoặc disk.
- Hệ thống phục hồi trong cửa sổ đã thống nhất và không cần restart thủ công.

Nếu bất kỳ điều kiện nào không đạt, giữ rollout ở `observe` hoặc rollback mức enforce liên quan, đính kèm evidence theo timestamp và mở remediation trước khi thử tải lại.
