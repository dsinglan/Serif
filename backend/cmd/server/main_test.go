package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"serif/internal/api"
	"serif/internal/fetch"
)

// A tiny rate file: one practice, one price for 99213.
const smallRateFile = `{
  "reporting_entity_name": "Test Insurer", "version": "2.0.0", "last_updated_on": "2026-09-02",
  "provider_references": [
    {"provider_group_id": 1, "provider_groups": [{"npi": [1234567893], "tin": {"type": "ein", "value": "11-1111111", "business_name": "Clinic"}}]}
  ],
  "in_network": [
    {"billing_code_type": "CPT", "billing_code": "99213", "description": "OFFICE VISIT",
     "negotiated_rates": [{"provider_references": [1], "negotiated_prices": [
       {"negotiated_type": "negotiated", "negotiated_rate": 51.45, "billing_class": "professional", "service_code": ["02"]}]}]}
  ]
}`

// rateFilePath is where fakeCentene serves the rate file.
const rateFilePath = "/2026-08-31_test_fidelis-ex_in-network.json"

// fakeCentene serves an index and the rate file it points to; CMS's file is
// missing (a 404). With indexStatus other than 200, the index fails.
// override replaces what a path serves, such as "/hcpcs.zip".
func fakeCentene(t *testing.T, indexStatus int, override map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serve, ok := override[r.URL.Path]; ok {
			serve(w, r)
			return
		}
		switch r.URL.Path {
		case "/index.json":
			if indexStatus != http.StatusOK {
				w.WriteHeader(indexStatus)
				return
			}
			fmt.Fprintf(w, `{"reporting_entity_name": "Test Insurer", "last_updated_on": "2026-08-31",
			  "reporting_structure": [{"reporting_plans": [{"plan_name": "Gold", "issuer_name": "Test Issuer", "plan_market_type": "individual"}],
			  "in_network_files": [{"location": %q}]}]}`, srv.URL+rateFilePath)
		case rateFilePath:
			fmt.Fprint(w, smallRateFile)
		default:
			http.NotFound(w, r) // including CMS's file: it's optional
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testConfig(srv *httptest.Server, dataDir string) config {
	return config{
		indexURL:    srv.URL + "/index.json",
		match:       "fidelis-ex",
		dataDir:     dataDir,
		hcpcsURL:    srv.URL + "/hcpcs.zip",
		client:      srv.Client(),
		download:    fetch.Options{StallTimeout: time.Second, Tries: 1},
		cmsDeadline: 5 * time.Second,
	}
}

func TestLoadReady(t *testing.T) {
	srv := fakeCentene(t, http.StatusOK, nil)
	cfg := testConfig(srv, t.TempDir())

	live := api.NewLive()
	load(live, cfg)
	st, idx := live.Snapshot()
	if st.Phase != api.PhaseReady || idx == nil || st.Source.RowCount != 1 {
		t.Fatalf("status = %+v, want ready with 1 row", st)
	}
	if !strings.Contains(st.Source.HCPCSError, "404") {
		t.Errorf("CMS's file was missing, so hcpcs_error should say why: %q", st.Source.HCPCSError)
	}
	if issuers, markets := strings.Join(st.Source.IssuerNames, ", "), strings.Join(st.Source.PlanMarketTypes, ", "); issuers != "Test Issuer" || markets != "individual" {
		t.Errorf("issuer_names %q and plan_market_types %q, want the index's Test Issuer and individual", issuers, markets)
	}

	// A restart with the network down uses the saved index and rate file.
	srv.Close()
	again := api.NewLive()
	load(again, cfg)
	if st, _ := again.Snapshot(); st.Phase != api.PhaseReady {
		t.Errorf("restart with no network: phase %s (%s), want ready from the saved copies", st.Phase, st.Error)
	}

	// -refresh with the network down fails, but says the saved copies work.
	cfg.refresh = true
	refreshed := api.NewLive()
	load(refreshed, cfg)
	if st, _ := refreshed.Snapshot(); st.Phase != api.PhaseFailed || !strings.Contains(st.Error, "start without -refresh") {
		t.Errorf("refresh with no network: phase %s (%s), want failed, pointing to the saved copies", st.Phase, st.Error)
	}
}

// A refresh that fails leaves the copies saved earlier as they were, and the
// error says so, for the rate file and CMS's file as for the index.
func TestRefreshKeepsSavedCopies(t *testing.T) {
	var rateFileDown, cmsDown atomic.Bool
	busyWhen := func(down *atomic.Bool, body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if down.Load() {
				http.Error(w, "busy", http.StatusInternalServerError)
				return
			}
			fmt.Fprint(w, body)
		}
	}
	srv := fakeCentene(t, http.StatusOK, map[string]http.HandlerFunc{
		rateFilePath: busyWhen(&rateFileDown, smallRateFile),
		"/hcpcs.zip": busyWhen(&cmsDown, "not really a zip"), // saved, though it can't be read
	})
	cfg := testConfig(srv, t.TempDir())
	load(api.NewLive(), cfg) // saves all three files
	cfg.refresh = true

	cmsDown.Store(true)
	live := api.NewLive()
	load(live, cfg)
	st, _ := live.Snapshot()
	if st.Phase != api.PhaseReady {
		t.Fatalf("refresh with CMS down: phase %s (%s), want ready: CMS's names are optional", st.Phase, st.Error)
	}
	if !strings.Contains(st.Source.HCPCSError, "start without -refresh") {
		t.Errorf("refresh with CMS down: hcpcs_error = %q, want it to point to the saved copy", st.Source.HCPCSError)
	}

	rateFileDown.Store(true)
	live = api.NewLive()
	load(live, cfg)
	if st, _ := live.Snapshot(); st.Phase != api.PhaseFailed || !strings.Contains(st.Error, "in-network.json") || !strings.Contains(st.Error, "start without -refresh") {
		t.Errorf("refresh with the rate file down: phase %s (%s), want failed, naming the file and pointing to its saved copy", st.Phase, st.Error)
	}
}

func TestCMSTooSlow(t *testing.T) {
	srv := fakeCentene(t, http.StatusOK, map[string]http.HandlerFunc{
		"/hcpcs.zip": func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done() // CMS never answers
		},
	})
	cfg := testConfig(srv, t.TempDir())
	cfg.cmsDeadline = 100 * time.Millisecond

	live := api.NewLive()
	load(live, cfg)
	st, _ := live.Snapshot()
	if st.Phase != api.PhaseReady {
		t.Fatalf("phase %s (%s), want ready: CMS's names are optional", st.Phase, st.Error)
	}
	if !strings.Contains(st.Source.HCPCSError, "didn't arrive within 100ms") {
		t.Errorf("hcpcs_error = %q, want it to name the deadline", st.Source.HCPCSError)
	}

	// A timeout inside the download, such as waiting too long for the
	// server's headers, isn't the deadline, so it's reported as itself.
	quick := srv.Client().Transport.(*http.Transport).Clone()
	quick.ResponseHeaderTimeout = 50 * time.Millisecond
	cfg = testConfig(srv, t.TempDir())
	cfg.client = &http.Client{Transport: quick}
	live = api.NewLive()
	load(live, cfg)
	st, _ = live.Snapshot()
	if st.Phase != api.PhaseReady {
		t.Fatalf("phase %s (%s), want ready: CMS's names are optional", st.Phase, st.Error)
	}
	if err := st.Source.HCPCSError; strings.Contains(err, "didn't arrive within") || !strings.Contains(err, "timeout awaiting response headers") {
		t.Errorf("hcpcs_error = %q, want the header timeout, not the deadline", err)
	}
}

// panicky is an HTTP transport with a bug in it: it crashes.
type panicky struct{}

func (panicky) RoundTrip(*http.Request) (*http.Response, error) { panic("a bug in a download") }

func TestLoadCrash(t *testing.T) {
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	cfg := testConfig(fakeCentene(t, http.StatusOK, nil), t.TempDir())
	cfg.client = &http.Client{Transport: panicky{}}
	live := api.NewLive()
	load(live, cfg) // the crash is caught here, so the test goes on
	if st, _ := live.Snapshot(); st.Phase != api.PhaseFailed || !strings.Contains(st.Error, "crashed while loading: a bug in a download") {
		t.Errorf("status = %+v, want failed, naming the crash", st)
	}
	if !strings.Contains(logged.String(), "panicky.RoundTrip") {
		t.Errorf("the log's stack should show where the crash happened, in panicky.RoundTrip:\n%s", logged.String())
	}
}

func TestLoadFailed(t *testing.T) {
	cfg := testConfig(fakeCentene(t, http.StatusInternalServerError, nil), t.TempDir())

	live := api.NewLive()
	load(live, cfg)
	st, idx := live.Snapshot()
	if st.Phase != api.PhaseFailed || idx != nil || !strings.Contains(st.Error, "500") {
		t.Errorf("status = %+v, want failed, naming the server's 500", st)
	}

	// With -refresh and nothing saved yet, there's no saved copy to point to.
	cfg.refresh = true
	again := api.NewLive()
	load(again, cfg)
	if st, _ := again.Snapshot(); st.Phase != api.PhaseFailed || strings.Contains(st.Error, "saved earlier") {
		t.Errorf("refresh with nothing saved: phase %s (%s), want failed, with no mention of a saved copy", st.Phase, st.Error)
	}
}
