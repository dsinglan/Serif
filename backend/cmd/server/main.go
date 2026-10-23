// Command server serves the price search. It starts answering right away and
// loads the data in the background: the index, the rate file it points to
// and CMS's HCPCS names, each saved in the data folder and reused after.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"serif/internal/api"
	"serif/internal/cms"
	"serif/internal/fetch"
	"serif/internal/rates"
	"serif/internal/tic"
)

// The index file named in the assignment.
const defaultIndexURL = "https://www.centene.com/content/dam/centene/Centene%20Corporate/json/DOCUMENT/2026-08-31_fidelis_index.json"

// CMS's official HCPCS file for October 2026. CMS posts a new one each
// quarter at https://www.cms.gov/medicare/coding/hcpcsreleasecodesets/hcpcs-quarterly-update
const defaultHCPCSURL = "https://www.cms.gov/files/zip/october-2026-alpha-numeric-hcpcs-file.zip"

// config is the settings from the command line, plus how downloads are made.
type config struct {
	indexURL, match, dataDir, hcpcsURL string
	refresh                            bool
	client                             *http.Client  // the tests use their own
	download                           fetch.Options // how patient downloads are

	// cmsDeadline is how long CMS's file may take. Its names are optional,
	// so a slow or silent CMS server mustn't hold back the rows for long.
	cmsDeadline time.Duration
}

func main() {
	addr := flag.String("addr", "localhost:8080", "address to serve the API on")
	cfg := config{client: fetch.NewClient(), download: fetch.DefaultOptions, cmsDeadline: 2 * time.Minute}
	flag.StringVar(&cfg.indexURL, "index", defaultIndexURL, "URL of the index (table of contents) file")
	flag.StringVar(&cfg.match, "match", "fidelis-ex", "load the in-network file whose name contains this text")
	flag.StringVar(&cfg.dataDir, "data", "../data", "folder where downloaded files are saved")
	flag.StringVar(&cfg.hcpcsURL, "hcpcs", defaultHCPCSURL, "URL of CMS's HCPCS file (official names for HCPCS codes and modifiers)")
	flag.BoolVar(&cfg.refresh, "refresh", false, "download the files again, even if copies are saved")
	flag.Parse()

	// Take the port first: if it's busy, stop before downloading anything.
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	dataDir, _ := filepath.Abs(cfg.dataDir) // -data is relative to the folder the server starts in
	log.Printf("serving the API on http://%s (try /api/status); the page: npm run dev in frontend/, then http://localhost:5173", *addr)
	log.Printf("data folder: %s", dataDir)

	live := api.NewLive()
	go load(live, cfg) // in the background, so the server answers right away
	srv := &http.Server{Handler: api.New(live), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.Serve(ln))
}

// load runs the loading steps and reports how they end. Any failure, even a
// crash, ends up in /api/status instead of stopping the server, and a crash
// logs its stack, so the log says where it happened.
func load(live *api.Live, cfg config) {
	defer func() {
		if v := recover(); v != nil {
			log.Printf("loading crashed: %v\n%s", v, debug.Stack())
			live.Fail(fmt.Errorf("crashed while loading: %v", v))
		}
	}()
	if err := loadSteps(live, cfg); err != nil {
		log.Printf("loading failed: %v", err)
		live.Fail(err)
	}
}

// loadSteps gets the index and the rate file it points to, reads CMS's
// names if it can, builds the rows and hands the finished index to live.
// Each file is saved in the data folder, so a restart works offline.
func loadSteps(live *api.Live, cfg config) error {
	ctx := context.Background()

	live.SetPhase(api.PhaseIndex)
	indexFile, err := ensure(ctx, cfg, cfg.indexURL, cfg.download)
	if err != nil {
		return fmt.Errorf("getting the index: %w", err)
	}
	toc, err := readIndex(indexFile) // the index: a table of contents
	if err != nil {
		return err
	}
	choice, err := tic.ChooseInNetworkFile(toc, cfg.match)
	if err != nil {
		return err
	}

	// When the rate file is saved already, this phase lasts a moment.
	live.SetPhase(api.PhaseDownload)
	opt := cfg.download
	opt.Progress = live.SetProgress
	file, err := ensure(ctx, cfg, choice.URL, opt)
	if err != nil {
		return err
	}
	info, err := os.Stat(file)
	if err != nil {
		return err
	}

	// CMS's names help but aren't required: if they can't be loaded, say so
	// and carry on with the rate file's own names.
	live.SetPhase(api.PhaseCMS)
	ref, cmsErr := loadCMS(cfg)
	if cmsErr != nil {
		log.Printf("CMS's HCPCS names are unavailable: %v", cmsErr)
	}

	live.SetPhase(api.PhaseReading)
	start := time.Now()
	got, err := loadRows(file, ref)
	if err != nil {
		return err
	}
	listed := 0
	for _, r := range got.rows {
		listed += r.Source.ListedTimes
	}
	log.Printf("ready: %d rows from %d listings in %s (%s, version %s, updated %s)",
		len(got.rows), listed, time.Since(start).Round(time.Millisecond), filepath.Base(file), got.head.Version, got.head.LastUpdatedOn)

	src := api.Source{
		IndexURL:             cfg.indexURL,
		IndexUpdated:         toc.LastUpdatedOn,
		ReportingEntityName:  got.head.ReportingEntityName,
		ReportingEntityType:  got.head.ReportingEntityType,
		FileURL:              choice.URL,
		FileVersion:          got.head.Version,
		FileUpdated:          got.head.LastUpdatedOn,
		FileBytes:            info.Size(),
		IssuerNames:          choice.IssuerNames(),
		PlanMarketTypes:      choice.MarketTypes(),
		PlanNames:            choice.PlanNames(),
		ReportingPlanCount:   len(choice.Plans),
		RowCount:             len(got.rows),
		ListingCount:         listed,
		SkippedItemCount:     got.head.Skipped,
		UnlinkedListingCount: got.unlinked,
		MissingRateCount:     got.noRate,
		HCPCSURL:             cfg.hcpcsURL,
	}
	idx := rates.NewIndex(got.rows)
	idx.CodeNames = got.names
	if cmsErr != nil {
		src.HCPCSError = cmsErr.Error()
	} else {
		src.HCPCSFile, src.HCPCSCodeCount, src.HCPCSModifierCount = ref.File, len(ref.Codes), len(ref.Modifiers)
		idx.ModifierMeanings = ref.Modifiers
	}
	live.Ready(idx, src)
	return nil
}

// loadCMS gets CMS's HCPCS file, within cfg.cmsDeadline, and reads its
// official names.
func loadCMS(cfg config) (*cms.Reference, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.cmsDeadline)
	defer cancel()
	file, err := ensure(ctx, cfg, cfg.hcpcsURL, cfg.download)
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		// Ask the deadline itself, not the error: Go's own dial and header
		// timeouts inside a download also count as "deadline exceeded".
		return nil, fmt.Errorf("CMS's file didn't arrive within %s: %w", cfg.cmsDeadline, err)
	}
	if err != nil {
		return nil, err
	}
	ref, err := cms.ReadZip(file)
	if err != nil {
		return nil, damaged(file, err)
	}
	return ref, nil
}

// ensure gets one file through fetch.Ensure. When a refresh fails, the copy
// saved earlier is left as it was, so the error says how to use it.
func ensure(ctx context.Context, cfg config, fileURL string, opt fetch.Options) (string, error) {
	file, err := fetch.Ensure(ctx, cfg.client, fileURL, cfg.dataDir, cfg.refresh, opt)
	if err != nil && cfg.refresh {
		if saved, pathErr := fetch.LocalPath(cfg.dataDir, fileURL); pathErr == nil && exists(saved) {
			return "", fmt.Errorf("%w (the copy saved earlier is unchanged: start without -refresh to use it)", err)
		}
	}
	return file, err
}

// exists reports whether a file is there.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readIndex reads the saved index.
func readIndex(file string) (tic.Index, error) {
	f, err := os.Open(file)
	if err != nil {
		return tic.Index{}, err
	}
	defer f.Close()
	toc, err := tic.ReadIndex(f)
	if err != nil {
		return tic.Index{}, damaged(file, err)
	}
	return toc, nil
}

// loaded is what reading the rate file produced.
type loaded struct {
	rows     []rates.Row
	names    map[string]rates.CodeName // what each code is called, for the code card
	head     tic.Header
	unlinked int // price listings that point at no provider group
	noRate   int // prices the file lists without a rate, left out
}

// loadRows reads the rate file and builds the rows: one price for one
// provider group, with the copies removed. ref, if not nil, is CMS's HCPCS
// file: official names and end dates for HCPCS codes.
func loadRows(file string, ref *cms.Reference) (loaded, error) {
	f, err := os.Open(file)
	if err != nil {
		return loaded{}, err
	}
	defer f.Close()

	b := rates.NewBuilder()
	if ref != nil {
		b.UseOfficialNames(ref.Codes)
		b.UseEndDates(ref.Ended)
	}
	head, err := tic.Parse(f, tic.Handlers{
		ProviderReference: b.AddProviderReference,
		InNetwork:         b.AddInNetwork,
	})
	if err != nil {
		return loaded{}, damaged(file, err)
	}
	rows, unlinked := b.Rows()
	return loaded{rows: rows, names: b.CodeNames(), head: head, unlinked: unlinked, noRate: b.PricesWithoutRate}, nil
}

// damaged explains a saved file that can't be read, and the way out.
func damaged(file string, err error) error {
	return fmt.Errorf("reading the saved %s: %w (if it's damaged, run with -refresh to download it again)", filepath.Base(file), err)
}
