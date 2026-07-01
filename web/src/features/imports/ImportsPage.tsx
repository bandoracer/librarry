import React, { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  CheckCircle2,
  CheckSquare,
  FileCheck2,
  FolderInput,
  FolderSearch,
  HardDriveDownload,
  Inbox,
  RefreshCw,
  Square,
  Trash2,
  UploadCloud
} from "lucide-react";
import {
  Badge,
  Button,
  Card,
  DataTable,
  EmptyState,
  Field,
  FormGrid,
  InlineNotice,
  LoadingRow,
  Modal,
  PageHeader,
  Segmented,
  StatBar,
  ToolbarButton
} from "../../components/ui";
import { useToast } from "../../components/toast";
import { keys, useImportReviews, useInvalidatingMutation, useLibraryFiles } from "../../lib/queries";
import {
  importCompletedDownloads,
  importLibraryFile,
  resolveLibraryImportReview,
  resolveLibraryImportReviewsBulk,
  scanLibrary,
  type CompletedImportOutcome,
  type ImportReview,
  type LibraryImportOutcome,
  type LibraryScanOutcome,
  type ReviewBulkDecisionOutcome
} from "../../lib/api";
import { formatBytes, truncateMiddle } from "../../lib/format";
import { navItems } from "../../app/nav";
import {
  errorText,
  fileName,
  importReviewCandidateLabel,
  importReviewCandidateOptionLabel,
  importReviewEvidenceChips,
  importReviewResolvedFormat,
  importReviewResolvedWantedID,
  importReviewSuggestedWanted,
  importReviewWantedCandidates,
  isPersistenceRequiredError,
  libraryErrorMessage,
  stringMetadataValue
} from "./lib";
import "./imports.css";

type ScanFormat = "ebook" | "audiobook" | "any";
type ImportMode = "copy" | "move" | "hardlink" | "hardlinkOrCopy";
type ConflictAction = "rename" | "replace" | "skip" | "fail";
type ReviewAction = "import" | "skip" | "reject";
type ReviewStatusFilter = "pending" | "resolved" | "all";
type FileFormatFilter = "any" | "ebook" | "audiobook";

const pageSubtitle = navItems.find((item) => item.id === "imports")?.subtitle;

const fileKeys = [keys.libraryFiles("any"), keys.libraryFiles("ebook"), keys.libraryFiles("audiobook")] as const;
const reviewKeys = [keys.importReviews("pending"), keys.importReviews("resolved"), keys.importReviews("all")] as const;
const downstreamKeys = [keys.downloads(), keys.history(), keys.wanted] as const;

const TRACKED_FILE_ROW_CAP = 100;

function plural(count: number, singular: string, pluralWord = `${singular}s`): string {
  return `${count} ${count === 1 ? singular : pluralWord}`;
}

function importStatusTone(status: string) {
  const normalized = (status || "").toLowerCase();
  if (normalized === "imported") return "success" as const;
  if (normalized === "available") return "info" as const;
  if (normalized === "pending" || normalized === "review") return "warn" as const;
  if (normalized === "error" || normalized === "failed" || normalized === "rejected") return "danger" as const;
  return "neutral" as const;
}

function mediaFormatTone(format: string) {
  if (format === "ebook") return "info" as const;
  if (format === "audiobook") return "accent" as const;
  return "warn" as const;
}

export default function ImportsPage() {
  const toast = useToast();

  /* ------------------------------ Form state ------------------------------ */
  const [scanRoot, setScanRoot] = useState("");
  const [importPath, setImportPath] = useState("");
  const [importFormat, setImportFormat] = useState<"ebook" | "audiobook">("ebook");
  const [importMode, setImportMode] = useState<ImportMode>("copy");
  const [conflictAction, setConflictAction] = useState<ConflictAction>("rename");

  /* ---------------------------- Review selection --------------------------- */
  const [reviewStatus, setReviewStatus] = useState<ReviewStatusFilter>("pending");
  const [selectedReviewIDs, setSelectedReviewIDs] = useState<string[]>([]);
  const [reviewWantedChoices, setReviewWantedChoices] = useState<Record<string, string>>({});
  const [reviewActionID, setReviewActionID] = useState("");
  const [bulkModalOpen, setBulkModalOpen] = useState(false);
  const [bulkAction, setBulkAction] = useState<ReviewAction>("import");

  /* ------------------------------ Last outcomes ---------------------------- */
  const [scanActionID, setScanActionID] = useState("");
  const [lastScan, setLastScan] = useState<LibraryScanOutcome | null>(null);
  const [lastImport, setLastImport] = useState<LibraryImportOutcome | null>(null);
  const [lastCompleted, setLastCompleted] = useState<CompletedImportOutcome | null>(null);
  const [lastBulk, setLastBulk] = useState<ReviewBulkDecisionOutcome | null>(null);

  /* ------------------------------ Tracked files ---------------------------- */
  const [fileFormat, setFileFormat] = useState<FileFormatFilter>("any");

  /* --------------------------------- Queries ------------------------------- */
  const reviewsQuery = useImportReviews(reviewStatus);
  const pendingReviewsQuery = useImportReviews("pending");
  const filesQuery = useLibraryFiles(fileFormat);
  const allFilesQuery = useLibraryFiles("any");

  const reviews = useMemo(() => reviewsQuery.data ?? [], [reviewsQuery.data]);
  const pendingReviews = pendingReviewsQuery.data ?? [];
  const files = filesQuery.data ?? [];
  const allFiles = allFilesQuery.data ?? [];

  /* -------------------------------- Mutations ------------------------------ */
  const scanMutation = useInvalidatingMutation(
    (args: { format: ScanFormat; root?: string }) => scanLibrary(args.format, { root: args.root }),
    [...fileKeys]
  );
  const importMutation = useInvalidatingMutation(importLibraryFile, [...fileKeys, keys.wanted]);
  const completedMutation = useInvalidatingMutation(importCompletedDownloads, [...fileKeys, ...reviewKeys, ...downstreamKeys]);
  const resolveMutation = useInvalidatingMutation(
    (args: { reviewId: string; options: Parameters<typeof resolveLibraryImportReview>[1] }) =>
      resolveLibraryImportReview(args.reviewId, args.options),
    [...fileKeys, ...reviewKeys, ...downstreamKeys]
  );
  const bulkMutation = useInvalidatingMutation(resolveLibraryImportReviewsBulk, [...fileKeys, ...reviewKeys, ...downstreamKeys]);

  const isScanning = scanMutation.isPending;
  const reviewBusy = Boolean(reviewActionID);

  /* --------------------------- Selection derivations ----------------------- */
  const selectableReviewIDs = useMemo(
    () => reviews.filter((review) => review.status === "pending").map((review) => review.id).filter(Boolean),
    [reviews]
  );
  const selectedReviewSet = useMemo(() => new Set(selectedReviewIDs), [selectedReviewIDs]);
  const selectedReviews = useMemo(
    () => reviews.filter((review) => review.status === "pending" && selectedReviewSet.has(review.id)),
    [reviews, selectedReviewSet]
  );
  const allReviewsSelected =
    selectableReviewIDs.length > 0 && selectableReviewIDs.every((id) => selectedReviewSet.has(id));
  const selectedReviewsCanImport =
    selectedReviews.length > 0 &&
    selectedReviews.every((review) => Boolean(importReviewResolvedWantedID(review, reviewWantedChoices)));

  // Prune selections when reviews disappear (resolved elsewhere or refetched).
  useEffect(() => {
    const available = new Set(reviews.filter((review) => review.status === "pending").map((review) => review.id));
    setSelectedReviewIDs((current) => {
      const next = current.filter((id) => available.has(id));
      return next.length === current.length ? current : next;
    });
  }, [reviews]);

  function toggleReviewSelection(review: ImportReview) {
    if (!review.id) return;
    setSelectedReviewIDs((current) =>
      current.includes(review.id) ? current.filter((id) => id !== review.id) : [...current, review.id]
    );
  }

  function toggleAllReviews() {
    setSelectedReviewIDs((current) => {
      const next = new Set(current);
      const everySelected = selectableReviewIDs.length > 0 && selectableReviewIDs.every((id) => next.has(id));
      for (const id of selectableReviewIDs) {
        if (everySelected) {
          next.delete(id);
        } else {
          next.add(id);
        }
      }
      return Array.from(next);
    });
  }

  /* --------------------------------- Actions ------------------------------- */

  async function runScan(actionID: string, format: ScanFormat, root?: string) {
    setScanActionID(actionID);
    try {
      const outcome = await scanMutation.mutateAsync({ format, root });
      setLastScan(outcome);
      const message = `Scanned ${plural(outcome.roots.length, "root")}, ${plural(outcome.upserted, "new file")} (${outcome.scanned} seen, ${outcome.skipped} skipped)`;
      if (outcome.errors?.length) {
        toast.notify(`${message}; ${plural(outcome.errors.length, "error")}`, "warn");
      } else {
        toast.success(message);
      }
    } catch (error) {
      toast.error(errorText(error, "Library scan failed"));
    } finally {
      setScanActionID("");
    }
  }

  async function runManualImport() {
    const sourcePath = importPath.trim();
    if (!sourcePath) return;
    try {
      const outcome = await importMutation.mutateAsync({
        sourcePath,
        format: importFormat,
        move: importMode === "move",
        importMode,
        conflictAction,
        overwrite: conflictAction === "replace"
      });
      setLastImport(outcome);
      if (outcome.skipped) {
        toast.notify(outcome.message || "Import skipped", "warn");
      } else {
        toast.success(`Imported ${outcome.file.title || fileName(outcome.destinationPath)}`);
      }
    } catch (error) {
      toast.error(errorText(error, "Library import failed"));
    }
  }

  async function runCompletedImport() {
    try {
      const outcome = await completedMutation.mutateAsync({
        move: importMode === "move",
        importMode,
        conflictAction,
        overwrite: conflictAction === "replace",
        limit: 50
      });
      setLastCompleted(outcome);
      const message = `Checked ${plural(outcome.checked, "download")}: ${outcome.imported} imported, ${outcome.autoMatched} auto-matched, ${outcome.reviewQueued} queued for review, ${outcome.skipped} skipped, ${plural(outcome.errored, "error")}`;
      toast.notify(message, outcome.errored ? "warn" : "success");
    } catch (error) {
      toast.error(errorText(error, "Completed import failed"));
    }
  }

  async function runResolveReview(review: ImportReview, action: ReviewAction) {
    const wantedID = importReviewResolvedWantedID(review, reviewWantedChoices);
    setReviewActionID(`${review.id}:${action}`);
    setLastBulk(null);
    try {
      const nextFormat =
        review.mediaFormat === "unknown" ? importReviewResolvedFormat(review, wantedID, importFormat) : review.mediaFormat;
      const outcome = await resolveMutation.mutateAsync({
        reviewId: review.id,
        options: {
          action,
          wantedId: action === "import" ? wantedID : review.wantedId,
          format: nextFormat,
          move: importMode === "move",
          importMode,
          conflictAction,
          overwrite: conflictAction === "replace"
        }
      });
      if (outcome.import?.imported) {
        setLastImport(outcome.import);
      }
      setSelectedReviewIDs((current) => current.filter((id) => id !== review.id));
      setReviewWantedChoices((current) => {
        const next = { ...current };
        delete next[review.id];
        return next;
      });
      const label = review.title || fileName(review.sourcePath);
      toast.success(
        action === "import" ? `Imported ${label}` : action === "skip" ? `Skipped ${label}` : `Rejected ${label}`
      );
    } catch (error) {
      toast.error(errorText(error, "Import review update failed"));
    } finally {
      setReviewActionID("");
    }
  }

  async function runBulkResolve() {
    const reviewIDs = selectedReviews.map((review) => review.id).filter(Boolean);
    if (!reviewIDs.length) return;
    const singleReview = selectedReviews.length === 1 ? selectedReviews[0] : undefined;
    const singleWantedID = singleReview ? importReviewResolvedWantedID(singleReview, reviewWantedChoices) : "";
    const singleFormat = singleReview
      ? singleReview.mediaFormat === "unknown"
        ? importReviewResolvedFormat(singleReview, singleWantedID, importFormat)
        : singleReview.mediaFormat
      : undefined;
    setReviewActionID(`bulk:${bulkAction}`);
    setLastBulk(null);
    try {
      const outcome = await bulkMutation.mutateAsync({
        ids: reviewIDs,
        action: bulkAction,
        wantedId: bulkAction === "import" && singleReview ? singleWantedID : undefined,
        format: bulkAction === "import" ? singleFormat : undefined,
        move: importMode === "move",
        importMode,
        conflictAction,
        overwrite: conflictAction === "replace"
      });
      setLastBulk(outcome);
      const resolvedIDs = new Set(outcome.results.filter((result) => result.status !== "error").map((result) => result.id));
      setSelectedReviewIDs((current) => current.filter((id) => !resolvedIDs.has(id)));
      setBulkModalOpen(false);
      const message = `Bulk ${bulkAction}: ${outcome.resolved} resolved, ${outcome.imported} imported, ${outcome.skipped} skipped, ${outcome.rejected} rejected, ${plural(outcome.errored, "error")}`;
      toast.notify(message, outcome.errored ? "warn" : "success");
    } catch (error) {
      toast.error(errorText(error, "Bulk import review update failed"));
    } finally {
      setReviewActionID("");
    }
  }

  /* ------------------------------ Render helpers --------------------------- */

  const reviewsError = reviewsQuery.error ? libraryErrorMessage(reviewsQuery.error.message) : "";
  const filesError = filesQuery.error ? libraryErrorMessage(filesQuery.error.message) : "";

  const bulkFailures = lastBulk ? lastBulk.results.filter((result) => result.status === "error") : [];
  const reviewTitleByID = useMemo(() => {
    const map = new Map<string, string>();
    for (const review of reviews) {
      map.set(review.id, review.title || fileName(review.sourcePath));
    }
    return map;
  }, [reviews]);

  const visibleFiles = files.slice(0, TRACKED_FILE_ROW_CAP);
  const trackedSubtitle = lastScan
    ? `${lastScan.upserted} files indexed from ${plural(lastScan.roots.length, "root")} on the last scan.`
    : `${plural(allFiles.length, "tracked file")} from library scans and imports.`;

  return (
    <>
      <PageHeader
        title="Library Import"
        subtitle={pageSubtitle}
        actions={
          <>
            <ToolbarButton
              icon={FolderSearch}
              label="Scan Ebook Root"
              disabled={isScanning}
              busy={scanActionID === "toolbar:ebook"}
              onClick={() => runScan("toolbar:ebook", "ebook")}
            />
            <ToolbarButton
              icon={FolderSearch}
              label="Scan Audio Root"
              disabled={isScanning}
              busy={scanActionID === "toolbar:audiobook"}
              onClick={() => runScan("toolbar:audiobook", "audiobook")}
            />
            <ToolbarButton
              icon={RefreshCw}
              label="Scan All"
              disabled={isScanning}
              busy={scanActionID === "toolbar:any"}
              onClick={() => runScan("toolbar:any", "any")}
            />
            <ToolbarButton
              icon={HardDriveDownload}
              label="Import Completed Downloads"
              tone="accent"
              busy={completedMutation.isPending}
              onClick={runCompletedImport}
            />
          </>
        }
      />

      <Card
        title="Scan & import"
        subtitle="Scan a custom root or import a single file. Mode and conflict settings also apply to completed-download and review imports."
      >
        <div className="imports-scan-grid">
          <div className="imports-block">
            <span className="field-label">Custom root scan</span>
            <div className="imports-root-row">
              <Field label="Root" hint="Absolute path to scan outside the configured library roots.">
                <input
                  value={scanRoot}
                  onChange={(event) => setScanRoot(event.target.value)}
                  placeholder="/data/media/books/ebooks"
                />
              </Field>
              <div className="imports-root-buttons" role="group" aria-label="Scan custom library root">
                <Button
                  size="sm"
                  disabled={!scanRoot.trim() || isScanning}
                  busy={scanActionID === "root:ebook"}
                  onClick={() => runScan("root:ebook", "ebook", scanRoot)}
                >
                  Ebooks
                </Button>
                <Button
                  size="sm"
                  disabled={!scanRoot.trim() || isScanning}
                  busy={scanActionID === "root:audiobook"}
                  onClick={() => runScan("root:audiobook", "audiobook", scanRoot)}
                >
                  Audio
                </Button>
                <Button
                  size="sm"
                  disabled={!scanRoot.trim() || isScanning}
                  busy={scanActionID === "root:any"}
                  onClick={() => runScan("root:any", "any", scanRoot)}
                >
                  All
                </Button>
              </div>
            </div>
            {lastScan ? (
              <div className="imports-outcome" aria-label="Last scanned roots">
                <span className="field-label">{isScanning ? "Scanning" : "Last scan"}</span>
                <span className="field-hint">
                  {lastScan.upserted} indexed · {lastScan.scanned} seen · {lastScan.skipped} skipped
                </span>
                {lastScan.roots.length ? (
                  <div className="imports-chiprow">
                    {lastScan.roots.map((root) => (
                      <Badge key={root} title={root}>
                        {truncateMiddle(root, 44)}
                      </Badge>
                    ))}
                  </div>
                ) : null}
                {lastScan.errors?.length ? (
                  <span className="field-hint">
                    {plural(lastScan.errors.length, "scan error")} — {lastScan.errors[0]}
                  </span>
                ) : null}
              </div>
            ) : null}
          </div>

          <div className="imports-block">
            <span className="field-label">Manual file import</span>
            <FormGrid columns={2}>
              <Field label="Source path" hint="File is matched against wanted items during import.">
                <input
                  value={importPath}
                  onChange={(event) => setImportPath(event.target.value)}
                  placeholder="Source file path to import into the library"
                />
              </Field>
              <Field label="Format">
                <select value={importFormat} onChange={(event) => setImportFormat(event.target.value as "ebook" | "audiobook")}>
                  <option value="ebook">Ebook</option>
                  <option value="audiobook">Audiobook</option>
                </select>
              </Field>
              <Field label="Mode">
                <select value={importMode} onChange={(event) => setImportMode(event.target.value as ImportMode)}>
                  <option value="copy">Copy</option>
                  <option value="move">Move</option>
                  <option value="hardlink">Hardlink</option>
                  <option value="hardlinkOrCopy">Hardlink or copy</option>
                </select>
              </Field>
              <Field label="Conflict">
                <select value={conflictAction} onChange={(event) => setConflictAction(event.target.value as ConflictAction)}>
                  <option value="rename">Keep both</option>
                  <option value="replace">Replace</option>
                  <option value="skip">Skip</option>
                  <option value="fail">Fail</option>
                </select>
              </Field>
            </FormGrid>
            <div className="form-actions">
              <Button
                variant="primary"
                icon={UploadCloud}
                disabled={!importPath.trim()}
                busy={importMutation.isPending}
                onClick={runManualImport}
              >
                Import
              </Button>
            </div>
            {lastImport ? (
              <div className="imports-outcome">
                <span className="field-label">
                  {lastImport.skipped ? "Import skipped" : lastImport.file.title || "Imported file"}
                </span>
                <span className="field-hint" title={lastImport.destinationPath}>
                  {lastImport.message || truncateMiddle(lastImport.destinationPath, 72)}
                </span>
              </div>
            ) : null}
            {lastCompleted ? (
              <div className="imports-outcome">
                <span className="field-label">
                  Completed import: {lastCompleted.imported} imported, {lastCompleted.autoMatched} auto,{" "}
                  {lastCompleted.reviewQueued} review, {lastCompleted.skipped} skipped, {lastCompleted.errored} errors
                </span>
                <span className="field-hint">{lastCompleted.checked} downloads checked from the Librarry queue.</span>
              </div>
            ) : null}
          </div>
        </div>
      </Card>

      <Card
        title="Pending import reviews"
        subtitle="Files that need a manual wanted-item match before they can be imported."
        actions={
          <Segmented<ReviewStatusFilter>
            ariaLabel="Import review status filter"
            options={[
              { value: "pending", label: "Pending" },
              { value: "resolved", label: "Resolved" },
              { value: "all", label: "All" }
            ]}
            value={reviewStatus}
            onChange={(value) => {
              setReviewStatus(value);
              setSelectedReviewIDs([]);
            }}
          />
        }
      >
        <div className="imports-statbar-wrap">
          <StatBar
            stats={[
              { label: "Tracked", value: allFiles.length },
              {
                label: "Imported",
                value: allFiles.filter((file) => file.importStatus === "imported").length,
                tone: "success"
              },
              { label: "Review", value: pendingReviews.length, tone: pendingReviews.length ? "warn" : "neutral" },
              { label: "Ebooks", value: allFiles.filter((file) => file.mediaFormat === "ebook").length },
              { label: "Audiobooks", value: allFiles.filter((file) => file.mediaFormat === "audiobook").length },
              { label: "Scanned", value: lastScan?.scanned ?? 0 },
              { label: "Skipped", value: lastScan?.skipped ?? 0 }
            ]}
          />
        </div>

        {reviewsError ? (
          <InlineNotice tone={isPersistenceRequiredError(reviewsQuery.error?.message ?? "") ? "info" : "danger"}>
            {reviewsError}
          </InlineNotice>
        ) : null}

        {lastBulk ? (
          <InlineNotice tone={lastBulk.errored ? "warn" : "success"} onDismiss={() => setLastBulk(null)}>
            Bulk review: {lastBulk.resolved} resolved, {lastBulk.imported} imported, {lastBulk.skipped} skipped,{" "}
            {lastBulk.rejected} rejected, {lastBulk.errored} errors ({lastBulk.requested} requested).
            {bulkFailures.length
              ? ` Failures — ${bulkFailures
                  .map((failure) => `${reviewTitleByID.get(failure.id) ?? failure.id}: ${failure.message || "error"}`)
                  .join("; ")}`
              : ""}
          </InlineNotice>
        ) : null}

        {reviewsQuery.isLoading ? (
          <LoadingRow label="Loading import reviews…" />
        ) : reviews.length === 0 ? (
          <EmptyState icon={Inbox} title={reviewStatus === "pending" ? "No pending import reviews" : "No import reviews"}>
            Completed downloads that cannot be matched automatically land here for a manual decision.
          </EmptyState>
        ) : (
          <>
            <div className="imports-bulkbar" aria-label="Bulk import review actions">
              <Button
                size="sm"
                icon={allReviewsSelected ? CheckSquare : Square}
                disabled={selectableReviewIDs.length === 0 || reviewBusy}
                onClick={toggleAllReviews}
              >
                {allReviewsSelected ? "Clear all" : "Select all"}
              </Button>
              <span className="field-hint">{selectedReviews.length} selected</span>
              <Button
                size="sm"
                icon={CheckCircle2}
                disabled={selectedReviews.length === 0 || reviewBusy}
                onClick={() => {
                  setBulkAction(selectedReviewsCanImport ? "import" : "skip");
                  setBulkModalOpen(true);
                }}
              >
                Resolve selected…
              </Button>
            </div>

            <DataTable>
              <thead>
                <tr>
                  <th className="imports-review-select" aria-label="Select" />
                  <th>File</th>
                  <th>Detected</th>
                  <th>Reason</th>
                  <th>Match</th>
                  <th aria-label="Actions" />
                </tr>
              </thead>
              <tbody>
                {reviews.map((review) => {
                  const candidates = importReviewWantedCandidates(review);
                  const suggestedWanted = importReviewSuggestedWanted(review);
                  const evidenceChips = importReviewEvidenceChips(review);
                  const resolvedWantedID = importReviewResolvedWantedID(review, reviewWantedChoices);
                  const requiresWantedChoice = !resolvedWantedID && candidates.length > 0;
                  const isPending = review.status === "pending";
                  const selected = selectedReviewSet.has(review.id);
                  return (
                    <tr key={review.id} className={selected ? "selected" : undefined}>
                      <td className="imports-review-select">
                        {isPending ? (
                          <input
                            type="checkbox"
                            checked={selected}
                            onChange={() => toggleReviewSelection(review)}
                            aria-label={`Select ${review.title || review.sourcePath}`}
                          />
                        ) : null}
                      </td>
                      <td>
                        <div className="imports-review-file">
                          <span className="cell-primary">{review.title || fileName(review.sourcePath) || "Pending import"}</span>
                          <span className="cell-muted" title={review.sourcePath}>
                            {truncateMiddle(review.sourcePath, 64)}
                          </span>
                        </div>
                      </td>
                      <td>
                        <div className="imports-review-detected">
                          <span>{review.authorName || "Unknown author"}</span>
                          <span className="imports-chiprow">
                            <Badge tone={mediaFormatTone(review.mediaFormat)}>{review.mediaFormat}</Badge>
                            <span className="cell-muted">{formatBytes(review.sizeBytes ?? 0)}</span>
                          </span>
                        </div>
                      </td>
                      <td>
                        <Badge tone="warn">{review.reason}</Badge>
                      </td>
                      <td>
                        <div className="imports-review-match">
                          {suggestedWanted ? <span className="cell-muted">Suggested: {suggestedWanted}</span> : null}
                          {isPending && candidates.length ? (
                            <select
                              className="imports-candidate-select"
                              value={resolvedWantedID}
                              onChange={(event) =>
                                setReviewWantedChoices((current) => ({ ...current, [review.id]: event.target.value }))
                              }
                              disabled={Boolean(review.wantedId)}
                              aria-label={requiresWantedChoice ? "Choose wanted match" : "Wanted match"}
                            >
                              <option value="">{requiresWantedChoice ? "Select a wanted item" : "No wanted item"}</option>
                              {candidates.map((candidate) => {
                                const wantedID = stringMetadataValue(candidate.wantedId);
                                return (
                                  <option value={wantedID} key={wantedID || importReviewCandidateLabel(candidate)}>
                                    {importReviewCandidateOptionLabel(candidate)}
                                  </option>
                                );
                              })}
                            </select>
                          ) : null}
                          {evidenceChips.length ? (
                            <div className="imports-chiprow" aria-label="Import review evidence">
                              {evidenceChips.map((chip) => (
                                <Badge key={chip}>{chip}</Badge>
                              ))}
                            </div>
                          ) : null}
                        </div>
                      </td>
                      <td>
                        {isPending ? (
                          <div className="cell-actions">
                            <Button
                              size="sm"
                              icon={CheckCircle2}
                              disabled={reviewBusy || !resolvedWantedID}
                              busy={reviewActionID === `${review.id}:import`}
                              title={
                                resolvedWantedID
                                  ? "Import review into selected wanted item"
                                  : "Select a wanted item before importing"
                              }
                              onClick={() => runResolveReview(review, "import")}
                            >
                              Import
                            </Button>
                            <Button
                              size="sm"
                              variant="ghost"
                              icon={Trash2}
                              disabled={reviewBusy}
                              busy={reviewActionID === `${review.id}:skip`}
                              onClick={() => runResolveReview(review, "skip")}
                            >
                              Skip
                            </Button>
                            <Button
                              size="sm"
                              variant="danger"
                              icon={Trash2}
                              disabled={reviewBusy}
                              busy={reviewActionID === `${review.id}:reject`}
                              onClick={() => runResolveReview(review, "reject")}
                            >
                              Reject
                            </Button>
                          </div>
                        ) : (
                          <div className="cell-actions">
                            <Badge tone={importStatusTone(review.decision || review.status)}>
                              {review.decision || review.status}
                            </Badge>
                          </div>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </DataTable>
          </>
        )}
      </Card>

      <Card
        title="Tracked files"
        subtitle={trackedSubtitle}
        actions={
          <Segmented<FileFormatFilter>
            ariaLabel="Tracked file format filter"
            options={[
              { value: "any", label: "All" },
              { value: "ebook", label: "Ebooks" },
              { value: "audiobook", label: "Audiobooks" }
            ]}
            value={fileFormat}
            onChange={setFileFormat}
          />
        }
      >
        {filesError ? (
          <InlineNotice tone={isPersistenceRequiredError(filesQuery.error?.message ?? "") ? "info" : "danger"}>
            {filesError}
          </InlineNotice>
        ) : null}

        {filesQuery.isLoading ? (
          <LoadingRow label="Loading tracked files…" />
        ) : visibleFiles.length === 0 ? (
          <EmptyState
            icon={FolderInput}
            title="No tracked files"
            actions={
              <Button
                icon={FolderSearch}
                disabled={isScanning}
                busy={scanActionID === "empty:any"}
                onClick={() => runScan("empty:any", "any")}
              >
                Scan all roots
              </Button>
            }
          >
            Run a library scan or import a completed download to start tracking files.
          </EmptyState>
        ) : (
          <>
            <DataTable>
              <thead>
                <tr>
                  <th>File</th>
                  <th>Author</th>
                  <th>Format</th>
                  <th>Status</th>
                  <th>Size</th>
                  <th>Type</th>
                  <th>Wanted</th>
                </tr>
              </thead>
              <tbody>
                {visibleFiles.map((file) => {
                  const wantedID =
                    stringMetadataValue(file.metadata?.wantedId) || stringMetadataValue(file.metadata?.librarryWantedId);
                  return (
                    <tr key={file.id || file.path}>
                      <td>
                        <div className="imports-review-file">
                          <span className="cell-primary">{file.title || fileName(file.path)}</span>
                          <span className="cell-muted" title={file.path}>
                            {truncateMiddle(file.path, 64)}
                          </span>
                        </div>
                      </td>
                      <td>{file.authorName || "Unknown author"}</td>
                      <td>
                        <Badge tone={mediaFormatTone(file.mediaFormat)}>{file.mediaFormat}</Badge>
                      </td>
                      <td>
                        <Badge tone={importStatusTone(file.importStatus)}>{file.importStatus || "available"}</Badge>
                      </td>
                      <td>{formatBytes(file.sizeBytes ?? 0)}</td>
                      <td>
                        <span className="cell-muted">{file.extension || "file"}</span>
                      </td>
                      <td>
                        {wantedID ? (
                          <Link to="/wanted" title={`Bound to wanted item ${wantedID}`}>
                            <FileCheck2 size={13} aria-hidden /> Wanted
                          </Link>
                        ) : (
                          <span className="cell-muted">—</span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </DataTable>
            <span className="field-hint imports-table-caption">
              Showing {visibleFiles.length} of {files.length} tracked files
              {files.length >= TRACKED_FILE_ROW_CAP ? " (most recent first, capped at 100)" : ""}.
            </span>
          </>
        )}
      </Card>

      <Modal
        title="Resolve selected reviews"
        open={bulkModalOpen}
        onClose={() => setBulkModalOpen(false)}
        footer={
          <>
            <Button variant="ghost" onClick={() => setBulkModalOpen(false)} disabled={reviewBusy}>
              Cancel
            </Button>
            <Button
              variant={bulkAction === "import" ? "primary" : "danger"}
              icon={bulkAction === "import" ? CheckCircle2 : Trash2}
              disabled={selectedReviews.length === 0 || (bulkAction === "import" && !selectedReviewsCanImport)}
              busy={reviewActionID === `bulk:${bulkAction}`}
              title={
                bulkAction === "import" && !selectedReviewsCanImport
                  ? "Select a wanted item for each review before bulk import"
                  : undefined
              }
              onClick={runBulkResolve}
            >
              {bulkAction === "import" ? "Import" : bulkAction === "skip" ? "Skip" : "Reject"}{" "}
              {plural(selectedReviews.length, "review")}
            </Button>
          </>
        }
      >
        <p>
          Apply one decision to the {plural(selectedReviews.length, "selected review")}. Imports use the current mode (
          {importMode}) and conflict ({conflictAction === "rename" ? "keep both" : conflictAction}) settings.
        </p>
        <Segmented<ReviewAction>
          ariaLabel="Bulk review action"
          options={[
            { value: "import", label: "Import" },
            { value: "skip", label: "Skip" },
            { value: "reject", label: "Reject" }
          ]}
          value={bulkAction}
          onChange={setBulkAction}
        />
        {bulkAction === "import" && !selectedReviewsCanImport ? (
          <InlineNotice tone="warn">
            Every selected review needs a wanted match before bulk import. Pick one in each row&apos;s candidate select.
          </InlineNotice>
        ) : null}
      </Modal>
    </>
  );
}
