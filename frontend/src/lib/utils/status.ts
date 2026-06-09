export const statusLabels: Record<string, string> = {
  draft: 'Nháp',
  submitted: 'Đã gửi',
  waiting_approval: 'Chờ duyệt',
  approved: 'Đã duyệt',
  rejected: 'Từ chối',
  in_progress: 'Đang xử lý',
  pending: 'Chờ xử lý',
  ready: 'Sẵn sàng',
  blocked: 'Bị chặn',
  completed: 'Hoàn tất',
  cancelled: 'Đã hủy',
  overdue: 'Quá hạn',
  partially_paid: 'Trả một phần',
  paid: 'Đã trả',
  partially_received: 'Thu một phần',
  received: 'Đã thu',
  published: 'Đã publish',
  archived: 'Lưu trữ',
  open: 'Mở',
  resolved: 'Đã xử lý',
  closed: 'Đã đóng',
  fail: 'Không đạt',
  pass: 'Đạt',
  conditional_pass: 'Đạt có điều kiện',
  handed_over: 'Đã bàn giao',
  checking: 'Đang kiểm tra',
};

export function statusLabel(status?: string) {
  if (!status) return '-';
  return statusLabels[status] ?? status;
}
