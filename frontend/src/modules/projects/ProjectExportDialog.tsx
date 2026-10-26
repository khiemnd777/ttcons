import { useEffect, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { Download, FileJson, FileSpreadsheet } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { downloadBlob } from '../../lib/api/client';
import { saveProjectExcelBlob } from './projectExcel';
import type { DataPackageFormat } from './projectExcel';

export type ProjectExcelExportFilters = {
  search: string;
  status: string;
  sort: string;
  direction: string;
};

export function ProjectExportDialog({ open, selectedProjectIDs, filteredCount, filters, onClose, onSuccess, onError }: {
  open: boolean;
  selectedProjectIDs: string[];
  filteredCount: number;
  filters: ProjectExcelExportFilters;
  onClose: () => void;
  onSuccess: (scope: 'filtered' | 'selected') => void;
  onError: (title: string, error: unknown) => void;
}) {
  const [scope, setScope] = useState<'filtered' | 'selected'>('filtered');
  const [format, setFormat] = useState<DataPackageFormat>('xlsx');

  useEffect(() => {
    if (open) {
      setScope(selectedProjectIDs.length ? 'selected' : 'filtered');
      setFormat('xlsx');
    }
  }, [open, selectedProjectIDs.length]);

  const exportMutation = useMutation({
    mutationFn: async () => {
      const params = new URLSearchParams({
        scope,
        search: filters.search,
        status: filters.status,
        sort: filters.sort,
        direction: filters.direction,
        format,
      });
      if (scope === 'selected') params.set('ids', selectedProjectIDs.join(','));
      return downloadBlob(`/projects/export?${params.toString()}`);
    },
    onSuccess: (blob) => {
      const now = new Date();
      const timestamp = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`;
      saveProjectExcelBlob(blob, `cong-trinh-${timestamp}.${format}`);
      onSuccess(scope);
      onClose();
    },
    onError: (error) => onError('Không xuất được công trình', error),
  });

  const selectedAvailable = selectedProjectIDs.length > 0 && selectedProjectIDs.length <= 100;
  const canExport = scope === 'selected' ? selectedAvailable : filteredCount <= 10000;

  return (
    <Dialog
      open={open}
      title="Xuất công trình"
      size="lg"
      onClose={onClose}
      footer={(
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-xs text-slate-500">Tối đa 10.000 dòng theo bộ lọc hoặc 100 công trình đã chọn</p>
          <div className="flex gap-2">
            <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-white disabled:opacity-50" onClick={onClose} disabled={exportMutation.isPending}>Hủy</button>
            <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" disabled={!canExport || exportMutation.isPending} onClick={() => exportMutation.mutate()}>
              <Download className="h-4 w-4" />{exportMutation.isPending ? 'Đang tạo file…' : `Xuất ${format === 'xlsx' ? 'Excel' : 'JSON'}`}
            </button>
          </div>
        </div>
      )}
    >
      <div className="space-y-4">
        <fieldset>
          <legend className="mb-2 text-sm font-semibold text-ink">Định dạng dữ liệu</legend>
          <div className="grid grid-cols-2 gap-2 sm:max-w-md">
            {(['xlsx', 'json'] as const).map((item) => <label key={item} className={`flex cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm font-semibold ${format === item ? 'border-blue-300 bg-blue-50 text-blue-800' : 'border-line bg-white text-slate-700'}`}><input type="radio" name="project-export-format" className="h-4 w-4 text-blue-600" checked={format === item} disabled={exportMutation.isPending} onChange={() => setFormat(item)} />{item === 'xlsx' ? 'Excel (.xlsx)' : 'JSON (.json)'}</label>)}
          </div>
        </fieldset>
        <div className="rounded-lg border border-blue-200 bg-blue-50 p-4 text-sm leading-6 text-blue-900">
          <div className="flex items-start gap-3">{format === 'xlsx' ? <FileSpreadsheet className="mt-0.5 h-5 w-5 shrink-0" /> : <FileJson className="mt-0.5 h-5 w-5 shrink-0" />}<p>{format === 'xlsx' ? 'Workbook gồm Cong_trinh, Thong_tin_xuat và đầy đủ sheet reference.' : 'JSON gồm projects, project_snapshots và đầy đủ Pipeline/Form/Field/Client references.'} Giá trị hợp đồng chỉ là snapshot đọc, không phải dữ liệu được phép import.</p></div>
        </div>
        <fieldset className="space-y-3">
          <legend className="text-sm font-semibold text-ink">Chọn phạm vi xuất</legend>
          <label className={`flex cursor-pointer items-start gap-3 rounded-lg border p-4 ${scope === 'filtered' ? 'border-blue-300 bg-blue-50/50' : 'border-line'}`}>
            <input type="radio" name="project-export-scope" className="mt-1 h-4 w-4 text-blue-600" checked={scope === 'filtered'} onChange={() => setScope('filtered')} />
            <span><span className="block text-sm font-semibold text-ink">Theo bộ lọc hiện tại</span><span className="mt-1 block text-xs leading-5 text-slate-600">{filteredCount.toLocaleString('vi-VN')} công trình khớp tìm kiếm, trạng thái và thứ tự sắp xếp hiện tại.</span></span>
          </label>
          <label className={`flex items-start gap-3 rounded-lg border p-4 ${selectedAvailable ? 'cursor-pointer' : 'cursor-not-allowed opacity-60'} ${scope === 'selected' ? 'border-blue-300 bg-blue-50/50' : 'border-line'}`}>
            <input type="radio" name="project-export-scope" className="mt-1 h-4 w-4 text-blue-600" checked={scope === 'selected'} disabled={!selectedAvailable} onChange={() => setScope('selected')} />
            <span><span className="block text-sm font-semibold text-ink">Công trình đã chọn</span><span className="mt-1 block text-xs leading-5 text-slate-600">{selectedProjectIDs.length} công trình trên trang hiện tại. Chọn tối đa 100 công trình.</span></span>
          </label>
        </fieldset>
        {scope === 'filtered' && filteredCount > 10000 ? <p className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">Kết quả vượt 10.000 công trình. Vui lòng thu hẹp bộ lọc trước khi xuất.</p> : null}
      </div>
    </Dialog>
  );
}

function pad(value: number) {
  return String(value).padStart(2, '0');
}
