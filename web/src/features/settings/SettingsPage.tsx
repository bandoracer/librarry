import React from "react";
import { Navigate, NavLink, Route, Routes } from "react-router-dom";
import { PageHeader, TabNav } from "../../components/ui";
import { navItems } from "../../app/nav";
import { GeneralTab } from "./GeneralTab";
import { MediaTab } from "./MediaTab";
import { ProfilesTab } from "./ProfilesTab";
import { ConnectionsTab } from "./ConnectionsTab";
import { ImportTab } from "./ImportTab";
import "./settings.css";

const subtitle = navItems.find((item) => item.id === "settings")?.subtitle;

/** Tab paths are stable API — bookmarks and proxies rely on them. */
const tabs = [
  { label: "General", to: "/settings", end: true },
  { label: "Media Management", to: "/settings/media" },
  { label: "Profiles", to: "/settings/profiles" },
  { label: "Connections", to: "/settings/connections" },
  { label: "Import", to: "/settings/import" }
];

export default function SettingsPage() {
  return (
    <>
      <PageHeader title="Settings" subtitle={subtitle}>
        <TabNav
          tabs={tabs}
          render={(tab) => (
            <NavLink key={tab.to} to={tab.to} end={tab.end} className={({ isActive }) => (isActive ? "active" : undefined)}>
              {tab.label}
            </NavLink>
          )}
        />
      </PageHeader>
      <div className="settings-tab-body">
        <Routes>
          <Route index element={<GeneralTab />} />
          <Route path="media" element={<MediaTab />} />
          <Route path="profiles" element={<ProfilesTab />} />
          <Route path="connections" element={<ConnectionsTab />} />
          <Route path="import" element={<ImportTab />} />
          <Route path="*" element={<Navigate to="/settings" replace />} />
        </Routes>
      </div>
    </>
  );
}
