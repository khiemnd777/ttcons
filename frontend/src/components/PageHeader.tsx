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
    md: 'md:flex-row md:items-center md:justify-between',
    xl: 'xl:flex-row xl:items-center xl:justify-between',
    '2xl': '2xl:flex-row 2xl:items-center 2xl:justify-between',
  }[horizontalFrom];

  return (
    <div className={`mb-3 flex flex-col gap-2 border-b border-line pb-2 ${responsiveLayout}`}>
      <div className="min-w-0 flex-1 md:flex md:items-baseline md:gap-2">
        <h1
          className="max-w-full truncate text-lg font-semibold leading-6 tracking-normal text-ink md:max-w-[42%] md:shrink-0"
          title={title}
        >
          {title}
        </h1>
        {description ? (
          <>
            <span className="hidden text-xs text-slate-300 md:inline" aria-hidden="true">·</span>
            <p
              className="line-clamp-2 min-w-0 text-xs leading-[18px] text-slate-500 md:block md:truncate"
              title={description}
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
