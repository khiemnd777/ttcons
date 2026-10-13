"""Read-only audit of mobile Utility placement and retained BM04 export/history evidence."""
import hashlib
import json
from pathlib import Path
import zipfile
import openpyxl

OUT = Path(__file__).resolve().parent
ROOT = OUT.parents[2]
PRIOR = ROOT / 'docs/acceptance-evidence/bm04-list-detail-development-2026-10-13/action-column-fix'


def load(path):
    return json.loads(path.read_text())


def workbook_content(path):
    workbook = openpyxl.load_workbook(path, read_only=False, data_only=False)
    return {sheet.title: {(c.row, c.column): c.value for row in sheet for c in row if c.value not in [None, '']} for sheet in workbook}


def media(path):
    with zipfile.ZipFile(path) as package:
        return {name: hashlib.sha256(package.read(name)).hexdigest() for name in package.namelist() if name.startswith('xl/media/')}


def verify():
    before = {**load(OUT / 'before-database.json'), **load(OUT / 'before-construction-history.json')}
    after = {**load(OUT / 'after-database.json'), **load(OUT / 'after-construction-history.json')}
    changes, additions, checked = [], [], 0
    for table, rows in before.items():
        if not isinstance(rows, list):
            continue
        old = {r['id']: r['md5'] for r in rows}
        now = {r['id']: r['md5'] for r in after[table]}
        checked += len(old)
        changes.extend((table, key) for key in old if now.get(key) != old[key])
        additions.extend((table, key) for key in now if key not in old)
    original = load(OUT / 'source-before.json')['files']
    built = load(OUT / 'source-build-manifest.json')['files']
    actual = {name: hashlib.sha256((ROOT / name).read_bytes()).hexdigest() for name in built}
    delta = {name for name in original if original[name] != actual[name]}
    allowed = {'frontend/src/layouts/AppShell.tsx', 'frontend/src/modules/projects/ProjectWorkspaceLayout.tsx', 'frontend/src/modules/utilities/UtilityDock.tsx', 'frontend/tests/utilityDock.test.ts'}
    measurements = load(OUT / 'browser-measurements.json')
    by_name = {m['scenario']: m for m in measurements}
    mobile = [by_name['mobile' + str(w) + '-end'] for w in [320, 390, 430, 639]]
    desktop = [by_name['desktop' + str(w)] for w in [1024, 1280]]
    draft = load(OUT / 'browser-draft-preservation.json')
    standard = by_name['standard390-project-list']
    assertions = {
        'all_2889_baseline_rows_unchanged_and_retained': not changes and checked == 2889,
        'only_two_new_export_audits': len(additions) == 2 and all(t == 'audit_logs' for t, _ in additions) and all(r['event_type'] == 'native_source_exported' for r in load(OUT / 'new-audit-rows.json')),
        'migration_158_unchanged': before['migration_count'] == after['migration_count'] == 158,
        'all_149_storage_files_unchanged': load(OUT / 'storage-before.json') == load(OUT / 'storage-after.json'),
        'current_source_matches_built_manifest': actual == built,
        'only_approved_existing_source_owners_changed': delta == allowed,
        'accepted_desktop_source_unchanged': actual['frontend/src/modules/operations/ConstructionWorkspaceLists.tsx'] == original['frontend/src/modules/operations/ConstructionWorkspaceLists.tsx'],
        'one_labelled_header_launcher_at_all_mobile_sizes': all(m['launcherCount'] == 1 and m['launcherInHeader'] and m['launcherText'] == 'Tiện ích 15' for m in mobile),
        'mobile_no_document_horizontal_overflow': all(m['documentWidth'] == m['viewport']['width'] for m in mobile),
        'mobile_end_cta_visible_uncovered_and_hit_test_passes': all(m['intersectionArea'] == 0 and m['ctaVisible'] and m['ctaCenterHit'] for m in mobile),
        'header_launcher_inside_own_header': all(m['launcherRect']['top'] >= m['headerRect']['top'] and m['launcherRect']['bottom'] <= m['headerRect']['bottom'] and m['launcherRect']['right'] <= m['viewport']['width'] for m in mobile),
        'middle_scroll_observed_with_no_overlap': 0 < by_name['mobile390-middle']['scrollY'] < by_name['mobile390-end']['scrollY'] and by_name['mobile390-middle']['intersectionArea'] == 0,
        'empty_state_retains_single_header_launcher': by_name['mobile390-empty']['launcherCount'] == 1 and by_name['mobile390-empty']['launcherInHeader'],
        'standard_header_keeps_one_icon_launcher': standard['launcherCount'] == 1 and standard['launcherInHeader'] and standard['launcherRect']['width'] == 40 and standard['launcherText'] == '15',
        '640_boundary_hides_mobile_launcher': by_name['boundary640-end']['launcherCount'] == 0 and by_name['boundary640-end']['ctaCenterHit'],
        'desktop_action_column_geometry_unchanged': all(m['buttonsInsideActionCell'] and m['horizontalOverlapPixels'] == 0 and m['titleGapPixels'] == 24 for m in desktop),
        'actual_browser_draft_preserved_after_spa_navigation': draft['preserved'] and draft['afterNavigation'] == draft['expected'],
        'test_draft_cleared_without_sending': draft['cleared'] and not draft['sent'],
    }
    for label, download in load(OUT / 'downloads.json').items():
        path = OUT / ('BM04-' + label + '.xlsx')
        prior = PRIOR / path.name
        assertions[label + '_actual_download_byte_equal'] = path.read_bytes() == Path(download['download_path']).read_bytes()
        assertions[label + '_all_cells_formulas_unchanged'] = workbook_content(path) == workbook_content(prior)
        assertions[label + '_embedded_photo_unchanged'] = media(path) == media(prior) and len(media(path)) == 1
    return {'passed': all(assertions.values()), 'assertions': assertions, 'old_unique_rows_checked': checked, 'changed_or_lost_rows': changes, 'new_rows': additions, 'source_files_checked': len(built), 'source_delta': sorted(delta), 'limits': 'Development Admin, one existing BM04 open action plus empty filter; actual source UI screenshots, mobile/desktop geometry, real draft SPA navigation and unmodified downloads. Google import, physical Safari/Android safe-area/keyboard and capacity/all-role checks NOT_RUN. Standard MyWork390 documentWidth663 was observed separately; its cause not investigated. User UX sign-off pending.'}


if __name__ == '__main__':
    result = verify()
    (OUT / 'verification.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps(result, ensure_ascii=False, indent=2))
    assert result['passed']
