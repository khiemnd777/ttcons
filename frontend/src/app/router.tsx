import { createBrowserRouter, Navigate } from 'react-router-dom';
import type { ReactNode } from 'react';
import { AppShell } from '../layouts/AppShell';
import { useAuth } from '../lib/auth/AuthContext';
import { LoadingSpinner } from '../components/LoadingSpinner';
import { PermissionDenied } from '../components/PermissionDenied';
import { LoginPage } from '../modules/auth/LoginPage';
import { DashboardPage } from '../modules/dashboard/DashboardPage';
import { ProjectsPage } from '../modules/projects/ProjectsPage';
import { ProjectProgressPage } from '../modules/projects/ProjectProgressPage';
import { ProjectDetailPage } from '../modules/projects/ProjectDetailPage';
import { ProjectWorkspaceLayout } from '../modules/projects/ProjectWorkspaceLayout';
import { MyWorkPage } from '../modules/my-work/MyWorkPage';
import { WorkItemDetailPage } from '../modules/work-items/WorkItemDetailPage';
import { PipelineBuilderPage } from '../modules/pipeline-builder/PipelineBuilderPage';
import { WorkflowFormsPage } from '../modules/workflow-forms/WorkflowFormsPage';
import { PlanningDailyReportsPage } from '../modules/planning-daily-reports/PlanningDailyReportsPage';
import { MeetingMinutesPage } from '../modules/meeting-minutes/MeetingMinutesPage';
import { PlanningChecklistsPage } from '../modules/planning-checklists/PlanningChecklistsPage';
import { PlanningPage } from '../modules/planning/PlanningPage';
import { BOQPage } from '../modules/boq/BOQPage';
import { ProcurementPage } from '../modules/procurement/ProcurementPage';
import { ContractsPage } from '../modules/contracts/ContractsPage';
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
  return <Navigate to={hasAnyPermission(['project.view']) ? '/projects' : '/my-work'} replace />;
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
      { path: '/progress', element: guarded(<ProjectProgressPage />, ['project.view']) },
      { path: '/my-work', element: <MyWorkPage /> },
      { path: '/work-items/:id', element: <WorkItemDetailPage /> },
      { path: '/projects', element: guarded(<ProjectsPage />, ['project.view']) },
      {
        path: '/projects/:id',
        element: guarded(<ProjectWorkspaceLayout />, ['project.view']),
        children: [
          { index: true, element: <ProjectDetailPage /> },
          { path: 'work-items/:workItemId', element: <WorkItemDetailPage /> },
          { path: 'pipeline', element: guarded(<PlanningPage mode="pipeline" />, ['pipeline.view']) },
          { path: 'planning', element: guarded(<PlanningPage />, ['planning.view', 'pipeline.view']) },
          { path: 'boq', element: guarded(<BOQPage />, ['boq.view']) },
          { path: 'procurement', element: guarded(<ProcurementPage scoped />, ['procurement.view']) },
          { path: 'contracts', element: guarded(<ContractsPage />, ['contract.view']) },
          { path: 'warehouse', element: guarded(<WarehousePage scoped />, ['warehouse.view']) },
          { path: 'operations', element: guarded(<OperationsPage />, ['operation.view', 'quality.view', 'rework.view', 'replanning.view']) },
          { path: 'quality', element: guarded(<Navigate to="../operations?tab=checks" replace />, ['quality.view']) },
          { path: 'acceptance', element: guarded(<AcceptancePage />, ['acceptance.view']) },
          { path: 'rework', element: guarded(<Navigate to="../operations?tab=actions" replace />, ['rework.view']) },
          { path: 'handover', element: guarded(<HandoverPage />, ['handover.view']) },
          { path: 'accounting', element: guarded(<AccountingPage />, ['accounting.view']) },
          { path: 'debt', element: guarded(<DebtPage kind="both" />, ['accounting.payable_view', 'accounting.receivable_view']) },
          { path: 'audit', element: guarded(<AuditPage scoped />, ['audit.view']) },
        ],
      },
      { path: '/pipeline-builder', element: guarded(<PipelineBuilderPage />, ['pipeline.view']) },
      { path: '/pipeline-builder/:id', element: guarded(<PipelineBuilderPage />, ['pipeline.view']) },
      { path: '/workflow-forms', element: guarded(<WorkflowFormsPage />, ['pipeline.view']) },
      { path: '/planning/daily-reports', element: guarded(<PlanningDailyReportsPage />, ['planning.view', 'pipeline.view']) },
      { path: '/planning/meeting-minutes', element: guarded(<MeetingMinutesPage />, ['planning.view', 'pipeline.view']) },
      { path: '/planning/checklists', element: guarded(<PlanningChecklistsPage />, ['planning.view', 'pipeline.view']) },
      { path: '/procurement', element: guarded(<ProcurementPage />, ['procurement.view']) },
      { path: '/warehouse', element: guarded(<WarehousePage />, ['warehouse.view']) },
      { path: '/operations', element: guarded(<OperationsPage global />, ['operation.view', 'quality.view', 'rework.view', 'replanning.view']) },
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
      { path: '/reports', element: guarded(<ReportsPage />, ['report.view', 'report.finance.view', 'accounting.view']) },
      { path: '/reports/:reportKey', element: guarded(<ReportsPage />, ['report.view', 'report.finance.view', 'accounting.view']) },
      { path: '/settings', element: guarded(<SettingsPage />, ['settings.view']) },
    ],
  },
]);
