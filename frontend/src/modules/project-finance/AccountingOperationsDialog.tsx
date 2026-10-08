import { useState, type ReactNode } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ClipboardList, FileInput, History } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { CurrencyInput } from '../../components/CurrencyInput';
import { RecordSection } from '../../components/RecordDetailLayout';
import { WorkspaceTabs } from '../../components/WorkspaceTabs';
import { useToast } from '../../components/ToastProvider';
import { get, post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { formatCurrencyInputValue, resolveCurrencyFractionDigits } from '../../lib/utils/currencyInput';
import { AccountingHandoffDetailDialog } from './AccountingHandoffDetailDialog';
import { EvidenceReferenceViewer } from '../_shared/EvidenceReferenceViewer';
import { ProtectedDocumentViewerDialog } from '../_shared/EvidenceRecordViewerDialog';
import { ConstructionEvidenceCapture, type ConstructionEvidenceUploadDraft } from '../_shared/EvidenceCapture';
import { uploadWorkItemEvidenceDrafts } from '../_shared/evidenceUploadApi';
import { type AccountingOperations, type AccountingBasis, reconciliationLabel, eligibleMISATransactions, misaCSVHeader } from './accountingOperations';

const inputClass = 'w-full rounded border border-line bg-white px-2 py-2 text-sm';
const buttonClass = 'rounded border border-line px-3 py-2 text-sm disabled:opacity-40';
const eventLabels: Record<string, string> = {
    accounting_basis_confirmed: 'Xác nhận căn cứ',
    accounting_misa_import_confirmed: 'Nhập file MISA',
    accounting_misa_allocated: 'Ghép giao dịch',
    accounting_misa_allocation_revoked: 'Hủy ghép giao dịch',
    accounting_misa_reviewed: 'Xác nhận đối soát',
};
type SourceContext = { source_type: string; source_id: string };

function Field({ label, children, error }: { label: string; children: ReactNode; error?: boolean }) {
    return <label className={`block space-y-1 text-sm ${error ? 'rounded border border-red-500 p-1 text-red-700' : ''}`}><span>{label}</span>{children}{error && <span className="block text-xs">Cần bổ sung hoặc sửa trường này.</span>}</label>;
}
function Money({ value, currency }: { value: string; currency: string }) {
    const formatted = formatCurrencyInputValue(value, { allowNegative: true, maximumFractionDigits: resolveCurrencyFractionDigits(currency) }).formattedValue;
    return <span>{formatted} {currency}</span>;
}

export function AccountingOperationsButton({ projectId, basisId, source, onClose }: { projectId: string; basisId?: string; source?: SourceContext; onClose?: () => void }) {
    const [open, setOpen] = useState(false);
    const { hasPermission } = useAuth();
    if (!hasPermission('accounting_reconciliation.view')) return null;
    return <><button className={buttonClass} onClick={() => setOpen(true)}>Đối soát MISA</button>{open && <AccountingOperationsDialog key={projectId} projectId={projectId} basisId={basisId} source={source} onClose={() => { setOpen(false); onClose?.(); }}/>}</>;
}

export function AccountingOperationsDialog({ projectId, basisId, source, onClose }: { projectId: string; basisId?: string; source?: SourceContext; onClose: () => void }) {
    const { user } = useAuth();
    const qc = useQueryClient();
    const toast = useToast();
    const base = `/projects/${projectId}`;
    const query = useQuery({ queryKey: ['accounting-operations', user?.id, projectId], queryFn: () => get<AccountingOperations>(`${base}/accounting-operations`), staleTime: 0, gcTime: 0, refetchOnMount: 'always', retry: false });
    const [tab, setTab] = useState<'bases' | 'misa' | 'history'>('bases');
    const [selected, setSelected] = useState(basisId ?? '');
    const [filter, setFilter] = useState('all');
    const [form, setForm] = useState<'basis' | 'import' | null>(null);
    const [reason, setReason] = useState('');
    const [pending, setPending] = useState(false);
    const [failure, setFailure] = useState('');
    const [evidence, setEvidence] = useState<string | null>(null);
    const [file, setFile] = useState<{ id: string; filename: string } | null>(null);
    const [recordId, setRecordId] = useState('');
    const [amount, setAmount] = useState('');
    const [note, setNote] = useState('');
    const [revoke, setRevoke] = useState('');
    const [allocationKey, setAllocationKey] = useState(() => crypto.randomUUID());
    const [errors, setErrors] = useState<string[]>([]);
    const data = query.isFetchedAfterMount && !query.isFetching && !query.isError ? query.data : undefined;
    const current = data?.bases.find(b => selected ? b.id === selected : !!source && b.source_type === source.source_type && b.source_id === source.source_id);
    const visible = data?.bases.filter(b => (!source || b.source_type === source.source_type && b.source_id === source.source_id)
        && (filter === 'all' || (filter === 'reviewed' ? b.reviewed : filter === 'partial' ? Number(b.difference_amount) !== 0 : !b.reviewed))) ?? [];
    async function run(path: string, body: unknown, success: () => void = () => {}) {
        if (path === 'misa-allocations') body = { ...(body as object), id: allocationKey };
        setPending(true); setFailure('');
        try {
            await post(`${base}/${path}`, body);
            success();
            if (path === 'misa-allocations') setAllocationKey(crypto.randomUUID());
            await qc.invalidateQueries({ queryKey: ['accounting-operations'] });
            toast.success('Đã cập nhật đối soát MISA.');
            return true;
        } catch (e) {
            setFailure(e instanceof Error ? e.message : 'Không lưu được dữ liệu');
            toast.error('Không cập nhật được đối soát', e);
            await query.refetch();
            return false;
        } finally { setPending(false); }
    }
    function allocate() {
        if (!current || !data || current.source_available === false) return;
        const record = eligibleMISATransactions(data.records, current).find(m => m.id === recordId);
        const invalid = [...(!record ? ['record'] : []), ...(!Number.isFinite(Number(amount)) || !(Number(amount) > 0) || record && Number(amount) > Number(record.available_amount) ? ['amount'] : [])];
        setErrors(invalid);
        if (invalid.length) { toast.warn('Chọn giao dịch phù hợp và số tiền ghép lớn hơn 0, không vượt số tiền giao dịch còn lại.'); return; }
        void run('misa-allocations', { basis_id: current.id, record_id: recordId, amount }, () => { setAmount(''); setRecordId(''); });
    }
    function review() {
        if (!current || current.source_available === false) return;
        setErrors(note.trim() ? [] : ['note']);
        if (!note.trim()) { toast.warn('Bổ sung ghi chú đối soát được đánh dấu.'); return; }
        void run('misa-reviews', { basis_id: current.id, note, fingerprint: current.fingerprint }, () => setNote(''));
    }
    const canReconcile = data?.capabilities['accounting_misa.reconcile'];
    return <Dialog open title="Đối soát MISA" size="workspace" mobileFullscreen manageFocus onClose={() => { if (!pending && !form && !revoke) onClose(); }}>
        <p className="mb-3 rounded bg-blue-50 p-2 text-xs">Đối chiếu hồ sơ với dữ liệu MISA đã nhập. Chênh lệch là kết quả so sánh, không xác nhận tình trạng thanh toán hoặc chặn công việc công trình.</p>
        <WorkspaceTabs tabs={[{ id: 'bases', label: 'Hồ sơ đối soát', icon: ClipboardList }, { id: 'misa', label: 'Kết quả MISA', icon: FileInput }, { id: 'history', label: 'Lịch sử', icon: History }]} activeTab={tab} onChange={setTab} ariaLabel="Hồ sơ và đối soát"/>
        {query.isError ? <p role="alert">Không tải được hồ sơ hoặc không có quyền. <button onClick={() => void query.refetch()}>Thử lại</button></p> : !data ? <p role="status">Đang tải hồ sơ…</p> : <div className="space-y-3 py-3">
            {failure && <p role="alert" className="text-red-700">{failure}</p>}
            <p className="font-semibold">{data.project.code} · {data.project.name}</p>
            {tab === 'bases' && <>
                <WorkspaceTabs tabs={[{ id: 'all', label: 'Tất cả', icon: ClipboardList }, { id: 'pending', label: 'Chờ đối soát', icon: ClipboardList }, { id: 'partial', label: 'Có chênh lệch', icon: ClipboardList }, { id: 'reviewed', label: 'Đã đối soát', icon: History }]} activeTab={filter} onChange={setFilter} ariaLabel="Lọc hồ sơ đối soát"/>
                <div className="flex flex-wrap gap-2">{data.capabilities['accounting_reconciliation.manage'] && <button className={buttonClass} onClick={() => setForm('basis')}>Xác nhận căn cứ</button>}{canReconcile && <button className={buttonClass} onClick={() => setForm('import')}>Nhập file MISA</button>}</div>
                {!visible.length && <p>{data.bases.length ? 'Không có hồ sơ phù hợp với bộ lọc.' : 'Chưa có căn cứ. Xác nhận căn cứ từ hồ sơ công trình để đối soát.'}</p>}
                <div className="grid gap-2 md:grid-cols-2">{visible.map(b => <div key={b.id} className={`space-y-2 rounded border p-3 text-sm ${current?.id === b.id ? 'border-blue-500 bg-blue-50' : 'border-line'}`}>
                    <strong>{b.snapshot.code} · {b.snapshot.title}</strong><p>{b.partner_name} · {b.direction === 'outbound' ? 'Chi' : 'Thu'}</p>
                    <p>Giá trị căn cứ: <Money value={b.amount} currency={b.currency}/></p><p>Đã ghép: <Money value={b.allocated_amount} currency={b.currency}/> · Chênh lệch: <Money value={b.difference_amount} currency={b.currency}/></p>
                    <p>{reconciliationLabel(b)}</p><SourceLinks basis={b} projectId={projectId}/>
                    <button className={buttonClass} disabled={pending} onClick={() => { setSelected(b.id); setRecordId(''); setAmount(''); setNote(''); setErrors([]); }}>Chọn đối chiếu</button>
                </div>)}</div>
                {current && <section className="space-y-3 rounded border border-line p-3">
                    <h3 className="font-semibold">{current.snapshot.code} · {current.snapshot.title}</h3>
                    <p className="text-sm">Chênh lệch = đã ghép − giá trị căn cứ. Số dương: ghép vượt căn cứ; số âm: ghép ít hơn căn cứ.</p>
                    <p className="text-sm">{current.note}</p><Files ids={current.evidence_ids ?? []} data={data} open={setEvidence}/>
                    {current.source_available === false && <p role="alert" className="rounded bg-amber-50 p-2 text-sm text-amber-900">{current.source_unavailable_reason || 'Hồ sơ nguồn không còn đủ điều kiện đối soát.'}</p>}
                    {canReconcile && current.source_available !== false && <div className="grid gap-3 rounded bg-slate-50 p-3 md:grid-cols-2">
                        <Field label="Giao dịch MISA cùng đối tác, tiền tệ và chiều" error={errors.includes('record')}><select className={inputClass} disabled={pending} value={recordId} onChange={e => { setRecordId(e.target.value); setErrors(x => x.filter(k => k !== 'record')); }}><option value="">Chọn giao dịch</option>{eligibleMISATransactions(data.records, current).map(m => <option value={m.id} key={m.id}>{m.external_id} · {m.record_date.slice(0, 10)} · còn {m.available_amount} {m.currency}</option>)}</select></Field>
                        <Field label="Số tiền ghép" error={errors.includes('amount')}><CurrencyInput currencyCode={current.currency} value={amount} disabled={pending} invalid={errors.includes('amount')} onValueChange={v => { setAmount(v.numericString); setErrors(x => x.filter(k => k !== 'amount')); }}/></Field>
                        <button className={buttonClass} disabled={pending} onClick={allocate}>Ghép với hồ sơ</button>
                        <Field label="Ghi chú đối soát *" error={errors.includes('note')}><textarea className={inputClass} disabled={pending} value={note} onChange={e => { setNote(e.target.value); setErrors(x => x.filter(k => k !== 'note')); }}/></Field>
                        <button className={buttonClass} disabled={pending} onClick={review}>Xác nhận đối soát</button>
                    </div>}
                    {!data.records.some(m => m.active && m.kind === 'transaction') && <p className="text-sm">Chưa nhập giao dịch MISA để đối chiếu thu/chi.</p>}
                    {data.allocations.filter(a => a.basis_id === current.id).map(a => <div key={a.id} className="flex flex-wrap items-center gap-2 border-t py-2 text-sm"><span>{a.external_id} · {a.record_date.slice(0, 10)} · <Money value={a.amount} currency={current.currency}/> · {a.active ? 'Đang ghép' : `Đã hủy ghép: ${a.revoke_reason}`}</span>{a.active && canReconcile && <button className={buttonClass} disabled={pending} onClick={() => { setRevoke(a.id); setReason(''); }}>Hủy ghép</button>}</div>)}
                </section>}
            </>}
            {tab === 'misa' && <>
                {canReconcile && <button className={buttonClass} onClick={() => setForm('import')}>Nhập file MISA</button>}
                <p className="text-xs">Số dư tại ngày chốt và giao dịch thu/chi được trình bày riêng. Dữ liệu lấy từ file nhập, không đồng bộ trực tiếp.</p>
                {(['transaction', 'balance'] as const).map(kind => <section key={kind}><h3 className="font-semibold">{kind === 'transaction' ? 'Giao dịch thu/chi đã nhập' : 'Số dư tại ngày chốt — không cộng với giao dịch'}</h3>{!data.records.some(m => m.kind === kind && m.active) && <p className="text-sm">Chưa có dữ liệu.</p>}<div className="grid gap-2 md:grid-cols-2">{data.records.filter(m => m.kind === kind && m.active).map(m => <div key={m.id} className="rounded border p-3 text-sm"><strong>{m.external_id} · {m.line_key}</strong><p>{m.partner_name} · {m.entity_code} · {m.record_date.slice(0, 10)}</p><p>{kind === 'balance' ? (m.direction === 'inbound' ? 'Số dư phải thu' : 'Số dư phải trả') : (m.direction === 'inbound' ? 'Thu' : 'Chi')}: <Money value={m.amount} currency={m.currency}/></p>{kind === 'transaction' && <p>Chưa ghép: <Money value={m.available_amount} currency={m.currency}/></p>}<details><summary>Mã dòng để đối chiếu/thay thế</summary><code className="break-all">{m.id}</code></details></div>)}</div></section>)}
            </>}
            {tab === 'history' && <>
                <h3 className="font-semibold">Đợt nhập MISA và file gốc</h3>{!data.batches.length && <p>Chưa nhập dữ liệu.</p>}{data.batches.map(b => <div key={b.id} className="flex flex-wrap items-center gap-2 border-b py-2 text-sm"><span>{b.filename} · {b.entity_code} · {b.created_at} · {b.status === 'confirmed' ? 'Đã nhập' : 'Chờ xác nhận'}</span><button className={buttonClass} onClick={() => setFile(b)}>Xem file gốc</button>{b.status === 'preview' && canReconcile && <button className={buttonClass} onClick={() => setForm('import')}>Mở form kiểm tra file</button>}</div>)}
                <h3 className="font-semibold">Lịch sử hồ sơ và ghép giao dịch</h3>{data.events.filter(e => !current || !e.basis_id || e.basis_id === current.id).map(e => {
                    const basis = data.bases.find(b => b.id === e.basis_id);
                    const snapshot = e.snapshot;
                    const evidenceIDs = Array.isArray(snapshot.evidence_ids) ? snapshot.evidence_ids.filter((id): id is string => typeof id === 'string') : [];
                    return <details key={e.id} className="rounded border p-2 text-sm"><summary>{eventLabels[e.action] ?? 'Cập nhật đối soát'} · {e.created_at}</summary>
                        {basis && <p>{basis.snapshot.code} · {basis.snapshot.title}</p>}
                        {typeof snapshot.note === 'string' && <p>{snapshot.note}</p>}{typeof snapshot.reason === 'string' && <p>{snapshot.reason}</p>}
                        {snapshot.amount != null && basis && <p><Money value={String(snapshot.amount)} currency={basis.currency}/></p>}
                        <Files ids={evidenceIDs} data={data} open={setEvidence}/>
                    </details>;
                })}
                <h3 className="font-semibold">Lịch sử đối soát</h3>{data.reviews.filter(r => !current || r.basis_id === current.id).map(r => <p key={r.id} className="text-sm">{r.created_at} · {r.note}</p>)}
            </>}
        </div>}
        {form && data && <AccountingOperationForm mode={form} data={data} source={source} projectId={projectId} onClose={() => setForm(null)} onSaved={async () => { setForm(null); await query.refetch(); }}/>}
        {revoke && <Dialog open title="Hủy ghép giao dịch" onClose={() => { if (!pending) setRevoke(''); }}><Field label="Lý do *" error={errors.includes('reason')}><textarea className={inputClass} disabled={pending} value={reason} onChange={e => { setReason(e.target.value); setErrors(x => x.filter(k => k !== 'reason')); }}/></Field><button className={buttonClass} disabled={pending} onClick={() => { if (!reason.trim()) { setErrors(['reason']); toast.warn('Bổ sung lý do hủy ghép.'); return; } void run(`misa-allocations/${revoke}/revoke`, { reason }, () => setRevoke('')); }}>Hủy ghép</button></Dialog>}
        <EvidenceReferenceViewer reference={evidence ? { id: evidence } : null} onClose={() => setEvidence(null)}/>
        {file && <ProtectedDocumentViewerDialog name={file.filename} mimeType="text/csv" protectedFilePath={`${base}/misa-imports/${file.id}/file`} sourceLabel="File gốc của đợt nhập MISA" onClose={() => setFile(null)}/>}
    </Dialog>;
}
function SourceLinks({ basis, projectId }: { basis: AccountingBasis; projectId: string }) {
    const [open, setOpen] = useState(false);
    return <RecordSection title="Hồ sơ nguồn"><p>{basis.snapshot.code} · {basis.snapshot.title}</p><button className={buttonClass} onClick={() => setOpen(true)}>Mở hồ sơ</button>{open && <AccountingHandoffDetailDialog target={{ project_id: projectId, source_type: basis.source_type, source_id: basis.source_id }} showReconciliation={false} onClose={() => setOpen(false)}/>}</RecordSection>;
}
function Files({ ids, data, open }: { ids: string[]; data: AccountingOperations; open: (id: string) => void }) {
    return <div className="flex flex-wrap gap-2">{ids.map(id => <button key={id} className={buttonClass} onClick={() => open(id)}>{data.evidence.find(e => e.id === id)?.name ?? 'Xem chứng từ đã liên kết'}</button>)}</div>;
}

function AccountingOperationForm({ mode, data, source, projectId, onClose, onSaved }: {
    mode: 'basis' | 'import';
    data: AccountingOperations;
    source?: {
        source_type: string;
        source_id: string;
    };
    projectId: string;
    onClose: () => void;
    onSaved: () => Promise<void>;
}) {
    const toast = useToast();
    const base = `/projects/${projectId}`;
    const [values, setValues] = useState<Record<string, string>>(() => ({ source_id: data.bases.some(b => b.source_id === source?.source_id && b.source_type === source?.source_type) ? '' : source?.source_id ?? '', note: '', amount: '', entity_code: '' }));
    const [ids, setIds] = useState<string[]>([]);
    const [errors, setErrors] = useState<string[]>([]);
    const [pending, setPending] = useState(false);
    const [failure, setFailure] = useState('');
    const [drafts, setDrafts] = useState<ConstructionEvidenceUploadDraft[]>([]);
    const [uploadTarget, setUploadTarget] = useState('');
    const [content, setContent] = useState('');
    const [filename, setFilename] = useState('');
    const [preview, setPreview] = useState<{
        id: string;
        status: string;
        rows: Record<string, string>[];
    } | null>(null);
    const selectedSource = data.sources.find(s => s.id === values.source_id);
    const currency = selectedSource?.currency ?? 'VND';
    function change(k: string, v: string) { setValues(x => ({ ...x, [k]: v })); setErrors(x => x.filter(e => e !== k)); if (k === 'entity_code')
        setPreview(null); }
    function check(required: string[]) {
        const missing = required.filter(k => !values[k]?.trim());
        if (mode === 'basis' && (!Number.isFinite(Number(values.amount)) || !(Number(values.amount) > 0)))
            missing.push('amount');
        if (mode === 'basis' && !ids.length && !drafts.length)
            missing.push('evidence_ids');
        if (drafts.length && !uploadTarget)
            missing.push('upload_target');
        setErrors(missing);
        if (missing.length) {
            toast.warn('Bổ sung hoặc sửa nguồn hồ sơ, giá trị, ghi chú và chứng từ được đánh dấu.');
            return false;
        }
        return true;
    }
    async function save() {
        if (!check(mode === 'basis' ? ['source_id', 'amount', 'note'] : ['entity_code']))
            return;
        if (mode === 'import' && !content) {
            setErrors(previous => [...previous, 'file']);
            toast.warn('Chọn file CSV trước khi kiểm tra.');
            return;
        }
        setPending(true);
        setFailure('');
        try {
            let savedIDs = ids;
            if (mode !== 'import' && drafts.length) {
                const files = await uploadWorkItemEvidenceDrafts(uploadTarget, drafts);
                savedIDs = Array.from(new Set([...ids, ...files.map(f => f.id)]));
                setIds(savedIDs);
            }
            if (mode === 'import') {
                if (preview) {
                    await post(`${base}/misa-imports/${preview.id}/confirm`);
                    toast.success('Đã nhập kết quả MISA.');
                    await onSaved();
                }
                else {
                    const p = await post<{
                        id: string;
                        status: string;
                        rows: Record<string, string>[];
                    }>(`${base}/misa-imports/preview`, { entity_code: values.entity_code, filename, content });
                    setPreview(p);
                    toast.success(p.status === 'confirmed' ? 'File này đã được nhập; không tạo trùng.' : 'Đã kiểm tra file; xem dữ liệu trước khi xác nhận.');
                }
            }
            else {
                await post(`${base}/accounting-bases`, { source_type: selectedSource?.source_type, source_id: values.source_id, amount: values.amount, note: values.note, evidence_ids: savedIDs });
                toast.success('Đã xác nhận căn cứ đối soát.');
                await onSaved();
            }
        }
        catch (e) {
            setFailure(e instanceof Error ? e.message : 'Không lưu được dữ liệu');
            toast.error('Không lưu được hồ sơ', e);
        }
        finally {
            setPending(false);
        }
    }
    const moneyField = (key: string, label: string) => <Field label={label} error={errors.includes(key)}><CurrencyInput currencyCode={currency} value={values[key]} disabled={pending} invalid={errors.includes(key)} onValueChange={v => change(key, v.numericString)}/></Field>;
    return <Dialog open title={mode === 'basis' ? 'Xác nhận căn cứ đối soát' : 'Nhập kết quả MISA'} size="xl" mobileFullscreen manageFocus onClose={() => { if (!pending)
        onClose(); }} footer={<div className="flex justify-end gap-2"><button className={buttonClass} disabled={pending} onClick={onClose}>Đóng</button><button className={buttonClass} disabled={pending || preview?.status === 'confirmed'} onClick={() => void save()}>{pending ? 'Đang xử lý…' : mode === 'import' ? (preview ? 'Xác nhận nhập' : 'Kiểm tra file') : 'Xác nhận căn cứ'}</button></div>}>
 <div className="space-y-3">{failure && <p role="alert" className="text-red-700">{failure}</p>}
 {mode !== 'import' && <details><summary>Bổ sung tệp vào Đầu việc nguồn</summary><Field label="Chọn đúng Đầu việc nhận tệp" error={errors.includes('upload_target')}><select className={inputClass} value={uploadTarget} disabled={pending || drafts.length > 0} onChange={e => setUploadTarget(e.target.value)}><option value="">Chọn Đầu việc</option>{data.upload_targets.map(w => <option key={w.id} value={w.id}>{w.title}</option>)}</select></Field><ConstructionEvidenceCapture drafts={drafts} onChange={setDrafts} hasSource={!!uploadTarget} disabled={pending || !uploadTarget} uploadScope={`/work-items/${uploadTarget}/evidence/upload`} maxFiles={Math.max(0, 30 - ids.length)} hint="Tệp được tải vào Đầu việc đã chọn khi lưu hồ sơ; quyền và trạng thái Đầu việc được kiểm tra tại máy chủ."/></details>}
 {mode === 'basis' && <><p className="text-xs">Xác nhận giá trị dự kiến để so sánh với dữ liệu MISA theo chứng từ nguồn. Mỗi nguồn có một căn cứ được xác nhận, không sửa đè.</p><Field label="Nguồn *" error={errors.includes('source_id')}><select className={inputClass} value={values.source_id} disabled={pending} onChange={e => change('source_id', e.target.value)}><option value="">Chọn nguồn có đối tác</option>{data.sources.filter(s => !data.bases.some(b => b.source_id === s.id && b.source_type === s.source_type)).map(s => <option key={s.id} value={s.id}>{s.code} · {s.title}</option>)}</select></Field>{selectedSource && <p className="text-sm">Giá trị tham chiếu hồ sơ nguồn: <Money value={selectedSource.reference_amount} currency={currency}/></p>}{moneyField('amount', `Giá trị căn cứ đã xác nhận (${currency}) *`)}</>}
 {mode !== 'import' && <><Field label={'Lý do/căn cứ xác nhận *'} error={errors.includes('note')}><textarea className={inputClass} value={values.note} onChange={e => change('note', e.target.value)}/></Field><fieldset className={`rounded border p-2 ${errors.includes('evidence_ids') ? 'border-red-500' : ''}`}><legend>Chứng từ có quyền xem *</legend>{!data.evidence.length && <p className="text-sm">Chưa có chứng từ. Bổ sung tệp tại hồ sơ nguồn rồi mở lại form.</p>}<div className="max-h-44 overflow-auto">{data.evidence.map(e => <label key={e.id} className="block text-sm"><input type="checkbox" checked={ids.includes(e.id)} onChange={v => setIds(v.target.checked ? [...ids, e.id] : ids.filter(id => id !== e.id))}/> {e.name}</label>)}</div></fieldset></>}
 {mode === 'import' && <>
 <p className="text-sm">Mỗi file thuộc công trình đang mở và một đơn vị kế toán. Dùng ID đối tác Consflow đã ánh xạ. Ngày YYYY-MM-DD; số tiền không dấu phân cách; kind: transaction hoặc balance; direction: inbound hoặc outbound. replaces_id chỉ điền khi thay thế dòng đã nhập.</p>
 <a className={buttonClass} download="misa-consflow-template.csv" href={`data:text/csv;charset=utf-8,${encodeURIComponent(misaCSVHeader + '\n')}`}>Tải mẫu CSV</a>
 <Field label="Mã đơn vị kế toán *" error={errors.includes('entity_code')}><input className={inputClass} value={values.entity_code} onChange={e => change('entity_code', e.target.value)}/></Field>
 <Field label="File CSV *" error={errors.includes('file')}><input type="file" accept=".csv,text/csv" disabled={pending} onChange={async (e) => { const f = e.target.files?.[0]; setPreview(null); setContent(''); setErrors(x => x.filter(k => k !== 'file')); if (!f)
            return; if (f.size > 2 * 1024 * 1024) {
            toast.warn('File tối đa 2 MB');
            return;
        } setFilename(f.name); setContent(await f.text()); }}/></Field>
 <details><summary>Mã đối tác của các căn cứ</summary>{Array.from(new Map(data.bases.map(b => [b.partner_id, b])).values()).map(b => <p key={b.partner_id} className="break-all text-xs">{b.partner_name}: {b.partner_id}</p>)}</details>
 {preview && <><p>{preview.status === 'confirmed' ? 'File đã nhập trước đó.' : `${preview.rows.length} dòng chờ xác nhận; kiểm tra công trình, đối tác, chiều thu/chi và ngày.`}</p><div className="max-h-80 space-y-2 overflow-auto">{preview.rows.map((r, i) => <div key={i} className="rounded border p-2 text-xs">{r.external_id} · {r.line_key} · {r.kind} · {r.direction} · {r.record_date}<p>{r.partner_id} · {r.amount} {r.currency}</p>{r.replaces_id && <p>Thay thế: {r.replaces_id}</p>}</div>)}</div></>}
 </>}
 </div></Dialog>;
}
