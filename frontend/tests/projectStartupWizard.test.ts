import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const source = (path: string) => readFile(new URL(path, import.meta.url), 'utf8');

test('manual project creation uses the three-step startup wizard', async () => {
  const wizard = await source('../src/modules/projects/ProjectStartupWizard.tsx');
  const projects = await source('../src/modules/projects/ProjectsPage.tsx');
  const fields = await source('../src/modules/projects/projectFormFields.ts');

  assert.match(wizard, /label: 'Thông tin'/);
  assert.match(wizard, /label: 'Chọn nhân sự'/);
  assert.match(wizard, /label: 'Thiết lập'/);
  assert.match(projects, /onCreateProject=\{\(values\) => createProject\.mutateAsync\(values\)\}/);
  assert.match(projects, /description: 'Tiếp theo: chọn nhân sự tham gia công trình\.'/);
  assert.match(fields, /export function buildProjectFormFields/);
  assert.match(fields, /useProjectFormCatalogs/);
});

test('completed startup steps navigate backward and resumed setup reloads editable project data', async () => {
  const wizard = await source('../src/modules/projects/ProjectStartupWizard.tsx');
  const domain = await source('../src/lib/types/domain.ts');

  assert.match(wizard, /queryKey: \['project', activeProjectID\]/);
  assert.match(wizard, /get<Project>\(`\/projects\/\$\{activeProjectID\}`\)/);
  assert.match(wizard, /can_update_project/);
  assert.match(wizard, /Quay lại Thông tin/);
  assert.match(wizard, /Quay lại Chọn nhân sự/);
  assert.match(wizard, /aria-label=\{`Quay lại bước \$\{item\.label\}`\}/);
  assert.match(wizard, /Lưu thay đổi & tiếp tục/);
  assert.match(wizard, /setStep\(1\)/);
  assert.match(wizard, /setStep\(2\)/);
  assert.match(domain, /can_update_project: boolean/);
});

test('member selection is searchable, checkbox-based, atomic, and keeps saved members locked', async () => {
  const wizard = await source('../src/modules/projects/ProjectStartupWizard.tsx');

  assert.match(wizard, /normalizeMemberSearch\(search\)/);
  assert.match(wizard, /type="checkbox"/);
  assert.match(wizard, /checked disabled/);
  assert.match(wizard, /members\/bulk/);
  assert.match(wizard, /selectedDrafts\.length/);
  assert.match(wizard, /Mỗi lần có thể thêm tối đa 50 thành viên/);
  assert.doesNotMatch(wizard, /DELETE.*members/i);
});

test('wizard requires valid access and a PM-eligible member before setup', async () => {
  const wizard = await source('../src/modules/projects/ProjectStartupWizard.tsx');

  assert.match(wizard, /invalidDrafts\.length === 0 && hasPMEligibleMember/);
  assert.match(wizard, /grants_project_view/);
  assert.match(wizard, /grants_project_member_manage/);
  assert.match(wizard, /grants_pipeline_run/);
  assert.match(wizard, /eligibleActiveMembers/);
  assert.match(wizard, /disabled=\{!readiness\.data\.ready_to_start/);
});

test('setup shows the full saved team and whole-template operational role coverage', async () => {
  const wizard = await source('../src/modules/projects/ProjectStartupWizard.tsx');
  const domain = await source('../src/lib/types/domain.ts');

  assert.match(wizard, /teamMembers=\{savedActiveMembers\}/);
  assert.match(wizard, /Đội ngũ công trình/);
  assert.match(wizard, /Toàn bộ thành viên đã lưu và vai trò vận hành của họ/);
  assert.match(wizard, /member\.role_ids\.map/);
  assert.match(wizard, /Tài khoản ngừng/);
  assert.match(wizard, /Độ phủ vai trò quy trình/);
  assert.match(wizard, /data\.role_coverage \?\? \[\]/);
  assert.match(wizard, /Array\.isArray\(role\.members\)/);
  assert.match(wizard, /roleMembers\.map/);
  assert.match(wizard, /Còn thiếu/);
  assert.match(domain, /export type ProjectStartupRoleCoverage/);
  assert.match(domain, /role_coverage\?: ProjectStartupRoleCoverage\[\]/);
});
