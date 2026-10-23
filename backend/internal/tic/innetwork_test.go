package tic

import (
	"fmt"
	"strings"
	"testing"
)

// A tiny in-network file. The keys are in an unusual order on purpose, one
// key is unknown, and the NPIs come both as numbers and as strings.
const smallInNetwork = `{
  "in_network": [
    {
      "negotiation_arrangement": "ffs",
      "name": "OFFICE VISIT (TEST)",
      "billing_code_type": "CPT",
      "billing_code": "99213",
      "description": "A made-up description",
      "negotiated_rates": [
        {
          "provider_references": [1],
          "negotiated_prices": [
            {"negotiated_type": "negotiated", "negotiated_rate": 51.45, "expiration_date": "9999-12-31",
             "service_code": ["02", "10"], "billing_class": "professional", "setting": "outpatient"}
          ]
        }
      ]
    }
  ],
  "reporting_entity_name": "Test Insurer",
  "something_new": {"nested": [1, 2, 3]},
  "provider_references": [
    {"provider_group_id": 1, "network_name": ["Test"], "provider_groups": [
      {"npi": [1234567893, "1234567893"], "tin": {"type": "ein", "value": "12-3456789", "business_name": "Test Clinic"}}
    ]},
    {"provider_group_id": "2", "network_name": ["Test"], "provider_groups": []}
  ],
  "version": "2.0.0",
  "last_updated_on": "2026-09-02"
}`

func TestParse(t *testing.T) {
	var refs []ProviderReference
	var items []InNetworkItem
	head, err := Parse(strings.NewReader(smallInNetwork), Handlers{
		ProviderReference: func(_ int, pr ProviderReference) { refs = append(refs, pr) },
		InNetwork:         func(_ int, item InNetworkItem) { items = append(items, item) },
	})
	if err != nil {
		t.Fatal(err)
	}

	if head.ReportingEntityName != "Test Insurer" || head.Version != "2.0.0" || head.LastUpdatedOn != "2026-09-02" {
		t.Errorf("header = %+v", head)
	}
	if len(refs) != 2 || len(items) != 1 {
		t.Fatalf("got %d provider references and %d items, want 2 and 1", len(refs), len(items))
	}
	if npis := refs[0].ProviderGroups[0].NPI; len(npis) != 2 || npis[0] != "1234567893" || npis[1] != "1234567893" {
		t.Errorf("NPIs = %v, want the same NPI twice (once from a number, once from a string)", npis)
	}
	if refs[1].ProviderGroupID != "2" {
		t.Errorf("second provider_group_id = %q, want 2", refs[1].ProviderGroupID)
	}
	price := items[0].NegotiatedRates[0].NegotiatedPrices[0]
	if items[0].BillingCode != "99213" || price.NegotiatedRate == nil || *price.NegotiatedRate != 51.45 || len(price.ServiceCode) != 2 {
		t.Errorf("item = %+v", items[0])
	}
}

func TestParseCutOff(t *testing.T) {
	cut := smallInNetwork[:len(smallInNetwork)/2]
	if _, err := Parse(strings.NewReader(cut), Handlers{}); err == nil {
		t.Error("want an error for a cut-off file, got none")
	}
}

func TestParseSkipsBadItem(t *testing.T) {
	// The middle code's price is written as text, so that one code is set
	// aside. So is the null at the end, which Go would otherwise pass on empty.
	file := `{"in_network": [
	  {"billing_code": "99213", "negotiated_rates": [{"negotiated_prices": [{"negotiated_rate": 51.45}]}]},
	  {"billing_code": "99214", "negotiated_rates": [{"negotiated_prices": [{"negotiated_rate": "75.00"}]}]},
	  {"billing_code": "99215", "negotiated_rates": [{"negotiated_prices": [{"negotiated_rate": 98.10}]}]} ,
	  null
	]}`
	var codes []string
	head, err := Parse(strings.NewReader(file), Handlers{
		InNetwork: func(i int, item InNetworkItem) { codes = append(codes, fmt.Sprintf("%d:%s", i, item.BillingCode)) },
	})
	if err != nil {
		t.Fatal(err)
	}
	// 99215 is still at position 2: the skipped item counts toward the position.
	if got := strings.Join(codes, ", "); got != "0:99213, 2:99215" {
		t.Errorf("codes read = %q, want %q", got, "0:99213, 2:99215")
	}
	if head.Skipped != 2 {
		t.Errorf("Skipped = %d, want 2", head.Skipped)
	}
}

func TestParseUnreadableValues(t *testing.T) {
	// The second group's NPI isn't a number: that group is set aside. The
	// price written as null is kept, with no rate, for the next step to count.
	file := `{"provider_references": [
	    {"provider_group_id": 1, "provider_groups": [{"npi": [1234567893], "tin": {"type": "ein", "value": "11-1111111"}}]},
	    {"provider_group_id": 2, "provider_groups": [{"npi": ["N/A"], "tin": {"type": "ein", "value": "22-2222222"}}]}
	  ],
	  "in_network": [
	    {"billing_code": "99213", "negotiated_rates": [{"provider_references": [1], "negotiated_prices": [{"negotiated_rate": null}]}]}
	  ]}`
	var refs []ProviderReference
	var items []InNetworkItem
	head, err := Parse(strings.NewReader(file), Handlers{
		ProviderReference: func(_ int, pr ProviderReference) { refs = append(refs, pr) },
		InNetwork:         func(_ int, item InNetworkItem) { items = append(items, item) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || head.Skipped != 1 {
		t.Errorf("got %d provider references and %d skipped, want 1 and 1", len(refs), head.Skipped)
	}
	if len(items) != 1 || items[0].NegotiatedRates[0].NegotiatedPrices[0].NegotiatedRate != nil {
		t.Errorf("the null price should come through as a missing rate: %+v", items)
	}
}
