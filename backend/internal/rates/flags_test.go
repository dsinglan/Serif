package rates

import (
	"slices"
	"testing"
)

func TestFlags(t *testing.T) {
	for _, c := range []struct {
		name string
		row  Row
		want []string
	}{
		{"an ordinary row", Row{BillingCodeType: "CPT", Rate: 75, TIN: "20-3910419", BusinessName: "Groth"}, nil},
		{"the made-up tax ID", Row{BillingCodeType: "CPT", Rate: 75, TIN: "01-2345678", BusinessName: "Dummy Group - NYNM"},
			[]string{FlagPlaceholderProvider}},
		{"a placeholder name on a real tax ID", Row{BillingCodeType: "CPT", Rate: 75, TIN: "11-3322995", BusinessName: "Dummy Group - NYNM"},
			[]string{FlagPlaceholderProvider}},
		{"CPT at $0.81", Row{BillingCodeType: "CPT", Rate: 0.81, TIN: "20-3910419"}, []string{FlagOneDollarOrLess}},
		{"CPT at exactly $1", Row{BillingCodeType: "CPT", Rate: 1, TIN: "20-3910419"}, []string{FlagOneDollarOrLess}},
		{"CPT at $1.01", Row{BillingCodeType: "CPT", Rate: 1.01, TIN: "20-3910419"}, nil},
		{"HCPCS per unit at $0.35", Row{BillingCodeType: "HCPCS", Rate: 0.35, TIN: "20-3910419"}, nil},
	} {
		got := flagsFor(c.row)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: flags = %v, want %v", c.name, got, c.want)
		}
		if got == nil {
			t.Errorf("%s: flags = nil, want [] so the API always sends a list", c.name)
		}
	}
}

func TestEINDigits(t *testing.T) {
	if got := einDigits("06-1494923"); got != "061494923" {
		t.Errorf(`einDigits("06-1494923") = %q, want "061494923" (leading zero kept)`, got)
	}
}
