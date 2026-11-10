import { useEffect, useState, type FormEvent } from 'react';
import { LockKeyhole, Mail, ShieldCheck } from 'lucide-react';
import { AuthCard } from './AuthCard';
import { PasswordInput } from '../../components/PasswordInput';
import { useToast } from '../../components/ToastProvider';
import { post } from '../../lib/api/client';

const developerEmail = 'khiemnd777@gmail.com';

export function DeveloperBootstrapPage() {
  const [sent, setSent] = useState(false);
  const [until, setUntil] = useState(0);
  const [now, setNow] = useState(Date.now());
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [otp, setOtp] = useState('');
  const [name, setName] = useState('');
  const [password, setPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [invalid, setInvalid] = useState<string[]>([]);
  const toast = useToast();
  const seconds = Math.max(0, Math.ceil((until - now) / 1000));
  useEffect(() => { const timer = window.setInterval(() => setNow(Date.now()), 1000); return () => window.clearInterval(timer); }, []);

  async function send() {
    setBusy(true); setError('');
    try {
      await post('/auth/bootstrap/otp');
      setSent(true); setOtp(''); setUntil(Date.now() + 60_000);
      toast.success({ title: 'Đã gửi mã xác minh', description: 'Kiểm tra hộp thư Developer.' });
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Không gửi được mã xác minh.';
      setError(message); toast.error({ title: 'Không gửi được mã', description: message });
    }
    finally { setBusy(false); }
  }

  async function complete(event: FormEvent) {
    event.preventDefault();
    const failures: string[] = [];
    if (!/^\d{8}$/.test(otp)) failures.push('otp');
    if (Array.from(name.trim()).length < 2 || Array.from(name.trim()).length > 160) failures.push('name');
    if (!password.trim() || Array.from(password).length < 8 || new TextEncoder().encode(password).length > 72) failures.push('password');
    if (confirmation !== password || !confirmation) failures.push('confirmation');
    setInvalid(failures);
    if (failures.length) { toast.warn({ title: 'Vui lòng kiểm tra thông tin', description: 'Kiểm tra mã xác minh, họ tên và mật khẩu.' }); return; }
    setBusy(true); setError('');
    try {
      await post('/auth/bootstrap/complete', { otp, full_name: name, password, password_confirmation: confirmation });
      window.location.assign('/login');
    } catch (e) {
      const message = e instanceof Error ? e.message : 'Không hoàn tất được thiết lập.';
      setError(message); toast.error({ title: 'Không hoàn tất được thiết lập', description: message });
    }
    finally { setBusy(false); }
  }
  const inputClass = (field: string) => `mt-1 w-full rounded-md border px-3 py-2 text-sm focus-ring ${invalid.includes(field) ? 'border-rose-400 bg-rose-50' : 'border-line'}`;
  return (
    <AuthCard title="Thiết lập tài khoản Developer" description="Hệ thống cần tài khoản Developer để bắt đầu.">
      <div className="space-y-4">
        <p className="flex items-center gap-2 text-sm font-semibold text-slate-700"><ShieldCheck className="h-4 w-4" aria-hidden="true" /> {sent ? 'Bước 2 / 2 · Xác minh và tạo tài khoản' : 'Bước 1 / 2 · Xác minh email'}</p>
        <label className="block text-sm font-semibold text-slate-700">Email Developer<input value={developerEmail} readOnly className="mt-1 w-full rounded-md border border-line bg-slate-50 px-3 py-2 text-sm" /></label>
        {!sent ? <p className="text-sm text-slate-600">Mã xác minh sẽ được gửi đến địa chỉ này.</p> : null}
        {sent ? <form onSubmit={complete} noValidate className="space-y-4">
          <label className="block text-sm font-semibold text-slate-700">Mã xác minh<input value={otp} onChange={e => setOtp(e.target.value)} inputMode="numeric" autoComplete="one-time-code" maxLength={8} aria-invalid={invalid.includes('otp')} className={inputClass('otp')} />{invalid.includes('otp') ? <span className="text-xs text-rose-600">Nhập mã gồm 8 chữ số.</span> : null}</label>
          <button type="button" onClick={send} disabled={busy || seconds > 0} className="text-sm font-semibold text-blue-700 disabled:text-slate-400">{seconds > 0 ? `Gửi lại mã sau ${seconds} giây` : 'Gửi lại mã'}</button>
          <label className="block text-sm font-semibold text-slate-700">Họ và tên<input value={name} onChange={e => setName(e.target.value)} autoComplete="name" aria-invalid={invalid.includes('name')} className={inputClass('name')} />{invalid.includes('name') ? <span className="text-xs text-rose-600">Nhập họ tên từ 2 đến 160 ký tự.</span> : null}</label>
          <label className="block text-sm font-semibold text-slate-700">Mật khẩu<span className="mt-1 block"><PasswordInput value={password} onChange={e => setPassword(e.target.value)} autoComplete="new-password" invalid={invalid.includes('password')} className={inputClass('password')} /></span>{invalid.includes('password') ? <span className="text-xs text-rose-600">Mật khẩu cần ít nhất 8 ký tự, tối đa 72 byte.</span> : null}</label>
          <label className="block text-sm font-semibold text-slate-700">Xác nhận mật khẩu<span className="mt-1 block"><PasswordInput value={confirmation} onChange={e => setConfirmation(e.target.value)} autoComplete="new-password" invalid={invalid.includes('confirmation')} className={inputClass('confirmation')} /></span>{invalid.includes('confirmation') ? <span className="text-xs text-rose-600">Mật khẩu xác nhận chưa khớp.</span> : null}</label>
          <button disabled={busy} className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"><LockKeyhole className="h-4 w-4" aria-hidden="true" />{busy ? 'Đang xử lý...' : 'Hoàn tất thiết lập'}</button>
        </form> : <button type="button" onClick={send} disabled={busy} className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white disabled:opacity-60"><Mail className="h-4 w-4" aria-hidden="true" />{busy ? 'Đang gửi...' : 'Gửi mã xác minh'}</button>}
        {error ? <p role="alert" className="rounded-md bg-rose-50 px-3 py-2 text-sm text-rose-700">{error}</p> : null}
        <p className="text-sm text-slate-600">Hệ thống đang chờ hoàn tất thiết lập.</p>
      </div>
    </AuthCard>
  );
}
