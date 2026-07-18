type BrandLogoProps = {
  size: number;
  className?: string;
};

export function BrandLogo({ size, className }: BrandLogoProps) {
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
