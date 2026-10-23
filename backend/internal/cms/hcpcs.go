// Package cms reads CMS's official HCPCS file: the public list of HCPCS
// Level II codes (drugs, supplies, equipment, ambulance) and modifiers.
//
// The file comes from cms.gov as a ZIP ("Alpha-Numeric HCPCS File"). It has
// no CPT (Level I) codes or number modifiers: their text belongs to the AMA.
package cms

import (
	"archive/zip"
	"bufio"
	"fmt"
	"strings"
)

// Reference holds the official names from one CMS HCPCS file.
type Reference struct {
	File      string            // the text file inside the ZIP it was read from
	Codes     map[string]string // HCPCS code, such as "J1100" -> its long description
	Modifiers map[string]string // modifier, such as "GT" -> its long description
	Ended     map[string]string // a discontinued code, such as "J0640" -> the last date Medicare providers may use it ("2026-09-30")
}

// ReadZip opens a CMS HCPCS ZIP and reads its main text file, the one named
// like HCPC2026_OCT_ANWEB_09232026.txt.
func ReadZip(path string) (*Reference, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()

	for _, f := range zr.File {
		name := f.Name
		if !strings.Contains(name, "ANWEB") || !strings.HasSuffix(strings.ToLower(name), ".txt") {
			continue
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer r.Close()
		ref, err := read(bufio.NewScanner(r))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", name, err)
		}
		ref.File = name
		return ref, nil
	}
	return nil, fmt.Errorf("%s has no ANWEB .txt file", path)
}

// read parses the file's fixed-width lines. Each line is one record, and the
// record type sits in column 11:
//
//	3: a code's first line     4: more of that code's description
//	7: a modifier's first line 8: more of that modifier's description
//
// The code is in columns 1-5 (a modifier in 4-5), the long description in
// columns 12-91, and a code's termination date in columns 285-292.
func read(sc *bufio.Scanner) (*Reference, error) {
	ref := &Reference{Codes: map[string]string{}, Modifiers: map[string]string{}, Ended: map[string]string{}}
	var lastCode, lastModifier string
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if len(line) < 12 {
			continue // a blank or short line carries no record
		}
		text := strings.TrimSpace(line[11:min(len(line), 91)])
		switch line[10] {
		case '3':
			lastCode = strings.TrimSpace(line[0:5])
			ref.Codes[lastCode] = text
			if date, ok := termination(line); ok {
				ref.Ended[lastCode] = date
			}
		case '4':
			ref.Codes[lastCode] = join(ref.Codes[lastCode], text)
		case '7':
			lastModifier = strings.TrimSpace(line[3:5])
			ref.Modifiers[lastModifier] = text
		case '8':
			ref.Modifiers[lastModifier] = join(ref.Modifiers[lastModifier], text)
		}
	}
	return ref, sc.Err()
}

// termination returns a code's termination date as YYYY-MM-DD. CMS's record
// layout (HCPC2026_recordlayout.txt, in the same ZIP) puts it in columns
// 285-292: the "last date for which a procedure or modifier code may be used
// by Medicare providers". It's blank for a code still in use.
func termination(line string) (string, bool) {
	if len(line) < 292 {
		return "", false
	}
	d := line[284:292]
	for _, c := range d {
		if c < '0' || c > '9' {
			return "", false // blank: still in use
		}
	}
	return d[:4] + "-" + d[4:6] + "-" + d[6:], true
}

func join(a, b string) string {
	if b == "" {
		return a
	}
	return a + " " + b
}
