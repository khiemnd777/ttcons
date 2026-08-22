import assert from 'node:assert/strict';
import test from 'node:test';
import {
  generateSuggestedPassword,
  passwordConfirmationMessage,
  passwordValidationMessage,
  SUGGESTED_PASSWORD_LENGTH,
} from '../src/lib/security/password.ts';

test('password policy counts Unicode code points and UTF-8 bytes consistently', () => {
  assert.match(passwordValidationMessage('Abc12!x') ?? '', /ít nhất 8 ký tự/);
  assert.equal(passwordValidationMessage('Abc12!xy'), undefined);
  assert.match(passwordValidationMessage('        ') ?? '', /khoảng trắng/);
  assert.match(passwordValidationMessage('😀😀😀😀') ?? '', /ít nhất 8 ký tự/);
  assert.equal(passwordValidationMessage('😀'.repeat(8)), undefined);
  assert.equal(passwordValidationMessage('a'.repeat(72)), undefined);
  assert.match(passwordValidationMessage('a'.repeat(73)) ?? '', /72 byte/);
  assert.match(passwordValidationMessage('😀'.repeat(19)) ?? '', /72 byte/);
});

test('password confirmation uses one exact-match rule', () => {
  assert.equal(passwordConfirmationMessage('Abc12!xy', 'Abc12!xy'), undefined);
  assert.equal(passwordConfirmationMessage('', 'Abc12!xy'), 'Nhập lại mật khẩu mới');
  assert.equal(passwordConfirmationMessage('Abc12!xz', 'Abc12!xy'), 'Mật khẩu xác nhận không khớp');
});

test('suggested passwords are strong, policy-valid and generated at the approved length', () => {
  assert.equal(SUGGESTED_PASSWORD_LENGTH, 8);
  for (let index = 0; index < 64; index += 1) {
    const password = generateSuggestedPassword();
    assert.equal(password.length, SUGGESTED_PASSWORD_LENGTH);
    assert.match(password, /[A-Z]/);
    assert.match(password, /[a-z]/);
    assert.match(password, /[0-9]/);
    assert.match(password, /[!@#$%^&*]/);
    assert.equal(passwordValidationMessage(password), undefined);
  }
});
