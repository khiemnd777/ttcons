import { createContext, useContext, type ReactNode } from 'react';
import { useLocation } from 'react-router-dom';
import { wordlessEnabled, wordlessScope, wordlessDetailScope, wordlessBannerSurface } from '../lib/utils/wordless';
import { ContextHelp } from './ContextHelp';
import { LabelWithHelp } from './LabelWithHelp';

const WordlessContext = createContext(false);
const BannerContext = createContext<ReturnType<typeof wordlessBannerSurface>>(undefined);
export const useWordless = () => useContext(WordlessContext);
export const useBannerWordless = (surface: NonNullable<ReturnType<typeof wordlessBannerSurface>>) => useContext(BannerContext) === surface;

export function WordlessBannerScope({ children }: { children: ReactNode }) {
  const location = useLocation();
  return <BannerContext.Provider value={wordlessEnabled(location.search) ? wordlessBannerSurface(location.pathname) : undefined}>{children}</BannerContext.Provider>;
}

export function BannerLabel({ surface, text, help }: { surface: NonNullable<ReturnType<typeof wordlessBannerSurface>>; text: string; help: ReactNode }) {
  const enabled = useBannerWordless(surface);
  return enabled ? <LabelWithHelp text={text} help={<ContextHelp tooltip label={`Giải thích: ${text}`}>{help}</ContextHelp>} /> : <>{text}</>;
}

export function WordlessScope({ children }: { children: ReactNode }) {
  const location = useLocation();
  return <WordlessContext.Provider value={(wordlessScope(location.pathname) || wordlessDetailScope(location.pathname, location.search)) && wordlessEnabled(location.search)}>{children}</WordlessContext.Provider>;
}

export function WordlessExplanation({ label, children, className }: { label: string; children: ReactNode; className?: string }) {
  const enabled = useWordless();
  return enabled ? null : <p className={className}>{children}</p>;
}

export function WordlessLabel({ text, label, help }: { text: string; label: string; help?: ReactNode }) {
  const enabled = useWordless();
  return enabled && help ? <LabelWithHelp text={text} help={<ContextHelp tooltip label={label}>{help}</ContextHelp>} /> : <>{text}</>;
}
