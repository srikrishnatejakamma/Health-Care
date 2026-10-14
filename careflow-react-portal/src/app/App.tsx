import { CssBaseline, ThemeProvider } from '@mui/material';
import { QueryClientProvider } from '@tanstack/react-query';
import { lazy, Suspense, useEffect, useMemo } from 'react';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import { useSession } from '../stores/session';
import { queryClient } from '../services/apiClient';
import { makeTheme } from './theme';
import { MainLayout } from '../layouts/MainLayout';
import { ErrorBoundary, LoadingState } from '../shared/components';
const Login = lazy(() => import('../features/auth/Login'));
const AuthCallback = lazy(() => import('../features/auth/Login').then((module) => ({ default: module.AuthCallback })));

const Dashboard = lazy(() => import('../features/dashboard/Dashboard'));
const Records = lazy(() => import('../features/patients/Records'));
const Scheduler = lazy(() => import('../features/scheduler/Scheduler'));
const Appointments = lazy(() => import('../features/appointments/Appointments'));
const Designer = lazy(() => import('../features/workflows/Designer'));
const Monitoring = lazy(() => import('../features/monitoring/Monitoring'));
const Administration = lazy(() => import('../features/administration/Administration'));
const Guide = lazy(() => import('../features/administration/Guide'));
export default function App() {
  const mode = useSession((s) => s.theme);
  const me = useSession((s) => s.me);
  const theme = useMemo(() => makeTheme(mode), [mode]);
  useEffect(() => {
    document.documentElement.dataset.theme = mode;
  }, [mode]);
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <QueryClientProvider client={queryClient}>
        <ErrorBoundary>
          <BrowserRouter>
            <Suspense
              fallback={
                <div className="route-loading">
                  <LoadingState />
                </div>
              }
            >
              <Routes>
                <Route path="/login" element={<Login />} />
                <Route path="/auth/callback" element={<AuthCallback />} />
                <Route element={me ? <MainLayout /> : <Navigate to="/login" replace />}>
                  <Route index element={<Navigate to="/dashboard" replace />} />
                  <Route path="dashboard" element={<Dashboard />} />
                  {[
                    'patients',
                    'therapy',
                    'prescriptions',
                    'insurance',
                    'pharmacy',
                    'billing',
                    'workflows',
                  ].flatMap((path) => [
                    <Route key={path} path={path} element={<Records />} />,
                    <Route key={path + 'detail'} path={`${path}/:id`} element={<Records />} />,
                  ])}
                  <Route path="appointments" element={<Appointments />} />
                  <Route path="scheduler" element={<Scheduler />} />
                  <Route path="designer" element={<Designer />} />
                  {['databases', 'concurrency', 'memory', 'events', 'callbacks', 'health', 'audit'].map(
                    (path) => (
                      <Route key={path} path={path} element={<Monitoring />} />
                    ),
                  )}
                  {['configuration', 'users', 'alerts'].map((path) => (
                    <Route key={path} path={path} element={<Administration />} />
                  ))}
                  <Route path="guide" element={<Guide />} />
                  <Route path="*" element={<Navigate to="/dashboard" replace />} />
                </Route>
              </Routes>
            </Suspense>
          </BrowserRouter>
        </ErrorBoundary>
      </QueryClientProvider>
    </ThemeProvider>
  );
}
