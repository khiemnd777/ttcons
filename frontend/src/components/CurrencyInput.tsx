import { forwardRef, useEffect, useRef, useState, type FocusEvent, type InputHTMLAttributes } from 'react';
import clsx from 'clsx';
import { currencyCaretPosition, formatCurrencyInputValue, parseCurrencyInput, resolveCurrencyFractionDigits, type CurrencyInputParts } from '../lib/utils/currencyInput';

export type CurrencyInputProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'value' | 'onChange' | 'inputMode'> & {
  value: string | number | null | undefined;
  onValueChange: (value: CurrencyInputParts) => void;
  currencyCode?: string;
  allowNegative?: boolean;
  maximumFractionDigits?: number;
  invalid?: boolean;
  inputClassName?: string;
};

export const CurrencyInput = forwardRef<HTMLInputElement, CurrencyInputProps>(function CurrencyInput({
  value,
  onValueChange,
  currencyCode = 'VND',
  allowNegative = false,
  maximumFractionDigits,
  invalid = false,
  className,
  inputClassName,
  disabled,
  readOnly,
  onBlur,
  ...inputProps
}, forwardedRef) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const lastEmittedValue = useRef<string | null>(null);
  const resolvedFractionDigits = resolveCurrencyFractionDigits(currencyCode, maximumFractionDigits);
  const [displayValue, setDisplayValue] = useState(() => formatCurrencyInputValue(value, { allowNegative, maximumFractionDigits: resolvedFractionDigits }).formattedValue);

  useEffect(() => {
    const formatted = formatCurrencyInputValue(value, { allowNegative, maximumFractionDigits: resolvedFractionDigits });
    if (lastEmittedValue.current === formatted.numericString) return;
    if (lastEmittedValue.current !== null && formatted.numericString !== '') {
      const emittedNumber = Number(lastEmittedValue.current);
      const propNumber = Number(formatted.numericString);
      if (Number.isFinite(emittedNumber) && emittedNumber === propNumber) return;
    }
    lastEmittedValue.current = null;
    setDisplayValue(formatted.formattedValue);
  }, [allowNegative, resolvedFractionDigits, value]);

  function assignRef(node: HTMLInputElement | null) {
    inputRef.current = node;
    if (typeof forwardedRef === 'function') forwardedRef(node);
    else if (forwardedRef) forwardedRef.current = node;
  }

  function handleChange(rawValue: string, selectionStart: number | null) {
    const digitsBeforeCaret = rawValue.slice(0, selectionStart ?? rawValue.length).replace(/\D/g, '').length;
    const parsed = parseCurrencyInput(rawValue, { allowNegative, maximumFractionDigits: resolvedFractionDigits });
    lastEmittedValue.current = parsed.numericString;
    setDisplayValue(parsed.formattedValue);
    onValueChange(parsed);
    window.requestAnimationFrame(() => {
      const node = inputRef.current;
      if (!node || document.activeElement !== node) return;
      const caret = currencyCaretPosition(parsed.formattedValue, digitsBeforeCaret);
      node.setSelectionRange(caret, caret);
    });
  }

  function handleBlur(event: FocusEvent<HTMLInputElement>) {
    const formatted = formatCurrencyInputValue(lastEmittedValue.current ?? value, { allowNegative, maximumFractionDigits: resolvedFractionDigits });
    setDisplayValue(formatted.formattedValue);
    onBlur?.(event);
  }

  return (
    <span className={clsx('relative block min-w-0', className)}>
      <input
        {...inputProps}
        ref={assignRef}
        type="text"
        inputMode="decimal"
        autoComplete="off"
        className={clsx(
          'w-full rounded-md border bg-white py-2 pl-3 !pr-16 text-right tabular-nums outline-none focus:ring-2 focus:ring-blue-100',
          invalid ? 'border-rose-300 bg-rose-50' : 'border-line',
          (disabled || readOnly) && 'cursor-not-allowed bg-slate-100 text-slate-500',
          inputClassName,
        )}
        value={displayValue}
        disabled={disabled}
        readOnly={readOnly}
        aria-invalid={invalid || undefined}
        onChange={(event) => handleChange(event.target.value, event.target.selectionStart)}
        onBlur={handleBlur}
      />
      <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs font-semibold text-slate-500" aria-hidden="true">
        {currencyCode}
      </span>
    </span>
  );
});
