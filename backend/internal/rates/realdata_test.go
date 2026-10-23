package rates

import (
	"errors"
	"io/fs"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"serif/internal/cms"
	"serif/internal/tic"
)

// realBuilder reads the real rate file into a Builder, with CMS's official
// names and end dates when ref isn't nil. These tests check the numbers we
// quote, and run only once the file is downloaded into data/.
func realBuilder(t *testing.T, ref *cms.Reference) *Builder {
	t.Helper()
	f, err := os.Open("../../../data/2026-08-31_centene-management-company-llc_fidelis-ex_in-network.json")
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("the rate file isn't in data/ yet; run the server once to download it")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	b := NewBuilder()
	if ref != nil {
		b.UseOfficialNames(ref.Codes)
		b.UseEndDates(ref.Ended)
	}
	if _, err := tic.Parse(f, tic.Handlers{ProviderReference: b.AddProviderReference, InNetwork: b.AddInNetwork}); err != nil {
		t.Fatal(err)
	}
	return b
}

// realRows builds the rows from the real rate file (see realBuilder).
func realRows(t *testing.T, ref *cms.Reference) (rows []Row, unlinked int) {
	return realBuilder(t, ref).Rows()
}

// realIndex is the search index over the real rows, with each code's names
// for its card, as the server builds it.
func realIndex(t *testing.T, ref *cms.Reference) *Index {
	b := realBuilder(t, ref)
	rows, _ := b.Rows()
	idx := NewIndex(rows)
	idx.CodeNames = b.CodeNames()
	return idx
}

// realCMS reads CMS's HCPCS file from data/, or skips the test until it's there.
func realCMS(t *testing.T) *cms.Reference {
	t.Helper()
	ref, err := cms.ReadZip("../../../data/october-2026-alpha-numeric-hcpcs-file.zip")
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("CMS's HCPCS file isn't in data/ yet; run the server once to download it")
	}
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func TestRealSearch(t *testing.T) {
	idx := realIndex(t, nil)
	if n := len(idx.byCode); n != 1086 {
		t.Errorf("searchable codes = %d, want 1086 (802 codes plus 284 found only in side fields)", n)
	}
	for _, c := range []struct {
		name                   string
		q                      Query
		professional, hospital int
		cheapest               float64
	}{
		{"99213: doctor fees, plus clinic fees with 99213 in the side field", Query{Code: "99213"}, 1108, 102, 41.31},
		{"99213 at Groth MD PC", Query{Code: "99213", EIN: "20-3910419"}, 2, 0, 75},
		{"99213 for one doctor listed under two practices", Query{Code: "99213", NPI: "1780644302"}, 4, 0, 70.16},
		{"27447 knee replacement, only in the side field", Query{Code: "27447"}, 0, 2, 2160.86},
		{"the ER, typed as 0450", Query{Code: "0450"}, 0, 207, 136.30},
		{"the ER, typed as 450", Query{Code: "450"}, 0, 207, 136.30},
		{"J1100, priced per mg", Query{Code: "j1100"}, 50, 0, 0.11},
		{"99213 at organizations with Sinai in the name", Query{Code: "99213", Name: "sinai"}, 14, 1, 70.16},
		{"the ER at The Unity Hospital of Rochester, Inc., typed without its punctuation", Query{Code: "450", Name: "Unity Hospital of Rochester Inc"}, 0, 2, 151.97},
		{"the ER at Unity Hospital, not at the Community Hospitals", Query{Code: "450", Name: "unity hospital"}, 0, 2, 151.97},
	} {
		res := idx.Search(c.q)
		all := append(res.Professional, res.Institutional...)
		if len(res.Professional) != c.professional || len(res.Institutional) != c.hospital || len(all) == 0 || all[0].Rate != c.cheapest {
			t.Errorf("%s: got %d doctor and %d hospital rows; want %d and %d, cheapest %v",
				c.name, len(res.Professional), len(res.Institutional), c.professional, c.hospital, c.cheapest)
		}
	}
	if res := idx.Search(Query{Code: "70450"}); res.CodeFound {
		t.Error("70450 (a CT of the head) isn't in this file, so it shouldn't be found")
	}
	if res := idx.Search(Query{Code: "99213", NPI: "1234567893"}); res.ProviderFound {
		t.Error("NPI 1234567893 isn't in this file, so it shouldn't be found")
	}

	// One doctor's NPI: look-alike rows merge, conflicting prices are flagged.
	gupta := idx.Search(Query{Code: "76377", NPI: "1144635004"}).Professional
	if len(gupta) != 2 || gupta[0].Groups+gupta[1].Groups != 3 {
		t.Errorf("76377 for NPI 1144635004: %d rows in %d groups, want 2 rows in 3 groups", len(gupta), gupta[0].Groups+gupta[1].Groups)
	}
	ent := idx.Search(Query{Code: "99213", NPI: "1780644302"}).Professional
	if flagged := count(ent, func(r Row) bool { return slices.Contains(r.Flags, FlagSeveralPrices) }); len(ent) != 4 || flagged != 2 {
		t.Errorf("99213 for NPI 1780644302: %d rows, %d flagged; want 4 rows, 2 flagged ($137.39 and $139.39)", len(ent), flagged)
	}
	if res := idx.Search(Query{Code: "99213", NPI: "1780644302", EIN: "20-3910419"}); res.ProviderFound || len(res.NotFound) != 0 {
		t.Errorf("an NPI and EIN never listed together: provider found %v, not found %v; want false and none", res.ProviderFound, res.NotFound)
	}
	if n := len(idx.Search(Query{Code: "0490", Name: "One Day Surgery Center, LLC"}).Institutional); n != 13 {
		t.Errorf("0490 for \"One Day Surgery Center, LLC\" typed with single spaces: %d rows, want 13", n)
	}

	// The code card: what the page's heading shows.
	for _, c := range []struct {
		code string
		want *Code
	}{
		{"99213", &Code{Type: "CPT", Display: "99213", ShortName: "OFFICE/OUTPATIENT VISIT EST",
			Name: "OFFICE OR OTH OUTPATIENT VISIT FOR THE EVALUATION AND MANAGEMENT OF AN ESTABLISHED PATIENT, WHICH RE…"}},
		{"450", &Code{Type: "RC", Display: "0450", ShortName: "Emergency Room General", Name: "Emergency Room General"}},
		{"27447", &Code{Type: "CPT", Display: "27447"}}, // only in hospitals' side fields, and named nowhere
		{"70450", nil}, // not in the file
	} {
		if got := idx.Search(Query{Code: c.code}).Code; !reflect.DeepEqual(got, c.want) {
			t.Errorf("card for %s = %+v, want %+v", c.code, got, c.want)
		}
	}

	// Who an NPI and an EIN are listed under, for a search with no rows.
	res := idx.Search(Query{Code: "99213", NPI: "1265503726", EIN: "11-3471596"})
	if npi, ein := strings.Join(res.Organizations.NPI, "; "), strings.Join(res.Organizations.EIN, "; "); res.ProviderFound ||
		npi != "East Point Medical PC" || ein != "Minkowitz Consulting Pathologists PC" {
		t.Errorf("NPI 1265503726 with EIN 11-3471596: provider found %v, NPI under %q, EIN under %q; want false, East Point Medical PC and Minkowitz Consulting Pathologists PC",
			res.ProviderFound, npi, ein)
	}
	if n := CountOrganizations(idx.Search(Query{Code: "99213"}).Professional); n != 957 {
		t.Errorf("99213's doctor fees come from %d organizations (tax IDs), want 957", n)
	}
}

func TestRealFile(t *testing.T) {
	rows, unlinked := realRows(t, nil)

	listed, professional, doctor99213 := 0, 0, 0
	var groth *Row              // Timothy D. Groth MD PC's $75.00 office price for 99213
	padded := map[string]bool{} // codes the file stored without their leading zeros
	for i, r := range rows {
		listed += r.Source.ListedTimes
		if r.DisplayCode != r.BillingCode {
			padded[r.BillingCodeType+" "+r.DisplayCode] = true
		}
		if r.BillingClass == "professional" {
			professional++
			if r.BillingCode == "99213" {
				doctor99213++
			}
		}
		if r.BillingCode == "99213" && r.TIN == "20-3910419" && r.Rate == 75 {
			groth = &rows[i]
		}
	}

	// The row points back to the exact spot in the file:
	// in_network[1417].negotiated_rates[44].negotiated_prices[0], provider
	// reference 1115.
	want := Source{InNetworkIndex: 1417, NegotiatedRatesIndex: 44, NegotiatedPriceIndex: 0,
		ProviderGroupID: "1115", ProviderGroupIndex: 0, ListedTimes: 1}
	if groth == nil || groth.Source != want || len(groth.NPIs) != 5 {
		t.Errorf("Groth's 99213 row = %+v, want source %+v and 5 NPIs", groth, want)
	}
	for _, c := range []struct {
		name      string
		got, want int
	}{
		{"rows", len(rows), 29446},
		{"listings", listed, 201907},
		{"professional rows", professional, 24621},
		{"99213 doctor-fee rows", doctor99213, 1108},
		{"unlinked listings", unlinked, 0},
		{"codes padded back to full width", len(padded), 119},
	} {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
	if !padded["CPT 00731"] || !padded["CPT 00812"] {
		t.Error("CPT 731 and 812 should show as 00731 and 00812")
	}

	// Names: revenue codes get their standard name, never the copied procedure name.
	labels := map[string]string{} // display code -> label
	unnamed := 0
	for _, r := range rows {
		if _, ok := labels[r.DisplayCode]; !ok && r.BillingCodeType == "RC" && r.Label == "" {
			unnamed++
		}
		labels[r.DisplayCode] = r.Label
	}
	for code, want := range map[string]string{
		"0450": "Emergency Room General",
		"0490": "Ambulatory Surgical Care General",
		"0821": "",
		"0851": "",
	} {
		if labels[code] != want {
			t.Errorf("label for %s = %q, want %q", code, labels[code], want)
		}
	}
	if unnamed != 2 {
		t.Errorf("%d revenue codes have no standard name, want 2 (0821 and 0851)", unnamed)
	}
	if !strings.HasSuffix(labels["99213"], "WHICH RE…") {
		t.Errorf("99213's label should end with the file's cut-off and an ellipsis: %q", labels["99213"])
	}
	if !strings.HasSuffix(labels["99291"], "PATIENT; FIRST…") {
		t.Errorf("99291's name stops at 99 characters, so its label should end with an ellipsis: %q", labels["99291"])
	}
	cut := 0
	for _, label := range labels {
		if strings.HasSuffix(label, "…") {
			cut++
		}
	}
	if cut != 255 {
		t.Errorf("%d codes have a name the file cut off, want 255 (212 at 100 characters, 43 at 99)", cut)
	}

	// Procedures in the hospital side field.
	withProcedure, knee := 0, 0
	procedures, named := map[string]bool{}, map[string]bool{}
	for _, r := range rows {
		if r.ProcedureCode == "" {
			continue
		}
		withProcedure++
		procedures[r.ProcedureCode] = true
		if r.ProcedureLabel != "" {
			named[r.ProcedureCode] = true
		}
		if r.ProcedureCode == "27447" && r.DisplayCode == "0490" && r.ProcedureLabel == "" {
			knee++ // a knee replacement: only in the side field, never an item of its own
		}
	}
	for _, c := range []struct {
		name      string
		got, want int
	}{
		{"hospital rows with a procedure", withProcedure, 1848},
		{"different procedures", len(procedures), 465},
		{"procedures named in the file", len(named), 181},
		{"knee replacement (27447) rows under 0490", knee, 2},
		{"rows with the whole practice as the contract (NPI 0 only)", count(rows, func(r Row) bool { return r.TINLevel }), 15},
		{"rows listing an invalid NPI (124507857)", count(rows, func(r Row) bool { return slices.Contains(r.InvalidNPIs, "124507857") }), 5},
		{"rows flagged as a placeholder provider", count(rows, func(r Row) bool { return slices.Contains(r.Flags, FlagPlaceholderProvider) }), 36},
		{"CPT rows flagged $1 or less", count(rows, func(r Row) bool { return slices.Contains(r.Flags, FlagOneDollarOrLess) }), 95},
		{"CPT rows at exactly $1", count(rows, func(r Row) bool { return r.BillingCodeType == "CPT" && r.Rate == 1 }), 3},
		{"HCPCS rows at $1 or less, not flagged", count(rows, func(r Row) bool { return r.BillingCodeType == "HCPCS" && r.Rate <= 1 && len(r.Flags) == 0 }), 91},
	} {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
}

func count(rows []Row, match func(Row) bool) int {
	n := 0
	for _, r := range rows {
		if match(r) {
			n++
		}
	}
	return n
}

// TestRealFileWithCMSNames checks the names CMS's HCPCS file adds, once both
// files are in data/.
func TestRealFileWithCMSNames(t *testing.T) {
	idx := realIndex(t, realCMS(t))
	rows := idx.Rows

	// CMS's full names, and its end dates for discontinued codes, reach the cards.
	for _, c := range []struct {
		code string
		want *Code
	}{
		{"J1100", &Code{Type: "HCPCS", Display: "J1100", ShortName: "INJ DEXMETH SODM PHOSHA 1MG", Name: "Injection, dexamethasone sodium phosphate, 1 mg"}},
		{"J0640", &Code{Type: "HCPCS", Display: "J0640", Name: "Injection, leucovorin calcium, per 50 mg", Discontinued: "2026-09-30"}},
		{"J9190", &Code{Type: "HCPCS", Display: "J9190", Name: "Injection, fluorouracil, 500 mg", Discontinued: "2026-09-30"}},
	} {
		if got := idx.Search(Query{Code: c.code}).Code; !reflect.DeepEqual(got, c.want) {
			t.Errorf("card for %s = %+v, want %+v", c.code, got, c.want)
		}
	}
	if n := count(rows, func(r Row) bool { return r.Discontinued != "" || r.ProcedureDiscontinued != "" }); n != 2 {
		t.Errorf("%d rows name a code CMS discontinued, want 2 (J0640 and J9190, each in one clinic row's side field)", n)
	}

	named := map[string]bool{}
	cut := map[string]bool{} // codes whose name shows the cut mark
	for _, r := range rows {
		if r.DisplayCode == "J1100" && r.Label != "Injection, dexamethasone sodium phosphate, 1 mg" {
			t.Errorf("J1100 label = %q, want CMS's official name", r.Label)
		}
		if r.ProcedureCode != "" && r.ProcedureLabel != "" {
			named[r.ProcedureCode] = true
		}
		if strings.HasSuffix(r.Label, "…") {
			cut[r.DisplayCode] = true
		}
	}
	// 181 side-field procedures were named by the file; CMS names the 20 HCPCS ones it didn't.
	if len(named) != 201 {
		t.Errorf("side-field procedures with a name = %d, want 201", len(named))
	}
	// CMS's full names replace the 18 HCPCS names the file cut, so only CPT names keep the mark.
	if len(cut) != 237 {
		t.Errorf("%d codes show a cut name with CMS's names loaded, want 237 (255 less the 18 HCPCS)", len(cut))
	}
}
