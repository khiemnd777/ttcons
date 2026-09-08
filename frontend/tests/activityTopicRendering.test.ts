import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

// Server render the real card/provider. No browser, requests, DOM or simulated source data reads.
const bundle = await build({
  entryPoints: [fileURLToPath(new URL('../src/modules/work-items/ActivityTopic.tsx', import.meta.url))],
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic', logLevel: 'silent',
  loader: { '.css': 'text' },
  plugins: [{ name: 'topic-render-environment', setup(build) {
    build.onResolve({ filter: /\/auth\/AuthContext$/ }, () => ({ path: 'auth', namespace: 'topic-test' }));
    build.onResolve({ filter: /\.\/ActivityTopicDetails$/ }, () => ({ path: 'details', namespace: 'topic-test' }));
    build.onLoad({ filter: /.*/, namespace: 'topic-test' }, ({ path }) => ({
      contents: path === 'auth' ? 'export const useAuth = () => ({ user: { id: "actor" } });' : 'export default function Details() { throw new Error("Closed topic must not render details"); }',
      loader: 'js',
    }));
  } }],
});
const module = { exports: {} as Record<string, any> };
new Function('require', 'module', 'exports', bundle.outputFiles[0].text)(createRequire(import.meta.url), module, module.exports);
const { ActivityTopicCard, ActivityTopicProvider } = module.exports;
const projectId = '74c409ae-eb91-434a-baa8-7b3fa4422b9a';
const recordId = '11111111-1111-4111-8111-111111111111';
const render = (source: object) => renderToStaticMarkup(createElement(ActivityTopicProvider, { projectId }, createElement(ActivityTopicCard, { source })));

test('BOQ topic renders a named keyboard-accessible dialog button, visibly separate from message metadata', () => {
  const html = render({ scope_type: 'boq_item', scope_id: recordId, target_label: 'Sơn nội thất [MẪU]' });
  assert.match(html, /<button[^>]+type="button"/);
  assert.match(html, /aria-haspopup="dialog"/);
  assert.match(html, /aria-label="Mở Dòng BOQ: Sơn nội thất \[MẪU\]"/);
  assert.match(html, /bg-blue-50/);
  assert.match(html, /focus-visible:ring/);
  assert.match(html, /break-words/);
  assert.doesNotMatch(html, /Thành viên công trình|<a /);
});

test('project context is omitted while broken historical references stay understandable without a fake action', () => {
  assert.equal(render({ scope_type: 'project', scope_id: projectId, target_label: 'Tên công trình' }), '');
  const html = render({ scope_type: 'boq_item', target_label: 'Sơn nội thất' });
  assert.match(html, /Sơn nội thất/);
  assert.match(html, /không còn khả dụng/);
  assert.doesNotMatch(html, /<button|<a /);
});

test('source labels are escaped and remain ordinary text', () => {
  const html = render({ scope_type: 'procurement_request', scope_id: recordId, target_label: '<img src=x onerror=alert(1)>' });
  assert.match(html, /&lt;img/);
  assert.doesNotMatch(html, /<img/);
});
