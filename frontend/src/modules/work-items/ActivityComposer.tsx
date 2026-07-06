import { FormEvent, useMemo, useState } from 'react';
import { Loader2, Plus, Send, X } from 'lucide-react';
import clsx from 'clsx';
import { useToast } from '../../components/ToastProvider';
import type { ActivityAttachmentPayload, ActivityMentionPayload, ActivityPayload } from '../../lib/types/domain';
import { enumLabel, statusLabel } from '../../lib/utils/status';

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
  { value: 'project', label: enumLabel('visibility', 'project') },
  { value: 'internal', label: enumLabel('visibility', 'internal') },
  { value: 'client', label: enumLabel('visibility', 'client') },
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
  { value: 'user', label: enumLabel('mention_type', 'user') },
  { value: 'role', label: enumLabel('mention_type', 'role') },
];

const attachmentTypeOptions: Array<{ value: AttachmentType; label: string }> = [
  { value: 'evidence', label: enumLabel('attachment_type', 'evidence') },
  { value: 'document', label: enumLabel('attachment_type', 'document') },
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
      toast.warn({ title: 'Thiếu người được nhắc', description: mentionType === 'role' ? 'Nhập nhãn vai trò cần nhắc.' : 'Nhập ID cần nhắc.' });
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
      needsReference && !attachments.length ? 'minh chứng/tài liệu' : '',
      needsTargetId && !targetId.trim() ? 'bản ghi đích' : '',
    ].filter(Boolean);
    if (missing.length) {
      toast.warn({ title: 'Thiếu thông tin hoạt động', description: `Cần nhập: ${missing.join(', ')}` });
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
          <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Loại</span>
          <select
            className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
            value={activityType}
            onChange={(event) => setActivityType(event.target.value as ActivityType)}
          >
            {activityOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
          </select>
        </label>
        <label className="block">
          <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Phạm vi hiển thị</span>
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
            <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Phạm vi</span>
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
              <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Đích</span>
              {visibleTargetOptions.length ? (
                <select
                  className={clsx(
                    'w-full rounded-md border bg-white px-3 py-2 text-sm font-medium text-slate-700',
                    missingTarget ? 'border-amber-300 bg-amber-50' : 'border-line',
                  )}
                  value={targetId}
                  onChange={(event) => setTargetId(event.target.value)}
                >
                  <option value="">Chọn đích</option>
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
                  placeholder="ID bản ghi"
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
        placeholder="Nhập nội dung hoạt động"
      />

      {showsDueAt || activityType === 'decision' || needsReference ? (
        <div className="mt-2 grid gap-2 sm:grid-cols-2">
          {showsDueAt ? (
            <label className="block">
              <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">{activityType === 'reminder' ? 'Ngày nhắc' : 'Hạn điểm chặn'}</span>
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
              <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">ID người phụ trách</span>
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
                  ) : (
                    <input
                      className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
                      value={referenceId}
                      onChange={(event) => setReferenceId(event.target.value)}
                      placeholder={`${enumLabel('attachment_type', referenceType)} UUID`}
                    />
                  )}
                </label>
                <label className="block">
                  <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Nhãn</span>
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

      <div className="mt-2 rounded-md border border-line bg-white p-2">
        <div className="grid gap-2 sm:grid-cols-[130px_minmax(0,1fr)_minmax(0,1fr)_auto]">
          <label className="block">
            <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Nhắc đến</span>
            <select
              className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm font-medium text-slate-700"
              value={mentionType}
              onChange={(event) => setMentionType(event.target.value as MentionType)}
            >
              {mentionTypeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select>
          </label>
          <label className="block">
            <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">{mentionType === 'role' ? 'Nhãn vai trò' : 'ID'}</span>
            <input
              className="w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
              value={mentionId}
              onChange={(event) => setMentionId(event.target.value)}
              placeholder={mentionType === 'role' ? 'PM' : `${mentionType} UUID`}
            />
          </label>
          <label className="block">
            <span className="mb-1 block text-xs font-semibold uppercase tracking-wide text-slate-500">Nhãn</span>
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

      <div className="mt-2 flex justify-end">
        <button
          className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
          type="submit"
          disabled={submitting}
        >
          {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
          Gửi hoạt động
        </button>
      </div>
    </form>
  );
}
