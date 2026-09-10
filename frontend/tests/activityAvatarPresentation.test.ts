import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('project discussion renderers reuse the current-user avatar without exposing peer avatars', async () => {
  const [thread, dock, avatar] = await Promise.all([
    source('../src/modules/work-items/ActivityThread.tsx'),
    source('../src/modules/utilities/UtilityDock.tsx'),
    source('../src/components/UserAvatar.tsx'),
  ]);

  assert.match(thread, /import \{ UserAvatar \} from '\.\.\/\.\.\/components\/UserAvatar';/);
  assert.match(thread, /currentUser=\{user \?\? undefined\}/);
  assert.match(thread, /own && currentUser \? \(\s*<UserAvatar user=\{currentUser\} size="sm" decorative className=\{compact \? 'invisible' : undefined\} \/>/);
  assert.match(dock, /import \{ UserAvatar \} from '\.\.\/\.\.\/components\/UserAvatar';/);
  assert.match(dock, /own && user \? <UserAvatar user=\{user\} size="xs" decorative \/>/);
  assert.match(avatar, /xs: 'h-7 w-7 text-\[10px\]'/);
  assert.doesNotMatch(thread, /\/auth\/avatar\/\$\{item\.actor_id\}/);
  assert.doesNotMatch(dock, /\/auth\/avatar\/\$\{item\.actor_id\}/);
});
