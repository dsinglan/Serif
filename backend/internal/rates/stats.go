package rates

import (
	"cmp"
	"slices"
	"strings"
)

// minPrices is the fewest prices worth summing up: with fewer, the table
// says it all. A line for one place or one procedure needs as many.
const minPrices = 5

// maxProcedureLines is the most procedure lines a summary lists, the ones
// with the most prices. The table has the rest.
const maxProcedureLines = 10

// Summary sums up the prices for one code and one class of fees: every
// price, before any NPI, EIN or name narrows them. Each price counts once,
// as in the table, and flagged prices are left out. A revenue code's
// facility fees are for different procedures, so its summary breaks them
// down by procedure; modifiers (such as 26 and TC) aren't split out.
type Summary struct {
	Count             int     `json:"count"`              // prices summed up
	LeftOut           int     `json:"left_out"`           // flagged prices, not counted
	OrganizationCount int     `json:"organization_count"` // different tax IDs among the prices counted
	ProcedureCount    int     `json:"procedure_count"`    // different procedures the prices name in hospitals' side fields
	WithProcedure     int     `json:"with_procedure"`     // prices that name one; in many revenue codes most name none
	Lowest            float64 `json:"lowest"`
	Middle            float64 `json:"middle"` // by nearest rank, so always a price someone posted
	Highest           float64 `json:"highest"`
	// MostCommon is the price posted most often (the lowest, in a tie), when
	// it's a pattern: at least 2 organizations post it, and it's at least a
	// tenth of the prices. Otherwise the three fields are 0.
	MostCommon              float64 `json:"most_common"`
	MostCommonCount         int     `json:"most_common_count"`
	MostCommonOrganizations int     `json:"most_common_organizations"`
	Places                  []Line  `json:"places"` // places with at least minPrices prices, the most first
	// Procedures are the procedures with at least minPrices prices, the most
	// first, up to maxProcedureLines. It's empty unless at least 2 qualify:
	// one line would only repeat the numbers above.
	Procedures []Line `json:"procedures"`
	// MixedPricing says the prices don't all carry the same pricing
	// modifiers (see pricingModifiers), so the middles mix different
	// services, such as a professional reading (26) with a whole test, or a
	// rental (RR) with a purchase (NU). The page doesn't compare fees with
	// them.
	MixedPricing bool `json:"mixed_pricing"`
}

// pricingModifiers price a part of a service, or another form of it, so a
// price with one isn't for the same thing as a price without it: the
// professional (26) and technical (TC) parts, a bilateral (50), multiple
// (51) or reduced (52) procedure, and new (NU), used (UE) or rented (RR)
// equipment. Others, such as 25 or 95, don't change what's priced.
var pricingModifiers = map[string]bool{"26": true, "TC": true, "50": true, "51": true, "52": true, "NU": true, "UE": true, "RR": true}

// pricingKey is a row's pricing modifiers, sorted and joined: "" for none.
func pricingKey(r Row) string {
	var kept []string
	for _, m := range r.Modifiers {
		if pricingModifiers[m] {
			kept = append(kept, m)
		}
	}
	slices.Sort(kept)
	return strings.Join(kept, " ")
}

// Line sums up the prices for one place of service, or one procedure. A
// price posted for several places counts in each.
type Line struct {
	Code    string  `json:"code"` // a place, such as "11"; a procedure, such as "45380", or "" for prices that name none
	Name    string  `json:"name"` // a procedure's short name in the file; else its full name (CMS's for a HCPCS code, such as H0020); "" for places and unnamed procedures
	Count   int     `json:"count"`
	Lowest  float64 `json:"lowest"`
	Middle  float64 `json:"middle"`
	Highest float64 `json:"highest"`
}

// Summary sums up every price for a code in one class of fees, before any
// NPI, EIN or name narrows them. It's nil when there are too few to sum up.
func (idx *Index) Summary(code, billingClass string) *Summary {
	var rows []Row
	for _, i := range idx.byCode[codeKey(code)] {
		if r := idx.Rows[i]; r.BillingClass == billingClass {
			rows = append(rows, r)
		}
	}
	s := summarize(rows)
	if s == nil {
		return nil
	}
	for i, l := range s.Procedures {
		if short := idx.CodeNames[codeKey(l.Code)].Short; short != "" {
			s.Procedures[i].Name = short // the file's short name fits a line better than the full one
		}
	}
	return s
}

// summarize sums up rows' prices, overall, for each place of service and
// for each procedure. It's nil when fewer than minPrices prices are left
// once flagged ones are taken out.
func summarize(rows []Row) *Summary {
	s := Summary{}
	var counted []Row
	var prices []float64
	byPlace := map[string][]float64{}
	byProcedure := map[string][]float64{}
	procedureNames := map[string]string{}
	pricing := map[string]bool{} // the different sets of pricing modifiers
	for _, r := range rows {
		if len(r.Flags) > 0 {
			s.LeftOut++
			continue
		}
		counted = append(counted, r)
		prices = append(prices, r.Rate)
		pricing[pricingKey(r)] = true
		for _, place := range r.ServiceCodes {
			byPlace[place] = append(byPlace[place], r.Rate)
		}
		byProcedure[r.ProcedureCode] = append(byProcedure[r.ProcedureCode], r.Rate)
		procedureNames[r.ProcedureCode] = r.ProcedureLabel
	}
	if len(prices) < minPrices {
		return nil
	}

	s.Count = len(prices)
	s.OrganizationCount = CountOrganizations(counted)
	s.ProcedureCount = len(byProcedure)
	if _, some := byProcedure[""]; some {
		s.ProcedureCount-- // prices that name no procedure aren't one
	}
	s.WithProcedure = s.Count - len(byProcedure[""])
	s.Lowest, s.Middle, s.Highest = spread(prices)
	s.MostCommon, s.MostCommonCount, s.MostCommonOrganizations = mostCommon(counted)
	s.Places = lines(byPlace, nil)
	s.Procedures = lines(byProcedure, procedureNames)
	if len(s.Procedures) < 2 {
		s.Procedures = []Line{}
	}
	s.Procedures = s.Procedures[:min(len(s.Procedures), maxProcedureLines)]
	s.MixedPricing = len(pricing) > 1
	return &s
}

// lines sums up each group of prices that has at least minPrices of them,
// the most prices first (a tie by code). names gives a group's name, if it
// has one.
func lines(groups map[string][]float64, names map[string]string) []Line {
	out := []Line{}
	for code, prices := range groups {
		if len(prices) >= minPrices {
			l := Line{Code: code, Name: names[code], Count: len(prices)}
			l.Lowest, l.Middle, l.Highest = spread(prices)
			out = append(out, l)
		}
	}
	slices.SortFunc(out, func(a, b Line) int {
		return cmp.Or(b.Count-a.Count, strings.Compare(a.Code, b.Code))
	})
	return out
}

// spread sorts prices and returns the lowest, the middle and the highest.
func spread(prices []float64) (lowest, middle, highest float64) {
	slices.Sort(prices)
	return prices[0], nearestRank(prices, 50), prices[len(prices)-1]
}

// nearestRank picks the p-th percentile of sorted prices by nearest rank:
// the price at rank p% of n, rounded up. For the middle (p = 50) of 102
// prices that's the 51st, so it's always a price someone posted, never the
// average of two.
func nearestRank(sorted []float64, p int) float64 {
	rank := (p*len(sorted) + 99) / 100 // p% of n, rounded up
	return sorted[rank-1]
}

// mostCommon finds the price posted most often (the lowest, in a tie), how
// many rows post it, and how many organizations. It returns zeros unless
// the price is a pattern: at least 2 organizations post it, and it's at
// least a tenth of the rows.
func mostCommon(rows []Row) (price float64, n, organizations int) {
	counts := map[float64]int{}
	for _, r := range rows {
		counts[r.Rate]++
	}
	for p, c := range counts {
		if c > n || c == n && p < price {
			price, n = p, c
		}
	}
	var posting []Row
	for _, r := range rows {
		if r.Rate == price {
			posting = append(posting, r)
		}
	}
	organizations = CountOrganizations(posting)
	if organizations < 2 || n*10 < len(rows) {
		return 0, 0, 0
	}
	return price, n, organizations
}
