import React, { useMemo, useState } from "react";
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
  useAuthorMetadataReviews,
  useAuthorSubscriptions,
  useLibraryFiles,
  useMetadataProfiles,
  useWanted,
  useWantedMetadataReview
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
  buildAuthorSubscriptionStatsMap,
  emptyAuthorSubscriptionStats,
  errorMessage,
  firstAuthorName,
  metadataReviewMap,
  wantedPresenceMap
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

  const subscriptionsQuery = useAuthorSubscriptions();
  const reviewsQuery = useAuthorMetadataReviews();
  const wantedQuery = useWanted();
  const wantedReviewQuery = useWantedMetadataReview();
  const filesQuery = useLibraryFiles("any");
  const metadataProfilesQuery = useMetadataProfiles();

  const subscriptions = useMemo(() => subscriptionsQuery.data ?? [], [subscriptionsQuery.data]);
  const reviews = useMemo(() => reviewsQuery.data ?? [], [reviewsQuery.data]);
  const wantedItems = useMemo(() => wantedQuery.data ?? [], [wantedQuery.data]);
  const libraryFiles = useMemo(() => filesQuery.data ?? [], [filesQuery.data]);
  const metadataProfiles = metadataProfilesQuery.data ?? [];

  const presence = useMemo(() => wantedPresenceMap(wantedItems, libraryFiles), [wantedItems, libraryFiles]);
  const reviewByID = useMemo(() => metadataReviewMap(wantedReviewQuery.data), [wantedReviewQuery.data]);
  const statsByKey = useMemo(
    () => buildAuthorSubscriptionStatsMap(subscriptions, wantedItems, presence, reviewByID),
    [subscriptions, wantedItems, presence, reviewByID]
  );

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
      toast.success(`${subscription.authorName}: add filters updated`);
      setFiltersOpenKey("");
      setFiltersForm(null);
      await invalidate(keys.authorSubscriptions);
    } catch (error) {
      toast.error(appErrorMessage(errorMessage(error, "Author filters update failed")));
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
        toast.success(`Marked “${outcome.wantedItem?.title ?? skipped.result.work.title}” wanted`);
      } else {
        const wantedFormat = skipped.result.edition?.format === "audiobook" ? "audiobook" : subscription.format;
        const item = await createWanted(skipped.result, wantedFormat, subscription.qualityProfile, subscription.tags ?? []);
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
      const outcome = await resolveAuthorMetadataReview(review.id, action);
      if (action === "wanted") {
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
          label={isRunningMonitor ? "Running authors" : "Author Monitor"}
          busy={isRunningMonitor && !monitorTargetKey}
          disabled={isRunningMonitor}
          title="Check due author subscriptions for new or missing books"
          onClick={() => void runMonitor({ force: false })}
        />
        <ToolbarButton
          icon={RefreshCw}
          label="Force"
          disabled={isRunningMonitor}
          title="Force-refresh all author subscriptions"
          onClick={() => void runMonitor({ force: true })}
        />
      </Toolbar>

      {queryNotices.map((notice) => (
        <InlineNotice key={notice} tone="danger">
          {notice}
        </InlineNotice>
      ))}

      <div className="wanted-author-grid">
        <Card
          title="Author subscriptions"
          subtitle={
            subscriptions.length
              ? `${subscriptions.length} monitored author${subscriptions.length === 1 ? "" : "s"} can create wanted items from metadata search.`
              : "Monitor authors for new or missing books before release acquisition."
          }
          padded={false}
        >
          {subscriptionsQuery.isLoading ? (
            <LoadingRow label="Loading author subscriptions…" />
          ) : subscriptions.length ? (
            <div className="wanted-author-list">
              {subscriptions.map((subscription) => {
                const monitorKey = authorSubscriptionKey(subscription);
                const refreshingAuthor = monitorTargetKey === monitorKey;
                const stats = statsByKey.get(monitorKey) ?? emptyAuthorSubscriptionStats();
                const filterCount = activeFilterCount(subscription);
                const filtersOpen = filtersOpenKey === monitorKey;
                const activeProfile = metadataProfiles.find((profile) => profile.id === subscription.metadataProfileId);
                return (
                  <React.Fragment key={monitorKey}>
                    <article className="wanted-author-row">
                      <div className="wanted-author-main">
                        <Link className="cell-primary" to={libraryAuthorPath(subscription.authorName)}>
                          <strong>{subscription.authorName}</strong>
                        </Link>
                        <span>
                          {subscription.provider} · {subscription.format} · {subscription.qualityProfile}
                          {activeProfile ? ` · ${activeProfile.name}` : ""}
                        </span>
                        <small className="wanted-author-stats">{authorSubscriptionStatsSummary(stats)}</small>
                        <div className="wanted-author-counts" aria-label={`${subscription.authorName} wanted book status`}>
                          <Badge tone={subscription.monitorNewItems ? "success" : "neutral"}>
                            {subscription.monitorNewItems ? "Monitored" : "Not monitoring new"}
                          </Badge>
                          {filterCount > 0 ? (
                            <Badge tone="info" title={`${filterCount} add filter${filterCount === 1 ? "" : "s"} active`}>
                              {filterCount} filter{filterCount === 1 ? "" : "s"}
                            </Badge>
                          ) : null}
                          {authorSubscriptionStatsBadges(stats).map(([label, value]) => (
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
                            label={`${filtersOpen ? "Hide" : "Edit"} add filters for ${subscription.authorName}`}
                            onClick={() => toggleFilters(subscription, monitorKey)}
                          />
                          <IconButton
                            icon={ArrowUpRight}
                            label={`Open ${subscription.authorName} author page`}
                            onClick={() => navigate(libraryAuthorPath(subscription.authorName))}
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
                          <div className="settings-field-wide">
                            <Field
                              label="Metadata profile"
                              hint="Primary filter set; the fields below override it per author."
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
                            Save filters
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
            <EmptyState icon={UserRoundSearch} title="No author subscriptions">
              Subscribe to an author from metadata search to monitor for new or missing books.
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
            subtitle={
              reviews.length
                ? `${reviews.length} skipped metadata candidate${reviews.length === 1 ? "" : "s"} need review.`
                : "No skipped author candidates are pending review."
            }
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
            {reviewsQuery.isLoading ? (
              <LoadingRow label="Loading author reviews…" />
            ) : reviews.length ? (
              reviews.slice(0, 6).map((review) => {
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
                    </div>
                    <div className="wanted-review-queue-actions">
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
                    </div>
                  </article>
                );
              })
            ) : (
              <EmptyState title="Nothing pending review">
                Skipped author candidates appear here when the monitor holds books back per policy.
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
