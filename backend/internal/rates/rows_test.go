package rates

import (
	"encoding/json"
	"strings"
	"testing"

	"serif/internal/tic"
)

// group makes one provider group: a practice and its doctors.
func group(ein, name string, npis ...string) tic.ProviderGroup {
	g := tic.ProviderGroup{TIN: tic.TIN{Type: "ein", Value: ein, BusinessName: name}}
	for _, n := range npis {
		g.NPI = append(g.NPI, json.Number(n))
	}
	return g
}

// reference makes one provider reference: its provider_group_id and groups.
func reference(id string, groups ...tic.ProviderGroup) tic.ProviderReference {
	return tic.ProviderReference{ProviderGroupID: json.Number(id), ProviderGroups: groups}
}

// item makes a code with one office price, pointing at the given provider
// references.
func item(codeType, code string, rate float64, refs ...string) tic.InNetworkItem {
	nr := tic.NegotiatedRate{NegotiatedPrices: []tic.NegotiatedPrice{{
		NegotiatedType: "negotiated", NegotiatedRate: &rate, BillingClass: "professional", ServiceCode: []string{"11"},
	}}}
	for _, r := range refs {
		nr.ProviderReferences = append(nr.ProviderReferences, json.Number(r))
	}
	return tic.InNetworkItem{BillingCodeType: codeType, BillingCode: code, NegotiatedRates: []tic.NegotiatedRate{nr}}
}

func TestRows(t *testing.T) {
	b := NewBuilder()
	groth := group("20-3910419", "Groth", "1111111111", "2222222222")
	b.AddProviderReference(0, reference("1115", groth))
	b.AddProviderReference(1, reference("1116", groth)) // another provider reference holding the same group
	b.AddProviderReference(2, reference("2000", group("11-1111111", "Clinic A", "3333333333"), group("22-2222222", "Clinic B", "4444444444")))

	b.AddInNetwork(0, item("CPT", "99213", 75.00, "1115"))
	b.AddInNetwork(1, item("CPT", "99213", 75.00, "1116")) // the same price for the same group: a copy
	b.AddInNetwork(2, item("RC", "490", 100.00, "2000"))   // one price, two groups: two rows
	b.AddInNetwork(3, item("RC", "490", 100.00, "2000"))   // the same menu printed again: copies
	b.AddInNetwork(4, item("CPT", "99214", 90.00, "9999")) // a provider reference that doesn't exist

	rows, unlinked := b.Rows()
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3: %+v", len(rows), rows)
	}
	if unlinked != 1 {
		t.Errorf("unlinked = %d, want 1", unlinked)
	}

	// Groth's row stands for both listings and points at the first one.
	if r := rows[0]; r.BusinessName != "Groth" || r.Source.ListedTimes != 2 ||
		r.Source.InNetworkIndex != 0 || r.Source.ProviderGroupID != "1115" {
		t.Errorf("Groth's row = %+v", r)
	}
	// The 490 price became one row per clinic, each standing for the menu's two printings.
	for i, name := range []string{"Clinic A", "Clinic B"} {
		if r := rows[i+1]; r.BusinessName != name || r.BillingCode != "490" || r.Source.ListedTimes != 2 {
			t.Errorf("row %d = %+v", i+1, r)
		}
	}
}

// An empty list goes out as [], never null or left out.
func TestEmptyLists(t *testing.T) {
	b := NewBuilder()
	b.AddProviderReference(0, reference("1", group("11-1111111", "Clinic", "1234567893")))
	b.AddInNetwork(0, item("CPT", "99213", 75, "1"))
	rows, _ := b.Rows()
	got, _ := json.Marshal(rows[0])
	for _, empty := range []string{`"flags":[]`, `"invalid_npis":[]`, `"billing_code_modifier":[]`} {
		if !strings.Contains(string(got), empty) {
			t.Errorf("the row should have %s: %s", empty, got)
		}
	}
}

func TestPriceWithoutRate(t *testing.T) {
	b := NewBuilder()
	b.AddProviderReference(0, reference("1", group("11-1111111", "Clinic", "1234567893")))
	missing := item("CPT", "99213", 0, "1")
	missing.NegotiatedRates[0].NegotiatedPrices[0].NegotiatedRate = nil // the file wrote null
	b.AddInNetwork(0, missing)
	b.AddInNetwork(1, item("CPT", "99214", 90, "1"))
	nowhere := item("CPT", "99215", 0) // no rate and no provider reference: counted once, as no rate
	nowhere.NegotiatedRates[0].NegotiatedPrices[0].NegotiatedRate = nil
	b.AddInNetwork(2, nowhere)
	b.AddInNetwork(3, item("CPT", "99212", 40)) // a rate, but no provider reference: unlinked

	rows, unlinked := b.Rows()
	if len(rows) != 1 || rows[0].BillingCode != "99214" || b.PricesWithoutRate != 2 || unlinked != 1 {
		t.Errorf("got %d rows, %d prices without a rate and %d unlinked; want only 99214's row, 2 and 1",
			len(rows), b.PricesWithoutRate, unlinked)
	}
}
