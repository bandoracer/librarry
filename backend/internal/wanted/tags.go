package wanted

import (
	"hash/fnv"
	"strconv"
	"strings"
)

// Tags on wanted items and author subscriptions are stored as comma-separated
// labels (the 0016 text format). Labels are normalized to lowercase so the
// native tags table (M6.4) can rewrite them exactly on rename/delete.

// NormalizeTagLabel trims and lowercases a tag label.
func NormalizeTagLabel(label string) string {
	return strings.ToLower(strings.TrimSpace(label))
}

// compactTagLabels normalizes, dedupes, and drops empty labels.
func compactTagLabels(labels []string) []string {
	if len(labels) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		label = NormalizeTagLabel(label)
		if label == "" || seen[label] {
			continue
		}
		seen[label] = true
		out = append(out, label)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func tagLabelsString(labels []string) string {
	return strings.Join(compactTagLabels(labels), ",")
}

func splitTagLabels(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '|' || r == '\t' || r == '\n' || r == '\r'
	})
	return compactTagLabels(parts)
}

// TagLabelHash mirrors the compat layer's stable FNV-1a int ids so label
// tags can round-trip through Readarr-compatible integer tag arrays.
func TagLabelHash(label string) int {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(NormalizeTagLabel(label)))
	return int(hash.Sum32() & 0x7fffffff)
}

// releaseRestrictionAppliesToLabels reports whether a compat restriction
// (which carries integer tag ids) applies to an item's label tags: it matches
// when a label's stable hash — or a legacy numeric label — equals one of the
// restriction's tag ids.
func releaseRestrictionAppliesToLabels(restriction ReleaseRestriction, labels []string) bool {
	if len(restriction.Tags) == 0 {
		return true
	}
	labels = compactTagLabels(labels)
	if len(labels) == 0 {
		return false
	}
	itemTagSet := make(map[int]bool, len(labels)*2)
	for _, label := range labels {
		itemTagSet[TagLabelHash(label)] = true
		if numeric, err := strconv.Atoi(label); err == nil {
			itemTagSet[numeric] = true
		}
	}
	for _, tag := range restriction.Tags {
		if itemTagSet[tag] {
			return true
		}
	}
	return false
}
