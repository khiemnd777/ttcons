import { Link, NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import {
  BarChart3,
  BriefcaseBusiness,
  Building2,
  ClipboardList,
  Calculator,
  Database,
  FileClock,
  GitBranch,
  Kanban,
  LayoutDashboard,
  LogOut,
  Settings,
  ShieldCheck,
  Users,
  type LucideIcon,
} from 'lucide-react';
import clsx from 'clsx';
import { BrandLogo } from '../components/BrandLogo';
import { PoweredBySignature } from '../components/PoweredBySignature';
import { useAuth } from '../lib/auth/AuthContext';

type NavItem = {
  to: string;
  label: string;
  icon: LucideIcon;
  permissions?: string[];
};

const navGroups: { title: string; items: NavItem[] }[] = [
  {
    title: 'Công việc',
    items: [
      { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard, permissions: ['report.view'] },
      { to: '/progress', label: 'Tiến trình', icon: Kanban, permissions: ['project.view'] },
      { to: '/projects', label: 'Công trình', icon: Building2, permissions: ['project.view'] },
      { to: '/my-work', label: 'Việc của tôi', icon: BriefcaseBusiness },
      { to: '/reports', label: 'Báo cáo', icon: BarChart3, permissions: ['report.view'] },
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
      { to: '/core-data', label: 'Dữ liệu lõi', icon: Database, permissions: ['catalog.view'] },
      { to: '/staff', label: 'Nhân sự', icon: Users, permissions: ['staff.view'] },
      { to: '/roles', label: 'Vai trò', icon: ShieldCheck, permissions: ['role.view'] },
      { to: '/audit', label: 'Nhật ký hệ thống', icon: FileClock, permissions: ['audit.view'] },
      { to: '/settings', label: 'Cấu hình', icon: Settings, permissions: ['settings.view'] },
    ],
  },
];

export function AppShell() {
  const { user, logout, hasAnyPermission } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const visibleNavGroups = navGroups
    .map((group) => ({
      ...group,
      items: group.items.filter((item) => hasAnyPermission(item.permissions)),
    }))
    .filter((group) => group.items.length > 0);

  const projectFocusMode = /^\/projects\/[^/]+/.test(location.pathname);

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
            <Link
              to="/"
              aria-label="Về trang mặc định"
              className="flex h-full w-full items-center px-4 transition-colors hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500"
            >
              <BrandLogo size={40} variant="lockup" />
            </Link>
          </div>
          <nav className="flex-1 overflow-auto px-3 py-5">
            {visibleNavGroups.map((group) => (
              <div key={group.title} className="mb-5">
                <p className="mb-2 px-3 text-xs font-semibold uppercase tracking-wide text-slate-400">{group.title}</p>
                <div className="space-y-1">
                  {group.items.map((item) => (
                    <NavLink
                      key={item.to}
                      to={item.to}
                      className={({ isActive }) =>
                        clsx(
                          'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium',
                          isActive ? 'bg-blue-50 text-blue-700' : 'text-slate-600 hover:bg-slate-100 hover:text-ink',
                        )
                      }
                    >
                      <item.icon className="h-4 w-4" />
                      {item.label}
                    </NavLink>
                  ))}
                </div>
              </div>
            ))}
          </nav>
          <div className="border-t border-line px-4 py-3">
            <PoweredBySignature />
          </div>
        </div>
      </aside>

      <div className="flex min-h-screen flex-col lg:pl-72">
        <header className="sticky top-0 z-20 flex h-16 items-center border-b border-line bg-white/95 px-4 backdrop-blur md:px-6">
          <div className="flex w-full items-center justify-between gap-4">
            <div className="flex min-w-0 items-center gap-3">
              <Link
                to="/"
                aria-label="Về trang mặc định"
                className="shrink-0 rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 lg:hidden"
              >
                <BrandLogo size={40} />
              </Link>
              <p className="text-sm font-medium text-ink">Bắt đầu từ công trình, xử lý nghiệp vụ theo cấu trúc rõ ràng</p>
            </div>
            <div className="flex items-center gap-3">
              <div className="hidden text-right sm:block">
                <p className="text-sm font-semibold text-ink">{user?.full_name}</p>
                <p className="text-xs text-slate-500">{user?.roles?.join(', ')}</p>
              </div>
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
