import { z } from 'zod';

export const PASSWORD_MINIMUM_CODE_POINTS = 8;
export const PASSWORD_MAXIMUM_UTF8_BYTES = 72;
export const SUGGESTED_PASSWORD_LENGTH = 8;
export const PASSWORD_RULE_HELPER = 'Tối thiểu 8 ký tự Unicode, tối đa 72 byte và không chỉ gồm khoảng trắng.';

export function passwordValidationMessage(value: string, label = 'Mật khẩu mới') {
  if (value === '') return `${label} là bắt buộc`;
  if (value.trim() === '') return `${label} không được chỉ chứa khoảng trắng`;
  if (Array.from(value).length < PASSWORD_MINIMUM_CODE_POINTS) return `${label} phải có ít nhất 8 ký tự`;
  if (new TextEncoder().encode(value).length > PASSWORD_MAXIMUM_UTF8_BYTES) return `${label} không được vượt quá 72 byte`;
  return undefined;
}

export function passwordValueSchema(label = 'Mật khẩu mới') {
  return z.string().superRefine((value, context) => {
    const message = passwordValidationMessage(value, label);
    if (message) context.addIssue({ code: 'custom', message });
  });
}

export function passwordConfirmationMessage(value: string, password: string) {
  if (!value) return 'Nhập lại mật khẩu mới';
  if (value !== password) return 'Mật khẩu xác nhận không khớp';
  return undefined;
}

export function generateSuggestedPassword(length = SUGGESTED_PASSWORD_LENGTH) {
  if (!Number.isInteger(length) || length < 4) throw new Error('Độ dài mật khẩu đề xuất phải từ 4 ký tự');
  const groups = ['ABCDEFGHJKLMNPQRSTUVWXYZ', 'abcdefghijkmnopqrstuvwxyz', '23456789', '!@#$%^&*'];
  const alphabet = groups.join('');
  const characters = groups.map((group) => group[secureRandomIndex(group.length)]);
  while (characters.length < length) characters.push(alphabet[secureRandomIndex(alphabet.length)]);
  for (let index = characters.length - 1; index > 0; index -= 1) {
    const swapIndex = secureRandomIndex(index + 1);
    [characters[index], characters[swapIndex]] = [characters[swapIndex], characters[index]];
  }
  return characters.join('');
}

function secureRandomIndex(upperBound: number) {
  const limit = 256 - (256 % upperBound);
  const random = new Uint8Array(1);
  do {
    globalThis.crypto.getRandomValues(random);
  } while (random[0] >= limit);
  return random[0] % upperBound;
}
