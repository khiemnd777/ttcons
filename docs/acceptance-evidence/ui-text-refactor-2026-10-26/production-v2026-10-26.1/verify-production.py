"""Đối chiếu release/image/schema và smoke HTTP sau pipeline; chỉ đọc production."""
from pathlib import Path
import subprocess, json, datetime, re, hashlib

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
TAG = 'v2026-10-26.1'
COMMIT = 'd0b36fa5b3c6ca97a27caf5dcd214f76dd91120a'
values = {}
for line in (ROOT / 'deploy/config/production.env').open():
    if '=' in line:
        key, value = line.rstrip('\n').split('=', 1)
        if key in {'VPS_HOST', 'VPS_PORT', 'VPS_USER'}:
            values[key] = value
remote = '''set -eu
current=$(readlink -f /opt/consflow/current)
printf 'CURRENT_RELEASE=%s\\n' "$current"
cat "$current/release.env"
for service in backend admin; do
  container=$(docker ps -q --filter label=com.docker.compose.project=consflow-prod --filter label=com.docker.compose.service="$service")
  printf 'SERVICE=%s ' "$service"
  docker inspect --format 'REVISION={{index .Config.Labels "org.opencontainers.image.revision"}} VERSION={{index .Config.Labels "org.opencontainers.image.version"}} HEALTH={{.State.Health.Status}}' "$container"
done
postgres_id=$(docker ps -q --filter label=com.docker.compose.project=consflow-prod --filter label=com.docker.compose.service=postgres)
printf 'MIGRATIONS='
docker exec --user postgres "$postgres_id" sh -c 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "SELECT count(*) FROM schema_migrations"'
'''
args = ['ssh', '-i', str(ROOT/'deploy/config/production_id_ed25519'), '-o', 'UserKnownHostsFile='+str(ROOT/'deploy/config/production_known_hosts'), '-o', 'StrictHostKeyChecking=yes', '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=15', '-p', values['VPS_PORT'], values['VPS_USER']+'@'+values['VPS_HOST'], 'bash -s']
result = subprocess.run(args, input=remote, text=True, capture_output=True, timeout=45)
assert result.returncode == 0, 'Read-only VPS verification failed'
lines = result.stdout.strip().splitlines()
assert f'CURRENT_RELEASE=/opt/consflow/releases/{TAG}' in lines, lines
for service in ['backend', 'admin']:
    assert f'SERVICE={service} REVISION={COMMIT} VERSION={TAG} HEALTH=healthy' in lines, lines
assert 'MIGRATIONS=160' in lines, lines
def get(path):
    return subprocess.check_output(['curl', '--fail', '--silent', '--show-error', '--max-time', '20', '--retry', '3', '--retry-delay', '2', '--retry-all-errors', 'https://tt-dnh.com'+path])
checks = {}
for path in ['/gateway-health', '/api/health', '/api/ready', '/login']:
    status = int(subprocess.check_output(['curl', '--fail', '--silent', '--show-error', '--max-time', '20', '--output', '/dev/null', '--write-out', '%{http_code}', 'https://tt-dnh.com'+path], text=True))
    assert status == (204 if path == '/gateway-health' else 200), (path, status)
    checks[path] = status
html = get('/login')
entry = re.search(rb'src="(/assets/index-[A-Za-z0-9_-]+\.js)"', html)
assert entry, 'Production entry script absent'
entry_path = entry.group(1).decode()
entry_body = get(entry_path)
chunk = re.search(rb'assets/NotificationsPage-[A-Za-z0-9_-]+\.js', entry_body)
assert chunk, 'Notification route chunk absent from production entry'
chunk_path = '/' + chunk.group().decode()
chunk_body = get(chunk_path)
assert 'Không tải được danh mục thông báo.'.encode() in chunk_body, 'New catalog error presentation absent'
assert 'Chưa có nhóm thông báo để cấu hình.'.encode() in chunk_body, 'New empty presentation absent'
summary = {
    'checked_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'status': 'PRODUCTION_DEPLOYMENT_VERIFIED', 'tag': TAG, 'source_commit': COMMIT,
    'workflow_run': 'https://github.com/khiemnd777/ttcons/actions/runs/40293383665',
    'vps_read_only': lines, 'public_http_status': checks,
    'frontend_entry': entry_path, 'frontend_entry_sha256': hashlib.sha256(entry_body).hexdigest(),
    'notification_chunk': chunk_path, 'notification_chunk_sha256': hashlib.sha256(chunk_body).hexdigest(),
    'new_notification_copy_present': True,
    'tls_and_ssh_verification': 'Enabled; pinned SSH host, system curl TLS trust',
    'database_migrations_before': 160, 'database_migrations_after': 160,
    'scope': 'Release/current symlink, exact OCI commit/version labels, healthy services, public HTTPS and delivered new frontend chunk',
    'limits': ['Không chạy browser hoặc đăng nhập production trong lượt deploy', 'Chưa gọi catalog có xác thực trên production; không dùng credentials bootstrap để smoke nghiệp vụ', 'Không ghi dữ liệu nghiệp vụ production'],
}
(HERE/'verification.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({key: summary[key] for key in ['status', 'tag', 'source_commit', 'public_http_status', 'database_migrations_after']}, ensure_ascii=False))
