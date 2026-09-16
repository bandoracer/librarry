package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestEverySettingHasAnEnvironmentBinding(t *testing.T) {
	schema := reflect.TypeOf(Config{})
	seen := map[string]bool{}
	for i := 0; i < schema.NumField(); i++ {
		field := schema.Field(i)
		key := field.Tag.Get("env")
		if !strings.HasPrefix(key, "LIBRARRY_") || seen[key] {
			t.Fatalf("invalid or duplicate binding for %s: %q", field.Name, key)
		}
		seen[key] = true
	}
}

func TestInvalidAutomationEnvironmentFailsBeforeDefaultsApply(t *testing.T) {
	for _, tc := range []struct{ key, value string }{
		{"LIBRARRY_MONITOR_AUTO_GRAB", "flase"},
		{"LIBRARRY_COMPLETED_REMOVE_ENABLED", "disabled"},
		{"LIBRARRY_MONITOR_INTERVAL", "thirty"},
		{"LIBRARRY_MONITOR_INTERVAL", "0s"},
		{"LIBRARRY_MONITOR_LIMIT", "many"},
		{"LIBRARRY_UPGRADE_SEARCH_MIN_DELTA", "NaN"},
		{"LIBRARRY_COMPLETED_IMPORT_MODE", "move"},
	} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			err := ValidateEnvironment()
			if err == nil || !strings.Contains(err.Error(), tc.key) || strings.Contains(err.Error(), tc.value) {
				t.Fatalf("expected redacted validation error for %s: %v", tc.key, err)
			}
		})
	}
	t.Setenv("LIBRARRY_MONITOR_AUTO_GRAB", "false")
	t.Setenv("LIBRARRY_MONITOR_INTERVAL", "30")
	t.Setenv("LIBRARRY_MONITOR_LIMIT", "50")
	t.Setenv("LIBRARRY_UPGRADE_SEARCH_MIN_DELTA", "5.5")
	t.Setenv("LIBRARRY_COMPLETED_IMPORT_MODE", "hardlinkOrCopy")
	if err := ValidateEnvironment(); err != nil {
		t.Fatal(err)
	}
	if FromEnv().MonitorAutoGrab {
		t.Fatal("explicit false was ignored")
	}
}

func TestImportListSchedulingHasItsOwnEnableFlag(t *testing.T) {
	t.Setenv("LIBRARRY_FEED_SYNC_ENABLED", "false")
	t.Setenv("LIBRARRY_IMPORT_LIST_SYNC_ENABLED", "")
	if !FromEnv().ImportListSyncEnabled {
		t.Fatal("import lists must remain enabled by default independently of feed sync")
	}
	t.Setenv("LIBRARRY_IMPORT_LIST_SYNC_ENABLED", "false")
	if FromEnv().ImportListSyncEnabled {
		t.Fatal("import-list disable flag ignored")
	}
	t.Setenv("LIBRARRY_IMPORT_LIST_SYNC_ENABLED", "flase")
	if err := ValidateEnvironment(); err == nil {
		t.Fatal("invalid import-list flag accepted")
	}
}
