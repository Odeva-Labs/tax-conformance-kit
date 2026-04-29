package cvdr

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	DefaultCBSMunicipalityDatasetURL = "https://datasets.cbs.nl/CSV/CBS/nl/86247NED"
)

type MunicipalityCatalog struct {
	SchemaVersion  string                     `json:"schema_version"`
	CountryCode    string                     `json:"country_code"`
	ImportedAt     string                     `json:"imported_at"`
	Source         MunicipalityCatalogSource  `json:"source"`
	Municipalities []MunicipalityCatalogEntry `json:"municipalities"`
}

type MunicipalityCatalogSource struct {
	Kind          string `json:"kind"`
	DatasetID     string `json:"dataset_id"`
	ReferenceYear string `json:"reference_year"`
	SourceURL     string `json:"source_url"`
	ArchivePath   string `json:"archive_path"`
}

type MunicipalityCatalogEntry struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type ImportMunicipalityCatalogRequest struct {
	ArchivePath   string
	ReferenceYear string
	SourceURL     string
}

type MunicipalityMatch struct {
	Entry      MunicipalityCatalogEntry
	MatchKind  string
	LookupName string
}

type BackfillMunicipalityCodesRequest struct {
	CatalogPath string
	FixtureRoot string
}

type BackfillMunicipalityCodesResult struct {
	UpdatedCount    int      `json:"updated_count"`
	SkippedCount    int      `json:"skipped_count"`
	UnresolvedCount int      `json:"unresolved_count"`
	UpdatedPaths    []string `json:"updated_paths,omitempty"`
	UnresolvedNames []string `json:"unresolved_names,omitempty"`
}

type municipalityCatalogIndex struct {
	exact map[string]MunicipalityCatalogEntry
	bare  map[string]MunicipalityCatalogEntry
}

func ImportMunicipalityCatalog(req ImportMunicipalityCatalogRequest) (MunicipalityCatalog, error) {
	if req.ArchivePath == "" {
		return MunicipalityCatalog{}, fmt.Errorf("archive path is required")
	}

	reader, err := zip.OpenReader(req.ArchivePath)
	if err != nil {
		return MunicipalityCatalog{}, err
	}
	defer reader.Close()

	rows, err := readCSVFromZip(reader.File, "RegioSCodes.csv")
	if err != nil {
		return MunicipalityCatalog{}, err
	}

	entries := make([]MunicipalityCatalogEntry, 0, len(rows))
	for _, row := range rows {
		identifier := strings.TrimSpace(row["Identifier"])
		if !strings.HasPrefix(identifier, "GM") {
			continue
		}
		code := strings.TrimPrefix(identifier, "GM")
		name := strings.TrimSpace(row["Title"])
		if code == "" || name == "" {
			continue
		}
		entries = append(entries, MunicipalityCatalogEntry{
			Code: code,
			Name: name,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Code < entries[j].Code })

	return MunicipalityCatalog{
		SchemaVersion: "1",
		CountryCode:   "NL",
		ImportedAt:    time.Now().UTC().Format(time.RFC3339),
		Source: MunicipalityCatalogSource{
			Kind:          "cbs_statline_zip",
			DatasetID:     "86247NED",
			ReferenceYear: firstNonEmpty(req.ReferenceYear, "2026"),
			SourceURL:     firstNonEmpty(req.SourceURL, DefaultCBSMunicipalityDatasetURL),
			ArchivePath:   filepath.ToSlash(req.ArchivePath),
		},
		Municipalities: entries,
	}, nil
}

func ReadMunicipalityCatalog(path string) (MunicipalityCatalog, error) {
	var catalog MunicipalityCatalog
	if err := readJSON(path, &catalog); err != nil {
		return MunicipalityCatalog{}, err
	}
	return catalog, nil
}

func MunicipalityCatalogDataEqual(a, b MunicipalityCatalog) bool {
	if a.SchemaVersion != b.SchemaVersion || a.CountryCode != b.CountryCode {
		return false
	}
	if a.Source.Kind != b.Source.Kind ||
		a.Source.DatasetID != b.Source.DatasetID ||
		a.Source.ReferenceYear != b.Source.ReferenceYear ||
		a.Source.SourceURL != b.Source.SourceURL {
		return false
	}
	if len(a.Municipalities) != len(b.Municipalities) {
		return false
	}
	for i := range a.Municipalities {
		if a.Municipalities[i] != b.Municipalities[i] {
			return false
		}
	}
	return true
}

func LookupMunicipalityCode(catalog MunicipalityCatalog, name string) (MunicipalityMatch, bool) {
	index := buildMunicipalityCatalogIndex(catalog)
	key := normalizeMunicipalityName(name)
	if key == "" {
		return MunicipalityMatch{}, false
	}
	if entry, ok := index.exact[key]; ok {
		return MunicipalityMatch{Entry: entry, MatchKind: "exact", LookupName: name}, true
	}
	bareKey := simplifyMunicipalityName(name)
	if bareKey == "" {
		return MunicipalityMatch{}, false
	}
	if entry, ok := index.bare[bareKey]; ok {
		return MunicipalityMatch{Entry: entry, MatchKind: "bare_unique", LookupName: name}, true
	}
	return MunicipalityMatch{}, false
}

func BackfillMunicipalityCodes(req BackfillMunicipalityCodesRequest) (BackfillMunicipalityCodesResult, error) {
	if req.CatalogPath == "" {
		return BackfillMunicipalityCodesResult{}, fmt.Errorf("catalog path is required")
	}
	if req.FixtureRoot == "" {
		return BackfillMunicipalityCodesResult{}, fmt.Errorf("fixture root is required")
	}

	catalog, err := ReadMunicipalityCatalog(req.CatalogPath)
	if err != nil {
		return BackfillMunicipalityCodesResult{}, err
	}

	paths, err := filepath.Glob(filepath.Join(req.FixtureRoot, "*", "*.json"))
	if err != nil {
		return BackfillMunicipalityCodesResult{}, err
	}
	sort.Strings(paths)

	result := BackfillMunicipalityCodesResult{
		UpdatedPaths:    []string{},
		UnresolvedNames: []string{},
	}
	seenUnresolved := map[string]struct{}{}

	for _, path := range paths {
		var ruleset struct {
			ID               string           `json:"id"`
			Domain           string           `json:"domain"`
			Lifecycle        string           `json:"lifecycle"`
			Jurisdiction     map[string]any   `json:"jurisdiction"`
			Notes            string           `json:"notes"`
			AssessmentPolicy map[string]any   `json:"assessment_policy"`
			Rules            []map[string]any `json:"rules"`
		}
		if err := readJSON(path, &ruleset); err != nil {
			return BackfillMunicipalityCodesResult{}, err
		}

		if ruleset.Lifecycle != "draft" || !strings.Contains(ruleset.Notes, "Auto-generated from CVDR publication analysis.") {
			result.SkippedCount++
			continue
		}
		name := firstRuleMunicipalityName(ruleset.Rules)
		if name == "" {
			result.SkippedCount++
			continue
		}
		match, ok := LookupMunicipalityCode(catalog, name)
		if !ok {
			result.UnresolvedCount++
			if _, seen := seenUnresolved[name]; !seen {
				seenUnresolved[name] = struct{}{}
				result.UnresolvedNames = append(result.UnresolvedNames, name)
			}
			continue
		}

		changed := false
		for _, rule := range ruleset.Rules {
			code, _ := rule["municipality_code"].(string)
			if strings.TrimSpace(code) == "" || code == "TODO" {
				rule["municipality_code"] = match.Entry.Code
				changed = true
			}
			if notes, ok := rule["notes"].(string); ok {
				clean := strings.TrimSpace(strings.ReplaceAll(notes, "Generated from CVDR analysis; municipality_code still needs confirmation.", ""))
				if clean != notes {
					rule["notes"] = clean
					changed = true
				}
			}
		}

		cleanNotes := strings.TrimSpace(strings.ReplaceAll(ruleset.Notes, " municipality_code is TODO until canonical municipal code mapping is added.", ""))
		if cleanNotes != ruleset.Notes {
			ruleset.Notes = cleanNotes
			changed = true
		}

		if !changed {
			result.SkippedCount++
			continue
		}
		if err := writeJSONFile(path, ruleset); err != nil {
			return BackfillMunicipalityCodesResult{}, err
		}
		result.UpdatedCount++
		result.UpdatedPaths = append(result.UpdatedPaths, filepath.ToSlash(path))
	}

	if result.UpdatedCount > 0 {
		if err := reconcileGeneratedFixtureValidityWindows(req.FixtureRoot); err != nil {
			return BackfillMunicipalityCodesResult{}, err
		}
	}

	sort.Strings(result.UnresolvedNames)
	return result, nil
}

func buildMunicipalityCatalogIndex(catalog MunicipalityCatalog) municipalityCatalogIndex {
	exact := map[string]MunicipalityCatalogEntry{}
	bareBuckets := map[string][]MunicipalityCatalogEntry{}
	for _, entry := range catalog.Municipalities {
		exact[normalizeMunicipalityName(entry.Name)] = entry
		bareKey := simplifyMunicipalityName(entry.Name)
		if bareKey != "" {
			bareBuckets[bareKey] = append(bareBuckets[bareKey], entry)
		}
	}
	bare := map[string]MunicipalityCatalogEntry{}
	for key, entries := range bareBuckets {
		if len(entries) == 1 {
			bare[key] = entries[0]
		}
	}
	return municipalityCatalogIndex{exact: exact, bare: bare}
}

func readCSVFromZip(files []*zip.File, target string) ([]map[string]string, error) {
	for _, file := range files {
		if filepath.Base(file.Name) != target {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer reader.Close()

		csvReader := csv.NewReader(reader)
		csvReader.Comma = ';'
		csvReader.FieldsPerRecord = -1
		headers, err := csvReader.Read()
		if err != nil {
			return nil, err
		}
		stripBOM(headers)

		rows := []map[string]string{}
		for {
			record, err := csvReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			row := map[string]string{}
			for i, header := range headers {
				if i < len(record) {
					row[header] = record[i]
				}
			}
			rows = append(rows, row)
		}
		return rows, nil
	}
	return nil, fmt.Errorf("zip entry not found: %s", target)
}

func stripBOM(values []string) {
	if len(values) == 0 {
		return
	}
	values[0] = strings.TrimPrefix(values[0], "\ufeff")
}

var municipalityParensPattern = regexp.MustCompile(`\s*\([^)]*\)`)
var municipalityNoisePattern = regexp.MustCompile(`[^[:alnum:]]+`)

func normalizeMunicipalityName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "’", "'")
	value = strings.ReplaceAll(value, "'", "")
	value = municipalityNoisePattern.ReplaceAllString(value, " ")
	value = strings.TrimSpace(strings.Join(strings.Fields(value), " "))
	return value
}

func simplifyMunicipalityName(value string) string {
	return normalizeMunicipalityName(municipalityParensPattern.ReplaceAllString(value, ""))
}

func firstRuleMunicipalityName(rules []map[string]any) string {
	for _, rule := range rules {
		if value, ok := rule["municipality_name"].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func DefaultMunicipalityCatalogPath(repoRoot string) string {
	return filepath.Join(repoRoot, "core", "data", "nl", "municipality-codes.cbs-2026.json")
}

func maybeReadMunicipalityCatalog(path string) (*MunicipalityCatalog, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	catalog, err := ReadMunicipalityCatalog(path)
	if err != nil {
		return nil, err
	}
	return &catalog, nil
}
