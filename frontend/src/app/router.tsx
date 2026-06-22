import { createBrowserRouter, Navigate } from 'react-router-dom';
import { AppShell } from '../layouts/AppShell';
import { useAuth } from '../lib/auth/AuthContext';
import { LoadingSpinner } from '../components/LoadingSpinner';
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

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  {
    element: <RequireAuth />,
    children: [
      { index: true, element: <Navigate to="/dashboard" replace /> },
      { path: '/dashboard', element: <DashboardPage /> },
      { path: '/my-work', element: <MyWorkPage /> },
      { path: '/projects', element: <ProjectsPage /> },
      { path: '/projects/:id', element: <ProjectDetailPage /> },
      { path: '/projects/:id/pipeline', element: <PlanningPage mode="pipeline" /> },
      { path: '/projects/:id/planning', element: <PlanningPage /> },
      { path: '/projects/:id/boq', element: <BOQPage /> },
      { path: '/projects/:id/procurement', element: <ProcurementPage scoped /> },
      { path: '/projects/:id/warehouse', element: <WarehousePage scoped /> },
      { path: '/projects/:id/operations', element: <OperationsPage /> },
      { path: '/projects/:id/acceptance', element: <AcceptancePage /> },
      { path: '/projects/:id/rework', element: <ReworkPage /> },
      { path: '/projects/:id/handover', element: <HandoverPage /> },
      { path: '/projects/:id/accounting', element: <AccountingPage /> },
      { path: '/projects/:id/debt', element: <DebtPage kind="both" /> },
      { path: '/projects/:id/audit', element: <AuditPage scoped /> },
      { path: '/pipeline-builder', element: <PipelineBuilderPage /> },
      { path: '/pipeline-builder/:id', element: <PipelineBuilderPage /> },
      { path: '/procurement', element: <ProcurementPage /> },
      { path: '/warehouse', element: <WarehousePage /> },
      { path: '/operations', element: <OperationsPage global /> },
      { path: '/acceptance', element: <AcceptancePage global /> },
      { path: '/rework', element: <ReworkPage /> },
      { path: '/handover', element: <HandoverPage /> },
      { path: '/accounting', element: <AccountingPage global /> },
      { path: '/debt/payables', element: <DebtPage kind="payables" /> },
      { path: '/debt/receivables', element: <DebtPage kind="receivables" /> },
      { path: '/core-data', element: <CoreDataPage /> },
      { path: '/staff', element: <StaffPage /> },
      { path: '/roles', element: <RolesPage /> },
      { path: '/permissions', element: <PermissionsPage /> },
      { path: '/audit', element: <AuditPage /> },
      { path: '/reports', element: <ReportsPage /> },
      { path: '/settings', element: <SettingsPage /> },
    ],
  },
]);
