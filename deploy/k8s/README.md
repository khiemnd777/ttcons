# Consflow Kubernetes Production Baseline

Baseline này đưa lớp ứng dụng Consflow ra khỏi topology single-host mà không thay đổi API, schema, RBAC hay workflow. Nó chỉ triển khai `admin`, `backend` và migration Job; PostgreSQL, Redis, object storage, TLS, load balancer, WAF, backup và secret manager phải là dịch vụ ngoài cluster hoặc managed service đã được đơn vị vận hành phê duyệt.

```text
Internet
  -> Managed CDN / DDoS / WAF / TLS
  -> Managed load balancer or ingress controller
  -> admin ClusterIP Service
       -> admin pod A (node/zone A)
       -> admin pod B (node/zone B)
  -> backend ClusterIP Service
       -> backend pod A (node/zone A)
       -> backend pod B (node/zone B)
  -> external managed PostgreSQL + Redis + S3-compatible object storage
```

Docker Compose production vẫn là topology single-host được kiểm soát. Không dùng đồng thời Compose và Kubernetes để chạy hai release ghi vào cùng database nếu chưa có kế hoạch migration/compatibility riêng.

## Impact matrix

| Area | Existing owner | Collision | Contract in this baseline |
|---|---|---|---|
| Gateway DNS | `frontend/nginx.conf` dùng Docker resolver `127.0.0.11` | Image nguyên trạng sẽ trả `502` trong Kubernetes | Init container kiểm tra template, loại Docker-only resolver/dynamic variable và render `proxy_pass http://backend:8080` tới stable ClusterIP Service; frontend source không đổi |
| Schema migration | `consflow-migrate`, PostgreSQL advisory lock | Migration theo từng pod/init container sẽ chạy lặp và có thể đụng binary cũ | Một release-unique Job với credential DDL/owner riêng chạy trước rollout; backend chỉ mount runtime DML credential, không có migration init container và luôn `AUTO_MIGRATE=false` |
| Database pool | `DB_MAX_OPEN_CONNS` là theo process | Scale replica nhân tổng connection | Capacity gate dùng `backend replicas × DB_MAX_OPEN_CONNS + migration/ops headroom` |
| Redis/SSE | Redis Pub/Sub có shared fanout; limiter/SSE caps có local process state | Redis outage hoặc scale-out làm local fallback/caps khác với global quota | Redis managed là production dependency; test cross-replica fanout/reconnect/failover và tính quota theo tổng pod trước mở traffic |
| Protected files | storage registry/provider | Local disk/`emptyDir` không chia sẻ giữa pod | S3-compatible provider và migration inventory/checksum là prerequisite trước khi chạy hai backend replicas |
| Metrics | backend `/metrics`, admin exact `/metrics` trả `404` | Public scrape làm lộ internal metrics | Prometheus scrape private `backend:8080/metrics`; admin/LB không publish endpoint này |

## Fail-closed base

Không áp trực tiếp `deploy/k8s/base` vào production. Base cố ý có bốn cổng chặn:

- image trỏ tới `registry.example.invalid`;
- `TRUSTED_PROXY_CIDR(S)` là giá trị `REPLACE_*` không hợp lệ;
- `STORAGE_PROVIDER=s3` nhưng bucket/region để rỗng;
- Secret `consflow-runtime-secrets` không được render từ Kustomize.

Production overlay phải thay image bằng immutable digest, thay origin/proxy trust bằng topology thật và đồng bộ Secret từ secret manager trước khi tạo pod. File `secrets/runtime-secret.example.yaml` chỉ mô tả key; không thêm file đã điền secret vào Kustomize hoặc git.

## Platform prerequisites

1. Cluster có ít nhất hai worker node schedulable. Required pod anti-affinity giữ hai replica cùng loại khỏi một node; zone spread sẽ dàn tải khi cluster có nhiều zone. Rolling update dùng `maxSurge: 0`, `maxUnavailable: 1` để thay một pod mỗi lượt mà không cần node thứ ba.
2. Managed PostgreSQL có TLS, backup/PITR, capacity đủ cho toàn bộ pool và maintenance window đã được review.
3. Managed Redis có auth/TLS, Pub/Sub, persistence/failover phù hợp và latency đã đo từ cluster.
4. S3-compatible object storage, bucket policy, encryption, lifecycle và object migration đã hoàn thành. Không dùng local provider với nhiều backend pod.
5. External secret manager/KMS đồng bộ Secret `consflow-runtime-secrets`; Kubernetes Secret encryption at rest và RBAC/namespace access được bật.
6. CNI thực thi Kubernetes `NetworkPolicy`. Managed load balancer/Ingress/Gateway kết thúc TLS, ghi đè real-IP header, bật WAF/DDoS và firewall chỉ cho phép đường edge tới origin.
7. Pod edge và metrics collector có private reachability/nhãn nguồn đúng contract NetworkPolicy bên dưới. Metrics Server chỉ bắt buộc nếu operator bật optional HPA.
8. Backup PostgreSQL mới và release/image digest được ghi trong change record.

## Runtime Secret

Bốn key ứng dụng tối thiểu hiện tại:

- `database_url`: absolute `postgres://`/`postgresql://` URL của runtime role ít quyền, chỉ có DML/quyền cần thiết cho API và không có quyền DDL/schema owner;
- `migration_database_url`: URL của role migration có quyền DDL/owner cần cho custom migration runner; key này chỉ được mount vào schema Job;
- `redis_url`: absolute `redis://`/`rediss://` URL tới managed Redis;
- `jwt_secret`: ít nhất 32 byte ngẫu nhiên.

Secret được mount read-only mode `0440` qua supplemental group. Backend chỉ đọc `database_url`, `redis_url`, `jwt_secret`; schema Job chỉ đọc `migration_database_url`. Storage migration Job dùng runtime `database_url` vì chỉ đọc/update metadata, không cần DDL. Cách này giữ kiểm tra bounded/non-empty/NUL/trailing-newline hiện có, giảm blast radius và không đặt secret trực tiếp trong ConfigMap hay Pod spec.

Storage runtime dùng contract đã xác nhận:

- ConfigMap: `STORAGE_PROVIDER=s3`, `STORAGE_OPERATION_TIMEOUT`, required `STORAGE_S3_BUCKET`, required `STORAGE_S3_REGION`, optional HTTPS `STORAGE_S3_ENDPOINT`, `STORAGE_S3_USE_PATH_STYLE`, `STORAGE_S3_ALLOW_INSECURE_ENDPOINT=false`;
- AWS SDK v2 default credential chain: production ưu tiên workload identity do platform inject qua overlay; portable S3-compatible fallback dùng optional Secret refs `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` và `AWS_SESSION_TOKEN`, tương ứng các key lower-case trong Secret example. Backend production chạy put/open/read/delete preflight trước khi start HTTP server; thiếu identity/static credential hoặc sai bucket policy làm container fail trước readiness;
- HTTP endpoint bị production validation từ chối trừ khi operator chủ động đặt `STORAGE_S3_ALLOW_INSECURE_ENDPOINT=true`; không bật ngoại lệ này cho public endpoint.

Ưu tiên External Secrets/Secrets Store CSI hoặc cơ chế tương đương của platform. Nếu bootstrap thủ công từ các file do secret manager cấp, bảo đảm file không xuất hiện trong shell history, ticket, log hay repository:

```bash
kubectl apply -f deploy/k8s/base/namespace.yaml
kubectl apply -f deploy/k8s/base/service-account.yaml
kubectl -n consflow create secret generic consflow-runtime-secrets \
  --from-file=database_url=/secure/runtime/database_url \
  --from-file=migration_database_url=/secure/migration/database_url \
  --from-file=redis_url=/secure/runtime/redis_url \
  --from-file=jwt_secret=/secure/runtime/jwt_secret \
  --dry-run=client -o yaml | kubectl apply -f -
```

Nếu không dùng workload identity, thêm `--from-file=aws_access_key_id=...`, `--from-file=aws_secret_access_key=...` và chỉ thêm session token khi provider cấp temporary credential. Không chạy lệnh này với `--from-literal` trong shared terminal/history. Với production thật, external secret controller nên là owner của Secret. Workload-identity overlay thêm service-account annotation/projected token cần thiết và phải chứng minh một S3 read/write/delete probe trên staging trước rollout; base giữ `automountServiceAccountToken=false` để không cấp Kubernetes API token rộng ngoài ý muốn.

## Network isolation contract

Base áp ingress isolation trực tiếp lên cả `admin` và `backend`:

- `admin:8080` chỉ nhận traffic từ pod có `consflow.network/edge-client=enabled` trong namespace có `consflow.network/edge=enabled`;
- `backend:8080` chỉ nhận application traffic từ pod `admin` cùng namespace, hoặc scrape từ pod có `consflow.network/metrics-client=enabled` trong namespace có `consflow.network/monitoring=enabled`;
- không có public backend Service và không có đường arbitrary pod -> backend.

Hai selector namespace/pod trong cùng peer là điều kiện **AND**. Platform overlay phải gắn nhãn namespace và pod template của đúng controller/collector, không gắn nhãn rộng cho mọi workload. Ví dụ shape cần được quản lý bằng GitOps/platform config:

```yaml
# ingress/gateway controller namespace + pod template
metadata.labels[consflow.network/edge]: enabled
spec.template.metadata.labels[consflow.network/edge-client]: enabled

# monitoring namespace + collector pod template
metadata.labels[consflow.network/monitoring]: enabled
spec.template.metadata.labels[consflow.network/metrics-client]: enabled
```

Nếu managed load balancer đi trực tiếp từ node/IP thay vì pod-based Ingress/Gateway, CNI có thể nhìn source khác selector trên. Phải chứng minh packet path và policy admission ở staging; không nới NetworkPolicy thành toàn namespace/VPC để chữa `502`.

## Production overlays and immutable images

Tạo overlay môi trường ngoài repository hoặc trong repo bằng một review riêng. Ví dụ shape ứng dụng:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - /absolute/path/to/consflow/deploy/k8s/base
images:
  - name: registry.example.invalid/consflow/backend
    newName: registry.company.example/consflow/backend
    digest: sha256:REPLACE_WITH_64_HEX
  - name: registry.example.invalid/consflow/admin
    newName: registry.company.example/consflow/admin
    digest: sha256:REPLACE_WITH_64_HEX
patches:
  - target:
      kind: ConfigMap
      name: consflow-runtime
    patch: |-
      - op: replace
        path: /data/FRONTEND_ORIGIN
        value: https://workflow.company.example
      - op: replace
        path: /data/SERVER_NAME
        value: workflow.company.example
      - op: replace
        path: /data/TRUSTED_PROXY_CIDR
        value: 10.20.0.0/24
      - op: replace
        path: /data/TRUSTED_PROXY_CIDRS
        value: 10.30.0.0/16
      - op: replace
        path: /data/STORAGE_S3_BUCKET
        value: consflow-production
      - op: replace
        path: /data/STORAGE_S3_REGION
        value: ap-southeast-1
      - op: replace
        path: /data/STORAGE_S3_ENDPOINT
        value: https://s3.ap-southeast-1.amazonaws.com
```

Không thêm `namePrefix`/`nameSuffix` vào application overlay: K8s-specific Nginx config cố ý target exact stable Service `backend`. Release-unique `nameSuffix` chỉ áp dụng cho one-shot operator Jobs.

`TRUSTED_PROXY_CIDR` là peer trực tiếp tới admin/Nginx. `TRUSTED_PROXY_CIDRS` là peer trực tiếp từ admin/service-mesh tới Fiber. Không dùng toàn VPC CIDR chỉ vì thuận tiện; xác minh actual source address sau kube-proxy/CNI/mesh và giữ allowlist nhỏ nhất.

AWS S3 có thể để endpoint rỗng và dùng regional resolver của SDK. S3-compatible/MinIO thường cần HTTPS endpoint riêng và `STORAGE_S3_USE_PATH_STYLE=true`; chỉ dùng các giá trị đã kiểm thử với conditional create/checksum behavior của provider.

Migration dùng overlay riêng và release-unique `nameSuffix`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - /absolute/path/to/consflow/deploy/k8s/migrate
nameSuffix: -r20260722-shortsha
images:
  - name: registry.example.invalid/consflow/backend
    newName: registry.company.example/consflow/backend
    digest: sha256:REPLACE_WITH_THE_SAME_BACKEND_DIGEST
```

## Deployment sequence

### Local-to-S3 prerequisite

Schema V70 mở allowlist `s3` cho evidence, signature visual, workflow PDF và input/output invoice metadata. V71 giữ tương thích với metadata legacy nhưng chặn mọi internal key mới bị null, rỗng hoặc không canonical. Schema Job của release phải apply cả V70 và V71 trước operator migration. Binary `/app/consflow-storage-migrate` dùng dry-run mặc định, chỉ copy/update metadata khi có `--apply`, và cần `STORAGE_MIGRATION_TIMEOUT` cùng storage runtime/S3 credential contract ở trên.

Kubernetes application base không mount filesystem cho object storage; backend chỉ khởi tạo S3 và không có legacy local objects. Vì vậy trước khi scale từ Compose/single-host sang baseline hai replica:

1. Backup database, giữ nguyên local upload directory và snapshot inventory/checksum.
2. Apply V70 và V71 bằng migration Job/release image tương thích.
3. Drain/stop application writes để inventory không phát sinh record local mới.
4. Chạy `/app/consflow-storage-migrate` không flag từ operator container/host đang mount đúng legacy `STORAGE_PATH`; chỉ tiếp tục nếu JSON summary có `failed=0` và `source_verified=inventory`.
5. Chạy lại với `--apply`; yêu cầu `failed=0`, `updated=inventory`, destination read/checksum verification đầy đủ.
6. Chạy dry-run lần cuối; inventory local phải bằng 0. Giữ source objects read-only qua rollback retention window, không xóa ngay.
7. Chỉ sau đó apply Kubernetes overlay có `STORAGE_PROVIDER=s3` và hai backend replicas, rồi kiểm thử upload ở pod A/download ở pod B cùng các protected endpoint.

Phương án dùng RWX volume để dual-read local objects trong Kubernetes cần review platform riêng. Application base không cung cấp PVC/RWX hoặc `emptyDir` cho object storage; PVC chỉ xuất hiện trong operator Job mẫu như nguồn legacy read-only.

Nếu platform đã copy nguyên vẹn legacy tree vào một existing read-only RWX PVC, có thể dùng `deploy/k8s/storage-migrate` làm operator Job. Tạo overlay ngoài repo thay image bằng cùng immutable backend digest, thêm run-unique `nameSuffix`, thay `STORAGE_S3_BUCKET/REGION/ENDPOINT` và thay claim `replace-with-legacy-upload-rwx-pvc`. Lượt đầu giữ nguyên không có `args` để dry-run:

```bash
kubectl create -k /secure/consflow/storage-migrate-dry-run-overlay
kubectl -n consflow wait --for=condition=complete --timeout=130m \
  job/consflow-storage-migrate-dry-run-run-id
kubectl -n consflow logs job/consflow-storage-migrate-dry-run-run-id
```

Chỉ sau khi dry-run pass, tạo overlay thứ hai có tên khác và JSON patch thêm `args: ["--apply"]`, rồi `kubectl create -k`/wait/log tương tự. Job mẫu `backoffLimit: 0`, mount legacy PVC read-only và không nằm trong application base. Nếu legacy files chỉ tồn tại trên Compose host, chạy cùng binary/contract tại host đó; không copy một phần tree vào pod tạm rồi coi là migration hoàn tất.

### Preflight

1. Chạy static validation:

   ```bash
   deploy/k8s/scripts/validate.sh
   kubectl kustomize /secure/consflow/application-overlay >/dev/null
   kubectl kustomize /secure/consflow/migration-overlay >/dev/null
   ```

2. Xác nhận Secret có đủ bốn key, runtime backend không mount `migration_database_url`, schema Job không mount `database_url`, và từng workload identity có thể mount; không in data bằng `kubectl get secret -o yaml` vào shared logs.
3. Xác nhận S3 object inventory/migration đã pass. Trên staging, backend startup preflight phải log `event=storage_preflight provider=s3 status=ready`; sau rollout tiếp tục chứng minh cross-replica protected upload/download.
4. Tính database connection upper bound. Với baseline hai backend pod và `DB_MAX_OPEN_CONNS=20`, application có thể mở 40 connection; giữ thêm migration, operator, monitoring và provider failover headroom.
5. Xác nhận migration của release là backward-compatible theo expand-contract. Advisory lock chống hai runner, không biến migration phá vỡ binary cũ thành an toàn.

### Compatible rolling release

1. Giữ release cũ phục vụ nếu toàn bộ migration là backward-compatible.
2. Tạo đúng một migration Job bằng release-unique overlay:

   ```bash
   kubectl create -k /secure/consflow/migration-overlay
   kubectl -n consflow wait --for=condition=complete --timeout=10m job/consflow-migrate-r20260722-shortsha
   kubectl -n consflow logs job/consflow-migrate-r20260722-shortsha
   ```

3. Nếu Job exit non-zero, dừng release. Không start backend mới, không chỉnh migration đã áp dụng và không retry bằng init container.
4. Sau Job thành công, apply application overlay và theo dõi tuần tự. Với required hostname anti-affinity trên cluster hai node, controller xóa một pod rồi tạo pod thay thế (`maxUnavailable: 1`, `maxSurge: 0`); ít nhất một replica vẫn phục vụ nhưng đây không phải zero-capacity-loss rollout:

   ```bash
   kubectl apply -k /secure/consflow/application-overlay
   kubectl -n consflow rollout status deployment/backend --timeout=10m
   kubectl -n consflow rollout status deployment/admin --timeout=10m
   kubectl -n consflow get pods,pdb,services
   ```

5. Smoke `/gateway-health`, `/api/health`, `/api/ready`, auth/read path và protected upload/download qua managed edge. Xác nhận public `/metrics` vẫn `404`.

### Incompatible migration / maintenance release

Nếu migration chưa expand-contract, không cho old/new overlap:

1. Bật maintenance ở managed edge, rút backend khỏi traffic và chờ request/SSE drain.
2. Scale `admin` và `backend` về 0; xác nhận pod cũ đã dừng.
3. Chạy backup gate rồi tạo migration Job như trên.
4. Chỉ sau Job thành công, apply overlay có `replicas: 2`, chờ readiness và smoke test.
5. Tắt maintenance sau khi cả hai Deployment available trên hai node và cross-replica checks pass.

## Rollback

1. Đóng/rút traffic nếu release mới có data/schema risk; lưu rollout status, events, application logs và migration Job logs trước cleanup.
2. Trước khi rollback image, chạy migration Job bằng **image rollback**. Runner phải xác nhận image manifest chứa mọi version đã ghi trong `schema_migrations`; nếu không, rollback đó không tương thích và phải dừng.
3. Nếu schema tương thích, đặt lại cả backend/admin immutable digests trong overlay rồi `kubectl apply -k`; theo dõi hai rollout và PDB.
4. Không rollback một mình application Secret khi password/database/Redis rotation đã đổi server-side credentials. Khôi phục một secret-manager version phải đi cùng trạng thái external service tương ứng.
5. Chỉ restore PostgreSQL khi change record chứng minh migration/data write yêu cầu restore; dùng `docs/ops/backup-restore.md` và giữ ứng dụng drain trong suốt restore.
6. Cutover metadata sang S3 là **forward-only** ở baseline này: không có reverse metadata trigger/Job. Rollback thông thường phải giữ một image tương thích S3 và khôi phục availability/credential của provider. Inventory/checksum manifest và source local read-only chỉ là bằng chứng/retention, không tự đảo metadata.
7. Revert về image pre-S3/local chỉ được làm trong maintenance window bằng restore đồng bộ database + local-object snapshot về đúng cutover point; mọi write phát sinh sau snapshot sẽ mất nếu không có reconciliation được review riêng.

## Availability and scale gates

- `replicas: 2`, `maxUnavailable: 1`, `maxSurge: 0`, required host anti-affinity, zone spread và PDB `minAvailable: 1` giữ ít nhất một replica phục vụ khi thay pod/drain tự nguyện và không deadlock rollout trên hai node. Cluster một node sẽ để replica thứ hai Pending thay vì tạo ảo giác HA.
- Liveness backend dùng dependency-free `/api/health`; readiness dùng DB-aware `/api/ready`. Redis degradation không làm readiness fail: monitor Redis riêng và coi local limiter/SSE fallback kéo dài là incident.
- Redis Pub/Sub chuyển invalidation giữa pod khi Redis hoạt động. Process/IP/actor SSE caps, Nginx `sse_global` và local limiter fallback vẫn là **per pod**; tổng cluster tăng theo replica và phải được edge/distributed quota giới hạn.
- `SSE_MAX_CONNECTION_LIFETIME` tạo reconnect/revalidation định kỳ. Staging phải chứng minh một client trên pod A nhận event phát từ pod B, reconnect qua pod khác, và failover không làm lộ project scope.
- Backend Service có annotation scrape `/metrics`. NetworkPolicy chỉ cho collector mang đủ namespace+pod label truy cập `backend:8080`; không route endpoint này qua admin/Ingress.
- `optional/hpa.yaml` không nằm trong base. Required hostname anti-affinity cần ít nhất một schedulable node cho mỗi replica; trước khi bật HPA phải có node capacity tới `maxReplicas` hoặc một overlay affinity khác đã review. Đồng thời Metrics Server phải ổn định, resource requests phản ánh tải thật, DB/Redis/object-store capacity chịu được `maxReplicas`, và scale-down SSE drain đã pass. CPU/memory HPA không đại diện trực tiếp cho queue/SSE demand.

## Secret rotation

1. Tạo version mới trong secret manager và xác minh external service chấp nhận giai đoạn overlap nếu provider hỗ trợ.
2. Đồng bộ `consflow-runtime-secrets`; backend đọc secret lúc startup nên phải rolling restart sau sync.
3. Rotate runtime database role và migration DDL/owner role như hai credential độc lập. Thu hồi runtime credential cũ chỉ sau pod mới ready; chỉ thu hồi migration credential cũ sau một release-unique schema Job dùng version mới thành công. Rotate Redis theo provider order tương tự.
4. JWT hiện dùng một signing secret, không có dual-key rotation contract. Đổi JWT secret sẽ vô hiệu token đang tồn tại; thực hiện trong maintenance window hoặc triển khai key-ring support qua review riêng.
5. Xác nhận rollout, auth refresh, DB/Redis reconnect, SSE và audit logs rồi mới thu hồi version cũ.
6. Không log Secret object, env dump, DSN, access key hoặc signed object URL.

## Validation scope

`deploy/k8s/scripts/validate.sh` dùng bundled `kubectl kustomize` để render base/migration và Ruby standard YAML parser để kiểm tra các invariant: hai replicas, non-deadlocking rolling strategy, probes, resources, PDB, anti-affinity/zone spread, private Services, ingress NetworkPolicy, tách runtime/migration database credential, separate migration Job, fail-closed placeholders và không có stateful database/Redis/HPA trong base.

Check này không liên hệ cluster và không thay thế server-side dry-run/policy admission trên staging cluster đúng Kubernetes version/CNI/IngressClass. Repo không tự deploy cluster thật.
