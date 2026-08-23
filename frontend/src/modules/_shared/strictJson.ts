const FORBIDDEN_KEYS = new Set(['__proto__', 'prototype', 'constructor']);
const MAXIMUM_DEPTH = 64;

export function parseStrictJson(text: string): unknown {
  let index = 0;

  function skipWhitespace() {
    while (index < text.length && /\s/.test(text[index])) index += 1;
  }

  function parseString(): string {
    skipWhitespace();
    if (text[index] !== '"') throw new Error('JSON cần key hoặc chuỗi hợp lệ.');
    const start = index;
    index += 1;
    let escaped = false;
    while (index < text.length) {
      const character = text[index];
      index += 1;
      if (escaped) {
        escaped = false;
        continue;
      }
      if (character === '\\') {
        escaped = true;
        continue;
      }
      if (character === '"') {
        return JSON.parse(text.slice(start, index)) as string;
      }
      if (character.charCodeAt(0) < 0x20) throw new Error('JSON chứa ký tự điều khiển không hợp lệ.');
    }
    throw new Error('Chuỗi JSON chưa được đóng.');
  }

  function parsePrimitive() {
    skipWhitespace();
    const start = index;
    while (index < text.length && !/[\s,\]}]/.test(text[index])) index += 1;
    if (start === index) throw new Error('Giá trị JSON không hợp lệ.');
    JSON.parse(text.slice(start, index));
  }

  function parseValue(depth: number) {
    if (depth > MAXIMUM_DEPTH) throw new Error(`JSON vượt quá độ sâu ${MAXIMUM_DEPTH}.`);
    skipWhitespace();
    if (text[index] === '{') {
      parseObject(depth + 1);
      return;
    }
    if (text[index] === '[') {
      parseArray(depth + 1);
      return;
    }
    if (text[index] === '"') {
      parseString();
      return;
    }
    parsePrimitive();
  }

  function parseObject(depth: number) {
    index += 1;
    skipWhitespace();
    const keys = new Set<string>();
    if (text[index] === '}') {
      index += 1;
      return;
    }
    while (index < text.length) {
      const key = parseString();
      if (keys.has(key)) throw new Error(`JSON có key bị trùng: ${key}.`);
      if (FORBIDDEN_KEYS.has(key)) throw new Error(`JSON chứa key không được phép: ${key}.`);
      keys.add(key);
      skipWhitespace();
      if (text[index] !== ':') throw new Error(`JSON thiếu dấu : sau key ${key}.`);
      index += 1;
      parseValue(depth);
      skipWhitespace();
      if (text[index] === '}') {
        index += 1;
        return;
      }
      if (text[index] !== ',') throw new Error('JSON object thiếu dấu phẩy.');
      index += 1;
    }
    throw new Error('JSON object chưa được đóng.');
  }

  function parseArray(depth: number) {
    index += 1;
    skipWhitespace();
    if (text[index] === ']') {
      index += 1;
      return;
    }
    while (index < text.length) {
      parseValue(depth);
      skipWhitespace();
      if (text[index] === ']') {
        index += 1;
        return;
      }
      if (text[index] !== ',') throw new Error('JSON array thiếu dấu phẩy.');
      index += 1;
    }
    throw new Error('JSON array chưa được đóng.');
  }

  skipWhitespace();
  parseValue(0);
  skipWhitespace();
  if (index !== text.length) throw new Error('JSON chỉ được chứa một document.');
  return JSON.parse(text) as unknown;
}

export function isJsonRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

export function assertExactKeys(value: Record<string, unknown>, allowed: readonly string[], label: string) {
  const allowedSet = new Set(allowed);
  const unknown = Object.keys(value).filter((key) => !allowedSet.has(key));
  if (unknown.length) throw new Error(`${label} chứa key không được hỗ trợ: ${unknown.join(', ')}.`);
}
