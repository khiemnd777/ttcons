import { useEffect, useRef, useState } from 'react';
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import {
  ArrowDownToLine,
  ArrowUpFromLine,
  BarChart3,
  BriefcaseBusiness,
  Building2,
  Bug,
  ClipboardList,
  Calculator,
  FileClock,
  FileText,
  GitBranch,
  Kanban,
  LayoutDashboard,
  LogOut,
  Menu,
  Settings,
  ShieldCheck,
  UserRound,
  Users,
  X,
  type LucideIcon,
} from 'lucide-react';
import clsx from 'clsx';
import { BrandHomeLink } from '../components/BrandLogo';
import { PoweredBySignature } from '../components/PoweredBySignature';
import { useAuth } from '../lib/auth/AuthContext';
import { roleSummary } from '../lib/utils/roles';
import { NotificationPanel } from '../modules/notifications/NotificationPanel';
import { useShellIndicators } from '../modules/notifications/useShellIndicators';

type NavItem = {
  to: string;
  label: string;
  icon: LucideIcon;
  permissions?: string[];
};

type NavGroup = { title: string; items: NavItem[] };
type NavBadge = { value: number | '…' | '—'; error?: boolean };

const navGroups: NavGroup[] = [
  {
    title: 'Công việc',
    items: [
      { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard, permissions: ['report.view'] },
      { to: '/my-work', label: 'Bàn làm việc', icon: BriefcaseBusiness },
      { to: '/progress', label: 'Tiến trình', icon: Kanban, permissions: ['project.view'] },
      { to: '/projects', label: 'Công trình', icon: Building2, permissions: ['project.view'] },
      { to: '/reports', label: 'Báo cáo', icon: FileText, permissions: ['work_report.view'] },
      { to: '/statistics', label: 'Thống kê', icon: BarChart3, permissions: ['statistics.view'] },
      { to: '/accounting', label: 'Tài chính công trình', icon: Calculator, permissions: ['project_finance.view'] },
    ],
  },
  {
    title: 'Workflow',
    items: [
      { to: '/pipeline-builder', label: 'Trình thiết kế quy trình', icon: GitBranch, permissions: ['pipeline.view'] },
      { to: '/workflow-forms', label: 'Biểu mẫu quy trình', icon: ClipboardList, permissions: ['pipeline.view'] },
    ],
  },
  {
    title: 'Quản trị',
    items: [
      { to: '/core-data/input', label: 'Đầu vào', icon: ArrowDownToLine, permissions: ['catalog.view'] },
      { to: '/core-data/output', label: 'Đầu ra', icon: ArrowUpFromLine, permissions: ['catalog.view'] },
      { to: '/staff', label: 'Nhân sự', icon: Users, permissions: ['staff.view'] },
      { to: '/roles', label: 'Vai trò', icon: ShieldCheck, permissions: ['role.view'] },
      { to: '/audit', label: 'Audit nghiệp vụ', icon: FileClock, permissions: ['audit.view'] },
      { to: '/event-logs', label: 'Nhật ký sự kiện', icon: Bug, permissions: ['event_log.view'] },
      { to: '/settings', label: 'Cấu hình', icon: Settings, permissions: ['settings.view'] },
    ],
  },
];

export function AppShell() {
  const { user, logout, hasAnyPermission } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const mobileNavTriggerRef = useRef<HTMLButtonElement>(null);
  const mobileNavCloseRef = useRef<HTMLButtonElement>(null);
  const mobileNavPanelRef = useRef<HTMLElement>(null);
  const userRoleSummary = roleSummary(user?.roles);
  const projectFocusMode = /^\/projects\/[^/]+/.test(location.pathname);
  const indicators = useShellIndicators(!location.pathname.startsWith('/pipeline-builder') && !projectFocusMode);
  const navigationBadges: Record<string, NavBadge> = {
    '/my-work': indicatorBadge(indicators.myWork.data?.total, indicators.myWork.isLoading, indicators.myWork.isError),
    '/reports': indicatorBadge(indicators.workReports.data?.total_visible, indicators.workReports.isLoading, indicators.workReports.isError),
  };
  const visibleNavGroups = navGroups
    .map((group) => ({
      ...group,
      items: group.items.filter((item) => hasAnyPermission(item.permissions)),
    }))
    .filter((group) => group.items.length > 0);

  useEffect(() => {
    setMobileNavOpen(false);
  }, [location.pathname, location.search]);

  useEffect(() => {
    if (!mobileNavOpen) return undefined;

    const previousOverflow = document.body.style.overflow;
    const focusFrame = window.requestAnimationFrame(() => mobileNavCloseRef.current?.focus());
    document.body.style.overflow = 'hidden';

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        setMobileNavOpen(false);
        window.requestAnimationFrame(() => mobileNavTriggerRef.current?.focus());
        return;
      }

      if (event.key !== 'Tab') return;
      const focusable = Array.from(
        mobileNavPanelRef.current?.querySelectorAll<HTMLElement>('a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])') ?? [],
      );
      if (!focusable.length) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.cancelAnimationFrame(focusFrame);
      window.removeEventListener('keydown', handleKeyDown);
      document.body.style.overflow = previousOverflow;
    };
  }, [mobileNavOpen]);

  const closeMobileNavigation = (returnFocus = true) => {
    setMobileNavOpen(false);
    if (returnFocus) window.requestAnimationFrame(() => mobileNavTriggerRef.current?.focus());
  };

  if (location.pathname.startsWith('/pipeline-builder') || projectFocusMode) {
    return (
      <div className="min-h-screen bg-surface">
        <Outlet />
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-surface">
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-72 border-r border-line bg-white lg:block">
        <div className="flex h-full flex-col">
          <div className="h-16 border-b border-line">
            <BrandHomeLink />
          </div>
          <AppNavigation groups={visibleNavGroups} badges={navigationBadges} className="flex-1 overflow-auto" />
          <div className="border-t border-line px-4 py-3">
            <PoweredBySignature />
          </div>
        </div>
      </aside>

      {mobileNavOpen ? (
        <div className="fixed inset-0 z-50 lg:hidden">
          <button
            type="button"
            tabIndex={-1}
            aria-label="Đóng menu chính"
            className="absolute inset-0 bg-slate-950/40 backdrop-blur-[1px]"
            onClick={() => closeMobileNavigation()}
          />
          <aside
            ref={mobileNavPanelRef}
            id="mobile-navigation"
            role="dialog"
            aria-modal="true"
            aria-label="Menu chính"
            className="relative flex h-full w-[min(18rem,calc(100vw-3rem))] flex-col border-r border-line bg-white shadow-2xl"
          >
            <div className="flex h-16 shrink-0 items-stretch border-b border-line">
              <div className="min-w-0 flex-1">
                <BrandHomeLink onClick={() => setMobileNavOpen(false)} />
              </div>
              <button
                ref={mobileNavCloseRef}
                type="button"
                aria-label="Đóng menu chính"
                className="m-2 grid h-10 w-10 shrink-0 place-items-center rounded-md text-slate-600 hover:bg-slate-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
                onClick={() => closeMobileNavigation()}
              >
                <X className="h-5 w-5" />
              </button>
            </div>
            <AppNavigation groups={visibleNavGroups} badges={navigationBadges} className="min-h-0 flex-1 overflow-auto" onNavigate={() => setMobileNavOpen(false)} />
            <div className="border-t border-line px-4 py-3">
              <p className="truncate text-sm font-semibold text-ink">{user?.full_name}</p>
              <p className="truncate text-xs text-slate-500">{userRoleSummary}</p>
              <button
                type="button"
                className="mt-3 inline-flex w-full items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                onClick={() => {
                  setMobileNavOpen(false);
                  navigate('/profile');
                }}
              >
                <UserRound className="h-4 w-4" />
                Hồ sơ cá nhân
              </button>
              <button
                type="button"
                className="mt-2 inline-flex w-full items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                onClick={() => {
                  setMobileNavOpen(false);
                  void logout().finally(() => navigate('/login'));
                }}
              >
                <LogOut className="h-4 w-4" />
                Đăng xuất
              </button>
            </div>
            <div className="border-t border-line px-4 py-3">
              <PoweredBySignature />
            </div>
          </aside>
        </div>
      ) : null}

      <div className="flex min-h-screen flex-col lg:pl-72">
        <header className="sticky top-0 z-20 flex h-16 items-center border-b border-line bg-white/95 px-4 backdrop-blur md:px-6">
          <div className="flex w-full items-center justify-between gap-4">
            <div className="flex min-w-0 items-center gap-3">
              <button
                ref={mobileNavTriggerRef}
                type="button"
                aria-label="Mở menu chính"
                aria-controls="mobile-navigation"
                aria-expanded={mobileNavOpen}
                title="Mở menu chính"
                className="grid h-10 w-10 shrink-0 place-items-center rounded-md border border-line text-slate-700 hover:bg-slate-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 lg:hidden"
                onClick={() => setMobileNavOpen(true)}
              >
                <Menu className="h-5 w-5" />
              </button>
              <BrandHomeLink compact className="lg:hidden" />
              <p className="hidden text-sm font-medium text-ink sm:block">Bắt đầu từ việc ưu tiên, luôn rõ Dự án › Node › Bước</p>
            </div>
            <div className="flex items-center gap-3">
              <NotificationPanel summary={indicators.notifications.data} />
              <button
                type="button"
                className="hidden rounded-md px-2 py-1 text-right hover:bg-slate-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 sm:block"
                onClick={() => navigate('/profile')}
                aria-label="Mở hồ sơ cá nhân"
              >
                <p className="text-sm font-semibold text-ink">{user?.full_name}</p>
                <p className="text-xs text-slate-500">{userRoleSummary}</p>
              </button>
              <button
                className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
                onClick={() => {
                  void logout().finally(() => navigate('/login'));
                }}
              >
                <LogOut className="h-4 w-4" />
                Đăng xuất
              </button>
            </div>
          </div>
        </header>
        <main className="flex-1 px-4 py-5 md:px-6">
          <Outlet />
        </main>
        <footer className="border-t border-line bg-white px-4 py-3 lg:hidden">
          <PoweredBySignature />
        </footer>
      </div>
    </div>
  );
}

function AppNavigation({
  groups,
  badges,
  className,
  onNavigate,
}: {
  groups: NavGroup[];
  badges: Record<string, NavBadge>;
  className?: string;
  onNavigate?: () => void;
}) {
  return (
    <nav aria-label="Điều hướng chính" className={clsx('px-3 py-5', className)}>
      {groups.map((group) => (
        <div key={group.title} className="mb-5 last:mb-0">
          <p className="mb-2 px-3 text-xs font-semibold uppercase tracking-wide text-slate-400">{group.title}</p>
          <div className="space-y-1">
            {group.items.map((item) => (
              <NavLink
                key={item.to}
                to={item.to}
                onClick={onNavigate}
                className={({ isActive }) =>
                  clsx(
                    'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium',
                    isActive ? 'bg-blue-50 text-blue-700' : 'text-slate-600 hover:bg-slate-100 hover:text-ink',
                  )
                }
              >
                <item.icon className="h-4 w-4" />
                <span className="min-w-0 flex-1">{item.label}</span>
                {badges[item.to] ? (
                  <span className={clsx('min-w-6 rounded-full px-1.5 text-center text-xs font-semibold leading-5', badges[item.to].error ? 'bg-rose-50 text-rose-700' : 'bg-slate-100 text-slate-700')} aria-label={`${item.label}: ${badges[item.to].value}`}>
                    {compactNavigationCount(badges[item.to].value)}
                  </span>
                ) : null}
              </NavLink>
            ))}
          </div>
        </div>
      ))}
    </nav>
  );
}

function indicatorBadge(value: number | undefined, loading: boolean, error: boolean): NavBadge {
  if (loading) return { value: '…' };
  if (error) return { value: '—', error: true };
  return { value: value ?? 0 };
}

function compactNavigationCount(value: NavBadge['value']) {
  return typeof value === 'number' && value > 999 ? '999+' : value;
}
