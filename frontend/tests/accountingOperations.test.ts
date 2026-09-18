import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { readFile } from 'node:fs/promises';
import { build } from 'esbuild';
import { renderToStaticMarkup } from 'react-dom/server';
import { createElement } from 'react';
import { eligibleMISATransactions, reconciliationLabel, proposalMoneyInput } from '../src/modules/project-finance/accountingOperations.ts';
const require = createRequire(import.meta.url);
test('editing SQL numeric money does not resubmit padding as VND fractional digits',()=>{assert.equal(proposalMoneyInput('100.000'),'100');assert.equal(proposalMoneyInput('0.000'),'0');assert.equal(proposalMoneyInput('1.230'),'1.23')});
const react = require('react');
const project = '10000000-0000-4000-8000-000000000001';
const r = { id: 'request', project_id: project, basis_id: 'basis', title: 'Đợt chính xác', kind: 'progress', status: 'submitted', revision: 4, due_date: '2026-09-30', amount: '80', gross_amount: '100', recovery_amount: '10', retention_amount: '10', allocated_amount: '0', uncovered_amount: '80', currency: 'VND', partner_id: 'partner', partner_name: 'Đối tác', direction: 'outbound', evidence_ids: ['exact-file'], stock_document_ids: [], fingerprint: 'fp', reviewed: false };
const basis = { id: 'basis', source_type: 'procurement_request', source_id: '10000000-0000-4000-8000-000000000002', snapshot: { code: 'PO-1', title: 'Nguồn' }, currency: 'VND', evidence_ids: [], amount: '100' };
const data = { requests: [r], bases: [basis], sources: [], stock_documents: [], evidence: [{ id: 'exact-file', name: 'Tệp đã chọn' }], allocations: [], batches: [], reviews: [], records: [], events: [], upload_targets: [], capabilities: { 'accounting_request.review': true, 'accounting_request.submit': true, 'accounting_misa.reconcile': true } };
const bundle = await build({ stdin: { resolveDir: fileURLToPath(new URL('../src', import.meta.url)), contents: "export * from './modules/project-finance/AccountingOperationsDialog'; export * from './modules/project-finance/AccountingRequestQueue';" }, bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic', loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///test"' }, plugins: [{ name: 'contexts', setup(b) {
                b.onLoad({ filter: /AuthContext\.tsx$/ }, () => ({ contents: 'export function useAuth(){return {user:{id:"actor"},hasPermission:()=>true,hasAnyPermission:()=>true}}', loader: 'js' }));
                b.onLoad({ filter: /ToastProvider\.tsx$/ }, () => ({ contents: 'export function useToast(){return {success(){},warn(){},error(){}}}', loader: 'js' }));
                b.onLoad({ filter: /api\/client\.ts$/ }, () => ({ contents: 'export async function post(path,body){globalThis.__accountingCalls.push({path,body});return {}}; export async function get(){};export async function downloadBlob(){}; export async function postFormWithProgress(){}; export function resolveApiUrl(x){return x};', loader: 'js' }));
                b.onLoad({ filter: /AccountingOperationsDialog\.tsx$/ }, async ({ path }) => ({ contents: await readFile(path, 'utf8') + '\nexport {AccountingOperationForm};', loader: 'tsx' }));
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
    return { render(name = 'AccountingOperationsDialog', props: any = {}) { cursor = 0; return m[name]({ projectId: project, requestId: r.id, onClose() { }, ...props }); } };
}
test('MISA matches exact partner, direction, currency and active transaction only', () => {
    const match = { id: 'correct', partner_id: 'partner', currency: 'VND', direction: 'outbound', kind: 'transaction', active: true, available_amount: '80' };
    const rows = [match, { ...match, id: 'balance', kind: 'balance' }, { ...match, id: 'wrong-partner', partner_id: 'other' }, { ...match, id: 'old', active: false }, { ...match, id: 'USD', currency: 'USD' }, { ...match, id: 'inbound', direction: 'inbound' }];
    assert.deepEqual(eligibleMISATransactions(rows as any, r as any).map(x => x.id), ['correct']);
    assert.equal(reconciliationLabel(r), 'Cần xử lý');
    assert.match(reconciliationLabel({ ...r, reviewed: true }), /còn chênh lệch/);
});
test('loading and error do not present stale financial records or actions', () => {
    for (const overrides of [{ isFetching: true }, { isError: true }]) {
        const f = harness(overrides), tree = f.render();
        assert.equal(find(tree, n => n.type === 'h3' && n.props.children === r.title), undefined);
        assert.equal(find(tree, n => n.type === 'button' && n.props.children === 'Tiếp nhận'), undefined);
    }
});
test('request action uses displayed request ID and revision, not handoff identity', async () => {
    (globalThis as any).__accountingCalls = [];
    const f = harness();
    let tree = f.render();
    find(tree, n => n.type === 'button' && n.props.children === 'Tiếp nhận').props.onClick();
    tree = f.render();
    await find(tree, n => n.type === 'button' && n.props.children === 'Xác nhận').props.onClick();
    // onClick intentionally fires an async action; its request is made before the first await.
    assert.equal((globalThis as any).__accountingCalls[0].path, `/projects/${project}/accounting-requests/request/acknowledge`);
    assert.equal((globalThis as any).__accountingCalls[0].body.expected_revision, 4);
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
test('proposal forms render standard currency and uploader, no native money inputs', () => {
    const m = moduleFor((name: string) => name === '@tanstack/react-query' ? { ...require(name), useQueryClient: () => ({}), useQuery: () => ({}) } : require(name));
    const html = renderToStaticMarkup(createElement(m.AccountingOperationForm, { mode: 'request', data, projectId: project, onClose() { }, onSaved: async () => { } }));
    assert.match(html, /data-component="evidence-uploader"/);
    assert.match(html, /Giá trị đợt/);
    assert.doesNotMatch(html, /type="number"/);
    assert.match(html, /Phiếu xuất kho sẵn có/);
});
test('new and queue entrypoints preserve project and request identity', () => {
    const f = harness();
    let tree = f.render('AccountingOperationsButton', { projectId: project, requestId: 'selected-request' });
    find(tree, n => n.type === 'button').props.onClick();
    tree = f.render('AccountingOperationsButton', { projectId: project, requestId: 'selected-request' });
    const dialog = find(tree, n => n.type?.name === 'AccountingOperationsDialog');
    assert.equal(dialog.props.projectId, project);
    assert.equal(dialog.props.requestId, 'selected-request');
});
