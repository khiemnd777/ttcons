import { dateText } from '../../lib/utils/format';
import {
  Bell,
  CalendarClock,
  Check,
  ClipboardCheck,
  FileText,
  GitBranch,
  Handshake,
  MessageCircle,
  PackageCheck,
  RefreshCw,
  ShieldCheck,
  Signature,
  TriangleAlert,
  UserRound,
  Warehouse,
  type LucideIcon,
} from 'lucide-react';
import type { UserNotification } from '../../lib/types/domain';

const iconCatalog: Record<string, LucideIcon> = {
  alert: TriangleAlert,
  approval: ClipboardCheck,
  bell: Bell,
  calendar: CalendarClock,
  check: Check,
  checklist: ClipboardCheck,
  commercial: Handshake,
  construction: ClipboardCheck,
  evidence: FileText,
  finance: FileText,
  form: FileText,
  handover: Handshake,
  mention: MessageCircle,
  message: MessageCircle,
  procurement: PackageCheck,
  quality: ClipboardCheck,
  refresh: RefreshCw,
  report: FileText,
  review: ClipboardCheck,
  shield: ShieldCheck,
  signature: Signature,
  task: ClipboardCheck,
  users: UserRound,
  warehouse: Warehouse,
  workflow: GitBranch,
};

const toneCatalog: Record<string, string> = {
  critical: 'bg-rose-50 text-rose-700',
  neutral: 'bg-slate-100 text-slate-700',
  primary: 'bg-blue-50 text-blue-700',
  success: 'bg-emerald-50 text-emerald-700',
  warning: 'bg-amber-50 text-amber-700',
};

export function notificationPresentation(notification: Pick<UserNotification, 'icon_key' | 'tone_key' | 'category_label'>) {
  return {
    Icon: iconCatalog[notification.icon_key] ?? Bell,
    className: toneCatalog[notification.tone_key] ?? toneCatalog.neutral,
    label: notification.category_label || 'Cập nhật',
  };
}

export function safeNotificationHref(value?: string | null) {
  const candidate = String(value ?? '').trim();
  if (!candidate.startsWith('/') || candidate.startsWith('//') || candidate.includes('\\')) return '/notifications';
  try {
    const parsed = new URL(candidate, window.location.origin);
    if (parsed.origin !== window.location.origin) return '/notifications';
    return `${parsed.pathname}${parsed.search}${parsed.hash}`;
  } catch {
    return '/notifications';
  }
}

export function compactNotificationCount(value: number) {
  return value > 99 ? '99+' : String(value);
}

export function notificationDisplayText(value: string) {
  return value.replace(/Anh\/chị/g, 'Bạn').replace(/anh\/chị/g, 'bạn');
}

export function notificationRelativeTime(value: string) {
  const createdAt = new Date(value).getTime();
  if (!Number.isFinite(createdAt)) return value;
  const seconds = Math.max(0, Math.floor((Date.now() - createdAt) / 1000));
  if (seconds < 60) return 'Vừa xong';
  if (seconds < 3600) return `${Math.floor(seconds / 60)} phút`;
  if (seconds < 86_400) return `${Math.floor(seconds / 3600)} giờ`;
  if (seconds < 604_800) return `${Math.floor(seconds / 86_400)} ngày`;
  return dateText(value);
}
