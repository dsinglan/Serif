package rates

import (
	"reflect"
	"slices"
	"strconv"
	"testing"
)

// findIndex is a small file: two office visits, the ER's revenue code with a
// fee that names an ER visit, the ER visit itself, and a procedure found only
// inside a hospital fee, named nowhere.
func findIndex() *Index {
	rows := []Row{
		{BillingCodeType: "CPT", BillingCode: "99213", DisplayCode: "99213", BillingClass: "professional"},
		{BillingCodeType: "CPT", BillingCode: "99213", DisplayCode: "99213", BillingClass: "professional"},
		{BillingCodeType: "CPT", BillingCode: "99214", DisplayCode: "99214", BillingClass: "professional"},
		{BillingCodeType: "RC", BillingCode: "450", DisplayCode: "0450", BillingClass: "institutional"},
		{BillingCodeType: "RC", BillingCode: "450", DisplayCode: "0450", BillingClass: "institutional", ProcedureCode: "99285"},
		{BillingCodeType: "RC", BillingCode: "450", DisplayCode: "0450", BillingClass: "institutional"},
		{BillingCodeType: "CPT", BillingCode: "99285", DisplayCode: "99285", BillingClass: "professional"},
		{BillingCodeType: "RC", BillingCode: "490", DisplayCode: "0490", BillingClass: "institutional", ProcedureCode: "27447"},
	}
	idx := NewIndex(rows)
	idx.CodeNames = map[string]CodeName{
		"99213": {Short: "OFFICE/OUTPATIENT VISIT EST", Full: "OFFICE OR OTH OUTPATIENT VISIT"},
		"99214": {Short: "OFFICE/OUTPATIENT VISIT EST", Full: "OFFICE OR OTH OUTPATIENT VISIT"},
		"450":   {Short: "Emergency Room General", Full: "Emergency Room General"},
		"99285": {Short: "EMERGENCY DEPT VISIT", Full: "EMERGENCY DEPARTMENT VISIT"},
		"490":   {Short: "Ambulatory Surgical Care General", Full: "Ambulatory Surgical Care General"},
	}
	return idx
}

// found lists matches as "code:count", in order.
func found(matches []CodeMatch) []string {
	out := []string{}
	for _, m := range matches {
		out = append(out, m.Display+":"+strconv.Itoa(m.Count))
	}
	return out
}

func TestFindCodes(t *testing.T) {
	idx := findIndex()
	for _, c := range []struct {
		typed string
		want  []string
	}{
		{"office visit", []string{"99213:2", "99214:1"}},   // every word, in either name; the most prices first
		{"OFFICE   Visit", []string{"99213:2", "99214:1"}}, // any capitals and spacing
		{"emergency", []string{"0450:3", "99285:2"}},       // 99285 counts the hospital fee that names it
		{"emergency visit", []string{"99285:2"}},           // 0450's name has no "visit"
		{"department", []string{"99285:2"}},                // only in 99285's full name
		{"general", []string{"0450:3", "0490:1"}},
		{"patient", []string{"99213:2", "99214:1"}}, // a long word can be inside another: OUTPATIENT
		{"or", []string{"99213:2", "99214:1"}},      // a short word starts a word: OR, not inside AMBULATORY
		{"ent", []string{}},                         // only inside OUTPATIENT and DEPARTMENT
		{"27447", []string{}},                       // named nowhere: a code's number isn't one of its names
		{"knee", []string{}},
		{"   ", []string{}},
		{"-", []string{}},  // hyphens alone aren't a word, so nothing to find,
		{"--", []string{}}, // however many
	} {
		if got := found(idx.FindCodes(c.typed)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("FindCodes(%q) = %v, want %v", c.typed, got, c.want)
		}
	}
}

func TestWordPattern(t *testing.T) {
	for _, c := range []struct {
		word, name string
		want       bool
	}{
		{"ct", "ct abd & pelv w/contrast", true},       // starts the name
		{"ct", "psytx complex interactive", false},     // only inside a word
		{"iv", "ther/proph/diag iv inf init", true},    // after a space
		{"inj", "drain/inj joint/bursa w/us", true},    // after a slash
		{"e&m", "visit cplx inherent e&m asso", true},  // a short word with a sign in it
		{"therapy", "psychotherapy, 60 minutes", true}, // a long word, inside another
		{"body", "hepatitis c antibody;", true},        // 4 characters is long enough
		{"19", "sars-cov-2 covid19 w/optic", true},     // a number, inside a word
		{"x", "needle localization by xray", true},     // starting a word is enough: XRAY
		{"ray", "needle localization by xray", false},  // inside XRAY
		{"mg", "inj dexmeth sodm phosha 1mg", false},   // a digit is part of the word: 1MG
		{"(eg", "joint or bursa (eg, shoulder", true},  // signs are matched as typed,
		{"(als", "level 1 (als 1 - emergency)", true},  // short or long, not as patterns
		{"xray", "x-ray exam of knee 3", true},         // hyphens don't matter,
		{"x-ray", "needle localization by xray", true}, // either way,
		{"covid-19", "sars-cov-2 covid19 w/optic", true},
		{"cov", "sars-cov-2 test", true},                       // a short word starts a word after a hyphen,
		{"ray", "x-ray exam of knee 3", true},                  // so RAY in X-RAY starts one (not in XRAY, above)
		{"e-n-t", "office or oth outpatient visit", false},     // short once its hyphens are dropped: only inside OUTPATIENT
		{"21", "prev visit est age 12-17", false},              // between digits, a hyphen is a range:
		{"12", "chiropract manj 1-2 regions", false},           // never optional,
		{"1-2", "chiropract manj 1-2 regions", true},           // and kept as typed,
		{"1-2", "ecg routine ecg w/least 12 lds w/i&r", false}, // so 1-2 isn't 12
	} {
		if got := wordPattern(c.word).MatchString(c.name); got != c.want {
			t.Errorf("%q in %q: %v, want %v", c.word, c.name, got, c.want)
		}
	}
}

func TestFindCodesShortName(t *testing.T) {
	idx := findIndex()
	for _, c := range []struct {
		typed string
		want  bool
	}{
		{"emergency", true},         // EMERGENCY DEPT VISIT
		{"department", false},       // only in the full name, EMERGENCY DEPARTMENT VISIT
		{"emergency visit", true},   // both words in the short name
		{"department visit", false}, // one word only in the full name
	} {
		matches := idx.FindCodes(c.typed)
		i := slices.IndexFunc(matches, func(m CodeMatch) bool { return m.Display == "99285" })
		if i < 0 || matches[i].ShortNameMatches != c.want {
			t.Errorf("FindCodes(%q): 99285 found %v, short name matches %v; want found, %v", c.typed, i >= 0, i >= 0 && matches[i].ShortNameMatches, c.want)
		}
	}
}

func TestFindCodesTie(t *testing.T) {
	idx := findIndex()
	idx.CodeNames["99285"] = CodeName{Short: "OFFICE VISIT, AS A TEST", Full: ""} // two codes with 2 prices each
	if got := found(idx.FindCodes("office")); !reflect.DeepEqual(got, []string{"99213:2", "99285:2", "99214:1"}) {
		t.Errorf("a tie in prices goes by code: got %v", got)
	}
}

func TestRealFindCodes(t *testing.T) {
	idx := realIndex(t, realCMS(t))

	// The office visits: 9 CPT codes, 99214 with the most prices, and 2 HCPCS
	// codes whose CMS names mention an office visit (G2211 and G0498).
	visits := idx.FindCodes("office visit")
	if len(visits) != 11 || visits[0].Display != "99214" || visits[0].Count != 1699 || visits[1].Display != "99213" || visits[1].Count != 1210 {
		t.Errorf("office visit: %d codes, first %+v; want 11, 99214 with 1,699 prices then 99213 with 1,210", len(visits), visits[:min(2, len(visits))])
	}

	// "emergency" finds all three kinds: the ER's revenue codes, the ER
	// visits, and two ambulance codes whose CMS names say emergency.
	kinds := map[string][]string{}
	for _, m := range idx.FindCodes("emergency") {
		kinds[m.Type] = append(kinds[m.Type], m.Display)
	}
	want := map[string][]string{
		"RC":    {"0450", "0456", "0459", "0981"},
		"CPT":   {"99284", "99285", "99283"},
		"HCPCS": {"A0427", "A0429"},
	}
	if !reflect.DeepEqual(kinds, want) {
		t.Errorf("emergency: %v, want %v", kinds, want)
	}

	// "ct" finds the CT scans, not words with "ct" inside, such as
	// INTERACTIVE (90785, a psychotherapy code), and "e&m" finds G2211.
	scans := idx.FindCodes("ct scan")
	if len(scans) == 0 || slices.ContainsFunc(scans, func(m CodeMatch) bool { return m.Type != "RC" }) {
		t.Errorf("ct scan: %d codes, want only revenue codes (CT Scan General, Head scan...)", len(scans))
	}
	if slices.ContainsFunc(idx.FindCodes("ct"), func(m CodeMatch) bool { return m.Display == "90785" }) {
		t.Error("ct found 90785 through INTERACTIVE; a short word must start a word")
	}
	if !slices.ContainsFunc(idx.FindCodes("e&m"), func(m CodeMatch) bool { return m.Display == "G2211" }) {
		t.Error("e&m: want G2211, VISIT CPLX INHERENT E&M ASSO")
	}
	if got := found(idx.FindCodes("covid 19")); !reflect.DeepEqual(got, []string{"87635:93", "87811:12"}) {
		t.Errorf("covid 19: %v, want 87635 (COVID-19) and 87811 (COVID19)", got)
	}

	// A knee replacement is in the file (27447), but only inside hospital
	// fees, where nothing names it.
	if got := idx.FindCodes("knee replacement"); len(got) != 0 {
		t.Errorf("knee replacement: %+v, want nothing", got)
	}
}
