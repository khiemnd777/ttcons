import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const bundle = await build({
  stdin: { resolveDir: fileURLToPath(new URL('../src/', import.meta.url)), contents: `
    export { DeveloperBootstrapPage } from './modules/auth/DeveloperBootstrapPage';
    export { DeveloperBootstrapGate } from './modules/auth/DeveloperBootstrapGate';
    export { ToastProvider } from './components/ToastProvider';` },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic',
  loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///test"' },
});
const module = { exports: {} as Record<string, any> };
new Function('require', 'module', 'exports', bundle.outputFiles[0].text)(createRequire(import.meta.url), module, module.exports);
const components = module.exports;

test('bootstrap initially shows fixed readonly email and send action', () => {
  const html = renderToStaticMarkup(createElement(components.ToastProvider, null, createElement(components.DeveloperBootstrapPage)));
  assert.match(html, /khiemnd777@gmail\.com/);
  assert.match(html, /<input readonly=""[^>]*value="khiemnd777@gmail\.com"/);
  assert.match(html, /Gửi mã xác minh/);
  assert.doesNotMatch(html, /Hoàn tất thiết lập/);
});

test('gate does not render application before status is known', () => {
  const html = renderToStaticMarkup(createElement(components.DeveloperBootstrapGate, null, 'Nội dung ứng dụng'));
  assert.doesNotMatch(html, /Nội dung ứng dụng/);
});
