import { useEffect, useState } from 'react';
import { api } from '../../lib/api/client';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { AuthCard } from './AuthCard';
import { DeveloperBootstrapPage } from './DeveloperBootstrapPage';
import type { ReactNode } from 'react';

export function DeveloperBootstrapGate({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<'loading' | 'error' | 'required' | 'ready'>('loading');
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    let active = true;
    let pending = false;
    async function check() {
      if (pending) return;
      pending = true;
      try {
        const result = await api<{ required: boolean }>('/bootstrap/status');
        if (active) setStatus(result.required ? 'required' : 'ready');
      } catch { if (active) setStatus('error'); }
      finally { pending = false; }
    }
    void check();
    const timer = window.setInterval(check, 15000);
    return () => { active = false; window.clearInterval(timer); };
  }, [retry]);
  if (status === 'loading') return <div className="p-6"><LoadingSpinner /></div>;
  if (status === 'error') return <AuthCard title="Không kiểm tra được hệ thống"><p role="alert" className="text-sm text-rose-700">Không thể xác định trạng thái thiết lập. Vui lòng thử lại.</p><button className="mt-4 rounded-md bg-blue-600 px-4 py-2 text-white" onClick={() => { setStatus('loading'); setRetry(value => value + 1); }}>Thử lại</button></AuthCard>;
  if (status === 'required') return <DeveloperBootstrapPage />;
  return <>{children}</>;
}
