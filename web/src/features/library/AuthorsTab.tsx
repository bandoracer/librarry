import React, { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { ArrowUpRight, RadioTower, RefreshCw, SlidersHorizontal, Trash2, UserRoundSearch } from "lucide-react";
import {
  createWanted,
  deleteAuthorSubscription,
  resolveAuthorMetadataReview,
  runAuthorMonitor,
  updateAuthorSubscription
} from "../../lib/api";
import type {
  AuthorCollectionOptions,
  AuthorReviewOptions,
  AuthorFilterFields,
  AuthorFilterUpdateRequest,
  AuthorMetadataReview,
  AuthorMissingBookPolicy,
  AuthorMonitorRun,
  AuthorSkippedItem,
  AuthorSubscription,
  FilteredAuthorSubscription
} from "../../lib/api";
import {
  keys,
  useAuthorReviewCollection,
  useAuthorCollection,
  useMetadataProfiles,
  useRootFolders,
  useQualityProfiles
} from "../../lib/queries";
import { formatDateTime } from "../../lib/format";
import { useToast } from "../../components/toast";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  Field,
  FormGrid,
  IconButton,
  InlineNotice,
  LoadingRow,
  Modal,
  Toolbar,
  ToolbarButton
} from "../../components/ui";
// Author-subscription presentation helpers live with the wanted feature; this
// cross-feature import is intentional (the tab moved here in wave B).
import {
  appErrorMessage,
  authorMissingPolicyLabel,
  authorMissingPolicyOptions,
  authorMonitorRunSummary,
  authorSkippedDateLabel,
  authorSkippedItemKey,
  authorSubscriptionKey,
  authorSubscriptionMonitorOptions,
  authorSubscriptionStatsBadges,
  authorSubscriptionStatsSummary,
  emptyAuthorSubscriptionStats,
  errorMessage,
  firstAuthorName
} from "../wanted/lib";
import { libraryAuthorPath } from "./lib";
import "../wanted/wanted.css";

type AuthorMonitorOptions = {
  authorIds?: string[];
  providerKeys?: string[];
  force?: boolean;
  targetKey?: string;
};

/* ----------------------- Per-subscription add filters ----------------------- */

/** Editable string form of the metadata filters (comma inputs, number text). */
type AuthorFiltersForm = {
  rootFolderId: string;
  qualityProfile: string;
  metadataProfileId: string;
  allowedLanguages: string;
  mustNotContain: string;
  skipMissingIsbn: boolean;
  minPages: string;
};

/** The backend returns the M6 filter fields on the subscription payload. */
function subscriptionFilters(subscription: AuthorSubscription): AuthorFilterFields {
  return subscription as FilteredAuthorSubscription;
}

function activeFilterCount(subscription: AuthorSubscription): number {
  const filters = subscriptionFilters(subscription);
  let count = 0;
  if (subscription.metadataProfileId) count += 1;
  if (filters.allowedLanguages?.length) count += 1;
  if (filters.mustNotContain?.length) count += 1;
  if (filters.skipMissingIsbn) count += 1;
  if ((filters.minPages ?? 0) > 0) count += 1;
  return count;
}

function filtersToForm(subscription: AuthorSubscription): AuthorFiltersForm {
  const filters = subscriptionFilters(subscription);
  return {
    rootFolderId: subscription.rootFolderId ?? "",
    qualityProfile: subscription.qualityProfile,
    metadataProfileId: subscription.metadataProfileId ?? "",
    allowedLanguages: (filters.allowedLanguages ?? []).join(", "),
    mustNotContain: (filters.mustNotContain ?? []).join(", "),
    skipMissingIsbn: Boolean(filters.skipMissingIsbn),
    minPages: (filters.minPages ?? 0) > 0 ? String(filters.minPages) : ""
  };
}

function splitCommaTerms(value: string): string[] {
  return value
    .split(",")
    .map((term) => term.trim())
    .filter(Boolean);
}

/** Empty inputs clear the corresponding filter (empty lists / 0 / "" = disabled). */
function filtersFormPayload(form: AuthorFiltersForm): AuthorFilterUpdateRequest {
  return {
    rootFolderId: form.rootFolderId,
    qualityProfile: form.qualityProfile,
    metadataProfileId: form.metadataProfileId,
    allowedLanguages: splitCommaTerms(form.allowedLanguages),
    mustNotContain: splitCommaTerms(form.mustNotContain),
    skipMissingIsbn: form.skipMissingIsbn,
    minPages: Math.max(0, Math.round(Number(form.minPages) || 0))
  };
}

/**
 * Library → Authors tab: monitored author subscriptions (policy, metadata
 * profile, per-author filter overrides), author monitor runs (including
 * skipped candidates), and the author metadata review queue. Moved from the
 * Wanted page in the wave-B Readarr alignment; author rows link to
 * /library/author/:id.
 */
export function AuthorsTab() {
  const navigate = useNavigate();
  const toast = useToast();
  const client = useQueryClient();

  const [search, setSearch] = useState("");
  const [querySearch, setQuerySearch] = useState("");
  const [format, setFormat] = useState<AuthorCollectionOptions["format"]>("all");
  const [status, setStatus] = useState<AuthorCollectionOptions["status"]>("monitored");
  const [cursors, setCursors] = useState<string[]>([""]);
  useEffect(() => { if (search === querySearch) return; const timer = setTimeout(() => { setQuerySearch(search); setCursors([""]); }, 250); return () => clearTimeout(timer); }, [search, querySearch]);
  const subscriptionsQuery = useAuthorCollection({ q: querySearch, format, status, cursor: cursors[cursors.length - 1], limit: 100 });
  const [reviewSearch, setReviewSearch] = useState("");
  const [reviewQuery, setReviewQuery] = useState("");
  const [reviewStatus, setReviewStatus] = useState<AuthorReviewOptions["status"]>("pending");
  const [reviewFormat, setReviewFormat] = useState<AuthorReviewOptions["format"]>("all");
  const [reviewCursors, setReviewCursors] = useState<string[]>([""]);
  useEffect(() => { if (reviewSearch === reviewQuery) return; const timer = setTimeout(() => { setReviewQuery(reviewSearch); setReviewCursors([""]); }, 250); return () => clearTimeout(timer); }, [reviewSearch, reviewQuery]);
  const reviewsQuery = useAuthorReviewCollection({ q: reviewQuery, status: reviewStatus, format: reviewFormat, cursor: reviewCursors[reviewCursors.length - 1], limit: 6 });
  const metadataProfilesQuery = useMetadataProfiles();
  const rootsQuery = useRootFolders();
  const profilesQuery = useQualityProfiles();
  const collection = subscriptionsQuery.data;
  const subscriptions = collection?.authors ?? [];
  const reviews = useMemo(() => reviewsQuery.data?.reviews ?? [], [reviewsQuery.data]);
  const metadataProfiles = metadataProfilesQuery.data ?? [];

  const [isRunningMonitor, setIsRunningMonitor] = useState(false);
  const [monitorTargetKey, setMonitorTargetKey] = useState("");
  const [monitorRun, setMonitorRun] = useState<AuthorMonitorRun | null>(null);
  const [updatingAuthorID, setUpdatingAuthorID] = useState("");
  const [removingAuthorID, setRemovingAuthorID] = useState("");
  const [unsubscribeTarget, setUnsubscribeTarget] = useState<AuthorSubscription | null>(null);
  const [markingSkippedKey, setMarkingSkippedKey] = useState("");
  const [reviewActionID, setReviewActionID] = useState("");
  const [filtersOpenKey, setFiltersOpenKey] = useState("");
  const [filtersForm, setFiltersForm] = useState<AuthorFiltersForm | null>(null);
  const [savingFiltersID, setSavingFiltersID] = useState("");

  function invalidate(...queryKeys: readonly (readonly unknown[])[]) {
    return Promise.all(queryKeys.map((key) => client.invalidateQueries({ queryKey: key })));
  }

  async function runMonitor(options: AuthorMonitorOptions = {}) {
    setIsRunningMonitor(true);
    setMonitorTargetKey(options.targetKey ?? (options.authorIds?.[0] || options.providerKeys?.[0] || ""));
    try {
      const run = await runAuthorMonitor({
        authorIds: options.authorIds ?? [],
        providerKeys: options.providerKeys ?? [],
        force: options.force ?? false
      });
      setMonitorRun(run);
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
      setIsRunningMonitor(false);
      setMonitorTargetKey("");
    }
  }

  async function updateMissingPolicy(subscription: AuthorSubscription, missingBookPolicy: AuthorMissingBookPolicy) {
    if (!subscription.id || subscription.missingBookPolicy === missingBookPolicy) return;
    setUpdatingAuthorID(subscription.id);
    try {
      const updated = await updateAuthorSubscription(subscription.id, { missingBookPolicy });
      toast.success(`${updated.authorName}: missing-book policy set to ${authorMissingPolicyLabel(missingBookPolicy)}`);
      await invalidate(keys.authorSubscriptions);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Author subscription update failed")));
    } finally {
      setUpdatingAuthorID("");
    }
  }

  function toggleFilters(subscription: AuthorSubscription, monitorKey: string) {
    if (filtersOpenKey === monitorKey) {
      setFiltersOpenKey("");
      setFiltersForm(null);
      return;
    }
    setFiltersOpenKey(monitorKey);
    setFiltersForm(filtersToForm(subscription));
  }

  function updateFiltersForm(changes: Partial<AuthorFiltersForm>) {
    setFiltersForm((current) => (current ? { ...current, ...changes } : current));
  }

  async function saveFilters(subscription: AuthorSubscription) {
    if (!subscription.id || !filtersForm) return;
    setSavingFiltersID(subscription.id);
    try {
      await updateAuthorSubscription(subscription.id, filtersFormPayload(filtersForm));
      toast.success(`${subscription.authorName}: author settings updated`);
      setFiltersOpenKey("");
      setFiltersForm(null);
      await invalidate(keys.authorSubscriptions);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Author settings update failed")));
    } finally {
      setSavingFiltersID("");
    }
  }

  async function removeSubscription(subscription: AuthorSubscription) {
    if (!subscription.id) return;
    setRemovingAuthorID(subscription.id);
    try {
      await deleteAuthorSubscription(subscription.id);
      setUnsubscribeTarget(null);
      toast.success(`Unsubscribed from ${subscription.authorName}`);
      await invalidate(keys.authorSubscriptions, keys.authorMetadataReviews);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Author subscription remove failed")));
    } finally {
      setRemovingAuthorID("");
    }
  }

  async function markSkippedWanted(subscription: AuthorSubscription, skipped: AuthorSkippedItem) {
    const key = authorSkippedItemKey(subscription, skipped);
    setMarkingSkippedKey(key);
    try {
      if (skipped.reviewId) {
        const outcome = await resolveAuthorMetadataReview(skipped.reviewId, "wanted");
        toast.success(outcome.replayed ? "This review decision was already saved" : outcome.alreadyTracked ? "Already tracked; existing book settings retained" : `Marked “${outcome.wantedItem?.title ?? skipped.result.work.title}” wanted`);
      } else {
        const wantedFormat = skipped.result.edition?.format === "audiobook" ? "audiobook" : subscription.format;
        const item = await createWanted(skipped.result, wantedFormat, subscription.qualityProfile, subscription.tags ?? [], subscription.rootFolderId);
        toast.success(`Marked “${item.title}” wanted`);
      }
      await invalidate(keys.wanted, keys.authorMetadataReviews, keys.acquisitionQueue, keys.history());
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Mark skipped book wanted failed")));
    } finally {
      setMarkingSkippedKey("");
    }
  }

  async function resolveReview(review: AuthorMetadataReview, action: "wanted" | "ignore") {
    if (!review.id) return;
    setReviewActionID(`${review.id}:${action}`);
    try {
      const outcome = await resolveAuthorMetadataReview(review.id, action, review.revision);
      if (outcome.replayed) {
        toast.success("This review decision was already saved");
      } else if (outcome.alreadyTracked) {
        toast.success("Already tracked; existing book settings retained");
      } else if (action === "wanted") {
        toast.success(`Marked “${outcome.wantedItem?.title ?? review.title}” wanted`);
      } else {
        toast.success(`Ignored “${review.title || review.result.work.title}”`);
      }
      await invalidate(keys.authorMetadataReviews, keys.wanted, keys.acquisitionQueue, keys.history());
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Author metadata review update failed")));
    } finally {
      setReviewActionID("");
    }
  }

  const queryNotices = [
    subscriptionsQuery.error
      ? appErrorMessage(errorMessage(subscriptionsQuery.error, "Author subscriptions refresh failed"))
      : "",
    reviewsQuery.error ? appErrorMessage(errorMessage(reviewsQuery.error, "Author metadata review refresh failed")) : ""
  ].filter(Boolean);

  return (
    <>
      <Toolbar align="start">
        <ToolbarButton
          icon={RadioTower}
          label={isRunningMonitor ? "Running authors" : "Check Author Batch"}
          busy={isRunningMonitor && !monitorTargetKey}
          disabled={isRunningMonitor}
          title="Check up to 50 due author subscriptions for new or missing books"
          onClick={() => void runMonitor({ force: false })}
        />
        <ToolbarButton
          icon={RefreshCw}
          label="Force Author Batch"
          disabled={isRunningMonitor}
          title="Force-refresh up to 50 author subscriptions"
          onClick={() => void runMonitor({ force: true })}
        />
      </Toolbar>

      {queryNotices.map((notice) => (
        <InlineNotice key={notice} tone="danger">
          {notice}
        </InlineNotice>
      ))}

      <div className="library-pagination">
        <input aria-label="Filter author subscriptions" placeholder="Filter author subscriptions" value={search} onChange={event => setSearch(event.target.value)} />
        <select aria-label="Author subscription format" value={format} onChange={event => { setFormat(event.target.value as AuthorCollectionOptions["format"]); setCursors([""]); }}>
          <option value="all">All formats</option><option value="ebook">Ebooks</option><option value="audiobook">Audiobooks</option>
        </select>
        <select aria-label="Author subscription status" value={status} onChange={event => { setStatus(event.target.value as AuthorCollectionOptions["status"]); setCursors([""]); }}>
          <option value="monitored">Monitored</option><option value="unmonitored">Unmonitored</option><option value="all">All subscriptions</option>
        </select>
        <Button disabled={cursors.length === 1 || subscriptionsQuery.isFetching} onClick={() => setCursors(value => value.slice(0, -1))}>Previous</Button>
        <Button disabled={!collection?.nextCursor || subscriptionsQuery.isFetching} onClick={() => { if (collection?.nextCursor) setCursors(value => [...value, collection.nextCursor!]); }}>Next</Button>
        <Button onClick={() => void subscriptionsQuery.refetch()} disabled={subscriptionsQuery.isFetching}>Refresh page</Button>
      </div>
      {(collection?.downloads === "partial" || collection?.downloads === "unavailable") && <InlineNotice tone="info">Download-client evidence is incomplete. Some book counts are unknown.</InlineNotice>}
      <div className="wanted-author-grid">
        <Card
          title="Author subscriptions"
          subtitle={collection ? `${subscriptions.length} shown · ${collection.filtered} matching · ${collection.total} total subscriptions` : "Monitor authors for new or missing books."}
          padded={false}
        >
          {subscriptionsQuery.isLoading ? (
            <LoadingRow label="Loading author subscriptions…" />
          ) : subscriptionsQuery.isError ? (
            <EmptyState icon={UserRoundSearch} title="Author subscriptions could not be loaded">
              <Button onClick={() => void subscriptionsQuery.refetch()}>Retry</Button>
            </EmptyState>
          ) : subscriptions.length ? (
            <div className="wanted-author-list">
              {subscriptions.map((subscription) => {
                const monitorKey = authorSubscriptionKey(subscription);
                const refreshingAuthor = monitorTargetKey === monitorKey;
                const stats = { ...emptyAuthorSubscriptionStats(), ...subscription.counts, total: subscription.totalBooks };
                const filterCount = activeFilterCount(subscription);
                const filtersOpen = filtersOpenKey === monitorKey;
                const activeProfile = metadataProfiles.find((profile) => profile.id === subscription.metadataProfileId);
                return (
                  <React.Fragment key={monitorKey}>
                    <article className="wanted-author-row">
                      <div className="wanted-author-main">
                        <Link className="cell-primary" to={libraryAuthorPath(subscription.authorName, subscription.id)}>
                          <strong>{subscription.authorName}</strong>
                        </Link>
                        <span>
                          {subscription.provider} · {subscription.format} · {subscription.qualityProfile}
                          {activeProfile ? ` · ${activeProfile.name}` : ""}
                        </span>
                        <small className="wanted-author-stats">{subscription.identityLinked ? authorSubscriptionStatsSummary(stats) : "Provider identity is not linked. Book counts are unavailable."}</small>
                        <div className="wanted-author-counts" aria-label={`${subscription.authorName} wanted book status`}>
                          <Badge tone={subscription.status === "monitored" && subscription.monitorNewItems ? "success" : "neutral"}>
                            {subscription.status !== "monitored" ? "Unmonitored" : subscription.monitorNewItems ? "Monitored" : "Not monitoring new"}
                          </Badge>
                          {filterCount > 0 ? (
                            <Badge tone="info" title={`${filterCount} add filter${filterCount === 1 ? "" : "s"} active`}>
                              {filterCount} filter{filterCount === 1 ? "" : "s"}
                            </Badge>
                          ) : null}
                          {subscription.identityLinked && authorSubscriptionStatsBadges(stats).map(([label, value]) => (
                            <Badge key={label}>
                              {value} {label}
                            </Badge>
                          ))}
                        </div>
                      </div>
                      <div className="wanted-author-controls">
                        <select
                          aria-label={`${subscription.authorName} missing-book policy`}
                          disabled={updatingAuthorID === subscription.id}
                          onChange={(event) => void updateMissingPolicy(subscription, event.target.value as AuthorMissingBookPolicy)}
                          value={authorMissingPolicyOptions.includes(subscription.missingBookPolicy) ? subscription.missingBookPolicy : "all"}
                        >
                          {authorMissingPolicyOptions.map((option) => (
                            <option key={option} value={option}>
                              {authorMissingPolicyLabel(option)}
                            </option>
                          ))}
                        </select>
                        <div className="wanted-author-actions">
                          <IconButton
                            icon={SlidersHorizontal}
                            tone={filtersOpen ? "accent" : filterCount > 0 ? "info" : "neutral"}
                            label={`${filtersOpen ? "Hide" : "Edit"} author settings for ${subscription.authorName}`}
                            onClick={() => toggleFilters(subscription, monitorKey)}
                          />
                          <IconButton
                            icon={ArrowUpRight}
                            label={`Open ${subscription.authorName} author page`}
                            onClick={() => navigate(libraryAuthorPath(subscription.authorName, subscription.id))}
                          />
                          <IconButton
                            icon={RefreshCw}
                            label={refreshingAuthor ? `Refreshing ${subscription.authorName}` : `Refresh ${subscription.authorName}`}
                            disabled={isRunningMonitor}
                            busy={refreshingAuthor && isRunningMonitor}
                            onClick={() => void runMonitor(authorSubscriptionMonitorOptions(subscription))}
                          />
                          <IconButton
                            icon={Trash2}
                            tone="danger"
                            label={`Unsubscribe from ${subscription.authorName}`}
                            disabled={!subscription.id || Boolean(removingAuthorID)}
                            onClick={() => setUnsubscribeTarget(subscription)}
                          />
                        </div>
                        <em>{subscription.lastSyncAt ? formatDateTime(subscription.lastSyncAt) : "never synced"}</em>
                      </div>
                    </article>
                    {filtersOpen && filtersForm ? (
                      <div style={{ padding: "2px 0 14px", borderBottom: "1px solid var(--border)" }}>
                        <FormGrid columns={2}>
                          <Field label="Root folder" hint="Applies to future additions; existing books keep their destination.">
                            <select value={filtersForm.rootFolderId} aria-label={`${subscription.authorName} root folder`}
                              onChange={event => updateFiltersForm({ rootFolderId: event.target.value })}>
                              <option value="">Format default</option>
                              {filtersForm.rootFolderId && !(rootsQuery.data ?? []).some(root => root.id === filtersForm.rootFolderId && root.mediaFormat === subscription.format) ? <option value={filtersForm.rootFolderId}>Saved root unavailable or incompatible</option> : null}
                              {(rootsQuery.data ?? []).filter(root => root.mediaFormat === subscription.format).map(root => <option key={root.id} value={root.id}>{root.name || root.path}</option>)}
                            </select>
                          </Field>
                          <Field label="Quality profile" hint="Applies to newly added books.">
                            <select value={filtersForm.qualityProfile} aria-label={`${subscription.authorName} quality profile`}
                              onChange={event => updateFiltersForm({ qualityProfile: event.target.value })}>
                              {!(profilesQuery.data ?? []).some(profile => profile.name === filtersForm.qualityProfile && (profile.mediaFormat === "any" || profile.mediaFormat === subscription.format)) ? <option value={filtersForm.qualityProfile}>{filtersForm.qualityProfile}</option> : null}
                              {(profilesQuery.data ?? []).filter(profile => profile.mediaFormat === "any" || profile.mediaFormat === subscription.format).map(profile => <option key={profile.name} value={profile.name}>{profile.name}</option>)}
                            </select>
                          </Field>
                          <div className="settings-field-wide">
                            <Field
                              label="Metadata profile"
                              hint="When selected, this profile supplies the filters instead of the per-author fields below."
                            >
                              <select
                                value={filtersForm.metadataProfileId}
                                onChange={(event) => updateFiltersForm({ metadataProfileId: event.target.value })}
                                aria-label={`${subscription.authorName} metadata profile`}
                              >
                                <option value="">No profile (overrides only)</option>
                                {metadataProfiles.map((profile) => (
                                  <option key={profile.id} value={profile.id}>
                                    {profile.name}
                                  </option>
                                ))}
                              </select>
                            </Field>
                          </div>
                          <Field
                            label="Allowed languages"
                            hint="Comma-separated; leave empty to allow every language."
                          >
                            <input
                              value={filtersForm.allowedLanguages}
                              onChange={(event) => updateFiltersForm({ allowedLanguages: event.target.value })}
                              placeholder="English, German"
                              aria-label={`${subscription.authorName} allowed languages`}
                            />
                          </Field>
                          <Field label="Must not contain" hint="Comma-separated terms that reject a candidate title.">
                            <input
                              value={filtersForm.mustNotContain}
                              onChange={(event) => updateFiltersForm({ mustNotContain: event.target.value })}
                              placeholder="omnibus, boxed set"
                              aria-label={`${subscription.authorName} must-not-contain terms`}
                            />
                          </Field>
                          <Field label="Minimum pages" hint="0 disables; applies when the provider reports pages.">
                            <input
                              type="number"
                              min={0}
                              value={filtersForm.minPages}
                              onChange={(event) => updateFiltersForm({ minPages: event.target.value })}
                              placeholder="0"
                              aria-label={`${subscription.authorName} minimum pages`}
                            />
                          </Field>
                          <label
                            style={{ display: "inline-flex", alignItems: "center", gap: 7, cursor: "pointer", alignSelf: "end", paddingBottom: 6, fontSize: "12.5px" }}
                          >
                            <input
                              type="checkbox"
                              checked={filtersForm.skipMissingIsbn}
                              onChange={(event) => updateFiltersForm({ skipMissingIsbn: event.target.checked })}
                              aria-label={`${subscription.authorName} skip books without ISBN`}
                            />
                            <span>Skip books without ISBN</span>
                          </label>
                        </FormGrid>
                        <div className="form-actions">
                          <Button
                            size="sm"
                            variant="primary"
                            busy={savingFiltersID === subscription.id}
                            disabled={!subscription.id || Boolean(savingFiltersID)}
                            onClick={() => void saveFilters(subscription)}
                          >
                            Save author settings
                          </Button>
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => {
                              setFiltersOpenKey("");
                              setFiltersForm(null);
                            }}
                          >
                            Cancel
                          </Button>
                        </div>
                      </div>
                    ) : null}
                  </React.Fragment>
                );
              })}
            </div>
          ) : (
            <EmptyState icon={UserRoundSearch} title={collection?.total ? "No matching subscriptions on this page" : "No author subscriptions"}>
              {collection?.total ? "Change the filters or return to the first page." : "Subscribe to an author from metadata search to monitor for new or missing books."}
              {cursors.length > 1 && <Button onClick={() => setCursors([""])}>First page</Button>}
            </EmptyState>
          )}
        </Card>

        <div className="wanted-detail-stack">
          <Card
            title="Author monitor"
            subtitle={
              monitorRun
                ? `${monitorRun.status}: checked ${monitorRun.authorsChecked}, created ${monitorRun.wantedCreated} wanted items.`
                : "Run due authors to refresh monitored writers."
            }
          >
            {monitorRun ? (
              <>
                <div className="wanted-monitor-summary">
                  <strong>{monitorRun.status}</strong>
                  <span>
                    {monitorRun.authorsChecked} checked · {monitorRun.itemsFound} metadata hits · {monitorRun.wantedCreated}{" "}
                    wanted · {monitorRun.errorCount} errors
                  </span>
                </div>
                {monitorRun.items?.slice(0, 8).map((item) => (
                  <article
                    className="wanted-monitor-row"
                    key={item.subscription.id || item.subscription.providerKey}
                  >
                    <div className="wanted-monitor-row-head">
                      <div>
                        <strong>{item.subscription.authorName}</strong>
                        <span>
                          {item.resultsFound} hits · {item.wantedCreated} wanted · {item.skippedCount ?? 0} skipped
                          {item.error ? ` · ${item.error}` : ""}
                        </span>
                      </div>
                      <Badge tone={item.error ? "danger" : "neutral"}>{item.subscription.format}</Badge>
                    </div>
                    {item.skippedItems?.length ? (
                      <div className="wanted-skipped-list">
                        {item.skippedItems.slice(0, 4).map((skipped) => {
                          const skippedKey = authorSkippedItemKey(item.subscription, skipped);
                          const busy = markingSkippedKey === skippedKey;
                          return (
                            <div className="wanted-skipped-row" key={skippedKey}>
                              <div>
                                <strong>{skipped.result.work.title || skipped.result.edition?.title || "Untitled"}</strong>
                                <span>
                                  {firstAuthorName(skipped.result)} · {authorSkippedDateLabel(skipped.result)} · {skipped.reason}
                                </span>
                              </div>
                              <Button
                                size="sm"
                                disabled={Boolean(markingSkippedKey) && !busy}
                                busy={busy}
                                onClick={() => void markSkippedWanted(item.subscription, skipped)}
                              >
                                {busy ? "Marking" : "Mark wanted"}
                              </Button>
                            </div>
                          );
                        })}
                      </div>
                    ) : null}
                  </article>
                ))}
              </>
            ) : (
              <EmptyState icon={UserRoundSearch} title="No author monitor run yet">
                Use “Author Monitor” in the toolbar to check due authors, or “Force” to refresh all of them.
              </EmptyState>
            )}
          </Card>

          <Card
            title="Author review queue"
            subtitle={`${reviews.length} shown · ${reviewsQuery.data?.filtered ?? 0} matching · ${reviewsQuery.data?.counts.pending ?? 0} pending`}
            actions={
              <Button
                size="sm"
                icon={RefreshCw}
                disabled={Boolean(reviewActionID)}
                busy={reviewsQuery.isFetching}
                onClick={() => void reviewsQuery.refetch()}
              >
                Refresh
              </Button>
            }
          >
            <FormGrid>
              <Field label="Search author reviews"><input aria-label="Search author reviews" value={reviewSearch} onChange={event => setReviewSearch(event.target.value)} /></Field>
              <Field label="Review status"><select aria-label="Author review status" value={reviewStatus} onChange={event => { setReviewStatus(event.target.value as AuthorReviewOptions["status"]); setReviewCursors([""]); }}>
                <option value="pending">Pending</option><option value="wanted">Wanted</option><option value="ignored">Ignored</option><option value="all">All decisions</option>
              </select></Field>
              <Field label="Review format"><select aria-label="Author review format" value={reviewFormat} onChange={event => { setReviewFormat(event.target.value as AuthorReviewOptions["format"]); setReviewCursors([""]); }}>
                <option value="all">All formats</option><option value="ebook">Ebook</option><option value="audiobook">Audiobook</option>
              </select></Field>
            </FormGrid>
            <div className="library-pagination">
              <Button disabled={reviewCursors.length === 1 || reviewsQuery.isFetching || Boolean(reviewActionID)} onClick={() => setReviewCursors(value => value.slice(0, -1))}>Previous reviews</Button>
              <span>Page {reviewCursors.length}</span>
              <Button disabled={!reviewsQuery.data?.nextCursor || reviewsQuery.isFetching || Boolean(reviewActionID)} onClick={() => { if (reviewsQuery.data?.nextCursor) setReviewCursors(value => [...value, reviewsQuery.data!.nextCursor!]); }}>Next reviews</Button>
            </div>
            {reviewsQuery.isError ? (
              <EmptyState title="Author reviews could not be loaded"><Button onClick={() => void reviewsQuery.refetch()}>Retry reviews</Button></EmptyState>
            ) : reviewsQuery.isLoading ? (
              <LoadingRow label="Loading author reviews…" />
            ) : reviews.length ? (
              reviews.map((review) => {
                const wantedActionID = `${review.id}:wanted`;
                const ignoreActionID = `${review.id}:ignore`;
                return (
                  <article className="wanted-review-queue-row" key={review.id}>
                    <div>
                      <strong>{review.title || review.result.work.title || "Untitled"}</strong>
                      <span>
                        {review.authorName || firstAuthorName(review.result)} · {authorSkippedDateLabel(review.result)} ·{" "}
                        {review.reason}
                      </span>
                      <span>
                        {review.qualityProfile} · {review.rootFolderId
                          ? (rootsQuery.data ?? []).find(root => root.id === review.rootFolderId)?.path ?? "Saved destination"
                          : "Format default destination"}
                      </span>
                    </div>
                    <div className="wanted-review-queue-actions">
                      {review.status !== "pending" ? (review.wantedId ? <Link to={`/library/book/${review.wantedId}`}>Open tracked book</Link> : <Badge>{review.status}</Badge>) : <>
                      <Button
                        size="sm"
                        disabled={Boolean(reviewActionID) && reviewActionID !== wantedActionID}
                        busy={reviewActionID === wantedActionID}
                        onClick={() => void resolveReview(review, "wanted")}
                      >
                        {reviewActionID === wantedActionID ? "Marking" : "Mark wanted"}
                      </Button>
                      <Button
                        size="sm"
                        variant="danger"
                        disabled={Boolean(reviewActionID) && reviewActionID !== ignoreActionID}
                        busy={reviewActionID === ignoreActionID}
                        onClick={() => void resolveReview(review, "ignore")}
                      >
                        {reviewActionID === ignoreActionID ? "Ignoring" : "Ignore"}
                      </Button>
                      </>}
                    </div>
                  </article>
                );
              })
            ) : (
              <EmptyState title="No matching author reviews">
                Try another filter or refresh the queue.
              </EmptyState>
            )}
          </Card>
        </div>
      </div>

      <Modal
        title="Unsubscribe author"
        open={Boolean(unsubscribeTarget)}
        onClose={() => setUnsubscribeTarget(null)}
        footer={
          <>
            <Button onClick={() => setUnsubscribeTarget(null)}>Cancel</Button>
            <Button
              variant="danger"
              icon={Trash2}
              busy={Boolean(removingAuthorID)}
              onClick={() => unsubscribeTarget && void removeSubscription(unsubscribeTarget)}
            >
              {removingAuthorID ? "Removing" : "Unsubscribe"}
            </Button>
          </>
        }
      >
        <p>
          Stop monitoring <strong>{unsubscribeTarget?.authorName ?? "this author"}</strong>? Pending metadata reviews for
          this subscription are discarded; existing wanted books are kept.
        </p>
      </Modal>
    </>
  );
}
