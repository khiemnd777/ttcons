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
import { RecordLink } from '../_shared/RecordLinks';
import { EvidenceReferenceViewer } from '../_shared/EvidenceReferenceViewer';
import { ProtectedDocumentViewerDialog } from '../_shared/EvidenceRecordViewerDialog';
import { ConstructionEvidenceCapture, type ConstructionEvidenceUploadDraft } from '../_shared/EvidenceCapture';
import { uploadWorkItemEvidenceDrafts } from '../_shared/evidenceUploadApi';
import { type AccountingOperations, type AccountingRequest, type AccountingBasis, requestStatuses, requestKinds, reconciliationLabel, eligibleMISATransactions, misaCSVHeader, proposalMoneyInput } from './accountingOperations';
const inputClass = 'w-full rounded border border-line bg-white px-2 py-2 text-sm';
const buttonClass = 'rounded border border-line px-3 py-2 text-sm disabled:opacity-40';
function Field({ label, children, error }: {
    label: string;
    children: ReactNode;
    error?: boolean;
}) { return <label className={`block space-y-1 text-sm ${error ? 'rounded border border-red-500 p-1 text-red-700' : ''}`}><span>{label}</span>{children}{error && <span className="block text-xs">Cần bổ sung hoặc sửa trường này.</span>}</label>; }
function Money({ value, currency }: {
    value: string;
    currency: string;
}) { return <span>{new Intl.NumberFormat('vi-VN', { style: 'currency', currency }).format(Number(value))}</span>; }
export function AccountingOperationsButton({ projectId, requestId, source, onClose }: {
    projectId: string;
    requestId?: string;
    source?: {
        source_type: string;
        source_id: string;
    };
    onClose?: () => void;
}) {
    const [open, setOpen] = useState(false);
    const { hasPermission } = useAuth();
    if (!hasPermission('accounting_request.view'))
        return null;
    return <><button className={buttonClass} onClick={() => setOpen(true)}>{requestId ? 'Xử lý đề nghị' : 'Đề nghị theo đợt & kết quả MISA'}</button>{open && <AccountingOperationsDialog key={projectId} projectId={projectId} requestId={requestId} source={source} onClose={() => { setOpen(false); onClose?.(); }}/>}</>;
}
export function AccountingOperationsDialog({ projectId, requestId, source, onClose }: {
    projectId: string;
    requestId?: string;
    source?: {
        source_type: string;
        source_id: string;
    };
    onClose: () => void;
}) {
    const { user } = useAuth();
    const qc = useQueryClient();
    const toast = useToast();
    const base = `/projects/${projectId}`;
    const query = useQuery({ queryKey: ['accounting-operations', user?.id, projectId], queryFn: () => get<AccountingOperations>(`${base}/accounting-operations`), staleTime: 0, gcTime: 0, refetchOnMount: 'always', retry: false });
    const [tab, setTab] = useState<'requests' | 'misa' | 'history'>('requests');
    const [selected, setSelected] = useState(requestId ?? '');
    const [filter, setFilter] = useState('all');
    const [form, setForm] = useState<'basis' | 'request' | 'import' | null>(null);
    const [edit, setEdit] = useState<AccountingRequest>();
    const [action, setAction] = useState<{
        request: AccountingRequest;
        action: string;
    } | null>(null);
    const [reason, setReason] = useState('');
    const [pending, setPending] = useState(false);
    const [failure, setFailure] = useState('');
    const [evidence, setEvidence] = useState<string | null>(null);
    const [file, setFile] = useState<{
        id: string;
        filename: string;
    } | null>(null);
    const [recordId, setRecordId] = useState('');
    const [amount, setAmount] = useState('');
    const [note, setNote] = useState('');
    const [revoke, setRevoke] = useState('');
    const [allocationKey, setAllocationKey] = useState(() => crypto.randomUUID());
    const data = query.isFetchedAfterMount && !query.isFetching && !query.isError ? query.data : undefined;
    const current = data?.requests.find(r => r.id === selected);
    async function run(path: string, body: unknown, success: () => void = () => { }) {
        if (path === 'misa-allocations')
            body = { ...(body as object), id: allocationKey };
        setPending(true);
        setFailure('');
        try {
            await post(`${base}/${path}`, body);
            success();
            if (path === 'misa-allocations')
                setAllocationKey(crypto.randomUUID());
            await Promise.all([qc.invalidateQueries({ queryKey: ['accounting-operations'] }), qc.invalidateQueries({ queryKey: ['accounting-request-queue'] })]);
            toast.success('Đã lưu hồ sơ kế toán.');
            return true;
        }
        catch (e) {
            setFailure(e instanceof Error ? e.message : 'Không lưu được hồ sơ');
            toast.error('Không cập nhật được hồ sơ', e);
            await query.refetch();
            return false;
        }
        finally {
            setPending(false);
        }
    }
    const visible = data?.requests.filter(r => (!source || data.bases.some(b => b.id === r.basis_id && b.source_type === source.source_type && b.source_id === source.source_id)) && (filter === 'all' || (filter === 'reviewed' ? r.reviewed : filter === 'partial' ? Number(r.allocated_amount) > 0 && Number(r.uncovered_amount) > 0 && !r.reviewed : !r.reviewed))) ?? [];
    return <Dialog open title="Đề nghị kế toán & đối soát MISA" size="workspace" mobileFullscreen manageFocus onClose={() => { if (!pending && !form && !action)
        onClose(); }}>
 <p className="mb-3 rounded bg-blue-50 p-2 text-xs">Đề nghị và tiếp nhận hồ sơ không xác nhận đã thu/chi. Kết quả MISA là dữ liệu từ file đã nhập, theo ngày chứng từ; không đồng bộ trực tiếp.</p>
 <WorkspaceTabs tabs={[{ id: 'requests', label: 'Đề nghị theo đợt', icon: ClipboardList }, { id: 'misa', label: 'Kết quả MISA', icon: FileInput }, { id: 'history', label: 'Lịch sử', icon: History }]} activeTab={tab} onChange={setTab} ariaLabel="Hồ sơ và đối soát"/>
 {query.isError ? <p role="alert">Không tải được hồ sơ hoặc không có quyền. <button onClick={() => void query.refetch()}>Thử lại</button></p> : !data ? <p role="status">Đang tải hồ sơ…</p> : <div className="space-y-3 py-3">
 {failure && <p role="alert" className="text-red-700">{failure}</p>}
 {data.project && <p className="font-semibold">{data.project.code} · {data.project.name}</p>}
 {tab === 'requests' && <>
 <WorkspaceTabs tabs={[{ id: 'all', label: 'Tất cả', icon: ClipboardList }, { id: 'pending', label: 'Cần xử lý', icon: ClipboardList }, { id: 'partial', label: 'Khớp một phần', icon: ClipboardList }, { id: 'reviewed', label: 'Đã đối soát', icon: History }]} activeTab={filter} onChange={setFilter} ariaLabel="Lọc đề nghị kế toán"/>
 <div className="flex flex-wrap gap-2">{data.capabilities['accounting_request.review'] && <button className={buttonClass} onClick={() => setForm('basis')}>Xác nhận căn cứ</button>}{data.capabilities['accounting_request.submit'] && <button className={buttonClass} onClick={() => { setEdit(undefined); setForm('request'); }}>Lập đề nghị</button>}</div>
 {!visible.length && <p>Chưa có đề nghị. Xác nhận căn cứ của nguồn trước khi lập đợt.</p>}
 <details><summary>Vật tư kho sẵn có — hồ sơ xuất cho công trình</summary><p className="text-xs">Các phiếu dưới đây chỉ ghi nhận cấp vật tư. Không tự tạo đề nghị chi hoặc giá trị công nợ.</p>{!data.stock_documents.length && <p className="text-sm">Chưa có phiếu xuất kho được phép xem.</p>}{data.stock_documents.map(s => <div key={s.id}><RecordLink target={{ kind: 'stock_document', id: s.id, projectId }}>{s.document_code}</RecordLink></div>)}</details>
 <div className="grid gap-2 md:grid-cols-2">{visible.map(r => <button key={r.id} className={`rounded border p-3 text-left text-sm ${selected === r.id ? 'border-blue-500 bg-blue-50' : 'border-line'}`} onClick={() => { setSelected(r.id); setRecordId(''); setAmount(''); setNote(''); }}><strong>{r.title}</strong><p>{r.partner_name} · {r.direction === 'outbound' ? 'Đề nghị chi' : 'Đề nghị thu'}</p><p><Money value={r.amount} currency={r.currency}/> · {requestStatuses[r.status]}</p><p>{reconciliationLabel(r)}</p></button>)}</div>
 {current && <section className="space-y-3 rounded border border-line p-3">
 <h3 className="font-semibold">{current.title}</h3><p className="text-sm">{requestKinds[current.kind]} · Hạn {current.due_date.slice(0, 10)} · {current.reason}</p>
 <p className="text-sm">Giá trị đợt: <Money value={current.gross_amount} currency={current.currency}/> · Thu hồi tạm ứng: <Money value={current.recovery_amount} currency={current.currency}/> · Giữ lại: <Money value={current.retention_amount} currency={current.currency}/></p>
 <p className="text-sm">Đề nghị: <Money value={current.amount} currency={current.currency}/> · Đã phân bổ: <Money value={current.allocated_amount} currency={current.currency}/> · Chưa được bao phủ bởi giao dịch đã nhập: <Money value={current.uncovered_amount} currency={current.currency}/></p>
 <SourceLinks basis={data.bases.find(b => b.id === current.basis_id)!} projectId={projectId}/>
 <Files ids={current.evidence_ids ?? []} data={data} open={setEvidence}/>
 {(current.stock_document_ids ?? []).map(id => <div key={id}><RecordLink target={{ kind: 'stock_document', id, projectId }}>{data.stock_documents.find(s => s.id === id)?.document_code ?? 'Phiếu xuất kho đã liên kết'}</RecordLink></div>)}
 <div className="flex flex-wrap gap-2">{data.capabilities['accounting_request.submit'] && ['draft', 'returned'].includes(current.status) && <><button className={buttonClass} disabled={pending} onClick={() => { setEdit(current); setForm('request'); }}>Sửa</button><button className={buttonClass} disabled={pending} onClick={() => { setReason(''); setAction({ request: current, action: 'submit' }); }}>Gửi Kế toán</button><button className={buttonClass} onClick={() => { setReason(''); setAction({ request: current, action: 'cancel' }); }}>Hủy đề nghị</button></>}{data.capabilities['accounting_request.review'] && current.status === 'submitted' && <><button className={buttonClass} onClick={() => { setReason(''); setAction({ request: current, action: 'acknowledge' }); }}>Tiếp nhận</button><button className={buttonClass} onClick={() => { setReason(''); setAction({ request: current, action: 'return' }); }}>Yêu cầu bổ sung</button></>}</div>
 {current.status === 'acknowledged' && data.capabilities['accounting_misa.reconcile'] && <div className="grid gap-3 rounded bg-slate-50 p-3 md:grid-cols-2">
 <Field label="Giao dịch MISA cùng đối tác, tiền tệ và hướng"><select className={inputClass} value={recordId} onChange={e => setRecordId(e.target.value)}><option value="">Chọn giao dịch</option>{eligibleMISATransactions(data.records, current).map(m => <option value={m.id} key={m.id}>{m.external_id} · {m.record_date.slice(0, 10)} · còn {m.available_amount} {m.currency}</option>)}</select></Field>
 <Field label="Số tiền phân bổ"><CurrencyInput currencyCode={current.currency} value={amount} onValueChange={v => setAmount(v.numericString)}/></Field>
 <button className={buttonClass} disabled={pending || !recordId || !amount} onClick={() => void run('misa-allocations', { request_id: current.id, record_id: recordId, amount }, () => { setAmount(''); setRecordId(''); })}>Lưu phân bổ</button>
 <Field label="Ghi chú đối soát *"><textarea className={inputClass} value={note} onChange={e => setNote(e.target.value)}/></Field><button className={buttonClass} disabled={pending || !note.trim()} onClick={() => void run('misa-reviews', { request_id: current.id, note, fingerprint: current.fingerprint }, () => setNote(''))}>Xác nhận đối soát dữ liệu hiện tại</button>
 </div>}
 {data.allocations.filter(a => a.request_id === current.id).map(a => <div key={a.id} className="flex flex-wrap items-center gap-2 border-t py-2 text-sm"><span>{a.external_id} · {a.record_date.slice(0, 10)} · <Money value={a.amount} currency={current.currency}/> · {a.active ? 'Đang phân bổ' : `Đã thu hồi: ${a.revoke_reason}`}</span>{a.active && data.capabilities['accounting_misa.reconcile'] && <button className={buttonClass} onClick={() => { setRevoke(a.id); setReason(''); }}>Thu hồi phân bổ</button>}</div>)}
 </section>}
 <details><summary>Căn cứ đã xác nhận ({data.bases.length})</summary>{data.bases.map(b => <div key={b.id} className="space-y-2 border-t py-2 text-sm"><SourceLinks basis={b} projectId={projectId}/><p><Money value={b.amount} currency={b.currency}/> · {b.note}</p><Files ids={b.evidence_ids ?? []} data={data} open={setEvidence}/></div>)}</details>
 </>}
 {tab === 'misa' && <>
 {data.capabilities['accounting_misa.reconcile'] && <button className={buttonClass} onClick={() => setForm('import')}>Nhập dữ liệu CSV</button>}
 <p className="text-xs">Số dư tại ngày chốt và giao dịch thu/chi được trình bày riêng. Chưa có dòng nhập không có nghĩa là chưa thanh toán.</p>
 {(['transaction', 'balance'] as const).map(kind => <section key={kind}><h3 className="font-semibold">{kind === 'transaction' ? 'Giao dịch thu/chi đã nhập' : 'Số dư công nợ tại ngày chốt — không cộng với giao dịch'}</h3>{!data.records.some(m => m.kind === kind && m.active) && <p className="text-sm">Chưa có dữ liệu.</p>}<div className="grid gap-2 md:grid-cols-2">{data.records.filter(m => m.kind === kind && m.active).map(m => <div key={m.id} className="rounded border p-3 text-sm"><strong>{m.external_id} · {m.line_key}</strong><p>{m.partner_name} · {m.entity_code} · {m.record_date.slice(0, 10)}</p><p>{kind === 'balance' ? (m.direction === 'inbound' ? 'Phải thu' : 'Phải trả') : (m.direction === 'inbound' ? 'Thu' : 'Chi')}: <Money value={m.amount} currency={m.currency}/></p>{kind === 'transaction' && <p>Chưa phân bổ: <Money value={m.available_amount} currency={m.currency}/></p>}<details><summary>Mã dòng để đối chiếu/thay thế</summary><code className="break-all">{m.id}</code></details></div>)}</div></section>)}
 </>}
 {tab === 'history' && <>
 <h3 className="font-semibold">Đợt nhập MISA và file gốc</h3>{!data.batches.length && <p>Chưa nhập dữ liệu.</p>}{data.batches.map(b => <div key={b.id} className="flex flex-wrap items-center gap-2 border-b py-2 text-sm"><span>{b.filename} · {b.entity_code} · {b.created_at} · {b.status === 'confirmed' ? 'Đã nhập' : 'Chờ xác nhận'}</span><button className={buttonClass} onClick={() => setFile(b)}>Xem file gốc</button>{b.status === 'preview' && data.capabilities['accounting_misa.reconcile'] && <button className={buttonClass} onClick={() => setForm('import')}>Mở form kiểm tra file</button>}</div>)}
 <h3 className="font-semibold">Phiên bản đề nghị</h3>{data.events.filter(e => !selected || e.request_id === selected).map(e => <details key={e.id} className="rounded border p-2 text-sm"><summary>{e.snapshot.title} · Phiên bản {e.revision} · {e.action} · {e.created_at}</summary><p>{requestStatuses[e.snapshot.status]} · {e.snapshot.note}</p><p><Money value={e.snapshot.amount} currency={e.snapshot.basis.currency}/></p><Files ids={e.snapshot.evidence_ids ?? []} data={data} open={setEvidence}/></details>)}
 <h3 className="font-semibold">Lịch sử đối soát</h3>{data.reviews.filter(r => !selected || r.request_id === selected).map(r => <p key={r.id} className="text-sm">{r.created_at} · {r.note}</p>)}
 </>}
 </div>}
 {form && data && <AccountingOperationForm mode={form} data={data} edit={edit} source={source} projectId={projectId} onClose={() => setForm(null)} onSaved={async () => { setForm(null); await query.refetch(); await qc.invalidateQueries({ queryKey: ['accounting-request-queue'] }); }}/>}
 {action && <Dialog open title={action.action === 'submit' ? 'Gửi đề nghị cho Kế toán' : action.action === 'acknowledge' ? 'Tiếp nhận hồ sơ' : action.action === 'return' ? 'Yêu cầu bổ sung' : 'Hủy đề nghị'} onClose={() => { if (!pending)
        setAction(null); }}><p className="mb-3">{action.request.title}</p><Field label="Ghi chú / lý do (bắt buộc khi trả hoặc hủy)"><textarea className={inputClass} value={reason} onChange={e => setReason(e.target.value)}/></Field><button className={buttonClass} disabled={pending || (['return', 'cancel'].includes(action.action) && !reason.trim())} onClick={() => void run(`accounting-requests/${action.request.id}/${action.action}`, { expected_revision: action.request.revision, reason }, () => setAction(null))}>Xác nhận</button></Dialog>}
 {revoke && <Dialog open title="Thu hồi phân bổ" onClose={() => { if (!pending)
        setRevoke(''); }}><Field label="Lý do *"><textarea className={inputClass} value={reason} onChange={e => setReason(e.target.value)}/></Field><button className={buttonClass} disabled={pending || !reason.trim()} onClick={() => void run(`misa-allocations/${revoke}/revoke`, { reason }, () => setRevoke(''))}>Thu hồi</button></Dialog>}
 <EvidenceReferenceViewer reference={evidence ? { id: evidence } : null} onClose={() => setEvidence(null)}/>
 {file && <ProtectedDocumentViewerDialog name={file.filename} mimeType="text/csv" protectedFilePath={`${base}/misa-imports/${file.id}/file`} sourceLabel="File gốc của đợt nhập MISA" onClose={() => setFile(null)}/>}
 </Dialog>;
}
function SourceLinks({ basis, projectId }: {
    basis: AccountingBasis;
    projectId: string;
}) { return <RecordSection title="Hồ sơ nguồn"><RecordLink target={{ kind: basis.source_type === 'contract_milestone' ? 'contract' : 'procurement_request', id: basis.source_type === 'contract_milestone' ? basis.snapshot.contract_id! : basis.source_id, projectId }}>{basis.snapshot.code} · {basis.snapshot.title}</RecordLink></RecordSection>; }
function Files({ ids, data, open }: {
    ids: string[];
    data: AccountingOperations;
    open: (id: string) => void;
}) { return <div className="flex flex-wrap gap-2">{ids.map(id => <button key={id} className={buttonClass} onClick={() => open(id)}>{data.evidence.find(e => e.id === id)?.name ?? 'Xem chứng từ đã liên kết'}</button>)}</div>; }
function AccountingOperationForm({ mode, data, edit, source, projectId, onClose, onSaved }: {
    mode: 'basis' | 'request' | 'import';
    data: AccountingOperations;
    edit?: AccountingRequest;
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
    const [values, setValues] = useState<Record<string, string>>(() => ({ id: edit?.id ?? crypto.randomUUID(), basis_id: edit?.basis_id ?? data.bases.find(b => b.source_id === source?.source_id && b.source_type === source?.source_type)?.id ?? '', source_id: source?.source_id ?? '', title: edit?.title ?? '', kind: edit?.kind ?? 'progress', due_date: edit?.due_date.slice(0, 10) ?? '', gross_amount: proposalMoneyInput(edit?.gross_amount ?? ''), recovery_amount: proposalMoneyInput(edit?.recovery_amount ?? '0'), retention_amount: proposalMoneyInput(edit?.retention_amount ?? '0'), advance_request_id: edit?.advance_request_id ?? '', release_request_id: edit?.release_request_id ?? '', note: edit?.note ?? '', amount: '', entity_code: '' }));
    const [ids, setIds] = useState<string[]>(edit?.evidence_ids ?? []);
    const [stock, setStock] = useState<string[]>(edit?.stock_document_ids ?? []);
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
    const basis = data.bases.find(b => b.id === values.basis_id);
    const currency = basis?.currency ?? selectedSource?.currency ?? 'VND';
    function change(k: string, v: string) { setValues(x => ({ ...x, ...(k === 'kind' ? { recovery_amount: '0', retention_amount: '0', advance_request_id: '', release_request_id: '' } : {}), [k]: v })); setErrors(x => x.filter(e => e !== k)); if (k === 'entity_code')
        setPreview(null); }
    function check(required: string[]) {
        const missing = required.filter(k => !values[k]?.trim());
        if (mode === 'basis' && !(Number(values.amount) > 0))
            missing.push('amount');
        if (mode === 'request') {
            if (!(Number(values.gross_amount) > Number(values.recovery_amount) + Number(values.retention_amount)))
                missing.push('gross_amount');
            if (Number(values.recovery_amount) < 0)
                missing.push('recovery_amount');
            if (Number(values.retention_amount) < 0)
                missing.push('retention_amount');
            if (Number(values.recovery_amount) > 0 && !values.advance_request_id)
                missing.push('advance_request_id');
            if (values.kind === 'retention_release' && !values.release_request_id)
                missing.push('release_request_id');
        }
        if (mode === 'basis' && !ids.length && !drafts.length)
            missing.push('evidence_ids');
        if (drafts.length && !uploadTarget)
            missing.push('upload_target');
        setErrors(missing);
        if (missing.length) {
            toast.warn('Bổ sung hoặc sửa các trường bắt buộc, số tiền và đợt tham chiếu được đánh dấu.');
            return false;
        }
        return true;
    }
    async function save() {
        if (!check(mode === 'basis' ? ['source_id', 'amount', 'note'] : mode === 'request' ? ['basis_id', 'title', 'due_date', 'gross_amount'] : ['entity_code']))
            return;
        if (mode === 'import' && !content) {
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
                await post(`${base}/${mode === 'basis' ? 'accounting-bases' : 'accounting-requests'}`, mode === 'basis' ? { source_type: selectedSource?.source_type, source_id: values.source_id, amount: values.amount, note: values.note, evidence_ids: savedIDs } : { ...values, evidence_ids: savedIDs, stock_document_ids: stock, expected_revision: edit?.revision ?? 0 });
                toast.success(mode === 'basis' ? 'Đã xác nhận căn cứ.' : 'Đã lưu nháp đề nghị.');
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
    const moneyField = (key: string, label: string) => <Field label={label} error={errors.includes(key)}><CurrencyInput currencyCode={currency} value={values[key]} invalid={errors.includes(key)} onValueChange={v => change(key, v.numericString)}/></Field>;
    return <Dialog open title={mode === 'basis' ? 'Xác nhận căn cứ đề nghị' : mode === 'request' ? 'Đề nghị theo đợt' : 'Nhập kết quả MISA'} size="xl" mobileFullscreen manageFocus onClose={() => { if (!pending)
        onClose(); }} footer={<div className="flex justify-end gap-2"><button className={buttonClass} disabled={pending} onClick={onClose}>Đóng</button><button className={buttonClass} disabled={pending || preview?.status === 'confirmed'} onClick={() => void save()}>{pending ? 'Đang xử lý…' : mode === 'import' ? (preview ? 'Xác nhận nhập' : 'Kiểm tra file') : mode === 'basis' ? 'Xác nhận căn cứ' : 'Lưu nháp'}</button></div>}>
 <div className="space-y-3">{failure && <p role="alert" className="text-red-700">{failure}</p>}
 {mode !== 'import' && <details><summary>Bổ sung tệp vào Đầu việc nguồn</summary><Field label="Chọn đúng Đầu việc nhận tệp" error={errors.includes('upload_target')}><select className={inputClass} value={uploadTarget} disabled={pending || drafts.length > 0} onChange={e => setUploadTarget(e.target.value)}><option value="">Chọn Đầu việc</option>{data.upload_targets.map(w => <option key={w.id} value={w.id}>{w.title}</option>)}</select></Field><ConstructionEvidenceCapture drafts={drafts} onChange={setDrafts} hasSource={!!uploadTarget} disabled={pending || !uploadTarget} uploadScope={`/work-items/${uploadTarget}/evidence/upload`} maxFiles={Math.max(0, 30 - ids.length)} hint="Tệp được tải vào Đầu việc đã chọn khi lưu hồ sơ; quyền và trạng thái Đầu việc được kiểm tra tại máy chủ."/></details>}
 {mode === 'basis' && <><p className="text-xs">Xác nhận giá trị đề nghị dựa trên chứng từ. Không phải ghi nhận công nợ. Mỗi nguồn có một căn cứ đã xác nhận, không sửa đè.</p><Field label="Nguồn *" error={errors.includes('source_id')}><select className={inputClass} value={values.source_id} onChange={e => change('source_id', e.target.value)}><option value="">Chọn nguồn có đối tác</option>{data.sources.filter(s => !data.bases.some(b => b.source_id === s.id && b.source_type === s.source_type)).map(s => <option key={s.id} value={s.id}>{s.code} · {s.title}</option>)}</select></Field>{moneyField('amount', `Giá trị căn cứ đã xác nhận (${currency}) *`)}</>}
 {mode === 'request' && <><Field label="Căn cứ *" error={errors.includes('basis_id')}><select className={inputClass} disabled={!!edit} value={values.basis_id} onChange={e => change('basis_id', e.target.value)}><option value="">Chọn căn cứ</option>{data.bases.map(b => <option key={b.id} value={b.id}>{b.snapshot.code} · {b.partner_name} · {b.amount} {b.currency}</option>)}</select></Field><Field label="Tên đợt *" error={errors.includes('title')}><input className={inputClass} value={values.title} onChange={e => change('title', e.target.value)}/></Field><div className="grid gap-3 sm:grid-cols-2"><Field label="Loại đề nghị"><select className={inputClass} value={values.kind} onChange={e => change('kind', e.target.value)}>{Object.entries(requestKinds).map(([k, v]) => <option key={k} value={k}>{v}</option>)}</select></Field><Field label="Hạn đề nghị *" error={errors.includes('due_date')}><input className={inputClass} type="date" value={values.due_date} onChange={e => change('due_date', e.target.value)}/></Field>{moneyField('gross_amount', `Giá trị đợt (${currency}) *`)}{moneyField('recovery_amount', 'Thu hồi tạm ứng')}{moneyField('retention_amount', 'Giữ lại')}<Field label="Đợt tạm ứng được thu hồi" error={errors.includes('advance_request_id')}><select className={inputClass} value={values.advance_request_id} onChange={e => change('advance_request_id', e.target.value)}><option value="">Không thu hồi</option>{data.requests.filter(r => r.basis_id === values.basis_id && r.kind === 'advance' && r.status === 'acknowledged').map(r => <option key={r.id} value={r.id}>{r.title}</option>)}</select></Field>{values.kind === 'retention_release' && <Field label="Đợt đã giữ lại *" error={errors.includes('release_request_id')}><select className={inputClass} value={values.release_request_id} onChange={e => change('release_request_id', e.target.value)}><option value="">Chọn đợt giữ lại</option>{data.requests.filter(r => r.basis_id === values.basis_id && r.status === 'acknowledged' && Number(r.retention_amount) > 0).map(r => <option key={r.id} value={r.id}>{r.title}</option>)}</select></Field>}</div><p className="text-sm">Số tiền đề nghị = giá trị đợt − thu hồi tạm ứng − giữ lại. {basis?.direction === 'inbound' ? 'Đề nghị thu' : 'Đề nghị chi'} theo đối tác của căn cứ.</p></>}
 {mode !== 'import' && <><Field label={mode === 'basis' ? 'Lý do/căn cứ xác nhận *' : 'Ghi chú'} error={errors.includes('note')}><textarea className={inputClass} value={values.note} onChange={e => change('note', e.target.value)}/></Field><fieldset className={`rounded border p-2 ${errors.includes('evidence_ids') ? 'border-red-500' : ''}`}><legend>Chứng từ có quyền xem {mode === 'basis' ? '*' : ''}</legend>{!data.evidence.length && <p className="text-sm">Chưa có chứng từ. Bổ sung tệp tại hồ sơ nguồn rồi mở lại form.</p>}<div className="max-h-44 overflow-auto">{data.evidence.map(e => <label key={e.id} className="block text-sm"><input type="checkbox" checked={ids.includes(e.id)} onChange={v => setIds(v.target.checked ? [...ids, e.id] : ids.filter(id => id !== e.id))}/> {e.name}</label>)}</div></fieldset>{mode === 'request' && <fieldset className="rounded border p-2"><legend>Phiếu xuất kho sẵn có — chỉ liên kết, không tạo chi tiền</legend>{!data.stock_documents.length && <p className="text-sm">Chưa có phiếu xuất kho được phép xem.</p>}{data.stock_documents.map(s => <label className="block text-sm" key={s.id}><input type="checkbox" checked={stock.includes(s.id)} onChange={e => setStock(e.target.checked ? [...stock, s.id] : stock.filter(id => id !== s.id))}/> {s.document_code}</label>)}</fieldset>}</>}
 {mode === 'import' && <>
 <p className="text-sm">Mỗi file thuộc công trình đang mở và một đơn vị kế toán. Dùng ID đối tác Consflow đã ánh xạ. Ngày YYYY-MM-DD; số tiền không dấu phân cách; kind: transaction hoặc balance; direction: inbound hoặc outbound. replaces_id chỉ điền khi thay thế dòng đã nhập.</p>
 <a className={buttonClass} download="misa-consflow-template.csv" href={`data:text/csv;charset=utf-8,${encodeURIComponent(misaCSVHeader + '\n')}`}>Tải mẫu CSV</a>
 <Field label="Mã đơn vị kế toán *" error={errors.includes('entity_code')}><input className={inputClass} value={values.entity_code} onChange={e => change('entity_code', e.target.value)}/></Field>
 <Field label="File CSV *"><input type="file" accept=".csv,text/csv" disabled={pending} onChange={async (e) => { const f = e.target.files?.[0]; setPreview(null); setContent(''); if (!f)
            return; if (f.size > 2 * 1024 * 1024) {
            toast.warn('File tối đa 2 MB');
            return;
        } setFilename(f.name); setContent(await f.text()); }}/></Field>
 <details><summary>Mã đối tác của các căn cứ</summary>{Array.from(new Map(data.bases.map(b => [b.partner_id, b])).values()).map(b => <p key={b.partner_id} className="break-all text-xs">{b.partner_name}: {b.partner_id}</p>)}</details>
 {preview && <><p>{preview.status === 'confirmed' ? 'File đã nhập trước đó.' : `${preview.rows.length} dòng chờ xác nhận; kiểm tra công trình, đối tác, chiều thu/chi và ngày.`}</p><div className="max-h-80 space-y-2 overflow-auto">{preview.rows.map((r, i) => <div key={i} className="rounded border p-2 text-xs">{r.external_id} · {r.line_key} · {r.kind} · {r.direction} · {r.record_date}<p>{r.partner_id} · {r.amount} {r.currency}</p>{r.replaces_id && <p>Thay thế: {r.replaces_id}</p>}</div>)}</div></>}
 </>}
 </div></Dialog>;
}
