import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import test from 'node:test';
import ts from 'typescript';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { createFormControl } from 'react-hook-form';
import { passwordValueSchema, passwordConfirmationMessage } from '../src/lib/security/password.ts';

// Evaluate the actual page schema declarations without mounting React or copying validation rules.
function pageSchemas(file: string, names: string[]) {
  const source = ts.createSourceFile(file, readFileSync(new URL(`../src/${file}`, import.meta.url), 'utf8'), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const declarations = source.statements.filter((node) =>
    ts.isFunctionDeclaration(node) && names.includes(node.name?.text ?? '') ||
    ts.isVariableStatement(node) && node.declarationList.declarations.some((declaration) => names.includes(declaration.name.getText(source))),
  ).map((node) => node.getText(source)).join('\n');
  const code = ts.transpileModule(`${declarations}\nmodule.exports = { ${names.join(',')} };`, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText;
  const context = { z, passwordValueSchema, passwordConfirmationMessage, module: { exports: {} } };
  vm.runInNewContext(code, context, { timeout: 1000 });
  return context.module.exports as Record<string, z.ZodType | ((email: string) => z.ZodType)>;
}

const schemas = [
  ['login', pageSchemas('modules/auth/LoginPage.tsx', ['loginSchema']).loginSchema],
  ['forgot', pageSchemas('modules/auth/ForgotPasswordPage.tsx', ['schema']).schema],
  ['reset', pageSchemas('modules/auth/ResetPasswordPage.tsx', ['schema']).schema],
  ['initial', pageSchemas('modules/auth/InitialPasswordChangePage.tsx', ['schema']).schema],
] as [string, z.ZodType][];
const profile = pageSchemas('modules/profile/ProfilePage.tsx', ['passwordSchema', 'recoveryEmailSchema', 'removeRecoveryEmailSchema', 'profileSchema', 'validProfilePhone']);
schemas.push(['password', profile.passwordSchema as z.ZodType], ['recovery', profile.recoveryEmailSchema as z.ZodType],
  ['removeRecovery', profile.removeRecoveryEmailSchema as z.ZodType], ['profile', (profile.profileSchema as (email: string) => z.ZodType)('worker@example.com')]);
const base = { email: 'worker@example.com', password: 'Current123!', current_password: 'Current123!', new_password: 'Newpass123!', new_password_confirmation: 'Newpass123!', recovery_email: 'recovery@example.com', full_name: 'Nguyễn Văn An', phone: '0901234567' };

for (const [name, schema] of schemas) {
  test(`${name}: resolver returns field errors in Vietnamese and handleSubmit blocks invalid submission`, async () => {
    const resolver = zodResolver(schema);
    const valid = await resolver(base, {}, { fields: {}, shouldUseNativeValidation: false });
    assert.deepEqual(valid.errors, {});
    const invalid = { ...base, email: 'invalid', password: '', current_password: '', new_password: 'short', new_password_confirmation: 'mismatch', recovery_email: 'invalid', full_name: '', phone: 'abc' };
    const rejected = await resolver(invalid, {}, { fields: {}, shouldUseNativeValidation: false });
    assert.ok(Object.keys(rejected.errors).length > 0);
    for (const error of Object.values(rejected.errors)) assert.match(String(error?.message), /[À-ỹ]/);
    let submitted = 0;
    let invalidCalled = 0;
    const form = createFormControl({ defaultValues: invalid, resolver });
    await form.handleSubmit(() => { submitted++; }, () => { invalidCalled++; })();
    assert.equal(submitted, 0);
    assert.equal(invalidCalled, 1);
    const validForm = createFormControl({ defaultValues: base, resolver });
    await validForm.handleSubmit(() => { submitted++; })();
    assert.equal(submitted, 1);
  });
}

test('password schemas preserve Unicode, UTF-8, confirmation and current-password rules', () => {
  for (const name of ['reset', 'initial', 'password']) {
    const schema = schemas.find(([key]) => key === name)![1];
    for (const [password, accepted] of [['a'.repeat(7), false], ['a'.repeat(8), true], ['a'.repeat(72), true], ['a'.repeat(73), false], ['😀'.repeat(7), false], ['😀'.repeat(8), true], ['😀'.repeat(18), true], ['😀'.repeat(18) + 'a', false], ['        ', false]] as const) {
      assert.equal(schema.safeParse({ ...base, new_password: password, new_password_confirmation: password }).success, accepted, `${name}: ${password}`);
    }
    assert.equal(schema.safeParse({ ...base, new_password_confirmation: 'Different123!' }).success, false);
    if (name !== 'reset') assert.equal(schema.safeParse({ ...base, new_password: base.current_password, new_password_confirmation: base.current_password }).success, false);
  }
});

test('profile retains conditional email authorization, trim and phone/name constraints', () => {
  const schema = schemas.find(([name]) => name === 'profile')![1];
  assert.equal(schema.safeParse({ ...base, current_password: '' }).success, true);
  const changed = schema.safeParse({ ...base, email: 'other@example.com', current_password: '' });
  assert.equal(changed.success, false);
  if (!changed.success) assert.equal(changed.error.issues[0].message, 'Nhập mật khẩu hiện tại để đổi email');
  assert.equal(schema.safeParse({ ...base, email: 'other@example.com' }).success, true);
  const trimmed = schema.parse({ ...base, full_name: '  Nguyễn Văn An  ', email: ' worker@example.com ', phone: ' +84 (90) 123-4567 ' }) as typeof base;
  assert.equal(trimmed.full_name, base.full_name);
  assert.equal(trimmed.email, base.email);
  for (const phone of ['1234567', '1'.repeat(16), 'abc']) assert.equal(schema.safeParse({ ...base, phone }).success, false);
  for (const phone of ['', '1'.repeat(8), '1'.repeat(15)]) assert.equal(schema.safeParse({ ...base, phone }).success, true);
  for (const full_name of ['a', 'a'.repeat(121)]) assert.equal(schema.safeParse({ ...base, full_name }).success, false);
});

test('auth/profile email schemas preserve the pre-upgrade acceptance corpus', () => {
  // Expected results were captured with lockfile Zod3.25.76 before upgrading.
  const corpus = [
    ['name@example.com', true], ['name+tag@example.com', true],
    ['a..b@example.com', false], ['"quoted"@example.com', false],
    ['user@localhost', false], ['user@foo.c', false], ['üser@example.com', false],
    ['user@xn--bcher-kva.example', true], ['user@domain.123', false],
  ] as const;
  for (const name of ['login', 'forgot', 'recovery', 'profile']) {
    const schema = schemas.find(([key]) => key === name)![1];
    for (const [email, accepted] of corpus) {
      assert.equal(schema.safeParse({ ...base, email, recovery_email: email }).success, accepted, `${name}: ${email}`);
    }
  }
});
