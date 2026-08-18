import { AsyncProjectPicker } from '../_shared/AsyncProjectPicker';

export function ReportProjectPicker({
  value,
  onChange,
  disabled = false,
}: {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  return (
    <AsyncProjectPicker
      value={value}
      onChange={onChange}
      endpoint="/statistics/options"
      queryKey="statistics-project-options"
      emptyLabel="Tất cả công trình được phép xem"
      disabled={disabled}
    />
  );
}
