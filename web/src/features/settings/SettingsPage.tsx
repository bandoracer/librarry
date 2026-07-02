import React from "react";
import { Navigate, NavLink, Route, Routes } from "react-router-dom";
import { PageHeader, TabNav } from "../../components/ui";
import { navItems } from "../../app/nav";
import { GeneralTab } from "./GeneralTab";
import { MediaTab } from "./MediaTab";
import { ProfilesTab } from "./ProfilesTab";
import { QualityTab } from "./QualityTab";
import { IndexersTab } from "./IndexersTab";
import { DownloadClientsTab } from "./DownloadClientsTab";
import { ConnectTab } from "./ConnectTab";
import { ImportListsTab } from "./ImportListsTab";
import { TagsTab } from "./TagsTab";
import { ImportTab } from "./ImportTab";
import "./settings.css";

const subtitle = navItems.find((item) => item.id === "settings")?.subtitle;

/** Tab paths are stable API — bookmarks and proxies rely on them. */
const tabs = [
  { label: "General", to: "/settings", end: true },
  { label: "Media Management", to: "/settings/media" },
  { label: "Profiles", to: "/settings/profiles" },
  { label: "Quality", to: "/settings/quality" },
  { label: "Indexers", to: "/settings/indexers" },
  { label: "Download Clients", to: "/settings/download-clients" },
  { label: "Import Lists", to: "/settings/import-lists" },
  { label: "Connect", to: "/settings/connect" },
  { label: "Tags", to: "/settings/tags" },
  { label: "Readarr Import", to: "/settings/import" }
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
          <Route path="quality" element={<QualityTab />} />
          <Route path="indexers" element={<IndexersTab />} />
          <Route path="download-clients" element={<DownloadClientsTab />} />
          {/* Legacy path: the Connections tab split into Indexers + Download Clients. */}
          <Route path="connections" element={<Navigate to="/settings/download-clients" replace />} />
          <Route path="connect" element={<ConnectTab />} />
          <Route path="import-lists" element={<ImportListsTab />} />
          <Route path="tags" element={<TagsTab />} />
          <Route path="import" element={<ImportTab />} />
          <Route path="*" element={<Navigate to="/settings" replace />} />
        </Routes>
      </div>
    </>
  );
}
