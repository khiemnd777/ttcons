import { FormEvent, useMemo, useState } from 'react';
import { Loader2, Plus, Send, X } from 'lucide-react';
import clsx from 'clsx';
import { useToast } from '../../components/ToastProvider';
import type { ActivityAttachmentPayload, ActivityMentionPayload, ActivityPayload } from '../../lib/types/domain';

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
  { value: 'comment', label: 'Ghi chú' },
  { value: 'status', label: 'Trạng thái' },
  { value: 'blocker', label: 'Blocker' },
  { value: 'question', label: 'Câu hỏi' },
  { value: 'decision', label: 'Quyết định' },
  { value: 'reminder', label: 'Nhắc việc' },
  { value: 'file_note', label: 'Ghi chú file' },
];

const workItemOnlyOptions: Array<{ value: ActivityType; label: string }> = [];

const visibilityOptions: Array<{ value: Visibility; label: string }> = [
  { value: 'project', label: 'Project' },
  { value: 'internal', label: 'Internal' },
  { value: 'client', label: 'Client' },
];

const targetScopeOptions = [
  { value: 'project', label: 'Project' },
  { value: 'work_item', label: 'Work item' },
  { value: 'boq_item', label: 'BOQ item' },
  { value: 'procurement_request', label: 'Procurement' },
  { value: 'stock_movement', label: 'Stock movement' },
  { value: 'acceptance', label: 'Acceptance' },
  { value: 'rework', label: 'Rework' },
  { value: 'handover', label: 'Handover' },
  { value: 'payable', label: 'Payable' },
  { value: 'receivable', label: 'Receivable' },
  { value: 'payment', label: 'Payment' },
];

const mentionTypeOptions: Array<{ value: MentionType; label: string }> = [
  { value: 'staff', label: 'Staff' },
  { value: 'user', label: 'User' },
  { value: 'role', label: 'Role' },
];

const attachmentTypeOptions: Array<{ value: AttachmentType; label: string }> = [
  { value: 'evidence', label: 'Evidence' },
  { value: 'document', label: 'Document' },
];

export function ActivityComposer({
  mode,
  submitting,
  allowTarget = false,
  targetOptions = [],
  attachmentOptions = [],
  onSubmit,
}: {
  mode: ComposerMode;
  submitting?: boolean;
  allowTarget?: boolean;
  targetOptions?: ActivityTargetOption[];
  attachmentOptions?: ActivityAttachmentOption[];
  onSubmit: (payload: ActivityPayload) => Promise<void>;
}) {
  const toast = useToast();
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

  const activityOptions = useMemo(() => {
    if (mode === 'work_item') return [...baseActivityOptions, ...workItemOnlyOptions];
    return baseActivityOptions;
  }, [mode]);

  const needsDueAt = activityType === 'reminder';
  const showsDueAt = activityType === 'reminder' || activityType === 'blocker';
  const needsReference = activityType === 'file_note';
  const needsTargetId = allowTarget && targetScope !== 'project';
  const missingComment = touched && !comment.trim();
  const missingDueAt = touched && needsDueAt && !dueAt;
  const missingReference = touched && needsReference && !attachments.length;
  const missingTarget = touched && needsTargetId && !targetId.trim();
  const visibleTargetOptions = targetOptions.filter((option) => option.scope_type === targetScope);
  const visibleAttachmentOptions = attachmentOptions.filter((option) => option.type === referenceType);

  function addMention() {
    const id = mentionId.trim();
    if (!id) {
      toast.warn({ title: 'Thiếu mention', description: mentionType === 'role' ? 'Nhập role label cần mention.' : 'Nhập ID cần mention.' });
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
      toast.warn({ title: 'Thiếu attachment', description: 'Cần chọn hoặc nhập ID evidence/document.' });
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
    setTouched(true);
    const missing = [
      !comment.trim() ? 'nội dung' : '',
      needsDueAt && !dueAt ? 'ngày nhắc' : '',
      needsReference && !attachments.length ? 'evidence/document' : '',
      needsTargetId && !targetId.trim() ? 'target record' : '',
    ].filter(Boolean);
    if (missing.length) {
      toast.warn({ title: 'Thiếu thông tin activity', description: `Cần nhập: ${missing.join(', ')}` });
      return;
    }

    const metadata: Record<string, string> = {};
    if (dueAt) metadata.due_at = dueAt;
    if (ownerId.trim()) metadata.owner_id = ownerId.trim();
    const selectedTarget = targetOptions.find((option) => option.scope_type === targetScope && option.scope_id === targetId.trim());

    try {
      await onSubmit({
        activity_type: activityType,
        comment: comment.trim(),
        visibility,
        scope_type: allowTarget ? targetScope : undefined,
        scope_id: allowTarget && targetScope !== 'project' ? targetId.trim() : undefined,
        work_item_id: selectedTarget?.work_item_id,
        mentions,
        attachments,
        metadata,
      });
    } catch {
      return;
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
  }

  return (
    <form className="rounded-md border border-line bg-slate-50 p-3" onSubmit={submit}>
      <div className="grid gap-2 sm:grid-cols-2">
        <label className="block">
          <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Type</span>
          <select
            className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
            value={activityType}
            onChange={(event) => setActivityType(event.target.value as ActivityType)}
          >
            {activityOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </select>
        </label>
        <label className="block">
          <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Visibility</span>
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
            <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Scope</span>
            <select
              className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
              value={targetScope}
              onChange={(event) => {
                setTargetScope(event.target.value);
                setTargetId('');
              }}
            >
              {targetScopeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select>
          </label>
          {targetScope !== 'project' ? (
            <label className="block">
              <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Target</span>
              {visibleTargetOptions.length ? (
                <select
                  className={clsx(
                    'w-full rounded-md border bg-white px-3 py-2 text-sm font-medium text-slate-700',
                    missingTarget ? 'border-amber-300 bg-amber-50' : 'border-line',
                  )}
                  value={targetId}
                  onChange={(event) => setTargetId(event.target.value)}
                >
                  <option value="">Chọn target</option>
                  {visibleTargetOptions.map((option) => <option key={`${option.scope_type}:${option.scope_id}`} value={option.scope_id}>{option.label}</option>)}
                </select>
              ) : (
                <input
                  className={clsx(
                    'w-full rounded-md border bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500',
                    missingTarget ? 'border-amber-300 bg-amber-50' : 'border-line',
                  )}
                  value={targetId}
                  onChange={(event) => setTargetId(event.target.value)}
                  placeholder="Record ID"
                />
              )}
            </label>
          ) : null}
        </div>
      ) : null}

      <textarea
        className={clsx(
          'mt-2 min-h-20 w-full rounded-md border bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500',
          missingComment ? 'border-amber-300 bg-amber-50' : 'border-line',
        )}
        value={comment}
        onBlur={() => setTouched(true)}
        onChange={(event) => setComment(event.target.value)}
        placeholder="Nhập nội dung activity"
      />

      {showsDueAt || activityType === 'decision' || needsReference ? (
        <div className="mt-2 grid gap-2 sm:grid-cols-2">
          {showsDueAt ? (
            <label className="block">
              <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">{activityType === 'reminder' ? 'Due date' : 'Blocker due'}</span>
              <input
                className={clsx(
                  'w-full rounded-md border bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500',
                  missingDueAt ? 'border-amber-300 bg-amber-50' : 'border-line',
                )}
                type="date"
                value={dueAt}
                onChange={(event) => setDueAt(event.target.value)}
              />
            </label>
          ) : null}
          {activityType === 'blocker' || activityType === 'decision' ? (
            <label className="block">
              <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Owner ID</span>
              <input
                className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
                value={ownerId}
                onChange={(event) => setOwnerId(event.target.value)}
              />
            </label>
          ) : null}
          {needsReference ? (
            <div className={clsx('rounded-md border bg-white p-2 sm:col-span-2', missingReference ? 'border-amber-300 bg-amber-50' : 'border-line')}>
              <div className="grid gap-2 sm:grid-cols-[130px_minmax(0,1fr)_minmax(0,1fr)_auto]">
                <label className="block">
                  <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Attachment</span>
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
                  <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Record</span>
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
                      <option value="">Chọn attachment</option>
                      {visibleAttachmentOptions.map((option) => (
                        <option key={`${option.type}:${option.id}`} value={option.id}>
                          {option.label}{option.status ? ` · ${option.status}` : ''}{option.visibility ? ` · ${option.visibility}` : ''}
                        </option>
                      ))}
                    </select>
                  ) : (
                    <input
                      className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
                      value={referenceId}
                      onChange={(event) => setReferenceId(event.target.value)}
                      placeholder={`${referenceType} UUID`}
                    />
                  )}
                </label>
                <label className="block">
                  <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Label</span>
                  <input
                    className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
                    value={referenceLabel}
                    onChange={(event) => setReferenceLabel(event.target.value)}
                    placeholder="Tên hiển thị"
                  />
                </label>
                <button
                  className="mt-5 inline-flex h-10 items-center justify-center rounded-md border border-line px-3 text-sm font-semibold text-slate-700 hover:bg-slate-50"
                  type="button"
                  onClick={addAttachment}
                  aria-label="Add attachment"
                >
                  <Plus className="h-4 w-4" />
                </button>
              </div>
              {attachments.length ? (
                <div className="mt-2 flex flex-wrap gap-1">
                  {attachments.map((attachment) => (
                    <span key={`${attachment.type}:${attachment.id}`} className="inline-flex max-w-full items-center gap-1 rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700">
                      <span className="truncate">{attachment.type}: {attachment.label || attachment.id}</span>
                      <button
                        type="button"
                        className="rounded p-0.5 hover:bg-emerald-100"
                        onClick={() => setAttachments((current) => current.filter((item) => !(item.type === attachment.type && item.id === attachment.id)))}
                        aria-label="Remove attachment"
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

      <div className="mt-2 rounded-md border border-line bg-white p-2">
        <div className="grid gap-2 sm:grid-cols-[130px_minmax(0,1fr)_minmax(0,1fr)_auto]">
          <label className="block">
            <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Mention</span>
            <select
              className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
              value={mentionType}
              onChange={(event) => setMentionType(event.target.value as MentionType)}
            >
              {mentionTypeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select>
          </label>
          <label className="block">
            <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">{mentionType === 'role' ? 'Role label' : 'ID'}</span>
            <input
              className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
              value={mentionId}
              onChange={(event) => setMentionId(event.target.value)}
              placeholder={mentionType === 'role' ? 'PM' : `${mentionType} UUID`}
            />
          </label>
          <label className="block">
            <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Label</span>
            <input
              className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
              value={mentionLabel}
              onChange={(event) => setMentionLabel(event.target.value)}
              placeholder="Tên hiển thị"
            />
          </label>
          <button
            className="mt-5 inline-flex h-10 items-center justify-center rounded-md border border-line px-3 text-sm font-semibold text-slate-700 hover:bg-slate-50"
            type="button"
            onClick={addMention}
            aria-label="Add mention"
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
                  aria-label="Remove mention"
                >
                  <X className="h-3 w-3" />
                </button>
              </span>
            ))}
          </div>
        ) : null}
      </div>

      <div className="mt-2 flex justify-end">
        <button
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
          type="submit"
          disabled={submitting}
        >
          {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
          Gửi activity
        </button>
      </div>
    </form>
  );
}
