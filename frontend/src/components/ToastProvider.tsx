import { createContext, ReactNode, useCallback, useContext, useMemo, useState } from 'react';
import { AlertCircle, AlertTriangle, CheckCircle2, Info, X } from 'lucide-react';
import clsx from 'clsx';
import { ApiError } from '../lib/api/client';

type ToastVariant = 'success' | 'warn' | 'error' | 'info';

type ToastItem = {
  id: string;
  variant: ToastVariant;
  title: string;
  description?: string;
};

type ToastInput = string | {
  title: string;
  description?: string;
};

type ToastContextValue = {
  success: (input: ToastInput) => void;
  warn: (input: ToastInput) => void;
  error: (input: ToastInput, error?: unknown) => void;
  info: (input: ToastInput) => void;
};

const ToastContext = createContext<ToastContextValue | null>(null);

const toastLifetime: Record<ToastVariant, number> = {
  success: 3200,
  warn: 6200,
  error: 6500,
  info: 4500,
};

function normalizeToast(input: ToastInput) {
  if (typeof input === 'string') return { title: input };
  return input;
}

function errorDescription(error: unknown) {
  if (!error) return undefined;
  if (error instanceof ApiError && error.message) {
    return error.status >= 500 && error.requestId
      ? `${error.message} Mã tra cứu: ${error.requestId}`
      : error.message;
  }
  if (error instanceof Error && error.message) return error.message;
  if (typeof error === 'string') return error;
  return 'Vui lòng thử lại hoặc kiểm tra dữ liệu đầu vào.';
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const dismiss = useCallback((id: string) => {
    setToasts((current) => current.filter((toast) => toast.id !== id));
  }, []);

  const addToast = useCallback((variant: ToastVariant, input: ToastInput, error?: unknown) => {
    const content = normalizeToast(input);
    const id = `${Date.now()}-${Math.random().toString(36).slice(2)}`;
    const toast: ToastItem = {
      id,
      variant,
      title: content.title,
      description: variant === 'error' ? content.description ?? errorDescription(error) : content.description,
    };

    setToasts((current) => {
      const duplicate = current.some((item) => item.variant === toast.variant && item.title === toast.title && item.description === toast.description);
      if (duplicate) return current;
      return [toast, ...current].slice(0, 3);
    });
    window.setTimeout(() => dismiss(id), toastLifetime[variant]);
  }, [dismiss]);

  const value = useMemo<ToastContextValue>(() => ({
    success: (input) => addToast('success', input),
    warn: (input) => addToast('warn', input),
    error: (input, error) => addToast('error', input, error),
    info: (input) => addToast('info', input),
  }), [addToast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div className="fixed right-4 top-4 z-50 flex w-[calc(100vw-2rem)] max-w-sm flex-col gap-2" role="status" aria-live="polite">
        {toasts.map((toast) => {
          const Icon = toast.variant === 'success' ? CheckCircle2 : toast.variant === 'warn' ? AlertTriangle : toast.variant === 'error' ? AlertCircle : Info;
          return (
            <div
              key={toast.id}
              className={clsx(
                'flex items-start gap-3 rounded-lg border bg-white p-3 shadow-lg',
                toast.variant === 'success' && 'border-emerald-200',
                toast.variant === 'warn' && 'border-amber-200',
                toast.variant === 'error' && 'border-rose-200',
                toast.variant === 'info' && 'border-blue-200',
              )}
            >
              <Icon
                className={clsx(
                  'mt-0.5 h-5 w-5 shrink-0',
                  toast.variant === 'success' && 'text-emerald-600',
                  toast.variant === 'warn' && 'text-amber-600',
                  toast.variant === 'error' && 'text-rose-600',
                  toast.variant === 'info' && 'text-blue-600',
                )}
              />
              <div className="min-w-0 flex-1">
                <p className="text-sm font-semibold text-ink">{toast.title}</p>
                {toast.description ? <p className="mt-1 text-sm leading-5 text-slate-600">{toast.description}</p> : null}
              </div>
              <button className="rounded-md p-1 text-slate-400 hover:bg-slate-100 hover:text-slate-600" onClick={() => dismiss(toast.id)} aria-label="Đóng thông báo">
                <X className="h-4 w-4" />
              </button>
            </div>
          );
        })}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast() {
  const context = useContext(ToastContext);
  if (!context) throw new Error('useToast must be used inside ToastProvider');
  return context;
}
