import { useEffect, useMemo, useState } from 'react';
import { Dialog } from '../../components/Dialog';
import type { Partner } from '../../lib/types/domain';
import { currency } from '../../lib/utils/format';
import { enumOptions } from '../../lib/utils/status';
import { partnerOptions, vendorPartnerTypes } from '../core-data/partnerUtils';
import { SimpleForm, type FieldDef, type FormValues } from '../_shared/SimpleForm';

export type DebtCreateMode = 'payable' | 'receivable';

type ProjectOption = { id: string; name: string };

export function DebtCreateDialog({
  open,
  initialMode,
  projectId,
  projects,
  partners,
  canCreatePayable,
  canCreateReceivable,
  onClose,
  onCreatePayable,
  onCreateReceivable,
}: {
  open: boolean;
  initialMode: DebtCreateMode;
  projectId?: string;
  projects: ProjectOption[];
  partners: Partner[];
  canCreatePayable: boolean;
  canCreateReceivable: boolean;
  onClose: () => void;
  onCreatePayable: (values: FormValues) => Promise<unknown>;
  onCreateReceivable: (values: FormValues) => Promise<unknown>;
}) {
  const [mode, setMode] = useState<DebtCreateMode>(initialMode);

  useEffect(() => {
    if (open) setMode(initialMode);
  }, [initialMode, open]);

  const payableFields = useMemo<FieldDef[]>(() => [
    { name: 'project_id', label: projectId ? 'Công trình (đã khóa)' : 'Công trình', type: 'select', defaultValue: projectId ?? '', required: true, options: projects.map((project) => ({ label: project.name, value: project.id })), disabledWhen: () => Boolean(projectId) },
    { name: 'payable_code', label: 'Mã phải trả', defaultValue: `PAY-${Date.now().toString().slice(-5)}`, required: true },
    { name: 'vendor_type', label: 'Loại đối tác', type: 'select', defaultValue: 'supplier', required: true, options: enumOptions('vendor_type', ['supplier', 'subcontractor', 'labor_team', 'transport', 'equipment_rental', 'service_provider', 'other']) },
    { name: 'vendor_partner_id', label: 'Đối tác trong danh mục', type: 'select', options: partnerOptions(partners, vendorPartnerTypes), helperText: 'Nếu chọn từ danh mục, hệ thống sẽ chụp lại tên đối tác vào công nợ.' },
    { name: 'vendor_name', label: 'Tên đối tác nhập tay', required: true, hiddenWhen: (values) => Boolean(values.vendor_partner_id) },
    { name: 'due_date', label: 'Hạn thanh toán', type: 'date' },
    { name: 'gross_amount', label: 'Giá trị ghi nhận', type: 'currency', defaultValue: 10000000, required: true, min: 1 },
    { name: 'advance_amount', label: 'Tạm ứng đã khấu trừ', type: 'currency', defaultValue: 0, min: 0 },
    { name: 'retention_amount', label: 'Giữ lại theo dõi riêng', type: 'currency', defaultValue: 0, min: 0 },
    { name: 'deduction_amount', label: 'Khấu trừ khác', type: 'currency', defaultValue: 0, min: 0, validate: (value, values) => Number(values.advance_amount ?? 0) + Number(value ?? 0) > Number(values.gross_amount ?? 0) ? 'Tạm ứng và khấu trừ không được vượt giá trị ghi nhận' : undefined },
  ], [partners, projectId, projects]);

  const receivableFields = useMemo<FieldDef[]>(() => [
    { name: 'project_id', label: projectId ? 'Công trình (đã khóa)' : 'Công trình', type: 'select', defaultValue: projectId ?? '', required: true, options: projects.map((project) => ({ label: project.name, value: project.id })), disabledWhen: () => Boolean(projectId) },
    { name: 'receivable_code', label: 'Mã phải thu', defaultValue: `REC-${Date.now().toString().slice(-5)}`, required: true },
    { name: 'client_partner_id', label: 'Khách hàng trong danh mục', type: 'select', options: partnerOptions(partners, ['client']), helperText: 'Nếu chọn từ danh mục, hệ thống sẽ chụp lại tên khách hàng vào công nợ.' },
    { name: 'client_name', label: 'Tên khách hàng nhập tay', required: true, hiddenWhen: (values) => Boolean(values.client_partner_id) },
    { name: 'milestone_name', label: 'Đợt thanh toán', required: true },
    { name: 'due_date', label: 'Hạn thu', type: 'date' },
    { name: 'contract_amount', label: 'Giá trị hợp đồng', type: 'currency', min: 0 },
    { name: 'gross_amount', label: 'Giá trị ghi nhận', type: 'currency', defaultValue: 30000000, required: true, min: 1 },
  ], [partners, projectId, projects]);

  const selectedProject = (values: FormValues) => projects.find((project) => project.id === values.project_id)?.name ?? 'Chưa chọn';

  return (
    <Dialog
      open={open}
      title="Ghi nhận công nợ"
      description="Tạo nghĩa vụ ở trạng thái nháp; số đã thu/chi, trạng thái và số dư đối soát tiếp tục do hệ thống quản lý."
      onClose={onClose}
      size="xl"
    >
      <div className="mb-5 flex w-fit rounded-lg border border-line bg-slate-50 p-1">
        {canCreatePayable ? <ModeButton active={mode === 'payable'} label="Phải trả" onClick={() => setMode('payable')} /> : null}
        {canCreateReceivable ? <ModeButton active={mode === 'receivable'} label="Phải thu" onClick={() => setMode('receivable')} /> : null}
      </div>

      {mode === 'payable' ? (
        <SimpleForm
          key="payable-create"
          fields={payableFields}
          layout="two-column"
          sections={[
            { title: 'Thông tin nghĩa vụ', description: 'Xác định công trình, đối tác và thời hạn xử lý.', fields: ['project_id', 'payable_code', 'vendor_type', 'vendor_partner_id', 'vendor_name', 'due_date'] },
            { title: 'Giá trị', description: 'Giữ lại được theo dõi riêng; số dư mở trừ tạm ứng và khấu trừ.', fields: ['gross_amount', 'advance_amount', 'retention_amount', 'deduction_amount'] },
          ]}
          renderAside={(values) => <PayableSummary projectName={selectedProject(values)} values={values} />}
          submitLabel="Tạo phải trả nháp"
          onCancel={onClose}
          onSubmit={onCreatePayable}
        />
      ) : (
        <SimpleForm
          key="receivable-create"
          fields={receivableFields}
          layout="two-column"
          sections={[
            { title: 'Thông tin nghĩa vụ', description: 'Khóa khách hàng, đợt thanh toán và hạn thu.', fields: ['project_id', 'receivable_code', 'client_partner_id', 'client_name', 'milestone_name', 'due_date'] },
            { title: 'Giá trị', description: 'Hệ thống khởi tạo số đã thu bằng 0 và quản lý số dư còn lại.', fields: ['contract_amount', 'gross_amount'] },
          ]}
          renderAside={(values) => <ReceivableSummary projectName={selectedProject(values)} values={values} />}
          submitLabel="Tạo phải thu nháp"
          onCancel={onClose}
          onSubmit={onCreateReceivable}
        />
      )}
    </Dialog>
  );
}

function ModeButton({ active, label, onClick }: { active: boolean; label: string; onClick: () => void }) {
  return <button type="button" className={`rounded-md px-4 py-2 text-sm font-semibold ${active ? 'bg-white text-blue-700 shadow-sm' : 'text-slate-500 hover:text-ink'}`} onClick={onClick}>{label}</button>;
}

function PayableSummary({ projectName, values }: { projectName: string; values: FormValues }) {
  const gross = Number(values.gross_amount ?? 0);
  const advance = Number(values.advance_amount ?? 0);
  const deduction = Number(values.deduction_amount ?? 0);
  const opening = Math.max(gross - advance - deduction, 0);
  return <Summary title="Tóm tắt ghi nhận" projectName={projectName} rows={[
    ['Giá trị', currency(gross)],
    ['Tạm ứng', `− ${currency(advance)}`],
    ['Khấu trừ', `− ${currency(deduction)}`],
    ['Số dư mở', currency(opening)],
  ]} note={`Giữ lại ${currency(values.retention_amount)} được theo dõi riêng và chưa làm giảm số dư mở.`} />;
}

function ReceivableSummary({ projectName, values }: { projectName: string; values: FormValues }) {
  return <Summary title="Tóm tắt ghi nhận" projectName={projectName} rows={[
    ['Giá trị hợp đồng', currency(values.contract_amount)],
    ['Giá trị phải thu', currency(values.gross_amount)],
    ['Số dư mở', currency(values.gross_amount)],
  ]} note="Phiếu được tạo ở trạng thái nháp; số đã thu khởi tạo bằng 0." />;
}

function Summary({ title, projectName, rows, note }: { title: string; projectName: string; rows: Array<[string, string]>; note: string }) {
  return <div>
    <div className="text-xs font-semibold uppercase tracking-wide text-slate-500">{title}</div>
    <div className="mt-2 text-sm font-semibold text-ink">{projectName}</div>
    <div className="mt-4 space-y-2 text-sm">{rows.map(([label, value], index) => <div key={label} className={`flex items-center justify-between gap-3 ${index === rows.length - 1 ? 'border-t border-line pt-3 font-semibold text-ink' : 'text-slate-600'}`}><span>{label}</span><span>{value}</span></div>)}</div>
    <div className="mt-4 rounded-md border border-blue-100 bg-blue-50 p-3 text-xs leading-5 text-blue-800">{note}</div>
    <div className="mt-3 text-xs text-slate-500">Trạng thái sau tạo: <span className="font-semibold text-slate-700">Nháp</span></div>
  </div>;
}
