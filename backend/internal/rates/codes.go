package rates

import "strings"

// codeKey is how codes are compared: trimmed, in capitals and without
// leading zeros. So "0450", "450" and " 450 " all match, and "j1100" finds
// J1100. Typed searches get the same treatment.
func codeKey(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	key := strings.TrimLeft(code, "0")
	if key == "" && code != "" {
		return "0" // a code that was all zeros
	}
	return key
}

// standardWidth is how many characters a code has on a bill, for the types
// whose leading zeros the file drops.
var standardWidth = map[string]int{"RC": 4, "CPT": 5}

// displayCode pads a code back to its standard width, so revenue code 450
// shows as 0450 and CPT 731 as 00731. It goes by the code's type, not its
// length: 731 (CPT) and 450 (RC) are both three digits.
func displayCode(codeType, code string) string {
	width := standardWidth[codeType]
	if len(code) >= width {
		return code
	}
	return strings.Repeat("0", width-len(code)) + code
}
