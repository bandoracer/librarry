import React, { Suspense, lazy } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { QueryClientProvider } from "@tanstack/react-query";
import { AppLayout } from "./AppLayout";
import { defaultPath } from "./nav";
import { queryClient } from "../lib/queries";
import { ToastProvider } from "../components/toast";
import { LoadingRow } from "../components/ui";

const DashboardPage = lazy(() => import("../features/dashboard/DashboardPage"));
const LibraryPage = lazy(() => import("../features/library/LibraryPage"));
const SearchPage = lazy(() => import("../features/search/SearchPage"));
const WantedPage = lazy(() => import("../features/wanted/WantedPage"));
const ActivityPage = lazy(() => import("../features/activity/ActivityPage"));
const ImportsPage = lazy(() => import("../features/imports/ImportsPage"));
const SystemPage = lazy(() => import("../features/system/SystemPage"));
const SettingsPage = lazy(() => import("../features/settings/SettingsPage"));

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <Suspense fallback={<div className="page-loading"><LoadingRow /></div>}>
          <Routes>
            <Route element={<AppLayout />}>
              <Route index element={<Navigate to={defaultPath} replace />} />
              <Route path="/dashboard" element={<DashboardPage />} />
              <Route path="/library" element={<LibraryPage />} />
              <Route path="/search" element={<SearchPage />} />
              <Route path="/wanted" element={<WantedPage />} />
              <Route path="/downloads" element={<ActivityPage />} />
              <Route path="/downloads/history" element={<ActivityPage />} />
              <Route path="/downloads/blocklist" element={<ActivityPage />} />
              <Route path="/imports" element={<ImportsPage />} />
              <Route path="/providers" element={<SystemPage />} />
              <Route path="/settings/*" element={<SettingsPage />} />
              <Route path="*" element={<Navigate to={defaultPath} replace />} />
            </Route>
          </Routes>
        </Suspense>
      </ToastProvider>
    </QueryClientProvider>
  );
}
