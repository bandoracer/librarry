package compat

import (
	"context"
	"strings"
	"testing"
)

func TestReleaseRestrictionProviderMapsCompatPayloads(t *testing.T) {
	provider := NewReleaseRestrictionProvider(fakeResourceLister{resources: []Resource{{
		CompatID: 3,
		Payload: map[string]any{
			"mustContain":    "retail; epub",
			"mustNotContain": "screener",
			"preferredTerms": []any{"proper", "repack"},
			"tags":           []any{float64(5), "7", 0},
		},
	}}})

	restrictions, err := provider.ListReleaseRestrictions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(restrictions) != 1 {
		t.Fatalf("expected one restriction, got %#v", restrictions)
	}
	restriction := restrictions[0]
	if restriction.ID != "3" {
		t.Fatalf("expected compat id, got %q", restriction.ID)
	}
	if strings.Join(restriction.RequiredTerms, "|") != "retail|epub" {
		t.Fatalf("unexpected required terms: %#v", restriction.RequiredTerms)
	}
	if strings.Join(restriction.IgnoredTerms, "|") != "screener" {
		t.Fatalf("unexpected ignored terms: %#v", restriction.IgnoredTerms)
	}
	if strings.Join(restriction.PreferredTerms, "|") != "proper|repack" {
		t.Fatalf("unexpected preferred terms: %#v", restriction.PreferredTerms)
	}
	if len(restriction.Tags) != 2 || restriction.Tags[0] != 5 || restriction.Tags[1] != 7 {
		t.Fatalf("unexpected tags: %#v", restriction.Tags)
	}
}

type fakeResourceLister struct {
	resources []Resource
}

func (f fakeResourceLister) ListResources(_ context.Context, resourceType string) ([]Resource, error) {
	if resourceType != "restriction" {
		return nil, nil
	}
	return f.resources, nil
}
