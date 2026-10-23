package rates

import (
	"fmt"
	"reflect"
	"testing"
)

// Tax IDs for the made-up organizations below.
const org1, org2, org3 = "11-1111111", "22-2222222", "33-3333333"

// fee is a doctor's fee at a price, from an organization, for some places.
func fee(rate float64, tin string, places ...string) Row {
	return Row{BillingClass: "professional", Rate: rate, TIN: tin, ServiceCodes: places}
}

// facilityFee is a hospital's fee under revenue code 0490, for the
// procedure in its side field ("" for none) and that procedure's name.
func facilityFee(rate float64, tin, procedure, name string) Row {
	return Row{BillingCodeType: "RC", BillingCode: "490", BillingClass: "institutional", Rate: rate, TIN: tin,
		ProcedureCode: procedure, ProcedureLabel: name}
}

func TestSummarize(t *testing.T) {
	rows := []Row{
		fee(50, org1, "02"),
		fee(50, org2, "02"),
		fee(60, org3, "02", "11"), // posted for two places: counts in each
		fee(110, "44-4444444", "11"),
		fee(120, "44-4444444", "11"), // the same organization again
		fee(130, "55-5555555", "11"),
		fee(140, "66-6666666", "11"),
		fee(0.5, "77-7777777", "11"),
	}
	rows[7].Flags = []string{FlagOneDollarOrLess} // flagged: left out, even from its place
	want := &Summary{
		Count: 7, LeftOut: 1, OrganizationCount: 6,
		Lowest: 50, Middle: 110, Highest: 140,
		MostCommon: 50, MostCommonCount: 2, MostCommonOrganizations: 2,
		// Place 02 has only 3 prices, too few for a line of its own.
		Places: []Line{{Code: "11", Count: 5, Lowest: 60, Middle: 120, Highest: 140}},
		// Doctors' fees name no procedure: a single group, so no lines.
		Procedures: []Line{},
	}
	if got := summarize(rows); !reflect.DeepEqual(got, want) {
		t.Errorf("summarize = %+v,\nwant %+v", got, want)
	}
}

func TestSummarizeTooFew(t *testing.T) {
	rows := []Row{fee(10, org1), fee(20, org1), fee(30, org1), fee(40, org1), fee(0.5, org1)}
	rows[4].Flags = []string{FlagOneDollarOrLess}
	if got := summarize(rows); got != nil {
		t.Errorf("4 prices once the flagged one is out: got %+v, want no summary", got)
	}
	rows[4] = fee(50, org1) // a fifth price: just enough
	if got := summarize(rows); got == nil || got.Count != 5 {
		t.Errorf("5 prices: got %+v, want a summary of 5", got)
	}
}

func TestSummarizeMixedPricing(t *testing.T) {
	for _, c := range []struct {
		name      string
		modifiers [][]string // one list per price
		want      bool
	}{
		{"no modifiers", [][]string{{}, {}, {}, {}, {}}, false},
		{"only modifiers that don't change what's priced", [][]string{{"25"}, {"95"}, {}, {"GT", "95"}, {"59"}}, false},
		{"every price a professional reading", [][]string{{"26"}, {"26"}, {"26", "59"}, {"26"}, {"26"}}, false},
		{"a reading beside whole tests", [][]string{{"26"}, {}, {}, {}, {}}, true},
		{"a purchase beside rentals", [][]string{{"RR"}, {"RR"}, {"NU"}, {"RR"}, {"RR"}}, true},
		{"the same modifiers in another order", [][]string{{"26", "50"}, {"50", "26"}, {"26", "50"}, {"50", "26"}, {"26", "50"}}, false},
	} {
		var rows []Row
		for _, m := range c.modifiers {
			r := fee(10, org1, "11")
			r.Modifiers = m
			rows = append(rows, r)
		}
		if got := summarize(rows).MixedPricing; got != c.want {
			t.Errorf("%s: MixedPricing = %v, want %v", c.name, got, c.want)
		}
	}
	// Each pricing modifier, beside prices with none, mixes the prices.
	for _, m := range []string{"26", "TC", "50", "51", "52", "NU", "UE", "RR"} {
		rows := []Row{fee(10, org1, "11"), fee(10, org1, "11"), fee(10, org1, "11"), fee(10, org1, "11"), fee(10, org1, "11")}
		rows[0].Modifiers = []string{m}
		if !summarize(rows).MixedPricing {
			t.Errorf("a price with %s beside prices with none: MixedPricing = false, want true", m)
		}
	}
}

func TestSummarizePlaceOrder(t *testing.T) {
	var rows []Row
	for range 5 {
		rows = append(rows, fee(10, org1, "11"), fee(10, org1, "10"))
	}
	rows = append(rows, fee(10, org1, "02", "11"))
	// 11 has 6 prices; 10 has 5; 02 has 1, too few for a line.
	if order := codes(summarize(rows).Places); !reflect.DeepEqual(order, []string{"11", "10"}) {
		t.Errorf("places in order %v, want [11 10]: the most prices first", order)
	}
	// A tie in count goes by code.
	if order := codes(summarize(rows[:10]).Places); !reflect.DeepEqual(order, []string{"10", "11"}) {
		t.Errorf("places with 5 prices each in order %v, want [10 11]", order)
	}
}

func TestSummarizeProcedures(t *testing.T) {
	var rows []Row
	for i := range 6 {
		rows = append(rows, facilityFee(float64(400+i), org1, "45380", "COLONOSCOPY, FLEXIBLE; WITH BIOPSY"))
	}
	for i := range 5 {
		rows = append(rows, facilityFee(float64(1300+i), org2, "66984", "")) // a procedure the file doesn't name
		rows = append(rows, facilityFee(float64(3700+i), org3, "", ""))      // no procedure named
	}
	rows = append(rows, facilityFee(600, org1, "29881", ""), facilityFee(610, org1, "29881", "")) // too few for a line
	s := summarize(rows)
	want := []Line{
		{Code: "45380", Name: "COLONOSCOPY, FLEXIBLE; WITH BIOPSY", Count: 6, Lowest: 400, Middle: 402, Highest: 405},
		{Code: "", Count: 5, Lowest: 3700, Middle: 3702, Highest: 3704}, // a tie with 66984: by code, so "" first
		{Code: "66984", Count: 5, Lowest: 1300, Middle: 1302, Highest: 1304},
	}
	if s.ProcedureCount != 3 || s.WithProcedure != 13 || !reflect.DeepEqual(s.Procedures, want) {
		t.Errorf("procedures: %d named by %d of 18 prices, lines %+v;\nwant 3 named by 13, lines %+v", s.ProcedureCount, s.WithProcedure, s.Procedures, want)
	}

	// One procedure (as in a search for 99213's facility fees): no lines,
	// which would only repeat the numbers above.
	if s := summarize(rows[:6]); s.ProcedureCount != 1 || len(s.Procedures) != 0 {
		t.Errorf("a single procedure: %d named, %d lines; want 1 and none", s.ProcedureCount, len(s.Procedures))
	}

	// At most maxProcedureLines lines: the ones with the most prices.
	rows = nil
	for p := range 12 {
		for i := range 5 + p {
			rows = append(rows, facilityFee(float64(100+i), org1, string(rune('A'+p)), ""))
		}
	}
	if got := codes(summarize(rows).Procedures); !reflect.DeepEqual(got, []string{"L", "K", "J", "I", "H", "G", "F", "E", "D", "C"}) {
		t.Errorf("12 procedures: lines %v, want the 10 with the most prices, most first", got)
	}
}

func TestSummaryShortNames(t *testing.T) {
	var rows []Row
	for i := range 5 {
		rows = append(rows, facilityFee(float64(400+i), org1, "45380", "COLONOSCOPY, FLEXIBLE; WITH BIOPSY"))
		rows = append(rows, facilityFee(float64(1300+i), org2, "66984", ""))
	}
	idx := NewIndex(rows)
	idx.CodeNames = map[string]CodeName{codeKey("45380"): {Short: "COLONOSCOPY AND BIOPSY", Full: "COLONOSCOPY, FLEXIBLE; WITH BIOPSY"}}
	var got []string
	for _, l := range idx.Summary("0490", "institutional").Procedures {
		got = append(got, l.Code+": "+l.Name)
	}
	// The file's short name where it has one; 66984 has no name at all.
	if want := []string{"45380: COLONOSCOPY AND BIOPSY", "66984: "}; !reflect.DeepEqual(got, want) {
		t.Errorf("lines %q, want %q", got, want)
	}
}

func TestSpread(t *testing.T) {
	for _, c := range []struct {
		prices                  []float64
		lowest, middle, highest float64
	}{
		{[]float64{30, 10, 20}, 10, 20, 30},
		{[]float64{40, 10, 30, 20}, 10, 20, 40}, // the 2nd of 4, not the average of 20 and 30
		{[]float64{7}, 7, 7, 7},
	} {
		lo, mid, hi := spread(c.prices)
		if lo != c.lowest || mid != c.middle || hi != c.highest {
			t.Errorf("spread(%v) = %v, %v, %v; want %v, %v, %v", c.prices, lo, mid, hi, c.lowest, c.middle, c.highest)
		}
	}
}

func TestNearestRank(t *testing.T) {
	sorted := []float64{10, 20, 30, 40}
	for _, c := range []struct {
		p    int
		want float64
	}{
		{50, 20},  // 50% of 4 is 2: the 2nd
		{75, 30},  // 75% of 4 is 3: the 3rd
		{90, 40},  // 90% of 4 is 3.6, rounded up: the 4th
		{100, 40}, // the highest
	} {
		if got := nearestRank(sorted, c.p); got != c.want {
			t.Errorf("nearestRank(%v, %d) = %v, want %v", sorted, c.p, got, c.want)
		}
	}
}

func TestMostCommon(t *testing.T) {
	// others makes n fees at different prices, each from its own organization.
	others := func(n int) []Row {
		var rows []Row
		for i := range n {
			rows = append(rows, fee(float64(1000+i), fmt.Sprintf("40-00000%02d", i)))
		}
		return rows
	}
	for _, c := range []struct {
		name       string
		rows       []Row
		price      float64
		n, posting int
	}{
		{"a tie goes to the lower price", []Row{fee(90, org1), fee(70, org1), fee(70, org2), fee(50, org1), fee(50, org2)}, 50, 2, 2},
		{"posted by one organization only", []Row{fee(70, org1), fee(70, org1), fee(50, org2), fee(60, org3)}, 0, 0, 0},
		{"exactly a tenth of the prices", append([]Row{fee(50, org1), fee(50, org2)}, others(18)...), 50, 2, 2},
		{"under a tenth", append([]Row{fee(50, org1), fee(50, org2)}, others(19)...), 0, 0, 0},
		{"no two the same", []Row{fee(10, org1), fee(20, org2), fee(30, org3)}, 0, 0, 0},
	} {
		if price, n, posting := mostCommon(c.rows); price != c.price || n != c.n || posting != c.posting {
			t.Errorf("%s: mostCommon = %v on %d from %d organizations, want %v on %d from %d",
				c.name, price, n, posting, c.price, c.n, c.posting)
		}
	}
}

func TestRealSummary(t *testing.T) {
	idx := realIndex(t, nil)

	// 99213 doctor fees: one price dominates, and the place changes the answer.
	doctor := idx.Summary("99213", "professional")
	want := &Summary{
		Count: 1108, OrganizationCount: 957,
		Lowest: 41.31, Middle: 51.45, Highest: 159.15,
		MostCommon: 51.45, MostCommonCount: 618, MostCommonOrganizations: 536,
		Places: []Line{
			{Code: "02", Count: 588, Lowest: 41.31, Middle: 51.45, Highest: 90},
			{Code: "10", Count: 365, Lowest: 50.4, Middle: 86.71, Highest: 153.62},
			{Code: "11", Count: 309, Lowest: 50.4, Middle: 113.59, Highest: 159.15},
			{Code: "19", Count: 22, Lowest: 67.21, Middle: 71.38, Highest: 89.29},
			{Code: "20", Count: 6, Lowest: 81.14, Middle: 81.14, Highest: 122.7}, // the 3rd of 6
			{Code: "22", Count: 5, Lowest: 43.93, Middle: 70.16, Highest: 71.38},
		},
		Procedures: []Line{},
	}
	if !reflect.DeepEqual(doctor, want) {
		t.Errorf("99213 doctor fees: %+v,\nwant %+v", doctor, want)
	}

	// The same code's facility fees: 102 prices, so the middle is the 51st.
	// Its most posted price, $145.06, is only 6 of them: not a pattern.
	if f := idx.Summary("99213", "institutional"); f == nil || f.Count != 102 || f.Middle != 162.56 || f.OrganizationCount != 84 ||
		f.MostCommon != 0 || f.ProcedureCount != 1 || len(f.Procedures) != 0 {
		t.Errorf("99213 facility fees: %+v, want 102 prices from 84 organizations, middle $162.56, no most common, one procedure", f)
	}

	// 90837: one flagged price is left out, and $94.50 is most of the rest.
	if s := idx.Summary("90837", "professional"); s == nil || s.Count != 3010 || s.LeftOut != 1 ||
		s.MostCommon != 94.5 || s.MostCommonCount != 2373 || s.MostCommonOrganizations != 2068 {
		t.Errorf("90837 doctor fees: %+v, want 3,010 prices, 1 left out, $94.50 on 2,373 from 2,068 organizations", s)
	}

	// 0490, the surgery center's fee, is for 161 procedures: lines for the
	// 10 with the most prices, by the file's short names.
	surgery := idx.Summary("0490", "institutional")
	first := Line{Code: "45380", Name: "COLONOSCOPY AND BIOPSY", Count: 42, Lowest: 226.51, Middle: 433.08, Highest: 829.02}
	if surgery == nil || surgery.ProcedureCount != 161 || surgery.WithProcedure != 471 || len(surgery.Procedures) != 10 || surgery.Procedures[0] != first {
		t.Errorf("0490: %+v, want 161 procedures named by 471 of 485 prices, and 10 lines, the first %+v", surgery, first)
	} else if none := surgery.Procedures[5]; none.Code != "" || none.Count != 14 || none.Middle != 3718.92 {
		t.Errorf("0490's 6th line: %+v, want the 14 prices that name no procedure, middle $3,718.92", none)
	}

	// The ER's fee (0450): 144 of its 207 prices name no procedure, so the
	// box mustn't say all 207 are for its 3 procedures.
	if er := idx.Summary("0450", "institutional"); er == nil || er.Count != 207 || er.ProcedureCount != 3 || er.WithProcedure != 63 {
		t.Errorf("0450: %+v, want 207 prices, 63 of them naming one of 3 procedures", er)
	}

	// 0450, the ER: the visit levels, and the fees that name none. The
	// file's short name is the same for all three levels.
	er := idx.Summary("0450", "institutional")
	var got []string
	for _, l := range er.Procedures {
		got = append(got, l.Code+" "+l.Name)
	}
	if want := []string{" ", "99285 EMERGENCY DEPT VISIT", "99284 EMERGENCY DEPT VISIT", "99283 EMERGENCY DEPT VISIT"}; !reflect.DeepEqual(got, want) {
		t.Errorf("0450's lines: %q, want %q", got, want)
	}

	// 0420's most posted price, $17.66, comes from one organization.
	if s := idx.Summary("0420", "institutional"); s == nil || s.MostCommon != 0 {
		t.Errorf("0420: %+v, want no most common price", s)
	}

	// Too few to sum up, or none at all.
	for _, c := range [][2]string{{"27447", "institutional"}, {"70450", "professional"}, {"0450", "professional"}} {
		if s := idx.Summary(c[0], c[1]); s != nil {
			t.Errorf("%s %s: got %+v, want no summary", c[0], c[1], s)
		}
	}
}

// codes lists lines' codes, in order.
func codes(lines []Line) []string {
	var out []string
	for _, l := range lines {
		out = append(out, l.Code)
	}
	return out
}
