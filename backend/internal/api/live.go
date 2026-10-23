package api

import (
	"fmt"
	"sync"

	"serif/internal/rates"
)

// The phases of loading, in order. It ends in ready or failed.
const (
	PhaseStarting = "starting"
	PhaseIndex    = "fetching_index"
	PhaseDownload = "downloading"
	PhaseCMS      = "fetching_cms_names"
	PhaseReading  = "reading"
	PhaseReady    = "ready"
	PhaseFailed   = "failed"
)

// Status is what /api/status reports: where loading is, and once the data is
// ready, where it came from.
type Status struct {
	Phase      string  `json:"phase"`
	Error      string  `json:"error,omitempty"`
	BytesDone  int64   `json:"bytes_done,omitempty"`  // while downloading
	BytesTotal int64   `json:"bytes_total,omitempty"` // -1 when the server doesn't say
	Source     *Source `json:"source,omitempty"`      // once ready
}

// Source says where the data came from and what loading counted. The page
// uses it for its ready line, the saved file's name in the jq commands, and
// whether CMS's names loaded. A count's name ends in _count; a name like
// plan_names is the list itself.
type Source struct {
	IndexURL             string   `json:"index_url"`
	IndexUpdated         string   `json:"index_last_updated_on"`
	ReportingEntityName  string   `json:"reporting_entity_name"`
	ReportingEntityType  string   `json:"reporting_entity_type"` // who posted the file, such as "Third Party Administrator"
	FileURL              string   `json:"file_url"`
	FileVersion          string   `json:"file_version"`
	FileUpdated          string   `json:"file_last_updated_on"`
	FileBytes            int64    `json:"file_bytes"`
	IssuerNames          []string `json:"issuer_names"`           // the plans' issuers, each once, such as "Fidelis Care"
	PlanMarketTypes      []string `json:"plan_market_types"`      // each once, such as "individual"
	PlanNames            []string `json:"plan_names"`             // each plan name once
	ReportingPlanCount   int      `json:"reporting_plan_count"`   // the index's reporting_plans entries for this file; some share a name
	RowCount             int      `json:"row_count"`              // one price for one provider group, copies removed
	ListingCount         int      `json:"listing_count"`          // price-for-group listings, copies included
	SkippedItemCount     int      `json:"skipped_item_count"`     // in_network items and provider_references entries set aside: null, or a value of the wrong kind
	UnlinkedListingCount int      `json:"unlinked_listing_count"` // listings that point at no provider group
	MissingRateCount     int      `json:"missing_rate_count"`     // prices listed with no rate, left out

	// CMS's HCPCS file: official names for HCPCS codes and modifiers. It's
	// optional, so HCPCSError says why when it couldn't be used.
	HCPCSURL           string `json:"hcpcs_url"`
	HCPCSFile          string `json:"hcpcs_file,omitempty"`
	HCPCSCodeCount     int    `json:"hcpcs_code_count"`
	HCPCSModifierCount int    `json:"hcpcs_modifier_count"`
	HCPCSError         string `json:"hcpcs_error,omitempty"`
}

// Live holds the loading status and, once ready, the index. The loader
// writes to it in the background while requests read it, so a lock keeps a
// request from seeing a half-written update.
type Live struct {
	mu     sync.Mutex
	status Status
	index  *rates.Index
}

// NewLive returns a Live that's just starting.
func NewLive() *Live {
	return &Live{status: Status{Phase: PhaseStarting}}
}

// SetPhase moves loading on to its next phase. Download progress only
// belongs to the downloading phase, so it's cleared on the way out.
func (l *Live) SetPhase(phase string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.status.Phase = phase
	if phase != PhaseDownload {
		l.status.BytesDone, l.status.BytesTotal = 0, 0
	}
}

// SetProgress records how much of the rate file has downloaded.
func (l *Live) SetProgress(done, total int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.status.BytesDone, l.status.BytesTotal = done, total
}

// Ready hands over the finished index and where the data came from.
func (l *Live) Ready(idx *rates.Index, src Source) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.index = idx
	l.status = Status{Phase: PhaseReady, Source: &src}
}

// Fail records why loading stopped. Like Ready, it replaces the whole
// status, so no download progress is left over.
func (l *Live) Fail(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.status = Status{Phase: PhaseFailed, Error: err.Error()}
}

// Snapshot returns the status, and the index once it's ready (nil before).
func (l *Live) Snapshot() (Status, *rates.Index) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.status, l.index
}

// describe says where loading is, in words for the page.
func describe(st Status) string {
	switch st.Phase {
	case PhaseIndex:
		return "fetching the index"
	case PhaseDownload:
		if st.BytesTotal > 0 {
			return fmt.Sprintf("downloading the rate file (%d%%, %.0f of %.0f MB)",
				st.BytesDone*100/st.BytesTotal, float64(st.BytesDone)/1e6, float64(st.BytesTotal)/1e6)
		}
		return "downloading the rate file"
	case PhaseCMS:
		return "loading CMS's names for HCPCS codes"
	case PhaseReading:
		return "reading the rate file"
	}
	return "starting"
}
