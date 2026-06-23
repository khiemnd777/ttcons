import { createBrowserRouter, Navigate } from 'react-router-dom';
import type { ReactNode } from 'react';
import { AppShell } from '../layouts/AppShell';
import { useAuth } from '../lib/auth/AuthContext';
import { LoadingSpinner } from '../components/LoadingSpinner';
import { PermissionDenied } from '../components/PermissionDenied';
import { LoginPage } from '../modules/auth/LoginPage';
import { DashboardPage } from '../modules/dashboard/DashboardPage';
import { ProjectsPage } from '../modules/projects/ProjectsPage';
import { ProjectDetailPage } from '../modules/projects/ProjectDetailPage';
import { MyWorkPage } from '../modules/my-work/MyWorkPage';
import { PipelineBuilderPage } from '../modules/pipeline-builder/PipelineBuilderPage';
import { PlanningPage } from '../modules/planning/PlanningPage';
import { BOQPage } from '../modules/boq/BOQPage';
import { ProcurementPage } from '../modules/procurement/ProcurementPage';
import { WarehousePage } from '../modules/warehouse/WarehousePage';
import { OperationsPage } from '../modules/operations/OperationsPage';
import { AcceptancePage } from '../modules/acceptance/AcceptancePage';
import { ReworkPage } from '../modules/rework/ReworkPage';
import { HandoverPage } from '../modules/handover/HandoverPage';
import { AccountingPage } from '../modules/accounting/AccountingPage';
import { DebtPage } from '../modules/debt/DebtPage';
import { CoreDataPage } from '../modules/core-data/CoreDataPage';
import { StaffPage } from '../modules/staff/StaffPage';
import { RolesPage } from '../modules/rbac/RolesPage';
import { PermissionsPage } from '../modules/rbac/PermissionsPage';
import { AuditPage } from '../modules/audit/AuditPage';
import { ReportsPage } from '../modules/reports/ReportsPage';
import { SettingsPage } from '../modules/settings/SettingsPage';

function RequireAuth() {
  const { user, loading } = useAuth();
  if (loading) return <div className="p-6"><LoadingSpinner /></div>;
  if (!user) return <Navigate to="/login" replace />;
  return <AppShell />;
}

function RequirePermission({ permissions, children }: { permissions?: string[]; children: ReactNode }) {
  const { hasAnyPermission } = useAuth();
  if (!hasAnyPermission(permissions)) return <PermissionDenied permissions={permissions} />;
  return <>{children}</>;
}

function HomeRedirect() {
  const { hasAnyPermission } = useAuth();
  return <Navigate to={hasAnyPermission(['report.view']) ? '/dashboard' : '/my-work'} replace />;
}

function guarded(element: ReactNode, permissions?: string[]) {
  return <RequirePermission permissions={permissions}>{element}</RequirePermission>;
}

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  {
    element: <RequireAuth />,
    children: [
      { index: true, element: <HomeRedirect /> },
      { path: '/dashboard', element: guarded(<DashboardPage />, ['report.view']) },
      { path: '/my-work', element: <MyWorkPage /> },
      { path: '/projects', element: guarded(<ProjectsPage />, ['project.view']) },
      { path: '/projects/:id', element: guarded(<ProjectDetailPage />, ['project.view']) },
      { path: '/projects/:id/pipeline', element: guarded(<PlanningPage mode="pipeline" />, ['pipeline.view']) },
      { path: '/projects/:id/planning', element: guarded(<PlanningPage />, ['planning.view', 'pipeline.view']) },
      { path: '/projects/:id/boq', element: guarded(<BOQPage />, ['boq.view']) },
      { path: '/projects/:id/procurement', element: guarded(<ProcurementPage scoped />, ['procurement.view']) },
      { path: '/projects/:id/warehouse', element: guarded(<WarehousePage scoped />, ['warehouse.view']) },
      { path: '/projects/:id/operations', element: guarded(<OperationsPage />, ['operation.view']) },
      { path: '/projects/:id/acceptance', element: guarded(<AcceptancePage />, ['acceptance.view']) },
      { path: '/projects/:id/rework', element: guarded(<ReworkPage />, ['rework.view']) },
      { path: '/projects/:id/handover', element: guarded(<HandoverPage />, ['handover.view']) },
      { path: '/projects/:id/accounting', element: guarded(<AccountingPage />, ['accounting.view']) },
      { path: '/projects/:id/debt', element: guarded(<DebtPage kind="both" />, ['accounting.payable_view', 'accounting.receivable_view']) },
      { path: '/projects/:id/audit', element: guarded(<AuditPage scoped />, ['audit.view']) },
      { path: '/pipeline-builder', element: guarded(<PipelineBuilderPage />, ['pipeline.view']) },
      { path: '/pipeline-builder/:id', element: guarded(<PipelineBuilderPage />, ['pipeline.view']) },
      { path: '/procurement', element: guarded(<ProcurementPage />, ['procurement.view']) },
      { path: '/warehouse', element: guarded(<WarehousePage />, ['warehouse.view']) },
      { path: '/operations', element: guarded(<OperationsPage global />, ['operation.view']) },
      { path: '/acceptance', element: guarded(<AcceptancePage global />, ['acceptance.view']) },
      { path: '/rework', element: guarded(<ReworkPage />, ['rework.view']) },
      { path: '/handover', element: guarded(<HandoverPage />, ['handover.view']) },
      { path: '/accounting', element: guarded(<AccountingPage global />, ['accounting.view']) },
      { path: '/debt/payables', element: guarded(<DebtPage kind="payables" />, ['accounting.payable_view']) },
      { path: '/debt/receivables', element: guarded(<DebtPage kind="receivables" />, ['accounting.receivable_view']) },
      { path: '/core-data', element: guarded(<CoreDataPage />, ['catalog.view']) },
      { path: '/staff', element: guarded(<StaffPage />, ['staff.view']) },
      { path: '/roles', element: guarded(<RolesPage />, ['role.view']) },
      { path: '/permissions', element: guarded(<PermissionsPage />, ['permission.view']) },
      { path: '/audit', element: guarded(<AuditPage />, ['audit.view']) },
      { path: '/reports', element: guarded(<ReportsPage />, ['report.view', 'report.finance.view']) },
      { path: '/settings', element: guarded(<SettingsPage />, ['settings.view']) },
    ],
  },
]);
