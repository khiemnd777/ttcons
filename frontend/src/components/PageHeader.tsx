import { ReactNode } from 'react';

export function PageHeader({
  title,
  description,
  actions,
  horizontalFrom = 'md',
  density = 'standard',
}: {
  title: string;
  description?: string;
  actions?: ReactNode;
  horizontalFrom?: 'md' | 'xl' | '2xl';
  density?: 'standard' | 'compact';
}) {
  const compact = density === 'compact';
  const responsiveLayout = {
    md: compact ? 'md:flex-row md:items-center md:justify-between' : 'md:flex-row md:items-start md:justify-between',
    xl: compact ? 'xl:flex-row xl:items-center xl:justify-between' : 'xl:flex-row xl:items-start xl:justify-between',
    '2xl': compact ? '2xl:flex-row 2xl:items-center 2xl:justify-between' : '2xl:flex-row 2xl:items-start 2xl:justify-between',
  }[horizontalFrom];

  return (
    <div
      data-page-header-density={density}
      className={`${compact ? 'mb-3 gap-2 pb-2' : 'mb-5 gap-3 pb-5'} flex flex-col border-b border-line ${responsiveLayout}`}
    >
      <div className={compact ? 'min-w-0 flex-1 md:flex md:items-baseline md:gap-2' : undefined}>
        <h1
          className={compact ? 'max-w-full truncate text-lg font-semibold leading-6 tracking-normal text-ink md:max-w-[42%] md:shrink-0' : 'text-2xl font-semibold tracking-normal text-ink'}
          title={compact ? title : undefined}
        >
          {title}
        </h1>
        {description ? (
          <>
            {compact ? <span className="hidden text-xs text-slate-300 md:inline" aria-hidden="true">·</span> : null}
            <p
              className={compact ? 'line-clamp-2 min-w-0 text-xs leading-[18px] text-slate-500 md:block md:truncate' : 'mt-1 max-w-3xl text-sm leading-6 text-slate-600'}
              title={compact ? description : undefined}
            >
              {description}
            </p>
          </>
        ) : null}
      </div>
      {actions ? <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div> : null}
    </div>
  );
}
