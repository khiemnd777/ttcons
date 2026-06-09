export function LoadingSpinner({ label = 'Đang tải dữ liệu...' }: { label?: string }) {
  return (
    <div className="flex items-center gap-3 rounded-lg border border-line bg-white p-4 text-sm text-slate-600">
      <div className="h-4 w-4 animate-spin rounded-full border-2 border-blue-600 border-t-transparent" />
      {label}
    </div>
  );
}
