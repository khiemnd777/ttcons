export function SampleDataBadge({ compact = false }: { compact?: boolean }) {
  return (
    <span className="inline-flex shrink-0 items-center rounded bg-violet-50 px-2 py-0.5 text-[11px] font-semibold text-violet-700">
      {compact ? 'Mẫu' : 'Dữ liệu mẫu'}
    </span>
  );
}
