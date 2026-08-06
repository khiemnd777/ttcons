import assert from 'node:assert/strict';
import test from 'node:test';
import { currencyCaretPosition, formatCurrencyInputValue, parseCurrencyInput, resolveCurrencyFractionDigits } from '../src/lib/utils/currencyInput.ts';

test('formats canonical monetary values with vi-VN grouping without forcing zero decimals', () => {
  assert.deepEqual(formatCurrencyInputValue(3100000000), {
    numericString: '3100000000',
    formattedValue: '3.100.000.000',
    floatValue: 3100000000,
  });
  assert.equal(formatCurrencyInputValue('125500.50').formattedValue, '125.500,50');
});

test('parses vi-VN and common pasted currency formats into one canonical numeric string', () => {
  assert.equal(parseCurrencyInput('3.100.000.000,50').numericString, '3100000000.50');
  assert.equal(parseCurrencyInput('3,100,000,000.50').numericString, '3100000000.50');
  assert.equal(parseCurrencyInput('VND 3 100 000 000').numericString, '3100000000');
});

test('uses ISO minor units while forcing VND to integer input and display', () => {
  assert.equal(resolveCurrencyFractionDigits('VND'), 0);
  assert.equal(resolveCurrencyFractionDigits('vnd', 2), 0);
  assert.equal(resolveCurrencyFractionDigits('USD'), 2);
  assert.equal(resolveCurrencyFractionDigits('JPY'), 0);
  assert.equal(resolveCurrencyFractionDigits('KWD'), 3);
  assert.equal(resolveCurrencyFractionDigits('USD', 4), 4);

  const vndFractionDigits = resolveCurrencyFractionDigits('VND');
  assert.deepEqual(formatCurrencyInputValue('2850000000.00', { maximumFractionDigits: vndFractionDigits }), {
    numericString: '2850000000',
    formattedValue: '2.850.000.000',
    floatValue: 2850000000,
  });
  assert.equal(parseCurrencyInput('2.850.000.000,00', { maximumFractionDigits: vndFractionDigits }).numericString, '2850000000');
  assert.equal(parseCurrencyInput('1,5', { maximumFractionDigits: vndFractionDigits }).numericString, '1');
  assert.equal(parseCurrencyInput('1.00', { maximumFractionDigits: vndFractionDigits }).numericString, '100');
  assert.equal(parseCurrencyInput('1.0000', { maximumFractionDigits: vndFractionDigits }).numericString, '10000');
});

test('supports explicit negative values and clamps the configured fraction length', () => {
  assert.equal(parseCurrencyInput('-', { allowNegative: true }).numericString, '-');
  assert.equal(formatCurrencyInputValue('-', { allowNegative: true }).formattedValue, '-');
  assert.equal(parseCurrencyInput('-125.500,567', { allowNegative: true, maximumFractionDigits: 2 }).numericString, '-125500.56');
  assert.equal(parseCurrencyInput('-125.500', { allowNegative: false }).numericString, '125500');
});

test('preserves an empty value and maps a digit-relative caret across grouping separators', () => {
  assert.deepEqual(parseCurrencyInput(''), { numericString: '', formattedValue: '', floatValue: null });
  assert.equal(currencyCaretPosition('3.100.000', 4), 5);
});
