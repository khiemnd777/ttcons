import { useEffect, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { Download, FileJson, FileSpreadsheet } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { downloadBlob } from '../../lib/api/client';
import { saveWorkflowDefinitionBlob } from './workflowDefinitionExcel';
import type { DataPackageFormat } from '../_shared/dataPackage';

export function WorkflowDefinitionExportDialog({ open, scope, selectedID, selectedName, onClose, onSuccess, onError }: {
  open: boolean;
  scope: 'form' | 'pipeline';
  selectedID: string;
  selectedName?: string;
  onClose: () => void;
  onSuccess: () => void;
  onError: (title: string, error: unknown) => void;
}) {
  const [includeDependencies, setIncludeDependencies] = useState(true);
  const [format, setFormat] = useState<DataPackageFormat>('xlsx');
  useEffect(() => {
    if (open) {
      setIncludeDependencies(true);
      setFormat('xlsx');
    }
  }, [open]);
  const exportMutation = useMutation({
    mutationFn: () => {
      const params = new URLSearchParams({ scope, ids: selectedID, include_dependencies: String(scope === 'pipeline' && includeDependencies), format });
      return downloadBlob(`/workflow-definitions/export?${params.toString()}`);
    },
    onSuccess: (blob) => {
      saveWorkflowDefinitionBlob(blob, scope === 'pipeline' && includeDependencies ? 'bundle' : scope, 'export', format);
      onSuccess();
      onClose();
    },
    onError: (error) => onError('Không xuất được định nghĩa workflow', error),
  });
  return (
    <Dialog
      open={open}
      title="Xuất dữ liệu định nghĩa workflow"
      description="Tạo gói di chuyển định nghĩa bằng package key, không dùng UUID nguồn làm quan hệ nội bộ."
      size="lg"
      onClose={onClose}
      footer={<div className="flex justify-end gap-2"><button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700" onClick={onClose} disabled={exportMutation.isPending}>Hủy</button><button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={!selectedID || exportMutation.isPending} onClick={() => exportMutation.mutate()}><Download className="h-4 w-4" />{exportMutation.isPending ? 'Đang tạo file…' : `Xuất ${format === 'xlsx' ? 'Excel' : 'JSON'}`}</button></div>}
    >
      <div className="space-y-4">
        <fieldset>
          <legend className="mb-2 text-sm font-semibold text-ink">Định dạng dữ liệu</legend>
          <div className="grid grid-cols-2 gap-2 sm:max-w-md">{(['xlsx', 'json'] as const).map((item) => <label key={item} className={`flex cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm font-semibold ${format === item ? 'border-blue-300 bg-blue-50 text-blue-800' : 'border-line bg-white text-slate-700'}`}><input type="radio" name="workflow-export-format" className="h-4 w-4 text-blue-600" checked={format === item} disabled={exportMutation.isPending} onChange={() => setFormat(item)} />{item === 'xlsx' ? 'Excel (.xlsx)' : 'JSON (.json)'}</label>)}</div>
        </fieldset>
        <div className="flex items-start gap-3 rounded-lg border border-blue-200 bg-blue-50 p-4 text-sm leading-6 text-blue-900">{format === 'xlsx' ? <FileSpreadsheet className="mt-0.5 h-5 w-5 shrink-0" /> : <FileJson className="mt-0.5 h-5 w-5 shrink-0" />}<p><strong>{selectedName || 'Định nghĩa đang chọn'}</strong> sẽ được xuất với topology, Form/Fields, provenance và catalog tham chiếu. Export không thay đổi dữ liệu.</p></div>
        {scope === 'pipeline' ? <label className="flex items-start gap-3 rounded-lg border border-line p-4"><input type="checkbox" className="mt-1 h-4 w-4 text-blue-600" checked={includeDependencies} onChange={(event) => setIncludeDependencies(event.target.checked)} /><span><span className="block text-sm font-semibold text-ink">Kèm đầy đủ Form/Fields dependency</span><span className="mt-1 block text-xs leading-5 text-slate-600">Khuyến nghị để bundle có thể import vào môi trường chưa có Form tương ứng.</span></span></label> : null}
        <p className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">File xuất giữ trạng thái nguồn trong provenance; khi import, hệ thống luôn tạo bản nháp mới và không tự phát hành.</p>
      </div>
    </Dialog>
  );
}
