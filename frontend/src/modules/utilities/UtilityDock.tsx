import { useDeferredValue, useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { createPortal } from 'react-dom';
import {
  AlertCircle,
  ArrowRight,
  BriefcaseBusiness,
  CheckCheck,
  ChevronRight,
  Loader2,
  MessageCircleMore,
  MessageSquarePlus,
  Minus,
  Plus,
  RefreshCw,
  Reply,
  Search,
  Sparkles,
  X,
} from 'lucide-react';
import clsx from 'clsx';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { useToast } from '../../components/ToastProvider';
import { UserAvatar } from '../../components/UserAvatar';
import { get, post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type {
  ActivityPayload,
  AccountingWorkItem,
  MyWorkNavigationSummary,
  MyWorkflowAction,
  ProjectActivityFeed,
  ProjectActivityOptions,
  ProjectDiscussionPage,
  ProjectDiscussionProject,
  ProjectStartupAction,
  ProjectStartupActionsResponse,
  WorkItem,
  WorkItemActivity,
} from '../../lib/types/domain';
import { statusLabel } from '../../lib/utils/status';
import { orderedWorkListEntries, type WorkListEntry, type WorkQueueEntry } from '../my-work/myWorkDomain';
import { ActivityAttachments } from '../work-items/ActivityAttachments';
import { ActivityComposer, type ActivityAttachmentOption, type ActivityMentionOption, type ActivityTargetOption } from '../work-items/ActivityComposer';
import { ActivityTopicCard, ActivityTopicProvider } from '../work-items/ActivityTopic';
import { DiscussionWorkUpdateCard } from '../work-items/DiscussionWorkUpdateCard';
import { activityVisibilityLabel } from '../work-items/activityTopicDomain';
import { ActivityReplyQuote } from '../work-items/ActivityConversationMeta';
import { actorInitials, chatBubbleTone } from '../work-items/activityThreadDomain';
import { executionTargetPath } from '../work-items/workItemExecutionTarget';
import { currentReturnTo } from '../work-items/workItemNavigation';
import { activityLabel, compactDateTime, priorityLabel } from '../work-items/workItemPresentation';

type UtilityPanel = 'work' | 'discussion';
type DesktopDockKey = 'work' | `project:${string}`;

type MyWorkPageResponse = {
  items: WorkItem[];
  entries?: WorkQueueEntry[];
  total: number;
};

type MyWorkflowActionsResponse = {
  items: MyWorkflowAction[];
  total: number;
};

const maxDiscussionTabs = 5;

function projectDockKey(projectID: string): DesktopDockKey {
  return `project:${projectID}`;
}

function desktopPanelCapacity(viewportWidth: number) {
  if (viewportWidth >= 1280) return 5;
  if (viewportWidth >= 1024) return 4;
  if (viewportWidth >= 768) return 3;
  return 1;
}

function responsiveWindowClass(desktopExpanded: boolean, mobileExpanded: boolean) {
  if (desktopExpanded && mobileExpanded) return 'flex';
  if (desktopExpanded) return 'hidden sm:flex';
  if (mobileExpanded) return 'flex sm:hidden';
  return 'hidden';
}

export function UtilityDock() {
  const { user } = useAuth();
  const location = useLocation();
  const toast = useToast();
  const actorID = user?.id ?? '';
  const [openProjectIDs, setOpenProjectIDs] = useState<string[]>([]);
  const [sessionProjects, setSessionProjects] = useState<ProjectDiscussionProject[]>([]);
  const [expandedDockKeys, setExpandedDockKeys] = useState<DesktopDockKey[]>([]);
  const [desktopCapacity, setDesktopCapacity] = useState(() => desktopPanelCapacity(typeof window === 'undefined' ? 0 : window.innerWidth));
  const [mobilePanel, setMobilePanel] = useState<UtilityPanel | null>(null);
  const [mobileProjectID, setMobileProjectID] = useState('');
  const [pickerOpen, setPickerOpen] = useState(false);
  const [pickerSearch, setPickerSearch] = useState('');
  const [projectToReveal, setProjectToReveal] = useState('');
  const [mobileLauncherHost, setMobileLauncherHost] = useState<HTMLElement | null>(null);
  const projectStripRef = useRef<HTMLDivElement>(null);
  const routeProjectID = /^\/projects\/([^/]+)/.exec(location.pathname)?.[1] ?? '';
  const standardShell = !location.pathname.startsWith('/pipeline-builder') && !routeProjectID;

  useEffect(() => {
    setMobileLauncherHost(document.getElementById('utility-dock-mobile-launcher'));
  }, [location.pathname]);

  const workSummary = useQuery({
    queryKey: ['my-work-navigation-summary', actorID],
    queryFn: () => get<MyWorkNavigationSummary>('/me/work-items/navigation-summary'),
    enabled: Boolean(actorID),
    retry: false,
    staleTime: 30_000,
  });
  const discussions = useQuery({
    queryKey: ['project-discussions', actorID, 'dock'],
    queryFn: () => get<ProjectDiscussionPage>('/me/project-discussions?limit=50'),
    enabled: Boolean(actorID),
    retry: false,
    staleTime: 30_000,
  });
  const routeProject = useQuery({
    queryKey: ['project-discussions', actorID, 'route', routeProjectID],
    queryFn: () => get<ProjectDiscussionPage>(`/me/project-discussions?limit=1&project_id=${encodeURIComponent(routeProjectID)}`),
    enabled: Boolean(actorID && routeProjectID && !discussions.data?.items.some((project) => project.project_id === routeProjectID)),
    retry: false,
    staleTime: 30_000,
  });

  const projects = useMemo(() => {
    const rows = [...sessionProjects, ...(routeProject.data?.items ?? []), ...(discussions.data?.items ?? [])];
    return Array.from(new Map(rows.map((project) => [project.project_id, project])).values());
  }, [discussions.data?.items, routeProject.data?.items, sessionProjects]);

  useEffect(() => {
    const updateCapacity = () => setDesktopCapacity(desktopPanelCapacity(window.innerWidth));
    updateCapacity();
    window.addEventListener('resize', updateCapacity);
    return () => window.removeEventListener('resize', updateCapacity);
  }, []);

  useEffect(() => {
    setExpandedDockKeys((current) => current.length > desktopCapacity ? current.slice(-desktopCapacity) : current);
  }, [desktopCapacity]);

  useEffect(() => {
    if (!projectToReveal) return undefined;
    const frame = window.requestAnimationFrame(() => {
      const lanes = projectStripRef.current?.querySelectorAll<HTMLElement>('[data-project-dock-lane]') ?? [];
      const lane = Array.from(lanes).find((item) => item.dataset.projectDockLane === projectToReveal);
      lane?.scrollIntoView({ behavior: 'smooth', block: 'nearest', inline: 'nearest' });
      setProjectToReveal('');
    });
    return () => window.cancelAnimationFrame(frame);
  }, [openProjectIDs, projectToReveal]);

  useEffect(() => {
    if (!pickerOpen && !mobilePanel && !expandedDockKeys.length) return undefined;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      event.preventDefault();
      if (pickerOpen) {
        setPickerOpen(false);
        setPickerSearch('');
        return;
      }
      if (mobilePanel) {
        setMobilePanel(null);
        return;
      }
      setExpandedDockKeys((current) => current.slice(0, -1));
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [expandedDockKeys.length, mobilePanel, pickerOpen]);

  if (!actorID || user?.must_change_password) return null;

  const expandDesktopDock = (dockKey: DesktopDockKey) => {
    setExpandedDockKeys((current) => [...current.filter((key) => key !== dockKey), dockKey].slice(-desktopCapacity));
  };
  const toggleDesktopDock = (dockKey: DesktopDockKey) => {
    if (dockKey !== 'work') setProjectToReveal(dockKey.slice('project:'.length));
    setExpandedDockKeys((current) => current.includes(dockKey)
      ? current.filter((key) => key !== dockKey)
      : [...current, dockKey].slice(-desktopCapacity));
  };
  const addProject = (project: ProjectDiscussionProject) => {
    const projectID = project.project_id;
    if (!openProjectIDs.includes(projectID) && openProjectIDs.length >= maxDiscussionTabs) {
      toast.warn({
        title: 'Đã mở tối đa 5 công trình',
        description: 'Hãy đóng một dock trao đổi trước khi mở công trình khác. Các bản nháp hiện tại vẫn được giữ nguyên.',
      });
      return;
    }
    setSessionProjects((current) => Array.from(new Map([...current, project].map((item) => [item.project_id, item])).values()));
    setOpenProjectIDs((current) => current.includes(projectID) ? current : [...current, projectID]);
    setMobileProjectID(projectID);
    setProjectToReveal(projectID);
    expandDesktopDock(projectDockKey(projectID));
    setPickerOpen(false);
    setPickerSearch('');
  };
  const removeProject = (projectID: string) => {
    const next = openProjectIDs.filter((id) => id !== projectID);
    setSessionProjects((current) => current.filter((project) => project.project_id !== projectID));
    setOpenProjectIDs(next);
    setExpandedDockKeys((current) => current.filter((key) => key !== projectDockKey(projectID)));
    if (mobileProjectID === projectID) {
      setMobileProjectID(next.at(-1) ?? '');
      if (!next.length && mobilePanel === 'discussion') setMobilePanel(null);
    }
  };
  const closeAllProjects = () => {
    setOpenProjectIDs([]);
    setSessionProjects([]);
    setExpandedDockKeys((current) => current.filter((key) => key === 'work'));
    setMobileProjectID('');
    if (mobilePanel === 'discussion') setMobilePanel(null);
    setPickerOpen(false);
    setPickerSearch('');
  };

  const workCount = workSummary.data?.work_total ?? 0;
  const discussionCount = discussions.data?.unread_total ?? 0;
  const showMobileDiscussion = () => {
    const preferred = projects.find((project) => project.project_id === mobileProjectID && openProjectIDs.includes(project.project_id))
      ?? projects.find((project) => openProjectIDs.includes(project.project_id) && project.project_id === routeProjectID)
      ?? projects.find((project) => openProjectIDs.includes(project.project_id) && project.unread_count > 0)
      ?? projects.find((project) => openProjectIDs.includes(project.project_id))
      ?? projects.find((project) => project.project_id === routeProjectID)
      ?? projects.find((project) => project.unread_count > 0)
      ?? projects[0];
    if (!preferred) {
      setMobilePanel(null);
      setPickerOpen(true);
      return;
    }
    if (!openProjectIDs.includes(preferred.project_id)) {
      setOpenProjectIDs((current) => [...current, preferred.project_id]);
      setSessionProjects((current) => Array.from(new Map([...current, preferred].map((item) => [item.project_id, item])).values()));
    }
    setMobileProjectID(preferred.project_id);
    setMobilePanel('discussion');
    setPickerOpen(false);
    setPickerSearch('');
  };
  const selectMobilePanel = (panel: UtilityPanel) => {
    if (panel === 'discussion') {
      showMobileDiscussion();
      return;
    }
    setPickerOpen(false);
    setMobilePanel('work');
  };
  const toggleProjectPicker = () => {
    setPickerOpen((current) => {
      const next = !current;
      if (!next) setPickerSearch('');
      return next;
    });
  };
  const workExpanded = expandedDockKeys.includes('work');
  const compactUtilityLaunchers = openProjectIDs.length >= 4;
  const mobileLauncher = (
    <button
      type="button"
      className={clsx('pointer-events-auto sm:hidden', mobileLauncherHost
        ? 'relative grid h-10 w-10 place-items-center rounded-md border border-line bg-white text-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500'
        : 'inline-flex h-11 items-center gap-2 rounded-full border border-slate-200 bg-white px-4 text-sm font-semibold text-slate-700 shadow-lg hover:border-blue-200 hover:text-blue-700')}
      onClick={() => {
        if (pickerOpen || mobilePanel) {
          setPickerOpen(false);
          setMobilePanel(null);
          return;
        }
        if (discussionCount > 0) showMobileDiscussion();
        else setMobilePanel('work');
      }}
      aria-label={`Mở tiện ích, ${workCount} việc làm ngay, ${discussionCount} tin mới`}
      aria-expanded={Boolean(pickerOpen || mobilePanel)}
    >
      <Sparkles className="h-4 w-4 text-blue-600" />
      {mobileLauncherHost ? <span className="absolute -right-1 -top-1 min-w-4 rounded-full bg-blue-600 px-1 text-center text-[10px] font-bold leading-4 text-white">{compactCount(workCount + discussionCount)}</span>
        : <>Tiện ích <CountBadge value={workCount + discussionCount} /></>}
    </button>
  );

  return (
    <div className="pointer-events-none fixed inset-x-2 bottom-[calc(4.75rem+env(safe-area-inset-bottom))] z-[35] flex flex-col items-end gap-2 sm:bottom-4">
      <div className="pointer-events-none flex w-full items-end gap-2 pb-0.5">
        <div className={clsx('pointer-events-none min-w-0 flex-1', mobilePanel === 'work' ? 'hidden sm:block' : 'block')}>
          <div ref={projectStripRef} data-project-dock-strip className="pointer-events-auto ml-auto flex w-full items-end gap-2 overflow-x-auto sm:w-fit sm:max-w-full sm:snap-x sm:snap-proximity">
            {openProjectIDs.map((projectID) => {
              const project = projects.find((item) => item.project_id === projectID);
              if (!project) return null;
              const dockKey = projectDockKey(projectID);
              const desktopExpanded = expandedDockKeys.includes(dockKey);
              const mobileExpanded = mobilePanel === 'discussion' && mobileProjectID === projectID;
              return (
                <div
                  key={projectID}
                  data-project-dock-lane={projectID}
                  className={clsx(
                    'pointer-events-none w-full min-w-0 flex-col gap-2 sm:w-[336px] sm:flex-none sm:snap-start',
                    mobileExpanded ? 'flex' : 'hidden sm:flex',
                  )}
                >
                  <section
                    role="region"
                    aria-label={`Trao đổi ${project.project_code}`}
                    className={clsx(
                      'pointer-events-auto h-[min(72vh,680px)] w-full flex-col overflow-hidden rounded-xl border border-slate-200 bg-white shadow-2xl',
                      responsiveWindowClass(desktopExpanded, mobileExpanded),
                    )}
                  >
                    <UtilityWindowHeader
                      panel="discussion"
                      title={`${project.project_code} · ${project.project_name}`}
                      subtitle={project.unread_count > 0 ? `${project.unread_count} tin mới` : 'Đã đọc đến tin mới nhất'}
                      workCount={workCount}
                      discussionCount={discussionCount}
                      onSelectMobile={selectMobilePanel}
                      onMinimize={() => {
                        setExpandedDockKeys((current) => current.filter((key) => key !== dockKey));
                        if (mobileProjectID === projectID) setMobilePanel(null);
                      }}
                      onClose={() => removeProject(projectID)}
                    />
                    <MobileProjectTabs
                      projects={projects}
                      openProjectIDs={openProjectIDs}
                      activeProjectID={projectID}
                      onSelect={(nextProjectID) => {
                        setMobileProjectID(nextProjectID);
                        setMobilePanel('discussion');
                      }}
                      onClose={removeProject}
                      onOpenPicker={() => setPickerOpen(true)}
                    />
                    <ProjectConversation project={project} />
                  </section>
                  <div
                    className={clsx(
                      'pointer-events-auto hidden h-11 w-full min-w-0 items-center rounded-full border bg-white shadow-lg sm:flex',
                      desktopExpanded
                        ? 'border-blue-400 text-blue-700 ring-2 ring-blue-100'
                        : project.unread_count > 0
                          ? 'border-blue-200 text-slate-700'
                          : 'border-slate-200 text-slate-700',
                    )}
                  >
                    <button
                      type="button"
                      className="flex min-w-0 flex-1 items-center gap-1.5 py-2 pl-3 text-sm font-semibold"
                      onClick={() => toggleDesktopDock(dockKey)}
                      aria-expanded={desktopExpanded}
                      title={`${project.project_code} · ${project.project_name}`}
                    >
                      <MessageCircleMore className="h-4 w-4 shrink-0" />
                      <span className="truncate">{project.project_code}</span>
                      <CountBadge value={project.unread_count} highlight={project.unread_count > 0} />
                    </button>
                    <button
                      type="button"
                      className="mr-1 grid h-7 w-7 shrink-0 place-items-center rounded-full text-slate-400 hover:bg-slate-100 hover:text-slate-700"
                      onClick={() => removeProject(projectID)}
                      aria-label={`Đóng tab ${project.project_code}`}
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        <button
          type="button"
          className={clsx(
            'pointer-events-auto hidden h-11 shrink-0 items-center justify-center gap-1.5 self-end rounded-full border bg-white text-sm font-semibold shadow-lg sm:inline-flex',
            compactUtilityLaunchers ? 'w-14 px-1' : 'px-4',
            pickerOpen
              ? 'border-blue-400 text-blue-700 ring-2 ring-blue-100'
              : discussionCount > 0
                ? 'border-blue-300 text-blue-700 ring-2 ring-blue-100'
                : 'border-slate-200 text-slate-700 hover:border-blue-200 hover:text-blue-700',
          )}
          onClick={toggleProjectPicker}
          aria-expanded={pickerOpen}
          aria-label={`Mở danh sách trao đổi, ${discussionCount} tin mới`}
          title={compactUtilityLaunchers ? 'Trao đổi' : undefined}
        >
          <Plus className="h-4 w-4 shrink-0" />
          <span className={compactUtilityLaunchers ? 'sr-only' : undefined}>Trao đổi</span>
          <CountBadge value={discussionCount} highlight={discussionCount > 0} />
        </button>

        <div
          className={clsx(
            'pointer-events-none min-w-0 flex-col gap-2',
            mobilePanel === 'work' ? 'flex w-full' : 'hidden',
            workExpanded
              ? 'sm:flex sm:w-[320px] sm:flex-none'
              : compactUtilityLaunchers
                ? 'sm:flex sm:w-14 sm:flex-none'
                : 'sm:flex sm:w-[140px] sm:flex-none',
          )}
        >
          {workExpanded || mobilePanel === 'work' ? (
            <section
              role="region"
              aria-label="Bàn làm việc cần làm ngay"
              className={clsx(
                'pointer-events-auto h-[min(72vh,680px)] w-full flex-col overflow-hidden rounded-xl border border-slate-200 bg-white shadow-2xl',
                responsiveWindowClass(workExpanded, mobilePanel === 'work'),
              )}
            >
              <UtilityWindowHeader
                panel="work"
                title="Tôi phụ trách · Cần làm ngay"
                subtitle={`${workCount} việc cần xử lý`}
                workCount={workCount}
                discussionCount={discussionCount}
                onSelectMobile={selectMobilePanel}
                onMinimize={() => {
                  setExpandedDockKeys((current) => current.filter((key) => key !== 'work'));
                  if (mobilePanel === 'work') setMobilePanel(null);
                }}
              />
              <UtilityWorkPanel />
            </section>
          ) : null}
          <button
            type="button"
            className={clsx(
              'pointer-events-auto hidden h-11 w-full shrink-0 items-center justify-center gap-1.5 rounded-full border bg-white text-sm font-semibold shadow-lg sm:inline-flex',
              workExpanded
                ? 'border-blue-400 text-blue-700 ring-2 ring-blue-100'
                : 'border-slate-200 text-slate-700 hover:border-blue-200 hover:text-blue-700',
            )}
            onClick={() => toggleDesktopDock('work')}
            aria-expanded={workExpanded}
            title={compactUtilityLaunchers && !workExpanded ? 'Làm ngay' : undefined}
          >
            <BriefcaseBusiness className="h-4 w-4 shrink-0" />
            <span className={compactUtilityLaunchers && !workExpanded ? 'sr-only' : undefined}>Làm ngay</span>
            <CountBadge value={workCount} />
          </button>
        </div>
      </div>

      {pickerOpen ? (
        <ProjectPicker
          actorID={actorID}
          projects={projects}
          openProjectIDs={openProjectIDs}
          pickerSearch={pickerSearch}
          onPickerSearch={setPickerSearch}
          onSelectProject={addProject}
          onClose={() => { setPickerOpen(false); setPickerSearch(''); }}
          onCloseAll={closeAllProjects}
          compactUtilityLaunchers={compactUtilityLaunchers}
          workExpanded={workExpanded}
        />
      ) : null}

      {mobileLauncherHost ? createPortal(mobileLauncher, mobileLauncherHost)
        : !standardShell ? <div className="pointer-events-auto flex items-center gap-2 sm:hidden">{mobileLauncher}</div> : null}
    </div>
  );
}

function UtilityWindowHeader({ panel, title, subtitle, workCount, discussionCount, onSelectMobile, onMinimize, onClose }: {
  panel: UtilityPanel;
  title: string;
  subtitle: string;
  workCount: number;
  discussionCount: number;
  onSelectMobile: (panel: UtilityPanel) => void;
  onMinimize: () => void;
  onClose?: () => void;
}) {
  return (
    <header className="flex shrink-0 items-center gap-1 border-b border-line bg-slate-50 px-2 py-2">
      <div className="hidden min-w-0 flex-1 items-center gap-2 px-1 sm:flex">
        {panel === 'work' ? <BriefcaseBusiness className="h-4 w-4 shrink-0 text-blue-600" /> : <MessageCircleMore className="h-4 w-4 shrink-0 text-blue-600" />}
        <span className="min-w-0">
          <span className="block truncate text-sm font-semibold text-ink">
            {title}
          </span>
          <span className="block truncate text-[11px] text-slate-500">
            {subtitle}
          </span>
        </span>
      </div>
      <div className="flex min-w-0 flex-1 items-center gap-1 sm:hidden">
        <button type="button" className={panelTabClass(panel === 'work')} onClick={() => onSelectMobile('work')}>
          <BriefcaseBusiness className="h-4 w-4" /> Làm ngay <CountBadge value={workCount} />
        </button>
        <button type="button" className={panelTabClass(panel === 'discussion')} onClick={() => onSelectMobile('discussion')}>
          <MessageCircleMore className="h-4 w-4" /> Trao đổi <CountBadge value={discussionCount} highlight={discussionCount > 0} />
        </button>
      </div>
      <button type="button" className="rounded-md p-2 text-slate-500 hover:bg-slate-200" onClick={onMinimize} aria-label={`Thu nhỏ ${title}`} title="Thu nhỏ">
        <Minus className="h-4 w-4" />
      </button>
      {onClose ? (
        <button type="button" className="rounded-md p-2 text-slate-500 hover:bg-slate-200" onClick={onClose} aria-label={`Đóng ${title}`} title="Đóng dock trao đổi">
          <X className="h-4 w-4" />
        </button>
      ) : null}
    </header>
  );
}

function MobileProjectTabs({ projects, openProjectIDs, activeProjectID, onSelect, onClose, onOpenPicker }: {
  projects: ProjectDiscussionProject[];
  openProjectIDs: string[];
  activeProjectID: string;
  onSelect: (projectID: string) => void;
  onClose: (projectID: string) => void;
  onOpenPicker: () => void;
}) {
  return (
    <div className="flex shrink-0 items-center gap-1 overflow-x-auto border-b border-line bg-white px-2 py-2 sm:hidden">
      {openProjectIDs.map((projectID) => {
        const project = projects.find((item) => item.project_id === projectID);
        if (!project) return null;
        return (
          <div key={projectID} className={clsx('flex min-w-0 max-w-40 items-center rounded-md border', activeProjectID === projectID ? 'border-blue-200 bg-blue-50' : 'border-line bg-white')}>
            <button type="button" className="min-w-0 flex-1 truncate px-2 py-1.5 text-xs font-semibold text-slate-700" onClick={() => onSelect(projectID)} title={`${project.project_code} · ${project.project_name}`}>
              {project.project_code} {project.unread_count > 0 ? <span className="ml-1 text-blue-700">{compactCount(project.unread_count)}</span> : null}
            </button>
            <button type="button" className="mr-1 rounded p-1 text-slate-400 hover:bg-white hover:text-slate-700" onClick={() => onClose(projectID)} aria-label={`Đóng tab ${project.project_code}`}><X className="h-3 w-3" /></button>
          </div>
        );
      })}
      <button type="button" className="grid h-8 w-8 shrink-0 place-items-center rounded-md border border-dashed border-slate-300 text-slate-500 hover:border-blue-300 hover:text-blue-700" onClick={onOpenPicker} aria-label="Tìm và mở công trình">
        <Plus className="h-4 w-4" />
      </button>
    </div>
  );
}

function UtilityWorkPanel() {
  const { user } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const actorID = user?.id ?? '';
  const work = useQuery({
    queryKey: ['my-work-items', actorID, 'utility-responsible-need-action'],
    queryFn: () => get<MyWorkPageResponse>('/me/work-queue?queue=responsible&bucket=need_action&sort=priority&limit=6'),
    enabled: Boolean(actorID),
    retry: false,
  });
  const actions = useQuery({
    queryKey: ['my-workflow-actions', actorID, 'utility-responsible'],
    queryFn: () => get<MyWorkflowActionsResponse>('/me/workflow-actions?queue=responsible'),
    enabled: Boolean(actorID),
    retry: false,
  });
  const startups = useQuery({
    queryKey: ['my-project-startup-actions', actorID, 'utility'],
    queryFn: () => get<ProjectStartupActionsResponse>('/me/project-startup-actions'),
    enabled: Boolean(actorID),
    retry: false,
  });

  const entries = useMemo(() => {
    const workEntries = work.data?.entries ?? (work.data?.items ?? []).map((item): WorkQueueEntry => ({ kind: 'work_item', item }));
    const startupItems = (startups.data?.items ?? []).filter((item) => item.action_kind === 'setup_required');
    return orderedWorkListEntries(workEntries, startupItems, actions.data?.items ?? [], true).slice(0, 6);
  }, [actions.data?.items, startups.data?.items, work.data?.entries, work.data?.items]);
  const loading = work.isLoading || actions.isLoading || startups.isLoading;
  const allFailed = work.isError && actions.isError && startups.isError;

  const openEntry = (entry: WorkListEntry) => {
    if (entry.kind === 'work_item') {
      navigate(executionTargetPath(entry.item, currentReturnTo(location)));
      return;
    }
    const params = new URLSearchParams();
    if (entry.kind === 'accounting_handoff') params.set('task', `accounting:${entry.item.handoff_id}`);
    if (entry.kind === 'workflow_action') params.set('task', entry.item.id);
    if (entry.kind === 'project_startup') params.set('setup', entry.item.project_id);
    navigate(`/my-work?${params.toString()}`);
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="shrink-0 border-b border-line px-4 py-3 sm:hidden">
        <p className="text-sm font-semibold text-ink">Tôi phụ trách · Cần làm ngay</p>
        <p className="mt-0.5 text-xs text-slate-500">Ưu tiên việc có thể hành động; xử lý đầy đủ trong Bàn làm việc.</p>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto">
        {loading ? <UtilityLoading label="Đang tải việc cần làm…" /> : null}
        {!loading && allFailed ? (
          <UtilityError label="Không tải được Bàn làm việc tinh gọn." onRetry={() => { void work.refetch(); void actions.refetch(); void startups.refetch(); }} />
        ) : null}
        {!loading && !allFailed && !entries.length ? (
          <div className="grid h-full place-items-center px-8 text-center">
            <div><CheckCheck className="mx-auto h-8 w-8 text-emerald-500" /><p className="mt-2 text-sm font-semibold text-ink">Không có việc cần làm ngay</p><p className="mt-1 text-xs text-slate-500">Hàng đợi `Tôi phụ trách` đang trống.</p></div>
          </div>
        ) : null}
        {entries.map((entry) => <WorkUtilityRow key={workEntryKey(entry)} entry={entry} onOpen={() => openEntry(entry)} />)}
        {!loading && !allFailed && (work.isError || actions.isError || startups.isError) ? (
          <p className="m-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">Một nguồn phụ chưa tải được; danh sách đang hiển thị phần dữ liệu còn khả dụng.</p>
        ) : null}
      </div>
      <Link className="flex shrink-0 items-center justify-center gap-2 border-t border-line px-4 py-3 text-sm font-semibold text-blue-700 hover:bg-blue-50" to="/my-work">
        Mở Bàn làm việc đầy đủ <ArrowRight className="h-4 w-4" />
      </Link>
    </div>
  );
}

function WorkUtilityRow({ entry, onOpen }: { entry: WorkListEntry; onOpen: () => void }) {
  const presentation = workEntryPresentation(entry);
  return (
    <button type="button" className="flex w-full items-start gap-3 border-b border-line px-4 py-3 text-left hover:bg-slate-50" onClick={onOpen}>
      <span className={clsx('mt-0.5 grid h-8 w-8 shrink-0 place-items-center rounded-lg', presentation.tone)}>{presentation.icon}</span>
      <span className="min-w-0 flex-1">
        <span className="flex items-start justify-between gap-2">
          <span className="line-clamp-2 text-sm font-semibold leading-5 text-ink">{presentation.title}</span>
          <ChevronRight className="mt-0.5 h-4 w-4 shrink-0 text-slate-400" />
        </span>
        <span className="mt-1 block truncate text-xs font-medium text-slate-600">{presentation.project}</span>
        <span className="mt-1 flex flex-wrap gap-1.5 text-[11px] text-slate-500">
          <span className="rounded bg-slate-100 px-1.5 py-0.5">{presentation.action}</span>
          {presentation.meta ? <span className="px-1 py-0.5">{presentation.meta}</span> : null}
        </span>
      </span>
    </button>
  );
}

function ProjectPicker({ actorID, projects, openProjectIDs, pickerSearch, onPickerSearch, onSelectProject, onClose, onCloseAll, compactUtilityLaunchers, workExpanded }: {
  actorID: string;
  projects: ProjectDiscussionProject[];
  openProjectIDs: string[];
  pickerSearch: string;
  onPickerSearch: (value: string) => void;
  onSelectProject: (project: ProjectDiscussionProject) => void;
  onClose: () => void;
  onCloseAll: () => void;
  compactUtilityLaunchers: boolean;
  workExpanded: boolean;
}) {
  const deferredSearch = useDeferredValue(pickerSearch.trim());
  const searchResults = useQuery({
    queryKey: ['project-discussions', actorID, 'picker', deferredSearch],
    queryFn: () => get<ProjectDiscussionPage>(`/me/project-discussions?limit=20${deferredSearch ? `&search=${encodeURIComponent(deferredSearch)}` : ''}`),
    retry: false,
  });
  const items = searchResults.data?.items ?? (deferredSearch ? [] : projects);

  return (
    <section
      role="dialog"
      aria-label="Mở trao đổi công trình"
      className={clsx(
        'pointer-events-auto absolute bottom-14 right-0 z-30 flex max-h-[min(64vh,520px)] w-full flex-col overflow-hidden rounded-xl border border-slate-200 bg-white shadow-2xl sm:w-[360px]',
        workExpanded ? 'sm:right-[328px]' : compactUtilityLaunchers ? 'sm:right-16' : 'sm:right-[148px]',
      )}
    >
      <header className="flex shrink-0 items-center gap-2 border-b border-line bg-slate-50 px-3 py-2">
        <MessageSquarePlus className="h-4 w-4 shrink-0 text-blue-600" />
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-semibold text-ink">Mở trao đổi</span>
          <span className="block text-[11px] text-slate-500">Tối đa {maxDiscussionTabs} công trình trong một phiên</span>
        </span>
        <button type="button" className="rounded-md p-2 text-slate-500 hover:bg-slate-200" onClick={onClose} aria-label="Đóng danh sách trao đổi"><X className="h-4 w-4" /></button>
      </header>
      <label className="relative block shrink-0 border-b border-line p-2">
        <Search className="pointer-events-none absolute left-5 top-4 h-4 w-4 text-slate-400" />
        <input autoFocus className="w-full rounded-md border border-line py-2 pl-9 pr-3 text-sm outline-none focus:border-blue-300 focus:ring-2 focus:ring-blue-100" value={pickerSearch} onChange={(event) => onPickerSearch(event.target.value)} placeholder="Tìm mã hoặc tên công trình…" />
      </label>
      <div className="min-h-0 flex-1 overflow-y-auto p-1">
        {searchResults.isLoading ? <UtilityLoading label="Đang tìm công trình…" /> : null}
        {searchResults.isError ? <UtilityError label="Không tải được danh sách công trình." onRetry={() => void searchResults.refetch()} /> : null}
        {!searchResults.isLoading && !searchResults.isError && !items.length ? <p className="px-3 py-6 text-center text-xs text-slate-500">Không tìm thấy công trình phù hợp.</p> : null}
        {items.map((project) => {
          const opened = openProjectIDs.includes(project.project_id);
          return (
            <button key={project.project_id} type="button" className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left hover:bg-slate-50" onClick={() => onSelectProject(project)}>
              <span className="min-w-0 flex-1"><span className="block truncate text-sm font-semibold text-ink">{project.project_code} · {project.project_name}</span><span className="block truncate text-xs text-slate-500">{opened ? 'Đang mở · nhấn để hiện khung' : project.latest_actor_name ? `${project.latest_actor_name}: ${project.latest_comment || 'Trao đổi mới'}` : 'Chưa có trao đổi'}</span></span>
              {project.unread_count > 0 ? <CountBadge value={project.unread_count} highlight /> : null}
            </button>
          );
        })}
      </div>
      {openProjectIDs.length ? (
        <button type="button" className="shrink-0 border-t border-line px-3 py-2 text-xs font-semibold text-slate-600 hover:bg-slate-50 hover:text-rose-700" onClick={onCloseAll}>
          Đóng tất cả khung trao đổi
        </button>
      ) : null}
    </section>
  );
}

function ProjectConversation({ project }: { project: ProjectDiscussionProject }) {
  return <ActivityTopicProvider projectId={project.project_id}><ProjectConversationContent project={project} /></ActivityTopicProvider>;
}

function ProjectConversationContent({ project }: { project: ProjectDiscussionProject }) {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const toast = useToast();
  const [replyTo, setReplyTo] = useState<WorkItemActivity | null>(null);
  const projectID = project.project_id;
  const feed = useQuery({
    queryKey: ['utility-project-activity', projectID],
    queryFn: () => get<ProjectActivityFeed>(`/projects/${projectID}/activity?kind=discussion&limit=30`),
    retry: false,
  });
  const options = useQuery({
    queryKey: ['project-activity-options', projectID],
    queryFn: () => get<ProjectActivityOptions>(`/projects/${projectID}/activity/options`),
    enabled: Boolean(project.can_post),
    retry: false,
  });
  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ['utility-project-activity', projectID] });
    void queryClient.invalidateQueries({ queryKey: ['project-activity', projectID] });
    void queryClient.invalidateQueries({ queryKey: ['project-discussions'] });
  };
  const addActivity = useMutation({
    mutationFn: (payload: ActivityPayload) => post<WorkItemActivity>(`/projects/${projectID}/activity`, payload),
    onSuccess: () => { invalidate(); setReplyTo(null); toast.success('Đã gửi trao đổi'); },
    onError: (error) => toast.error('Không gửi được trao đổi', error),
  });
  const markRead = useMutation({
    mutationFn: (activityID: string) => post(`/projects/${projectID}/activity/read-through`, { activity_id: activityID }),
    onSuccess: () => { invalidate(); toast.success('Đã đánh dấu trao đổi là đã đọc'); },
    onError: (error) => toast.error('Không cập nhật được trạng thái đã đọc', error),
  });

  const descendingRows = feed.data?.items ?? [];
  const rows = [...descendingRows].reverse();
  const newestActivityID = descendingRows[0]?.id;
  const firstUnreadID = rows.find((item) => item.is_discussion_unread)?.id;
  const targets: ActivityTargetOption[] = (options.data?.targets ?? []).map((item) => ({ scope_type: item.scope_type, scope_id: item.scope_id, label: item.label, work_item_id: item.work_item_id }));
  const attachments: ActivityAttachmentOption[] = options.data?.attachments ?? [];
  const mentions: ActivityMentionOption[] = [
    ...(options.data?.members ?? []).map((item) => ({ type: 'staff' as const, id: item.id, label: item.label, description: [item.position, item.department].filter(Boolean).join(' · ') })),
    ...(options.data?.roles ?? []).map((item) => ({ type: 'role' as const, id: item.id, label: item.label })),
  ];

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex shrink-0 items-center gap-2 border-b border-line px-3 py-2">
        <span className="min-w-0 flex-1"><span className="block truncate text-sm font-semibold text-ink sm:hidden">{project.project_code} · {project.project_name}</span><span className="block text-[11px] text-slate-500">{project.unread_count > 0 ? `${project.unread_count} tin mới` : 'Đã đọc đến tin mới nhất'}</span></span>
        {project.unread_count > 0 && newestActivityID ? (
          <button type="button" className="inline-flex shrink-0 items-center gap-1 rounded-md border border-blue-200 bg-blue-50 px-2 py-1.5 text-xs font-semibold text-blue-700 hover:bg-blue-100 disabled:opacity-60" disabled={markRead.isPending} onClick={() => markRead.mutate(newestActivityID)}>
            {markRead.isPending ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <CheckCheck className="h-3.5 w-3.5" />} Đã đọc
          </button>
        ) : null}
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto bg-slate-50 px-3 py-3">
        {feed.isLoading ? <UtilityLoading label="Đang tải trao đổi…" /> : null}
        {feed.isError ? <UtilityError label="Không tải được trao đổi công trình." onRetry={() => void feed.refetch()} /> : null}
        {!feed.isLoading && !feed.isError && !rows.length ? <p className="py-12 text-center text-sm text-slate-500">Chưa có trao đổi. Hãy gửi tin đầu tiên cho công trình này.</p> : null}
        <div className="space-y-3">
          {rows.map((item) => {
            const own = item.actor_id === user?.id;
            return (
              <div key={item.id}>
                {item.id === firstUnreadID ? <div className="mb-3 flex items-center gap-2"><span className="h-px flex-1 bg-blue-200" /><span className="text-[11px] font-semibold uppercase tracking-wide text-blue-700">Tin mới</span><span className="h-px flex-1 bg-blue-200" /></div> : null}
                <article className={clsx('flex items-end gap-2', own && 'flex-row-reverse')}>
                  {own && user ? <UserAvatar user={user} size="xs" decorative /> : <span className="grid h-7 w-7 shrink-0 place-items-center rounded-full bg-slate-200 text-[10px] font-bold text-slate-600">{actorInitials(item.actor_name)}</span>}
                  <div className={clsx('max-w-[82%]', own && 'text-right')}>
                    <p className="mb-1 px-1 text-[11px] text-slate-500">{identityReferenceText(item.actor_name, item.actor_account_deleted, 'Hệ thống')} · {compactDateTime(item.created_at)}</p>
                    <div className={clsx('rounded-xl border px-3 py-2 text-left text-sm shadow-sm', chatBubbleTone(item.activity_type, own))}>
                      <ActivityReplyQuote replyTo={item.reply_to} />
                      <p className={clsx('mb-1 text-[10px] font-semibold uppercase tracking-wide', own ? 'text-blue-100' : 'text-slate-400')}>{activityLabel(item.activity_type)}</p>
                      {!item.withdrawn_at && item.activity_type !== 'work_update' ? <ActivityTopicCard source={item} /> : null}
                      {!item.withdrawn_at && item.activity_type === 'work_update' ? <DiscussionWorkUpdateCard item={item} /> : null}
                      <p className="whitespace-pre-wrap break-words leading-5">{item.withdrawn_at ? 'Tin nhắn đã được thu hồi' : item.comment || 'Không có nội dung'}</p>
                      {activityVisibilityLabel(item.visibility) ? <p className={clsx('mt-1 text-[11px]', own ? 'text-blue-100' : 'text-slate-500')}>{activityVisibilityLabel(item.visibility)}</p> : null}
                      <ActivityAttachments attachments={item.attachments} compact />
                    </div>
                    {!item.withdrawn_at && project.can_post ? <button type="button" className="mt-1 inline-flex items-center gap-1 px-1 text-[11px] font-semibold text-slate-500 hover:text-blue-700" onClick={() => setReplyTo(item)}><Reply className="h-3 w-3" /> Trả lời</button> : null}
                  </div>
                </article>
              </div>
            );
          })}
        </div>
      </div>
      {project.can_post ? (
        <div className="shrink-0 border-t border-line bg-white p-3">
          <ActivityComposer
            mode="project"
            allowTarget
            variant="chat"
            density="compact"
            targetOptions={targets}
            attachmentOptions={attachments}
            mentionOptions={mentions}
            mentionOptionsLoading={options.isLoading}
            mentionOptionsError={options.isError}
            onRetryMentionOptions={() => void options.refetch()}
            replyTo={replyTo}
            onCancelReply={() => setReplyTo(null)}
            submitting={addActivity.isPending}
            onSubmit={(payload) => addActivity.mutateAsync(payload).then(() => undefined)}
          />
        </div>
      ) : <p className="shrink-0 border-t border-line bg-amber-50 px-3 py-2 text-xs text-amber-800">Anh có quyền xem nhưng không có quyền gửi trao đổi trong công trình này.</p>}
      <Link className="flex shrink-0 items-center justify-center gap-1 border-t border-line px-3 py-2 text-xs font-semibold text-blue-700 hover:bg-blue-50" to={`/projects/${encodeURIComponent(projectID)}/discussion`}>Mở trao đổi đầy đủ <ArrowRight className="h-3.5 w-3.5" /></Link>
    </div>
  );
}

function workEntryKey(entry: WorkListEntry) {
  if (entry.kind === 'accounting_handoff') return `accounting:${entry.item.handoff_id}`;
  if (entry.kind === 'project_startup') return `startup:${entry.item.project_id}`;
  if (entry.kind === 'workflow_action') return `workflow:${entry.item.id}`;
  return `work:${entry.item.id}`;
}

function workEntryPresentation(entry: WorkListEntry) {
  if (entry.kind === 'project_startup') {
    return { title: 'Hoàn tất thiết lập công trình', project: `${entry.item.project_code} · ${entry.item.project_name}`, action: entry.item.ready_to_start ? 'Sẵn sàng khởi động' : `${entry.item.blockers.length} điều kiện còn thiếu`, meta: entry.item.owner_name, tone: 'bg-amber-100 text-amber-700', icon: <Sparkles className="h-4 w-4" /> };
  }
  if (entry.kind === 'workflow_action') {
    return { title: entry.item.title, project: `${entry.item.project_code} · ${entry.item.project_name}`, action: entry.item.action_label, meta: entry.item.run_node_name, tone: 'bg-blue-100 text-blue-700', icon: <ArrowRight className="h-4 w-4" /> };
  }
  if (entry.kind === 'accounting_handoff') {
    return accountingPresentation(entry.item);
  }
  return { title: entry.item.title, project: [entry.item.project_code, entry.item.project_name].filter(Boolean).join(' · ') || 'Công trình', action: statusLabel(entry.item.status), meta: [priorityLabel(entry.item.priority), entry.item.due_at ? `Hạn ${compactDateTime(entry.item.due_at)}` : ''].filter(Boolean).join(' · '), tone: entry.item.status === 'blocked' ? 'bg-rose-100 text-rose-700' : 'bg-slate-100 text-slate-700', icon: entry.item.status === 'blocked' ? <AlertCircle className="h-4 w-4" /> : <BriefcaseBusiness className="h-4 w-4" /> };
}

function accountingPresentation(item: AccountingWorkItem) {
  return { title: item.source_title || item.source_code, project: `${item.project_code} · ${item.project_name}`, action: item.status === 'returned' ? 'Bị trả lại · cần bổ sung' : 'Tiếp nhận hồ sơ kế toán', meta: item.counterparty_name, tone: item.status === 'returned' ? 'bg-rose-100 text-rose-700' : 'bg-violet-100 text-violet-700', icon: <BriefcaseBusiness className="h-4 w-4" /> };
}

function panelTabClass(active: boolean) {
  return clsx('inline-flex items-center gap-1.5 rounded-md px-2.5 py-2 text-xs font-semibold', active ? 'bg-white text-blue-700 shadow-sm ring-1 ring-slate-200' : 'text-slate-600 hover:bg-white/70');
}

function CountBadge({ value, highlight = false }: { value: number; highlight?: boolean }) {
  return <span className={clsx('min-w-5 rounded-full px-1.5 text-center text-[11px] font-bold leading-5', highlight ? 'bg-blue-600 text-white' : 'bg-slate-100 text-slate-700')}>{compactCount(value)}</span>;
}

function compactCount(value: number) {
  return value > 99 ? '99+' : value;
}

function UtilityLoading({ label }: { label: string }) {
  return <div className="flex items-center justify-center gap-2 px-4 py-10 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" /> {label}</div>;
}

function UtilityError({ label, onRetry }: { label: string; onRetry: () => void }) {
  return <div className="m-3 rounded-md border border-rose-200 bg-rose-50 px-3 py-3 text-xs text-rose-800"><p className="font-semibold">{label}</p><button type="button" className="mt-2 inline-flex items-center gap-1 font-semibold underline underline-offset-2" onClick={onRetry}><RefreshCw className="h-3.5 w-3.5" /> Thử lại</button></div>;
}
