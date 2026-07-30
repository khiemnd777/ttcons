import { ReactNode } from 'react';

export function PageHeader({
  title,
  description,
  actions,
  horizontalFrom = 'md',
}: {
  title: string;
  description?: string;
  actions?: ReactNode;
  horizontalFrom?: 'md' | 'xl' | '2xl';
}) {
  const responsiveLayout = {
    md: 'md:flex-row md:items-start md:justify-between',
    xl: 'xl:flex-row xl:items-start xl:justify-between',
    '2xl': '2xl:flex-row 2xl:items-start 2xl:justify-between',
  }[horizontalFrom];

  return (
    <div className={`mb-5 flex flex-col gap-3 border-b border-line pb-5 ${responsiveLayout}`}>
      <div>
        <h1 className="text-2xl font-semibold tracking-normal text-ink">{title}</h1>
        {description ? <p className="mt-1 max-w-3xl text-sm leading-6 text-slate-600">{description}</p> : null}
      </div>
      {actions ? <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div> : null}
    </div>
  );
}
