type BrandLogoProps = {
  size: number;
  className?: string;
  variant?: 'mark' | 'lockup';
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
