from pathlib import Path
import json, hashlib, datetime, subprocess, urllib.request

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
previous = json.loads((HERE.parent / 'fix-summary.json').read_text())
boundary = json.loads(subprocess.check_output(['node', str(HERE.parent / 'verify-boundary.mjs')], cwd=ROOT, text=True))
assert boundary['source_test_sha256'] == previous['source_test_sha256'], 'Source changed since the tested build'
assert not boundary['problems'], boundary['problems']
revision = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
branch = subprocess.check_output(['git', 'branch', '--show-current'], cwd=ROOT, text=True).strip()
assert revision == previous['baseline'], 'Git revision changed'
backend_sources = previous['checks']['backend']['changed_source_sha256']
for name, expected in backend_sources.items():
    assert hashlib.sha256((ROOT / 'backend/internal/core' / name).read_bytes()).hexdigest() == expected, f'Backend source changed: {name}'
rows = []; errors = []
for directory in ['ui', 'protected']:
    data = json.loads((HERE / directory / 'results.json').read_text())
    assert not data.get('runner_error'), data.get('runner_error')
    errors.extend(data.get('errors', []))
    for row in data['results']:
        row = {**row, 'evidence_dir': directory}
        for key in ['screenshot', 'currentScreenshot', 'baselineScreenshot', 'currentRegion', 'baselineRegion', 'openScreenshot']:
            if row.get(key):
                file = HERE / directory / row[key]
                assert file.is_file(), file
                row[key + '_sha256'] = hashlib.sha256(file.read_bytes()).hexdigest()
        rows.append(row)
assert len(rows) == 56, len(rows)
protected = [row for row in rows if row['name'].startswith('protected-')]
assert len(protected) == 14
counts = {status: sum(row['status'] == status for row in rows) for status in ['PASS', 'FAIL', 'BLOCKED', 'CHECK_REQUIRED', 'NOT_RUN']}
passed = counts['PASS'] == 56 and not errors and all(row['sameVisibleText'] for row in protected)
with urllib.request.urlopen('http://127.0.0.1:15173', timeout=10) as response:
    served_html = response.read()
assert served_html == (ROOT / 'frontend/dist/index.html').read_bytes(), 'Preview is serving a different build'
image = subprocess.check_output(['docker', 'inspect', '--format', '{{.Image}}', 'consflow-guide-20261022-backend-1'], text=True).strip()
assert image == previous['environment']['backend_image'], image
migrations = int(subprocess.check_output(['docker', 'exec', 'consflow-guide-20261022-postgres-1', 'psql', '-U', 'guide_operator', '-d', 'consflow_guide_20261022', '-At', '-c', 'SELECT count(*) FROM schema_migrations'], text=True).strip())
assert migrations == 160
summary = {
    'checked_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'status': 'LOCAL_UAT_PASS_AGENT' if passed else 'LOCAL_UAT_REQUIRES_REVIEW',
    'scope': 'Nghiệm thu local do agent theo yêu cầu Chạy nghiệm thu local; không phải xác nhận chấp nhận của người dùng',
    'counts': counts, 'page_errors': errors,
    'source_revision': revision, 'working_tree': branch + ', chưa commit',
    'source_test_sha256': boundary['source_test_sha256'],
    'served_frontend_html_sha256': hashlib.sha256(served_html).hexdigest(),
    'build_finished_at_utc': previous['checks']['frontend_build']['log_finished_at_utc'],
    'backend_image': image, 'database': 'consflow_guide_20261022', 'migrations': migrations,
    'backend_source_sha256': backend_sources,
    'fresh_checks': ['56 ca browser chạy lại tuần tự', '14 đối chiếu Pipeline/Form với baseline', '112 file và12 vùng source bảo vệ', 'Source fingerprint và preview HTML khớp build đã kiểm chứng', 'Backend image và DB migration state không đổi'],
    'reused_unchanged_build_checks': previous['checks'],
    'browser_environment': previous['browser'],
    'fixture_scope': previous['controlled_http_cases'],
    'limits': previous['limits'],
    'protected_comparisons': len(protected),
    'application_changes_this_run': False, 'commit_push_deploy': False,
    'mapping_maintenance': 'Không sửa ứng dụng, contracts, owner hoặc agent workflow; không cần cập nhật mapping',
    'results': rows,
}
(HERE / 'summary.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2) + '\n')
print(json.dumps({key: summary[key] for key in ['status', 'counts', 'protected_comparisons', 'source_test_sha256']}, ensure_ascii=False))
raise SystemExit(0 if passed else 1)
