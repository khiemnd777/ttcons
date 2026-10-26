// One-time acceptance check against the approved source baseline, not a permanent release gate.
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../../../', import.meta.url));
const require = createRequire(new URL('../../../frontend/package.json', import.meta.url));
const ts = require('typescript');
const baseline = process.argv[2] ?? 'fba39cdcf72a2d6714d5eac3d15eba8cdf4014c9';
const git = (...args) => execFileSync('git', args, { cwd: root, encoding: 'utf8' }).trimEnd();
const tracked = git('ls-tree', '-r', '--name-only', baseline, 'frontend/src').split('\n');
const exact = [
  'frontend/src/modules/planning/PlanningPage.tsx',
  'frontend/src/modules/planning/ConstructionHandoffPanel.tsx',
  'frontend/src/modules/work-items/WorkItemDetailDialog.tsx',
  'frontend/src/modules/work-items/WorkItemPrimarySurface.tsx',
  'frontend/src/modules/work-items/DiscussionWorkUpdateCard.tsx',
  'frontend/src/modules/work-items/DiscussionWorkflowActionCard.tsx',
  'frontend/src/modules/work-items/SignatureCaptureDialog.tsx',
  'frontend/src/modules/projects/ProjectStartupWizard.tsx',
];
const protectedFiles = tracked.filter(file =>
  file.startsWith('frontend/src/components/') ||
  (file.startsWith('frontend/src/modules/_shared/') && !file.endsWith('/RecordSourcePage.tsx')) ||
  /\/modules\/(pipeline-builder|workflow-forms|workflow-definitions)\//.test(file) ||
  /\/work-items\/Workflow/.test(file) || exact.includes(file));
const regions = {
  'frontend/src/modules/projects/ProjectDetailPage.tsx': [
    'ProjectWorkflowWorkspace', 'WorkflowEmptyState', 'WorkflowExecutionSummary', 'WorkflowSummaryMetric',
    'WorkflowStageRail', 'WorkflowWorkPanel', 'WorkflowWorkTableRow', 'WorkflowWorkMobileCard',
  ],
  'frontend/src/modules/my-work/MyWorkPage.tsx': [
    'WorkflowActionCard', 'NextQueuePanel', 'ProjectStartupActionCard', 'ProjectStartupNextPanel',
  ],
};
const before = file => execFileSync('git', ['show', `${baseline}:${file}`], { cwd: root, encoding: 'utf8' });
const current = file => fs.readFileSync(path.join(root, file), 'utf8');
const problems = protectedFiles.filter(file => current(file) !== before(file));
function namedFunctions(source) {
  const sourceFile = ts.createSourceFile('source.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const functions = new Map();
  function visit(node) {
    if (ts.isFunctionDeclaration(node) && node.name) functions.set(node.name.text, node.getText(sourceFile));
    ts.forEachChild(node, visit);
  }
  visit(sourceFile);
  return functions;
}
for (const [file, names] of Object.entries(regions)) {
  const previous = namedFunctions(before(file)), next = namedFunctions(current(file));
  for (const name of names) {
    if (!previous.has(name) || previous.get(name) !== next.get(name)) problems.push(`${file}::${name}`);
  }
}
const sourceFiles = git('ls-files', '--cached', '--others', '--exclude-standard', 'frontend/src', 'frontend/tests').split('\n').sort();
const digest = crypto.createHash('sha256');
for (const file of sourceFiles) digest.update(`${file}\0`).update(current(file)).update('\0');
console.log(JSON.stringify({
  baseline, checked_at: new Date().toISOString(), node: process.version,
  protected_files: protectedFiles.length, protected_regions: Object.values(regions).flat().length,
  regions, problems, source_test_sha256: digest.digest('hex'),
}, null, 2));
if (problems.length) process.exitCode = 1;
