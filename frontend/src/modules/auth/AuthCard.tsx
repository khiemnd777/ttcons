import type { ReactNode } from 'react';
import { BrandLogo } from '../../components/BrandLogo';
import { PoweredBySignature } from '../../components/PoweredBySignature';

export function AuthCard({ title, description, children }: { title: string; description?: string; children: ReactNode }) {
  return (
    <div className="grid min-h-screen place-items-center bg-surface px-4 py-8">
      <main className="w-full max-w-md rounded-lg border border-line bg-white p-6 shadow-panel">
        <div className="mb-6">
          <BrandLogo size={112} className="mb-5" />
          <h1 className="text-2xl font-semibold text-ink">{title}</h1>
          {description ? <p className="mt-1 text-sm leading-6 text-slate-600">{description}</p> : null}
        </div>
        {children}
        <div className="mt-6 border-t border-line pt-4">
          <PoweredBySignature />
        </div>
      </main>
    </div>
  );
}
