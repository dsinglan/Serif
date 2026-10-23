// Package api serves the search over HTTP, as JSON.
package api

import (
	"cmp"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"serif/internal/rates"
)

// New returns the API's routes. They answer from live, which the loader
// fills in the background.
func New(live *Live) http.Handler {
	s := &server{live: live}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.status)
	mux.HandleFunc("GET /api/search", s.search)
	mux.HandleFunc("GET /api/codes", s.codes)
	mux.HandleFunc("GET /api/", notFound)
	return recoverPanics(mux)
}

type server struct {
	live *Live
}

// status answers GET /api/status: where loading is, and where the data came
// from. It always answers 200, even while loading or after a failure.
func (s *server) status(w http.ResponseWriter, r *http.Request) {
	st, _ := s.live.Snapshot()
	writeJSON(w, http.StatusOK, st)
}

// searchInput is a search request after its input has been checked. The
// answer echoes it as it was asked, only tidied (trimmed, spaces taken out
// of the NPI and the EIN, dashes out of the NPI, the code in capitals): a
// box left empty is "", and a page or page size nobody asked for is 0. What
// the server chose, such as the kind of fee or the page shown, is in the
// answer's own fields.
type searchInput struct {
	Code     string `json:"code"`
	NPI      string `json:"npi"`
	EIN      string `json:"ein"`
	Name     string `json:"name"`      // part of an organization's name
	Class    string `json:"class"`     // professional or institutional; empty lets the server pick
	Place    string `json:"place"`     // a place of service, such as "11", to show only its rows; empty for every place
	Order    string `json:"order"`     // "highest" for the highest price first; empty or "lowest" for the lowest first
	Page     int    `json:"page"`      // 0 when not asked: the first page
	PageSize int    `json:"page_size"` // 0 when not asked: defaultPageSize
}

// defaultPageSize is how many rows a page has when the search doesn't say.
const defaultPageSize = 50

// searchResponse is the answer to GET /api/search.
type searchResponse struct {
	Query             searchInput         `json:"query"`                  // the search as asked (see searchInput)
	Code              *rates.Code         `json:"code"`                   // the code's card for the heading; null when the code isn't in the file
	CodeFound         bool                `json:"code_found"`             // the code has rows, before the NPI, EIN and name narrow them
	ProviderFound     bool                `json:"provider_found"`         // some row in the file matches every NPI, EIN and name given
	NotFound          []string            `json:"not_found"`              // "npi", "ein" or "name": a box that matches nothing in the file
	Organizations     rates.Organizations `json:"provider_organizations"` // the organizations the searched NPI and EIN are listed under
	Classes           []classCount        `json:"classes"`                // rows per class, for the page's fee switch
	Class             string              `json:"class"`                  // the class shown below: as asked, or picked by the server
	Summary           *rates.Summary      `json:"summary"`                // every price for the code in that class, before the NPI, EIN and name narrow them; null when there are too few
	Total             int                 `json:"total"`                  // rows in that class, at the place asked for
	OrganizationCount int                 `json:"organization_count"`     // different tax IDs among those rows
	Page              int                 `json:"page"`                   // the page shown: the last one when asked for one past it
	Pages             int                 `json:"pages"`
	PageSize          int                 `json:"page_size"` // rows per page
	Rows              []rates.Row         `json:"rows"`      // that page's rows
	Modifiers         []modifier          `json:"modifiers"` // what the modifiers on these rows mean
}

// modifier explains one modifier code on the page.
type modifier struct {
	Code     string `json:"code"`
	Meaning  string `json:"meaning,omitempty"`  // CMS's official meaning, for letter modifiers
	Source   string `json:"source,omitempty"`   // "CMS" when there's a meaning
	Licensed bool   `json:"licensed,omitempty"` // a CPT (number) modifier: the AMA licenses its text
}

// classCount is how many rows one class has, and from how many organizations.
type classCount struct {
	BillingClass      string `json:"billing_class"`
	Count             int    `json:"count"`              // rows
	OrganizationCount int    `json:"organization_count"` // different organizations among them, by tax ID
}

// search answers GET /api/search?code=99213&npi=…&ein=…&name=…&class=…&place=…&order=…&page=…&page_size=…
func (s *server) search(w http.ResponseWriter, r *http.Request) {
	in, bad := readSearch(r)
	if bad != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{bad})
		return
	}
	st, idx := s.live.Snapshot()
	if idx == nil {
		writeNotReady(w, st)
		return
	}
	res := idx.Search(rates.Query{Code: in.Code, NPI: in.NPI, EIN: in.EIN, Name: in.Name})

	// Show doctors' fees unless there are none and the hospital has some:
	// among the rows the filters leave or, when they leave none, among all
	// the code's rows, so the summary still shows the code's market.
	professional, institutional := len(res.Professional), len(res.Institutional)
	if professional+institutional == 0 {
		professional, institutional = res.MarketProfessional, res.MarketInstitutional
	}
	rows := map[string][]rates.Row{"professional": res.Professional, "institutional": res.Institutional}
	class := in.Class
	if class == "" {
		class = "professional"
		if professional == 0 && institutional > 0 {
			class = "institutional"
		}
	}
	all := rows[class]
	if in.Place != "" {
		all = atPlace(all, in.Place)
	}
	if in.Order == "highest" {
		// The search sorted this request's own list lowest price first; a
		// stable sort keeps that order (by name) among equal prices.
		slices.SortStableFunc(all, func(a, b rates.Row) int { return cmp.Compare(b.Rate, a.Rate) })
	}

	// cmp.Or picks the first value that isn't 0: what was asked, or the default.
	size := cmp.Or(in.PageSize, defaultPageSize)
	pages := max(1, (len(all)+size-1)/size)
	page := min(cmp.Or(in.Page, 1), pages) // past the end means the last page
	start := (page - 1) * size
	end := min(start+size, len(all))

	shown := all[start:end]
	writeJSON(w, http.StatusOK, searchResponse{
		Query:         in,
		Code:          res.Code,
		CodeFound:     res.CodeFound,
		ProviderFound: res.ProviderFound,
		NotFound:      res.NotFound,
		Organizations: res.Organizations,
		Classes: []classCount{
			{"professional", len(res.Professional), rates.CountOrganizations(res.Professional)},
			{"institutional", len(res.Institutional), rates.CountOrganizations(res.Institutional)},
		},
		Class:             class,
		Summary:           idx.Summary(in.Code, class),
		Total:             len(all),
		OrganizationCount: rates.CountOrganizations(all),
		Page:              page,
		Pages:             pages,
		PageSize:          size,
		Rows:              shown,
		Modifiers:         explainModifiers(shown, idx.ModifierMeanings),
	})
}

// maxCodeMatches is the most codes /api/codes lists of each kind. Each
// list's total counts them all.
const maxCodeMatches = 10

// codesResponse is the answer to GET /api/codes: the codes whose names
// contain the words, in two lists that are never ranked against each other.
// A revenue code is a hospital's charge code (in this file, every price for
// one is a facility fee); CPT and HCPCS codes are services.
type codesResponse struct {
	Query    string    `json:"query"`    // the words, as typed (trimmed)
	Charges  codeGroup `json:"charges"`  // revenue codes
	Services codeGroup `json:"services"` // CPT and HCPCS codes
}

// codeGroup is one kind of code's matches: the first maxCodeMatches, the
// most prices first, and how many there are in all.
type codeGroup struct {
	Total int               `json:"total"`
	Codes []rates.CodeMatch `json:"codes"`
}

// add puts a match in the group, if the group has room; total counts it
// either way.
func (g *codeGroup) add(m rates.CodeMatch) {
	g.Total++
	if len(g.Codes) < maxCodeMatches {
		g.Codes = append(g.Codes, m)
	}
}

// codes answers GET /api/codes?q=office visit: the codes whose names contain
// every word typed, for someone who doesn't know the code.
func (s *server) codes(w http.ResponseWriter, r *http.Request) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{&problem{Code: "invalid_query", Message: "The search address is malformed. Search again from the form."}})
		return
	}
	words := strings.TrimSpace(q.Get("q"))
	if n := utf8.RuneCountInString(words); n < 2 || n > 100 {
		writeJSON(w, http.StatusBadRequest, errorBody{&problem{"invalid_q", "q", "Type 2 to 100 characters of a code's name."}})
		return
	}
	st, idx := s.live.Snapshot()
	if idx == nil {
		writeNotReady(w, st)
		return
	}
	res := codesResponse{Query: words, Charges: codeGroup{Codes: []rates.CodeMatch{}}, Services: codeGroup{Codes: []rates.CodeMatch{}}}
	for _, m := range idx.FindCodes(words) { // already the most prices first
		if m.Type == "RC" {
			res.Charges.add(m)
		} else {
			res.Services.add(m)
		}
	}
	writeJSON(w, http.StatusOK, res)
}

// writeNotReady answers a search that came while the data is loading, or
// after loading failed: a 503 with the status, so the page can say which.
func writeNotReady(w http.ResponseWriter, st Status) {
	why := &problem{Code: "not_ready", Message: "Still loading: " + describe(st) + "."}
	if st.Phase == PhaseFailed {
		why = &problem{Code: "load_failed", Message: "Loading the data failed: " + st.Error}
	}
	if st.Phase != PhaseFailed {
		w.Header().Set("Retry-After", "2") // worth asking again soon; after a failure it isn't
	}
	writeJSON(w, http.StatusServiceUnavailable, notReadyBody{why, st})
}

// atPlace keeps the rows posted for place. A price posted for several
// places shows under each, as it counts in each of the summary's lines.
func atPlace(rows []rates.Row, place string) []rates.Row {
	kept := []rates.Row{}
	for _, r := range rows {
		if slices.Contains(r.ServiceCodes, place) {
			kept = append(kept, r)
		}
	}
	return kept
}

// explainModifiers lists each modifier on the page once, in order: CMS's
// meaning for a letter modifier, a "licensed" note for a number (CPT) one.
func explainModifiers(rows []rates.Row, meanings map[string]string) []modifier {
	seen := map[string]bool{}
	var codes []string
	for _, r := range rows {
		for _, m := range r.Modifiers {
			if !seen[m] {
				seen[m] = true
				codes = append(codes, m)
			}
		}
	}
	slices.Sort(codes)
	out := []modifier{}
	for _, c := range codes {
		m := modifier{Code: c}
		if meaning, ok := meanings[c]; ok {
			m.Meaning, m.Source = meaning, "CMS"
		} else if strings.Trim(c, "0123456789") == "" {
			m.Licensed = true
		}
		out = append(out, m)
	}
	return out
}

var (
	codeType   = regexp.MustCompile(`^(CPT|HCPCS|RC)\s+`) // the page prints codes so: "CPT 99213"
	codeShape  = regexp.MustCompile(`^[A-Z0-9]{3,7}$`)
	npiShape   = regexp.MustCompile(`^[0-9]{10}$`)
	placeShape = regexp.MustCompile(`^[0-9]{2}$`) // CMS's place-of-service codes, such as "11"
	einShape   = regexp.MustCompile(`^[0-9]{2}-?[0-9]{7}$`)
)

// readSearch checks the search's input. Each problem names its field, so the
// page can show the message next to the right box.
func readSearch(r *http.Request) (searchInput, *problem) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil { // a stray % or ; would otherwise drop a filter without a word
		return searchInput{}, &problem{Code: "invalid_query", Message: "The search address is malformed. Search again from the form."}
	}
	in := searchInput{
		Code:  codeType.ReplaceAllString(strings.ToUpper(strings.TrimSpace(q.Get("code"))), ""),
		NPI:   strings.NewReplacer(" ", "", "-", "").Replace(q.Get("npi")), // as in "178-064-4302"
		EIN:   strings.ReplaceAll(q.Get("ein"), " ", ""),                   // a paste can bring spaces, as in "16 0743029"
		Name:  strings.TrimSpace(q.Get("name")),
		Class: strings.TrimSpace(q.Get("class")),
		Place: strings.TrimSpace(q.Get("place")),
		Order: strings.TrimSpace(q.Get("order")),
	}

	if !codeShape.MatchString(in.Code) || strings.Trim(in.Code, "0") == "" {
		return in, &problem{"invalid_code", "code", "Enter a billing code of 3 to 7 letters or digits, like 99213, J1100 or 0450."}
	}
	if in.NPI != "" && !npiShape.MatchString(in.NPI) {
		return in, &problem{"invalid_npi", "npi", "An NPI has 10 digits."}
	}
	if in.NPI != "" && !rates.ValidNPI(in.NPI) {
		return in, &problem{"invalid_npi", "npi", in.NPI + " isn't a valid NPI (its check digit doesn't match). Check for a typo."}
	}
	if in.EIN != "" && !einShape.MatchString(in.EIN) {
		return in, &problem{"invalid_ein", "ein", "An EIN has 9 digits, like 20-3910419."}
	}
	if n := utf8.RuneCountInString(in.Name); in.Name != "" && (n < 2 || n > 100) {
		return in, &problem{"invalid_name", "name", "Type 2 to 100 characters of the organization's name."}
	}
	if in.Name != "" && lettersAndDigits(in.Name) < 2 {
		return in, &problem{"invalid_name", "name", "Type at least 2 letters or digits of the organization's name."} // names match on those alone
	}
	if in.Class != "" && in.Class != "professional" && in.Class != "institutional" {
		return in, &problem{"invalid_class", "class", "class is professional or institutional."}
	}
	if in.Place != "" && !placeShape.MatchString(in.Place) {
		return in, &problem{"invalid_place", "place", "A place of service is 2 digits, such as 11."}
	}
	if in.Order != "" && in.Order != "lowest" && in.Order != "highest" {
		return in, &problem{"invalid_order", "order", "order is lowest or highest."}
	}
	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return in, &problem{"invalid_page", "page", "page is a whole number, 1 or more."}
		}
		in.Page = n
	}
	if v := q.Get("page_size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 10 || n > 200 {
			return in, &problem{"invalid_page_size", "page_size", "page_size is a whole number from 10 to 200."}
		}
		in.PageSize = n
	}
	return in, nil
}

// lettersAndDigits counts the letters and digits in s: what a name matches on.
func lettersAndDigits(s string) int {
	n := 0
	for _, c := range s {
		if unicode.IsLetter(c) || unicode.IsDigit(c) {
			n++
		}
	}
	return n
}

// problem is an error the page can show: a short code, the field it's about
// (if any) and a message for people.
type problem struct {
	Code    string `json:"code"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type errorBody struct {
	Error *problem `json:"error"`
}

// notReadyBody is a 503 answer: the error, plus the status so the page can
// show progress.
type notReadyBody struct {
	Error  *problem `json:"error"`
	Status Status   `json:"status"`
}

// notFound answers any other /api/ address with JSON instead of a web page.
func notFound(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, errorBody{&problem{Code: "not_found", Message: "No API endpoint " + r.Method + " " + r.URL.Path}})
}

// recoverPanics answers a crash inside a handler with a JSON 500 the page
// can show, and logs the stack, so the log says where the crash happened.
// (net/http recovers too, but by closing the connection, which the page
// would see as a network error.)
func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Printf("panic serving %s: %v\n%s", r.URL, v, debug.Stack())
				writeJSON(w, http.StatusInternalServerError, errorBody{&problem{Code: "internal", Message: "Something went wrong on the server. See its log."}})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// writeJSON sends body as JSON. Go escapes &, < and > by default, for JSON
// placed inside an HTML page; this JSON never is (it's sent as
// application/json with nosniff, and the page shows it as text), so it
// writes them as they are, and "ENT & Allergy" reads plainly.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff") // a browser mustn't read it as anything else
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(body); err != nil {
		log.Printf("writing a response: %v", err)
	}
}
