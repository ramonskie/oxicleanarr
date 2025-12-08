import { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Toaster } from 'sonner';
import { useAuthStore } from '@/store/auth';
import ErrorBoundary from '@/components/ErrorBoundary';
import { errorLogger } from '@/lib/error-logger';
import LoginPage from '@/pages/LoginPage';
import DashboardPage from '@/pages/DashboardPage';
import TimelinePage from '@/pages/TimelinePage';
import LibraryPage from '@/pages/LibraryPage';
import ScheduledDeletionsPage from '@/pages/ScheduledDeletionsPage';
import JobHistoryPage from '@/pages/JobHistoryPage';
import ConfigurationPage from '@/pages/ConfigurationPage';
import RulesPage from '@/pages/RulesPage';
import ProtectedRoute from '@/components/ProtectedRoute';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (failureCount, error: any) => {
        // Don't retry on 401/403 (auth errors)
        if (error?.response?.status === 401 || error?.response?.status === 403) {
          return false;
        }
        // Don't retry on 404 (not found)
        if (error?.response?.status === 404) {
          return false;
        }
        // Retry up to 2 times for other errors (network issues, 500s, etc.)
        return failureCount < 2;
      },
      retryDelay: (attemptIndex) => {
        // Exponential backoff: 1s, 2s, 4s
        return Math.min(1000 * 2 ** attemptIndex, 30000);
      },
      refetchOnWindowFocus: true,
      staleTime: 30000, // Consider data stale after 30 seconds
      gcTime: 300000, // Keep unused data in cache for 5 minutes
    },
    mutations: {
      retry: false, // Don't retry mutations by default
      onError: (error: any) => {
        // Log mutation errors
        errorLogger.error('React Query mutation failed', error, {
          type: 'mutation',
        });
      },
    },
  },
});

function App() {
  const initializeAuth = useAuthStore((state) => state.initialize);

  useEffect(() => {
    initializeAuth();
  }, [initializeAuth]);

  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route
            path="/"
            element={
              <ProtectedRoute>
                <DashboardPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/timeline"
            element={
              <ProtectedRoute>
                <TimelinePage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/library"
            element={
              <ProtectedRoute>
                <LibraryPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/scheduled-deletions"
            element={
              <ProtectedRoute>
                <ScheduledDeletionsPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/job-history"
            element={
              <ProtectedRoute>
                <JobHistoryPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/configuration"
            element={<Navigate to="/settings/general" replace />}
          />
          <Route
            path="/settings/:section"
            element={
              <ProtectedRoute>
                <ConfigurationPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/rules"
            element={
              <ProtectedRoute>
                <RulesPage />
              </ProtectedRoute>
            }
          />
          <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </BrowserRouter>
        <Toaster position="top-right" richColors />
      </QueryClientProvider>
    </ErrorBoundary>
  );
}

export default App;
