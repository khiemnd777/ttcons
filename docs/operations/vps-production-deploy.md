# VPS Production Deployment — `tt-dnh.com`

Runbook này là nguồn vận hành chính cho single-host VPS `160.30.113.13`. Release chỉ đi từ tag `vYYYY-MM-DD.N`; push/merge vào `main` không deploy và không tự chạy GitHub Actions.

## Trạng thái prerequisite

- Domain: chờ đơn vị quản lý DNS cấu hình theo yêu cầu bên dưới.
- HTTPS: Caddy tự cấp/gia hạn chứng chỉ; certificate state nằm trong named volumes `consflow_caddy_data` và `consflow_caddy_config`.
- S3: Region đã xác nhận là `vn-hcm-1`; chờ Vietnix tạo private bucket `tt-dnh-consflow-prod` và cấp access key riêng.
- VPS: cần Docker Engine, Docker Compose plugin, SSH key và host key đã pin.
- GitHub: cần GHCR packages, Environment `production`, required reviewer và các environment secrets.

## Quy trình operator chính

Không tạo secret bundle có tên ngẫu nhiên và không Base64 toàn bộ production env.
Nguồn nhập liệu duy nhất trên máy operator là file bị Git-ignore, mode `0600`:

```text
deploy/config/production.env
```

Schema an toàn để commit nằm tại `deploy/config/production.env.example`. Bootstrap
lần đầu hoặc import gói tạm cũ bằng:

```bash
make production-bootstrap
# Chỉ dùng khi chuyển từ gói tạm đã sinh trước đây:
bash deploy/scripts/bootstrap-production.sh --import-legacy-bundle /path/to/legacy-bundle
```

Command sinh password/khóa còn thiếu bằng `openssl rand`, tạo một SSH deploy key
riêng, thu ED25519 host key dưới dạng candidate và không ghi đè giá trị đã có.
Operator phải nhập S3 key, GHCR read token và fingerprint đã đối chiếu qua console
vào `deploy/config/production.env`, sau đó chạy:

```bash
make production-check
```

`production-check` fail-closed nếu còn placeholder, quyền file khác `0600`, cặp
SSH key không khớp, known-hosts không chứa đúng VPS hoặc fingerprint không khớp
giá trị xác minh qua console.

Sau khi cài public key vào `authorized_keys` bằng một trusted password/console
session, provision VPS qua key-only SSH:

```bash
make production-provision
```

Command này render `.env.prod` và các file secret trong thư mục tạm mode `0700`,
truyền thẳng sang VPS, cài Docker/Compose trên Linux apt-based khi thiếu, đăng nhập
GHCR bằng read token, cài cấu hình vào `/opt/consflow/shared`, rồi xóa payload tạm.
Nó không nhận, lưu hoặc đưa mật khẩu VPS lên GitHub.

Chỉ năm deploy transport secrets được đưa lên GitHub, ở scope operator chọn rõ:

```bash
make production-github-secrets SCOPE=repository
# Hoặc sau khi GitHub plan hỗ trợ Environment theo yêu cầu:
make production-github-secrets SCOPE=environment
```

Các command trên upload raw multiline SSH key/known-hosts; không dùng Base64.
PostgreSQL, Redis, JWT, settings encryption, S3, GHCR và Admin bootstrap secrets
không được upload lên GitHub.

## Yêu cầu DNS gửi nhà cung cấp domain

Có thể gửi nguyên văn nội dung sau:

```text
Vui lòng cấu hình DNS cho domain tt-dnh.com:

1. Tạo/cập nhật bản ghi:
   Type: A
   Host/Name: @
   Value/Points to: 160.30.113.13
   TTL: 300 giây (hoặc giá trị thấp nhất nhà cung cấp hỗ trợ trong ngày cutover)

2. Xóa các bản ghi A khác của host @ nếu chúng trỏ tới IP cũ.

3. Không tạo bản ghi AAAA cho tt-dnh.com vì VPS hiện chưa khai báo IPv6.
   Nếu đang có AAAA, vui lòng xóa để tránh một phần client kết nối sai qua IPv6.

4. Không bật URL forwarding, parking page hoặc proxy/CDN tại lớp DNS.
   Domain phải trả trực tiếp IP 160.30.113.13.

5. Hiện chỉ cấu hình hostname tt-dnh.com, chưa cấu hình www.tt-dnh.com.

6. Nếu zone đang có CAA hạn chế CA, vui lòng thêm quyền cấp chứng chỉ:
   Type: CAA; Host/Name: @; Flag: 0; Tag: issue; Value: letsencrypt.org
   hoặc gửi lại danh sách CAA hiện tại để kiểm tra. Nếu zone không có CAA thì không
   cần tự tạo thêm bản ghi CAA.

Sau khi hoàn tất, vui lòng gửi ảnh chụp/bản ghi DNS cuối cùng và thời điểm thay đổi.
```

Không tạo `CNAME @`. Bản ghi apex `@` phải là `A`. Sau khi bên domain báo hoàn tất:

```bash
dig +short A tt-dnh.com @1.1.1.1
dig +short AAAA tt-dnh.com @1.1.1.1
```

Kết quả A phải chỉ có `160.30.113.13`; AAAA phải rỗng. Trước khi Caddy xin chứng chỉ, firewall VPS/provider phải cho phép inbound TCP `80` và `443`. Port `80` không được đóng vì Caddy dùng nó cho HTTP challenge và redirect sang HTTPS.

## Vietnix S3

Yêu cầu Vietnix hỗ trợ:

```text
Tạo private S3 bucket production với tên tt-dnh-consflow-prod tại location Hồ Chí Minh.
Vui lòng cung cấp chính xác Bucket, Region identifier, S3 endpoint và một access key
riêng chỉ dùng cho bucket này. Key cần ListBucket/HeadBucket ở bucket và GetObject,
PutObject, DeleteObject ở object; không cấp quyền sang bucket khác.
Không bật public-read. Nếu có, vui lòng bật versioning. Không cấu hình lifecycle tự
xóa object cho tới khi chính sách lưu trữ dữ liệu nghiệp vụ được duyệt riêng.
```

Cấu hình đã cố định:

```dotenv
STORAGE_PROVIDER=s3
STORAGE_S3_BUCKET=tt-dnh-consflow-prod
STORAGE_S3_REGION=<giá-trị-Vietnix-xác-nhận>
STORAGE_S3_ENDPOINT=https://s3.vn-hcm-1.vietnix.cloud
STORAGE_S3_USE_PATH_STYLE=false
STORAGE_S3_ALLOW_INSECURE_ENDPOINT=false
AWS_PROFILE=default
```

Tên bucket không có dấu chấm để dùng virtual-host addressing an toàn với TLS. Chỉ đổi `STORAGE_S3_USE_PATH_STYLE=true` nếu Vietnix xác nhận bắt buộc hoặc kiểm tra credential thất bại ở chế độ mặc định.

Lưu access key dưới `/opt/consflow/shared/secrets/aws_credentials`, không đặt trong `.env.prod`:

```ini
[default]
aws_access_key_id=<vietnix-access-key>
aws_secret_access_key=<vietnix-secret-key>
```

Trước deploy, dùng AWS CLI với cùng Region để chạy `head-bucket`, upload một object probe, đọc lại và xóa. Backend còn chạy startup preflight tạo/đọc/xóa `_consflow/probes/...`; preflight lỗi sẽ giữ backend ở trạng thái chưa ready.

## Chuẩn bị VPS một lần — chi tiết/fallback

Khuyến nghị tạo user `consflow-deploy`, dùng SSH key và cho phép vận hành Docker; không đưa mật khẩu `root` vào GitHub Actions. Nếu giai đoạn đầu buộc dùng `root`, vẫn phải dùng SSH private key và pin host key, sau đó chuyển sang deploy user.

Cấu trúc cố định:

```text
/opt/consflow/
  releases/vYYYY-MM-DD.N/
  current -> releases/<tag thành công>
  shared/
    .env.prod
    secrets/
```

Tạo thư mục và secret sources:

```bash
deploy_user=consflow-deploy # đổi thành root nếu VPS giai đoạn đầu deploy bằng root
install -d -m 0750 -o "$deploy_user" -g "$deploy_user" /opt/consflow
install -d -m 0750 -o "$deploy_user" -g "$deploy_user" /opt/consflow/releases
install -d -m 0700 -o "$deploy_user" -g "$deploy_user" /opt/consflow/shared
install -d -m 0700 -o "$deploy_user" -g "$deploy_user" /opt/consflow/shared/secrets
touch /opt/consflow/shared/secrets/postgres_password
touch /opt/consflow/shared/secrets/database_url
touch /opt/consflow/shared/secrets/redis_password
touch /opt/consflow/shared/secrets/redis_url
touch /opt/consflow/shared/secrets/redis_cache_password
touch /opt/consflow/shared/secrets/redis_cache_url
touch /opt/consflow/shared/secrets/jwt_secret
touch /opt/consflow/shared/secrets/settings_encryption_key
touch /opt/consflow/shared/secrets/aws_credentials
touch /opt/consflow/shared/secrets/bootstrap_admin_password
chown "$deploy_user:$deploy_user" /opt/consflow/shared/secrets/*
chmod 0644 /opt/consflow/shared/secrets/*
```

Thư mục cha `0700` là boundary bảo mật trên host; source file cần container UID đọc được qua Compose file secret. Người có quyền root hoặc Docker socket vẫn thuộc trust boundary.

Luồng operator chính tự render `.env.prod` từ `.env.prod.example`, đổi toàn bộ
đường dẫn secret thành `/opt/consflow/shared/secrets/...` và provision file. Các
lệnh dưới đây là fallback thủ công. File phải thuộc deploy user vì workflow đọc
file này và ghi release manifest/current symlink:

```bash
deploy_user=consflow-deploy # giữ cùng giá trị đã chọn ở bước trên
chown "$deploy_user:$deploy_user" /opt/consflow/shared/.env.prod
chmod 0600 /opt/consflow/shared/.env.prod
```

```dotenv
APP_DOMAIN=tt-dnh.com
FRONTEND_ORIGIN=https://tt-dnh.com
TRUSTED_PROXY_CIDR=127.0.0.1/32
BOOTSTRAP_ADMIN_EMAIL=admin@tt-dnh.com
BOOTSTRAP_ADMIN_PASSWORD_FILE=/opt/consflow/shared/secrets/bootstrap_admin_password
AWS_CREDENTIALS_FILE=/opt/consflow/shared/secrets/aws_credentials
```

Mật khẩu bootstrap phải có ít nhất 8 ký tự và không quá 72 byte. Command chỉ đọc password khi chưa có Admin. Khi cùng email đã là active global Admin và có active staff profile, command trả no-op và không reset password.

Command one-shot mà deploy script chạy ngay sau migration và trước khi start backend là:

```bash
docker compose --project-name consflow-prod \
  --env-file /opt/consflow/shared/.env.prod \
  -f /opt/consflow/releases/<tag>/docker-compose.prod.yml \
  --profile bootstrap run --rm --no-deps bootstrap-admin
```

Không chạy command này trước migration V112. Khi chạy thủ công, shell còn phải có `RELEASE_TAG`, `BACKEND_IMAGE` và `ADMIN_IMAGE` đúng release; quy trình GitHub/deploy script đã inject ba giá trị này. Command không phải API và không được dùng như công cụ reset mật khẩu.

Đăng nhập GHCR trên VPS một lần bằng deploy token chỉ có `read:packages`:

```bash
docker login ghcr.io
```

Không dùng personal token có quyền ghi repository trên production host.

## GitHub Environment và SSH

Script `deploy/scripts/configure-github-secrets.sh` là owner cho thao tác upload;
không nhập các giá trị thủ công trừ khi đang xử lý sự cố và đã đối chiếu cùng
contract. Repository private trên GitHub Free không dùng được Environment Secrets.
GitHub Pro cho phép Environment Secrets trên private repository, nhưng required
reviewer cho private environment cần gói hỗ trợ tương ứng; nếu chưa có, dùng scope
`repository` và xem đó là deploy không có GitHub Environment approval gate.

Tạo Environment `production`, bật required reviewer và khai báo:

- `VPS_HOST=160.30.113.13`
- `VPS_PORT=22` hoặc SSH port thực tế
- `VPS_USER=consflow-deploy` hoặc user đã được duyệt
- `VPS_SSH_PRIVATE_KEY`: private key dành riêng cho deploy
- `VPS_SSH_KNOWN_HOSTS`: dòng host key lấy qua kênh tin cậy và đối chiếu fingerprint trên VPS

Nếu SSH không dùng port 22, `VPS_SSH_KNOWN_HOSTS` phải dùng hostname dạng `[160.30.113.13]:<port>`. Workflow bật bắt buộc `StrictHostKeyChecking=yes`, `BatchMode=yes` và không fallback sang mật khẩu.

Protect pattern tag `v*` khỏi delete/force update. Workflow chỉ nhận tag khớp chính xác `vYYYY-MM-DD.N`, kiểm tra ngày hợp lệ và yêu cầu commit của tag nằm trong lịch sử `origin/main`.

## Tạo release

Operator command chính tự kiểm tra branch `main`, clean worktree, đồng nhất tuyệt
đối với `origin/main`, format/ngày tag và tag trùng trước khi tạo annotated tag:

```bash
make release TAG=v2026-08-20.1
```

Hoặc bỏ `TAG` để lấy số kế tiếp của ngày hiện tại:

```bash
make release
```

Các lệnh Git bên dưới là fallback thủ công tương đương:

```bash
git switch main
git pull --ff-only origin main
git tag -a v2026-08-20.1 -m "Release v2026-08-20.1"
git push origin v2026-08-20.1
```

Luồng thực tế:

```text
pull_request vào main -> validation
push main              -> không chạy Actions
push tag hợp lệ        -> validation -> build/push GHCR theo tag
                        -> chờ reviewer duyệt Environment production
                        -> deploy đúng image digest lên VPS
```

Deploy script thực hiện: validate Compose → pull digest → start dependency → kiểm tra secret mounts → drain backend/admin → migrate V112+ → bootstrap Admin → start/wait backend → start/wait Caddy. GitHub runner cuối cùng gọi HTTPS `/gateway-health` và `/api/ready`. Backend không publish host port; nó dùng `app_internal` cho Caddy/PostgreSQL/Redis và network `backend_egress` riêng chỉ để đi outbound tới Vietnix S3/SMTP.

Sau lần bootstrap đầu tiên:

1. Đăng nhập và đổi mật khẩu nếu quy trình nội bộ yêu cầu.
2. Xóa nội dung file `bootstrap_admin_password` bằng công cụ secret-management phù hợp.
3. Đặt `BOOTSTRAP_ADMIN_PASSWORD_FILE=/dev/null` trong `.env.prod`.
4. Không dùng bootstrap command để tạo Admin thứ hai; quản trị tiếp theo phải đi qua RBAC trong ứng dụng.

## HTTPS và smoke checks

```bash
curl -I http://tt-dnh.com
curl -I https://tt-dnh.com
curl -fsS https://tt-dnh.com/gateway-health -o /dev/null
curl -fsS https://tt-dnh.com/api/ready
openssl s_client -connect tt-dnh.com:443 -servername tt-dnh.com </dev/null
```

HTTP phải redirect sang HTTPS. Certificate phải đúng `tt-dnh.com`. Không xóa named volumes Caddy; mất `/data` khiến Caddy xin lại certificate và có thể chạm ACME rate limit.

## Rollback và giới hạn

Mỗi release thành công lưu `/opt/consflow/releases/<tag>/release.env` gồm tag và hai image digest. Rollback application chỉ dùng một release cũ có migration manifest tương thích với mọi version đã ghi trong `schema_migrations`; không tự rollback database.

Single-host deploy có downtime từ lúc drain đến khi backend/admin mới healthy. Direct Caddy Auto HTTPS không thay thế volumetric DDoS protection, WAF hoặc bot management. Khi bổ sung managed edge sau này, thực hiện review riêng cho firewall allowlist, trusted proxy CIDR/header và TLS topology theo [Production and DDoS Operations Runbook](production-ddos-runbook.md).
