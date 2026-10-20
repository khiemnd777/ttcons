import { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { AuthCard } from './AuthCard';
import { clearRecoveryTokenFragment, recoveryTokenFromFragment } from './recoveryToken';

type State = 'loading' | 'success' | 'error';

export function VerifyRecoveryEmailPage() {
  const token = useMemo(recoveryTokenFromFragment, []);
  const { user, refreshUser } = useAuth();
  const started = useRef(false);
  const [state, setState] = useState<State>(token ? 'loading' : 'error');
  const [message, setMessage] = useState(token ? '' : 'Liên kết xác minh bị thiếu.');

  useEffect(() => {
    if (!token || started.current) return;
    started.current = true;
    clearRecoveryTokenFragment();
    void post<{ recovery_email_verified: boolean }>('/auth/recovery-email/verify', { token })
      .then(async () => {
        if (user) await refreshUser().catch(() => undefined);
        setState('success');
      })
      .catch((error) => {
        setState('error');
        setMessage(error instanceof Error ? error.message : 'Không xác minh được email khôi phục.');
      });
  }, [refreshUser, token, user]);

  return (
    <AuthCard title="Xác minh email khôi phục" description="Xác nhận địa chỉ liên hệ bổ sung cho quá trình khôi phục tài khoản.">
      {state === 'loading' ? <div className="rounded-md border border-blue-200 bg-blue-50 p-4 text-sm text-blue-800" role="status">Đang xác minh email...</div> : null}
      {state === 'success' ? <div className="rounded-md border border-emerald-200 bg-emerald-50 p-4 text-sm leading-6 text-emerald-800" role="status">Email liên hệ đã được xác minh. Developer vẫn phải xác minh danh tính trước khi cấp liên kết.</div> : null}
      {state === 'error' ? <div className="rounded-md border border-rose-200 bg-rose-50 p-4 text-sm leading-6 text-rose-800" role="alert">{message}</div> : null}
      <div className="mt-4 grid gap-2 sm:grid-cols-2">
        <Link className="rounded-md bg-blue-600 px-4 py-2.5 text-center text-sm font-semibold text-white hover:bg-blue-700" to="/login">Đăng nhập</Link>
        <Link className="rounded-md border border-line px-4 py-2.5 text-center text-sm font-semibold text-slate-700 hover:bg-slate-50" to="/profile">Mở hồ sơ</Link>
      </div>
    </AuthCard>
  );
}
