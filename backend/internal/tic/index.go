// Package tic reads the files that the CMS Transparency in Coverage (TiC)
// rule makes insurers publish.
package tic

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"slices"
	"strings"
)

// Index is the table-of-contents file. It lists groups of plans, and each
// group points to the in-network price files those plans share.
type Index struct {
	ReportingEntityName string               `json:"reporting_entity_name"`
	LastUpdatedOn       string               `json:"last_updated_on"`
	ReportingStructure  []ReportingStructure `json:"reporting_structure"`
}

// ReportingStructure is one group of plans and the files they share.
type ReportingStructure struct {
	ReportingPlans []Plan     `json:"reporting_plans"`
	InNetworkFiles []FileLink `json:"in_network_files"`
}

// Plan is one row of reporting_plans. The same plan name can appear on
// several rows, under different plan IDs.
type Plan struct {
	PlanName       string `json:"plan_name"`
	PlanIDType     string `json:"plan_id_type"`
	PlanID         string `json:"plan_id"`
	PlanMarketType string `json:"plan_market_type"`
	IssuerName     string `json:"issuer_name"`
}

// FileLink points to one file.
type FileLink struct {
	Description string `json:"description"`
	Location    string `json:"location"`
}

// Choice is the in-network file we load and the plans that use it.
type Choice struct {
	URL      string
	FileName string
	Plans    []Plan
}

// ReadIndex decodes an index file.
func ReadIndex(r io.Reader) (Index, error) {
	var idx Index
	if err := json.NewDecoder(r).Decode(&idx); err != nil {
		return Index{}, fmt.Errorf("reading the index: %w", err)
	}
	return idx, nil
}

// ChooseInNetworkFile finds the in-network file whose file name contains
// match. It goes by name, not position, because the order of the groups
// can change from one month's index to the next. No match, or two
// different matching files, is an error.
func ChooseInNetworkFile(idx Index, match string) (Choice, error) {
	var choice Choice
	var names []string // every file name, for the error message
	for _, group := range idx.ReportingStructure {
		for _, file := range group.InNetworkFiles {
			name := fileName(file.Location)
			names = append(names, name)
			if !strings.Contains(name, match) {
				continue
			}
			if choice.URL != "" && choice.URL != file.Location {
				return Choice{}, fmt.Errorf("more than one in-network file matches %q: %s and %s", match, choice.FileName, name)
			}
			// The same file can be listed under several groups, so keep every group's plans.
			choice.URL = file.Location
			choice.FileName = name
			choice.Plans = append(choice.Plans, group.ReportingPlans...)
		}
	}
	if choice.URL == "" {
		return Choice{}, fmt.Errorf("no in-network file matches %q; the index lists %s", match, strings.Join(names, ", "))
	}
	return choice, nil
}

// PlanNames returns each plan name once, sorted.
func (c Choice) PlanNames() []string {
	return c.each(func(p Plan) string { return p.PlanName })
}

// IssuerNames returns each issuer's name once, sorted, such as "Fidelis Care".
func (c Choice) IssuerNames() []string {
	return c.each(func(p Plan) string { return p.IssuerName })
}

// MarketTypes returns each plan market type once, sorted, such as "individual".
func (c Choice) MarketTypes() []string {
	return c.each(func(p Plan) string { return p.PlanMarketType })
}

// each returns one field of the plans, each value once, sorted, leaving out
// empty ones: an empty list, not nil, when there are none.
func (c Choice) each(field func(Plan) string) []string {
	values := []string{}
	for _, p := range c.Plans {
		if v := field(p); v != "" && !slices.Contains(values, v) {
			values = append(values, v)
		}
	}
	slices.Sort(values)
	return values
}

// fileName returns the last part of a URL's path, such as
// "2026-08-31_centene-management-company-llc_fidelis-ex_in-network.json".
func fileName(location string) string {
	u, err := url.Parse(location)
	if err != nil {
		return location
	}
	return path.Base(u.Path)
}
