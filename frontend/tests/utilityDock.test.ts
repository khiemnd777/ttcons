import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('global Utility Dock persists across standard and focus AppShell branches', async () => {
  const [shell, dock] = await Promise.all([
    source('../src/layouts/AppShell.tsx'),
    source('../src/modules/utilities/UtilityDock.tsx'),
  ]);

  assert.match(shell, /import \{ UtilityDock \} from '\.\.\/modules\/utilities\/UtilityDock'/);
  assert.ok((shell.match(/<UtilityDock key="global-utility-dock" \/>/g) ?? []).length === 2, 'standard and focus shells must share one keyed utility identity');
  assert.match(dock, /type DesktopDockKey = 'work' \| `project:\$\{string\}`/);
  assert.match(dock, /const maxDiscussionTabs = 5/);
  assert.match(dock, /event\.key !== 'Escape'/);
  assert.match(dock, /setExpandedDockKeys\(\(current\) => current\.slice\(0, -1\)\)/);
  assert.match(dock, /setOpenProjectIDs\(\[\]\)/);
});

test('desktop Utility Dock keeps every project lane at a comfortable 336px and reveals the active dock', async () => {
  const dock = await source('../src/modules/utilities/UtilityDock.tsx');
  const deckStart = dock.indexOf('pointer-events-none flex w-full items-end gap-2 pb-0.5');
  const pickerStart = dock.indexOf('{pickerOpen ? (', deckStart);
  const mobileDockStart = dock.indexOf('pointer-events-auto flex items-center gap-2 sm:hidden', pickerStart);

  assert.ok(deckStart >= 0, 'desktop utility deck must span the available width');
  assert.ok(pickerStart > deckStart, 'project picker must remain outside the aligned lane deck');
  assert.ok(mobileDockStart > pickerStart, 'mobile launcher must remain a separate compact surface');
  const deck = dock.slice(deckStart, pickerStart);
  const projectsIndex = deck.indexOf('openProjectIDs.map');
  const discussionIndex = deck.indexOf('aria-label={`Mở danh sách trao đổi');
  const workIndex = deck.indexOf('aria-label="Bàn làm việc cần làm ngay"');

  assert.match(deck, /data-project-dock-lane=\{projectID\}/);
  assert.match(deck, /ref=\{projectStripRef\} data-project-dock-strip className="pointer-events-auto ml-auto flex w-full items-end gap-2 overflow-x-auto sm:w-fit sm:max-w-full sm:snap-x sm:snap-proximity"/);
  assert.match(deck, /<ProjectConversation project=\{project\} \/>/);
  assert.match(deck, /responsiveWindowClass\(desktopExpanded, mobileExpanded\)/);
  assert.match(deck, /sm:w-\[336px\] sm:flex-none sm:snap-start/);
  assert.doesNotMatch(deck, /sm:flex-1 sm:basis-0/);
  assert.doesNotMatch(deck, /sm:w-\[148px\]/);
  assert.doesNotMatch(deck, /sm:w-\[clamp\(220px,calc\(20vw-2\.2rem\),260px\)\]/);
  assert.ok((deck.match(/h-\[min\(72vh,680px\)\] w-full/g) ?? []).length >= 2, 'project and work windows must fill their lanes');
  assert.match(deck, /hidden h-11 w-full min-w-0 items-center/);
  assert.ok(projectsIndex >= 0, 'desktop deck must render open project lanes');
  assert.ok(projectsIndex < discussionIndex && discussionIndex < workIndex, 'desktop order must be projects, add discussion, then work');
  assert.match(deck, /toggleDesktopDock\(dockKey\)/);
  assert.match(deck, /Đóng tab \$\{project\.project_code\}/);
  assert.match(deck, /aria-expanded=\{pickerOpen\}/);
  assert.match(dock, /const compactUtilityLaunchers = openProjectIDs\.length >= 4/);
  assert.match(dock, /compactUtilityLaunchers \? 'w-14 px-1' : 'px-4'/);
  assert.match(dock, /viewportWidth >= 1280\) return 5/);
  assert.match(dock, /desktopPanelCapacity\(window\.innerWidth\)/);
  assert.match(dock, /current\.slice\(-desktopCapacity\)/);
  assert.match(dock, /const projectStripRef = useRef<HTMLDivElement>\(null\)/);
  assert.match(dock, /const \[projectToReveal, setProjectToReveal\] = useState\(''\)/);
  assert.match(dock, /lane\?\.scrollIntoView\(\{ behavior: 'smooth', block: 'nearest', inline: 'nearest' \}\)/);
  assert.match(dock, /setProjectToReveal\(projectID\)/);
  assert.match(dock, /Đã mở tối đa 5 công trình/);
  assert.doesNotMatch(dock, /items-end justify-end gap-2 overflow-hidden/);
  assert.doesNotMatch(dock, /current\.slice\(-\(maxDiscussionTabs - 1\)\)/);
});

test('discussion picker is separate, preserves mobile switching and supports explicit close all', async () => {
  const dock = await source('../src/modules/utilities/UtilityDock.tsx');

  assert.match(dock, /function ProjectPicker\(/);
  assert.match(dock, /role="dialog"\s+aria-label="Mở trao đổi công trình"/);
  assert.match(dock, /Đóng tất cả khung trao đổi/);
  assert.match(dock, /setExpandedDockKeys\(\(current\) => current\.filter\(\(key\) => key === 'work'\)\)/);
  assert.match(dock, /function MobileProjectTabs\(/);
  assert.match(dock, /mobilePanel === 'discussion' && mobileProjectID === projectID/);
  assert.match(dock, /setMobileProjectID\(nextProjectID\)/);
  assert.match(dock, /workExpanded \? 'sm:right-\[328px\]' : compactUtilityLaunchers \? 'sm:right-16' : 'sm:right-\[148px\]'/);
});

test('Utility Dock keeps discussion unread separate and reuses canonical work sources', async () => {
  const [dock, realtime, composer] = await Promise.all([
    source('../src/modules/utilities/UtilityDock.tsx'),
    source('../src/modules/notifications/ActorRealtimeProvider.tsx'),
    source('../src/modules/work-items/ActivityComposer.tsx'),
  ]);

  assert.match(dock, /\/me\/project-discussions\?limit=50/);
  assert.match(dock, /\/projects\/\$\{projectID\}\/activity\?kind=discussion&limit=30/);
  assert.match(dock, /\/projects\/\$\{projectID\}\/activity\/read-through/);
  assert.match(dock, /item\.is_discussion_unread/);
  assert.match(dock, /enabled: Boolean\(project\.can_post\)/);
  assert.match(dock, /variant="chat"\s+density="compact"/);
  assert.match(composer, /density\?: 'comfortable' \| 'compact'/);
  assert.match(composer, /variant === 'chat' && density === 'compact'/);
  assert.match(composer, /Math\.min\(Math\.max\(input\.scrollHeight, 44\), 112\)/);
  assert.match(composer, /Gắn vào: \{targetSummary\}/);
  assert.match(composer, /Nhắc đến thành viên hoặc vai trò/);
  assert.match(dock, /workSummary\.data\?\.work_total/);
  assert.match(dock, /\/me\/work-queue\?queue=responsible&bucket=need_action&sort=priority&limit=6/);
  assert.match(dock, /orderedWorkListEntries/);
  assert.match(dock, /executionTargetPath/);
  assert.doesNotMatch(dock, /pipeline-run-nodes\/\$\{[^}]+\}\/advance/);
  assert.match(realtime, /queryKey: \['project-discussions'\]/);
  assert.match(realtime, /queryKey: \['utility-project-activity'\]/);
});
