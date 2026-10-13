// Display only: the six-code V20 contract does not change stored answers or edit values.
const weatherLabels: Record<string, string> = {
  normal: 'Bình thường', hot: 'Nắng nóng', rain: 'Mưa', wind: 'Gió mạnh',
  restricted_access: 'Hạn chế mặt bằng', other: 'Khác',
};

export function constructionReportWeatherLabel(value: unknown): string {
  if (value && typeof value === 'object' && 'value' in value) value = value.value;
  if (typeof value !== 'string' || !value.trim()) return 'Chưa ghi thời tiết';
  return Object.prototype.hasOwnProperty.call(weatherLabels, value.trim()) ? weatherLabels[value.trim()] : value;
}
