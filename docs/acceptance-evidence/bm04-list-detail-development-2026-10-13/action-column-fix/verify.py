"""Read-only evidence audit of the approved BM04 desktop action-column repair."""
import hashlib
import json
from pathlib import Path
import zipfile

import openpyxl

OUT = Path(__file__).resolve().parent
PRIOR = OUT.parent
ROOT = OUT.parents[3]
OWNER = 'frontend/src/modules/operations/ConstructionWorkspaceLists.tsx'


def load(path):
    return json.loads(path.read_text())


def cells(path):
    workbook = openpyxl.load_workbook(path, read_only=False, data_only=False)
    return {sheet.title: {(cell.row, cell.column): cell.value for row in sheet for cell in row if cell.value not in [None, '']} for sheet in workbook}


def media(path):
    with zipfile.ZipFile(path) as package:
        return {name: hashlib.sha256(package.read(name)).hexdigest() for name in package.namelist() if name.startswith('xl/media/')}


def verify():
    before = {**load(OUT / 'before-database.json'), **load(OUT / 'before-construction-history.json')}
    after = {**load(OUT / 'after-database.json'), **load(OUT / 'after-construction-history.json')}
    changed, added, checked = [], [], 0
    for table, rows in before.items():
        if not isinstance(rows, list):
            continue
        old = {row['id']: row['md5'] for row in rows}
        current = {row['id']: row['md5'] for row in after[table]}
        checked += len(old)
        changed.extend((table, key) for key in old if current.get(key) != old[key])
        added.extend((table, key) for key in current if key not in old)
    source = load(OUT / 'source-build-manifest.json')['files']
    hashes = {name: hashlib.sha256((ROOT / name).read_bytes()).hexdigest() for name in source}
    original = load(PRIOR / 'source-manifest.json')['files']
    delta = [name for name in original if hashes[name] != original[name]]
    before_owner = (OUT / 'ConstructionWorkspaceLists.before.tsx').read_text()
    current_owner = (ROOT / OWNER).read_text()
    marker = '<div className="space-y-3 lg:hidden">'
    old_action_start = before_owner.index('export function ConstructionActionList(')
    new_action_start = current_owner.index('export function ConstructionActionList(')
    old_mobile = before_owner[before_owner.index(marker, old_action_start):]
    new_mobile = current_owner[current_owner.index(marker, new_action_start):]
    measurements = load(OUT / 'browser-measurements.json')
    desktop = [row for row in measurements if row['viewport']['width'] >= 1024]
    narrow = next(row for row in desktop if row['viewport']['width'] == 1024)
    assertions = {
        'old_rows_unchanged_and_not_lost': not changed,
        'only_two_new_export_audits': len(added) == 2 and all(table == 'audit_logs' for table, _ in added) and all(row['event_type'] == 'native_source_exported' for row in load(OUT / 'new-audit-rows.json')),
        'migration_count_158_unchanged': before['migration_count'] == after['migration_count'] == 158,
        'all_976_built_source_hashes_current': hashes == source and len(source) == 976,
        'only_approved_ui_owner_changed': delta == [OWNER],
        'mobile_branch_unchanged': old_mobile == new_mobile,
        'all_149_storage_files_unchanged': load(OUT / 'storage-after.json') == load(ROOT / 'docs/acceptance-evidence/bm08-rev10-development-2026-10-13/storage-after.json'),
        'three_desktop_sizes_measured': {row['viewport']['width'] for row in desktop} == {1024, 1280, 1440},
        'all_desktop_buttons_inside_cell_no_title_overlap': all(row['buttonsInsideActionCell'] and row['horizontalOverlapPixels'] == 0 and row['titleGapPixels'] >= 24 for row in desktop),
        'narrow_table_scroll_observed_without_document_overflow': narrow['tableScrollObserved']['scrollLeft'] > 0 and narrow['tableScrollObserved']['documentWidth'] == 1024,
        'mobile_no_document_horizontal_overflow': measurements[-1]['documentWidth'] == 390,
    }
    downloads = load(OUT / 'downloads.json')
    for label in ['list', 'detail']:
        path = OUT / ('BM04-' + label + '.xlsx')
        prior = PRIOR / path.name
        assertions[label + '_actual_download_byte_equal'] = path.read_bytes() == Path(downloads[label]['download_path']).read_bytes()
        assertions[label + '_all_cells_formulas_unchanged'] = cells(path) == cells(prior)
        assertions[label + '_same_embedded_photo'] = media(path) == media(prior) and len(media(path)) == 1
    return {'passed': all(assertions.values()), 'assertions': assertions, 'old_unique_rows_checked': checked, 'changed_or_lost_rows': changed, 'new_rows': added, 'source_delta': delta, 'existing_mobile_utility_overlap': measurements[-1]['existingUtilityOverlap'], 'limits': 'Admin synthetic Development, one open source. Desktop action-column repair verified. Existing mobile utility overlap remains unresolved. New downloads have identical cells/formulas/media to prior native Google verification; Google import was not repeated in this CSS-only turn. Other lifecycle rows, ordinary-role, capacity, native Excel and print NOT_RUN; user UX review pending.'}


if __name__ == '__main__':
    result = verify()
    (OUT / 'verification.json').write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps(result, ensure_ascii=False, indent=2))
    assert result['passed']
