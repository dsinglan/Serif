package cms

import (
	"archive/zip"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// record lays out one fixed-width line: the code (or a modifier in columns
// 4-5), a sequence number, the record type and the description.
func record(code, kind, text string) string {
	return code + "00100" + kind + text + "\r\n"
}

// writeZip makes a small CMS-style ZIP holding one ANWEB text file.
func writeZip(t *testing.T, lines string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hcpcs.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("HCPC2026_OCT_ANWEB_09232026.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(lines)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadZip(t *testing.T) {
	lines := record("J1100", "3", "Injection, dexamethasone sodium phosphate, 1 mg") +
		record("J0640", "3", fmt.Sprintf("%-273s20260930D", "Injection, leucovorin calcium, per 50 mg")) + // discontinued: a date in columns 285-292

		record("A0427", "3", "Ambulance service, advanced life support, emergency transport, level 1") +
		record("A0427", "4", "(als 1 - emergency)") + // the description goes on to a second line
		record("   GT", "7", "Via interactive audio and video telecommunication systems") +
		record("   XU", "7", "Unusual non-overlapping service, the use of a service that is distinct") +
		record("   XU", "8", "because it does not overlap usual components of the main service") +
		"\r\n"
	ref, err := ReadZip(writeZip(t, lines))
	if err != nil {
		t.Fatal(err)
	}

	for code, want := range map[string]string{
		"J1100": "Injection, dexamethasone sodium phosphate, 1 mg",
		"A0427": "Ambulance service, advanced life support, emergency transport, level 1 (als 1 - emergency)",
	} {
		if got := ref.Codes[code]; got != want {
			t.Errorf("code %s = %q, want %q", code, got, want)
		}
	}
	for mod, want := range map[string]string{
		"GT": "Via interactive audio and video telecommunication systems",
		"XU": "Unusual non-overlapping service, the use of a service that is distinct because it does not overlap usual components of the main service",
	} {
		if got := ref.Modifiers[mod]; got != want {
			t.Errorf("modifier %s = %q, want %q", mod, got, want)
		}
	}
	if ref.File != "HCPC2026_OCT_ANWEB_09232026.txt" {
		t.Errorf("File = %q", ref.File)
	}
	if len(ref.Ended) != 1 || ref.Ended["J0640"] != "2026-09-30" || ref.Codes["J0640"] != "Injection, leucovorin calcium, per 50 mg" {
		t.Errorf("Ended = %v and J0640 = %q; want only J0640, ended 2026-09-30, with its name", ref.Ended, ref.Codes["J0640"])
	}
}

func TestReadZipWithoutTheTextFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "other.zip")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	zw.Create("NOC codes_OCT2026.xlsx")
	zw.Close()
	f.Close()
	if _, err := ReadZip(path); err == nil || !strings.Contains(err.Error(), "no ANWEB") {
		t.Errorf("want an error naming the missing ANWEB file, got %v", err)
	}
}

// TestRealFile checks the counts against CMS's October 2026 file, once it has
// been downloaded into data/.
func TestRealFile(t *testing.T) {
	ref, err := ReadZip("../../../data/october-2026-alpha-numeric-hcpcs-file.zip")
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("the CMS HCPCS file isn't in data/ yet; run the server once to download it")
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(ref.Codes) != 8770 || len(ref.Modifiers) != 384 {
		t.Errorf("got %d codes and %d modifiers, want 8770 and 384", len(ref.Codes), len(ref.Modifiers))
	}
	if got := ref.Codes["J1100"]; got != "Injection, dexamethasone sodium phosphate, 1 mg" {
		t.Errorf("J1100 = %q", got)
	}
	if got := ref.Modifiers["GT"]; got != "Via interactive audio and video telecommunication systems" {
		t.Errorf("GT = %q", got)
	}
	// 1,322 codes carry a termination date, most from years ago; J0640 and
	// J9190 end this quarter.
	if len(ref.Ended) != 1322 || ref.Ended["J0640"] != "2026-09-30" || ref.Ended["J9190"] != "2026-09-30" {
		t.Errorf("got %d ended codes, J0640 %q, J9190 %q; want 1322, and 2026-09-30 for both", len(ref.Ended), ref.Ended["J0640"], ref.Ended["J9190"])
	}
}
