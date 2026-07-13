import { FormEvent, useEffect, useMemo, useState } from 'react';
import { ChevronUp, Loader2, MessageSquarePlus, Plus, Send, X } from 'lucide-react';
import clsx from 'clsx';
import { useToast } from '../../components/ToastProvider';
import type { ActivityAttachmentPayload, ActivityMentionPayload, ActivityPayload, WorkItemActivity } from '../../lib/types/domain';
import { enumLabel, statusLabel } from '../../lib/utils/status';
import { compactDateTime } from './workItemPresentation';

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

export type ActivityMentionOption = {
  type: 'staff' | 'role';
  id: string;
  label: string;
  description?: string;
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

export function ActivityComposer({
  mode,
  submitting,
  allowTarget = false,
  targetOptions = [],
  attachmentOptions = [],
  mentionOptions = [],
  defaultExpanded = false,
  variant = 'card',
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
  defaultExpanded?: boolean;
  variant?: 'card' | 'chat';
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

  useEffect(() => {
    if (!editItem) return;
    setComment(editItem.comment ?? '');
    setTouched(false);
  }, [editItem?.id]);

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

    if (editing && !editCommentChanged) return;

    if (editing && onSubmitEdit) {
      try {
        await onSubmitEdit(comment.trim());
      } catch {
        return;
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
        visibility: replying ? (replyTo?.visibility as Visibility ?? 'project') : visibility,
        scope_type: replying ? replyTo?.scope_type : allowTarget ? targetScope : undefined,
        scope_id: replying ? replyTo?.scope_id : allowTarget && targetScope !== 'project' ? targetId.trim() : undefined,
        work_item_id: replying ? replyTo?.work_item_id : selectedTarget?.work_item_id,
        parent_activity_id: replying ? replyTo?.id : undefined,
        mentions,
        attachments: replying ? [] : attachments,
        metadata: replying ? {} : metadata,
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
    if (variant === 'card') setExpanded(false);
  }

  if (variant === 'chat') {
    return (
      <form className="space-y-2" onSubmit={submit}>
        {editing && editItem ? (
          <div className="flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-left">
            <div className="min-w-0 flex-1">
              <p className="text-xs font-semibold text-amber-800">Đang chỉnh sửa tin nhắn · {compactDateTime(editItem.created_at)}</p>
              <p className="mt-0.5 truncate text-[11px] text-slate-500">
                {enumLabel('scope_type', editItem.scope_type ?? 'project')} · {visibilityOptions.find((option) => option.value === editItem.visibility)?.label ?? editItem.visibility}
              </p>
            </div>
            <button type="button" className="rounded-md p-1 text-slate-500 hover:bg-white" onClick={onCancelEdit} aria-label="Hủy chỉnh sửa"><X className="h-4 w-4" /></button>
          </div>
        ) : null}
        {replying && replyTo ? (
          <div className="flex items-start gap-2 rounded-lg border border-blue-100 bg-blue-50 px-3 py-2 text-left">
            <div className="min-w-0 flex-1">
              <p className="text-xs font-semibold text-blue-800">Đang trả lời {replyTo.actor_name ?? 'thành viên công trình'}</p>
              <p className="mt-0.5 line-clamp-2 text-xs leading-5 text-slate-600">{replyTo.comment || activityLabelForReply(replyTo.activity_type)}</p>
              <p className="mt-0.5 truncate text-[11px] text-slate-500">{replyTo.target_label ?? 'Công trình'} · {visibilityOptions.find((option) => option.value === replyTo.visibility)?.label ?? replyTo.visibility}</p>
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
                  <label className="block">
                    <span className="sr-only">Đối tượng được gắn</span>
                    <select
                      className={clsx(
                        'w-full rounded-md border bg-white px-2.5 py-2 text-xs font-semibold text-slate-600',
                        missingTarget ? 'border-amber-300 bg-amber-50' : 'border-line',
                      )}
                      value={targetId}
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
              <label className="block">
                <span className="sr-only">{activityType === 'reminder' ? 'Ngày nhắc' : 'Hạn điểm chặn'}</span>
                <input
                  className={clsx(
                    'w-full rounded-md border bg-white px-2.5 py-2 text-xs font-semibold text-slate-600',
                    missingDueAt ? 'border-amber-300 bg-amber-50' : 'border-line',
                  )}
                  type="date"
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
          <div className={clsx('grid gap-2 rounded-md border bg-slate-50 p-2 sm:grid-cols-[120px_minmax(0,1fr)_auto]', missingReference ? 'border-amber-300' : 'border-line')}>
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

        {!editing && (mentions.length || attachments.length) ? (
          <div className="flex flex-wrap gap-1.5">
            {mentions.map((mention) => (
              <span key={`${mention.type}:${mention.id}`} className="inline-flex max-w-full items-center gap-1 rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">
                <span className="truncate">@{mention.label || mention.id}</span>
                <button type="button" className="rounded p-0.5 hover:bg-blue-100" onClick={() => setMentions((current) => current.filter((item) => !(item.type === mention.type && item.id === mention.id)))} aria-label="Gỡ người được nhắc"><X className="h-3 w-3" /></button>
              </span>
            ))}
            {attachments.map((attachment) => (
              <span key={`${attachment.type}:${attachment.id}`} className="inline-flex max-w-full items-center gap-1 rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700">
                <span className="truncate">{attachment.label || attachment.id}</span>
                <button type="button" className="rounded p-0.5 hover:bg-emerald-100" onClick={() => setAttachments((current) => current.filter((item) => !(item.type === attachment.type && item.id === attachment.id)))} aria-label="Gỡ tệp đính kèm"><X className="h-3 w-3" /></button>
              </span>
            ))}
          </div>
        ) : null}

        <div className={clsx('rounded-xl border bg-white shadow-sm focus-within:border-blue-300 focus-within:ring-2 focus-within:ring-blue-100', missingComment ? 'border-amber-300' : 'border-line')}>
          <textarea
            className="max-h-40 min-h-16 w-full resize-y rounded-t-xl bg-transparent px-3 py-2.5 text-sm leading-6 outline-none"
            value={comment}
            onBlur={() => setTouched(true)}
            onChange={(event) => setComment(event.target.value)}
            placeholder={editing ? 'Nhập nội dung chỉnh sửa...' : replying ? `Trả lời ${replyTo?.actor_name ?? 'thành viên công trình'}...` : 'Nhắn vào thảo luận công trình...'}
          />
          <div className="flex flex-wrap items-center gap-1.5 border-t border-line px-2 py-2">
            {editing ? <span className="rounded-md bg-amber-50 px-2 py-1.5 text-xs font-semibold text-amber-700">✎ Chỉnh sửa</span> : replying ? <span className="rounded-md bg-blue-50 px-2 py-1.5 text-xs font-semibold text-blue-700">↩ Phản hồi</span> : (
              <label className="min-w-0">
                <span className="sr-only">Loại trao đổi</span>
                <select className="max-w-36 rounded-md border-0 bg-slate-100 px-2 py-1.5 text-xs font-semibold text-slate-600" value={activityType} onChange={(event) => setActivityType(event.target.value as ActivityType)}>
                  {activityOptions.map((option) => <option key={option.value} value={option.value}>＋ {option.label}</option>)}
                </select>
              </label>
            )}
            {!editing ? <details className="relative">
              <summary className="cursor-pointer list-none rounded-md px-2 py-1.5 text-xs font-semibold text-slate-600 hover:bg-slate-100">@ Nhắc đến</summary>
              <div className="absolute bottom-9 left-0 z-20 w-72 space-y-2 rounded-lg border border-line bg-white p-2 shadow-lg">
                <select className="w-full rounded-md border border-line px-2 py-2 text-xs" value={mentionType} onChange={(event) => { setMentionType(event.target.value as MentionType); setMentionId(''); setMentionLabel(''); }}>
                  {mentionTypeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                </select>
                <div className="flex gap-1.5">
                  <select className="min-w-0 flex-1 rounded-md border border-line px-2 py-2 text-xs" value={mentionId} onChange={(event) => { const selected = visibleMentionOptions.find((option) => option.id === event.target.value); setMentionId(event.target.value); setMentionLabel(selected?.label ?? ''); }}>
                    <option value="">Chọn người hoặc vai trò</option>
                    {visibleMentionOptions.map((option) => <option key={`${option.type}:${option.id}`} value={option.id}>{option.label}</option>)}
                  </select>
                  <button type="button" className="rounded-md border border-line px-2 text-slate-600 hover:bg-slate-50" onClick={addMention} aria-label="Thêm người được nhắc"><Plus className="h-4 w-4" /></button>
                </div>
              </div>
            </details> : null}
            {editing ? <span className="max-w-44 truncate rounded-md bg-slate-100 px-2 py-1.5 text-xs font-semibold text-slate-600">Phạm vi không đổi</span> : replying ? <span className="max-w-44 truncate rounded-md bg-slate-100 px-2 py-1.5 text-xs font-semibold text-slate-600" title="Phạm vi xem được kế thừa từ tin gốc">{visibilityOptions.find((option) => option.value === replyTo?.visibility)?.label ?? replyTo?.visibility}</span> : (
              <label className="min-w-0">
                <span className="sr-only">Ai có thể xem</span>
                <select className="max-w-44 rounded-md border-0 bg-slate-100 px-2 py-1.5 text-xs font-semibold text-slate-600" value={visibility} onChange={(event) => setVisibility(event.target.value as Visibility)}>
                  {visibilityOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                </select>
              </label>
            )}
            {editing && !editCommentChanged ? <span className="text-xs font-medium text-slate-400">Chưa có thay đổi</span> : null}
            <button className={clsx('ml-auto inline-flex h-9 items-center justify-center rounded-full bg-blue-600 text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60', editing ? 'gap-1.5 px-3 text-xs font-semibold' : 'w-9')} type="submit" disabled={submitting || (editing && !editCommentChanged)} aria-label={editing ? 'Lưu chỉnh sửa' : 'Gửi trao đổi'}>
              {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : editing ? 'Lưu chỉnh sửa' : <Send className="h-4 w-4" />}
            </button>
          </div>
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
    <form className="rounded-lg border border-blue-100 bg-blue-50/30 p-3 shadow-sm" onSubmit={submit}>
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
                  <option value="">Chọn đối tượng</option>
                  {visibleTargetOptions.map((option) => <option key={`${option.scope_type}:${option.scope_id}`} value={option.scope_id}>{option.label}</option>)}
                </select>
              ) : <div className="rounded-md border border-dashed border-line bg-slate-50 px-3 py-2 text-sm text-slate-500">Chưa có đối tượng phù hợp</div>}
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
            <div className={clsx('rounded-md border bg-white p-2 sm:col-span-2', missingReference ? 'border-amber-300 bg-amber-50' : 'border-line')}>
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

      <div className="mt-2 rounded-md border border-line bg-white p-2">
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
