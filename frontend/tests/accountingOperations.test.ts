import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { readFile } from 'node:fs/promises';
import { build } from 'esbuild';
import { renderToStaticMarkup } from 'react-dom/server';
import { createElement } from 'react';
import { eligibleMISATransactions, reconciliationLabel } from '../src/modules/project-finance/accountingOperations.ts';
const require = createRequire(import.meta.url);
const react = require('react');
const project = '10000000-0000-4000-8000-000000000001';
const basis = { id: 'basis', source_type: 'procurement_request', source_id: '10000000-0000-4000-8000-000000000002', snapshot: { code: 'PO-1', title: 'Nguồn chính xác' }, currency: 'VND', evidence_ids: ['exact-file'], amount: '100', allocated_amount: '0', difference_amount: '-100', fingerprint: 'fp', reviewed: false, partner_id: 'partner', partner_name: 'Đối tác', direction: 'outbound', note: 'Căn cứ nguồn' };
const matchingRecord = { id: 'misa-exact', external_id: 'CT-01', record_date: '2026-10-05', partner_id: 'partner', currency: 'VND', direction: 'outbound', kind: 'transaction', active: true, available_amount: '80' };
const data = { project: { id: project, code: 'CT-01', name: 'Công trình' }, bases: [basis], sources: [], stock_documents: [], evidence: [{ id: 'exact-file', name: 'Tệp đã chọn' }], allocations: [], batches: [], reviews: [], records: [matchingRecord], events: [], upload_targets: [], capabilities: { 'accounting_reconciliation.manage': true, 'accounting_misa.reconcile': true } };
const bundle = await build({ stdin: { resolveDir: fileURLToPath(new URL('../src', import.meta.url)), contents: "export * from './modules/project-finance/AccountingOperationsDialog';" }, bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic', loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///test"' }, plugins: [{ name: 'contexts', setup(b) {
                b.onLoad({ filter: /AuthContext\.tsx$/ }, () => ({ contents: 'export function useAuth(){return {user:{id:"actor"},hasPermission:()=>true,hasAnyPermission:()=>true}}', loader: 'js' }));
                b.onLoad({ filter: /ToastProvider\.tsx$/ }, () => ({ contents: 'export function useToast(){return {success(){},warn(){},error(){}}}', loader: 'js' }));
                b.onLoad({ filter: /api\/client\.ts$/ }, () => ({ contents: 'export async function post(path,body){globalThis.__accountingCalls.push({path,body});return {}}; export async function get(){};export async function downloadBlob(){}; export async function postFormWithProgress(){}; export function resolveApiUrl(x){return x};', loader: 'js' }));
                b.onLoad({ filter: /AccountingOperationsDialog\.tsx$/ }, async ({ path }) => ({ contents: await readFile(path, 'utf8') + '\nexport {AccountingOperationForm, SourceLinks, Money};', loader: 'tsx' }));
            } }] });
function moduleFor(mock: any) { const m = { exports: {} as any }; new Function('require', 'module', 'exports', bundle.outputFiles[0].text)(mock, m, m.exports); return m.exports; }
function find(n: any, p: (n: any) => boolean): any { if (!n || typeof n !== 'object')
    return; if (p(n))
    return n; for (const child of react.Children.toArray(n.props?.children)) {
    const hit = find(child, p);
    if (hit)
        return hit;
} }
function harness(overrides: any = {}) {
    const states: any[] = [];
    let cursor = 0;
    const m = moduleFor((name: string) => name === 'react' ? { ...react, useState(initial: any) { const i = cursor++; if (!(i in states))
            states[i] = typeof initial === 'function' ? initial() : initial; return [states[i], (value: any) => states[i] = typeof value === 'function' ? value(states[i]) : value]; } } : name === '@tanstack/react-query' ? { ...require(name), useQuery: () => ({ data, isFetchedAfterMount: true, isFetching: false, isError: false, refetch: async () => { }, ...overrides }), useQueryClient: () => ({ invalidateQueries: async () => { } }) } : require(name));
    return { render(name = 'AccountingOperationsDialog', props: any = {}) { cursor = 0; return m[name]({ projectId: project, basisId: basis.id, onClose() { }, ...props }); } };
}
test('MISA matches exact partner, direction, currency and active transaction only', () => {
    const match = { id: 'correct', partner_id: 'partner', currency: 'VND', direction: 'outbound', kind: 'transaction', active: true, available_amount: '80' };
    const rows = [match, { ...match, id: 'balance', kind: 'balance' }, { ...match, id: 'wrong-partner', partner_id: 'other' }, { ...match, id: 'old', active: false }, { ...match, id: 'USD', currency: 'USD' }, { ...match, id: 'inbound', direction: 'inbound' }];
    assert.deepEqual(eligibleMISATransactions(rows as any, basis as any).map(x => x.id), ['correct']);
    assert.equal(reconciliationLabel(basis), 'Chưa ghép giao dịch');
    assert.match(reconciliationLabel({ ...basis, reviewed: true }), /có chênh lệch/);
    assert.equal(reconciliationLabel({ ...basis, allocated_amount: '40', difference_amount: '-60' }), 'Khớp một phần');
    assert.equal(reconciliationLabel({ ...basis, allocated_amount: '120', difference_amount: '20' }), 'Ghép vượt giá trị căn cứ');
    assert.equal(reconciliationLabel({ ...basis, allocated_amount: '100', difference_amount: '0', reviewed: true }), 'Đã đối soát · khớp giá trị');
});
test('money display preserves large decimal strings, negative cents and integer VND with explicit currency', () => {
    const m = moduleFor(require);
    const money = (value: string, currency: string) => renderToStaticMarkup(createElement(m.Money, { value, currency }));
    assert.equal(money('999999999999999.99', 'USD'), '<span>999.999.999.999.999,99 USD</span>');
    assert.equal(money('-0.01', 'USD'), '<span>-0,01 USD</span>');
    assert.equal(money('999999999999999.00', 'VND'), '<span>999.999.999.999.999 VND</span>');
});
test('difference filter retains unmatched, partial and excess dossiers regardless of review status', () => {
    const cases = [
        { allocated_amount: '0', difference_amount: '-100', reviewed: false, visible: true },
        { allocated_amount: '80', difference_amount: '-20', reviewed: false, visible: true },
        { allocated_amount: '80', difference_amount: '-20', reviewed: true, visible: true },
        { allocated_amount: '120', difference_amount: '20', reviewed: true, visible: true },
        { allocated_amount: '100', difference_amount: '0', reviewed: false, visible: false },
        { allocated_amount: '100', difference_amount: '0', reviewed: true, visible: false },
    ];
    for (const row of cases) {
        const f = harness({ data: { ...data, bases: [{ ...basis, ...row }] } });
        let tree = f.render();
        find(tree, n => n.type?.name === 'WorkspaceTabs' && n.props.ariaLabel === 'Lọc hồ sơ đối soát').props.onChange('partial');
        tree = f.render();
        assert.equal(Boolean(find(tree, n => n.type?.name === 'SourceLinks')), row.visible, JSON.stringify(row));
    }
});
test('MISA results distinguish balance labels from actual receipt and payment transactions', () => {
    for (const row of [
        { kind: 'transaction', direction: 'inbound', label: 'Thu' },
        { kind: 'transaction', direction: 'outbound', label: 'Chi' },
        { kind: 'balance', direction: 'inbound', label: 'Số dư phải thu' },
        { kind: 'balance', direction: 'outbound', label: 'Số dư phải trả' },
    ]) {
        const f = harness({ data: { ...data, records: [{ ...matchingRecord, ...row, amount: '80', line_key: 'line', entity_code: 'company' }] } });
        let tree = f.render();
        find(tree, n => n.type?.name === 'WorkspaceTabs' && n.props.ariaLabel === 'Hồ sơ và đối soát').props.onChange('misa');
        tree = f.render();
        assert.ok(find(tree, n => n.type === 'p' && Array.isArray(n.props.children) && n.props.children[0] === row.label), JSON.stringify(row));
        if (row.kind === 'balance') {
            assert.equal(find(tree, n => n.type === 'p' && Array.isArray(n.props.children) && ['Thu', 'Chi'].includes(n.props.children[0])), undefined);
        }
    }
});
test('history renders the immutable large fractional amount without converting its source string', () => {
    const value = '999999999999999.99';
    const f = harness({ data: { ...data, bases: [{ ...basis, currency: 'USD' }], events: [{ id: 'large-event', basis_id: basis.id, action: 'accounting_basis_confirmed', created_at: '2026-10-05', snapshot: { amount: value } }] } });
    let tree = f.render();
    find(tree, n => n.type?.name === 'WorkspaceTabs').props.onChange('history');
    tree = f.render();
    const money = find(tree, n => n.type?.name === 'Money' && n.props.value === value);
    assert.ok(money);
    assert.equal(renderToStaticMarkup(money), '<span>999.999.999.999.999,99 USD</span>');
});
test('loading and error do not present stale financial records or actions', () => {
    for (const overrides of [{ isFetching: true }, { isError: true }]) {
        const f = harness(overrides), tree = f.render();
        assert.equal(find(tree, n => n.type === 'button' && n.props.children === 'Ghép với hồ sơ'), undefined);
        assert.equal(find(tree, n => n.type === 'button' && n.props.children === 'Xác nhận đối soát'), undefined);
    }
});
test('allocation rejects money beyond the remaining MISA transaction inline before posting', () => {
    (globalThis as any).__accountingCalls = [];
    const f = harness();
    let tree = f.render();
    find(tree, n => n.type === 'select').props.onChange({ target: { value: 'misa-exact' } });
    find(tree, n => typeof n.props?.onValueChange === 'function').props.onValueChange({ numericString: '81' });
    tree = f.render();
    find(tree, n => n.type === 'button' && n.props.children === 'Ghép với hồ sơ').props.onClick();
    tree = f.render();
    assert.equal((globalThis as any).__accountingCalls.length, 0);
    assert.equal(find(tree, n => n.type?.name === 'Field' && n.props.label === 'Số tiền ghép').props.error, true);
});
test('read-only reconciliation capability shows source dossier without mutation controls', () => {
    const tree = harness({ data: { ...data, capabilities: {} } }).render();
    assert.equal(find(tree, n => n.type === 'button' && n.props.children === 'Xác nhận căn cứ'), undefined);
    assert.equal(find(tree, n => n.type === 'button' && n.props.children === 'Nhập file MISA'), undefined);
    assert.equal(find(tree, n => n.type === 'button' && n.props.children === 'Ghép với hồ sơ'), undefined);
    assert.ok(find(tree, n => n.type?.name === 'SourceLinks' && n.props.basis.id === basis.id));
});
test('unavailable source preserves history context but blocks new matching and review', () => {
    const tree = harness({ data: { ...data, bases: [{ ...basis, source_available: false, source_unavailable_reason: 'Nguồn không đủ điều kiện hiện tại.' }] } }).render();
    assert.ok(find(tree, n => n.props?.role === 'alert' && n.props.children === 'Nguồn không đủ điều kiện hiện tại.'));
    assert.equal(find(tree, n => n.type === 'button' && n.props.children === 'Ghép với hồ sơ'), undefined);
    assert.equal(find(tree, n => n.type === 'button' && n.props.children === 'Xác nhận đối soát'), undefined);
    assert.ok(find(tree, n => n.type?.name === 'Files'));
});
test('source dossier opens the shared detail dialog for the exact source without a recursive reconciliation launcher', () => {
    const f = harness();
    let tree = f.render('SourceLinks', { basis });
    find(tree, n => n.type === 'button' && n.props.children === 'Mở hồ sơ').props.onClick();
    tree = f.render('SourceLinks', { basis });
    const dialog = find(tree, n => n.type?.name === 'AccountingHandoffDetailDialog');
    assert.deepEqual(dialog.props.target, { project_id: project, source_type: basis.source_type, source_id: basis.source_id });
    assert.equal(dialog.props.showReconciliation, false);
});
test('allocation targets the displayed basis and exact MISA record without a proposal', async () => {
    (globalThis as any).__accountingCalls = [];
    const f = harness({ data: { ...data, bases: [{ ...basis, amount: '20', difference_amount: '-20' }] } });
    let tree = f.render();
    find(tree, n => n.type === 'select').props.onChange({ target: { value: 'misa-exact' } });
    find(tree, n => typeof n.props?.onValueChange === 'function' && n.props.currencyCode === 'VND').props.onValueChange({ numericString: '40' });
    tree = f.render();
    find(tree, n => n.type === 'button' && n.props.children === 'Ghép với hồ sơ').props.onClick();
    assert.equal((globalThis as any).__accountingCalls[0].path, `/projects/${project}/misa-allocations`);
    assert.equal((globalThis as any).__accountingCalls[0].body.basis_id, 'basis');
    assert.equal((globalThis as any).__accountingCalls[0].body.record_id, 'misa-exact');
    assert.equal((globalThis as any).__accountingCalls[0].body.amount, '40');
    assert.equal('request_id' in (globalThis as any).__accountingCalls[0].body, false);
});
test('review submits current basis fingerprint and note', () => {
    (globalThis as any).__accountingCalls = [];
    const f = harness();
    let tree = f.render();
    find(tree, n => n.type === 'textarea').props.onChange({ target: { value: 'Đã đối chiếu hồ sơ' } });
    tree = f.render();
    find(tree, n => n.type === 'button' && n.props.children === 'Xác nhận đối soát').props.onClick();
    assert.equal((globalThis as any).__accountingCalls[0].path, `/projects/${project}/misa-reviews`);
    assert.deepEqual((globalThis as any).__accountingCalls[0].body, { basis_id: 'basis', note: 'Đã đối chiếu hồ sơ', fingerprint: 'fp' });
});
test('file and batch viewers preserve exact identities', () => {
    const f = harness();
    let tree = f.render();
    const files = find(tree, n => n.type?.name === 'Files' && n.props.ids.includes('exact-file'));
    files.props.open('exact-file');
    tree = f.render();
    assert.equal(find(tree, n => n.type?.name === 'EvidenceReferenceViewer').props.reference.id, 'exact-file');
    const g = harness({ data: { ...data, batches: [{ id: 'batch-exact', filename: 'source.csv', status: 'confirmed' }] } });
    let second = g.render();
    find(second, n => n.type?.name === 'WorkspaceTabs').props.onChange('history');
    second = g.render();
    find(second, n => n.type === 'button' && n.props.children === 'Xem file gốc').props.onClick();
    second = g.render();
    assert.equal(find(second, n => n.type?.name === 'ProtectedDocumentViewerDialog').props.protectedFilePath, `/projects/${project}/misa-imports/batch-exact/file`);
});
test('basis confirmation renders standard currency and uploader, without proposal fields', () => {
    const m = moduleFor((name: string) => name === '@tanstack/react-query' ? { ...require(name), useQueryClient: () => ({}), useQuery: () => ({}) } : require(name));
    const html = renderToStaticMarkup(createElement(m.AccountingOperationForm, { mode: 'basis', data, projectId: project, onClose() { }, onSaved: async () => { } }));
    assert.match(html, /data-component="evidence-uploader"/);
    assert.match(html, /Giá trị căn cứ đã xác nhận/);
    assert.doesNotMatch(html, /type="number"/);
    assert.doesNotMatch(html, /Tên đợt|Giữ lại|Thu hồi tạm ứng|Lưu nháp/);
});
test('source entrypoints preserve project and basis identity', () => {
    const f = harness();
    let tree = f.render('AccountingOperationsButton', { projectId: project, basisId: 'selected-basis' });
    find(tree, n => n.type === 'button').props.onClick();
    tree = f.render('AccountingOperationsButton', { projectId: project, basisId: 'selected-basis' });
    const dialog = find(tree, n => n.type?.name === 'AccountingOperationsDialog');
    assert.equal(dialog.props.projectId, project);
    assert.equal(dialog.props.basisId, 'selected-basis');
});
