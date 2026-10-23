package rates

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"serif/internal/tic"
)

// descriptionLimit is where the file cuts long descriptions off, mid-word.
const descriptionLimit = 100

// noteName remembers the name of an item's code, for label to use once
// everything has been read.
//
// Revenue-code items often carry a copied procedure name in ALL CAPS, such
// as "EMERGENCY DEPARTMENT VISIT..." under revenue code 0450. The one item
// named in normal capitalization ("Emergency Room General") holds the
// revenue code's real, standard name, so only that one is kept.
func (b *Builder) noteName(item tic.InNetworkItem) {
	key := codeKey(item.BillingCode)
	name := item.Description
	if name == "" {
		name = item.Name
	}
	if item.BillingCodeType == "RC" {
		if _, ok := b.revenueNames[key]; !ok && hasLowercase(name) {
			b.revenueNames[key] = name
		}
		return
	}
	if _, ok := b.procedureNames[key]; !ok {
		b.procedureNames[key] = name
		b.shortNames[key] = item.Name // such as "OFFICE/OUTPATIENT VISIT EST"
	}
}

// UseOfficialNames gives HCPCS codes their full official names from CMS's
// HCPCS file. They replace the file's own text, which is often cut off at
// 100 characters. CPT names stay the file's: CMS's file has none, since the
// AMA owns them.
func (b *Builder) UseOfficialNames(names map[string]string) {
	b.officialNames = names
}

// UseEndDates gives discontinued HCPCS codes CMS's last date for them, from
// CMS's HCPCS file, so rows can say so (J0640: "2026-09-30").
func (b *Builder) UseEndDates(ended map[string]string) {
	b.ended = ended
}

// CodeName is what a code is called: its short name, and its full name as
// rows show it.
type CodeName struct {
	Short, Full string
}

// CodeNames returns the names of each code that is an item of its own, by
// code key. A CPT or HCPCS code's short name is the file's, and its full
// name is CMS's official one or the file's description. A revenue code's
// standard name serves as both; one with no standard name is left out.
func (b *Builder) CodeNames() map[string]CodeName {
	names := map[string]CodeName{}
	for key, short := range b.shortNames {
		names[key] = CodeName{Short: short, Full: b.nameFor(key)}
	}
	for key, name := range b.revenueNames {
		names[key] = CodeName{Short: name, Full: name}
	}
	return names
}

// label is the name shown next to a row's code.
func (b *Builder) label(r Row) string {
	key := codeKey(r.BillingCode)
	if r.BillingCodeType == "RC" {
		if name, ok := b.revenueNames[key]; ok {
			return name
		}
		return "" // the file never names it properly; the page says so in its own words
	}
	return b.nameFor(key)
}

// procedureShape is what a CPT or HCPCS code looks like: five digits, four
// digits and a letter (0075T), or a letter and four digits (J1100).
var procedureShape = regexp.MustCompile(`^([0-9]{4}[0-9A-Z]|[A-Z][0-9]{4})$`)

// procedure pulls the procedure out of a hospital row's side field. The file
// lists the hospital's fee under a revenue code, such as 0490 (outpatient
// surgery), and puts the procedure, such as 27447 (a knee replacement), in
// additional_information. Its name comes from nameFor, and is empty when
// neither CMS nor the file describes it.
func (b *Builder) procedure(r Row) (code, label string) {
	side := strings.ToUpper(strings.TrimSpace(r.AdditionalInfo))
	if r.BillingCodeType != "RC" || !procedureShape.MatchString(side) {
		return "", ""
	}
	return side, b.nameFor(codeKey(side))
}

// nameFor names a CPT or HCPCS code: CMS's official name when there is one,
// otherwise the file's own description.
func (b *Builder) nameFor(key string) string {
	if name, ok := b.officialNames[key]; ok {
		return name
	}
	return withCutMark(b.procedureNames[key])
}

// withCutMark adds "…" to a name the file may have cut off: one 99 or 100
// characters long. The file cuts long names at 100, and where the cut fell
// on a space, 99 are left (99291 stops at "...PATIENT; FIRST"). A few of the
// 43 names that are exactly 99 characters long are whole (73221) and get
// the mark anyway, since showing a cut name as whole is the worse mistake.
func withCutMark(name string) string {
	if utf8.RuneCountInString(name) >= descriptionLimit-1 {
		return name + "…"
	}
	return name
}

func hasLowercase(s string) bool {
	return strings.IndexFunc(s, unicode.IsLower) >= 0
}
