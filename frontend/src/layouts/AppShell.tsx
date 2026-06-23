import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import {
  BarChart3,
  Boxes,
  BriefcaseBusiness,
  Building2,
  ClipboardCheck,
  ClipboardList,
  Database,
  FileClock,
  FileText,
  GitBranch,
  Handshake,
  LayoutDashboard,
  LogOut,
  PackageCheck,
  Receipt,
  Settings,
  ShieldCheck,
  Users,
  WalletCards,
  Warehouse,
  Wrench,
  type LucideIcon,
} from 'lucide-react';
import clsx from 'clsx';
import { useAuth } from '../lib/auth/AuthContext';

type NavItem = {
  to: string;
  label: string;
  icon: LucideIcon;
  permissions?: string[];
};

const navGroups: { title: string; items: NavItem[] }[] = [
  {
    title: 'Tổng quan',
    items: [
      { to: '/dashboard', label: 'Dashboard', icon: LayoutDashboard, permissions: ['report.view'] },
      { to: '/my-work', label: 'Việc của tôi', icon: BriefcaseBusiness },
      { to: '/projects', label: 'Công trình', icon: Building2, permissions: ['project.view'] },
      { to: '/pipeline-builder', label: 'Pipeline Builder', icon: GitBranch, permissions: ['pipeline.view'] },
      { to: '/workflow-forms', label: 'Workflow Forms', icon: FileText, permissions: ['pipeline.view'] },
      { to: '/planning/daily-reports', label: 'Daily report KH', icon: ClipboardList, permissions: ['planning.view', 'pipeline.view'] },
      { to: '/planning/meeting-minutes', label: 'Biên bản họp', icon: FileText, permissions: ['planning.view', 'pipeline.view'] },
      { to: '/planning/checklists', label: 'Checklist KH', icon: ClipboardCheck, permissions: ['planning.view', 'pipeline.view'] },
    ],
  },
  {
    title: 'Vận hành',
    items: [
      { to: '/procurement', label: 'Mua sắm vật tư', icon: PackageCheck, permissions: ['procurement.view'] },
      { to: '/warehouse', label: 'Kho vật tư', icon: Warehouse, permissions: ['warehouse.view'] },
      { to: '/operations', label: 'Thi công', icon: ClipboardList, permissions: ['operation.view'] },
      { to: '/acceptance', label: 'Nghiệm thu', icon: ClipboardCheck, permissions: ['acceptance.view'] },
      { to: '/rework', label: 'Rework / Replanning', icon: Wrench, permissions: ['rework.view'] },
      { to: '/handover', label: 'Bàn giao', icon: Handshake, permissions: ['handover.view'] },
    ],
  },
  {
    title: 'Tài chính',
    items: [
      { to: '/accounting', label: 'Kế toán', icon: WalletCards, permissions: ['accounting.view'] },
      { to: '/debt/payables', label: 'Công nợ đầu vào', icon: Receipt, permissions: ['accounting.payable_view'] },
      { to: '/debt/receivables', label: 'Công nợ đầu ra', icon: Receipt, permissions: ['accounting.receivable_view'] },
    ],
  },
  {
    title: 'Hệ thống',
    items: [
      { to: '/core-data', label: 'Dữ liệu lõi', icon: Database, permissions: ['catalog.view'] },
      { to: '/staff', label: 'Nhân sự', icon: Users, permissions: ['staff.view'] },
      { to: '/roles', label: 'Vai trò', icon: ShieldCheck, permissions: ['role.view'] },
      { to: '/permissions', label: 'Phân quyền', icon: ShieldCheck, permissions: ['permission.view'] },
      { to: '/audit', label: 'Audit Log', icon: FileClock, permissions: ['audit.view'] },
      { to: '/reports', label: 'Báo cáo', icon: BarChart3, permissions: ['report.view', 'report.finance.view'] },
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

  if (location.pathname.startsWith('/pipeline-builder')) {
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
          <div className="border-b border-line px-5 py-5">
            <div className="flex items-center gap-3">
              <div className="grid h-10 w-10 place-items-center rounded-lg bg-blue-600 text-sm font-black text-white">CF</div>
              <div>
                <p className="text-sm font-semibold text-ink">Consflow</p>
                <p className="text-xs text-slate-500">Construction Workflow OS</p>
              </div>
            </div>
          </div>
          <nav className="flex-1 overflow-auto px-3 py-4">
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
        </div>
      </aside>

      <div className="lg:pl-72">
        <header className="sticky top-0 z-20 border-b border-line bg-white/95 px-4 py-3 backdrop-blur md:px-6">
          <div className="flex items-center justify-between gap-4">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-400">Workflow Operating System</p>
              <p className="text-sm font-medium text-ink">Quản lý điều kiện để công việc được phép xảy ra</p>
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
        <main className="px-4 py-5 md:px-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
