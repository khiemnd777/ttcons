import type { MouseEventHandler } from 'react';
import { Link } from 'react-router-dom';

type BrandLogoProps = {
  size: number;
  className?: string;
  variant?: 'mark' | 'lockup';
};

type BrandHomeLinkProps = {
  className?: string;
  compact?: boolean;
  onClick?: MouseEventHandler<HTMLAnchorElement>;
};

export function BrandLogo({ size, className, variant = 'mark' }: BrandLogoProps) {
  if (variant === 'lockup') {
    return (
      <span className={['inline-flex shrink-0 items-center gap-3', className].filter(Boolean).join(' ')}>
        <img
          src="/brand-logo.svg"
          alt=""
          width={size}
          height={size}
          className="block shrink-0"
          aria-hidden="true"
          draggable={false}
        />
        <span className="flex min-w-0 flex-col">
          <span className="text-[15px] font-semibold leading-[18px] text-[#172033]">Thủ Thiêm</span>
          <span className="text-[10px] font-semibold uppercase leading-[14px] tracking-[0.22em] text-blue-600">Workflow</span>
        </span>
      </span>
    );
  }

  return (
    <img
      src="/brand-logo.svg"
      alt="Thủ Thiêm Workflow"
      width={size}
      height={size}
      className={['block shrink-0', className].filter(Boolean).join(' ')}
      draggable={false}
    />
  );
}

export function BrandHomeLink({ className, compact = false, onClick }: BrandHomeLinkProps) {
  return (
    <Link
      to="/"
      aria-label="Về trang mặc định"
      className={[
        compact
          ? 'inline-flex shrink-0 rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2'
          : 'flex h-full w-full items-center px-4 transition-colors hover:bg-slate-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-blue-500',
        className,
      ].filter(Boolean).join(' ')}
      onClick={onClick}
    >
      <BrandLogo size={40} variant={compact ? 'mark' : 'lockup'} />
    </Link>
  );
}
