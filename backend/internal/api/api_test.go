package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"serif/internal/rates"
)

// testAPI serves a small set of rows: Groth's two 99213 prices, a hospital's
// knee replacement under revenue code 0490, 25 doctor fees for 97110, 5
// facility fees under revenue code 0450 (filed as 450, as in the real file)
// and an ambulance's fee for HCPCS A0427, with the codes' names. The knee
// replacement, 27447, has none.
func testAPI() http.Handler {
	rows := []rates.Row{
		{BillingCodeType: "CPT", BillingCode: "99213", DisplayCode: "99213", BillingClass: "professional", Rate: 115.25, TIN: "20-3910419", BusinessName: "Groth", NPIs: []string{"1234567893"}},
		{BillingCodeType: "CPT", BillingCode: "99213", DisplayCode: "99213", BillingClass: "professional", Rate: 75, TIN: "20-3910419", BusinessName: "Groth", NPIs: []string{"1234567893"}},
		{BillingCodeType: "RC", BillingCode: "490", DisplayCode: "0490", BillingClass: "institutional", Rate: 9148.26, ProcedureCode: "27447", TIN: "11-1111111", BusinessName: "Hospital"},
	}
	for i := range 25 {
		rows = append(rows, rates.Row{BillingCodeType: "CPT", BillingCode: "97110", DisplayCode: "97110", BillingClass: "professional",
			Rate: float64(30 + i), TIN: fmt.Sprintf("30-00000%02d", i), BusinessName: fmt.Sprintf("Clinic %02d", i)})
	}
	for i := range 5 {
		rows = append(rows, rates.Row{BillingCodeType: "RC", BillingCode: "450", DisplayCode: "0450", BillingClass: "institutional",
			Rate: float64(500 + i), TIN: fmt.Sprintf("40-00000%02d", i), BusinessName: fmt.Sprintf("Hospital %02d", i)})
	}
	rows = append(rows, rates.Row{BillingCodeType: "HCPCS", BillingCode: "A0427", DisplayCode: "A0427", BillingClass: "professional",
		Rate: 638.53, TIN: "50-0000000", BusinessName: "Ambulance"})
	idx := rates.NewIndex(rows)
	idx.CodeNames = map[string]rates.CodeName{
		"A0427": {Short: "AMB SVC ALS ER TRNSPRT LVL1", Full: "Ambulance service, advanced life support, emergency transport, level 1 (als 1 - emergency)"},
		"99213": {Short: "OFFICE/OUTPATIENT VISIT EST", Full: "OFFICE OR OTH OUTPATIENT VISIT"},
		"97110": {Short: "THERAPEUTIC EXERCISES", Full: "THERAPEUTIC PROCEDURE"},
		"450":   {Short: "Emergency Room General", Full: "Emergency Room General"},
		"490":   {Short: "Ambulatory Surgical Care General", Full: "Ambulatory Surgical Care General"},
	}
	live := NewLive()
	live.Ready(idx, Source{RowCount: len(rows)})
	return New(live)
}

// get sends one request and decodes the JSON answer.
func get(t *testing.T, h http.Handler, method, url string) (int, map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, url, nil))
	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body
}

func TestSearch(t *testing.T) {
	h := testAPI()
	for _, c := range []struct {
		url      string
		class    string
		total    int
		cheapest float64
	}{
		{"/api/search?code=99213", "professional", 2, 75},
		{"/api/search?code=99213&ein=20-3910419&npi=1234567893", "professional", 2, 75},
		{"/api/search?code=27447", "institutional", 1, 9148.26}, // no doctor fees, so it shows the hospital's
		{"/api/search?code=%200490%20", "institutional", 1, 9148.26},
		{"/api/search?code=99213&name=GROTH", "professional", 2, 75},
	} {
		status, body := get(t, h, "GET", c.url)
		if status != http.StatusOK || body["class"] != c.class || body["total"] != float64(c.total) {
			t.Errorf("%s: status %d, class %v, total %v; want 200, %s, %d", c.url, status, body["class"], body["total"], c.class, c.total)
			continue
		}
		first := body["rows"].([]any)[0].(map[string]any)
		if first["negotiated_rate"] != c.cheapest {
			t.Errorf("%s: first row costs %v, want the cheapest, %v", c.url, first["negotiated_rate"], c.cheapest)
		}
	}
}

func TestPages(t *testing.T) {
	h := testAPI()
	for _, c := range []struct {
		url        string
		page, rows int
	}{
		{"/api/search?code=97110&page_size=10", 1, 10},
		{"/api/search?code=97110&page_size=10&page=3", 3, 5},
		{"/api/search?code=97110&page_size=10&page=9", 3, 5}, // past the end: the last page
	} {
		_, body := get(t, h, "GET", c.url)
		if body["page"] != float64(c.page) || len(body["rows"].([]any)) != c.rows || body["pages"] != float64(3) {
			t.Errorf("%s: page %v of %v with %d rows; want page %d of 3 with %d rows",
				c.url, body["page"], body["pages"], len(body["rows"].([]any)), c.page, c.rows)
		}
		if body["organization_count"] != float64(25) { // every page's, not this page's
			t.Errorf("%s: organization_count %v, want all 25", c.url, body["organization_count"])
		}
	}
}

// The answer echoes the search as asked, and says what the server chose in
// its own fields.
func TestQueryAsAsked(t *testing.T) {
	h := testAPI()
	_, body := get(t, h, "GET", "/api/search?code=27447")
	q, _ := body["query"].(map[string]any)
	if q["class"] != "" || body["class"] != "institutional" {
		t.Errorf("27447 with no class: query.class %q, class %q; want empty as asked, and institutional as picked", q["class"], body["class"])
	}
	if q["page"] != float64(0) || q["page_size"] != float64(0) || body["page"] != float64(1) || body["page_size"] != float64(50) {
		t.Errorf("27447 with no page: query page %v and page_size %v, shown page %v of size %v; want 0 and 0 as asked, and page 1 of size 50",
			q["page"], q["page_size"], body["page"], body["page_size"])
	}
	got, _ := json.Marshal(body["classes"])
	if want := `[{"billing_class":"professional","count":0,"organization_count":0},{"billing_class":"institutional","count":1,"organization_count":1}]`; string(got) != want {
		t.Errorf("classes = %s\nwant      %s", got, want)
	}

	// Spaces anywhere in an NPI or EIN are taken out, as a paste can bring them.
	status, body := get(t, h, "GET", "/api/search?code=99213&ein=20%203910419%20&npi=%201234%20567893")
	q, _ = body["query"].(map[string]any)
	if status != http.StatusOK || q["ein"] != "203910419" || q["npi"] != "1234567893" || body["total"] != float64(2) {
		t.Errorf("an EIN and NPI with spaces: status %d, query EIN %q and NPI %q, total %v; want 200, 203910419, 1234567893 and Groth's 2 rows",
			status, q["ein"], q["npi"], body["total"])
	}
	// So are dashes in an NPI, as some systems print it.
	status, body = get(t, h, "GET", "/api/search?code=99213&npi=123-456-7893")
	if q, _ = body["query"].(map[string]any); status != http.StatusOK || q["npi"] != "1234567893" || body["total"] != float64(2) {
		t.Errorf("an NPI with dashes: status %d, query NPI %q, total %v; want 200, 1234567893 and Groth's 2 rows", status, q["npi"], body["total"])
	}

	_, body = get(t, h, "GET", "/api/search?code=97110&page_size=10&page=9")
	q, _ = body["query"].(map[string]any)
	if q["page"] != float64(9) || body["page"] != float64(3) || q["page_size"] != float64(10) || body["page_size"] != float64(10) {
		t.Errorf("page 9 of 3: query page %v and page_size %v, shown page %v of size %v; want 9 and 10 as asked, and page 3 of size 10",
			q["page"], q["page_size"], body["page"], body["page_size"])
	}
}

// The answer carries the code's card, and who the searched NPI and EIN are
// listed under, even when no row matches.
func TestCodeAndOrganizations(t *testing.T) {
	h := testAPI()
	_, body := get(t, h, "GET", "/api/search?code=27447&npi=1234567893")
	if code, _ := body["code"].(map[string]any); code["type"] != "CPT" || code["display"] != "27447" || body["total"] != float64(0) {
		t.Errorf("code = %v, total %v; want 27447's card (a CPT code found only in a side field) and no rows for this NPI", body["code"], body["total"])
	}
	if got, _ := json.Marshal(body["provider_organizations"]); string(got) != `{"ein":[],"npi":["Groth"]}` {
		t.Errorf("provider_organizations = %s, want the NPI under Groth and no EIN searched", got)
	}
	if _, body = get(t, h, "GET", "/api/search?code=70450"); body["code"] != nil {
		t.Errorf("70450 isn't in the file, so code should be null, got %v", body["code"])
	}
}

// A code typed the way the page prints it, with its type in front, is the
// code: "CPT 99213", "hcpcs a0427", "RC 450".
func TestCodeAsPrinted(t *testing.T) {
	h := testAPI()
	for typed, want := range map[string]string{"CPT%2099213": "99213", "hcpcs%20a0427": "A0427", "RC%20%20450": "0450"} {
		_, body := get(t, h, "GET", "/api/search?code="+typed)
		if code, _ := body["code"].(map[string]any); code["display"] != want {
			t.Errorf("%s: code %v, want %s's card", typed, body["code"], want)
		}
	}
	// A word that only looks like a type is still a code to look for.
	if status, body := get(t, h, "GET", "/api/search?code=CPT"); status != http.StatusOK || body["code_found"] != false {
		t.Errorf("CPT alone: status %d, code_found %v; want 200, not found", status, body["code_found"])
	}
}

func TestBadInput(t *testing.T) {
	h := testAPI()
	for _, c := range []struct {
		url, code, field string
	}{
		{"/api/search", "invalid_code", "code"},
		{"/api/search?code=99-213!", "invalid_code", "code"},
		{"/api/search?code=0000", "invalid_code", "code"},
		{"/api/search?code=99213&npi=12345", "invalid_npi", "npi"},
		{"/api/search?code=99213&npi=1234567890", "invalid_npi", "npi"}, // the check digit doesn't match
		{"/api/search?code=99213&ein=12-345", "invalid_ein", "ein"},
		{"/api/search?code=99213&class=dental", "invalid_class", "class"},
		{"/api/search?code=99213&page=0", "invalid_page", "page"},
		{"/api/search?code=99213&page_size=5000", "invalid_page_size", "page_size"},
		{"/api/search?code=99213&name=s", "invalid_name", "name"},
		{"/api/search?code=99213&name=--", "invalid_name", "name"}, // no letter or digit to match
		{"/api/search?code=99213&name=a.", "invalid_name", "name"}, // one letter: names match on letters and digits
		{"/api/search?code=99213&npi=12345A7893", "invalid_npi", "npi"},
		{"/api/search?code=99213&place=1", "invalid_place", "place"},
		{"/api/search?code=99213&place=Office", "invalid_place", "place"},
		{"/api/search?code=99213&order=desc", "invalid_order", "order"},
	} {
		status, body := get(t, h, "GET", c.url)
		e, _ := body["error"].(map[string]any)
		if status != http.StatusBadRequest || e["code"] != c.code || e["field"] != c.field || e["message"] == "" {
			t.Errorf("%s: status %d, error %v; want 400 with %s on %s", c.url, status, body["error"], c.code, c.field)
		}
	}
}

func TestOtherAddresses(t *testing.T) {
	h := testAPI()
	if status, body := get(t, h, "GET", "/api/nope"); status != http.StatusNotFound || body["error"] == nil {
		t.Errorf("GET /api/nope: status %d, body %v; want a 404 with a JSON error", status, body)
	}
	if status, _ := get(t, h, "POST", "/api/search?code=99213"); status != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/search: status %d, want 405", status)
	}
}

func TestCrash(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	status, body := get(t, recoverPanics(http.HandlerFunc(buggyHandler)), "GET", "/api/search?code=99213")
	if e, _ := body["error"].(map[string]any); status != http.StatusInternalServerError || e["code"] != "internal" {
		t.Errorf("a crash: status %d, error %v; want a JSON 500", status, body["error"])
	}
	if !strings.Contains(logged.String(), "buggyHandler") {
		t.Errorf("the log's stack should show where the crash happened, in buggyHandler:\n%s", logged.String())
	}
}

// buggyHandler is a handler with a bug in it: it crashes.
func buggyHandler(http.ResponseWriter, *http.Request) { panic("a bug") }

func TestWhileLoading(t *testing.T) {
	live := NewLive()
	h := New(live)
	live.SetPhase(PhaseDownload)
	live.SetProgress(47_000_000, 89_359_830)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/search?code=99213", nil))
	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	e, _ := body["error"].(map[string]any)
	if w.Code != http.StatusServiceUnavailable || e["code"] != "not_ready" || w.Header().Get("Retry-After") == "" {
		t.Fatalf("search while downloading: status %d, body %v; want a 503 not_ready with Retry-After", w.Code, body)
	}
	if want := "Still loading: downloading the rate file (52%, 47 of 89 MB)."; e["message"] != want {
		t.Errorf("message = %q, want %q", e["message"], want)
	}

	// Bad input is still the person's to fix, even while loading.
	if status, _ := get(t, h, "GET", "/api/search?code=99-213!"); status != http.StatusBadRequest {
		t.Errorf("bad code while loading: status %d, want 400", status)
	}

	live.Fail(errors.New("downloading the index: no such host"))
	if st, _ := live.Snapshot(); st.BytesDone != 0 || st.BytesTotal != 0 {
		t.Errorf("after a failure, download progress is left over: %d of %d", st.BytesDone, st.BytesTotal)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/search?code=99213", nil))
	if w.Header().Get("Retry-After") != "" {
		t.Error("after a failed load, asking again won't help, so there should be no Retry-After")
	}
	status, body := get(t, h, "GET", "/api/search?code=99213")
	e, _ = body["error"].(map[string]any)
	if status != http.StatusServiceUnavailable || e["code"] != "load_failed" ||
		e["message"] != "Loading the data failed: downloading the index: no such host" {
		t.Errorf("search after a failure: status %d, error %v", status, body["error"])
	}
}

func TestStatus(t *testing.T) {
	live := NewLive()
	h := New(live)
	if _, body := get(t, h, "GET", "/api/status"); body["phase"] != PhaseStarting {
		t.Errorf("phase = %v, want %s", body["phase"], PhaseStarting)
	}
	live.Ready(rates.NewIndex(nil), Source{RowCount: 29446, FileVersion: "2.0.0"})
	status, body := get(t, h, "GET", "/api/status")
	src, _ := body["source"].(map[string]any)
	if status != http.StatusOK || body["phase"] != PhaseReady || src["row_count"] != float64(29446) {
		t.Errorf("status when ready: %d, %v", status, body)
	}
}

func TestModifierMeanings(t *testing.T) {
	rows := []rates.Row{
		{BillingCodeType: "CPT", BillingCode: "99213", DisplayCode: "99213", BillingClass: "professional", Rate: 51.45,
			TIN: "11-1111111", BusinessName: "Clinic", Modifiers: []string{"GT", "95"}},
		{BillingCodeType: "CPT", BillingCode: "99213", DisplayCode: "99213", BillingClass: "professional", Rate: 60,
			TIN: "22-2222222", BusinessName: "Other", Modifiers: []string{"RH", "GT"}},
	}
	idx := rates.NewIndex(rows)
	idx.ModifierMeanings = map[string]string{"GT": "Via interactive audio and video telecommunication systems"}
	live := NewLive()
	live.Ready(idx, Source{})

	_, body := get(t, New(live), "GET", "/api/search?code=99213")
	got, _ := json.Marshal(body["modifiers"])
	want := `[{"code":"95","licensed":true},` +
		`{"code":"GT","meaning":"Via interactive audio and video telecommunication systems","source":"CMS"},` +
		`{"code":"RH"}]`
	if string(got) != want {
		t.Errorf("modifiers = %s\nwant        %s", got, want)
	}
}

func TestMalformedAddress(t *testing.T) {
	status, body := get(t, testAPI(), "GET", "/api/search?code=99213&ein=99-9999999%")
	e, _ := body["error"].(map[string]any)
	if status != http.StatusBadRequest || e["code"] != "invalid_query" {
		t.Errorf("a stray %%: status %d, error %v; want a 400 invalid_query, not a search that ignores the EIN", status, body["error"])
	}
	_, body = get(t, testAPI(), "GET", "/api/search?code=99213&npi=12345A7893")
	if e, _ := body["error"].(map[string]any); e["message"] != "An NPI has 10 digits." {
		t.Errorf("a letter in an NPI: message %v, want the 10-digit message, not the check-digit one", e["message"])
	}
}

func TestNotFoundInAnswer(t *testing.T) {
	_, body := get(t, testAPI(), "GET", "/api/search?code=99213&npi=1497344725")
	got, _ := json.Marshal(body["not_found"])
	if body["provider_found"] != false || string(got) != `["npi"]` {
		t.Errorf("provider_found %v, not_found %s; want false and [\"npi\"]", body["provider_found"], got)
	}
}

func TestSummary(t *testing.T) {
	h := testAPI()

	// 97110's 25 prices run from $30 to $54, all different: the middle is
	// the 13th, $42, and no price is the most common. Its rows list no
	// places, so it has no place lines: an empty list, not null.
	_, body := get(t, h, "GET", "/api/search?code=97110")
	s, ok := body["summary"].(map[string]any)
	places, isList := s["places"].([]any)
	procedures, isList2 := s["procedures"].([]any)
	if !ok || s["count"] != float64(25) || s["left_out"] != float64(0) || s["lowest"] != float64(30) || s["middle"] != float64(42) ||
		s["highest"] != float64(54) || s["most_common"] != float64(0) || s["most_common_count"] != float64(0) ||
		!isList || len(places) != 0 || !isList2 || len(procedures) != 0 {
		t.Errorf("97110: summary %v, want 25 prices: $30, middle $42, $54, no most common, places [] and procedures []", body["summary"])
	}

	// The summary is for the class shown, and finds the code however it's
	// typed: 97110 has no facility fees, and 0450's rows are filed as 450.
	if _, body = get(t, h, "GET", "/api/search?code=97110&class=institutional"); body["summary"] != nil {
		t.Errorf("97110's facility fees: summary %v, want null", body["summary"])
	}
	_, body = get(t, h, "GET", "/api/search?code=0450")
	if s, ok := body["summary"].(map[string]any); body["class"] != "institutional" || !ok || s["count"] != float64(5) {
		t.Errorf("0450: class %v, summary %v; want the facility fees' summary of 5 prices", body["class"], body["summary"])
	}

	// Filters that leave no rows at all still show the code's market, in
	// the class a search with no filters would show.
	_, body = get(t, h, "GET", "/api/search?code=0450&name=zzzz")
	if s, ok := body["summary"].(map[string]any); body["class"] != "institutional" || body["total"] != float64(0) || !ok || s["count"] != float64(5) {
		t.Errorf("0450 for a name that matches nothing: class %v, total %v, summary %v; want institutional, 0, and the summary of 5",
			body["class"], body["total"], body["summary"])
	}

	// An EIN narrows the rows, not the summary: it still sums up every price.
	_, body = get(t, h, "GET", "/api/search?code=97110&ein=30-0000001")
	if s, ok := body["summary"].(map[string]any); body["total"] != float64(1) || !ok || s["count"] != float64(25) {
		t.Errorf("97110 for one EIN: total %v, summary %v; want 1 row, and the summary of all 25 prices", body["total"], body["summary"])
	}

	// Too few prices to sum up.
	_, body = get(t, h, "GET", "/api/search?code=99213")
	if body["summary"] != nil {
		t.Errorf("99213's 2 prices: summary %v, want null", body["summary"])
	}
}

func TestWriteJSONPlainSigns(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]string{"business_name": "ENT & Allergy <Associates>"})
	if got, want := rec.Body.String(), "{\"business_name\":\"ENT & Allergy <Associates>\"}\n"; got != want {
		t.Errorf("body %q, want %q: & < > as they are, not \\u0026 and the like", got, want)
	}
}

func TestCodes(t *testing.T) {
	h := testAPI()

	// Two kinds of code, in two lists: revenue codes are hospital charges,
	// CPT and HCPCS codes are services. Each list has the most prices first.
	status, body := get(t, h, "GET", "/api/codes?q=+general+")
	got, _ := json.Marshal(body)
	want := `{"charges":{"codes":[` +
		`{"count":5,"display":"0450","name":"Emergency Room General","short_name":"Emergency Room General","short_name_matches":true,"type":"RC"},` +
		`{"count":1,"display":"0490","name":"Ambulatory Surgical Care General","short_name":"Ambulatory Surgical Care General","short_name_matches":true,"type":"RC"}],"total":2},` +
		`"query":"general","services":{"codes":[],"total":0}}`
	if status != http.StatusOK || string(got) != want {
		t.Errorf("q=general: status %d\n got %s\nwant %s", status, got, want)
	}
	_, body = get(t, h, "GET", "/api/codes?q=office%20visit")
	services, _ := body["services"].(map[string]any)
	codes, _ := services["codes"].([]any)
	if len(codes) != 1 || codes[0].(map[string]any)["display"] != "99213" || codes[0].(map[string]any)["count"] != float64(2) {
		t.Errorf("q=office visit: services %v, want 99213 with its 2 prices", services)
	}
	// A HCPCS code is a service, like a CPT code. Its short name lacks
	// "emergency", so the page shows its full name too.
	_, body = get(t, h, "GET", "/api/codes?q=emergency")
	got, _ = json.Marshal(body)
	want = `{"charges":{"codes":[` +
		`{"count":5,"display":"0450","name":"Emergency Room General","short_name":"Emergency Room General","short_name_matches":true,"type":"RC"}],"total":1},` +
		`"query":"emergency","services":{"codes":[` +
		`{"count":1,"display":"A0427","name":"Ambulance service, advanced life support, emergency transport, level 1 (als 1 - emergency)","short_name":"AMB SVC ALS ER TRNSPRT LVL1","short_name_matches":false,"type":"HCPCS"}],"total":1}}`
	if string(got) != want {
		t.Errorf("q=emergency:\n got %s\nwant %s", got, want)
	}
	// A name nothing has: two empty lists, not nulls.
	_, body = get(t, h, "GET", "/api/codes?q=knee")
	if got, _ := json.Marshal(body); string(got) != `{"charges":{"codes":[],"total":0},"query":"knee","services":{"codes":[],"total":0}}` {
		t.Errorf("q=knee: %s, want two empty lists", got)
	}

	for _, c := range []struct {
		url, code string
	}{
		{"/api/codes", "invalid_q"},
		{"/api/codes?q=+e+", "invalid_q"}, // one letter, once the spaces are trimmed
		{"/api/codes?q=" + strings.Repeat("a", 101), "invalid_q"},
		{"/api/codes?q=%zz", "invalid_query"},
	} {
		status, body := get(t, h, "GET", c.url)
		if e, _ := body["error"].(map[string]any); status != http.StatusBadRequest || e["code"] != c.code || e["message"] == "" {
			t.Errorf("%.40s: status %d, error %v; want 400 %s", c.url, status, body["error"], c.code)
		}
	}
	if status, _ := get(t, h, "GET", "/api/codes?q="+strings.Repeat("a", 100)); status != http.StatusOK {
		t.Errorf("100 letters: status %d, want 200 (the most the server takes)", status)
	}

	// While loading, the same 503 a search gets.
	live := NewLive()
	live.SetPhase(PhaseDownload)
	w := httptest.NewRecorder()
	New(live).ServeHTTP(w, httptest.NewRequest("GET", "/api/codes?q=office", nil))
	if w.Code != http.StatusServiceUnavailable || w.Header().Get("Retry-After") == "" || !strings.Contains(w.Body.String(), `"not_ready"`) {
		t.Errorf("codes while loading: status %d, body %s; want a 503 not_ready with Retry-After", w.Code, w.Body)
	}
}

func TestCodeGroupCap(t *testing.T) {
	var g codeGroup
	for i := range 12 {
		g.add(rates.CodeMatch{Code: rates.Code{Display: fmt.Sprint(i)}})
	}
	if g.Total != 12 || len(g.Codes) != maxCodeMatches || g.Codes[9].Display != "9" {
		t.Errorf("12 matches: total %d, %d kept (last %q); want 12, the first %d", g.Total, len(g.Codes), g.Codes[len(g.Codes)-1].Display, maxCodeMatches)
	}
}

// placeAPI serves 99213's doctor fees at three places, from two
// organizations: one fee in an office, one posted for both an office and
// telehealth at home (both Office A's, NPI 1234567893), and one by telehealth
// outside the home (Telehealth B's, another NPI).
func placeAPI() http.Handler {
	fee := func(rate float64, tin, name, npi string, places ...string) rates.Row {
		return rates.Row{BillingCodeType: "CPT", BillingCode: "99213", DisplayCode: "99213", BillingClass: "professional",
			Rate: rate, TIN: tin, BusinessName: name, NPIs: []string{npi}, ServiceCodes: places}
	}
	rows := []rates.Row{
		fee(113.59, "11-1111111", "Office A", "1234567893", "11"),
		fee(86.71, "11-1111111", "Office A", "1234567893", "10", "11"),
		fee(51.45, "22-2222222", "Telehealth B", "1245319599", "02"),
	}
	live := NewLive()
	live.Ready(rates.NewIndex(rows), Source{RowCount: len(rows)})
	return New(live)
}

func TestPlace(t *testing.T) {
	h := placeAPI()

	// The office's fees, including the one posted for telehealth at home
	// too. The fee switch's counts are still every place's.
	_, body := get(t, h, "GET", "/api/search?code=99213&place=11")
	rows, _ := body["rows"].([]any)
	query, _ := body["query"].(map[string]any)
	classes, _ := body["classes"].([]any)
	professional, _ := classes[0].(map[string]any)
	if body["total"] != float64(2) || len(rows) != 2 || body["organization_count"] != float64(1) || query["place"] != "11" || professional["count"] != float64(3) {
		t.Errorf("place 11: total %v, %d rows, %v organizations, query.place %v, class count %v; want 2, 2, 1, 11 and 3",
			body["total"], len(rows), body["organization_count"], query["place"], professional["count"])
	}

	// Every place: all 3, from both organizations.
	if _, body := get(t, h, "GET", "/api/search?code=99213"); body["total"] != float64(3) || body["organization_count"] != float64(2) {
		t.Errorf("every place: total %v from %v organizations, want 3 from 2", body["total"], body["organization_count"])
	}

	// A place with an NPI: both filters at once. Office A's NPI has nothing
	// at 02, which only Telehealth B posts for.
	if _, body := get(t, h, "GET", "/api/search?code=99213&place=02"); body["total"] != float64(1) {
		t.Errorf("place 02: total %v, want 1", body["total"])
	}
	if _, body := get(t, h, "GET", "/api/search?code=99213&npi=1234567893&place=02"); body["total"] != float64(0) {
		t.Errorf("NPI 1234567893 and place 02: total %v, want 0", body["total"])
	}
	if _, body := get(t, h, "GET", "/api/search?code=99213&npi=1234567893&place=11"); body["total"] != float64(2) {
		t.Errorf("NPI 1234567893 and place 11: total %v, want 2", body["total"])
	}

	// A place none of the fees are posted for: no rows, not an error.
	status, body := get(t, h, "GET", "/api/search?code=99213&place=23")
	if rows, _ := body["rows"].([]any); status != http.StatusOK || body["total"] != float64(0) || rows == nil || len(rows) != 0 {
		t.Errorf("place 23: status %d, total %v, rows %v; want 200, 0 and an empty list", status, body["total"], body["rows"])
	}
}

// ratesOf lists the rows' prices, in order.
func ratesOf(body map[string]any) []float64 {
	out := []float64{}
	for _, r := range body["rows"].([]any) {
		out = append(out, r.(map[string]any)["negotiated_rate"].(float64))
	}
	return out
}

func TestOrder(t *testing.T) {
	h := placeAPI()
	for _, c := range []struct {
		url  string
		want []float64
	}{
		{"/api/search?code=99213", []float64{51.45, 86.71, 113.59}},                 // the lowest price first
		{"/api/search?code=99213&order=lowest", []float64{51.45, 86.71, 113.59}},    // the same, asked for
		{"/api/search?code=99213&order=highest", []float64{113.59, 86.71, 51.45}},   // the highest first
		{"/api/search?code=99213&place=11&order=highest", []float64{113.59, 86.71}}, // with a place
	} {
		_, body := get(t, h, "GET", c.url)
		if got := ratesOf(body); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.url, got, c.want)
		}
	}

	// The answer says which order it's in, as asked: the page reads it.
	for url, want := range map[string]string{"/api/search?code=99213": "", "/api/search?code=99213&order=highest": "highest"} {
		if _, body := get(t, h, "GET", url); body["query"].(map[string]any)["order"] != want {
			t.Errorf("%s: query.order %v, want %q", url, body["query"].(map[string]any)["order"], want)
		}
	}

	// The whole list is ordered before it's cut into pages: 97110's 25 fees
	// run from $30 to $54, so its first page of 10, highest first, starts at
	// $54 and its last ends at $30.
	_, first := get(t, testAPI(), "GET", "/api/search?code=97110&order=highest&page_size=10")
	_, last := get(t, testAPI(), "GET", "/api/search?code=97110&order=highest&page_size=10&page=3")
	if got, end := ratesOf(first), ratesOf(last); got[0] != 54 || end[len(end)-1] != 30 {
		t.Errorf("97110 highest first: page 1 starts at %v, page 3 ends at %v; want 54 and 30", got[0], end[len(end)-1])
	}

	// Equal prices keep the search's order, by name, highest first too.
	// Zeta comes first in the file; Alpha still comes first among the $80s.
	tie := func(rate float64, name string) rates.Row {
		return rates.Row{BillingCodeType: "CPT", BillingCode: "99213", DisplayCode: "99213", BillingClass: "professional",
			Rate: rate, TIN: "33-3333333", BusinessName: name, ServiceCodes: []string{"11"}}
	}
	live := NewLive()
	live.Ready(rates.NewIndex([]rates.Row{tie(80, "Zeta"), tie(80, "Alpha"), tie(90, "Gamma")}), Source{})
	_, body := get(t, New(live), "GET", "/api/search?code=99213&order=highest")
	names := []string{}
	for _, r := range body["rows"].([]any) {
		names = append(names, r.(map[string]any)["business_name"].(string))
	}
	if !slices.Equal(names, []string{"Gamma", "Alpha", "Zeta"}) {
		t.Errorf("highest first with a tie: %v, want [Gamma Alpha Zeta]", names)
	}
}
