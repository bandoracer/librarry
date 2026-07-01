import React, { useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { RadioTower, RefreshCw, Rss, TrendingUp } from "lucide-react";
import { runAuthorMonitor, runUpgradeSearch, runWantedFeedSync, runWantedMonitor } from "../../lib/api";
import type { AuthorMonitorRun } from "../../lib/api";
import { keys } from "../../lib/queries";
import { useToast } from "../../components/toast";
import { PageHeader, TabNav, ToolbarButton } from "../../components/ui";
import { BooksTab } from "./BooksTab";
import { AuthorsTab } from "./AuthorsTab";
import type { AuthorMonitorOptions } from "./AuthorsTab";
import {
  appErrorMessage,
  authorMonitorRunSummary,
  errorMessage,
  feedSyncRunSummary,
  monitorRunSummary,
  upgradeRunSummary
} from "./lib";
import "./wanted.css";

/**
 * Wanted: missing books, release decisions, metadata provenance review, and
 * author subscriptions. Tabs are synced to the URL (`?tab=authors`), and the
 * Books tab honors `?item=`, `?filter=`, and `&search=1` deep links.
 */
export default function WantedPage() {
  const [searchParams] = useSearchParams();
  const toast = useToast();
  const client = useQueryClient();

  const tab: "books" | "authors" = searchParams.get("tab") === "authors" ? "authors" : "books";

  const [isRunningMonitor, setIsRunningMonitor] = useState(false);
  const [isRunningFeedSync, setIsRunningFeedSync] = useState(false);
  const [isRunningUpgrade, setIsRunningUpgrade] = useState(false);
  const [isRunningAuthorMonitor, setIsRunningAuthorMonitor] = useState(false);
  const [authorMonitorTargetKey, setAuthorMonitorTargetKey] = useState("");
  const [authorMonitorRun, setAuthorMonitorRun] = useState<AuthorMonitorRun | null>(null);

  function invalidate(...queryKeys: readonly (readonly unknown[])[]) {
    return Promise.all(queryKeys.map((key) => client.invalidateQueries({ queryKey: key })));
  }

  async function handleSearchMonitor() {
    setIsRunningMonitor(true);
    try {
      const run = await runWantedMonitor({ force: false, autoGrab: false, paused: true });
      toast.success(monitorRunSummary(run));
      await invalidate(keys.wanted, keys.acquisitionQueue, keys.wantedMetadataReview, keys.downloads(), keys.history());
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Wanted monitor failed")));
    } finally {
      setIsRunningMonitor(false);
    }
  }

  async function handleFeedSync() {
    setIsRunningFeedSync(true);
    try {
      const run = await runWantedFeedSync({ format: "any", autoGrab: false, paused: true });
      toast.success(feedSyncRunSummary(run));
      await invalidate(keys.wanted, keys.acquisitionQueue, keys.downloads(), keys.history());
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Feed sync failed")));
    } finally {
      setIsRunningFeedSync(false);
    }
  }

  async function handleUpgradeSearch() {
    setIsRunningUpgrade(true);
    try {
      const run = await runUpgradeSearch({ autoGrab: false, paused: true, force: true });
      toast.success(upgradeRunSummary(run));
      await invalidate(keys.wanted, keys.acquisitionQueue, keys.downloads(), keys.history());
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Upgrade search failed")));
    } finally {
      setIsRunningUpgrade(false);
    }
  }

  async function handleAuthorMonitor(options: AuthorMonitorOptions = {}) {
    setIsRunningAuthorMonitor(true);
    setAuthorMonitorTargetKey(options.targetKey ?? (options.authorIds?.[0] || options.providerKeys?.[0] || ""));
    try {
      const run = await runAuthorMonitor({
        authorIds: options.authorIds ?? [],
        providerKeys: options.providerKeys ?? [],
        force: options.force ?? false
      });
      setAuthorMonitorRun(run);
      toast.success(authorMonitorRunSummary(run));
      await invalidate(
        keys.authorSubscriptions,
        keys.authorMetadataReviews,
        keys.wanted,
        keys.wantedMetadataReview,
        keys.acquisitionQueue,
        keys.history()
      );
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Author monitor failed")));
    } finally {
      setIsRunningAuthorMonitor(false);
      setAuthorMonitorTargetKey("");
    }
  }

  const tabs = [
    { label: "Books", to: "/wanted" },
    { label: "Authors", to: "/wanted?tab=authors" }
  ];

  return (
    <>
      <PageHeader
        title="Wanted"
        subtitle="Missing books, release decisions, metadata review, and author subscriptions."
        actions={
          tab === "books" ? (
            <>
              <ToolbarButton
                icon={RadioTower}
                label={isRunningMonitor ? "Running monitor" : "Search Monitor"}
                busy={isRunningMonitor}
                title="Search releases for due wanted items (paused grabs)"
                onClick={() => void handleSearchMonitor()}
              />
              <ToolbarButton
                icon={Rss}
                label={isRunningFeedSync ? "Syncing feeds" : "Feed Sync"}
                busy={isRunningFeedSync}
                title="Match indexer feed releases against the wanted queue"
                onClick={() => void handleFeedSync()}
              />
              <ToolbarButton
                icon={TrendingUp}
                label={isRunningUpgrade ? "Searching upgrades" : "Upgrade Search"}
                busy={isRunningUpgrade}
                title="Look for better-scored releases for grabbed books"
                onClick={() => void handleUpgradeSearch()}
              />
            </>
          ) : (
            <>
              <ToolbarButton
                icon={RadioTower}
                label={isRunningAuthorMonitor ? "Running authors" : "Author Monitor"}
                busy={isRunningAuthorMonitor && !authorMonitorTargetKey}
                disabled={isRunningAuthorMonitor}
                title="Check due author subscriptions for new or missing books"
                onClick={() => void handleAuthorMonitor({ force: false })}
              />
              <ToolbarButton
                icon={RefreshCw}
                label="Force"
                disabled={isRunningAuthorMonitor}
                title="Force-refresh all author subscriptions"
                onClick={() => void handleAuthorMonitor({ force: true })}
              />
            </>
          )
        }
      >
        <div className="wanted-tab-row">
          <TabNav
            tabs={tabs}
            render={(navTab) => (
              <Link
                key={navTab.label}
                to={navTab.to}
                className={(navTab.label === "Authors") === (tab === "authors") ? "active" : undefined}
              >
                {navTab.label}
              </Link>
            )}
          />
        </div>
      </PageHeader>

      {tab === "books" ? (
        <BooksTab />
      ) : (
        <AuthorsTab
          monitorRun={authorMonitorRun}
          isRunningMonitor={isRunningAuthorMonitor}
          monitorTargetKey={authorMonitorTargetKey}
          runMonitor={handleAuthorMonitor}
        />
      )}
    </>
  );
}
