export type CurrencyInputParts = {
  numericString: string;
  formattedValue: string;
  floatValue: number | null;
};

type CurrencyInputOptions = {
  allowNegative?: boolean;
  maximumFractionDigits?: number;
};

function clampFractionDigits(value: number | undefined) {
  if (value === undefined || !Number.isFinite(value)) return 2;
  return Math.max(0, Math.min(6, Math.trunc(value)));
}

export function resolveCurrencyFractionDigits(currencyCode = 'VND', configuredFractionDigits?: number) {
  const normalizedCode = currencyCode.trim().toUpperCase() || 'VND';
  if (normalizedCode === 'VND') return 0;
  if (configuredFractionDigits !== undefined) return clampFractionDigits(configuredFractionDigits);
  try {
    return clampFractionDigits(new Intl.NumberFormat('vi-VN', {
      style: 'currency',
      currency: normalizedCode,
    }).resolvedOptions().maximumFractionDigits);
  } catch {
    return 2;
  }
}

function splitUserInput(value: string, maximumFractionDigits: number) {
  const separators = [...value].map((character, index) => ({ character, index })).filter(({ character }) => character === '.' || character === ',');
  if (!separators.length) return { integerDigits: value.replace(/\D/g, ''), fractionDigits: '', hasDecimal: false };

  const last = separators[separators.length - 1];
  const trailingDigits = value.slice(last.index + 1).replace(/\D/g, '');
  const repeatedSameSeparator = separators.length > 1 && separators.every(({ character }) => character === last.character);
  const looksGrouped = trailingDigits.length === 3 && (repeatedSameSeparator || maximumFractionDigits < 3);
  const hasMixedSeparators = separators.some(({ character }) => character !== last.character);
  const unsupportedFraction = maximumFractionDigits === 0 && last.character === ',' && trailingDigits.length > 0 && trailingDigits.length < 3;
  const hasDecimal = hasMixedSeparators || unsupportedFraction || (!looksGrouped && trailingDigits.length <= maximumFractionDigits);

  if (!hasDecimal) {
    return { integerDigits: value.replace(/\D/g, ''), fractionDigits: '', hasDecimal: false };
  }
  return {
    integerDigits: value.slice(0, last.index).replace(/\D/g, ''),
    fractionDigits: trailingDigits.slice(0, maximumFractionDigits),
    hasDecimal: true,
  };
}

function groupInteger(value: string) {
  const normalized = value.replace(/^0+(?=\d)/, '') || '0';
  return normalized.replace(/\B(?=(\d{3})+(?!\d))/g, '.');
}

function result(numericString: string, formattedValue: string): CurrencyInputParts {
  const floatValue = numericString === '' || numericString === '-' ? null : Number(numericString);
  return {
    numericString,
    formattedValue,
    floatValue: floatValue !== null && Number.isFinite(floatValue) ? floatValue : null,
  };
}

export function parseCurrencyInput(value: string, options: CurrencyInputOptions = {}): CurrencyInputParts {
  const maximumFractionDigits = clampFractionDigits(options.maximumFractionDigits);
  const trimmed = value.trim();
  if (!trimmed) return result('', '');
  if (trimmed === '-' && options.allowNegative) return result('-', '-');

  const negative = Boolean(options.allowNegative && /^\s*-/.test(value));
  const { integerDigits, fractionDigits, hasDecimal } = splitUserInput(trimmed, maximumFractionDigits);
  if (!integerDigits && !fractionDigits && !hasDecimal) return result('', '');

  const integer = (integerDigits || '0').replace(/^0+(?=\d)/, '') || '0';
  const sign = negative ? '-' : '';
  const numericString = `${sign}${integer}${hasDecimal && maximumFractionDigits > 0 ? `.${fractionDigits}` : ''}`;
  const formattedValue = `${sign}${groupInteger(integer)}${hasDecimal && maximumFractionDigits > 0 ? `,${fractionDigits}` : ''}`;
  return result(numericString, formattedValue);
}

export function formatCurrencyInputValue(value: string | number | null | undefined, options: CurrencyInputOptions = {}): CurrencyInputParts {
  if (value === null || value === undefined || value === '') return result('', '');
  const maximumFractionDigits = clampFractionDigits(options.maximumFractionDigits);
  const raw = String(value).trim();
  if (raw === '-' && options.allowNegative) return result('-', '-');
  const negative = Boolean(options.allowNegative && raw.startsWith('-'));
  const unsigned = raw.replace(/^[+-]/, '');
  const [integerPart = '0', rawFraction = ''] = unsigned.split('.', 2);
  const integerDigits = integerPart.replace(/\D/g, '') || '0';
  const fractionDigits = rawFraction.replace(/\D/g, '').slice(0, maximumFractionDigits);
  const sign = negative ? '-' : '';
  const numericString = `${sign}${integerDigits.replace(/^0+(?=\d)/, '') || '0'}${fractionDigits ? `.${fractionDigits}` : ''}`;
  const formattedValue = `${sign}${groupInteger(integerDigits)}${fractionDigits ? `,${fractionDigits}` : ''}`;
  return result(numericString, formattedValue);
}

export function currencyCaretPosition(formattedValue: string, digitCount: number) {
  if (digitCount <= 0) return formattedValue.startsWith('-') ? 1 : 0;
  let seen = 0;
  for (let index = 0; index < formattedValue.length; index += 1) {
    if (/\d/.test(formattedValue[index])) seen += 1;
    if (seen >= digitCount) return index + 1;
  }
  return formattedValue.length;
}
