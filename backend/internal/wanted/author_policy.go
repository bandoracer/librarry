package wanted

import (
	"context"
	"strings"
	"time"

	"github.com/bandoracer/librarry/backend/internal/metadata"
)

// authorPolicyContext carries the batch-level facts an author missing-book
// policy needs: which candidates already have library files and which
// candidate is the earliest/most recent publication in the sync batch.
type authorPolicyContext struct {
	now       time.Time
	hasFile   map[string]bool
	firstKey  string
	latestKey string
}

// authorPolicyContext builds the policy context for one subscription sync.
// The library-file lookup only runs for policies that need it.
func (s *Service) authorPolicyContext(ctx context.Context, subscription AuthorSubscription, candidates []metadata.SearchResult, now time.Time) (authorPolicyContext, error) {
	policy := normalizeAuthorMissingBookPolicy(subscription.MissingBookPolicy, subscription.MonitorNewItems)
	var fileKeys map[string]bool
	var err error
	if policy == "missing" || policy == "existing" {
		fileKeys, err = s.store.WantedSourceKeysWithFiles(ctx)
		if err != nil {
			fileKeys = nil
		}
	}
	return buildAuthorPolicyContext(subscription, candidates, fileKeys, now), err
}

func buildAuthorPolicyContext(subscription AuthorSubscription, candidates []metadata.SearchResult, sourceKeysWithFiles map[string]bool, now time.Time) authorPolicyContext {
	policyCtx := authorPolicyContext{
		now:     now.UTC(),
		hasFile: map[string]bool{},
	}
	var firstDate, latestDate time.Time
	var firstDated, latestDated bool
	for index, candidate := range candidates {
		key := authorMetadataReviewCandidateKey(candidate)
		if sourceKeysWithFiles != nil {
			fileKey := wantedSourceFileKey(candidate.Provider, candidateSourceKey(candidate), subscription.Format)
			policyCtx.hasFile[key] = sourceKeysWithFiles[fileKey]
		}
		published, hasDate := resultPublicationDate(candidate)
		if hasDate {
			if !firstDated || published.Time.Before(firstDate) {
				firstDated = true
				firstDate = published.Time
				policyCtx.firstKey = key
			}
			if !latestDated || published.Time.After(latestDate) {
				latestDated = true
				latestDate = published.Time
				policyCtx.latestKey = key
			}
			continue
		}
		// Fall back to discovery order when no candidate carries a date.
		if !firstDated && policyCtx.firstKey == "" {
			policyCtx.firstKey = key
		}
		if !latestDated && index == len(candidates)-1 {
			policyCtx.latestKey = key
		}
	}
	return policyCtx
}

// authorResultAllowedByPolicy implements the monitor-mode selection semantics:
// all (everything), future (published after subscription), none (nothing),
// missing (books without library files), existing (books with files or future
// releases), first (only the earliest discovered book), latest (the most
// recent book plus future releases).
func authorResultAllowedByPolicy(subscription AuthorSubscription, result metadata.SearchResult, policyCtx authorPolicyContext) (bool, string) {
	key := authorMetadataReviewCandidateKey(result)
	switch normalizeAuthorMissingBookPolicy(subscription.MissingBookPolicy, subscription.MonitorNewItems) {
	case "none":
		return false, "author policy is set to none"
	case "future":
		return authorResultIsFuture(subscription, result, policyCtx.now)
	case "missing":
		if policyCtx.hasFile[key] {
			return false, "missing policy skips books that already have a library file"
		}
		return true, ""
	case "existing":
		if policyCtx.hasFile[key] {
			return true, ""
		}
		if future, _ := authorResultIsFuture(subscription, result, policyCtx.now); future {
			return true, ""
		}
		return false, "existing policy requires a library file or a future publication"
	case "first":
		if key == policyCtx.firstKey {
			return true, ""
		}
		return false, "first policy only monitors the earliest discovered book"
	case "latest":
		if key == policyCtx.latestKey {
			return true, ""
		}
		if future, _ := authorResultIsFuture(subscription, result, policyCtx.now); future {
			return true, ""
		}
		return false, "latest policy only monitors the most recent or future books"
	default:
		return true, ""
	}
}

func authorResultIsFuture(subscription AuthorSubscription, result metadata.SearchResult, now time.Time) (bool, string) {
	published, ok := resultPublicationDate(result)
	if !ok {
		return false, "future policy requires a publication date"
	}
	cutoff := subscription.CreatedAt
	if cutoff.IsZero() {
		cutoff = now
	}
	switch published.Precision {
	case "year":
		if published.Time.Year() >= cutoff.Year() {
			return true, ""
		}
	case "month":
		publishedMonth := time.Date(published.Time.Year(), published.Time.Month(), 1, 0, 0, 0, 0, time.UTC)
		cutoffMonth := time.Date(cutoff.UTC().Year(), cutoff.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
		if !publishedMonth.Before(cutoffMonth) {
			return true, ""
		}
	default:
		publishedDay := time.Date(published.Time.Year(), published.Time.Month(), published.Time.Day(), 0, 0, 0, 0, time.UTC)
		cutoffDay := time.Date(cutoff.UTC().Year(), cutoff.UTC().Month(), cutoff.UTC().Day(), 0, 0, 0, 0, time.UTC)
		if !publishedDay.Before(cutoffDay) {
			return true, ""
		}
	}
	return false, "published before the author subscription cutoff"
}

func candidateSourceKey(result metadata.SearchResult) string {
	return firstNonEmpty(result.Edition.ID, result.Work.ID, result.RawSourceKey)
}

func wantedSourceFileKey(provider string, sourceKey string, format string) string {
	return strings.ToLower(strings.TrimSpace(provider)) + "|" +
		strings.ToLower(strings.TrimSpace(sourceKey)) + "|" +
		normalizeFormat(format)
}
