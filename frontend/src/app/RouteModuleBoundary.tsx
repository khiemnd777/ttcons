import { Component, Suspense, type ErrorInfo, type ReactNode } from 'react';
import { AlertTriangle } from 'lucide-react';
import { useLocation, useNavigate } from 'react-router-dom';

type RouteModuleBoundaryProps = {
  children: ReactNode;
  focus?: boolean;
};

type RouteErrorBoundaryProps = RouteModuleBoundaryProps & {
  boundaryKey: string;
  onHome: () => void;
  onReload: () => void;
};

type RouteErrorBoundaryState = {
  error: Error | null;
};

function RouteLoading({ focus = false }: { focus?: boolean }) {
  return (
    <div
      className={`flex items-center justify-center ${focus ? 'min-h-screen px-4' : 'min-h-[240px]'}`}
      role="status"
      aria-live="polite"
    >
      <div className="flex items-center gap-3 rounded-lg border border-line bg-white px-5 py-4 text-sm font-medium text-slate-600 shadow-sm">
        <span className="h-4 w-4 animate-spin rounded-full border-2 border-blue-600 border-t-transparent" aria-hidden="true" />
        Đang mở màn hình...
      </div>
    </div>
  );
}

function RouteLoadError({ focus = false, onHome, onReload }: Pick<RouteErrorBoundaryProps, 'focus' | 'onHome' | 'onReload'>) {
  return (
    <div className={`flex items-center justify-center ${focus ? 'min-h-screen px-4' : 'min-h-[240px]'}`}>
      <section className="w-full max-w-2xl rounded-lg border border-amber-200 bg-amber-50 p-5 text-amber-950 shadow-sm" role="alert">
        <div className="flex items-start gap-3">
          <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0" aria-hidden="true" />
          <div className="min-w-0">
            <h1 className="text-base font-semibold">Không thể mở màn hình</h1>
            <p className="mt-1 text-sm text-amber-800">
              Đã xảy ra lỗi khi tải nội dung màn hình. Hãy tải lại trang; nếu lỗi tiếp diễn, quay về trang mặc định.
            </p>
            <div className="mt-4 flex flex-col gap-2 sm:flex-row">
              <button
                type="button"
                className="inline-flex justify-center rounded-md bg-amber-900 px-3 py-2 text-sm font-semibold text-white hover:bg-amber-800 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-700 focus-visible:ring-offset-2"
                onClick={onReload}
              >
                Tải lại trang
              </button>
              <button
                type="button"
                className="inline-flex justify-center rounded-md border border-amber-300 bg-white px-3 py-2 text-sm font-semibold text-amber-950 hover:bg-amber-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-700 focus-visible:ring-offset-2"
                onClick={onHome}
              >
                Về trang mặc định
              </button>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}

class RouteErrorBoundary extends Component<RouteErrorBoundaryProps, RouteErrorBoundaryState> {
  state: RouteErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): RouteErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('route_render_failed', { error, componentStack: errorInfo.componentStack });
  }

  componentDidUpdate(previousProps: RouteErrorBoundaryProps) {
    if (this.state.error && previousProps.boundaryKey !== this.props.boundaryKey) {
      this.setState({ error: null });
    }
  }

  render() {
    if (this.state.error) {
      return <RouteLoadError focus={this.props.focus} onHome={this.props.onHome} onReload={this.props.onReload} />;
    }
    return this.props.children;
  }
}

export function RouteModuleBoundary({ children, focus = false }: RouteModuleBoundaryProps) {
  const location = useLocation();
  const navigate = useNavigate();
  const boundaryKey = `${location.pathname}${location.search}`;

  return (
    <RouteErrorBoundary
      boundaryKey={boundaryKey}
      focus={focus}
      onHome={() => navigate('/')}
      onReload={() => window.location.reload()}
    >
      <Suspense fallback={<RouteLoading focus={focus} />}>{children}</Suspense>
    </RouteErrorBoundary>
  );
}
