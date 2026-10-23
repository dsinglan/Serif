package rates

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
)

// Index holds the finished rows and files them by code, once, at startup,
// so each search only picks rows out.
type Index struct {
	Rows   []Row
	byCode map[string][]int    // code key -> positions in Rows
	npis   map[string][]string // every NPI in the file -> the organizations it's listed under
	eins   map[string][]string // every tax ID in the file, as digits -> its organization's names
	names  map[string][]string // every organization name in the file -> its tidied forms (see nameForms)

	// ModifierMeanings are CMS's official meanings for letter modifiers, such
	// as "GT". Number modifiers (CPT) aren't in it: the AMA licenses that text.
	ModifierMeanings map[string]string
	// CodeNames are what each code is called, by code key (see
	// Builder.CodeNames), for the searched code's card.
	CodeNames map[string]CodeName
}

// NewIndex files every row under its own code. A hospital row is also filed
// under the procedure in its side field, so a search for 27447 finds it. It
// also notes the organizations each NPI and tax ID is listed under, each
// name once in any capitalization, spacing or punctuation, for the messages
// a search with no rows needs, and tidies every name once for matching.
func NewIndex(rows []Row) *Index {
	idx := &Index{Rows: rows, byCode: map[string][]int{}, npis: map[string][]string{}, eins: map[string][]string{}, names: map[string][]string{}}
	listed := map[[3]string]bool{} // {"npi" or "ein", the number, tidied name} already noted
	for i, r := range rows {
		key := codeKey(r.BillingCode)
		idx.byCode[key] = append(idx.byCode[key], i)
		if r.ProcedureCode != "" && codeKey(r.ProcedureCode) != key {
			idx.byCode[codeKey(r.ProcedureCode)] = append(idx.byCode[codeKey(r.ProcedureCode)], i)
		}

		name := normName(r.BusinessName)
		note := func(under map[string][]string, kind, number string) {
			if k := [3]string{kind, number, name}; !listed[k] {
				listed[k] = true
				under[number] = append(under[number], r.BusinessName)
			}
		}
		for _, npi := range r.NPIs {
			note(idx.npis, "npi", npi)
		}
		note(idx.eins, "ein", einDigits(r.TIN))
		if _, ok := idx.names[r.BusinessName]; !ok {
			idx.names[r.BusinessName] = nameForms(r.BusinessName)
		}
	}
	for _, under := range []map[string][]string{idx.npis, idx.eins} {
		for _, list := range under {
			slices.Sort(list)
		}
	}
	return idx
}

// Query is one search: a billing code, and optionally an NPI, an EIN and
// part of an organization's name. The API checks the input before it gets
// here.
type Query struct {
	Code string
	NPI  string // 10 digits, or empty
	EIN  string // with or without the dash, or empty
	Name string // words from an organization's name, each from the start of a word, in any order, capitalization or punctuation, or empty
}

// Result is what a search found, split the way the app shows it.
type Result struct {
	Code          *Code // the searched code's card, or nil when the code isn't in the file
	Professional  []Row // doctors' fees, cheapest first
	Institutional []Row // facilities' fees, cheapest first
	CodeFound     bool  // the code has rows, before the NPI, EIN and name narrow them

	// MarketProfessional and MarketInstitutional count the code's rows in
	// each class before the NPI, EIN and name narrow them.
	MarketProfessional, MarketInstitutional int

	// ProviderFound is true when some row in the file, for any code, matches
	// every NPI, EIN and name given together (and when none was given).
	ProviderFound bool
	// NotFound names each of "npi", "ein" and "name" that matches nothing in
	// the file even on its own, so the page can say which box to check.
	NotFound []string
	// Organizations names the organizations the file lists the searched NPI
	// and EIN under, so a search with no rows can say who they belong to.
	Organizations Organizations
}

// Organizations are the names an NPI and an EIN are listed under in the
// file, sorted. A list is empty when that box wasn't searched, or matches
// nothing.
type Organizations struct {
	NPI []string `json:"npi"`
	EIN []string `json:"ein"`
}

// Code is the card for a searched code: what kind of code it is and what
// it's called, for the page's heading.
type Code struct {
	Type         string `json:"type"`                   // CPT, HCPCS or RC; for a code only in side fields, read from its shape
	Display      string `json:"display"`                // at its standard width, such as "0450"
	ShortName    string `json:"short_name"`             // the file's short name, such as "OFFICE/OUTPATIENT VISIT EST"; "" if none
	Name         string `json:"name"`                   // the full name, as rows show it; "" when neither the file nor CMS names it
	Discontinued string `json:"discontinued,omitempty"` // HCPCS: CMS's last date for the code, if CMS discontinued it
}

// Search finds the rows for a code, keeps those matching every NPI, EIN and
// name given, and sorts them by price. A search for one doctor's NPI also
// merges look-alike rows and notes conflicting prices (see byDoctor).
func (idx *Index) Search(q Query) Result {
	p := provider{npi: q.NPI, ein: einDigits(q.EIN), name: normName(q.Name)}
	res := Result{Professional: []Row{}, Institutional: []Row{}, NotFound: idx.notFound(p),
		Organizations: Organizations{NPI: []string{}, EIN: []string{}}}
	if p.npi != "" {
		res.Organizations.NPI = append(res.Organizations.NPI, idx.npis[p.npi]...) // a copy, so the index's list stays as it is
	}
	if p.ein != "" {
		res.Organizations.EIN = append(res.Organizations.EIN, idx.eins[p.ein]...)
	}

	key := codeKey(q.Code)
	positions := idx.byCode[key]
	res.CodeFound = len(positions) > 0
	res.Code = idx.describe(key, positions)
	for _, i := range positions {
		r := idx.Rows[i]
		facility := r.BillingClass == "institutional"
		if facility {
			res.MarketInstitutional++
		} else {
			res.MarketProfessional++
		}
		if !idx.matches(p, r) {
			continue
		}
		if facility {
			res.Institutional = append(res.Institutional, r)
		} else {
			res.Professional = append(res.Professional, r)
		}
	}
	if p.npi != "" {
		res.Professional = byDoctor(res.Professional)
		res.Institutional = byDoctor(res.Institutional)
	}
	slices.SortStableFunc(res.Professional, byPrice)
	slices.SortStableFunc(res.Institutional, byPrice)

	res.ProviderFound = len(res.Professional)+len(res.Institutional) > 0 || idx.anyRow(p)
	return res
}

// describe makes the card for a code key from the rows filed under it, or
// returns nil when there are none.
func (idx *Index) describe(key string, positions []int) *Code {
	if len(positions) == 0 {
		return nil
	}
	for _, i := range positions {
		if r := idx.Rows[i]; codeKey(r.BillingCode) == key {
			n := idx.CodeNames[key]
			return &Code{Type: r.BillingCodeType, Display: r.DisplayCode, ShortName: n.Short, Name: n.Full, Discontinued: r.Discontinued}
		}
	}
	// The code is only in hospitals' side fields: it has no item of its own,
	// so no short name, and its type comes from its shape.
	r := idx.Rows[positions[0]]
	kind := "CPT" // such as 27447 or 0075T
	if c := r.ProcedureCode[0]; c >= 'A' && c <= 'Z' {
		kind = "HCPCS" // a letter first, such as J0640
	}
	return &Code{Type: kind, Display: r.ProcedureCode, Name: r.ProcedureLabel, Discontinued: r.ProcedureDiscontinued}
}

// CountOrganizations counts the different organizations among rows, by tax ID.
func CountOrganizations(rows []Row) int {
	tins := map[string]bool{}
	for _, r := range rows {
		tins[einDigits(r.TIN)] = true
	}
	return len(tins)
}

// provider is the provider part of a query, tidied for matching.
type provider struct {
	npi, ein, name string
}

// matches reports whether a row fits every provider filter given.
func (idx *Index) matches(p provider, r Row) bool {
	if p.npi != "" && !slices.Contains(r.NPIs, p.npi) {
		return false // not this doctor
	}
	if p.ein != "" && einDigits(r.TIN) != p.ein {
		return false // not this practice
	}
	if p.name != "" && !p.inName(idx.names[r.BusinessName]) {
		return false // not this organization
	}
	return true
}

// inName reports whether a name, in any of its tidied forms, holds every one
// of the provider's words, each at the start of one of its words, in any
// order (see nameHas).
func (p provider) inName(forms []string) bool {
	return slices.ContainsFunc(forms, func(n string) bool { return nameHas(n, p.name) })
}

// notFound lists the provider filters that match nothing in the file, even
// on their own.
func (idx *Index) notFound(p provider) []string {
	missing := []string{}
	if p.npi != "" && len(idx.npis[p.npi]) == 0 {
		missing = append(missing, "npi")
	}
	if p.ein != "" && len(idx.eins[p.ein]) == 0 {
		missing = append(missing, "ein")
	}
	if p.name != "" && !idx.anyName(p) {
		missing = append(missing, "name")
	}
	return missing
}

// anyName reports whether any organization name in the file holds the
// provider's words.
func (idx *Index) anyName(p provider) bool {
	for _, forms := range idx.names {
		if p.inName(forms) {
			return true
		}
	}
	return false
}

// anyRow reports whether any row in the file, for any code, fits every
// provider filter given together. With no filters, every row does.
func (idx *Index) anyRow(p provider) bool {
	if p == (provider{}) {
		return true
	}
	return slices.ContainsFunc(idx.Rows, func(r Row) bool { return idx.matches(p, r) })
}

// byDoctor tidies the rows found for one doctor's NPI. The same price can be
// listed for several groups that hold this doctor, differing only in their
// other members; to someone looking up the doctor those rows look the same,
// so they're merged into one that counts its groups. And when the file lists
// prices for the same organization that differ only in the amount (same
// place, modifier and every other detail), those rows are flagged, because
// the file doesn't say which one applies.
func byDoctor(rows []Row) []Row {
	merged := []Row{}
	at := map[lookAlike]int{} // where each price is in merged
	for _, r := range rows {
		key := lookAlike{priceKeyOf(r), r.TINType, r.TIN, r.BusinessName}
		if n, ok := at[key]; ok {
			merged[n].Groups++
			merged[n].Source.ListedTimes += r.Source.ListedTimes
			continue
		}
		r.Groups = 1
		at[key] = len(merged)
		merged = append(merged, r)
	}

	prices := map[priceContext]map[float64]bool{} // the distinct prices in each context
	for _, r := range merged {
		c := contextOf(r)
		if prices[c] == nil {
			prices[c] = map[float64]bool{}
		}
		prices[c][r.Rate] = true
	}
	for i, r := range merged {
		if len(prices[contextOf(r)]) > 1 {
			merged[i].Flags = append(slices.Clone(r.Flags), FlagSeveralPrices) // a copy: rows share the index's slices
		}
	}
	return merged
}

// lookAlike is everything shown about a row except its group's members.
type lookAlike struct {
	price              priceKey
	tinType, tin, name string
}

// priceContext is everything about a row's price except the amount, plus
// whose price it is. Two rows in the same context with different amounts
// conflict.
type priceContext struct {
	price priceKey // with the rate left out
	tin   string
}

func contextOf(r Row) priceContext {
	k := priceKeyOf(r)
	k.rate = 0
	return priceContext{k, r.TIN}
}

// tidyName tidies an organization name for matching, so punctuation typed or
// left out doesn't matter: lower case, "&" read as "and", any mark but a
// period or an apostrophe read as a space, and runs of spaces made single
// (18 names in the file have a doubled space). Periods and apostrophes are
// dropped ("Inc." is "inc", "Mary's" is "marys"), or with spaced, read as
// spaces too ("O'Connor" is "o connor").
func tidyName(s string, spaced bool) string {
	var b strings.Builder
	for _, c := range strings.ToLower(s) {
		switch {
		case c == '&':
			b.WriteString(" and ")
		case c == '.' || c == '\'' || c == '’':
			if spaced {
				b.WriteByte(' ')
			}
		case unicode.IsLetter(c) || unicode.IsDigit(c):
			b.WriteRune(c)
		default:
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// normName tidies a name with its periods and apostrophes dropped: how typed
// words are matched, and how the index tells names apart.
func normName(s string) string {
	return tidyName(s, false)
}

// nameForms are the forms a name in the file is matched in: with its periods
// and apostrophes dropped ("st marys", "janice oconnor"), and read as spaces
// ("st mary s", "janice o connor"), so "Marys", "OConnor" and "Connor" all
// find theirs. One form when the two are the same.
func nameForms(s string) []string {
	dropped, spaced := tidyName(s, false), tidyName(s, true)
	if dropped == spaced {
		return []string{dropped}
	}
	return []string{dropped, spaced}
}

// nameHas reports whether a tidied name holds every one of the tidied words,
// each at the start of one of its words, in any order: "unity hospital" and
// "hospital unity" are in "the unity hospital of rochester inc", but "unity
// hospital" isn't in "auburn community hospital".
func nameHas(name, words string) bool {
	for _, w := range strings.Fields(words) {
		if !strings.Contains(" "+name, " "+w) {
			return false
		}
	}
	return true
}

// byPrice orders rows by price, then by organization name, so the order
// never changes between searches.
func byPrice(a, b Row) int {
	return cmp.Or(
		cmp.Compare(a.Rate, b.Rate),
		strings.Compare(a.BusinessName, b.BusinessName),
		strings.Compare(a.TIN, b.TIN),
	)
}
