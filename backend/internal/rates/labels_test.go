package rates

import (
	"strings"
	"testing"

	"serif/internal/tic"
)

func TestLabels(t *testing.T) {
	b := NewBuilder()
	b.AddProviderReference(0, reference("1", group("11-1111111", "Clinic", "1234567893")))

	er := item("RC", "450", 500, "1")
	er.Description = "EMERGENCY DEPARTMENT VISIT FOR THE EVALUATION" // a copied procedure name, listed first
	er2 := item("RC", "450", 600, "1")
	er2.Description = "Emergency Room General" // the revenue code's real name
	unnamed := item("RC", "821", 50, "1")
	unnamed.Description = "HEMODIALYSIS PROCEDURE" // only a copied name
	visit := item("CPT", "99213", 75, "1")
	visit.Description = strings.Repeat("A", 99) + "B" // exactly 100 characters: cut off by the file
	critical := item("CPT", "99291", 300, "1")
	critical.Description = strings.Repeat("A", 98) + "B" // 99 characters: treated as cut too
	shorter := item("CPT", "99292", 150, "1")
	shorter.Description = strings.Repeat("A", 97) + "B" // 98: whole
	shot := item("HCPCS", "J1100", 0.10, "1")
	shot.Description = "INJ DEXMETH SODM PHOSHA 1MG"
	for i, it := range []tic.InNetworkItem{er, er2, unnamed, visit, critical, shorter, shot} {
		b.AddInNetwork(i, it)
	}

	want := map[string]string{
		"0450":  "Emergency Room General", // for both 0450 rows, even the one whose item had the copied name
		"0821":  "",                       // only a copied name, so no label
		"99213": strings.Repeat("A", 99) + "B…",
		"99291": strings.Repeat("A", 98) + "B…",
		"99292": strings.Repeat("A", 97) + "B",
		"J1100": "INJ DEXMETH SODM PHOSHA 1MG",
	}
	rows, _ := b.Rows()
	if len(rows) != 7 {
		t.Fatalf("got %d rows, want 7", len(rows))
	}
	for _, r := range rows {
		if r.Label != want[r.DisplayCode] {
			t.Errorf("label for %s ($%.2f) = %q, want %q", r.DisplayCode, r.Rate, r.Label, want[r.DisplayCode])
		}
	}
}

func TestProcedureFromSideField(t *testing.T) {
	b := NewBuilder()
	b.AddProviderReference(0, reference("1", group("11-1111111", "Hospital", "1234567893")))

	knee := item("RC", "490", 9148.26, "1")
	knee.NegotiatedRates[0].NegotiatedPrices[0].AdditionalInformation = "27447" // not an item of its own
	visit := item("RC", "510", 120, "1")
	visit.NegotiatedRates[0].NegotiatedPrices[0].AdditionalInformation = "99213" // listed as its own item below
	note := item("RC", "250", 15, "1")
	note.NegotiatedRates[0].NegotiatedPrices[0].AdditionalInformation = "see contract" // not a code
	doctor := item("CPT", "99213", 75, "1")
	doctor.Name = "OFFICE/OUTPATIENT VISIT EST"
	doctor.Description = "OFFICE VISIT"
	for i, it := range []tic.InNetworkItem{knee, visit, note, doctor} {
		b.AddInNetwork(i, it)
	}

	want := map[string][3]string{ // display code -> procedure code, its name and its short name
		"0490":  {"27447", "", ""},
		"0510":  {"99213", "OFFICE VISIT", "OFFICE/OUTPATIENT VISIT EST"},
		"0250":  {"", "", ""},
		"99213": {"", "", ""}, // a doctor's row: its own code is the procedure
	}
	rows, _ := b.Rows()
	for _, r := range rows {
		if got := [3]string{r.ProcedureCode, r.ProcedureLabel, r.ProcedureShortName}; got != want[r.DisplayCode] {
			t.Errorf("procedure for %s = %q, want %q", r.DisplayCode, got, want[r.DisplayCode])
		}
	}
}

func TestOfficialNames(t *testing.T) {
	b := NewBuilder()
	b.UseOfficialNames(map[string]string{
		"J1100": "Injection, dexamethasone sodium phosphate, 1 mg",
		"A0427": "Ambulance service, advanced life support, emergency transport, level 1 (als 1 - emergency)",
	})
	b.AddProviderReference(0, reference("1", group("11-1111111", "Clinic", "1234567893")))

	shot := item("HCPCS", "J1100", 0.11, "1")
	shot.Description = "INJECTION, DEXAMETHASONE SODIUM PH" // the file's own text, cut short
	visit := item("CPT", "99213", 75, "1")
	visit.Description = "OFFICE VISIT" // CPT keeps the file's name: CMS's file has none
	ride := item("RC", "540", 900, "1")
	ride.Description = "Ambulance General"
	ride.NegotiatedRates[0].NegotiatedPrices[0].AdditionalInformation = "A0427" // a HCPCS procedure in the side field
	for i, it := range []tic.InNetworkItem{shot, visit, ride} {
		b.AddInNetwork(i, it)
	}

	want := map[string][2]string{ // display code -> label and procedure name
		"J1100": {"Injection, dexamethasone sodium phosphate, 1 mg", ""},
		"99213": {"OFFICE VISIT", ""},
		"0540":  {"Ambulance General", "Ambulance service, advanced life support, emergency transport, level 1 (als 1 - emergency)"},
	}
	rows, _ := b.Rows()
	for _, r := range rows {
		if got := [2]string{r.Label, r.ProcedureLabel}; got != want[r.DisplayCode] {
			t.Errorf("%s: label and procedure name = %q, want %q", r.DisplayCode, got, want[r.DisplayCode])
		}
	}
}
