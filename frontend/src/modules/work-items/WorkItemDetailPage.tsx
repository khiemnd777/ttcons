import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { PageHeader } from '../../components/PageHeader';
import { WorkItemDetailWorkspace } from './WorkItemDetailDialog';
import { safeReturnPath } from './workItemNavigation';

export function WorkItemDetailPage() {
  const { id = '', workItemId = '' } = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const resolvedWorkItemId = workItemId || id;
  const fallbackReturnTo = workItemId && id ? `/projects/${encodeURIComponent(id)}` : '/my-work';
  const returnTo = safeReturnPath(new URLSearchParams(location.search).get('returnTo')) || fallbackReturnTo;

  function goBack() {
    navigate(returnTo);
  }

  return (
    <div className="space-y-4">
      <PageHeader
        title="Chi tiết đầu việc"
        description="Không gian xử lý đầu việc, hồ sơ nghiệp vụ, minh chứng, hoạt động và hành động workflow."
        actions={
          <button
            className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
            onClick={goBack}
            type="button"
          >
            <ArrowLeft className="h-4 w-4" />
            Quay lại
          </button>
        }
      />
      <WorkItemDetailWorkspace workItemId={resolvedWorkItemId} onClose={goBack} />
    </div>
  );
}
