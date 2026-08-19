import { useEffect, useState } from 'react';
import { FileText, Loader2, RefreshCw } from 'lucide-react';

type EvidencePdfViewerProps = {
  name: string;
  previewUrl?: string;
  loading?: boolean;
  error?: string;
  onRetry?: () => void;
};

export function EvidencePdfViewer({
  name,
  previewUrl,
  loading = false,
  error,
  onRetry,
}: EvidencePdfViewerProps) {
  const [frameFailed, setFrameFailed] = useState(false);

  useEffect(() => {
    setFrameFailed(false);
  }, [previewUrl]);

  const visibleError = error || (frameFailed ? 'Trình duyệt không thể hiển thị tệp PDF này.' : '');

  return (
    <div className="overflow-hidden rounded-lg border border-line bg-slate-200">
      <div className="flex h-[60vh] min-h-80 items-center justify-center sm:h-[65vh] xl:h-[calc(100vh-13rem)] xl:min-h-[36rem]">
        {loading ? (
          <div className="px-6 py-10 text-center text-slate-700">
            <Loader2 className="mx-auto h-8 w-8 animate-spin text-blue-700" />
            <p className="mt-3 text-sm font-semibold">Đang tải PDF…</p>
          </div>
        ) : visibleError ? (
          <div className="max-w-md px-6 py-10 text-center">
            <FileText className="mx-auto h-12 w-12 text-rose-400" />
            <p className="mt-3 text-sm font-semibold text-rose-700">{visibleError}</p>
            {onRetry ? (
              <button
                type="button"
                className="mt-4 inline-flex items-center gap-2 rounded-md border border-rose-200 bg-white px-3 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-50"
                onClick={onRetry}
              >
                <RefreshCw className="h-4 w-4" />
                Thử lại
              </button>
            ) : null}
          </div>
        ) : previewUrl ? (
          <iframe
            className="h-full w-full bg-white"
            src={`${previewUrl}#toolbar=1&navpanes=0&view=FitH`}
            title={`Trình đọc PDF ${name}`}
            onError={() => setFrameFailed(true)}
          />
        ) : (
          <div className="px-6 py-10 text-center text-slate-600">
            <FileText className="mx-auto h-12 w-12 text-slate-400" />
            <p className="mt-3 text-sm font-semibold">Hồ sơ chưa có tệp PDF để đọc.</p>
          </div>
        )}
      </div>
    </div>
  );
}
