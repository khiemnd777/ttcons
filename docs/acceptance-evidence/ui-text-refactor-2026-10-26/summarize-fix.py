from pathlib import Path
import json, hashlib, datetime, subprocess

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[2]
cases = {}
sources = [
    ('fix-browser-final', False), ('fix-browser-events-recheck', False), ('fix-layout-final', False),
    ('fix-protected-browser', True), ('fix-protected-mobile-final', True), ('fix-report-form-final', True),
]
errors = []
for directory, protected in sources:
    data = json.loads((HERE / directory / 'results.json').read_text())
    errors.extend(data.get('errors', []))
    for result in data['results']:
        result = {**result, 'evidence_dir': directory}
        key = f"{result.get('view', 'custom')}.{result['name']}"
        cases[key] = result
rows = list(cases.values())
assert len(rows) == 56, len(rows)
assert all(row['status'] == 'PASS' for row in rows), [(row['name'], row['status']) for row in rows if row['status'] != 'PASS']
assert not errors, errors
boundary = json.loads((HERE / 'fix-boundary.json').read_text())
assert not boundary['problems']
protected = [row for row in rows if row['name'].startswith('protected-')]
assert len(protected) == 14 and all(row['sameVisibleText'] for row in protected)
for row in rows:
    for key in ['screenshot', 'currentScreenshot', 'baselineScreenshot', 'currentRegion', 'baselineRegion', 'openScreenshot']:
        if row.get(key):
            file = HERE / row['evidence_dir'] / row[key]
            assert file.is_file(), file
            row[key + '_sha256'] = hashlib.sha256(file.read_bytes()).hexdigest()
def utcmtime(file):
    return datetime.datetime.fromtimestamp(file.stat().st_mtime, datetime.timezone.utc).isoformat()
summary = {
    'created_at_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'status': 'TECHNICALLY_VERIFIED_LOCAL',
    'human_acceptance': 'NOT UX ACCEPTED — chưa có người dùng đại diện hoặc thiết bị mobile thật',
    'baseline': boundary['baseline'], 'working_tree': 'main, chưa commit',
    'source_test_sha256': boundary['source_test_sha256'],
    'browser_counts': {'PASS': len(rows), 'FAIL': 0, 'BLOCKED': 0},
    'browser': 'Chrome156, Chromium sandbox bật; desktop1440×1000; mobile360/390/430×844 có touch emulation',
    'environment': {'frontend': 'local15173, baseline15174, cùng API18081', 'database': 'consflow_guide_20261022', 'migrations_before': 160, 'migrations_after': 160, 'auto_migrate': False, 'backend_image': 'sha256:c6e6f71d6e58d53b36593f512f01dad4b8906d8f6caf5732f91e53ac000c6cf3'},
    'checks': {
        'frontend_tests': {'status': 'PASS', 'pass': 622, 'fail': 0, 'log_finished_at_utc': utcmtime(HERE / 'fix-frontend-tests.txt')},
        'frontend_build': {'status': 'PASS', 'scope': 'TypeScript + Vite; VITE_API_URL=local18081', 'log_finished_at_utc': utcmtime(HERE / 'fix-frontend-build.txt')},
        'bundle_budget': {'status': 'PASS', 'chunks': 133},
        'backend': {'real_postgres_catalog_regression': 'PASS', 'notification_filter_and_template_tests': 'PASS', 'test_database': 'consflow_ui_fix_test_9fb9d24ace', 'full_backend_suite': 'NOT_RUN', 'changed_source_sha256': {name: hashlib.sha256((ROOT / 'backend/internal/core' / name).read_bytes()).hexdigest() for name in ['repository_notifications.go', 'notification_catalog_test.go']}, 'build_log_finished_at_utc': utcmtime(HERE / 'fix-backend-build.log')},
        'protected_source': boundary,
        'protected_browser': {'status': 'PASS', 'comparisons': 14, 'scope': 'Visible text; full WorkflowFormPanel crop for embedded Form, excluding approved outer AppShell tagline change; no universal pixel-identity claim'},
    },
    'catalog': {'get_status': 200, 'items': 83, 'enabled_events': 21, 'enabled_categories': 6, 'all_current_categories_required': True, 'permission_arrays_preserved': True},
    'controlled_http_cases': 'Error/empty/loading, optional category writes and partial failure use labelled browser HTTP fixtures. No catalog or preference database rows changed to make testing possible; real local fixture only has required categories.',
    'excluded_attempts': 'First concurrent run hit auth budgets and was excluded. Three later Event Logs cases used an unavailable fixed project ID; replaced by successful rechecks selecting actual API facet options. Raw receipts remain for audit; latest result wins by case key.',
    'limits': ['Chưa kiểm tra điện thoại vật lý hoặc người dùng đại diện', 'Không chạy toàn bộ backend tests', 'Không gửi/duyệt/xóa nghiệp vụ; không kiểm tra mọi Form/validation/camera', 'Chưa có ca lưu nhóm tùy chọn qua backend thật vì sáu nhóm local đều bắt buộc; luồng UI lưu được thử bằng HTTP fixtures'],
    'results': rows,
}
(HERE / 'fix-summary.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2) + '\n')
print(json.dumps({k: summary[k] for k in ['status', 'browser_counts', 'source_test_sha256']}, ensure_ascii=False))
