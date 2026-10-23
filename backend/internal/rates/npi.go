package rates

import "encoding/json"

// ValidNPI reports whether s can be a real NPI: 10 digits whose last digit
// checks out. CMS computes that check digit with the Luhn method (the one
// credit card numbers use) over 80840 followed by the first 9 digits.
func ValidNPI(s string) bool {
	if len(s) != 10 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	digits := "80840" + s[:9]
	sum := 0
	for pos := 0; pos < len(digits); pos++ { // counted from the right
		d := int(digits[len(digits)-1-pos] - '0')
		if pos%2 == 0 { // every other digit, starting with the rightmost, is doubled
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	check := (10 - sum%10) % 10
	return check == int(s[9]-'0')
}

// splitNPIs sorts a provider group's NPI list into valid and invalid NPIs
// and drops the 0 placeholder. A list that held only 0 means the contract
// is with the whole practice (its tax ID), not with named clinicians.
func splitNPIs(list []json.Number) (valid, invalid []string, wholePractice bool) {
	valid, invalid = []string{}, []string{} // never nil, so the API sends []
	sawZero := false
	for _, n := range list {
		s := n.String()
		switch {
		case s == "0":
			sawZero = true
		case ValidNPI(s):
			valid = append(valid, s)
		default:
			invalid = append(invalid, s)
		}
	}
	return valid, invalid, sawZero && len(valid) == 0 && len(invalid) == 0
}
