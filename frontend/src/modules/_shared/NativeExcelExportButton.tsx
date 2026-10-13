import { useState } from 'react';
import { Download, Loader2 } from 'lucide-react';
import { useToast } from '../../components/ToastProvider';
import { downloadNativeReportExcel } from './bmgsExport';

export function NativeExcelExportButton({ path, filename, label = 'Xuất báo cáo', disabled = false }: { path: string; filename: string; label?: string; disabled?: boolean }) {
  const [pending, setPending] = useState(false);
  const toast = useToast();
  async function download() {
    if (pending || disabled) return;
    setPending(true);
    try { await downloadNativeReportExcel(path, filename); toast.success('Đã tải biểu mẫu Excel'); }
    catch (error) { toast.error('Không xuất được biểu mẫu Excel', error); }
    finally { setPending(false); }
  }
  return <button type="button" disabled={pending || disabled} onClick={download} className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 disabled:opacity-50">{pending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}{pending ? 'Đang xuất...' : label}</button>;
}
