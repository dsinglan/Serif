package tic

import (
	"strings"
	"testing"
)

// A tiny index shaped like Centene's: two groups of plans, one file each.
const smallIndex = `{
  "reporting_entity_name": "Test Insurer",
  "last_updated_on": "2026-08-31",
  "reporting_structure": [
    {
      "reporting_plans": [
        {"plan_name": "Gold", "plan_id_type": "hios", "plan_id": "11111NY001", "plan_market_type": "individual", "issuer_name": "Test"},
        {"plan_name": "Gold", "plan_id_type": "hios", "plan_id": "11111NY002", "plan_market_type": "individual", "issuer_name": "Test"},
        {"plan_name": "Bronze", "plan_id_type": "hios", "plan_id": "11111NY003", "plan_market_type": "individual", "issuer_name": "Test"}
      ],
      "in_network_files": [
        {"description": "in-network file", "location": "https://example.com/files/2026-08-31_test_fidelis-ex_in-network.json"}
      ]
    },
    {
      "reporting_plans": [
        {"plan_name": "Essential Plan 1", "plan_id_type": "hios", "plan_id": "11111EP000", "plan_market_type": "individual", "issuer_name": "Test"}
      ],
      "in_network_files": [
        {"description": "in-network file", "location": "https://example.com/files/2026-08-31_test_fidelis-es_in-network.json"}
      ]
    }
  ]
}`

func TestChooseInNetworkFile(t *testing.T) {
	idx, err := ReadIndex(strings.NewReader(smallIndex))
	if err != nil {
		t.Fatal(err)
	}

	choice, err := ChooseInNetworkFile(idx, "fidelis-ex")
	if err != nil {
		t.Fatal(err)
	}
	if choice.FileName != "2026-08-31_test_fidelis-ex_in-network.json" {
		t.Errorf("FileName = %q", choice.FileName)
	}
	if len(choice.Plans) != 3 {
		t.Errorf("got %d plan rows, want 3", len(choice.Plans))
	}
	if got := strings.Join(choice.PlanNames(), ", "); got != "Bronze, Gold" {
		t.Errorf("PlanNames = %q, want %q", got, "Bronze, Gold")
	}
	if issuers, markets := strings.Join(choice.IssuerNames(), ", "), strings.Join(choice.MarketTypes(), ", "); issuers != "Test" || markets != "individual" {
		t.Errorf("IssuerNames = %q and MarketTypes = %q, want Test and individual, each once", issuers, markets)
	}
}

func TestChooseInNetworkFileErrors(t *testing.T) {
	idx, err := ReadIndex(strings.NewReader(smallIndex))
	if err != nil {
		t.Fatal(err)
	}

	// The first matches nothing; the second matches both files.
	for _, match := range []string{"no-such-file", "fidelis"} {
		if _, err := ChooseInNetworkFile(idx, match); err == nil {
			t.Errorf("match %q: want an error, got none", match)
		}
	}
}

func TestSameFileUnderTwoGroups(t *testing.T) {
	idx := Index{ReportingStructure: []ReportingStructure{
		{ReportingPlans: []Plan{{PlanName: "Gold"}}, InNetworkFiles: []FileLink{{Location: "https://example.com/a_fidelis-ex.json"}}},
		{ReportingPlans: []Plan{{PlanName: "Silver"}}, InNetworkFiles: []FileLink{{Location: "https://example.com/a_fidelis-ex.json"}}},
	}}

	choice, err := ChooseInNetworkFile(idx, "fidelis-ex")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(choice.PlanNames(), ", "); got != "Gold, Silver" {
		t.Errorf("PlanNames = %q, want %q", got, "Gold, Silver")
	}
}

func TestReadIndexCutOff(t *testing.T) {
	if _, err := ReadIndex(strings.NewReader(`{"reporting_structure": [`)); err == nil {
		t.Error("want an error for a cut-off file, got none")
	}
}
