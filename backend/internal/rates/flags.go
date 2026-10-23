package rates

import "strings"

// The flags a row can carry. Flagged rows are kept and shown, with a label,
// but left out of the summary numbers.
const (
	// FlagPlaceholderProvider marks a made-up tax ID (01-2345678) or a group
	// named "Dummy Group".
	FlagPlaceholderProvider = "placeholder_provider"
	// FlagOneDollarOrLess marks a CPT price of $1 or less, unusually low for a
	// procedure, though some cheap add-ons may be real. HCPCS codes are left
	// out because they're often priced per unit (lubricant at $0.35 an ounce).
	FlagOneDollarOrLess = "one_dollar_or_less"
	// FlagSeveralPrices marks conflicting prices in a search for one doctor's
	// NPI: the file lists more than one amount for that doctor at the same
	// organization, with the same place, modifier and every other detail,
	// and doesn't say which one applies.
	FlagSeveralPrices = "several_prices"
)

// placeholderTIN is the made-up tax ID in the file: 01-2345678.
const placeholderTIN = "012345678"

// flagsFor returns the flags that apply to a row: an empty list, not nil,
// when none do.
func flagsFor(r Row) []string {
	flags := []string{}
	if einDigits(r.TIN) == placeholderTIN || strings.HasPrefix(strings.ToLower(r.BusinessName), "dummy group") {
		flags = append(flags, FlagPlaceholderProvider)
	}
	if r.BillingCodeType == "CPT" && r.Rate <= 1 {
		flags = append(flags, FlagOneDollarOrLess)
	}
	return flags
}

// einDigits keeps only the digits of a tax ID, so "06-1494923" and
// "061494923" compare equal. It stays text, because 634 of the file's EINs
// start with 0, which a number would lose.
func einDigits(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}
