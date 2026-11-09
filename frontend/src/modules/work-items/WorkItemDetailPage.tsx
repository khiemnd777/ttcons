import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { WorkItemDetailWorkspace } from './WorkItemDetailDialog';
import { safeReturnPath } from './workItemNavigation';
import { withWordless } from '../../lib/utils/wordless';

export function WorkItemDetailPage() {
  const { id = '', workItemId = '' } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const resolvedWorkItemId = workItemId || id;
  const fallbackReturnTo = workItemId && id ? `/projects/${encodeURIComponent(id)}` : '/my-work';
  const returnTo = withWordless(safeReturnPath(new URLSearchParams(location.search).get('returnTo')) || fallbackReturnTo, location.search);

  function goBack() {
    navigate(returnTo);
  }

  return <WorkItemDetailWorkspace workItemId={resolvedWorkItemId} onClose={goBack} />;
}
