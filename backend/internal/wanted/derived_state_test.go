package wanted

import "testing"

func TestDeriveWantedState(t *testing.T) {
	monitored := WantedItem{Monitored: true, Status: "grabbed"}
	unmonitored := WantedItem{Monitored: false, Status: "wanted"}

	cases := []struct {
		name        string
		item        WantedItem
		hasFile     bool
		cutoffUnmet bool
		downloading bool
		expect      string
	}{
		{"file meets cutoff", monitored, true, false, false, DerivedStateDownloaded},
		{"file below cutoff", monitored, true, true, false, DerivedStateCutoffUnmet},
		{"file wins over live download", monitored, true, false, true, DerivedStateDownloaded},
		{"live download", monitored, false, false, true, DerivedStateDownloading},
		{"stranded grab derives missing", monitored, false, false, false, DerivedStateMissing},
		{"unmonitored without file", unmonitored, false, false, false, DerivedStateUnmonitored},
		{"unmonitored but downloading", unmonitored, false, false, true, DerivedStateDownloading},
	}
	for _, testCase := range cases {
		if got := deriveWantedState(testCase.item, testCase.hasFile, testCase.cutoffUnmet, testCase.downloading); got != testCase.expect {
			t.Fatalf("%s: expected %q, got %q", testCase.name, testCase.expect, got)
		}
	}
}

func TestAnnotateWantedStatesWithoutPersistenceIsNoOp(t *testing.T) {
	items := []WantedItem{{ID: "a", Monitored: true}}
	annotated := NewService(nil, nil).AnnotateWantedStates(nil, items)
	if annotated[0].DerivedState != "" {
		t.Fatalf("expected pass-through without database, got %+v", annotated[0])
	}
}
