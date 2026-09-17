package wanted

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/bandoracer/librarry/backend/internal/metadata"
)

type authorExclusions struct {
	sourceKeys map[string]bool
	titles     map[string][]string
	ignored    map[string]map[string]bool
}

func exclusionText(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func reviewScope(subscriptionID, format string) string {
	return subscriptionID + "|" + normalizeFormat(format)
}
func exclusionWorkKeys(result metadata.SearchResult) []string {
	keys := []string{}
	for _, alias := range workProviderAliases(result) {
		keys = append(keys, "work:"+exclusionText(alias.Provider)+"|"+exclusionText(alias.Key))
	}
	return keys
}
func (s *Store) authorExclusionSnapshot(ctx context.Context, subscriptions []AuthorSubscription) (authorExclusions, error) {
	snapshot := authorExclusions{sourceKeys: map[string]bool{}, titles: map[string][]string{}, ignored: map[string]map[string]bool{}}
	rows, err := s.db.QueryContext(ctx, `select source_key,title,author_name from import_list_exclusions`)
	if err != nil {
		return snapshot, err
	}
	for rows.Next() {
		var source, title, author string
		if err := rows.Scan(&source, &title, &author); err != nil {
			rows.Close()
			return snapshot, err
		}
		if source != "" {
			snapshot.sourceKeys[exclusionText(source)] = true
		}
		if title != "" {
			key := exclusionText(title)
			snapshot.titles[key] = append(snapshot.titles[key], exclusionText(author))
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return snapshot, err
	}
	if len(subscriptions) == 0 {
		return snapshot, nil
	}
	ids := make([]string, 0, len(subscriptions))
	for _, sub := range subscriptions {
		ids = append(ids, sub.ID)
	}
	rows, err = s.db.QueryContext(ctx, `select author_subscription_id::text,wanted_format,candidate_key,result from author_metadata_reviews where status='ignored' and author_subscription_id=any($1::uuid[])`, ids)
	if err != nil {
		return snapshot, err
	}
	defer rows.Close()
	for rows.Next() {
		var subscriptionID, format, key string
		var raw []byte
		if err := rows.Scan(&subscriptionID, &format, &key, &raw); err != nil {
			return snapshot, err
		}
		var result metadata.SearchResult
		if err := json.Unmarshal(raw, &result); err != nil {
			return snapshot, err
		}
		scope := reviewScope(subscriptionID, format)
		if snapshot.ignored[scope] == nil {
			snapshot.ignored[scope] = map[string]bool{}
		}
		snapshot.ignored[scope]["candidate:"+exclusionText(key)] = true
		for _, key := range exclusionWorkKeys(result) {
			snapshot.ignored[scope][key] = true
		}
	}
	return snapshot, rows.Err()
}
func (e authorExclusions) reason(subscription AuthorSubscription, result metadata.SearchResult) string {
	ignored := e.ignored[reviewScope(subscription.ID, subscription.Format)]
	if ignored["candidate:"+exclusionText(authorMetadataReviewCandidateKey(result))] {
		return "ignored by an earlier author review"
	}
	for _, key := range exclusionWorkKeys(result) {
		if ignored[key] {
			return "ignored by an earlier author review"
		}
	}
	keys := append([]string{result.Work.ID, result.Edition.ID}, result.Work.ProviderIDs...)
	keys = append(keys, result.Edition.ProviderIDs...)
	for _, key := range keys {
		if key != "" && e.sourceKeys[exclusionText(key)] {
			return "excluded by a saved book source identity"
		}
	}
	for _, title := range []string{result.Work.Title, result.Edition.Title} {
		for _, author := range e.titles[exclusionText(title)] {
			if author == "" {
				return "excluded by a saved book title"
			}
			for _, credit := range result.Work.Authors {
				if exclusionText(credit.Name) == author {
					return "excluded by a saved book title and author"
				}
			}
		}
	}
	return ""
}
