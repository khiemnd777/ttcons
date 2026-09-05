import assert from 'node:assert/strict';
import test from 'node:test';
import { readFile } from 'node:fs/promises';
import {
  chatKeyAction, chatMentionPayload, chatMentionSuggestions, filterMentionMembers, hasAllMention, inlineMentionPayload, insertInlineMention,
  mentionQueryAt, reconcileInlineMentions,
  type ActivityMentionOption,
} from '../src/modules/work-items/activityChatInputDomain.ts';

const an: ActivityMentionOption = { type: 'staff', id: 'staff-an', label: 'Nguyễn Văn An', description: 'Chỉ huy trưởng · Thi công' };
const otherAn: ActivityMentionOption = { ...an, id: 'staff-other-an', description: 'Kế toán' };
const mai: ActivityMentionOption = { type: 'staff', id: 'staff-mai', label: 'Đặng Mai' };

test('mention suggestions require an @ boundary, avoid email, allow Vietnamese and cursor edits', () => {
  assert.deepEqual(mentionQueryAt('@', 1), { start: 0, end: 1, query: '' });
  assert.deepEqual(mentionQueryAt('Anh @Nguyễn Văn', 15), { start: 4, end: 15, query: 'Nguyễn Văn' });
  assert.deepEqual(mentionQueryAt('Hi @nguyen nhé', 6), { start: 3, end: 10, query: 'ng' });
  assert.equal(mentionQueryAt('email@domain.vn', 14), null);
  assert.equal(mentionQueryAt('@An\nkhác', 8), null);
  assert.equal(mentionQueryAt('@An', 0, 3), null);
  assert.equal(mentionQueryAt('văn bản', 7), null);
});

test('search folds accents and đ, scopes to member IDs and disambiguates duplicate names', () => {
  const options = [an, otherAn, mai, an, { type: 'role' as const, id: 'pm', label: 'Quản lý' }];
  assert.deepEqual(filterMentionMembers(options, 'NGUYEN an'), [an, otherAn]);
  assert.deepEqual(filterMentionMembers(options, 'ke toan'), [otherAn]);
  assert.deepEqual(filterMentionMembers(options, 'dang'), [mai]);
  assert.equal(filterMentionMembers(options, '').length, 3);
  assert.deepEqual(filterMentionMembers(options, 'khong co'), []);
});

test('selecting a member replaces the active query, retains suffix and sends their exact ID', () => {
  const next = insertInlineMention('Hi @nguyen nhé', { start: 3, end: 10, query: 'ng' }, otherAn, []);
  assert.equal(next.value, 'Hi @Nguyễn Văn An nhé');
  assert.equal(next.caret, 18);
  assert.deepEqual(inlineMentionPayload(next.mentions), [{ type: 'staff', id: otherAn.id, label: otherAn.label }]);
  assert.equal(mentionQueryAt(next.value, next.caret, next.caret, next.mentions), null);
});

test('insertions before and after a mention shift ranges without losing identity', () => {
  const selected = insertInlineMention('@', { start: 0, end: 1, query: '' }, an, []);
  const next = `Chào ${selected.value}giúp tôi`;
  const before = reconcileInlineMentions(selected.value, `Chào ${selected.value}`, selected.mentions);
  const after = reconcileInlineMentions(`Chào ${selected.value}`, next, before);
  assert.equal(after[0].start, 5);
  assert.equal(after[0].end, 19);
  assert.deepEqual(inlineMentionPayload(after), inlineMentionPayload(selected.mentions));
});

test('editing, extending or deleting selected mention text removes its notification recipient', () => {
  const selected = insertInlineMention('@', { start: 0, end: 1, query: '' }, an, []);
  for (const next of ['', 'plain text', selected.value.replace('An', 'Anh'), selected.value.replace('@', ''), selected.value.replace('Nguyễn', 'Nguyen'), `x${selected.value}`]) {
    assert.deepEqual(reconcileInlineMentions(selected.value, next, selected.mentions), [], next);
  }
  assert.deepEqual(reconcileInlineMentions('', '@Nguyễn Văn An', []), [], 'pasting a name cannot invent a recipient');
});

test('multiple selected ranges deduplicate IDs; deleting one occurrence retains the other', () => {
  const first = insertInlineMention('@', { start: 0, end: 1, query: '' }, an, []);
  const second = insertInlineMention(first.value + '@', { start: first.value.length, end: first.value.length + 1, query: '' }, an, first.mentions);
  assert.equal(inlineMentionPayload(second.mentions).length, 1);
  const remainingText = second.value.slice(first.value.length);
  const remaining = reconcileInlineMentions(second.value, remainingText, second.mentions);
  assert.equal(remaining.length, 1);
  assert.equal(inlineMentionPayload(remaining)[0].id, an.id);
  const third = insertInlineMention(second.value + '@', { start: second.value.length, end: second.value.length + 1, query: '' }, otherAn, second.mentions);
  assert.equal(inlineMentionPayload(third.mentions).length, 2, 'duplicate names are different identities');
});

test('Enter selects before sending, modifier Enter inserts newline, IME and pending sends never submit', () => {
  assert.equal(chatKeyAction({ key: 'Enter' }, false, false), 'submit');
  assert.equal(chatKeyAction({ key: 'Enter' }, true, false), 'select');
  for (const popup of [true, false]) {
    assert.equal(chatKeyAction({ key: 'Enter', shiftKey: true }, popup, false), 'newline');
    assert.equal(chatKeyAction({ key: 'Enter', ctrlKey: true }, popup, false), 'newline');
    assert.equal(chatKeyAction({ key: 'Enter', isComposing: true }, popup, false), 'none');
    assert.equal(chatKeyAction({ key: 'Enter', keyCode: 229 }, popup, false), 'none');
    assert.equal(chatKeyAction({ key: 'Enter' }, popup, true), 'block');
  }
  assert.equal(chatKeyAction({ key: 'Escape' }, true, false), 'dismiss');
  assert.equal(chatKeyAction({ key: 'Escape' }, false, false), 'none');
  assert.equal(chatKeyAction({ key: 'ArrowUp' }, true, false), 'previous');
  assert.equal(chatKeyAction({ key: 'ArrowDown' }, true, false), 'next');
});

test('deleting the first of two equal names preserves the second person, not the first ID', () => {
  const first = insertInlineMention('@', { start: 0, end: 1, query: '' }, an, []);
  const second = insertInlineMention(first.value + '@', { start: first.value.length, end: first.value.length + 1, query: '' }, otherAn, first.mentions);
  const afterDeletingFirst = reconcileInlineMentions(second.value, second.value.slice(first.value.length), second.mentions, 0);
  assert.deepEqual(inlineMentionPayload(afterDeletingFirst), [{ type: 'staff', id: otherAn.id, label: otherAn.label }]);
  const afterDeletingSecond = reconcileInlineMentions(second.value, first.value, second.mentions, first.value.length);
  assert.deepEqual(inlineMentionPayload(afterDeletingSecond), [{ type: 'staff', id: an.id, label: an.label }]);
});

test('both chat layouts mount one shared input and preserve backend reply visibility', async () => {
  const composer = await readFile(new URL('../src/modules/work-items/ActivityComposer.tsx', import.meta.url), 'utf8');
  const fullStart = composer.indexOf("  if (variant === 'chat') {");
  const compactStart = composer.indexOf("  if (variant === 'chat' && density === 'compact') {");
  const chat = composer.slice(compactStart, composer.indexOf('if (!expanded)', fullStart));
  assert.equal((chat.match(/\{chatInput\}/g) ?? []).length, 2);
  assert.doesNotMatch(chat, /value=\{visibility\}|mentionDisclosure|<textarea/);
  assert.match(composer, /visibility: replying \? \(replyTo\?\.visibility as Visibility \?\? 'project'\) : variant === 'chat' \? 'project' : visibility/);
  assert.match(composer, /if \(submitting \|\| submissionInFlight.current\) return/);
});

test('@all is a reserved case-insensitive whole token, not an email or longer identifier', () => {
  for (const text of ['@all', '@All', '@ALL Họp nhé', 'Chào @aLl!', '(@all)', '@all.', 'Họp @all, lúc 3h', 'Tin\n@all\nxong']) {
    assert.equal(hasAllMention(text), true, text);
    assert.deepEqual(inlineMentionPayload([], text), [{ type: 'all', id: 'all', label: 'all' }]);
  }
  for (const text of ['all', '@allabc', '@all_2', '@allé', '@all@host.com', 'email@all.com', '@all.com', '@all/path', 'x@all', '@all\u0301']) {
    assert.equal(hasAllMention(text), false, text);
    assert.deepEqual(inlineMentionPayload([], text), []);
  }
  assert.equal(mentionQueryAt('@All họp lúc 3h', 15), null, 'a completed all token must not trap Enter in an empty suggestion list');
});

test('all suggestion is scoped to available project members and typing/removing all keeps payload current', () => {
  const options = chatMentionSuggestions([an, otherAn], 'Al');
  assert.equal(options[0].type, 'all');
  assert.equal(options[0].description, 'Tất cả thành viên dự án');
  assert.deepEqual(chatMentionSuggestions([], ''), []);
  assert.deepEqual(chatMentionSuggestions([an], 'nguyen'), [an]);
  const selected = insertInlineMention('@al', { start: 0, end: 3, query: 'al' }, options[0], []);
  assert.equal(selected.value, '@all ');
  assert.deepEqual(inlineMentionPayload(selected.mentions, selected.value), [{ type: 'all', id: 'all', label: 'all' }]);
  assert.deepEqual(inlineMentionPayload(selected.mentions, '@allabc'), []);
  assert.deepEqual(inlineMentionPayload(selected.mentions, ''), []);
  assert.equal(inlineMentionPayload(selected.mentions, '@all @All @ALL').length, 1);
  assert.deepEqual(chatMentionPayload('new draft', [{ type: 'all', id: 'all' }]), [], 'submission rechecks current text instead of trusting stale all state');
  assert.deepEqual(chatMentionPayload('@All hydrated draft', []), [{ type: 'all', id: 'all', label: 'all' }]);
});

test('sent chat drops the redundant recipient row but retains content and mentioned-me state', async () => {
  const thread = await readFile(new URL('../src/modules/work-items/ActivityThread.tsx', import.meta.url), 'utf8');
  assert.doesNotMatch(thread, /@\{mention.label \|\| mention.id\}|mentions.map/);
  assert.match(thread, /item.mentioned_me[\s\S]*@tôi/);
  assert.match(thread, /whitespace-pre-wrap break-words text-sm leading-6[\s\S]*\{comment\}/);
});
