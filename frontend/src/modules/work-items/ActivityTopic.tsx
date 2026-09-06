import { createContext, useContext, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { ArrowUpRight, FileText } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { useAuth } from '../../lib/auth/AuthContext';
import { activityTopic, activityTopicTypes, type ActivityTopic, type ActivityTopicSource } from './activityTopicDomain';
import ActivityTopicDetails from './ActivityTopicDetails';

const TopicContext = createContext<{ projectId: string; open: (topic: ActivityTopic) => void } | null>(null);

export function ActivityTopicProvider({ projectId, children }: { projectId: string; children: ReactNode }) {
  const { user } = useAuth();
  return <TopicSession key={`${user?.id ?? ''}:${projectId}`} projectId={projectId}>{children}</TopicSession>;
}

function TopicSession({ projectId, children }: { projectId: string; children: ReactNode }) {
  const [selected, setSelected] = useState<ActivityTopic | null>(null);
  return <TopicContext.Provider value={{ projectId, open: setSelected }}>
    {children}
    {selected ? createPortal(
      <div className="relative z-[100]">
        <Dialog open title={selected.scope === 'boq_item' ? 'Chi tiết dòng BOQ' : `Chi tiết ${activityTopicTypes[selected.scope].label.toLocaleLowerCase('vi')}`} size="xl" manageFocus onClose={() => setSelected(null)}>
          <ActivityTopicDetails key={`${selected.scope}:${selected.id}`} topic={selected} onClose={() => setSelected(null)} />
        </Dialog>
      </div>, document.body,
    ) : null}
  </TopicContext.Provider>;
}

export function ActivityTopicCard({ source }: { source: ActivityTopicSource }) {
  const context = useContext(TopicContext);
  const topic = context ? activityTopic(source, context.projectId) : null;
  if (!context) return null;
  if (!topic) {
    if (!source.scope_type || source.scope_type === 'project') return null;
    return <div className="my-2 rounded-lg border border-slate-200 bg-slate-100 px-3 py-2 text-xs text-slate-600">
      <p className="break-words font-semibold">{source.target_label || source.label || 'Hồ sơ liên quan'}</p>
      <p className="mt-1">Liên kết hồ sơ không còn khả dụng.</p>
    </div>;
  }
  return <button
    type="button"
    className="group my-2 flex w-full min-w-0 items-center gap-2 rounded-lg border border-blue-200 bg-blue-50 px-3 py-2 text-left text-blue-950 shadow-sm transition-colors hover:border-blue-400 hover:bg-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2"
    aria-label={`Mở ${activityTopicTypes[topic.scope].label}: ${topic.label}`}
    aria-haspopup="dialog"
    onClick={() => context?.open(topic)}
  >
    <FileText className="h-4 w-4 shrink-0 text-blue-600" aria-hidden="true" />
    <span className="min-w-0 flex-1">
      <span className="block text-[10px] font-bold uppercase tracking-wide text-blue-600">{activityTopicTypes[topic.scope].label}</span>
      <span className="block whitespace-normal break-words text-xs font-semibold leading-5 group-hover:underline">{topic.label}</span>
    </span>
    <ArrowUpRight className="h-4 w-4 shrink-0 text-blue-600" aria-hidden="true" />
  </button>;
}
