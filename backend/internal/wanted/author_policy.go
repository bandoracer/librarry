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
	type datedCandidate struct {
		key        string
		start, end time.Time
	}
	dated := []datedCandidate{}
	released := []datedCandidate{}
	unknownDate, uncertainReleased := false, false
	tomorrow := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day()+1, 0, 0, 0, 0, time.UTC)
	for _, candidate := range candidates {
		key := authorMetadataReviewCandidateKey(candidate)
		if sourceKeysWithFiles != nil {
			fileKey := wantedSourceFileKey(candidate.Provider, candidateSourceKey(candidate), subscription.Format)
			policyCtx.hasFile[key] = sourceKeysWithFiles[fileKey]
		}
		published, hasDate := resultPublicationDate(candidate)
		if !hasDate {
			unknownDate = true
			continue
		}
		entry := datedCandidate{key: key, start: published.Time, end: publicationEnd(published)}
		dated = append(dated, entry)
		if !entry.end.After(tomorrow) {
			released = append(released, entry)
		} else if entry.start.Before(tomorrow) {
			uncertainReleased = true
		}
	}
	// Partial dates represent intervals. Overlap and unknown dates cannot prove
	// a first/latest work; discovery order must not decide automatic acquisition.
	if !unknownDate && len(dated) > 0 {
		first := dated[0]
		for _, candidate := range dated {
			if candidate.start.Before(first.start) {
				first = candidate
			}
		}
		unique := true
		for _, other := range dated {
			if first.key != other.key && first.end.After(other.start) {
				unique = false
				break
			}
		}
		if unique {
			policyCtx.firstKey = first.key
		}
		if !uncertainReleased && len(released) > 0 {
			latest := released[0]
			for _, candidate := range released {
				if candidate.start.After(latest.start) {
					latest = candidate
				}
			}
			unique = true
			for _, other := range released {
				if latest.key != other.key && other.end.After(latest.start) {
					unique = false
					break
				}
			}
			if unique {
				policyCtx.latestKey = latest.key
			}
		}
	}
	return policyCtx
}

// authorResultAllowedByPolicy implements the monitor-mode selection semantics:
// all (everything), future (published after subscription), none (nothing),
// missing (books without library files), existing (books with files or future
// releases), first (only the unambiguously earliest dated book), latest (the most
// recent published book plus future releases).
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
		if policyCtx.firstKey == "" {
			return false, "first policy cannot establish publication order from missing or overlapping dates"
		}
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
		if policyCtx.latestKey == "" {
			return false, "latest policy cannot establish the most recent published book from missing or overlapping dates"
		}
		return false, "latest policy only monitors the most recent published or future books"
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
	cutoff = time.Date(cutoff.UTC().Year(), cutoff.UTC().Month(), cutoff.UTC().Day(), 0, 0, 0, 0, time.UTC)
	if !published.Time.Before(cutoff) {
		return true, ""
	}
	if publicationEnd(published).After(cutoff) {
		return false, "publication date is too imprecise to establish the subscription cutoff"
	}
	return false, "published before the author subscription cutoff"
}

func publicationEnd(published publicationDate) time.Time {
	switch published.Precision {
	case "year":
		return published.Time.AddDate(1, 0, 0)
	case "month":
		return published.Time.AddDate(0, 1, 0)
	default:
		return published.Time.AddDate(0, 0, 1)
	}
}

func candidateSourceKey(result metadata.SearchResult) string {
	return firstNonEmpty(result.Edition.ID, result.Work.ID, result.RawSourceKey)
}

func wantedSourceFileKey(provider string, sourceKey string, format string) string {
	return strings.ToLower(strings.TrimSpace(provider)) + "|" +
		strings.ToLower(strings.TrimSpace(sourceKey)) + "|" +
		normalizeFormat(format)
}

// SourceIdentity is the normalized provider identity key of a wanted item
// (provider|sourceKey|format); import-list sync uses it to dedupe entries
// against already-tracked books.
func SourceIdentity(provider string, sourceKey string, format string) string {
	return wantedSourceFileKey(provider, sourceKey, format)
}
