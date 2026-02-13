package gam

import (
	"encoding/json"
	"testing"
)

func TestConceptSpecSerialization(t *testing.T) {
	spec := ConceptSpec{
		TypeParams: []string{"S"},
		State: map[string]StateComponent{
			"sources": {Type: "set", Of: "S"},
			"name":    {Type: "map", From: "S", To: "string"},
			"enabled": {Type: "map", From: "S", To: "boolean"},
		},
		Actions: map[string]ActionSpec{
			"register": {
				Cases: []ActionCase{
					{
						Input:       map[string]string{"source": "S", "name": "string"},
						Output:      map[string]string{"source": "S"},
						Description: "add source to sources, set enabled true",
					},
					{
						Input:       map[string]string{"source": "S", "name": "string"},
						Output:      map[string]string{"error": "string"},
						Description: "if name not unique",
					},
				},
			},
		},
		OperationalPrinciple: "after register => query succeeds",
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded ConceptSpec
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(decoded.TypeParams) != 1 || decoded.TypeParams[0] != "S" {
		t.Errorf("type params mismatch: %v", decoded.TypeParams)
	}
	if len(decoded.State) != 3 {
		t.Errorf("state components count: want 3, got %d", len(decoded.State))
	}
	if len(decoded.Actions["register"].Cases) != 2 {
		t.Errorf("action cases count: want 2, got %d", len(decoded.Actions["register"].Cases))
	}
}

func TestSynchronizationSerialization(t *testing.T) {
	sync := Synchronization{
		Name: "FanOutSearch",
		WhenClause: []WhenPattern{
			{
				Concept:     "Web",
				Action:      "request",
				InputMatch:  map[string]string{"method": "search", "terms": "?terms"},
				OutputMatch: map[string]string{"request": "?request"},
			},
		},
		WhereClause: []WherePattern{
			{
				Concept: "SearchSource",
				Pattern: map[string]any{"?s": map[string]any{"enabled": true}},
			},
		},
		ThenClause: []ThenAction{
			{
				Concept: "SearchSource",
				Action:  "query",
				Args:    map[string]string{"source": "?s", "terms": "?terms"},
			},
		},
		Description: "Fan out search to all enabled sources",
		Enabled:     true,
	}

	data, err := json.Marshal(sync)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded Synchronization
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Name != "FanOutSearch" {
		t.Errorf("name mismatch: %s", decoded.Name)
	}
	if len(decoded.WhenClause) != 1 {
		t.Errorf("when clause count: want 1, got %d", len(decoded.WhenClause))
	}
	if decoded.WhenClause[0].Concept != "Web" {
		t.Errorf("when concept: want Web, got %s", decoded.WhenClause[0].Concept)
	}
	if len(decoded.ThenClause) != 1 || decoded.ThenClause[0].Action != "query" {
		t.Error("then clause mismatch")
	}
}

func TestProposalSerialization(t *testing.T) {
	proposal := Proposal{
		RegionPath:  "app.search.sources",
		ActionTaken: "implement",
		Evidence: ProposalEvidence{
			APIAnalysis: &APIAnalysis{
				ExportsBefore: []string{"Query"},
				ExportsAfter:  []string{"Query", "HealthCheck"},
				Additions:     []string{"HealthCheck"},
			},
			ModifiedRegions: []ModifiedRegion{
				{Path: "app.search.sources.btv2", File: "search/btv2.go"},
			},
			Summary: "Added health check to btv2 source",
		},
		Status: "PENDING",
	}

	data, err := json.Marshal(proposal)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded Proposal
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Evidence.APIAnalysis == nil {
		t.Fatal("api analysis should not be nil")
	}
	if len(decoded.Evidence.APIAnalysis.Additions) != 1 {
		t.Errorf("additions count: want 1, got %d", len(decoded.Evidence.APIAnalysis.Additions))
	}
}

func TestValidationResultFormat(t *testing.T) {
	result := ValidationResult{
		Tier:   0,
		Passed: false,
		Code:   1,
		Message: "Region app.missing not found",
		Details: []ValidationDetail{
			{
				Check:    "region_exists",
				Passed:   false,
				Expected: "region exists",
				Got:      "not found",
				Fix:      "Run: gam region touch app.missing --file <target>",
			},
		},
	}

	if result.Passed {
		t.Error("should not be passed")
	}
	if result.Details[0].Fix == "" {
		t.Error("fix should be populated for agent-actionable rejection")
	}
}

func TestStateMachineTransitions(t *testing.T) {
	sm := StateMachine{
		States: []string{"ACTIVE", "DISABLED"},
		Transitions: []Transition{
			{From: "ACTIVE", To: "DISABLED", Action: "disable"},
			{From: "DISABLED", To: "ACTIVE", Action: "enable"},
		},
	}

	// Valid transition
	found := false
	for _, tr := range sm.Transitions {
		if tr.From == "ACTIVE" && tr.To == "DISABLED" && tr.Action == "disable" {
			found = true
		}
	}
	if !found {
		t.Error("should find ACTIVE->DISABLED via disable")
	}

	// Invalid transition
	found = false
	for _, tr := range sm.Transitions {
		if tr.From == "ACTIVE" && tr.To == "ACTIVE" && tr.Action == "disable" {
			found = true
		}
	}
	if found {
		t.Error("should not find ACTIVE->ACTIVE via disable")
	}
}
