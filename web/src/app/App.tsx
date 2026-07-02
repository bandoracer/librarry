import React, { Suspense, lazy } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { QueryClientProvider } from "@tanstack/react-query";
import { AppLayout } from "./AppLayout";
import { defaultPath } from "./nav";
import { queryClient, useAuthStatus } from "../lib/queries";
import { ToastProvider } from "../components/toast";
import { LoadingRow } from "../components/ui";

const DashboardPage = lazy(() => import("../features/dashboard/DashboardPage"));
const LibraryPage = lazy(() => import("../features/library/LibraryPage"));
const AuthorPage = lazy(() => import("../features/library/AuthorPage"));
const BookPage = lazy(() => import("../features/library/BookPage"));
const SearchPage = lazy(() => import("../features/search/SearchPage"));
const WantedPage = lazy(() => import("../features/wanted/WantedPage"));
const ActivityPage = lazy(() => import("../features/activity/ActivityPage"));
const ImportsPage = lazy(() => import("../features/imports/ImportsPage"));
const SystemPage = lazy(() => import("../features/system/SystemPage"));
const SettingsPage = lazy(() => import("../features/settings/SettingsPage"));
const CalendarPage = lazy(() => import("../features/calendar/CalendarPage"));
const LoginPage = lazy(() => import("../features/auth/LoginPage"));

/**
 * Forms-auth gate: unauthenticated sessions see the login page. API-key
 * clients and none/basic installs pass straight through (useAuthStatus
 * resolves open on any failure so a broken probe can never lock the UI).
 */
function AuthGate(props: { children: React.ReactNode }) {
  const auth = useAuthStatus();
  if (auth.data && auth.data.method === "forms" && !auth.data.authenticated) {
    return <LoginPage />;
  }
  return <>{props.children}</>;
}

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <Suspense fallback={<div className="page-loading"><LoadingRow /></div>}>
          <AuthGate>
          <Routes>
            <Route element={<AppLayout />}>
              <Route index element={<Navigate to={defaultPath} replace />} />
              <Route path="/dashboard" element={<DashboardPage />} />
              <Route path="/library" element={<LibraryPage />} />
              <Route path="/library/author/:authorId" element={<AuthorPage />} />
              <Route path="/library/book/:wantedId" element={<BookPage />} />
              <Route path="/search" element={<SearchPage />} />
              <Route path="/wanted" element={<WantedPage />} />
              <Route path="/calendar" element={<CalendarPage />} />
              <Route path="/downloads" element={<ActivityPage />} />
              <Route path="/downloads/history" element={<ActivityPage />} />
              <Route path="/downloads/blocklist" element={<ActivityPage />} />
              <Route path="/imports" element={<ImportsPage />} />
              <Route path="/providers" element={<SystemPage />} />
              <Route path="/settings/*" element={<SettingsPage />} />
              <Route path="*" element={<Navigate to={defaultPath} replace />} />
            </Route>
          </Routes>
          </AuthGate>
        </Suspense>
      </ToastProvider>
    </QueryClientProvider>
  );
}
