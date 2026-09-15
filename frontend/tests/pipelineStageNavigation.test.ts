import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';
import ts from 'typescript';
import { executionTargetPath } from '../src/modules/work-items/workItemExecutionTarget.ts';
import { projectPipelinePath, projectPipelineReturnTo } from '../src/modules/work-items/workItemNavigation.ts';
import { currentTimelineNode } from '../src/modules/projects/projectOperationsDomain.ts';
import type { WorkItem } from '../src/lib/types/domain.ts';

function source(path: string) {
  return ts.createSourceFile(path, readFileSync(new URL(path, import.meta.url), 'utf8'), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
}

function findNode(root: ts.Node, predicate: (node: ts.Node) => boolean): ts.Node | undefined {
  if (predicate(root)) return root;
  let found: ts.Node | undefined;
  root.forEachChild((child) => { if (!found) found = findNode(child, predicate); });
  return found;
}

function variableExpression(file: ts.SourceFile, name: string) {
  const declaration = findNode(file, (node) => ts.isVariableDeclaration(node) && node.name.getText(file) === name) as ts.VariableDeclaration | undefined;
  assert.ok(declaration?.initializer, `Missing ${name} initializer in ${file.fileName}`);
  return declaration.initializer.getText(file);
}

function attributeExpression(file: ts.SourceFile, name: string, root: ts.Node = file) {
  const attribute = findNode(root, (node) => ts.isJsxAttribute(node) && node.name.getText(file) === name) as ts.JsxAttribute | undefined;
  assert.ok(attribute?.initializer && ts.isJsxExpression(attribute.initializer) && attribute.initializer.expression);
  return attribute.initializer.expression.getText(file);
}

function linkTo(file: ts.SourceFile, label: string) {
  const link = findNode(file, (node) => ts.isJsxElement(node)
    && node.openingElement.tagName.getText(file) === 'Link'
    && node.children.some((child) => ts.isJsxText(child) && child.text.trim() === label));
  assert.ok(link, `Missing ${label} link`);
  return attributeExpression(file, 'to', link);
}

const preparation = source('../src/modules/planning/PreparationPage.tsx');
const planning = source('../src/modules/planning/PlanningPage.tsx');
const project = source('../src/modules/projects/ProjectDetailPage.tsx');
const taskContext = source('../src/modules/projects/ProjectTaskContext.tsx');
const projectId = '30000000-0000-0000-0000-000000000010';
const financeItem = {
  id: '43000000-0000-0000-0000-000000000022', project_id: projectId,
  run_node_id: 'fixture-finance', run_node_name: 'Tài chính & Công nợ', status: 'completed',
  execution_target: { workspace: 'preparation', route: `/projects/${projectId}/preparation?task=43000000-0000-0000-0000-000000000022` },
} as WorkItem;
const nodes = Object.freeze([
  Object.freeze({ id: 'fixture-construction', name: 'Thi công', status: 'in_progress' }),
  Object.freeze({ id: 'fixture-finance', name: 'Tài chính & Công nợ', status: 'pending' }),
]);
// Exercise the actual page selection expressions without running a browser or React effects.
function selectedStage(path: string) {
  const location = new URL(path, 'http://localhost');
  const selectedNodeId = vm.runInNewContext(variableExpression(planning, 'selectedNodeId'), { location, URLSearchParams });
  return vm.runInNewContext(variableExpression(project, 'selectedNode'), { nodes, selectedNodeId, currentNode: currentTimelineNode(nodes) });
}

test('Preparation Quy trình and Tiến độ both return to the viewed Finance stage', () => {
  const pipelinePath = vm.runInNewContext(variableExpression(preparation, 'pipelinePath'), { id: projectId, selectedItem: financeItem, projectPipelinePath });
  const progress = findNode(preparation, (node) => ts.isPropertyAssignment(node) && node.name.getText(preparation) === 'progressPath') as ts.PropertyAssignment;
  assert.ok(progress);
  for (const expression of [linkTo(preparation, 'Quy trình'), progress.initializer.getText(preparation)]) {
    const path = vm.runInNewContext(expression, { pipelinePath });
    assert.equal(selectedStage(path).id, financeItem.run_node_id);
  }
  assert.equal(nodes[0].status, 'in_progress');
  assert.equal(nodes[1].status, 'pending');
});

test('stage selection writes the URL and follows reload and history location changes', () => {
  let path = projectPipelinePath(projectId, 'fixture-construction');
  const paths = [path];
  for (const stageId of ['fixture-finance', 'fixture-construction']) {
    const location = { search: new URL(path, 'http://localhost').search, hash: '#workflow', state: { origin: 'project' } };
    const select = vm.runInNewContext(`(${attributeExpression(planning, 'onSelectNode')})`, {
      id: projectId, location, projectPipelinePath,
      navigate: (next: string, options: { state: unknown }) => { path = next; assert.deepEqual(options.state, location.state); },
    });
    select(stageId);
    assert.equal(selectedStage(path).id, stageId);
    paths.push(path);
  }
  // Replay old URLs as reload/Back/Forward would supply them to useLocation.
  for (const index of [1, 0, 1, 2]) {
    assert.equal(selectedStage(paths[index]).id, index === 1 ? 'fixture-finance' : 'fixture-construction');
  }
});

test('missing or stale stage IDs fall back within the current timeline', () => {
  for (const stageId of [undefined, '', 'node-from-another-project', 'node-from-an-old-run']) {
    assert.equal(selectedStage(projectPipelinePath(projectId, stageId)).id, 'fixture-construction');
  }
});

test('opening work returns to that work stage even when the summary opens a different stage', () => {
  let target = '';
  const open = vm.runInNewContext(`(${attributeExpression(planning, 'onOpenWork')})`, {
    id: projectId, location: { search: '?stage=fixture-construction', hash: '#workflow' },
    projectPipelinePath, executionTargetPath, navigate: (path: string) => { target = path; },
  });
  open(financeItem);
  const url = new URL(target, 'http://localhost');
  assert.equal(url.searchParams.get('task'), financeItem.id);
  assert.equal(selectedStage(url.searchParams.get('returnTo')!).id, 'fixture-finance');
});

test('Pipeline URLs preserve unrelated query/hash, encode identifiers and remove recursive task context', () => {
  const path = projectPipelinePath('project/1', 'stage & 6', {
    search: '?queue=all&stage=old&stage=duplicate&task=old-task&returnTo=%2Fold', hash: '#workflow',
  });
  const url = new URL(path, 'http://localhost');
  assert.equal(url.pathname, '/projects/project%2F1/pipeline');
  assert.deepEqual([...url.searchParams], [['queue', 'all'], ['stage', 'stage & 6']]);
  assert.equal(url.hash, '#workflow');
  assert.equal(projectPipelinePath(projectId, null, { search: '?stage=old' }), `/projects/${projectId}/pipeline`);
});

test('old and stale same-project Pipeline return links recover the current work stage', () => {
  for (const file of [preparation, taskContext]) {
    const params = new URLSearchParams({ returnTo: `/projects/${projectId}/pipeline?queue=all&stage=old&task=old&returnTo=%2Fold#workflow` });
    const path = vm.runInNewContext(variableExpression(file, 'returnTo'), {
      searchParams: params, params, id: projectId, projectId, selectedItem: financeItem, item: financeItem, projectPipelineReturnTo,
    });
    const url = new URL(path, 'http://localhost');
    assert.deepEqual([...url.searchParams], [['queue', 'all'], ['stage', 'fixture-finance']]);
    assert.equal(url.hash, '#workflow');
  }
  assert.equal(selectedStage(projectPipelineReturnTo(`/projects/${projectId}/pipeline`, projectId, financeItem.run_node_id)).id, 'fixture-finance');
});

test('return normalization leaves other destinations and unknown work context intact', () => {
  for (const path of ['/my-work?scope=mine', `/projects/${projectId}/preparation`, `/projects/${projectId}/planning?task=other`, '/projects/other/pipeline?stage=their-stage']) {
    assert.equal(projectPipelineReturnTo(path, projectId, financeItem.run_node_id), path);
  }
  const pipeline = `/projects/${projectId}/pipeline?stage=existing#workflow`;
  assert.equal(projectPipelineReturnTo(pipeline, projectId, undefined), pipeline);
  for (const path of [undefined, null, '', 'https://example.com', '//example.com', 'relative/path']) {
    assert.equal(projectPipelineReturnTo(path, projectId, financeItem.run_node_id), '');
  }
});

test('domain task context progress uses the same work stage as Preparation', () => {
  const path = vm.runInNewContext(linkTo(taskContext, 'Tiến độ'), { projectId, item: financeItem, projectPipelinePath });
  assert.equal(selectedStage(path).id, 'fixture-finance');
});
