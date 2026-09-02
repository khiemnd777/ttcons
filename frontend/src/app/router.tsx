import { createBrowserRouter, Navigate, useLocation, useParams } from 'react-router-dom';
import { lazy, type ReactNode } from 'react';
import { AppShell } from '../layouts/AppShell';
import { useAuth } from '../lib/auth/AuthContext';
import { LoadingSpinner } from '../components/LoadingSpinner';
import { PermissionDenied } from '../components/PermissionDenied';
import { LoginPage } from '../modules/auth/LoginPage';
import { RouteModuleBoundary } from './RouteModuleBoundary';

const DashboardPage = lazy(() => import('../modules/dashboard/DashboardPage').then(({ DashboardPage }) => ({ default: DashboardPage })));
const ProjectsPage = lazy(() => import('../modules/projects/ProjectsPage').then(({ ProjectsPage }) => ({ default: ProjectsPage })));
const ProjectProgressPage = lazy(() => import('../modules/projects/ProjectProgressPage').then(({ ProjectProgressPage }) => ({ default: ProjectProgressPage })));
const ProjectDetailPage = lazy(() => import('../modules/projects/ProjectDetailPage').then(({ ProjectDetailPage }) => ({ default: ProjectDetailPage })));
const ProjectWorkspaceLayout = lazy(() => import('../modules/projects/ProjectWorkspaceLayout').then(({ ProjectWorkspaceLayout }) => ({ default: ProjectWorkspaceLayout })));
const MyWorkPage = lazy(() => import('../modules/my-work/MyWorkPage').then(({ MyWorkPage }) => ({ default: MyWorkPage })));
const ProfilePage = lazy(() => import('../modules/profile/ProfilePage').then(({ ProfilePage }) => ({ default: ProfilePage })));
const ForgotPasswordPage = lazy(() => import('../modules/auth/ForgotPasswordPage').then(({ ForgotPasswordPage }) => ({ default: ForgotPasswordPage })));
const ResetPasswordPage = lazy(() => import('../modules/auth/ResetPasswordPage').then(({ ResetPasswordPage }) => ({ default: ResetPasswordPage })));
const VerifyRecoveryEmailPage = lazy(() => import('../modules/auth/VerifyRecoveryEmailPage').then(({ VerifyRecoveryEmailPage }) => ({ default: VerifyRecoveryEmailPage })));
const InitialPasswordChangePage = lazy(() => import('../modules/auth/InitialPasswordChangePage').then(({ InitialPasswordChangePage }) => ({ default: InitialPasswordChangePage })));
const WorkItemDetailPage = lazy(() => import('../modules/work-items/WorkItemDetailPage').then(({ WorkItemDetailPage }) => ({ default: WorkItemDetailPage })));
const PipelineBuilderPage = lazy(() => import('../modules/pipeline-builder/PipelineBuilderPage').then(({ PipelineBuilderPage }) => ({ default: PipelineBuilderPage })));
const WorkflowFormsPage = lazy(() => import('../modules/workflow-forms/WorkflowFormsPage').then(({ WorkflowFormsPage }) => ({ default: WorkflowFormsPage })));
const PlanningDailyReportsPage = lazy(() => import('../modules/planning-daily-reports/PlanningDailyReportsPage').then(({ PlanningDailyReportsPage }) => ({ default: PlanningDailyReportsPage })));
const MeetingMinutesPage = lazy(() => import('../modules/meeting-minutes/MeetingMinutesPage').then(({ MeetingMinutesPage }) => ({ default: MeetingMinutesPage })));
const PlanningChecklistsPage = lazy(() => import('../modules/planning-checklists/PlanningChecklistsPage').then(({ PlanningChecklistsPage }) => ({ default: PlanningChecklistsPage })));
const PlanningPage = lazy(() => import('../modules/planning/PlanningPage').then(({ PlanningPage }) => ({ default: PlanningPage })));
const PreparationPage = lazy(() => import('../modules/planning/PreparationPage').then(({ PreparationPage }) => ({ default: PreparationPage })));
const BOQPage = lazy(() => import('../modules/boq/BOQPage').then(({ BOQPage }) => ({ default: BOQPage })));
const ProcurementPage = lazy(() => import('../modules/procurement/ProcurementPage').then(({ ProcurementPage }) => ({ default: ProcurementPage })));
const ContractsPage = lazy(() => import('../modules/contracts/ContractsPage').then(({ ContractsPage }) => ({ default: ContractsPage })));
const WarehousePage = lazy(() => import('../modules/warehouse/WarehousePage').then(({ WarehousePage }) => ({ default: WarehousePage })));
const OperationsPage = lazy(() => import('../modules/operations/OperationsPage').then(({ OperationsPage }) => ({ default: OperationsPage })));
const AcceptancePage = lazy(() => import('../modules/acceptance/AcceptancePage').then(({ AcceptancePage }) => ({ default: AcceptancePage })));
const ReworkPage = lazy(() => import('../modules/rework/ReworkPage').then(({ ReworkPage }) => ({ default: ReworkPage })));
const HandoverPage = lazy(() => import('../modules/handover/HandoverPage').then(({ HandoverPage }) => ({ default: HandoverPage })));
const ProjectFinancePage = lazy(() => import('../modules/project-finance/ProjectFinancePage').then(({ ProjectFinancePage }) => ({ default: ProjectFinancePage })));
const DebtPage = lazy(() => import('../modules/debt/DebtPage').then(({ DebtPage }) => ({ default: DebtPage })));
const CoreDataPage = lazy(() => import('../modules/core-data/CoreDataPage').then(({ CoreDataPage }) => ({ default: CoreDataPage })));
const StaffPage = lazy(() => import('../modules/staff/StaffPage').then(({ StaffPage }) => ({ default: StaffPage })));
const RolesPage = lazy(() => import('../modules/rbac/RolesPage').then(({ RolesPage }) => ({ default: RolesPage })));
const PermissionsPage = lazy(() => import('../modules/rbac/PermissionsPage').then(({ PermissionsPage }) => ({ default: PermissionsPage })));
const AuditPage = lazy(() => import('../modules/audit/AuditPage').then(({ AuditPage }) => ({ default: AuditPage })));
const EventLogsPage = lazy(() => import('../modules/event-logs/EventLogsPage').then(({ EventLogsPage }) => ({ default: EventLogsPage })));
const ReportsPage = lazy(() => import('../modules/reports/ReportsPage').then(({ ReportsPage }) => ({ default: ReportsPage })));
const WorkReportsPage = lazy(() => import('../modules/work-reports/WorkReportsPage').then(({ WorkReportsPage }) => ({ default: WorkReportsPage })));
const NotificationsPage = lazy(() => import('../modules/notifications/NotificationsPage').then(({ NotificationsPage }) => ({ default: NotificationsPage })));
const SettingsPage = lazy(() => import('../modules/settings/SettingsPage').then(({ SettingsPage }) => ({ default: SettingsPage })));

function RequireAuth() {
  const { user, loading } = useAuth();
  if (loading) return <div className="p-6"><LoadingSpinner /></div>;
  if (!user) return <Navigate to="/login" replace />;
  if (user.must_change_password) return <Navigate to="/change-initial-password" replace />;
  return <AppShell />;
}

function RequireInitialPasswordChange() {
  const { user, loading } = useAuth();
  if (loading) return <div className="p-6"><LoadingSpinner /></div>;
  if (!user) return <Navigate to="/login" replace />;
  if (!user.must_change_password) return <Navigate to="/" replace />;
  return <RouteModuleBoundary focus><InitialPasswordChangePage /></RouteModuleBoundary>;
}

function RequirePermission({ permissions, excludePermissions, children }: { permissions?: string[]; excludePermissions?: string[]; children: ReactNode }) {
  const { hasAnyPermission } = useAuth();
  if (!hasAnyPermission(permissions)) return <PermissionDenied permissions={permissions} />;
  if (excludePermissions?.length && hasAnyPermission(excludePermissions)) return <PermissionDenied />;
  return <>{children}</>;
}

function HomeRedirect() {
  return <Navigate to="/my-work" replace />;
}

function LegacyStatisticsRedirect() {
  const { reportKey } = useParams<{ reportKey?: string }>();
  const location = useLocation();
  const target = reportKey ? `/statistics/${reportKey}` : '/statistics';
  return <Navigate to={`${target}${location.search}`} replace />;
}

function guarded(element: ReactNode, permissions?: string[], focus = false, excludePermissions?: string[]) {
  return (
    <RequirePermission permissions={permissions} excludePermissions={excludePermissions}>
      <RouteModuleBoundary focus={focus}>{element}</RouteModuleBoundary>
    </RequirePermission>
  );
}

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  { path: '/forgot-password', element: <RouteModuleBoundary focus><ForgotPasswordPage /></RouteModuleBoundary> },
  { path: '/reset-password', element: <RouteModuleBoundary focus><ResetPasswordPage /></RouteModuleBoundary> },
  { path: '/verify-recovery-email', element: <RouteModuleBoundary focus><VerifyRecoveryEmailPage /></RouteModuleBoundary> },
  { path: '/change-initial-password', element: <RequireInitialPasswordChange /> },
  {
    element: <RequireAuth />,
    children: [
      { index: true, element: <HomeRedirect /> },
      { path: '/dashboard', element: guarded(<DashboardPage />, ['report.view']) },
      { path: '/progress', element: guarded(<ProjectProgressPage />, ['project.view']) },
      { path: '/my-work', element: guarded(<MyWorkPage />) },
      { path: '/profile', element: guarded(<ProfilePage />) },
      { path: '/notifications', element: guarded(<NotificationsPage />) },
      { path: '/work-items/:id', element: guarded(<WorkItemDetailPage />) },
      { path: '/projects', element: guarded(<ProjectsPage />, ['project.view']) },
      {
        path: '/projects/:id',
        element: guarded(<ProjectWorkspaceLayout />, ['project.view'], true),
        children: [
          { index: true, element: guarded(<ProjectDetailPage />) },
          { path: 'work-items/:workItemId', element: guarded(<WorkItemDetailPage />) },
          { path: 'pipeline', element: guarded(<PlanningPage mode="pipeline" />, ['pipeline.view']) },
          { path: 'preparation', element: guarded(<PreparationPage />, ['project.view']) },
          { path: 'planning', element: guarded(<PlanningPage />, ['planning.view', 'pipeline.view']) },
          { path: 'boq', element: guarded(<BOQPage />, ['boq.view']) },
          { path: 'procurement', element: guarded(<ProcurementPage scoped />, ['procurement.view']) },
          { path: 'contracts', element: guarded(<ContractsPage />, ['contract.view']) },
          { path: 'warehouse', element: guarded(<WarehousePage scoped />, ['warehouse.view', 'material.view']) },
          { path: 'operations', element: guarded(<OperationsPage />, ['operation.view', 'quality.view', 'rework.view', 'replanning.view']) },
          { path: 'quality', element: guarded(<Navigate to="../operations?tab=checks" replace />, ['quality.view']) },
          { path: 'acceptance', element: guarded(<AcceptancePage />, ['acceptance.view']) },
          { path: 'rework', element: guarded(<Navigate to="../operations?tab=actions" replace />, ['rework.view']) },
          { path: 'handover', element: guarded(<HandoverPage />, ['handover.view']) },
          { path: 'accounting', element: guarded(<ProjectFinancePage />, ['accounting_handoff.view', 'project_finance.view']) },
          { path: 'audit', element: guarded(<AuditPage scoped />, ['audit.view'], false, ['project.reconciliation.view']) },
        ],
      },
      { path: '/pipeline-builder', element: guarded(<PipelineBuilderPage />, ['pipeline.view'], true, ['project.reconciliation.view']) },
      { path: '/pipeline-builder/:id', element: guarded(<PipelineBuilderPage />, ['pipeline.view'], true, ['project.reconciliation.view']) },
      { path: '/workflow-forms', element: guarded(<WorkflowFormsPage />, ['pipeline.view'], false, ['project.reconciliation.view']) },
      { path: '/planning/daily-reports', element: guarded(<PlanningDailyReportsPage />, ['planning.view', 'pipeline.view']) },
      { path: '/planning/meeting-minutes', element: guarded(<MeetingMinutesPage />, ['planning.view', 'pipeline.view']) },
      { path: '/planning/checklists', element: guarded(<PlanningChecklistsPage />, ['planning.view', 'pipeline.view']) },
      { path: '/procurement', element: guarded(<ProcurementPage />, ['procurement.view']) },
      { path: '/warehouse', element: guarded(<WarehousePage />, ['warehouse.view', 'material.view']) },
      { path: '/operations', element: guarded(<OperationsPage global />, ['operation.view', 'quality.view', 'rework.view', 'replanning.view']) },
      { path: '/acceptance', element: guarded(<AcceptancePage global />, ['acceptance.view']) },
      { path: '/rework', element: guarded(<ReworkPage />, ['rework.view']) },
      { path: '/handover', element: guarded(<HandoverPage />, ['handover.view']) },
      { path: '/accounting', element: guarded(<ProjectFinancePage global />, ['accounting_handoff.view', 'project_finance.view']) },
      { path: '/debt/payables', element: guarded(<DebtPage kind="payables" />, ['accounting.payable_view']) },
      { path: '/debt/receivables', element: guarded(<DebtPage kind="receivables" />, ['accounting.receivable_view']) },
      { path: '/core-data', element: guarded(<Navigate to="/core-data/input" replace />, ['catalog.view']) },
      { path: '/core-data/input', element: guarded(<CoreDataPage key="partner-input" group="input" />, ['catalog.view']) },
      { path: '/core-data/output', element: guarded(<CoreDataPage key="partner-output" group="output" />, ['catalog.view']) },
      { path: '/staff', element: guarded(<StaffPage />, ['staff.view'], false, ['project.reconciliation.view']) },
      { path: '/roles', element: guarded(<RolesPage />, ['role.view'], false, ['project.reconciliation.view']) },
      { path: '/permissions', element: guarded(<PermissionsPage />, ['permission.view'], false, ['project.reconciliation.view']) },
      { path: '/audit', element: guarded(<AuditPage />, ['audit.view'], false, ['project.reconciliation.view']) },
      { path: '/event-logs', element: guarded(<EventLogsPage />, ['event_log.view'], false, ['project.reconciliation.view']) },
      { path: '/reports', element: guarded(<WorkReportsPage />, ['work_report.view']) },
      { path: '/reports/:reportKey', element: guarded(<LegacyStatisticsRedirect />, ['statistics.view']) },
      { path: '/statistics', element: guarded(<ReportsPage />, ['statistics.view']) },
      { path: '/statistics/:reportKey', element: guarded(<ReportsPage />, ['statistics.view']) },
      { path: '/settings', element: guarded(<SettingsPage />, ['settings.view'], false, ['project.reconciliation.view']) },
    ],
  },
]);
