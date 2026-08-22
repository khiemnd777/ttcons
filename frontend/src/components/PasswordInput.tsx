import { forwardRef, useState, type InputHTMLAttributes } from 'react';
import clsx from 'clsx';
import { Eye, EyeOff, RefreshCw } from 'lucide-react';

export type PasswordInputProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> & {
  invalid?: boolean;
  visible?: boolean;
  onVisibilityChange?: (visible: boolean) => void;
  onSuggest?: () => void;
  suggestLabel?: string;
};

export const PasswordInput = forwardRef<HTMLInputElement, PasswordInputProps>(function PasswordInput({
  className,
  disabled,
  invalid,
  visible,
  onVisibilityChange,
  onSuggest,
  suggestLabel = 'Đề xuất mật khẩu',
  ...inputProps
}, ref) {
  const [internalVisible, setInternalVisible] = useState(false);
  const passwordVisible = visible ?? internalVisible;

  function toggleVisibility() {
    const nextVisible = !passwordVisible;
    if (visible === undefined) setInternalVisible(nextVisible);
    onVisibilityChange?.(nextVisible);
  }

  return (
    <span className="flex items-stretch gap-2">
      <span className="relative min-w-0 flex-1">
        <input
          {...inputProps}
          ref={ref}
          type={passwordVisible ? 'text' : 'password'}
          disabled={disabled}
          aria-invalid={invalid || undefined}
          className={clsx(className, 'pr-11')}
        />
        <button
          type="button"
          className="absolute inset-y-0 right-0 grid w-10 place-items-center rounded-r-md text-slate-500 hover:bg-slate-50 hover:text-ink disabled:cursor-not-allowed disabled:opacity-50"
          onClick={toggleVisibility}
          disabled={disabled}
          aria-label={passwordVisible ? 'Ẩn mật khẩu' : 'Hiện mật khẩu'}
          title={passwordVisible ? 'Ẩn mật khẩu' : 'Hiện mật khẩu'}
        >
          {passwordVisible ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
        </button>
      </span>
      {onSuggest ? (
        <button
          type="button"
          className="grid w-10 shrink-0 place-items-center rounded-md border border-line bg-white text-slate-600 transition hover:border-blue-300 hover:bg-blue-50 hover:text-blue-700 active:[&>svg]:rotate-180 disabled:cursor-not-allowed disabled:opacity-50"
          onClick={onSuggest}
          disabled={disabled}
          aria-label={suggestLabel}
          title={suggestLabel}
        >
          <RefreshCw className="h-4 w-4 transition-transform duration-300" />
        </button>
      ) : null}
    </span>
  );
});
