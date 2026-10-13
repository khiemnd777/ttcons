import { createContext, useCallback, useContext, useMemo, useRef, useState, type ReactNode } from 'react';

type LauncherPresentation = 'icon' | 'label';
type LauncherHost = { element: HTMLElement; presentation: LauncherPresentation };
type LauncherHosts = { host: LauncherHost | null; register: (element: HTMLElement, presentation: LauncherPresentation) => () => void };
const LauncherHostContext = createContext<LauncherHosts | null>(null);

export function UtilityDockLauncherProvider({ children }: { children: ReactNode }) {
  const [host, setHost] = useState<LauncherHost | null>(null);
  const register = useCallback((element: HTMLElement, presentation: LauncherPresentation) => {
    const registration = { element, presentation };
    setHost(registration);
    // A retiring route or StrictMode ref must not clear a newer header host.
    return () => setHost((current) => current === registration ? null : current);
  }, []);
  const value = useMemo(() => ({ host, register }), [host, register]);
  return <LauncherHostContext.Provider value={value}>{children}</LauncherHostContext.Provider>;
}

function useLauncherHosts() {
  const hosts = useContext(LauncherHostContext);
  if (!hosts) throw new Error('Utility Dock launcher requires its AppShell provider.');
  return hosts;
}

export function useUtilityDockLauncherHost() {
  return useLauncherHosts().host;
}

export function UtilityDockLauncherHost({ presentation }: { presentation: LauncherPresentation }) {
  const { register } = useLauncherHosts();
  const release = useRef<(() => void) | undefined>();
  const attach = useCallback((element: HTMLDivElement | null) => {
    release.current?.();
    release.current = element ? register(element, presentation) : undefined;
  }, [register, presentation]);
  return <div ref={attach} id="utility-dock-mobile-launcher" className="sm:hidden" />;
}
