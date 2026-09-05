import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { ChevronUp, Link2, Loader2, MessageSquarePlus, Plus, Send, X } from 'lucide-react';
import clsx from 'clsx';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { useToast } from '../../components/ToastProvider';
import type { ActivityAttachmentPayload, ActivityMentionPayload, ActivityPayload, WorkItemActivity } from '../../lib/types/domain';
import { enumLabel, statusLabel } from '../../lib/utils/status';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';
import { compactDateTime } from './workItemPresentation';
import { ActivityChatInput } from './ActivityChatInput';
import { ActivityTopicCard } from './ActivityTopic';
import { activityVisibilityLabel } from './activityTopicDomain';
import type { ActivityMentionOption } from './activityChatInputDomain';
import { chatMentionPayload } from './activityChatInputDomain';
export type { ActivityMentionOption } from './activityChatInputDomain';

type ComposerMode = 'project' | 'work_item';
type ActivityType = ActivityPayload['activity_type'];
type Visibility = ActivityPayload['visibility'];
type MentionType = ActivityMentionPayload['type'];
type AttachmentType = ActivityAttachmentPayload['type'];

export type ActivityTargetOption = {
  scope_type: string;
  scope_id: string;
  label: string;
  work_item_id?: string;
};

export type ActivityAttachmentOption = {
  type: AttachmentType;
  id: string;
  label: string;
  status?: string;
  visibility?: string;
};

const baseActivityOptions: Array<{ value: ActivityType; label: string }> = [
  { value: 'comment', label: enumLabel('activity_type', 'comment') },
  { value: 'status', label: enumLabel('activity_type', 'status') },
  { value: 'blocker', label: enumLabel('activity_type', 'blocker') },
  { value: 'question', label: enumLabel('activity_type', 'question') },
  { value: 'decision', label: enumLabel('activity_type', 'decision') },
  { value: 'reminder', label: enumLabel('activity_type', 'reminder') },
  { value: 'file_note', label: enumLabel('activity_type', 'file_note') },
];

const workItemOnlyOptions: Array<{ value: ActivityType; label: string }> = [];

const visibilityOptions: Array<{ value: Visibility; label: string }> = [
  { value: 'project', label: 'Thành viên công trình' },
  { value: 'internal', label: 'Nội bộ' },
  { value: 'client', label: 'Chủ đầu tư' },
];

const targetScopeOptions = [
  { value: 'project', label: enumLabel('scope_type', 'project') },
  { value: 'work_item', label: enumLabel('scope_type', 'work_item') },
  { value: 'boq_item', label: enumLabel('scope_type', 'boq_item') },
  { value: 'procurement_request', label: enumLabel('scope_type', 'procurement_request') },
  { value: 'stock_movement', label: enumLabel('scope_type', 'stock_movement') },
  { value: 'acceptance', label: enumLabel('scope_type', 'acceptance') },
  { value: 'rework', label: enumLabel('scope_type', 'rework') },
  { value: 'handover', label: enumLabel('scope_type', 'handover') },
  { value: 'payable', label: enumLabel('scope_type', 'payable') },
  { value: 'receivable', label: enumLabel('scope_type', 'receivable') },
  { value: 'payment', label: enumLabel('scope_type', 'payment') },
];

const mentionTypeOptions: Array<{ value: MentionType; label: string }> = [
  { value: 'staff', label: enumLabel('mention_type', 'staff') },
  { value: 'role', label: enumLabel('mention_type', 'role') },
];

const attachmentTypeOptions: Array<{ value: AttachmentType; label: string }> = [
  { value: 'evidence', label: enumLabel('attachment_type', 'evidence') },
  { value: 'document', label: enumLabel('attachment_type', 'document') },
];

function useDismissableDisclosure() {
  const detailsRef = useRef<HTMLDetailsElement | null>(null);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!open) return undefined;

    const handlePointerDown = (event: PointerEvent) => {
      if (event.target instanceof Node && !detailsRef.current?.contains(event.target)) {
        setOpen(false);
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      event.stopPropagation();
      setOpen(false);
    };

    document.addEventListener('pointerdown', handlePointerDown);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('pointerdown', handlePointerDown);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [open]);

  return {
    detailsRef,
    open,
    toggle: () => setOpen((current) => !current),
  };
}

export function ActivityComposer({
  mode,
  submitting,
  allowTarget = false,
  targetOptions = [],
  attachmentOptions = [],
  mentionOptions = [],
  mentionOptionsLoading = false,
  mentionOptionsError = false,
  onRetryMentionOptions,
  defaultExpanded = false,
  variant = 'card',
  density = 'comfortable',
  replyTo,
  editItem,
  onCancelReply,
  onCancelEdit,
  onSubmitEdit,
  onSubmit,
}: {
  mode: ComposerMode;
  submitting?: boolean;
  allowTarget?: boolean;
  targetOptions?: ActivityTargetOption[];
  attachmentOptions?: ActivityAttachmentOption[];
  mentionOptions?: ActivityMentionOption[];
  mentionOptionsLoading?: boolean;
  mentionOptionsError?: boolean;
  onRetryMentionOptions?: () => void;
  defaultExpanded?: boolean;
  variant?: 'card' | 'chat';
  density?: 'comfortable' | 'compact';
  replyTo?: WorkItemActivity | null;
  editItem?: WorkItemActivity | null;
  onCancelReply?: () => void;
  onCancelEdit?: () => void;
  onSubmitEdit?: (comment: string) => Promise<void>;
  onSubmit: (payload: ActivityPayload) => Promise<void>;
}) {
  const toast = useToast();
	const [expanded, setExpanded] = useState(defaultExpanded);
  const [activityType, setActivityType] = useState<ActivityType>('comment');
  const [visibility, setVisibility] = useState<Visibility>('project');
  const [comment, setComment] = useState('');
  const [dueAt, setDueAt] = useState('');
  const [ownerId, setOwnerId] = useState('');
  const [referenceId, setReferenceId] = useState('');
  const [referenceType, setReferenceType] = useState<AttachmentType>('evidence');
  const [referenceLabel, setReferenceLabel] = useState('');
  const [targetScope, setTargetScope] = useState('project');
  const [targetId, setTargetId] = useState('');
  const [mentionType, setMentionType] = useState<MentionType>('staff');
  const [mentionId, setMentionId] = useState('');
  const [mentionLabel, setMentionLabel] = useState('');
  const [mentions, setMentions] = useState<ActivityMentionPayload[]>([]);
  const [attachments, setAttachments] = useState<ActivityAttachmentPayload[]>([]);
  const [touched, setTouched] = useState(false);
  const formRef = useRef<HTMLFormElement | null>(null);
  const targetDisclosure = useDismissableDisclosure();
  const submissionInFlight = useRef(false);
  const [sending, setSending] = useState(false);
  const navigateToValidation = useValidationNavigation(formRef);

  const activityOptions = useMemo(() => {
    if (mode === 'work_item') return [...baseActivityOptions, ...workItemOnlyOptions];
    return baseActivityOptions;
  }, [mode]);

  const editing = variant === 'chat' && Boolean(editItem);
  const replying = variant === 'chat' && !editing && Boolean(replyTo);
  const lockedContext = replying || editing;
  const editCommentChanged = editing && comment.trim() !== (editItem?.comment ?? '').trim();
  const effectiveActivityType: ActivityType = lockedContext ? 'comment' : activityType;
  const needsDueAt = effectiveActivityType === 'reminder';
  const showsDueAt = effectiveActivityType === 'reminder' || effectiveActivityType === 'blocker';
  const needsReference = effectiveActivityType === 'file_note';
  const needsTargetId = !lockedContext && allowTarget && targetScope !== 'project';
  const missingComment = touched && !comment.trim();
  const missingDueAt = touched && needsDueAt && !dueAt;
  const missingReference = touched && needsReference && !attachments.length;
  const missingTarget = touched && needsTargetId && !targetId.trim();
  const visibleTargetOptions = targetOptions.filter((option) => option.scope_type === targetScope);
  const availableTargetScopeOptions = targetScopeOptions.filter((option) => option.value === 'project' || targetOptions.some((target) => target.scope_type === option.value));
  const visibleAttachmentOptions = attachmentOptions.filter((option) => option.type === referenceType);
  const visibleMentionOptions = mentionOptions.filter((option) => option.type === mentionType);
  const selectedTargetOption = targetOptions.find((option) => option.scope_type === targetScope && option.scope_id === targetId);
  const targetSummary = targetScope === 'project'
    ? enumLabel('scope_type', 'project')
    : selectedTargetOption?.label ?? targetScopeOptions.find((option) => option.value === targetScope)?.label ?? 'Chọn đối tượng';
  const topicPreview = <ActivityTopicCard source={editing ? editItem! : replying ? replyTo! : selectedTargetOption ?? {}} />;

  useEffect(() => {
    if (!editItem) return;
    setComment(editItem.comment ?? '');
    setMentions([]);
    setTouched(false);
  }, [editItem?.id]);

  function addMention() {
    const id = mentionId.trim();
    if (!id) {
      toast.warn({ title: 'Thiếu người được nhắc', description: mentionType === 'role' ? 'Nhập nhãn vai trò cần nhắc.' : 'Nhập ID cần nhắc.' });
      navigateToValidation('mention');
      return;
    }
    const next = { type: mentionType, id, label: mentionLabel.trim() || undefined };
    setMentions((current) => {
      if (current.some((mention) => mention.type === next.type && mention.id.toLowerCase() === next.id.toLowerCase())) return current;
      return [...current, next];
    });
    setMentionId('');
    setMentionLabel('');
  }

  function addAttachment() {
    const id = referenceId.trim();
    if (!id) {
      toast.warn({ title: 'Thiếu tệp đính kèm', description: 'Cần chọn hoặc nhập ID minh chứng/tài liệu.' });
      navigateToValidation('reference');
      return;
    }
    const selected = attachmentOptions.find((option) => option.type === referenceType && option.id === id);
    const next = { type: referenceType, id, label: referenceLabel.trim() || selected?.label };
    setAttachments((current) => {
      if (current.some((attachment) => attachment.type === next.type && attachment.id === next.id)) return current;
      return [...current, next];
    });
    setReferenceId('');
    setReferenceLabel('');
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting || submissionInFlight.current) return;
    setTouched(true);
    const missing = [
      !comment.trim() ? 'nội dung' : '',
      needsDueAt && !dueAt ? 'ngày nhắc' : '',
      needsReference && !attachments.length ? 'minh chứng/tài liệu' : '',
      needsTargetId && !targetId.trim() ? 'bản ghi đích' : '',
    ].filter(Boolean);
    if (missing.length) {
      toast.warn({ title: 'Thiếu thông tin hoạt động', description: `Cần nhập: ${missing.join(', ')}` });
      navigateToValidation(
        !comment.trim()
          ? 'comment'
          : needsDueAt && !dueAt
            ? 'due_at'
            : needsReference && !attachments.length
              ? 'reference'
              : 'target_id',
      );
      return;
    }

    if (editing && (!editCommentChanged || !onSubmitEdit)) return;

    submissionInFlight.current = true;
    setSending(true);

    if (editing && onSubmitEdit) {
      try {
        await onSubmitEdit(comment.trim());
      } catch {
        return;
      } finally {
        submissionInFlight.current = false;
        setSending(false);
      }
      setComment('');
      setTouched(false);
      return;
    }

    const metadata: Record<string, string> = {};
    if (dueAt) metadata.due_at = dueAt;
    if (ownerId.trim()) metadata.owner_id = ownerId.trim();
    const selectedTarget = targetOptions.find((option) => option.scope_type === targetScope && option.scope_id === targetId.trim());

    try {
      await onSubmit({
        activity_type: effectiveActivityType,
        comment: comment.trim(),
        visibility: replying ? (replyTo?.visibility as Visibility ?? 'project') : variant === 'chat' ? 'project' : visibility,
        scope_type: replying ? replyTo?.scope_type : allowTarget ? targetScope : undefined,
        scope_id: replying ? replyTo?.scope_id : allowTarget && targetScope !== 'project' ? targetId.trim() : undefined,
        work_item_id: replying ? replyTo?.work_item_id : selectedTarget?.work_item_id,
        parent_activity_id: replying ? replyTo?.id : undefined,
        mentions: variant === 'chat' ? chatMentionPayload(comment, mentions) : mentions,
        attachments: replying ? [] : attachments,
        metadata: replying ? {} : metadata,
      });
    } catch {
      return;
    } finally {
      submissionInFlight.current = false;
      setSending(false);
    }
    setComment('');
    setDueAt('');
    setOwnerId('');
    setReferenceId('');
    setReferenceLabel('');
    setTargetScope('project');
    setTargetId('');
    setMentions([]);
    setAttachments([]);
    setTouched(false);
    if (variant === 'card') setExpanded(false);
  }

  const chatInput = (
    <ActivityChatInput
      key={editing ? `edit:${editItem?.id}` : 'compose'}
      value={comment}
      onChange={(value, selectedMentions) => { setComment(value); if (!editing) setMentions(selectedMentions); }}
      options={mentionOptions}
      optionsLoading={mentionOptionsLoading}
      optionsError={mentionOptionsError}
      onRetryOptions={onRetryMentionOptions}
      activityType={activityType}
      activityOptions={activityOptions}
      onActivityTypeChange={setActivityType}
      compact={density === 'compact'}
      editing={editing}
      replying={replying}
      editUnchanged={Boolean(editing && !editCommentChanged)}
      busy={Boolean(submitting || sending)}
      invalid={missingComment}
      onBlur={() => setTouched(true)}
      placeholder={editing ? 'Nhập nội dung chỉnh sửa...' : replying ? `Trả lời ${identityReferenceText(replyTo?.actor_name, replyTo?.actor_account_deleted, 'thành viên công trình')}...` : 'Nhắn vào thảo luận công trình...'}
    />
  );

  if (variant === 'chat' && density === 'compact') {
    return (
      <form ref={formRef} className="space-y-1.5" onSubmit={submit} noValidate>
        {editing && editItem ? (
          <div className="flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-2.5 py-1.5 text-left">
            <div className="min-w-0 flex-1">
              <p className="truncate text-xs font-semibold text-amber-800">Đang chỉnh sửa · {compactDateTime(editItem.created_at)}</p>
              <p className="truncate text-[11px] text-slate-500">Phạm vi và đối tượng không đổi</p>
            </div>
            <button type="button" className="rounded-md p-1 text-slate-500 hover:bg-white" onClick={onCancelEdit} aria-label="Hủy chỉnh sửa"><X className="h-3.5 w-3.5" /></button>
          </div>
        ) : null}
        {replying && replyTo ? (
          <div className="flex items-start gap-2 rounded-lg border border-blue-100 bg-blue-50 px-2.5 py-1.5 text-left">
            <div className="min-w-0 flex-1">
              <p className="truncate text-xs font-semibold text-blue-800">Trả lời {identityReferenceText(replyTo.actor_name, replyTo.actor_account_deleted, 'thành viên công trình')}</p>
              <p className="line-clamp-1 text-[11px] text-slate-600">{replyTo.comment || activityLabelForReply(replyTo.activity_type)}</p>
            </div>
            <button type="button" className="rounded-md p-1 text-slate-500 hover:bg-white" onClick={onCancelReply} aria-label="Hủy trả lời"><X className="h-3.5 w-3.5" /></button>
          </div>
        ) : null}

        <div
          data-validation-key="comment"
          data-validation-error={missingComment ? 'true' : undefined}
          className={clsx(
            'rounded-xl border bg-white shadow-sm focus-within:border-blue-300 focus-within:ring-2 focus-within:ring-blue-100',
            missingComment ? 'border-amber-300' : 'border-line',
          )}
        >
          {!lockedContext && allowTarget ? (
            <div className="flex min-h-8 items-center border-b border-line px-2">
              <details
                ref={targetDisclosure.detailsRef}
                open={targetDisclosure.open}
                data-validation-key={targetScope !== 'project' ? 'target_id' : undefined}
                data-validation-error={missingTarget ? 'true' : undefined}
                className="group relative min-w-0"
              >
                <summary
                  className={clsx(
                    'flex cursor-pointer list-none items-center gap-1.5 rounded-md px-1.5 py-1 text-[11px] font-semibold hover:bg-slate-50',
                    missingTarget ? 'text-amber-700' : 'text-slate-500',
                  )}
                  aria-expanded={targetDisclosure.open}
                  onClick={(event) => {
                    event.preventDefault();
                    targetDisclosure.toggle();
                  }}
                >
                  <Link2 className="h-3.5 w-3.5 shrink-0" />
                  <span className="truncate">Gắn vào: {targetSummary}</span>
                  <span aria-hidden="true" className="text-[9px] text-slate-400">▼</span>
                </summary>
                <div className="absolute bottom-full left-0 z-30 mb-1 w-[min(20rem,calc(100vw-2rem))] space-y-2 rounded-lg border border-line bg-white p-2 shadow-xl">
                  <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">Gắn vào</p>
                  <select
                    className="w-full rounded-md border border-line bg-white px-2.5 py-2 text-xs font-semibold text-slate-600"
                    value={targetScope}
                    onChange={(event) => {
                      setTargetScope(event.target.value);
                      setTargetId('');
                    }}
                  >
                    {availableTargetScopeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                  </select>
                  {targetScope !== 'project' ? (
                    visibleTargetOptions.length ? (
                      <select
                        className={clsx(
                          'w-full rounded-md border bg-white px-2.5 py-2 text-xs font-semibold text-slate-600',
                          missingTarget ? 'border-amber-300 bg-amber-50' : 'border-line',
                        )}
                        value={targetId}
                        aria-invalid={missingTarget}
                        onChange={(event) => setTargetId(event.target.value)}
                      >
                        <option value="">Chọn đối tượng</option>
                        {visibleTargetOptions.map((option) => <option key={`${option.scope_type}:${option.scope_id}`} value={option.scope_id}>{option.label}</option>)}
                      </select>
                    ) : <p className="rounded-md border border-dashed border-line bg-slate-50 px-2.5 py-2 text-xs text-slate-500">Chưa có đối tượng phù hợp</p>
                  ) : null}
                </div>
              </details>
            </div>
          ) : null}

          {!lockedContext && (showsDueAt || effectiveActivityType === 'decision' || needsReference) ? (
            <div className="grid gap-1.5 border-b border-line bg-slate-50 p-1.5 sm:grid-cols-2">
              {showsDueAt ? (
                <label data-validation-key="due_at" data-validation-error={missingDueAt ? 'true' : undefined} className="block rounded-md">
                  <span className="sr-only">{activityType === 'reminder' ? 'Ngày nhắc' : 'Hạn điểm chặn'}</span>
                  <input
                    className={clsx('h-8 w-full rounded-md border bg-white px-2 text-xs text-slate-600', missingDueAt ? 'border-amber-300 bg-amber-50' : 'border-line')}
                    type="date"
                    aria-invalid={missingDueAt}
                    value={dueAt}
                    onChange={(event) => setDueAt(event.target.value)}
                    title={activityType === 'reminder' ? 'Ngày nhắc' : 'Hạn điểm chặn'}
                  />
                </label>
              ) : null}
              {effectiveActivityType === 'blocker' || effectiveActivityType === 'decision' ? (
                <label className="block">
                  <span className="sr-only">Người phụ trách</span>
                  <select className="h-8 w-full rounded-md border border-line bg-white px-2 text-xs text-slate-600" value={ownerId} onChange={(event) => setOwnerId(event.target.value)}>
                    <option value="">Phụ trách: Chưa chỉ định</option>
                    {mentionOptions.filter((option) => option.type === 'staff').map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}
                  </select>
                </label>
              ) : null}
              {needsReference ? (
                <div data-validation-key="reference" data-validation-error={missingReference ? 'true' : undefined} className={clsx('grid gap-1.5 rounded-md sm:col-span-2 sm:grid-cols-[100px_minmax(0,1fr)_32px]', missingReference && 'ring-1 ring-amber-300')}>
                  <select className="h-8 rounded-md border border-line bg-white px-2 text-xs text-slate-600" value={referenceType} onChange={(event) => { setReferenceType(event.target.value as AttachmentType); setReferenceId(''); setReferenceLabel(''); }} aria-label="Loại tệp đính kèm">
                    {attachmentTypeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                  </select>
                  <select className="h-8 min-w-0 rounded-md border border-line bg-white px-2 text-xs text-slate-600" value={referenceId} onChange={(event) => { const selected = visibleAttachmentOptions.find((option) => option.id === event.target.value); setReferenceId(event.target.value); setReferenceLabel(selected?.label ?? ''); }} aria-label="Chọn tệp đính kèm">
                    <option value="">Chọn tệp đính kèm</option>
                    {visibleAttachmentOptions.map((option) => <option key={`${option.type}:${option.id}`} value={option.id}>{option.label}</option>)}
                  </select>
                  <button type="button" className="inline-flex h-8 items-center justify-center rounded-md border border-line bg-white text-slate-600 hover:bg-slate-100" onClick={addAttachment} aria-label="Thêm tệp đính kèm"><Plus className="h-3.5 w-3.5" /></button>
                </div>
              ) : null}
            </div>
          ) : null}

          {!editing && attachments.length > 0 ? (
            <div className="flex flex-wrap gap-1 border-b border-line px-2 py-1.5">
              {attachments.map((attachment) => (
                <span key={`${attachment.type}:${attachment.id}`} className="inline-flex max-w-full items-center gap-1 rounded-md bg-emerald-50 px-1.5 py-1 text-[11px] font-semibold text-emerald-700">
                  <span className="truncate">{attachment.label || attachment.id}</span>
                  <button type="button" className="rounded p-0.5 hover:bg-emerald-100" onClick={() => setAttachments((current) => current.filter((item) => !(item.type === attachment.type && item.id === attachment.id)))} aria-label="Gỡ tệp đính kèm"><X className="h-3 w-3" /></button>
                </span>
              ))}
            </div>
          ) : null}

          <div className="px-2">{topicPreview}</div>
          {chatInput}
        </div>
      </form>
    );
  }

  if (variant === 'chat') {
    return (
      <form ref={formRef} className="space-y-2" onSubmit={submit} noValidate>
        {editing && editItem ? (
          <div className="flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-left">
            <div className="min-w-0 flex-1">
              <p className="text-xs font-semibold text-amber-800">Đang chỉnh sửa tin nhắn · {compactDateTime(editItem.created_at)}</p>
              {activityVisibilityLabel(editItem.visibility) ? <p className="mt-0.5 text-[11px] text-slate-500">{activityVisibilityLabel(editItem.visibility)}</p> : null}
            </div>
            <button type="button" className="rounded-md p-1 text-slate-500 hover:bg-white" onClick={onCancelEdit} aria-label="Hủy chỉnh sửa"><X className="h-4 w-4" /></button>
          </div>
        ) : null}
        {replying && replyTo ? (
          <div className="flex items-start gap-2 rounded-lg border border-blue-100 bg-blue-50 px-3 py-2 text-left">
            <div className="min-w-0 flex-1">
              <p className="text-xs font-semibold text-blue-800">Đang trả lời {identityReferenceText(replyTo.actor_name, replyTo.actor_account_deleted, 'thành viên công trình')}</p>
              <p className="mt-0.5 line-clamp-2 text-xs leading-5 text-slate-600">{replyTo.comment || activityLabelForReply(replyTo.activity_type)}</p>
              {activityVisibilityLabel(replyTo.visibility) ? <p className="mt-0.5 text-[11px] text-slate-500">{activityVisibilityLabel(replyTo.visibility)}</p> : null}
            </div>
            <button type="button" className="rounded-md p-1 text-slate-500 hover:bg-white" onClick={onCancelReply} aria-label="Hủy trả lời"><X className="h-4 w-4" /></button>
          </div>
        ) : null}

        {!lockedContext && (allowTarget || showsDueAt || effectiveActivityType === 'decision') ? (
          <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
            {allowTarget ? (
              <>
                <label className="block">
                  <span className="sr-only">Gắn vào</span>
                  <select
                    className="w-full rounded-md border border-line bg-white px-2.5 py-2 text-xs font-semibold text-slate-600"
                    value={targetScope}
                    onChange={(event) => {
                      setTargetScope(event.target.value);
                      setTargetId('');
                    }}
                  >
                    {availableTargetScopeOptions.map((option) => <option key={option.value} value={option.value}>Gắn vào: {option.label}</option>)}
                  </select>
                </label>
                {targetScope !== 'project' ? (
                  <label data-validation-key="target_id" data-validation-error={missingTarget ? 'true' : undefined} className="block scroll-mt-24 rounded-md">
                    <span className="sr-only">Đối tượng được gắn</span>
                    <select
                      className={clsx(
                        'w-full rounded-md border bg-white px-2.5 py-2 text-xs font-semibold text-slate-600',
                        missingTarget ? 'border-amber-300 bg-amber-50' : 'border-line',
                      )}
                      value={targetId}
                      aria-invalid={missingTarget}
                      onChange={(event) => setTargetId(event.target.value)}
                    >
                      <option value="">Chọn đối tượng</option>
                      {visibleTargetOptions.map((option) => <option key={`${option.scope_type}:${option.scope_id}`} value={option.scope_id}>{option.label}</option>)}
                    </select>
                  </label>
                ) : null}
              </>
            ) : null}
            {showsDueAt ? (
              <label data-validation-key="due_at" data-validation-error={missingDueAt ? 'true' : undefined} className="block scroll-mt-24 rounded-md">
                <span className="sr-only">{activityType === 'reminder' ? 'Ngày nhắc' : 'Hạn điểm chặn'}</span>
                <input
                  className={clsx(
                    'w-full rounded-md border bg-white px-2.5 py-2 text-xs font-semibold text-slate-600',
                    missingDueAt ? 'border-amber-300 bg-amber-50' : 'border-line',
                  )}
                  type="date"
                  aria-invalid={missingDueAt}
                  value={dueAt}
                  onChange={(event) => setDueAt(event.target.value)}
                  title={activityType === 'reminder' ? 'Ngày nhắc' : 'Hạn điểm chặn'}
                />
              </label>
            ) : null}
            {effectiveActivityType === 'blocker' || effectiveActivityType === 'decision' ? (
              <label className="block">
                <span className="sr-only">Người phụ trách</span>
                <select
                  className="w-full rounded-md border border-line bg-white px-2.5 py-2 text-xs font-semibold text-slate-600"
                  value={ownerId}
                  onChange={(event) => setOwnerId(event.target.value)}
                >
                  <option value="">Người phụ trách: Chưa chỉ định</option>
                  {mentionOptions.filter((option) => option.type === 'staff').map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}
                </select>
              </label>
            ) : null}
          </div>
        ) : null}

        {needsReference ? (
          <div data-validation-key="reference" data-validation-error={missingReference ? 'true' : undefined} className={clsx('scroll-mt-24 grid gap-2 rounded-md border bg-slate-50 p-2 sm:grid-cols-[120px_minmax(0,1fr)_auto]', missingReference ? 'border-amber-300' : 'border-line')}>
            <select
              className="rounded-md border border-line bg-white px-2.5 py-2 text-xs font-semibold text-slate-600"
              value={referenceType}
              onChange={(event) => {
                setReferenceType(event.target.value as AttachmentType);
                setReferenceId('');
                setReferenceLabel('');
              }}
              aria-label="Loại tệp đính kèm"
            >
              {attachmentTypeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select>
            <select
              className="min-w-0 rounded-md border border-line bg-white px-2.5 py-2 text-xs font-semibold text-slate-600"
              value={referenceId}
              onChange={(event) => {
                const selected = visibleAttachmentOptions.find((option) => option.id === event.target.value);
                setReferenceId(event.target.value);
                setReferenceLabel(selected?.label ?? '');
              }}
              aria-label="Chọn tệp đính kèm"
            >
              <option value="">Chọn tệp đính kèm</option>
              {visibleAttachmentOptions.map((option) => <option key={`${option.type}:${option.id}`} value={option.id}>{option.label}</option>)}
            </select>
            <button type="button" className="inline-flex h-9 items-center justify-center rounded-md border border-line bg-white px-3 text-slate-600 hover:bg-slate-100" onClick={addAttachment} aria-label="Thêm tệp đính kèm">
              <Plus className="h-4 w-4" />
            </button>
          </div>
        ) : null}

        {!editing && attachments.length > 0 ? (
          <div className="flex flex-wrap gap-1.5">
            {attachments.map((attachment) => (
              <span key={`${attachment.type}:${attachment.id}`} className="inline-flex max-w-full items-center gap-1 rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700">
                <span className="truncate">{attachment.label || attachment.id}</span>
                <button type="button" className="rounded p-0.5 hover:bg-emerald-100" onClick={() => setAttachments((current) => current.filter((item) => !(item.type === attachment.type && item.id === attachment.id)))} aria-label="Gỡ tệp đính kèm"><X className="h-3 w-3" /></button>
              </span>
            ))}
          </div>
        ) : null}

        <div data-validation-key="comment" data-validation-error={missingComment ? 'true' : undefined} className={clsx('scroll-mt-24 rounded-xl border bg-white shadow-sm focus-within:border-blue-300 focus-within:ring-2 focus-within:ring-blue-100', missingComment ? 'border-amber-300' : 'border-line')}>
          <div className="px-2">{topicPreview}</div>
          {chatInput}
        </div>
      </form>
    );
  }

	if (!expanded) {
		return (
			<button
				type="button"
				className="flex w-full items-center gap-3 rounded-lg border border-line bg-white px-4 py-3 text-left shadow-sm transition hover:border-blue-200 hover:bg-blue-50/40"
				onClick={() => setExpanded(true)}
			>
				<span className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-blue-50 text-blue-700">
					<MessageSquarePlus className="h-4 w-4" />
				</span>
				<span className="min-w-0 flex-1 text-sm font-medium text-slate-500">Viết trao đổi, đặt câu hỏi hoặc ghi nhận quyết định...</span>
				<span className="hidden text-xs font-semibold text-blue-700 sm:inline">Tạo trao đổi</span>
			</button>
		);
	}

  return (
    <form ref={formRef} className="rounded-lg border border-blue-100 bg-blue-50/30 p-3 shadow-sm" onSubmit={submit} noValidate>
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-sm font-semibold text-ink">Tạo trao đổi</p>
          <p className="mt-0.5 text-xs text-slate-500">Chọn đúng loại để hệ thống ưu tiên và theo dõi phản hồi.</p>
        </div>
        <button type="button" className="rounded-md p-2 text-slate-500 hover:bg-white hover:text-ink" onClick={() => setExpanded(false)} aria-label="Thu gọn composer">
          <ChevronUp className="h-4 w-4" />
        </button>
      </div>

      <div className="mt-3 flex gap-1.5 overflow-x-auto pb-1">
        {activityOptions.map((option) => (
          <button
            key={option.value}
            type="button"
            className={clsx(
              'whitespace-nowrap rounded-md border px-3 py-1.5 text-xs font-semibold transition',
              activityType === option.value ? 'border-blue-200 bg-blue-600 text-white' : 'border-line bg-white text-slate-600 hover:border-blue-200',
            )}
            onClick={() => setActivityType(option.value)}
          >
            {option.label}
          </button>
        ))}
      </div>

      <div className="mt-2 grid gap-2 sm:grid-cols-2">
        <label className="block">
          <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Ai có thể xem</span>
          <select
            className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
            value={visibility}
            onChange={(event) => setVisibility(event.target.value as Visibility)}
          >
            {visibilityOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </select>
        </label>
      </div>

      {allowTarget ? (
        <div className="mt-2 grid gap-2 sm:grid-cols-2">
          <label className="block">
            <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Gắn vào</span>
            <select
              className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
              value={targetScope}
              onChange={(event) => {
                setTargetScope(event.target.value);
                setTargetId('');
              }}
            >
              {availableTargetScopeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select>
          </label>
          {targetScope !== 'project' ? (
            <label data-validation-key="target_id" data-validation-error={missingTarget ? 'true' : undefined} className="block scroll-mt-24 rounded-md">
              <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Đích</span>
              {visibleTargetOptions.length ? (
                <select
                  className={clsx(
                    'w-full rounded-md border bg-white px-3 py-2 text-sm font-medium text-slate-700',
                    missingTarget ? 'border-amber-300 bg-amber-50' : 'border-line',
                  )}
                  value={targetId}
                  aria-invalid={missingTarget}
                  onChange={(event) => setTargetId(event.target.value)}
                >
                  <option value="">Chọn đối tượng</option>
                  {visibleTargetOptions.map((option) => <option key={`${option.scope_type}:${option.scope_id}`} value={option.scope_id}>{option.label}</option>)}
                </select>
              ) : <div className="rounded-md border border-dashed border-line bg-slate-50 px-3 py-2 text-sm text-slate-500">Chưa có đối tượng phù hợp</div>}
            </label>
          ) : null}
        </div>
      ) : null}

      <textarea
        data-validation-key="comment"
        data-validation-error={missingComment ? 'true' : undefined}
        className={clsx(
          'mt-2 min-h-20 w-full rounded-md border bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500',
          missingComment ? 'border-amber-300 bg-amber-50' : 'border-line',
        )}
        value={comment}
        aria-invalid={missingComment}
        onBlur={() => setTouched(true)}
        onChange={(event) => setComment(event.target.value)}
        placeholder="Nhập nội dung hoạt động"
      />

      {showsDueAt || activityType === 'decision' || needsReference ? (
        <div className="mt-2 grid gap-2 sm:grid-cols-2">
          {showsDueAt ? (
            <label data-validation-key="due_at" data-validation-error={missingDueAt ? 'true' : undefined} className="block scroll-mt-24 rounded-md">
              <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">{activityType === 'reminder' ? 'Ngày nhắc' : 'Hạn điểm chặn'}</span>
              <input
                className={clsx(
                  'w-full rounded-md border bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500',
                  missingDueAt ? 'border-amber-300 bg-amber-50' : 'border-line',
                )}
                type="date"
                aria-invalid={missingDueAt}
                value={dueAt}
                onChange={(event) => setDueAt(event.target.value)}
              />
            </label>
          ) : null}
          {activityType === 'blocker' || activityType === 'decision' ? (
            <label className="block">
              <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Người phụ trách</span>
              <select
                className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
                value={ownerId}
                onChange={(event) => setOwnerId(event.target.value)}
              >
                <option value="">Chưa chỉ định</option>
                {mentionOptions.filter((option) => option.type === 'staff').map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}
              </select>
            </label>
          ) : null}
          {needsReference ? (
            <div data-validation-key="reference" data-validation-error={missingReference ? 'true' : undefined} className={clsx('scroll-mt-24 rounded-md border bg-white p-2 sm:col-span-2', missingReference ? 'border-amber-300 bg-amber-50' : 'border-line')}>
              <div className="grid gap-2 sm:grid-cols-[140px_minmax(0,1fr)_auto]">
                <label className="block">
                  <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Tệp đính kèm</span>
                  <select
                    className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
                    value={referenceType}
                    onChange={(event) => {
                      setReferenceType(event.target.value as AttachmentType);
                      setReferenceId('');
                      setReferenceLabel('');
                    }}
                  >
                    {attachmentTypeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                  </select>
                </label>
                <label className="block">
                  <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Bản ghi</span>
                  {visibleAttachmentOptions.length ? (
                    <select
                      className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
                      value={referenceId}
                      onChange={(event) => {
                        const selected = visibleAttachmentOptions.find((option) => option.id === event.target.value);
                        setReferenceId(event.target.value);
                        setReferenceLabel(selected?.label ?? '');
                      }}
                    >
                      <option value="">Chọn tệp đính kèm</option>
                      {visibleAttachmentOptions.map((option) => (
                        <option key={`${option.type}:${option.id}`} value={option.id}>
                          {option.label}{option.status ? ` · ${statusLabel(option.status)}` : ''}{option.visibility ? ` · ${enumLabel('visibility', option.visibility)}` : ''}
                        </option>
                      ))}
                    </select>
                  ) : <div className="rounded-md border border-dashed border-line bg-slate-50 px-3 py-2 text-sm text-slate-500">Chưa có tệp phù hợp</div>}
                </label>
                <button
                  className="mt-5 inline-flex h-10 items-center justify-center rounded-md border border-line px-3 text-sm font-semibold text-slate-700 hover:bg-slate-50"
                  type="button"
                  onClick={addAttachment}
                  aria-label="Thêm tệp đính kèm"
                >
                  <Plus className="h-4 w-4" />
                </button>
              </div>
              {attachments.length ? (
                <div className="mt-2 flex flex-wrap gap-1">
                  {attachments.map((attachment) => (
                    <span key={`${attachment.type}:${attachment.id}`} className="inline-flex max-w-full items-center gap-1 rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700">
                      <span className="truncate">{enumLabel('attachment_type', attachment.type)}: {attachment.label || attachment.id}</span>
                      <button
                        type="button"
                        className="rounded p-0.5 hover:bg-emerald-100"
                        onClick={() => setAttachments((current) => current.filter((item) => !(item.type === attachment.type && item.id === attachment.id)))}
                        aria-label="Gỡ tệp đính kèm"
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </span>
                  ))}
                </div>
              ) : null}
            </div>
          ) : null}
        </div>
      ) : null}

      <div data-validation-key="mention" className="mt-2 scroll-mt-24 rounded-md border border-line bg-white p-2">
        <div className="grid gap-2 sm:grid-cols-[140px_minmax(0,1fr)_auto]">
          <label className="block">
            <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Nhắc đến</span>
            <select
              className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
              value={mentionType}
              onChange={(event) => {
                setMentionType(event.target.value as MentionType);
                setMentionId('');
                setMentionLabel('');
              }}
            >
              {mentionTypeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select>
          </label>
          <label className="block">
            <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Người hoặc vai trò</span>
            <select
              className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
              value={mentionId}
              onChange={(event) => {
                const selected = visibleMentionOptions.find((option) => option.id === event.target.value);
                setMentionId(event.target.value);
                setMentionLabel(selected?.label ?? '');
              }}
            >
              <option value="">Chọn người được nhắc</option>
              {visibleMentionOptions.map((option) => <option key={`${option.type}:${option.id}`} value={option.id}>{option.label}{option.description ? ` · ${option.description}` : ''}</option>)}
            </select>
          </label>
          <button
            className="mt-5 inline-flex h-10 items-center justify-center rounded-md border border-line px-3 text-sm font-semibold text-slate-700 hover:bg-slate-50"
            type="button"
            onClick={addMention}
            aria-label="Thêm người được nhắc"
          >
            <Plus className="h-4 w-4" />
          </button>
        </div>
        {mentions.length ? (
          <div className="mt-2 flex flex-wrap gap-1">
            {mentions.map((mention) => (
              <span key={`${mention.type}:${mention.id}`} className="inline-flex max-w-full items-center gap-1 rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">
                <span className="truncate">@{mention.label || mention.id}</span>
                <button
                  type="button"
                  className="rounded p-0.5 hover:bg-blue-100"
                  onClick={() => setMentions((current) => current.filter((item) => !(item.type === mention.type && item.id === mention.id)))}
                  aria-label="Gỡ người được nhắc"
                >
                  <X className="h-3 w-3" />
                </button>
              </span>
            ))}
          </div>
        ) : null}
      </div>

      <div className="mt-3 flex justify-end gap-2">
        <button
          className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-50"
          type="button"
          onClick={() => setExpanded(false)}
        >
          Hủy
        </button>
        <button
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
          type="submit"
          disabled={submitting}
        >
          {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
          Gửi trao đổi
        </button>
      </div>
    </form>
  );
}

function activityLabelForReply(activityType: string) {
  return enumLabel('activity_type', activityType);
}
