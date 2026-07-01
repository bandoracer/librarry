import type { ImportReview } from "../../lib/api";

/*
 * Helpers ported from the legacy single-file App.tsx (import review metadata
 * readers around line 6606+). Pure functions over ImportReview metadata.
 */

export type WantedCandidate = Record<string, unknown>;

export function stringMetadataValue(value: unknown): string {
  if (typeof value === "string") return value.trim();
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return "";
}

export function importReviewWantedCandidates(review: ImportReview): WantedCandidate[] {
  const candidates = review.metadata?.wantedCandidates;
  if (!Array.isArray(candidates)) return [];
  return candidates.filter((item): item is WantedCandidate => Boolean(item) && typeof item === "object");
}

export function importReviewMatchedFieldLabel(field: string): string {
  switch (field) {
    case "isbn":
      return "ISBN";
    case "author":
      return "author";
    case "format":
      return "format";
    case "title":
      return "title";
    default:
      return field;
  }
}

export function importReviewCandidateMatchedFields(candidate: WantedCandidate): string[] {
  const fields = candidate?.matchedFields;
  if (!Array.isArray(fields)) return [];
  return Array.from(
    new Set(
      fields
        .map((field) => importReviewMatchedFieldLabel(stringMetadataValue(field)))
        .filter(Boolean)
    )
  );
}

export function importReviewCandidateLabel(candidate: WantedCandidate): string {
  const wantedID = stringMetadataValue(candidate.wantedId);
  const title = stringMetadataValue(candidate.title);
  const authorName = stringMetadataValue(candidate.authorName);
  if (title && authorName) return `${title} by ${authorName}`;
  return title || authorName || wantedID || "Wanted item";
}

export function importReviewCandidateOptionLabel(candidate: WantedCandidate): string {
  const label = importReviewCandidateLabel(candidate);
  const matched = importReviewCandidateMatchedFields(candidate);
  const score = Number(stringMetadataValue(candidate.score));
  const scoreLabel = Number.isFinite(score) && score > 0 ? score.toFixed(2) : "";
  const suffix = [matched.length ? matched.join("/") : "", scoreLabel].filter(Boolean).join(" · ");
  return suffix ? `${label} (${suffix})` : label;
}

export function importReviewSuggestedCandidate(review: ImportReview): WantedCandidate | undefined {
  const candidates = importReviewWantedCandidates(review);
  const suggestedID = stringMetadataValue(review.metadata?.suggestedWantedId) || review.wantedId;
  if (!suggestedID) return undefined;
  return candidates.find((item) => stringMetadataValue(item.wantedId) === suggestedID);
}

export function importReviewSuggestedWanted(review: ImportReview): string {
  const candidate = importReviewSuggestedCandidate(review);
  if (!candidate) return "";
  return importReviewCandidateLabel(candidate);
}

export function importReviewSuggestedWantedMatchedFields(review: ImportReview): string[] {
  const candidate = importReviewSuggestedCandidate(review);
  return candidate ? importReviewCandidateMatchedFields(candidate) : [];
}

/** wantedId already bound to the review > user selection > server suggestion. */
export function importReviewResolvedWantedID(review: ImportReview, selections: Record<string, string>): string {
  return review.wantedId || selections[review.id] || stringMetadataValue(review.metadata?.suggestedWantedId);
}

export function importReviewResolvedFormat(review: ImportReview, wantedID: string, fallbackFormat: string): string {
  const candidate = importReviewWantedCandidates(review).find((item) => stringMetadataValue(item.wantedId) === wantedID);
  const candidateFormat = stringMetadataValue(candidate?.format);
  return candidateFormat || (fallbackFormat === "any" ? "ebook" : fallbackFormat);
}

export function importReviewEvidenceChips(review: ImportReview): string[] {
  const chips: string[] = [];
  const confidence = stringMetadataValue(review.metadata?.matchConfidence);
  if (confidence) {
    chips.push(`Confidence ${confidence}`);
  }
  const suggestedWanted = importReviewSuggestedWanted(review);
  if (suggestedWanted) {
    chips.push(`Suggested ${suggestedWanted}`);
  }
  const matchedFields = importReviewSuggestedWantedMatchedFields(review);
  if (matchedFields.length) {
    chips.push(`Matched ${matchedFields.join(", ")}`);
  }
  const evidence = review.metadata?.reviewEvidence;
  if (Array.isArray(evidence)) {
    evidence.slice(0, 3).forEach((item) => {
      if (!item || typeof item !== "object") return;
      const payload = item as Record<string, unknown>;
      const label = stringMetadataValue(payload.label) || stringMetadataValue(payload.source);
      const value = stringMetadataValue(payload.value);
      if (label && value) {
        chips.push(`${label}: ${value}`);
      } else if (label) {
        chips.push(label);
      }
    });
  }
  return Array.from(new Set(chips)).slice(0, 4);
}

export function isPersistenceRequiredError(message: string): boolean {
  return message.toLowerCase().includes("requires database persistence");
}

/** Legacy appErrorMessage: decorate persistence errors with the fix. */
export function libraryErrorMessage(message: string): string {
  if (!isPersistenceRequiredError(message)) return message;
  return `${message}. Set LIBRARRY_DATABASE_URL to a Postgres database and restart the API to enable library files, import reviews, wanted queues, author monitoring, and release decisions.`;
}

export function errorText(error: unknown, fallback: string): string {
  const message = error instanceof Error && error.message ? error.message : fallback;
  return libraryErrorMessage(message);
}

export function fileName(path: string): string {
  return path.split("/").pop() || path;
}
