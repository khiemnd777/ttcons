import { Link } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';

export function ProjectBackLink({ projectId }: { projectId?: string | null }) {
  if (!projectId) return null;

  return (
    <Link
      to={`/projects/${projectId}`}
      className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
    >
      <ArrowLeft className="h-4 w-4" />
      Về công trình
    </Link>
  );
}
