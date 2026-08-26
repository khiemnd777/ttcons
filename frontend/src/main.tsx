import React from 'react';
import ReactDOM from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider } from 'react-router-dom';
import { AuthProvider } from './lib/auth/AuthContext';
import { router } from './app/router';
import { ToastProvider } from './components/ToastProvider';
import { ActorRealtimeProvider } from './modules/notifications/ActorRealtimeProvider';
import { shouldRetryQuery } from './lib/api/backpressure';
import './index.css';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: shouldRetryQuery,
    },
  },
});

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <ActorRealtimeProvider>
          <ToastProvider>
            <RouterProvider router={router} />
          </ToastProvider>
        </ActorRealtimeProvider>
      </AuthProvider>
    </QueryClientProvider>
  </React.StrictMode>,
);
