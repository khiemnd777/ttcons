import { currency } from '../lib/utils/format';

export function CurrencyText({ value }: { value: unknown }) {
  return <span>{currency(value)}</span>;
}
