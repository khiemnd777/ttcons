import type { ReactNode } from 'react';

/** Keep the last word and its help together, without preventing the label from wrapping. */
export function LabelWithHelp({ text, help, required }: { text: string; help: ReactNode; required?: boolean }) {
  const match = /^(.*\s)?(\S+)\s*$/s.exec(text);
  return <>{match?.[1]}<span className="inline-flex items-center whitespace-nowrap align-baseline" data-label-help-tail>
    <span>{match?.[2] ?? text}{required ? <span className="ml-1 text-rose-600">*</span> : null}</span>{help}
  </span></>;
}
