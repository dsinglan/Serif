package rates

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"serif/internal/tic"
)

// searchIndex is a small file: two practices with doctor fees for 99213,
// and a hospital fee filed under revenue code 0490 for a knee replacement.
func searchIndex() *Index {
	b := NewBuilder()
	b.AddProviderReference(0, reference("1", group("20-3910419", "Groth", "1234567893")))
	b.AddProviderReference(1, reference("2", group("13-4000192", "ENT & Allergy", "1780644302")))
	b.AddProviderReference(2, reference("3", group("11-1111111", "Hospital", "1093742272")))

	b.AddInNetwork(0, item("CPT", "99213", 115.25, "1"))
	b.AddInNetwork(1, item("CPT", "99213", 75.00, "1", "2")) // one price for both practices
	knee := item("RC", "490", 9148.26, "3")
	knee.NegotiatedRates[0].NegotiatedPrices[0].BillingClass = "institutional"
	knee.NegotiatedRates[0].NegotiatedPrices[0].AdditionalInformation = "27447"
	b.AddInNetwork(2, knee)

	rows, _ := b.Rows()
	return NewIndex(rows)
}

func TestMarketCounts(t *testing.T) {
	// The filters narrow the rows, but the market counts all the code's rows.
	res := searchIndex().Search(Query{Code: "99213", NPI: "1780644302"})
	if len(res.Professional) != 1 || res.MarketProfessional != 3 || res.MarketInstitutional != 0 {
		t.Errorf("99213 for one NPI: %d rows, market %d doctor and %d hospital rows; want 1, 3 and 0",
			len(res.Professional), res.MarketProfessional, res.MarketInstitutional)
	}
	res = searchIndex().Search(Query{Code: "27447", EIN: "20-3910419"})
	if len(res.Institutional) != 0 || res.MarketInstitutional != 1 {
		t.Errorf("27447 for a doctor's EIN: %d hospital rows, market %d; want 0 and 1", len(res.Institutional), res.MarketInstitutional)
	}
}

func TestSearch(t *testing.T) {
	idx := searchIndex()
	for _, c := range []struct {
		name                     string
		q                        Query
		professional, hospital   int
		codeFound, providerFound bool
		cheapest                 float64
	}{
		{"a code", Query{Code: "99213"}, 3, 0, true, true, 75},
		{"a code and an EIN, with or without the dash", Query{Code: "99213", EIN: "203910419"}, 2, 0, true, true, 75},
		{"a code and an NPI", Query{Code: "99213", NPI: "1780644302"}, 1, 0, true, true, 75},
		{"a procedure found through the side field", Query{Code: "27447"}, 0, 1, true, true, 9148.26},
		{"a revenue code with or without its zero", Query{Code: "0490"}, 0, 1, true, true, 9148.26},
		{"the same, typed as 490", Query{Code: " 490 "}, 0, 1, true, true, 9148.26},
		{"a code that isn't in the file", Query{Code: "70450"}, 0, 0, false, true, 0},
		{"a provider without this code", Query{Code: "27447", EIN: "20-3910419"}, 0, 0, true, true, 0},
		{"a provider that isn't in the file", Query{Code: "99213", NPI: "1497344725"}, 0, 0, true, false, 0},
		{"part of a name, in any capitals", Query{Code: "99213", Name: "ent & ALLERGY"}, 1, 0, true, true, 75},
		{"a name and an EIN together", Query{Code: "99213", Name: "groth", EIN: "20-3910419"}, 2, 0, true, true, 75},
		{"a name that isn't in the file", Query{Code: "99213", Name: "sinai"}, 0, 0, true, false, 0},
		{"a name typed with extra spaces", Query{Code: "99213", Name: "  ent   &  allergy "}, 1, 0, true, true, 75},
		{"a name with 'and' for '&'", Query{Code: "99213", Name: "ENT and Allergy"}, 1, 0, true, true, 75},
		{"a later word of a name", Query{Code: "99213", Name: "allergy"}, 1, 0, true, true, 75},
		{"letters inside a word aren't a name", Query{Code: "99213", Name: "roth"}, 0, 0, true, false, 0}, // in "Groth"
		{"an NPI and an EIN that are never listed together", Query{Code: "99213", NPI: "1780644302", EIN: "20-3910419"}, 0, 0, true, false, 0},
	} {
		res := idx.Search(c.q)
		if len(res.Professional) != c.professional || len(res.Institutional) != c.hospital ||
			res.CodeFound != c.codeFound || res.ProviderFound != c.providerFound {
			t.Errorf("%s: got %d doctor and %d hospital rows, code found %v, provider found %v; want %d, %d, %v, %v",
				c.name, len(res.Professional), len(res.Institutional), res.CodeFound, res.ProviderFound,
				c.professional, c.hospital, c.codeFound, c.providerFound)
			continue
		}
		all := append(res.Professional, res.Institutional...)
		if len(all) > 0 && all[0].Rate != c.cheapest {
			t.Errorf("%s: first row costs %v, want the cheapest, %v", c.name, all[0].Rate, c.cheapest)
		}
	}
}

func TestNotFound(t *testing.T) {
	res := searchIndex().Search(Query{Code: "99213", NPI: "1497344725", EIN: "20-3910419", Name: "zzz"})
	if !slices.Equal(res.NotFound, []string{"npi", "name"}) {
		t.Errorf("NotFound = %v, want [npi name]: the EIN is in the file, the NPI and name aren't", res.NotFound)
	}
	// A name matches from the start of a word, here too: "roth" is inside "Groth".
	if res := searchIndex().Search(Query{Code: "99213", Name: "roth"}); !slices.Equal(res.NotFound, []string{"name"}) {
		t.Errorf("NotFound for roth = %v, want [name]", res.NotFound)
	}
}

func TestSearchOneDoctor(t *testing.T) {
	b := NewBuilder()
	b.AddProviderReference(0, reference("10", group("30-0000001", "Gupta", "1124184387", "1194725770")))
	b.AddProviderReference(1, reference("11", group("30-0000001", "Gupta", "1124184387", "1497344725"))) // the same practice, a slightly different list
	b.AddInNetwork(0, item("CPT", "76377", 40.64, "10", "11"))                                           // one price for both groups
	b.AddInNetwork(1, item("CPT", "99213", 137.39, "10"))
	b.AddInNetwork(2, item("CPT", "99213", 139.39, "11")) // a second price, same place and modifier
	rows, _ := b.Rows()
	idx := NewIndex(rows)

	// The code alone keeps one row per group.
	if n := len(idx.Search(Query{Code: "76377"}).Professional); n != 2 {
		t.Errorf("76377: got %d rows, want 2, one per group", n)
	}

	// One doctor's NPI merges the look-alikes into one row that counts its groups.
	got := idx.Search(Query{Code: "76377", NPI: "1124184387"}).Professional
	if len(got) != 1 || got[0].Groups != 2 || got[0].Source.ListedTimes != 2 {
		t.Errorf("76377 for one doctor = %+v, want 1 row in 2 groups", got)
	}

	// Two prices for the doctor in the same situation are both flagged.
	got = idx.Search(Query{Code: "99213", NPI: "1124184387"}).Professional
	if len(got) != 2 || !slices.Contains(got[0].Flags, FlagSeveralPrices) || !slices.Contains(got[1].Flags, FlagSeveralPrices) {
		t.Errorf("99213 for one doctor = %+v, want 2 rows, both flagged %s", got, FlagSeveralPrices)
	}

	// Flagging a search's rows leaves the index's own rows as they were.
	for _, r := range idx.Rows {
		if slices.Contains(r.Flags, FlagSeveralPrices) {
			t.Errorf("index row %s %v was changed by a search", r.BusinessName, r.Rate)
		}
	}
}

// Two amounts for one doctor conflict only when every other detail of the
// price is the same: a difference in any one of them is a different price.
func TestSeveralPricesNeedTheSameDetails(t *testing.T) {
	for _, c := range []struct {
		detail string
		change func(*tic.InNetworkItem)
	}{
		{"setting", func(it *tic.InNetworkItem) { it.NegotiatedRates[0].NegotiatedPrices[0].Setting = "inpatient" }},
		{"negotiated_type", func(it *tic.InNetworkItem) { it.NegotiatedRates[0].NegotiatedPrices[0].NegotiatedType = "fee schedule" }},
		{"expiration_date", func(it *tic.InNetworkItem) { it.NegotiatedRates[0].NegotiatedPrices[0].ExpirationDate = "2026-12-31" }},
		{"additional_information", func(it *tic.InNetworkItem) {
			it.NegotiatedRates[0].NegotiatedPrices[0].AdditionalInformation = "after hours"
		}},
		{"negotiation_arrangement", func(it *tic.InNetworkItem) { it.NegotiationArrangement = "bundle" }},
	} {
		b := NewBuilder()
		b.AddProviderReference(0, reference("10", group("30-0000001", "Gupta", "1124184387")))
		other := item("CPT", "99214", 90, "10")
		c.change(&other)
		b.AddInNetwork(0, item("CPT", "99214", 80, "10"))
		b.AddInNetwork(1, other)
		rows, _ := b.Rows()

		got := NewIndex(rows).Search(Query{Code: "99214", NPI: "1124184387"}).Professional
		if len(got) != 2 || slices.Contains(got[0].Flags, FlagSeveralPrices) || slices.Contains(got[1].Flags, FlagSeveralPrices) {
			t.Errorf("$80 and $90 with a different %s: got %+v, want 2 rows, neither flagged", c.detail, got)
		}
	}
}

func TestCodeCard(t *testing.T) {
	b := NewBuilder()
	b.UseOfficialNames(map[string]string{"J0640": "Injection, leucovorin calcium, per 50 mg"})
	b.UseEndDates(map[string]string{"J0640": "2026-09-30"})
	b.AddProviderReference(0, reference("1", group("11-1111111", "Clinic", "1234567893")))
	visit := item("CPT", "99213", 75, "1")
	visit.Name, visit.Description = "OFFICE/OUTPATIENT VISIT EST", "OFFICE OR OTH OUTPATIENT VISIT FOR AN ESTABLISHED PATIENT"
	er := item("RC", "450", 500, "1")
	er.Name, er.Description = "Emergency Room General", "Emergency Room General"
	knee := item("RC", "490", 9148.26, "1")
	knee.NegotiatedRates[0].NegotiatedPrices[0].AdditionalInformation = "27447" // a procedure only in the side field
	drug := item("RC", "529", 12, "1")
	drug.NegotiatedRates[0].NegotiatedPrices[0].AdditionalInformation = "J0640" // one CMS has discontinued
	for i, it := range []tic.InNetworkItem{visit, er, knee, drug} {
		b.AddInNetwork(i, it)
	}
	rows, _ := b.Rows()
	idx := NewIndex(rows)
	idx.CodeNames = b.CodeNames()

	for _, c := range []struct {
		code string
		want *Code
	}{
		{"99213", &Code{Type: "CPT", Display: "99213", ShortName: "OFFICE/OUTPATIENT VISIT EST", Name: "OFFICE OR OTH OUTPATIENT VISIT FOR AN ESTABLISHED PATIENT"}},
		{"0450", &Code{Type: "RC", Display: "0450", ShortName: "Emergency Room General", Name: "Emergency Room General"}},
		{"27447", &Code{Type: "CPT", Display: "27447"}},
		{"J0640", &Code{Type: "HCPCS", Display: "J0640", Name: "Injection, leucovorin calcium, per 50 mg", Discontinued: "2026-09-30"}},
		{"70450", nil},
	} {
		if got := idx.Search(Query{Code: c.code}).Code; !reflect.DeepEqual(got, c.want) {
			t.Errorf("card for %s = %+v, want %+v", c.code, got, c.want)
		}
	}
}

func TestOrganizations(t *testing.T) {
	b := NewBuilder()
	b.AddProviderReference(0, reference("1", group("11-1111111", "Clinic A", "1234567893")))
	b.AddProviderReference(1, reference("2", group("22-2222222", "Clinic B", "1234567893"), group("22-2222222", "CLINIC  B.", "1497344725")))
	b.AddInNetwork(0, item("CPT", "99213", 75, "1", "2"))
	rows, _ := b.Rows()
	idx := NewIndex(rows)

	// A code with no rows still says who the NPI and EIN belong to.
	res := idx.Search(Query{Code: "27447", NPI: "1234567893", EIN: "22-2222222"})
	if got := strings.Join(res.Organizations.NPI, "; "); got != "Clinic A; Clinic B" {
		t.Errorf("NPI listed under %q, want Clinic A; Clinic B", got)
	}
	if got := strings.Join(res.Organizations.EIN, "; "); got != "Clinic B" {
		t.Errorf("EIN listed under %q, want Clinic B once (CLINIC  B. is the same name)", got)
	}
	if res := idx.Search(Query{Code: "99213"}); res.Organizations.NPI == nil || res.Organizations.EIN == nil || len(res.Organizations.NPI)+len(res.Organizations.EIN) != 0 {
		t.Errorf("with no NPI or EIN searched, want two empty lists, got %+v", res.Organizations)
	}
	if n := CountOrganizations(idx.Search(Query{Code: "99213"}).Professional); n != 2 {
		t.Errorf("99213 comes from %d organizations, want 2 (by tax ID)", n)
	}
}

func TestNormName(t *testing.T) {
	for in, want := range map[string]string{
		"One  Day   Surgery Center, LLC ":       "one day surgery center llc",
		"The Unity Hospital of Rochester, Inc.": "the unity hospital of rochester inc",
		"Ahmed & Ahmed Physicians, PC":          "ahmed and ahmed physicians pc",
		"St. Mary’s Hospital-Amsterdam":         "st marys hospital amsterdam",
		"B.T.M. Psychiatric NP Services, PLLC":  "btm psychiatric np services pllc",
		"Family & Children's Service of Ithaca": "family and childrens service of ithaca",
		"--":                                    "",
	} {
		if got := normName(in); got != want {
			t.Errorf("normName(%q) = %q, want %q", in, got, want)
		}
	}
}

// A name typed without the file's punctuation finds it, in either of the
// name's forms, and words match from the start of a word only. The names
// are the file's.
func TestNameHas(t *testing.T) {
	for _, c := range []struct {
		name, words string
		want        bool
	}{
		{"The Unity Hospital of Rochester, Inc.", "Unity Hospital of Rochester Inc", true},
		{"The Unity Hospital of Rochester, Inc.", "unity hospital", true},
		{"The Unity Hospital of Rochester, Inc.", "rochester, inc.", true},
		{"The Unity Hospital of Rochester, Inc.", "roch", true}, // the start of a word
		{"The Unity Hospital of Rochester, Inc.", "nity hospital", false},
		{"Auburn Community Hospital", "Unity Hospital", false},
		{"4 N Medical Care P.C.", "medical care pc", true},
		{"Janice O'Connor", "Connor", true}, // the word after the apostrophe
		{"Janice O'Connor", "OConnor", true},
		{"Janice O'Connor", "o'connor", true},
		{"L'Refuah and Rehabilitation Center/DBA Ezra Medical Ctr", "Refuah", true},
		{"Erika H.Connor", "Connor", true},                                // the word after the period
		{"The Unity Hospital of Rochester, Inc.", "hospital unity", true}, // words in any order
		{"The Unity Hospital of Rochester, Inc.", "unity buffalo", false}, // every word must be there
		{"Icahn School of Medicine at Mount Sinai", "icahn mount sinai", true},
		{"ENT & Allergy Associates, LLP", "ent allergy", true}, // "and" left out
	} {
		has := slices.ContainsFunc(nameForms(c.name), func(n string) bool { return nameHas(n, normName(c.words)) })
		if has != c.want {
			t.Errorf("%q in %q: %v, want %v", c.words, c.name, has, c.want)
		}
	}
}

// A search matches a name in either form, for its rows and for the note
// that a name isn't in the file.
func TestSearchNameForms(t *testing.T) {
	b := NewBuilder()
	b.AddProviderReference(0, reference("1", group("11-1111111", "Janice O'Connor", "1234567893")))
	b.AddInNetwork(0, item("CPT", "90837", 120, "1"))
	rows, _ := b.Rows()
	idx := NewIndex(rows)
	for _, words := range []string{"Connor", "OConnor", "O'Connor", "janice o connor"} {
		if res := idx.Search(Query{Code: "90837", Name: words}); len(res.Professional) != 1 || len(res.NotFound) != 0 {
			t.Errorf("%q: %d rows, not found %v; want 1 row", words, len(res.Professional), res.NotFound)
		}
	}
}
