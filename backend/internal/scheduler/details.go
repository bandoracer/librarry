package scheduler

import (
	"context"
	"regexp"
	"sync"
)

// RunDetails contains bounded structured diagnostics, not arbitrary provider
// payloads. Counts are nonnegative; operation IDs must be UUIDs.
type RunDetails struct {
	Counts       map[string]int `json:"counts,omitempty"`
	OperationIDs []string       `json:"operationIds,omitempty"`
	Errors       int            `json:"errors,omitempty"`
	NextAction   string         `json:"nextAction,omitempty"`
}
type detailsKey struct{}
type detailsCollector struct {
	mu    sync.Mutex
	value RunDetails
}

var diagnosticKey = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]{0,31}$`)
var diagnosticID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// RecordRunDetails replaces the current summary for this task invocation. It is
// safe for a task to report before returning an error; the failure keeps it.
func RecordRunDetails(ctx context.Context, details RunDetails) {
	collector, _ := ctx.Value(detailsKey{}).(*detailsCollector)
	if collector == nil {
		return
	}
	details = normalizeDetails(details)
	collector.mu.Lock()
	collector.value = details
	collector.mu.Unlock()
}
func normalizeDetails(details RunDetails) RunDetails {
	result := RunDetails{Errors: max(0, details.Errors), NextAction: details.NextAction, Counts: map[string]int{}}
	if len(result.NextAction) > 500 {
		result.NextAction = result.NextAction[:500]
	}
	for k, v := range details.Counts {
		if len(result.Counts) < 32 && diagnosticKey.MatchString(k) && v >= 0 {
			result.Counts[k] = v
		}
	}
	seen := map[string]bool{}
	for _, id := range details.OperationIDs {
		if len(result.OperationIDs) < 100 && diagnosticID.MatchString(id) && !seen[id] {
			result.OperationIDs = append(result.OperationIDs, id)
			seen[id] = true
		}
	}
	return result
}
func (c *detailsCollector) snapshot() RunDetails {
	c.mu.Lock()
	defer c.mu.Unlock()
	return normalizeDetails(c.value)
}
