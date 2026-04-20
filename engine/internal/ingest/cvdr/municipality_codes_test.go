package cvdr

import (
	"archive/zip"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportMunicipalityCatalogAndLookup(t *testing.T) {
	t.Parallel()

	archivePath := filepath.Join(t.TempDir(), "cbs.zip")
	if err := writeTestMunicipalityZip(archivePath, [][]string{
		{"Identifier", "DimensionGroupId", "DimensionId", "Index", "Title", "Description"},
		{"GM0363", "G1", "RegioS", "1", "Amsterdam", ""},
		{"GM0518", "G1", "RegioS", "2", "'s-Gravenhage (gemeente)", ""},
		{"GM0687", "G1", "RegioS", "3", "Middelburg (Z.)", ""},
		{"GM0888", "G1", "RegioS", "4", "Beek (L.)", ""},
		{"GM0373", "G1", "RegioS", "5", "Bergen (NH.)", ""},
		{"GM0893", "G1", "RegioS", "6", "Bergen (L.)", ""},
	}); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	catalog, err := ImportMunicipalityCatalog(ImportMunicipalityCatalogRequest{
		ArchivePath:   archivePath,
		ReferenceYear: "2026",
	})
	if err != nil {
		t.Fatalf("import catalog: %v", err)
	}

	for _, tc := range []struct {
		name      string
		wantCode  string
		wantMatch string
	}{
		{name: "Amsterdam", wantCode: "0363", wantMatch: "exact"},
		{name: "'s-Gravenhage", wantCode: "0518", wantMatch: "bare_unique"},
		{name: "Middelburg", wantCode: "0687", wantMatch: "bare_unique"},
		{name: "Beek", wantCode: "0888", wantMatch: "bare_unique"},
		{name: "Bergen (NH)", wantCode: "0373", wantMatch: "exact"},
	} {
		match, ok := LookupMunicipalityCode(catalog, tc.name)
		if !ok {
			t.Fatalf("expected lookup success for %s", tc.name)
		}
		if match.Entry.Code != tc.wantCode {
			t.Fatalf("lookup %s: expected code %s, got %s", tc.name, tc.wantCode, match.Entry.Code)
		}
		if match.MatchKind != tc.wantMatch {
			t.Fatalf("lookup %s: expected match kind %s, got %s", tc.name, tc.wantMatch, match.MatchKind)
		}
	}

	if _, ok := LookupMunicipalityCode(catalog, "Bergen"); ok {
		t.Fatalf("expected Bergen to remain ambiguous")
	}
}

func TestBackfillMunicipalityCodes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "catalog.json")
	if err := writeJSONFile(catalogPath, MunicipalityCatalog{
		SchemaVersion: "1",
		CountryCode:   "NL",
		Municipalities: []MunicipalityCatalogEntry{
			{Code: "0363", Name: "Amsterdam"},
			{Code: "0687", Name: "Middelburg (Z.)"},
		},
	}); err != nil {
		t.Fatalf("write catalog: %v", err)
	}

	fixtureRoot := filepath.Join(dir, "fixtures")
	if err := os.MkdirAll(filepath.Join(fixtureRoot, "middelburg"), 0o755); err != nil {
		t.Fatalf("mkdir fixture root: %v", err)
	}
	path := filepath.Join(fixtureRoot, "middelburg", "2026-01-01.json")
	if err := writeJSONFile(path, map[string]any{
		"id":        "nl-middelburg-2026-01-01",
		"domain":    "tourist_tax",
		"lifecycle": "draft",
		"jurisdiction": map[string]any{
			"country_code": "NL",
			"country_name": "Netherlands",
			"region_code":  "",
			"region_name":  "",
		},
		"notes": "Auto-generated from CVDR publication analysis. Source-backed draft only; review required before conformance use. municipality_code is TODO until canonical municipal code mapping is added.",
		"rules": []map[string]any{
			{
				"id":                "r1",
				"municipality_code": "TODO",
				"municipality_name": "Middelburg",
				"notes":             "Generated from CVDR analysis; municipality_code still needs confirmation.",
			},
		},
	}); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	result, err := BackfillMunicipalityCodes(BackfillMunicipalityCodesRequest{
		CatalogPath: catalogPath,
		FixtureRoot: fixtureRoot,
	})
	if err != nil {
		t.Fatalf("backfill codes: %v", err)
	}
	if result.UpdatedCount != 1 {
		t.Fatalf("expected 1 updated fixture, got %d", result.UpdatedCount)
	}

	var got map[string]any
	if err := readJSON(path, &got); err != nil {
		t.Fatalf("read updated fixture: %v", err)
	}
	rules := got["rules"].([]any)
	rule := rules[0].(map[string]any)
	if rule["municipality_code"] != "0687" {
		t.Fatalf("expected municipality_code 0687, got %v", rule["municipality_code"])
	}
	if got["notes"] == nil || got["notes"].(string) == "" || containsString(got["notes"].(string), "TODO") {
		t.Fatalf("expected cleaned ruleset notes, got %v", got["notes"])
	}
	if containsString(rule["notes"].(string), "municipality_code still needs confirmation") {
		t.Fatalf("expected cleaned rule notes, got %v", rule["notes"])
	}
}

func writeTestMunicipalityZip(path string, rows [][]string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := zip.NewWriter(file)
	defer writer.Close()

	entry, err := writer.Create("RegioSCodes.csv")
	if err != nil {
		return err
	}
	csvWriter := csv.NewWriter(entry)
	csvWriter.Comma = ';'
	if err := csvWriter.WriteAll(rows); err != nil {
		return err
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

func containsString(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
