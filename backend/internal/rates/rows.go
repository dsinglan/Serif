// Package rates turns the parsed rate file into rows: one price for one
// provider group, with the copies removed.
package rates

import (
	"slices"
	"strings"

	"serif/internal/tic"
)

// Row is one negotiated price for one provider group. The Builder never
// leaves a list nil, so the API sends [] for an empty one, never null.
type Row struct {
	// The service
	BillingCodeType string `json:"billing_code_type"`         // CPT, HCPCS or RC, as in the file
	BillingCode     string `json:"billing_code"`              // as in the file, such as "450"
	DisplayCode     string `json:"display_code"`              // at its standard width, such as "0450"
	Label           string `json:"label"`                     // the code's name, such as "Emergency Room General"; "" when the file has no proper one
	ProcedureCode   string `json:"procedure_code,omitempty"`  // hospital rows: the procedure from the side field, such as "27447"
	ProcedureLabel  string `json:"procedure_label,omitempty"` // its name: CMS's official one (a HCPCS code, such as H0020), or the file's when it lists the code as its own item
	// ProcedureShortName is the file's short name for the procedure, such as
	// "EMERGENCY DEPT VISIT", when the file lists it as its own item.
	ProcedureShortName string `json:"procedure_short_name,omitempty"`
	Arrangement        string `json:"negotiation_arrangement"`

	// CMS's last date for a discontinued HCPCS code: the row's own code, or
	// the procedure in its side field (J0640: "2026-09-30").
	Discontinued          string `json:"discontinued,omitempty"`
	ProcedureDiscontinued string `json:"procedure_discontinued,omitempty"`

	// The price
	NegotiatedType string   `json:"negotiated_type"`
	Rate           float64  `json:"negotiated_rate"`
	BillingClass   string   `json:"billing_class"` // professional (a doctor's fee) or institutional (a facility's fee)
	Setting        string   `json:"setting"`
	ServiceCodes   []string `json:"service_code"` // places of service, such as "11" for an office
	Modifiers      []string `json:"billing_code_modifier"`
	Expiration     string   `json:"expiration_date"`
	AdditionalInfo string   `json:"additional_information,omitempty"`

	// The provider group
	TINType      string   `json:"tin_type"` // ein or npi
	TIN          string   `json:"tin"`
	BusinessName string   `json:"business_name"`
	NPIs         []string `json:"npis"`             // valid 10-digit NPIs; the 0 placeholder is dropped
	InvalidNPIs  []string `json:"invalid_npis"`     // listed, but can't be real NPIs (such as 9 digits)
	TINLevel     bool     `json:"tin_level"`        // the file listed only NPI 0: the contract is with the whole practice
	Groups       int      `json:"groups,omitempty"` // NPI searches only: how many groups list this doctor at this exact price

	Flags  []string `json:"flags"` // placeholder_provider, one_dollar_or_less, several_prices (see flags.go)
	Source Source   `json:"source"`
}

// Source points at the first place in the file where this row was listed.
type Source struct {
	InNetworkIndex       int    `json:"in_network_index"`        // in_network[i]
	NegotiatedRatesIndex int    `json:"negotiated_rates_index"`  // .negotiated_rates[j]
	NegotiatedPriceIndex int    `json:"negotiated_prices_index"` // .negotiated_prices[k]
	ProviderGroupID      string `json:"provider_group_id"`       // the provider reference the price points at, such as "1115"
	ProviderGroupIndex   int    `json:"provider_groups_index"`   // .provider_groups[g] in that provider reference
	ListedTimes          int    `json:"listed_times"`            // how many identical listings this row stands for
}

// Builder collects the two big lists as Parse hands them over, then joins
// them into rows. The lists can come in either order, so nothing is looked
// up until both have been read.
type Builder struct {
	groups      map[string][]tic.ProviderGroup // provider_group_id -> that provider reference's groups
	listings    []listing                      // each distinct price for each provider reference, in file order
	seen        map[listingKey]int             // where each listing is in listings
	noReference int                            // prices that point at no provider reference at all

	// PricesWithoutRate counts the prices the file lists with no rate (null
	// or missing). They're left out rather than shown as $0.00.
	PricesWithoutRate int

	procedureNames map[string]string // CPT and HCPCS code key -> the file's description
	shortNames     map[string]string // CPT and HCPCS code key -> the file's short name
	revenueNames   map[string]string // revenue code key -> its normal-capitalization name
	officialNames  map[string]string // HCPCS code -> CMS's official name, when CMS's file was read
	ended          map[string]string // HCPCS code -> CMS's last date for it, when CMS discontinued it
}

// listing is one distinct price for one provider reference, before the
// lookup.
type listing struct {
	row   Row    // the price fields; the provider fields are filled in by Rows
	ref   string // the provider_group_id it points at
	times int    // how many identical listings were seen
}

// NewBuilder returns an empty Builder.
func NewBuilder() *Builder {
	return &Builder{
		groups:         map[string][]tic.ProviderGroup{},
		seen:           map[listingKey]int{},
		procedureNames: map[string]string{},
		shortNames:     map[string]string{},
		revenueNames:   map[string]string{},
	}
}

// AddProviderReference keeps one provider reference's groups under its
// provider_group_id.
func (b *Builder) AddProviderReference(_ int, pr tic.ProviderReference) {
	b.groups[pr.ProviderGroupID.String()] = pr.ProviderGroups
}

// AddInNetwork records each of a code's prices once for every provider
// reference it points at. An identical price for the same provider
// reference, such as a hospital menu printed again under another item, only
// adds to the first one's count. A price with no rate is counted once, as
// missing a rate, even if it points at no provider reference either; a price
// with a rate but no provider reference at all is counted as unlinked.
// (Provider references that turn out to be missing or empty are counted
// later, in Rows.)
func (b *Builder) AddInNetwork(i int, item tic.InNetworkItem) {
	b.noteName(item)
	for j, rate := range item.NegotiatedRates {
		for k, price := range rate.NegotiatedPrices {
			if price.NegotiatedRate == nil {
				b.PricesWithoutRate++
				continue
			}
			if len(rate.ProviderReferences) == 0 {
				b.noReference++
				continue
			}
			row := priceRow(item, price)
			row.Source = Source{InNetworkIndex: i, NegotiatedRatesIndex: j, NegotiatedPriceIndex: k}
			for _, ref := range rate.ProviderReferences {
				key := listingKey{priceKeyOf(row), ref.String()}
				if n, ok := b.seen[key]; ok {
					b.listings[n].times++
					continue
				}
				b.seen[key] = len(b.listings)
				b.listings = append(b.listings, listing{row: row, ref: ref.String(), times: 1})
			}
		}
	}
}

// Rows looks up each listing's provider reference and makes one row for
// every provider group in it. Rows identical in everything shown are kept
// once, because two provider references can hold the same practice and
// doctors. unlinked counts the price listings that pointed at a missing or
// empty provider reference, or at none.
func (b *Builder) Rows() (rows []Row, unlinked int) {
	rows = []Row{}
	seen := map[rowKey]int{}
	unlinked = b.noReference
	for _, l := range b.listings {
		groups := b.groups[l.ref]
		if len(groups) == 0 {
			unlinked += l.times
			continue
		}
		for g, group := range groups {
			row := l.row
			row.TINType = group.TIN.Type
			row.TIN = group.TIN.Value
			row.BusinessName = group.TIN.BusinessName
			row.NPIs, row.InvalidNPIs, row.TINLevel = splitNPIs(group.NPI)
			row.Source.ProviderGroupID = l.ref
			row.Source.ProviderGroupIndex = g

			key := rowKeyOf(row)
			if n, ok := seen[key]; ok {
				rows[n].Source.ListedTimes += l.times // a copy: count it on the first one
				continue
			}
			row.Source.ListedTimes = l.times
			seen[key] = len(rows)
			rows = append(rows, row)
		}
	}
	for i := range rows {
		rows[i].Label = b.label(rows[i])
		rows[i].ProcedureCode, rows[i].ProcedureLabel = b.procedure(rows[i])
		rows[i].Discontinued = b.ended[codeKey(rows[i].BillingCode)]
		if rows[i].ProcedureCode != "" {
			rows[i].ProcedureShortName = b.shortNames[codeKey(rows[i].ProcedureCode)]
			rows[i].ProcedureDiscontinued = b.ended[rows[i].ProcedureCode]
		}
		rows[i].Flags = flagsFor(rows[i])
	}
	return rows, unlinked
}

// priceKey is everything about a price except who it's for. Two prices with
// the same key are the same price.
type priceKey struct {
	codeType, code, arrangement, negotiatedType string
	rate                                        float64
	billingClass, setting, serviceCodes         string
	modifiers, expiration, additionalInfo       string
}

// listingKey is a price plus the provider reference it points at.
type listingKey struct {
	price priceKey
	ref   string
}

// rowKey is everything shown about a row. Two rows with the same key are
// copies of each other.
type rowKey struct {
	price                             priceKey
	tinType, tin, name, npis, invalid string
	tinLevel                          bool
}

func priceKeyOf(r Row) priceKey {
	return priceKey{
		codeType: r.BillingCodeType, code: r.BillingCode, arrangement: r.Arrangement,
		negotiatedType: r.NegotiatedType, rate: r.Rate,
		billingClass: r.BillingClass, setting: r.Setting, serviceCodes: strings.Join(r.ServiceCodes, ","),
		modifiers: strings.Join(r.Modifiers, ","), expiration: r.Expiration, additionalInfo: r.AdditionalInfo,
	}
}

func rowKeyOf(r Row) rowKey {
	return rowKey{priceKeyOf(r), r.TINType, r.TIN, r.BusinessName,
		strings.Join(sorted(r.NPIs), ","), strings.Join(sorted(r.InvalidNPIs), ","), r.TINLevel}
}

// priceRow copies a price's fields into a row. Places of service and
// modifiers are sorted, so ["10", "02"] and ["02", "10"] count as the same.
func priceRow(item tic.InNetworkItem, p tic.NegotiatedPrice) Row {
	return Row{
		BillingCodeType: item.BillingCodeType,
		BillingCode:     item.BillingCode,
		DisplayCode:     displayCode(item.BillingCodeType, item.BillingCode),
		Arrangement:     item.NegotiationArrangement,
		NegotiatedType:  p.NegotiatedType,
		Rate:            *p.NegotiatedRate, // AddInNetwork has already left out prices without one
		BillingClass:    p.BillingClass,
		Setting:         p.Setting,
		ServiceCodes:    sorted(p.ServiceCode),
		Modifiers:       sorted(p.BillingCodeModifier),
		Expiration:      p.ExpirationDate,
		AdditionalInfo:  p.AdditionalInformation,
	}
}

// sorted returns a sorted copy, never nil, so an empty list shows as [] in
// the API rather than null.
func sorted(s []string) []string {
	out := append([]string{}, s...)
	slices.Sort(out)
	return out
}
