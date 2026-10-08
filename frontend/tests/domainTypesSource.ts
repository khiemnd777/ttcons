import { readFile } from 'node:fs/promises';

const domainTypeFiles = [
  'common.ts',
  'projectsWorkflowDefinitions.ts',
  'commercial.ts',
  'constructionQuality.ts',
  'activityEvidence.ts',
  'reports.ts',
  'workflowForms.ts',
  'materialsWarehouse.ts',
  'workItems.ts',
  'projectFinance.ts',
];

export async function domainTypesSource() {
  const directory = new URL('../src/lib/types/domain/', import.meta.url);
  return Promise.all(domainTypeFiles.map((name) => readFile(new URL(name, directory), 'utf8')))
    .then((parts) => parts.join('\n'));
}
