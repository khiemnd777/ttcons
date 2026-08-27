import { useEffect, useMemo, useState } from 'react';
import { useLocation, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, ArrowDownToLine, ArrowLeftRight, Boxes, ClipboardList, Eye, PackageCheck, Plus, Search, Trash2, Warehouse as WarehouseIcon } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import type { Partner, StockDocument, StockDocumentItem, StockPosition, WarehouseReceiptOption } from '../../lib/types/domain';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { useToast } from '../../components/ToastProvider';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { Dialog } from '../../components/Dialog';
import { StatCard } from '../../components/StatCard';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { RecordFormDialog } from '../_shared/RecordFormDialog';
import { ProjectSelector } from '../_shared/ProjectSelector';
import { EnumChip } from '../../components/EnumChip';
import { useAuth } from '../../lib/auth/AuthContext';
import { partnerOptions } from '../core-data/partnerUtils';
import {
  syncWorkItemMutationTransition,
  withWorkItemMutationContext,
  workItemAutoStartedNotice,
  type WorkItemMutationResult,
} from '../work-items/workItemMutationContext';

type WarehouseTab = 'stock' | 'documents' | 'site' | 'materials';
type RecordRow = Record<string, any>;

export function WarehousePage({ scoped = false }: { scoped?: boolean }) {
  const { id } = useParams();
  const location = useLocation();
  const initialParams = useMemo(() => new URLSearchParams(location.search), [location.search]);
  const taskId = initialParams.get('task')?.trim() ?? '';
  const { hasPermission } = useAuth();
  const [projectId, setProjectId] = useState(id ?? '');
  const [tab, setTab] = useState<WarehouseTab>(() => validTab(initialParams.get('tab')));
  const [query, setQuery] = useState('');
  const [warehouseFilter, setWarehouseFilter] = useState('all');
  const [categoryFilter, setCategoryFilter] = useState('all');
  const [stockState, setStockState] = useState('all');
  const [openMaterial, setOpenMaterial] = useState(false);
  const [openDocument, setOpenDocument] = useState(false);
  const [documentId, setDocumentId] = useState('');
  const queryClient = useQueryClient();
  const toast = useToast();
  const materials = useQuery({ queryKey: ['materials'], queryFn: () => get<RecordRow[]>('/materials') });
  const warehouses = useQuery({ queryKey: ['warehouses'], queryFn: () => get<RecordRow[]>('/warehouses') });
  const partners = useQuery({ queryKey: ['partners', 'supplier', 'active'], queryFn: () => get<Partner[]>('/partners?status=active') });
  const positions = useQuery({ queryKey: ['stock-positions'], queryFn: () => get<StockPosition[]>('/stock/positions') });
  const documentsPath = projectId ? `/stock/documents?project_id=${encodeURIComponent(projectId)}` : '/stock/documents';
  const documents = useQuery({ queryKey: ['stock-documents', projectId], queryFn: () => get<StockDocument[]>(documentsPath) });
  const documentDetail = useQuery({ queryKey: ['stock-document', documentId], queryFn: () => get<StockDocument>(`/stock/documents/${documentId}`), enabled: Boolean(documentId) });
  const balances = useQuery({ queryKey: ['material-balances', projectId], queryFn: () => get<RecordRow[]>(`/projects/${projectId}/material-balances`), enabled: Boolean(projectId) });
  const receiptOptions = useQuery({ queryKey: ['warehouse-receipt-options', projectId], queryFn: () => get<WarehouseReceiptOption[]>(`/projects/${projectId}/warehouse-receipt-options`), enabled: Boolean(projectId) });
  const createMaterial = useMutation({
    mutationFn: (values: RecordRow) => post('/materials', values),
    onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ['materials'] }); setOpenMaterial(false); toast.success('Đã thêm vật tư'); },
    onError: (error) => toast.error('Không thêm được vật tư', error),
  });
  const createDocument = useMutation({
    mutationFn: (values: RecordRow) => post<StockDocument & WorkItemMutationResult>('/stock/documents', withWorkItemMutationContext(values, taskId)),
    onSuccess: async (record) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['stock-positions'] }),
        queryClient.invalidateQueries({ queryKey: ['stock-documents'] }),
        queryClient.invalidateQueries({ queryKey: ['material-balances'] }),
        queryClient.invalidateQueries({ queryKey: ['warehouse-receipt-options'] }),
        queryClient.invalidateQueries({ queryKey: ['procurement'] }),
      ]);
      syncWorkItemMutationTransition(queryClient, record);
      setOpenDocument(false); setDocumentId(record.id); setTab('documents'); toast.success(`Đã ghi nhận phiếu kho ${record.document_code}.${workItemAutoStartedNotice(record)}`);
    },
    onError: (error) => toast.error('Không ghi nhận được phiếu kho', error),
  });

  useEffect(() => {
    if (initialParams.get('receipt')) { setTab('documents'); setOpenDocument(true); }
  }, [initialParams]);

  const positionRows = useMemo(() => (positions.data ?? []).filter((row) => {
    const searchText = `${row.code} ${row.name} ${row.category ?? ''} ${row.warehouse_name}`.toLocaleLowerCase('vi-VN');
    return (!query.trim() || searchText.includes(query.trim().toLocaleLowerCase('vi-VN')))
      && (warehouseFilter === 'all' || row.warehouse_id === warehouseFilter)
      && (categoryFilter === 'all' || row.category === categoryFilter)
      && (stockState === 'all' || (stockState === 'available' ? numeric(row.stock_quantity) > 0 : numeric(row.stock_quantity) <= 0));
  }), [categoryFilter, positions.data, query, stockState, warehouseFilter]);
  const documentRows = useMemo(() => (documents.data ?? []).filter((row) => !query.trim() || `${row.document_code} ${row.project_name ?? ''} ${row.source_warehouse_name ?? ''} ${row.target_warehouse_name ?? ''}`.toLocaleLowerCase('vi-VN').includes(query.trim().toLocaleLowerCase('vi-VN'))), [documents.data, query]);
  const balanceRows = useMemo(() => (balances.data ?? []).filter((row) => !query.trim() || `${row.material_name ?? ''} ${row.code ?? ''}`.toLocaleLowerCase('vi-VN').includes(query.trim().toLocaleLowerCase('vi-VN'))), [balances.data, query]);
  const materialRows = useMemo(() => (materials.data ?? []).filter((row) => !query.trim() || `${row.code ?? ''} ${row.name ?? ''} ${row.category ?? ''}`.toLocaleLowerCase('vi-VN').includes(query.trim().toLocaleLowerCase('vi-VN'))), [materials.data, query]);
  const categories = useMemo(() => Array.from(new Set((materials.data ?? []).map((row) => String(row.category ?? '')).filter(Boolean))).sort((a, b) => a.localeCompare(b, 'vi')), [materials.data]);
  const waitingReceiptCount = new Set((receiptOptions.data ?? []).map((row) => row.incoming_acceptance_id)).size;
  const zeroCount = (positions.data ?? []).filter((row) => numeric(row.stock_quantity) <= 0).length;
  const pendingSurplus = (balances.data ?? []).filter((row) => ['pending', 'partial'].includes(row.surplus_status) && numeric(row.surplus_quantity) > 0).length;
  const receiptParam = initialParams.get('receipt') ?? '';
  const canCreateAnyDocument = ['warehouse.receive', 'warehouse.issue', 'warehouse.transfer', 'warehouse.surplus_handle'].some(hasPermission);

  return <>
    <PageHeader
      title="Kho vật tư"
      description="Tồn theo kho, phiếu nhập xuất, vật tư tại công trình và xử lý vật tư dư."
      actions={<>
        {scoped ? <ProjectBackLink projectId={projectId} /> : null}
        {!scoped ? <ProjectSelector value={projectId} onChange={setProjectId} /> : null}
        {canCreateAnyDocument ? <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpenDocument(true)}><Plus className="h-4 w-4" />Tạo phiếu kho</button> : null}
      </>}
    />
    <div className="mb-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <StatCard label="Chờ nhập từ nghiệm thu" value={waitingReceiptCount} icon={<PackageCheck className="h-4 w-4" />} />
      <StatCard label="Vị trí hết tồn" value={zeroCount} icon={<AlertTriangle className="h-4 w-4" />} />
      <StatCard label="Vị trí kho đang theo dõi" value={(positions.data ?? []).length} icon={<WarehouseIcon className="h-4 w-4" />} />
      <StatCard label="Vật tư dư chưa xử lý" value={pendingSurplus} icon={<Boxes className="h-4 w-4" />} />
    </div>
    <div className="mb-4 rounded-lg border border-line bg-white p-3 shadow-panel">
      <div className="flex flex-wrap gap-2 border-b border-line pb-3">
        <TabButton active={tab === 'stock'} onClick={() => setTab('stock')} icon={<WarehouseIcon className="h-4 w-4" />}>Tồn theo kho</TabButton>
        <TabButton active={tab === 'documents'} onClick={() => setTab('documents')} icon={<ClipboardList className="h-4 w-4" />}>Phiếu kho</TabButton>
        <TabButton active={tab === 'site'} onClick={() => setTab('site')} icon={<Boxes className="h-4 w-4" />}>Tại công trình & dư</TabButton>
        <TabButton active={tab === 'materials'} onClick={() => setTab('materials')} icon={<PackageCheck className="h-4 w-4" />}>Danh mục vật tư</TabButton>
      </div>
      <div className="grid gap-2 pt-3 lg:grid-cols-[minmax(240px,1fr)_180px_180px_160px]">
        <label className="relative"><Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" /><input className="w-full rounded-md border border-line py-2 pl-9 pr-3 text-sm" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Tìm vật tư, kho hoặc mã phiếu..." /></label>
        {tab === 'stock' ? <select className="rounded-md border border-line bg-white px-3 py-2 text-sm" value={warehouseFilter} onChange={(event) => setWarehouseFilter(event.target.value)}><option value="all">Tất cả kho</option>{(warehouses.data ?? []).map((row) => <option key={row.id} value={row.id}>{row.name}</option>)}</select> : <span />}
        {tab === 'stock' ? <select className="rounded-md border border-line bg-white px-3 py-2 text-sm" value={categoryFilter} onChange={(event) => setCategoryFilter(event.target.value)}><option value="all">Tất cả nhóm</option>{categories.map((category) => <option key={category} value={category}>{category}</option>)}</select> : <span />}
        {tab === 'stock' ? <select className="rounded-md border border-line bg-white px-3 py-2 text-sm" value={stockState} onChange={(event) => setStockState(event.target.value)}><option value="all">Tất cả tồn</option><option value="available">Có tồn</option><option value="zero">Hết tồn</option></select> : tab === 'materials' && hasPermission('warehouse.adjust') ? <button className="inline-flex items-center justify-center gap-2 rounded-md border border-blue-200 px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={() => setOpenMaterial(true)}><Plus className="h-4 w-4" />Thêm vật tư</button> : <span />}
      </div>
    </div>

    {tab === 'stock' ? <StockTab rows={positionRows} loading={positions.isLoading} error={positions.error} onRetry={() => positions.refetch()} /> : null}
    {tab === 'documents' ? <DocumentsTab rows={documentRows} loading={documents.isLoading} error={documents.error} onRetry={() => documents.refetch()} onView={setDocumentId} /> : null}
    {tab === 'site' ? <SiteTab rows={balanceRows} loading={balances.isLoading} error={balances.error} onRetry={() => balances.refetch()} projectId={projectId} /> : null}
    {tab === 'materials' ? <MaterialsTab rows={materialRows} loading={materials.isLoading} error={materials.error} onRetry={() => materials.refetch()} onCreate={hasPermission('warehouse.adjust') ? () => setOpenMaterial(true) : undefined} /> : null}

    <RecordFormDialog open={openMaterial} title="Thêm vật tư" submitLabel="Thêm vật tư" onClose={() => setOpenMaterial(false)} fields={[
      { name: 'code', label: 'Mã vật tư', defaultValue: `MAT-${Date.now().toString().slice(-4)}`, required: true },
      { name: 'name', label: 'Tên vật tư', defaultValue: '', required: true },
      { name: 'unit', label: 'Đơn vị', defaultValue: 'cái', required: true },
      { name: 'category', label: 'Nhóm', defaultValue: 'Phụ kiện', required: true },
      { name: 'default_supplier_partner_id', label: 'Nhà cung cấp tham khảo', type: 'select', placeholder: 'Chọn từ Danh mục đối tác', helperText: 'Dùng làm nguồn cung ban đầu; có thể thay đổi khi lấy báo giá.', options: partnerOptions(partners.data ?? [], ['supplier', 'service_provider', 'other']) },
      { name: 'default_supplier', label: 'Tên nhà cung cấp khác', placeholder: 'Nhập tên nhà cung cấp', helperText: 'Chỉ nhập khi nhà cung cấp chưa có trong Danh mục đối tác.', hiddenWhen: (values: RecordRow) => Boolean(values.default_supplier_partner_id) },
    ]} onSubmit={(values) => createMaterial.mutateAsync(values)} />

    <StockDocumentDialog open={openDocument} projectId={projectId} receiptId={receiptParam} materials={materials.data ?? []} warehouses={warehouses.data ?? []} receiptOptions={receiptOptions.data ?? []} hasPermission={hasPermission} pending={createDocument.isPending} onClose={() => setOpenDocument(false)} onSubmit={(values) => createDocument.mutateAsync(values)} />
    <Dialog open={Boolean(documentId)} title="Chi tiết phiếu kho" description={documentDetail.data ? `${documentDetail.data.document_code} · ${movementLabel(documentDetail.data.movement_type)}` : undefined} onClose={() => setDocumentId('')} size="xl">
      {documentDetail.isLoading ? <p className="py-6 text-sm text-slate-500">Đang tải phiếu kho...</p> : documentDetail.error ? <p className="rounded-md border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700">Không tải được phiếu kho.</p> : documentDetail.data ? <StockDocumentDetail record={documentDetail.data} /> : null}
    </Dialog>
  </>;
}

function StockTab({ rows, loading, error, onRetry }: { rows: StockPosition[]; loading: boolean; error: unknown; onRetry: () => void }) {
  return <><div className="hidden md:block"><DataTable rows={rows} loading={loading} error={error} onRetry={onRetry} pageSize={15} defaultSort={{ key: 'warehouse_name' }} columns={[
    { key: 'warehouse_name', header: 'Kho', sortable: true, render: (row) => <div><span className="font-semibold text-ink">{row.warehouse_name}</span><div className="text-xs text-slate-500">{row.location || row.warehouse_type}</div></div> },
    { key: 'code', header: 'Mã vật tư', sortable: true },
    { key: 'name', header: 'Vật tư', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.name}</span> },
    { key: 'category', header: 'Nhóm', sortable: true },
    { key: 'stock_quantity', header: 'Tồn sổ', align: 'right', sortable: true, sortValue: (row) => numeric(row.stock_quantity), render: (row) => <span className={numeric(row.stock_quantity) <= 0 ? 'font-semibold text-rose-700' : 'font-semibold text-ink'}>{formatQuantity(row.stock_quantity)}</span> },
    { key: 'unit', header: 'ĐVT' },
  ]} empty="Chưa có biến động kho phù hợp bộ lọc" /></div><div className="space-y-3 md:hidden">{rows.map((row) => <div key={`${row.warehouse_id}-${row.material_id}`} className="rounded-lg border border-line bg-white p-4 shadow-panel"><div className="flex justify-between gap-3"><div><p className="text-xs font-semibold uppercase text-slate-500">{row.warehouse_name}</p><p className="font-semibold text-ink">{row.name}</p><p className="text-xs text-slate-500">{row.code} · {row.category}</p></div><p className={numeric(row.stock_quantity) <= 0 ? 'font-semibold text-rose-700' : 'font-semibold text-ink'}>{formatQuantity(row.stock_quantity)} {row.unit}</p></div></div>)}</div></>;
}

function DocumentsTab({ rows, loading, error, onRetry, onView }: { rows: StockDocument[]; loading: boolean; error: unknown; onRetry: () => void; onView: (id: string) => void }) {
  return <><div className="hidden md:block"><DataTable rows={rows} loading={loading} error={error} onRetry={onRetry} pageSize={15} defaultSort={{ key: 'document_date', direction: 'desc' }} columns={[
    { key: 'document_code', header: 'Mã phiếu', sortable: true, render: (row) => <button className="font-semibold text-blue-700 hover:underline" onClick={() => onView(row.id)}>{row.document_code}</button> },
    { key: 'movement_type', header: 'Loại', sortable: true, render: (row) => <EnumChip group="movement_type" value={row.movement_type} /> },
    { key: 'warehouse', header: 'Kho / Công trình', render: (row) => <div><span className="font-semibold text-ink">{warehouseDirection(row)}</span><div className="text-xs text-slate-500">{row.project_name || 'Không gắn công trình'}</div></div> },
    { key: 'reference', header: 'Chứng từ nguồn', render: (row) => row.reference_type ? `${sourceLabel(row.reference_type)} · ${shortId(row.reference_id)}` : 'Nhập thủ công' },
    { key: 'item_count', header: 'Số dòng', align: 'right', sortable: true },
    { key: 'document_date', header: 'Ngày phiếu', sortable: true, render: (row) => formatDate(row.document_date) },
    { key: 'created_by_name', header: 'Người ghi nhận', render: (row) => identityReferenceText(row.created_by_name, row.created_by_account_deleted) },
    { key: 'actions', header: 'Thao tác', align: 'right', render: (row) => <button title="Xem phiếu" className="rounded-md border border-line p-1.5 hover:bg-slate-100" onClick={() => onView(row.id)}><Eye className="h-4 w-4" /></button> },
  ]} empty="Chưa có phiếu kho" /></div><div className="space-y-3 md:hidden">{rows.map((row) => <button key={row.id} className="w-full rounded-lg border border-line bg-white p-4 text-left shadow-panel" onClick={() => onView(row.id)}><div className="flex justify-between gap-3"><div><p className="font-semibold text-blue-700">{row.document_code}</p><p className="text-sm font-semibold text-ink">{movementLabel(row.movement_type)}</p></div><span className="text-xs text-slate-500">{formatDate(row.document_date)}</span></div><p className="mt-2 text-sm text-slate-600">{warehouseDirection(row)}</p><p className="text-xs text-slate-500">{row.item_count ?? 0} dòng · {identityReferenceText(row.created_by_name, row.created_by_account_deleted, 'Không rõ người lập')}</p></button>)}</div></>;
}

function SiteTab({ rows, loading, error, onRetry, projectId }: { rows: RecordRow[]; loading: boolean; error: unknown; onRetry: () => void; projectId: string }) { return <DataTable rows={rows} loading={loading} error={error} onRetry={onRetry} pageSize={15} defaultSort={{ key: 'surplus_quantity', direction: 'desc' }} columns={[
  { key: 'material_name', header: 'Vật tư', sortable: true, render: (row) => <div><span className="font-semibold text-ink">{row.material_name}</span><div className="text-xs text-slate-500">{row.code} · {row.category}</div></div> },
  { key: 'planned_quantity', header: 'Kế hoạch', align: 'right', sortable: true, sortValue: (row) => numeric(row.planned_quantity), render: (row) => formatQuantity(row.planned_quantity) },
  { key: 'issued_quantity', header: 'Đã xuất site', align: 'right', sortable: true, sortValue: (row) => numeric(row.issued_quantity), render: (row) => formatQuantity(row.issued_quantity) },
  { key: 'used_quantity', header: 'Đã dùng', align: 'right', sortable: true, sortValue: (row) => numeric(row.used_quantity), render: (row) => formatQuantity(row.used_quantity) },
  { key: 'surplus_quantity', header: 'Dư', align: 'right', sortable: true, sortValue: (row) => numeric(row.surplus_quantity), render: (row) => <span className={numeric(row.surplus_quantity) > 0 ? 'font-semibold text-amber-700' : ''}>{formatQuantity(row.surplus_quantity)}</span> },
  { key: 'unit', header: 'ĐVT' },
  { key: 'surplus_status', header: 'Xử lý', sortable: true, render: (row) => <EnumChip group="surplus_status" value={row.surplus_status} /> },
]} empty={projectId ? 'Chưa có vật tư tại công trình' : 'Chọn công trình để xem tồn tại site'} />; }

function MaterialsTab({ rows, loading, error, onRetry, onCreate }: { rows: RecordRow[]; loading: boolean; error: unknown; onRetry: () => void; onCreate?: () => void }) { return <DataTable rows={rows} loading={loading} error={error} onRetry={onRetry} pageSize={15} defaultSort={{ key: 'name' }} columns={[
  { key: 'code', header: 'Mã', sortable: true }, { key: 'name', header: 'Vật tư', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.name}</span> }, { key: 'category', header: 'Nhóm', sortable: true }, { key: 'unit', header: 'ĐVT' }, { key: 'default_supplier_partner_name', header: 'Nhà cung cấp tham khảo' },
]} empty="Chưa có vật tư hoạt động" emptyAction={onCreate ? <button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={onCreate}>Thêm vật tư</button> : undefined} />; }

type DraftLine = { key: string; material_id: string; quantity: number; unit: string; reason: string; source_line_type?: string; source_line_id?: string; max_quantity?: number };

function StockDocumentDialog({ open, projectId, receiptId, materials, warehouses, receiptOptions, hasPermission, pending, onClose, onSubmit }: { open: boolean; projectId: string; receiptId: string; materials: RecordRow[]; warehouses: RecordRow[]; receiptOptions: WarehouseReceiptOption[]; hasPermission: (permission: string) => boolean; pending: boolean; onClose: () => void; onSubmit: (values: RecordRow) => Promise<unknown> }) {
  const allowedTypes = useMemo(() => [
    hasPermission('warehouse.receive') ? { value: 'receive', label: 'Nhập kho' } : null,
    hasPermission('warehouse.issue') ? { value: 'issue', label: 'Xuất công trình' } : null,
    hasPermission('warehouse.transfer') ? { value: 'transfer', label: 'Điều chuyển' } : null,
    hasPermission('warehouse.surplus_handle') ? { value: 'return_to_warehouse', label: 'Lưu kho vật tư dư' } : null,
    hasPermission('warehouse.surplus_handle') ? { value: 'return_to_subcontractor', label: 'Trả nhà cung cấp / thầu phụ' } : null,
  ].filter(Boolean) as { value: string; label: string }[], [hasPermission]);
  const [movementType, setMovementType] = useState('receive');
  const [sourceWarehouseId, setSourceWarehouseId] = useState('');
  const [targetWarehouseId, setTargetWarehouseId] = useState('');
  const [sourceAcceptanceId, setSourceAcceptanceId] = useState('');
  const [documentDate, setDocumentDate] = useState(new Date().toISOString().slice(0, 10));
  const [note, setNote] = useState('');
  const [lines, setLines] = useState<DraftLine[]>([emptyLine()]);
  const [warning, setWarning] = useState('');
  const acceptanceGroups = useMemo(() => groupReceiptOptions(receiptOptions), [receiptOptions]);

  useEffect(() => { if (open && allowedTypes.length && !allowedTypes.some((item) => item.value === movementType)) setMovementType(allowedTypes[0].value); }, [allowedTypes, movementType, open]);
  useEffect(() => {
    if (!open) return;
    const requested = receiptId && acceptanceGroups.has(receiptId) ? receiptId : '';
    if (requested) selectAcceptance(requested);
  }, [acceptanceGroups, open, receiptId]);

  function selectAcceptance(id: string) {
    setSourceAcceptanceId(id);
    const options = acceptanceGroups.get(id) ?? [];
    if (options[0]?.warehouse_id) setTargetWarehouseId(options[0].warehouse_id);
    setLines(options.map((option) => ({ key: option.incoming_acceptance_item_id, material_id: option.material_id, quantity: numeric(option.remaining_quantity), unit: option.unit, reason: `Nhập theo ${option.acceptance_code}`, source_line_type: 'incoming_acceptance_item', source_line_id: option.incoming_acceptance_item_id, max_quantity: numeric(option.remaining_quantity) })));
  }

  function updateLine(index: number, patch: Partial<DraftLine>) { setLines((current) => current.map((line, itemIndex) => itemIndex === index ? { ...line, ...patch } : line)); }
  function selectMaterial(index: number, materialId: string) { const material = materials.find((row) => row.id === materialId); updateLine(index, { material_id: materialId, unit: material?.unit ?? '' }); }
  async function submit() {
    const cleanLines = lines.filter((line) => line.material_id && numeric(line.quantity) > 0);
    const needsProject = ['issue', 'return_to_warehouse', 'return_to_subcontractor'].includes(movementType);
    if (!cleanLines.length) return setWarning('Phiếu kho cần ít nhất một dòng vật tư có số lượng lớn hơn 0.');
    if (needsProject && !projectId) return setWarning('Vui lòng chọn công trình cho phiếu xuất hoặc hoàn trả.');
    if (['issue', 'transfer'].includes(movementType) && !sourceWarehouseId) return setWarning('Kho nguồn là bắt buộc.');
    if (['receive', 'return_to_warehouse', 'transfer'].includes(movementType) && !targetWarehouseId) return setWarning('Kho nhận là bắt buộc.');
    if (movementType === 'transfer' && sourceWarehouseId === targetWarehouseId) return setWarning('Kho đích phải khác kho nguồn.');
    const exceeded = cleanLines.find((line) => line.max_quantity !== undefined && numeric(line.quantity) > numeric(line.max_quantity) + 0.000001);
    if (exceeded) return setWarning('Số lượng nhập không được vượt khối lượng đạt nghiệm thu còn lại.');
    setWarning('');
    await onSubmit({ project_id: projectId || null, movement_type: movementType, source_warehouse_id: sourceWarehouseId || null, target_warehouse_id: targetWarehouseId || null, reference_type: sourceAcceptanceId ? 'incoming_acceptance' : null, reference_id: sourceAcceptanceId || null, document_date: documentDate, note, items: cleanLines });
  }

  return <Dialog open={open} title="Tạo phiếu kho" description="Ghi nhận chứng từ nhiều dòng; nghiệm thu đầu vào chỉ là nguồn, không tự động làm tăng tồn kho." onClose={onClose} size="xl">
    <div className="space-y-4">
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <Field label="Loại phiếu"><select className={controlClass} value={movementType} onChange={(event) => { setMovementType(event.target.value); setSourceAcceptanceId(''); setLines([emptyLine()]); }}>{allowedTypes.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></Field>
        <Field label="Ngày ghi nhận"><input className={controlClass} type="date" value={documentDate} onChange={(event) => setDocumentDate(event.target.value)} /></Field>
        {['issue', 'transfer', 'return_to_subcontractor'].includes(movementType) ? <Field label="Kho nguồn"><select className={controlClass} value={sourceWarehouseId} onChange={(event) => setSourceWarehouseId(event.target.value)}><option value="">Chọn kho nguồn</option>{warehouses.map((row) => <option key={row.id} value={row.id}>{row.name}</option>)}</select></Field> : null}
        {['receive', 'transfer', 'return_to_warehouse'].includes(movementType) ? <Field label="Kho nhận"><select className={controlClass} value={targetWarehouseId} onChange={(event) => setTargetWarehouseId(event.target.value)}><option value="">Chọn kho nhận</option>{warehouses.map((row) => <option key={row.id} value={row.id}>{row.name}</option>)}</select></Field> : null}
      </div>
      {movementType === 'receive' && projectId ? <Field label="Chứng từ nghiệm thu đầu vào (không bắt buộc)"><select className={controlClass} value={sourceAcceptanceId} onChange={(event) => event.target.value ? selectAcceptance(event.target.value) : (setSourceAcceptanceId(''), setLines([emptyLine()]))}><option value="">Nhập thủ công</option>{Array.from(acceptanceGroups.entries()).map(([id, rows]) => <option key={id} value={id}>{rows[0].acceptance_code} · {rows[0].request_code || rows[0].supplier_name || 'Nghiệm thu đầu vào'} · còn {rows.reduce((sum, row) => sum + numeric(row.remaining_quantity), 0).toLocaleString('vi-VN')}</option>)}</select></Field> : null}
      <div className="overflow-x-auto rounded-lg border border-line"><table className="min-w-[760px] w-full text-sm"><thead className="bg-slate-50 text-left text-xs uppercase text-slate-500"><tr><th className="w-12"><span className="sr-only">Thao tác</span></th><th className="px-3 py-2">Vật tư</th><th className="px-3 py-2 text-right">Tối đa</th><th className="px-3 py-2 text-right">Số lượng</th><th className="px-3 py-2">ĐVT</th><th className="px-3 py-2">Ghi chú dòng</th></tr></thead><tbody className="divide-y divide-line">{lines.map((line, index) => <tr key={line.key}><td className="px-3 py-2"><button disabled={lines.length === 1 || Boolean(sourceAcceptanceId)} className="rounded-md p-2 text-slate-500 hover:bg-rose-50 hover:text-rose-700 disabled:opacity-30" onClick={() => setLines((current) => current.filter((_, itemIndex) => itemIndex !== index))}><Trash2 className="h-4 w-4" /></button></td><td className="px-3 py-2"><select className={controlClass} disabled={Boolean(line.source_line_id)} value={line.material_id} onChange={(event) => selectMaterial(index, event.target.value)}><option value="">Chọn vật tư</option>{materials.map((row) => <option key={row.id} value={row.id}>{row.code} · {row.name}</option>)}</select></td><td className="px-3 py-2 text-right text-slate-500">{line.max_quantity === undefined ? '—' : formatQuantity(line.max_quantity)}</td><td className="px-3 py-2"><input className={`${controlClass} text-right`} type="number" min="0.001" step="0.001" max={line.max_quantity} value={line.quantity} onChange={(event) => updateLine(index, { quantity: numeric(event.target.value) })} /></td><td className="px-3 py-2"><input className={controlClass} value={line.unit} readOnly /></td><td className="px-3 py-2"><input className={controlClass} value={line.reason} onChange={(event) => updateLine(index, { reason: event.target.value })} /></td></tr>)}</tbody></table></div>
      {!sourceAcceptanceId ? <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-50" onClick={() => setLines((current) => [...current, emptyLine()])}><Plus className="h-4 w-4" />Thêm dòng</button> : null}
      <Field label="Ghi chú phiếu"><textarea className={controlClass} rows={2} value={note} onChange={(event) => setNote(event.target.value)} /></Field>
      {warning ? <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm font-medium text-amber-800">{warning}</div> : null}
      <div className="flex justify-end gap-2 border-t border-line pt-4"><button className="rounded-md border border-line px-4 py-2 text-sm font-semibold hover:bg-slate-50" onClick={onClose}>Hủy</button><button disabled={pending} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" onClick={submit}><ArrowDownToLine className="h-4 w-4" />{pending ? 'Đang ghi nhận...' : 'Ghi nhận phiếu'}</button></div>
    </div>
  </Dialog>;
}

function StockDocumentDetail({ record }: { record: StockDocument }) { return <div className="space-y-4"><div className="grid gap-3 rounded-lg border border-line bg-slate-50 p-3 sm:grid-cols-2 lg:grid-cols-4"><DetailMetric label="Loại phiếu" value={movementLabel(record.movement_type)} /><DetailMetric label="Luồng kho" value={warehouseDirection(record)} /><DetailMetric label="Công trình" value={record.project_name || 'Không gắn công trình'} /><DetailMetric label="Ngày phiếu" value={formatDate(record.document_date)} /><DetailMetric label="Chứng từ nguồn" value={record.reference_type ? `${sourceLabel(record.reference_type)} · ${shortId(record.reference_id)}` : 'Nhập thủ công'} /><DetailMetric label="Người ghi nhận" value={identityReferenceText(record.created_by_name, record.created_by_account_deleted)} /><DetailMetric label="Ghi chú" value={record.note || '—'} /></div><div className="overflow-x-auto rounded-lg border border-line"><table className="min-w-[720px] w-full text-sm"><thead className="bg-slate-50 text-left text-xs uppercase text-slate-500"><tr><th className="px-3 py-2">Vật tư</th><th className="px-3 py-2 text-right">Số lượng</th><th className="px-3 py-2">ĐVT</th><th className="px-3 py-2">Mã biến động</th><th className="px-3 py-2">Ghi chú</th></tr></thead><tbody className="divide-y divide-line">{(record.items ?? []).map((item: StockDocumentItem) => <tr key={item.id}><td className="px-3 py-2"><span className="font-semibold text-ink">{item.material_name}</span><div className="text-xs text-slate-500">{item.material_code}</div></td><td className="px-3 py-2 text-right font-semibold">{formatQuantity(item.quantity)}</td><td className="px-3 py-2">{item.unit}</td><td className="px-3 py-2">{item.movement_code || '—'}</td><td className="px-3 py-2">{item.reason || '—'}</td></tr>)}</tbody></table></div></div>; }

const controlClass = 'w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100 disabled:bg-slate-50';
function Field({ label, children }: { label: string; children: React.ReactNode }) { return <label className="block"><span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</span>{children}</label>; }
function DetailMetric({ label, value }: { label: string; value: React.ReactNode }) { return <div><p className="text-xs font-semibold uppercase text-slate-500">{label}</p><div className="mt-1 text-sm font-semibold text-ink">{value}</div></div>; }
function TabButton({ active, onClick, icon, children }: { active: boolean; onClick: () => void; icon: React.ReactNode; children: React.ReactNode }) { return <button className={`inline-flex items-center gap-2 rounded-md border px-3 py-2 text-sm font-semibold ${active ? 'border-blue-600 bg-blue-600 text-white' : 'border-line bg-white text-slate-600 hover:bg-slate-50'}`} onClick={onClick}>{icon}{children}</button>; }
function emptyLine(): DraftLine { return { key: `${Date.now()}-${Math.random()}`, material_id: '', quantity: 1, unit: '', reason: '' }; }
function groupReceiptOptions(rows: WarehouseReceiptOption[]) { const groups = new Map<string, WarehouseReceiptOption[]>(); rows.forEach((row) => groups.set(row.incoming_acceptance_id, [...(groups.get(row.incoming_acceptance_id) ?? []), row])); return groups; }
function validTab(value: string | null): WarehouseTab { return value === 'documents' || value === 'site' || value === 'materials' ? value : 'stock'; }
function numeric(value: unknown) { return Number(value ?? 0); }
function formatQuantity(value: unknown) { return numeric(value).toLocaleString('vi-VN', { maximumFractionDigits: 3 }); }
function formatDate(value: unknown) { const text = String(value ?? '').slice(0, 10); if (!text) return '—'; const date = new Date(`${text}T00:00:00`); return Number.isNaN(date.getTime()) ? text : date.toLocaleDateString('vi-VN'); }
function movementLabel(value: string) { return ({ receive: 'Nhập kho', issue: 'Xuất công trình', transfer: 'Điều chuyển', return_to_warehouse: 'Lưu kho vật tư dư', return_to_subcontractor: 'Trả nhà cung cấp / thầu phụ' } as Record<string, string>)[value] ?? value; }
function warehouseDirection(row: StockDocument) { if (row.movement_type === 'transfer') return `${row.source_warehouse_name || 'Kho nguồn'} → ${row.target_warehouse_name || 'Kho đích'}`; if (['receive', 'return_to_warehouse'].includes(row.movement_type)) return row.target_warehouse_name || 'Kho nhận'; return row.source_warehouse_name || 'Kho xuất'; }
function sourceLabel(value: string) { return value === 'incoming_acceptance' ? 'Nghiệm thu đầu vào' : value; }
function shortId(value?: string) { return value ? value.slice(0, 8) : '—'; }
