import { type ReactNode, useEffect, useMemo, useState } from 'react';
import { ArrowRight } from 'lucide-react';
import { Link } from 'react-router-dom';
import { Dialog } from '../../components/Dialog';
import { currency } from '../../lib/utils/format';
import { enumOptions } from '../../lib/utils/status';
import { SimpleForm, type FieldDef, type FormValues } from '../_shared/SimpleForm';

export type AccountingCreateMode = 'budget' | 'payment';

type ProjectOption = { id: string; name: string };
type AccountingOption = Record<string, any>;

export function AccountingCreateDialog({
  open,
  initialMode,
  projectId,
  projects,
	budgets,
	accounts,
  canCreateBudget,
  canCreatePayment,
  onClose,
  onCreateBudget,
  onCreatePayment,
}: {
  open: boolean;
  initialMode: AccountingCreateMode;
  projectId: string;
  projects: ProjectOption[];
	budgets: AccountingOption[];
	accounts: AccountingOption[];
  canCreateBudget: boolean;
  canCreatePayment: boolean;
  onClose: () => void;
  onCreateBudget: (values: FormValues) => Promise<unknown>;
  onCreatePayment: (values: FormValues) => Promise<unknown>;
}) {
  const [mode, setMode] = useState<AccountingCreateMode>(initialMode);

  useEffect(() => {
    if (open) setMode(initialMode);
  }, [initialMode, open]);

  const budgetFields = useMemo<FieldDef[]>(() => [
    { name: 'budget_code', label: 'Mã ngân sách', defaultValue: `BUD-${Date.now().toString().slice(-5)}`, required: true },
    { name: 'name', label: 'Tên ngân sách', defaultValue: 'Ngân sách công trình', required: true },
    { name: 'planned_amount', label: 'Giá trị kế hoạch', type: 'number', defaultValue: 100000000, required: true, min: 1 },
  ], []);

  const paymentFields = useMemo<FieldDef[]>(() => [
	{ name: 'project_id', label: 'Công trình', type: 'select', defaultValue: projectId, required: true, options: projects.map((project) => ({ label: project.name, value: project.id })), disabledWhen: () => true },
	{ name: 'payment_code', label: 'Số phiếu', defaultValue: `PC-${Date.now().toString().slice(-5)}`, required: true },
	{ name: 'payment_type', label: 'Loại chứng từ', type: 'select', defaultValue: 'outbound', required: true, options: enumOptions('payment_type', ['outbound', 'inbound']) },
	{ name: 'amount', label: 'Số tiền', type: 'number', defaultValue: 10000000, required: true, min: 1 },
	{ name: 'payment_method', label: 'Phương thức thanh toán', type: 'select', defaultValue: 'bank_transfer', required: true, options: enumOptions('payment_method', ['bank_transfer', 'cash']) },
	{ name: 'budget_id', label: 'Khoản mục ngân sách', type: 'select', defaultValue: budgets.find((budget) => budget.status === 'approved')?.id ?? '', required: true, hiddenWhen: (values) => values.payment_type !== 'outbound', options: budgets.filter((budget) => budget.status === 'approved').map((budget) => ({ label: `${budget.budget_code} · ${budget.name}`, value: budget.id })) },
	{ name: 'cash_account_id', label: 'Tài khoản tiền', type: 'select', defaultValue: accounts.find((account) => account.account_code === '1121')?.id ?? accounts[0]?.id ?? '', required: true, options: accounts.filter((account) => ['1111', '1121'].includes(account.account_code ?? '')).map((account) => ({ label: `${account.account_code} · ${account.name}`, value: account.id })) },
	], [accounts, budgets, projectId, projects]);

  const selectedProject = projects.find((project) => project.id === projectId)?.name ?? 'Chưa chọn công trình';

	return <Dialog open={open} title="Lập chứng từ kế toán" description="Lập ngân sách hoặc Phiếu thu/Phiếu chi khác. Chứng từ công nợ được lập tại sổ chi tiết phải trả/phải thu." onClose={onClose} size="xl">
    <div className="mb-5 flex w-fit rounded-lg border border-line bg-slate-50 p-1">
      {canCreateBudget ? <ModeButton active={mode === 'budget'} label="Ngân sách" onClick={() => setMode('budget')} /> : null}
	{canCreatePayment ? <ModeButton active={mode === 'payment'} label="Phiếu thu/Phiếu chi khác" onClick={() => setMode('payment')} /> : null}
    </div>
    {mode === 'budget' ? <SimpleForm
      key="budget-create"
      fields={budgetFields}
      layout="two-column"
      sections={[{ title: 'Ngân sách kế hoạch', description: 'Chỉ nhập dữ liệu sở hữu bởi người dùng; số duyệt, đã dùng và còn lại do hệ thống quản lý.', fields: ['budget_code', 'name', 'planned_amount'] }]}
      renderAside={(values) => <AccountingSummary title="Tóm tắt ngân sách" projectName={selectedProject} rows={[
        ['Giá trị kế hoạch', currency(values.planned_amount)],
        ['Đã duyệt', currency(0)],
        ['Đã dùng', currency(0)],
      ]} note="Sau khi tạo: Nháp. Khi duyệt, hệ thống lấy giá trị kế hoạch làm giá trị được duyệt." />}
      submitLabel="Tạo ngân sách nháp"
      onCancel={onClose}
      onSubmit={onCreateBudget}
    /> : <SimpleForm
      key="payment-create"
      fields={paymentFields}
      layout="two-column"
      sections={[
		{ title: 'Thông tin chứng từ', description: 'Phiếu này dành cho khoản thu/chi không qua công nợ. Số phiếu và loại chứng từ là căn cứ truy vết trên Sổ Nhật ký chung.', fields: ['project_id', 'payment_code', 'payment_type'] },
		{ title: 'Giá trị và định khoản', fields: ['amount', 'payment_method', 'budget_id', 'cash_account_id'] },
      ]}
      renderAside={(values) => <AccountingSummary title="Tóm tắt phiếu" projectName={selectedProject} rows={[
        ['Loại phiếu', paymentTypeLabel(values.payment_type)],
        ['Số tiền', currency(values.amount)],
		['Phương thức', values.payment_method === 'cash' ? 'Tiền mặt' : 'Chuyển khoản'],
		['Định khoản dự kiến', estimatedPosting(values, accounts)],
	  ]} note={<span>Chi trả nhà cung cấp hoặc thu tiền khách hàng phải lập tại <Link className="inline-flex items-center gap-1 font-semibold underline" to={`/projects/${projectId}/debt`}>Công nợ phải trả/phải thu<ArrowRight className="h-3 w-3" /></Link> để bảo toàn phân bổ.</span>} />}
	  submitLabel="Lập chứng từ nháp"
      onCancel={onClose}
      onSubmit={onCreatePayment}
    />}
  </Dialog>;
}

function ModeButton({ active, label, onClick }: { active: boolean; label: string; onClick: () => void }) {
  return <button type="button" className={`rounded-md px-4 py-2 text-sm font-semibold ${active ? 'bg-white text-blue-700 shadow-sm' : 'text-slate-500 hover:text-ink'}`} onClick={onClick}>{label}</button>;
}

function AccountingSummary({ title, projectName, rows, note }: { title: string; projectName: string; rows: Array<[string, string]>; note: ReactNode }) {
  return <div>
    <div className="text-xs font-semibold uppercase tracking-wide text-slate-500">{title}</div>
    <div className="mt-2 text-sm font-semibold text-ink">{projectName}</div>
    <div className="mt-4 space-y-2 text-sm">{rows.map(([label, value], index) => <div key={label} className={`flex items-center justify-between gap-3 ${index === 1 ? 'font-semibold text-ink' : 'text-slate-600'}`}><span>{label}</span><span>{value}</span></div>)}</div>
    <div className="mt-4 rounded-md border border-blue-100 bg-blue-50 p-3 text-xs leading-5 text-blue-800">{note}</div>
    <div className="mt-3 text-xs text-slate-500">Trạng thái sau tạo: <span className="font-semibold text-slate-700">Nháp</span></div>
  </div>;
}

function paymentTypeLabel(value: unknown) {
	return ({ outbound: 'Phiếu chi khác', inbound: 'Phiếu thu khác' } as Record<string, string>)[String(value)] ?? 'Chưa chọn';
}

function estimatedPosting(values: FormValues, accounts: AccountingOption[]) {
	const cash = accounts.find((account) => account.id === values.cash_account_id)?.account_code ?? (values.payment_method === 'cash' ? '1111' : '1121');
	return values.payment_type === 'inbound' ? `Nợ ${cash} / Có 711` : `Nợ 642 / Có ${cash}`;
}
