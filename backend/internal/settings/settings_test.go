package settings

import "testing"

func TestValidateProwlarrPair(t *testing.T) {
	result := Validate(Settings{ProwlarrURL: "http://prowlarr.local"})
	if result.Valid {
		t.Fatalf("expected invalid settings when prowlarr api key is missing")
	}
}

func TestValidateAllowsMissingOptionalProvidersWithWarnings(t *testing.T) {
	result := Validate(Settings{})
	if !result.Valid {
		t.Fatalf("expected missing optional providers to be valid with warnings: %#v", result)
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected warnings")
	}
}
