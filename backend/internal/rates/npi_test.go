package rates

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestValidNPI(t *testing.T) {
	for _, c := range []struct {
		npi  string
		want bool
	}{
		{"1234567893", true},  // CMS's own example
		{"1234567890", false}, // the same digits with the wrong check digit
		{"124507857", false},  // 9 digits: the one bad NPI in Centene's file
		{"12345678930", false},
		{"12345A7893", false},
		{"0", false},
	} {
		if got := ValidNPI(c.npi); got != c.want {
			t.Errorf("ValidNPI(%q) = %v, want %v", c.npi, got, c.want)
		}
	}
}

func TestSplitNPIs(t *testing.T) {
	list := func(ns ...string) []json.Number {
		var out []json.Number
		for _, n := range ns {
			out = append(out, json.Number(n))
		}
		return out
	}
	for _, c := range []struct {
		name          string
		in            []json.Number
		valid         []string
		invalid       []string
		wholePractice bool
	}{
		{"named clinicians", list("1234567893"), []string{"1234567893"}, nil, false},
		{"a stray 0 is dropped", list("0", "1234567893"), []string{"1234567893"}, nil, false},
		{"only 0: the whole practice", list("0"), []string{}, nil, true},
		{"a 9-digit NPI is set aside", list("124507857", "1234567893"), []string{"1234567893"}, []string{"124507857"}, false},
	} {
		valid, invalid, whole := splitNPIs(c.in)
		if !slices.Equal(valid, c.valid) || !slices.Equal(invalid, c.invalid) || whole != c.wholePractice {
			t.Errorf("%s: got %v, %v, %v; want %v, %v, %v", c.name, valid, invalid, whole, c.valid, c.invalid, c.wholePractice)
		}
	}
}
