package tic

import (
	"encoding/json"
	"fmt"
	"io"
)

// An in-network rate file has a small header and two big lists. The prices
// in in_network point at the provider groups in provider_references by number.

// Header is the part of the file outside the two big lists, plus a count of
// the items that had to be set aside.
type Header struct {
	ReportingEntityName string
	ReportingEntityType string
	LastUpdatedOn       string
	Version             string
	Skipped             int // list elements set aside (in_network items, provider_references entries): null, or a value of the wrong kind, such as a price written as text
}

// ProviderReference is one numbered list of provider groups.
type ProviderReference struct {
	ProviderGroupID json.Number     `json:"provider_group_id"`
	NetworkName     []string        `json:"network_name"`
	ProviderGroups  []ProviderGroup `json:"provider_groups"`
	Location        string          `json:"location"` // a list kept in another file: not fetched, so its prices count as unlinked
}

// ProviderGroup is one organization (its tax ID) and its clinicians (NPIs).
type ProviderGroup struct {
	NPI []json.Number `json:"npi"`
	TIN TIN           `json:"tin"`
}

// TIN is a tax ID: usually an EIN, or an NPI for a clinician who bills
// under their own.
type TIN struct {
	Type         string `json:"type"`
	Value        string `json:"value"`
	BusinessName string `json:"business_name"`
}

// InNetworkItem is one billing code and every price negotiated for it.
type InNetworkItem struct {
	NegotiationArrangement string           `json:"negotiation_arrangement"`
	Name                   string           `json:"name"`
	BillingCodeType        string           `json:"billing_code_type"`
	BillingCode            string           `json:"billing_code"`
	Description            string           `json:"description"`
	NegotiatedRates        []NegotiatedRate `json:"negotiated_rates"`
}

// NegotiatedRate ties a set of prices to the provider groups they apply to.
// This file points at provider_references by number. Older files list the
// groups inline, in ProviderGroups, which is read but not used yet: such
// prices count as unlinked.
type NegotiatedRate struct {
	ProviderReferences []json.Number     `json:"provider_references"`
	ProviderGroups     []ProviderGroup   `json:"provider_groups"`
	NegotiatedPrices   []NegotiatedPrice `json:"negotiated_prices"`
}

// NegotiatedPrice is one price and the conditions it applies under.
type NegotiatedPrice struct {
	NegotiatedType        string   `json:"negotiated_type"`
	NegotiatedRate        *float64 `json:"negotiated_rate"` // nil when the file leaves it out or writes null
	ExpirationDate        string   `json:"expiration_date"`
	ServiceCode           []string `json:"service_code"`
	BillingClass          string   `json:"billing_class"`
	Setting               string   `json:"setting"`
	BillingCodeModifier   []string `json:"billing_code_modifier"`
	AdditionalInformation string   `json:"additional_information"`
}

// Handlers receive the two big lists one item at a time, with each item's
// position in its list. Skipped items still count toward the position, so
// a row can point back to the exact spot in the file. Either can be nil.
type Handlers struct {
	ProviderReference func(i int, pr ProviderReference)
	InNetwork         func(i int, item InNetworkItem)
}

// Parse reads an in-network file one item at a time, so the whole file is
// never in memory at once. The top-level keys can come in any order.
func Parse(r io.Reader, h Handlers) (Header, error) {
	var head Header
	dec := json.NewDecoder(r)
	if err := expectDelim(dec, '{'); err != nil {
		return head, err
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return head, err
		}
		key, _ := tok.(string)
		switch key {
		case "reporting_entity_name":
			err = dec.Decode(&head.ReportingEntityName)
		case "reporting_entity_type":
			err = dec.Decode(&head.ReportingEntityType)
		case "last_updated_on":
			err = dec.Decode(&head.LastUpdatedOn)
		case "version":
			err = dec.Decode(&head.Version)
		case "provider_references":
			err = eachElement(dec, h.ProviderReference, &head.Skipped)
		case "in_network":
			err = eachElement(dec, h.InNetwork, &head.Skipped)
		default:
			var skip json.RawMessage // a key we don't use
			err = dec.Decode(&skip)
		}
		if err != nil {
			return head, fmt.Errorf("reading %s: %w", key, err)
		}
	}
	return head, expectDelim(dec, '}')
}

// eachElement reads a JSON list one element at a time and hands each one to
// fn. T stands for the element's type, such as InNetworkItem.
//
// Each element is read as raw JSON first, then filled in. If filling it in
// fails, say a price written as text or an NPI that isn't a number, that one
// element is set aside and counted in skipped, and the rest of the list is
// still read. (Go would leave the bad value empty, so a price written as text
// would show up as $0.00.) A null element is set aside the same way, since
// Go would fill in nothing and report no error. Doing it in two steps works
// the same on every Go version. Only a broken file, such as one cut off
// partway, stops the read.
func eachElement[T any](dec *json.Decoder, fn func(int, T), skipped *int) error {
	if err := expectDelim(dec, '['); err != nil {
		return err
	}
	for i := 0; dec.More(); i++ {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return err // the file itself is broken
		}
		var v T
		if string(raw) == "null" || json.Unmarshal(raw, &v) != nil {
			*skipped++ // null, or a value of the wrong kind: set this element aside
			continue
		}
		if fn != nil {
			fn(i, v)
		}
	}
	return expectDelim(dec, ']')
}

// expectDelim reads the next token and checks that it's want, such as '{'.
func expectDelim(dec *json.Decoder, want json.Delim) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || d != want {
		return fmt.Errorf("expected %q, found %v", want, tok)
	}
	return nil
}
