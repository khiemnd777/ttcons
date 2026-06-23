import { ShieldAlert } from 'lucide-react';
import { Link } from 'react-router-dom';

export function PermissionDenied({ permissions }: { permissions?: string[] }) {
  return (
    <div className="rounded-lg border border-amber-200 bg-amber-50 p-5 text-sm text-amber-900">
      <div className="flex items-start gap-3">
        <ShieldAlert className="mt-0.5 h-5 w-5 shrink-0" />
        <div>
          <h1 className="text-base font-semibold">Không đủ quyền</h1>
          <p className="mt-1 text-amber-800">
            Bạn cần quyền {permissions?.length ? <code className="rounded bg-white/70 px-1 py-0.5">{permissions.join(' hoặc ')}</code> : 'phù hợp'} để mở màn hình này.
          </p>
          <Link className="mt-3 inline-flex rounded-md border border-amber-200 bg-white px-3 py-2 text-xs font-semibold text-amber-900 hover:bg-amber-100" to="/dashboard">
            Quay về Dashboard
          </Link>
        </div>
      </div>
    </div>
  );
}
