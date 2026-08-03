import clsx from 'clsx';

type PoweredBySignatureProps = {
  className?: string;
};

export function PoweredBySignature({ className }: PoweredBySignatureProps) {
  return (
    <div
      className={clsx(
        'flex items-center justify-center gap-1 whitespace-nowrap text-[9px] font-semibold tracking-[0.08em]',
        className,
      )}
    >
      <span className="text-slate-400">POWERED BY</span>
      <a
        href="https://www.knasoftware.com"
        target="_blank"
        rel="noopener noreferrer"
        aria-label="Mở KNASOFTWARE trong tab mới"
        className="inline-flex items-center rounded-sm text-blue-700 transition-colors hover:text-blue-800 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2"
      >
        KNASOFTWARE
      </a>
    </div>
  );
}
